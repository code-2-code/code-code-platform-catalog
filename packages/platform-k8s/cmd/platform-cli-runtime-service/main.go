package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	cliruntimev1 "code-code.internal/go-contract/platform/cli_runtime/v1"
	"code-code.internal/platform-k8s/cliruntime"
	"code-code.internal/platform-k8s/cliversions"
	"code-code.internal/platform-k8s/domainevents"
	"code-code.internal/platform-k8s/internal/triggerhttp"
	"code-code.internal/platform-k8s/state"
	"code-code.internal/platform-k8s/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	namespace := envOrDefault("PLATFORM_CLI_RUNTIME_SERVICE_NAMESPACE", "code-code")
	runNamespace := envOrDefault("PLATFORM_CLI_RUNTIME_SERVICE_RUN_NAMESPACE", "code-code-runs")
	grpcAddr := envOrDefault("PLATFORM_CLI_RUNTIME_SERVICE_GRPC_ADDR", ":8081")
	httpAddr := envOrDefault("PLATFORM_CLI_RUNTIME_SERVICE_HTTP_ADDR", ":8080")
	imageRegistry := envOrDefault("PLATFORM_CLI_RUNTIME_SERVICE_IMAGE_REGISTRY", "")
	imageRegistryLookup := envOrDefault("PLATFORM_CLI_RUNTIME_SERVICE_IMAGE_REGISTRY_LOOKUP_PREFIX", "")
	registryInsecure, err := boolEnv("PLATFORM_CLI_RUNTIME_SERVICE_IMAGE_REGISTRY_LOOKUP_INSECURE")
	must(err)
	registryTimeout, err := durationEnv("PLATFORM_CLI_RUNTIME_SERVICE_REGISTRY_LIST_TIMEOUT")
	must(err)
	registryConcurrency, err := positiveIntEnv("PLATFORM_CLI_RUNTIME_SERVICE_REGISTRY_LIST_CONCURRENCY")
	must(err)
	sourceContext := envOrDefault("PLATFORM_CLI_RUNTIME_SERVICE_IMAGE_BUILD_SOURCE_CONTEXT", "")
	sourceRevision := envOrDefault("PLATFORM_CLI_RUNTIME_SERVICE_IMAGE_BUILD_SOURCE_REVISION", "")
	domainEventsNATSURL := envOrDefault("PLATFORM_CLI_RUNTIME_SERVICE_DOMAIN_EVENTS_NATS_URL", "")
	databaseURL := firstEnv("PLATFORM_DATABASE_URL", "PLATFORM_CLI_RUNTIME_SERVICE_DATABASE_URL")

	telemetryShutdown, err := telemetry.Setup(context.Background(), envOrDefault("OTEL_SERVICE_NAME", "platform-cli-runtime-service"))
	must(err)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := telemetryShutdown(shutdownCtx); err != nil {
			log.Printf("shutdown telemetry failed: %v", err)
		}
	}()

	statePool, err := state.OpenPostgres(context.Background(), databaseURL, "platform-cli-runtime-service")
	must(err)
	defer statePool.Close()
	outbox, err := domainevents.NewOutbox(statePool, "platform-cli-runtime-service")
	must(err)
	versionStore, err := cliversions.NewPostgresStore(statePool)
	must(err)
	versionSyncer, err := cliversions.NewSyncer(cliversions.SyncerConfig{
		Store:  versionStore,
		Logger: slog.Default(),
	})
	must(err)
	eventDispatcher, err := cliruntime.NewDomainEventDispatcher(outbox)
	must(err)
	service, err := cliruntime.NewService(cliruntime.Config{
		VersionSyncer:  versionSyncer,
		Dispatcher:     eventDispatcher,
		ImageRegistry:  imageRegistry,
		SourceContext:  sourceContext,
		SourceRevision: sourceRevision,
		Logger:         slog.Default(),
	})
	must(err)
	grpcService, err := cliruntime.NewServer(cliruntime.ServerConfig{
		Versions:            versionStore,
		ImageRegistry:       imageRegistry,
		ImageRegistryLookup: imageRegistryLookup,
		RegistryInsecure:    registryInsecure,
		RegistryTimeout:     registryTimeout,
		RegistryConcurrency: registryConcurrency,
	})
	must(err)
	triggerServer, err := triggerhttp.NewServer(triggerhttp.Config{
		Logger: slog.Default(),
		Actions: map[string]triggerhttp.ActionFunc{
			"sync-cli-versions": func(ctx context.Context, _ triggerhttp.Request) (any, error) {
				return service.SyncCLIVersions(ctx)
			},
		},
	})
	must(err)

	grpcListener, err := net.Listen("tcp", grpcAddr)
	must(err)
	httpListener, err := net.Listen("tcp", httpAddr)
	must(err)
	httpServer := &http.Server{Handler: triggerServer}
	grpcServer := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	cliruntimev1.RegisterCLIRuntimeServiceServer(grpcServer, grpcService)
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(cliruntimev1.CLIRuntimeService_ServiceDesc.ServiceName, healthv1.HealthCheckResponse_SERVING)
	healthv1.RegisterHealthServer(grpcServer, healthServer)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if domainEventsNATSURL != "" {
		startDomainEventRuntime(ctx, outbox, domainEventsNATSURL)
	}
	serveErr := make(chan error, 2)
	go func() {
		if err := grpcServer.Serve(grpcListener); err != nil {
			serveErr <- err
		}
	}()
	go func() {
		if err := httpServer.Serve(httpListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()
	go func() {
		<-ctx.Done()
		healthServer.SetServingStatus("", healthv1.HealthCheckResponse_NOT_SERVING)
		healthServer.SetServingStatus(cliruntimev1.CLIRuntimeService_ServiceDesc.ServiceName, healthv1.HealthCheckResponse_NOT_SERVING)
		grpcServer.GracefulStop()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("platform-cli-runtime-service starting (namespace=%s run_namespace=%s grpc=%s http=%s)", namespace, runNamespace, grpcAddr, httpAddr)
	select {
	case err := <-serveErr:
		must(err)
	case <-ctx.Done():
	}
}

func startDomainEventRuntime(ctx context.Context, outbox *domainevents.Outbox, natsURL string) {
	publisher, err := domainevents.NewPublisher(outbox, domainevents.PublisherConfig{
		NATSURL:    natsURL,
		ClientName: "platform-cli-runtime-service-domain-publisher",
	})
	must(err)
	go func() { _ = publisher.Run(ctx) }()
	log.Printf("domain event publisher enabled (nats=%s)", natsURL)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
