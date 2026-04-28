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
	egressservicev1 "code-code.internal/go-contract/platform/egress/v1"
	supportv1 "code-code.internal/go-contract/platform/support/v1"
	"code-code.internal/platform-k8s/internal/cliruntimeservice/cliruntime"
	"code-code.internal/platform-k8s/internal/cliruntimeservice/cliversions"
	"code-code.internal/platform-k8s/internal/platform/domainevents"
	"code-code.internal/platform-k8s/internal/platform/state"
	"code-code.internal/platform-k8s/internal/platform/telemetry"
	"code-code.internal/platform-k8s/internal/platform/temporalruntime"
	"code-code.internal/platform-k8s/internal/supportservice"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	temporalclient "go.temporal.io/sdk/client"
	temporalworker "go.temporal.io/sdk/worker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	namespace := envOrDefault("PLATFORM_SUPPORT_SERVICE_NAMESPACE", "code-code")
	runNamespace := envOrDefault("PLATFORM_SUPPORT_SERVICE_RUN_NAMESPACE", "code-code-runs")
	grpcAddr := envOrDefault("PLATFORM_SUPPORT_SERVICE_GRPC_ADDR", ":8081")
	httpAddr := envOrDefault("PLATFORM_SUPPORT_SERVICE_HTTP_ADDR", ":8080")
	egressAddr := envOrDefault("PLATFORM_SUPPORT_SERVICE_EGRESS_GRPC_ADDR", "platform-egress-service.code-code-net.svc.cluster.local:8081")
	imageRegistry := envOrDefault("PLATFORM_SUPPORT_SERVICE_IMAGE_REGISTRY", "")
	imageRegistryLookup := envOrDefault("PLATFORM_SUPPORT_SERVICE_IMAGE_REGISTRY_LOOKUP_PREFIX", "")
	registryInsecure, err := boolEnv("PLATFORM_SUPPORT_SERVICE_IMAGE_REGISTRY_LOOKUP_INSECURE")
	must(err)
	registryTimeout, err := durationEnv("PLATFORM_SUPPORT_SERVICE_REGISTRY_LIST_TIMEOUT")
	must(err)
	registryConcurrency, err := positiveIntEnv("PLATFORM_SUPPORT_SERVICE_REGISTRY_LIST_CONCURRENCY")
	must(err)
	sourceContext := envOrDefault("PLATFORM_SUPPORT_SERVICE_IMAGE_BUILD_SOURCE_CONTEXT", "")
	sourceRevision := envOrDefault("PLATFORM_SUPPORT_SERVICE_IMAGE_BUILD_SOURCE_REVISION", "")
	domainEventsNATSURL := envOrDefault("PLATFORM_SUPPORT_SERVICE_DOMAIN_EVENTS_NATS_URL", "")
	databaseURL := firstEnv("PLATFORM_DATABASE_URL", "PLATFORM_SUPPORT_SERVICE_DATABASE_URL")

	telemetryShutdown, err := telemetry.Setup(context.Background(), envOrDefault("OTEL_SERVICE_NAME", "platform-support-service"))
	must(err)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := telemetryShutdown(shutdownCtx); err != nil {
			log.Printf("shutdown telemetry failed: %v", err)
		}
	}()

	scheme := runtime.NewScheme()
	must(batchv1.AddToScheme(scheme))
	must(corev1.AddToScheme(scheme))
	client, err := ctrlclient.New(ctrl.GetConfigOrDie(), ctrlclient.Options{Scheme: scheme})
	must(err)
	statePool, err := state.OpenPostgres(context.Background(), databaseURL, "platform-support-service")
	must(err)
	defer statePool.Close()
	outbox, err := domainevents.NewOutbox(statePool, "platform-support-service")
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
	supportServer, err := supportservice.NewServer(supportservice.Config{
		Reader:    client,
		Namespace: namespace,
	})
	must(err)
	egressConn, err := grpc.NewClient(egressAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	must(err)
	defer func() { _ = egressConn.Close() }()
	egressClient := egressservicev1.NewEgressServiceClient(egressConn)
	imageBuildRunner, err := cliruntime.NewImageBuildJobRunner(client, runNamespace)
	must(err)
	temporalConfig := temporalruntime.ConfigFromEnv(cliruntime.TemporalTaskQueue)
	temporalRuntime, err := temporalruntime.Start(context.Background(), temporalConfig, func(worker temporalworker.Worker) error {
		return cliruntime.RegisterTemporalWorkflows(worker, service, imageBuildRunner)
	}, cliruntime.EnsureTemporalSchedules)
	must(err)
	defer temporalRuntime.Stop()

	grpcListener, err := net.Listen("tcp", grpcAddr)
	must(err)
	httpListener, err := net.Listen("tcp", httpAddr)
	must(err)
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	httpServer := &http.Server{Handler: httpMux}
	grpcServer := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	cliruntimev1.RegisterCLIRuntimeServiceServer(grpcServer, grpcService)
	supportv1.RegisterSupportServiceServer(grpcServer, supportServer)
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(cliruntimev1.CLIRuntimeService_ServiceDesc.ServiceName, healthv1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(supportv1.SupportService_ServiceDesc.ServiceName, healthv1.HealthCheckResponse_SERVING)
	healthv1.RegisterHealthServer(grpcServer, healthServer)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if domainEventsNATSURL != "" {
		startDomainEventRuntime(ctx, statePool, outbox, temporalRuntime.Client, temporalConfig.TaskQueue, domainEventsNATSURL)
	}
	go func() {
		if err := supportservice.SyncStartupExternalAccessSets(ctx, egressClient); err != nil && ctx.Err() == nil {
			log.Printf("startup external access sync failed: %v", err)
		}
	}()
	go func() {
		if err := supportservice.SyncStartupRuntimeTelemetryProfiles(ctx, supportServer, egressClient); err != nil && ctx.Err() == nil {
			log.Printf("startup runtime telemetry sync failed: %v", err)
		}
	}()
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
		healthServer.SetServingStatus(supportv1.SupportService_ServiceDesc.ServiceName, healthv1.HealthCheckResponse_NOT_SERVING)
		grpcServer.GracefulStop()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("platform-support-service starting (namespace=%s run_namespace=%s grpc=%s http=%s temporal=%s/%s)", namespace, runNamespace, grpcAddr, httpAddr, temporalConfig.Address, temporalConfig.TaskQueue)
	select {
	case err := <-serveErr:
		must(err)
	case <-ctx.Done():
	}
}

func startDomainEventRuntime(ctx context.Context, pool *pgxpool.Pool, outbox *domainevents.Outbox, temporalClient temporalclient.Client, taskQueue string, natsURL string) {
	publisher, err := domainevents.NewPublisher(outbox, domainevents.PublisherConfig{
		NATSURL:    natsURL,
		ClientName: "platform-support-service-domain-publisher",
	})
	must(err)
	go func() { _ = publisher.Run(ctx) }()
	dispatcher, err := cliruntime.NewTemporalImageBuildDispatcher(temporalClient, taskQueue)
	must(err)
	consumer, err := domainevents.NewConsumer(pool, domainevents.ConsumerConfig{
		NATSURL:     natsURL,
		ClientName:  "platform-support-service-image-build-dispatcher",
		DurableName: "platform-support-service-image-build-dispatcher",
		FilterSubjects: []string{
			domainevents.SubjectPrefix + "." + domainevents.AggregateCLIRuntime + ".image_build_requested",
		},
	}, dispatcher.HandleDomainEvent)
	must(err)
	go func() { _ = consumer.Run(ctx) }()
	log.Printf("domain event bus enabled (nats=%s)", natsURL)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
