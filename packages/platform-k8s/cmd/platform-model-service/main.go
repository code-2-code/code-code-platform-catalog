package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"code-code.internal/go-contract/platform/model/v1/modelservicev1connect"
	platformk8s "code-code.internal/platform-k8s"
	"code-code.internal/platform-k8s/internal/modelservice"
	"code-code.internal/platform-k8s/internal/platform/domainevents"
	"code-code.internal/platform-k8s/internal/platform/state"
	"code-code.internal/platform-k8s/internal/platform/telemetry"
	"code-code.internal/platform-k8s/internal/platform/triggerhttp"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	namespace := envOrDefault("PLATFORM_MODEL_SERVICE_NAMESPACE", "code-code")
	grpcAddr := envOrDefault("PLATFORM_MODEL_SERVICE_GRPC_ADDR", ":8081")
	httpAddr := envOrDefault("PLATFORM_MODEL_SERVICE_HTTP_ADDR", ":8080")
	domainEventsNATSURL := envOrDefault("PLATFORM_MODEL_SERVICE_DOMAIN_EVENTS_NATS_URL", "")
	internalActionToken := strings.TrimSpace(os.Getenv("PLATFORM_MODEL_SERVICE_INTERNAL_ACTION_TOKEN"))
	databaseURL := firstEnv("PLATFORM_DATABASE_URL", "PLATFORM_MODEL_SERVICE_DATABASE_URL")

	telemetryShutdown, err := telemetry.Setup(context.Background(), envOrDefault("OTEL_SERVICE_NAME", "platform-model-service"))
	must(err)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := telemetryShutdown(shutdownCtx); err != nil {
			log.Printf("shutdown telemetry failed: %v", err)
		}
	}()

	scheme := runtime.NewScheme()
	must(corev1.AddToScheme(scheme))
	must(coordinationv1.AddToScheme(scheme))
	must(platformk8s.AddToScheme(scheme))

	config := ctrl.GetConfigOrDie()
	statePool, err := state.OpenPostgres(context.Background(), databaseURL, "platform-model-service")
	must(err)
	defer statePool.Close()

	directClient, err := ctrlclient.New(config, ctrlclient.Options{Scheme: scheme})
	must(err)
	outbox, err := domainevents.NewOutbox(statePool, "platform-model-service")
	must(err)

	server, err := modelservice.NewServer(modelservice.Config{
		Client:    directClient,
		Reader:    directClient,
		StatePool: statePool,
		Outbox:    outbox,
		Namespace: namespace,
	})
	must(err)

	triggerServer, err := triggerhttp.NewServer(triggerhttp.Config{
		Logger: slog.Default(),
		Actions: map[string]triggerhttp.ActionFunc{
			"sync-model-definitions": func(ctx context.Context, _ triggerhttp.Request) (any, error) {
				response, err := server.SyncModelDefinitions(ctx, &modelservicev1.SyncModelDefinitionsRequest{})
				if err != nil {
					return nil, err
				}
				return map[string]string{"status": response.GetStatus()}, nil
			},
		},
		AuthToken: internalActionToken,
	})
	must(err)
	if internalActionToken == "" {
		log.Printf("internal action endpoints are disabled (env PLATFORM_MODEL_SERVICE_INTERNAL_ACTION_TOKEN is empty)")
	}

	listener, err := net.Listen("tcp", grpcAddr)
	must(err)
	httpListener, err := net.Listen("tcp", httpAddr)
	must(err)
	var natsHealthConn *nats.Conn
	var natsHealthJS jetstream.JetStream
	if domainEventsNATSURL != "" {
		natsHealthConn, err = nats.Connect(domainEventsNATSURL, nats.Name("platform-model-service-readiness"))
		must(err)
		defer natsHealthConn.Close()
		natsHealthJS, err = jetstream.New(natsHealthConn)
		must(err)
	}
	httpMux := http.NewServeMux()
	_, modelConnectHandler := modelservicev1connect.NewModelServiceHandler(server)
	httpMux.Handle(modelservicev1connect.ModelServiceListModelsProcedure, modelConnectHandler)
	httpMux.HandleFunc("/readyz", readyzHandler(slog.Default(), statePool, natsHealthJS))
	httpMux.Handle("/", triggerServer)
	httpServer := &http.Server{Handler: httpMux}
	grpcServer := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	modelservicev1.RegisterModelServiceServer(grpcServer, server)

	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(modelservicev1.ModelService_ServiceDesc.ServiceName, healthv1.HealthCheckResponse_SERVING)
	healthv1.RegisterHealthServer(grpcServer, healthServer)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if domainEventsNATSURL != "" {
		publisher, err := domainevents.NewPublisher(outbox, domainevents.PublisherConfig{
			NATSURL:    domainEventsNATSURL,
			ClientName: "platform-model-service-domain-publisher",
		})
		must(err)
		defer publisher.Close()
		go func() { _ = publisher.Run(ctx) }()
		log.Printf("domain event publisher enabled (nats=%s)", domainEventsNATSURL)
	}
	serveErr := make(chan error, 2)
	go func() { serveErr <- grpcServer.Serve(listener) }()
	go func() {
		if err := httpServer.Serve(httpListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	log.Printf("platform-model-service starting (namespace=%s grpc=%s http=%s)", namespace, grpcAddr, httpAddr)
	select {
	case err := <-serveErr:
		must(err)
	case <-ctx.Done():
		healthServer.SetServingStatus("", healthv1.HealthCheckResponse_NOT_SERVING)
		healthServer.SetServingStatus(modelservicev1.ModelService_ServiceDesc.ServiceName, healthv1.HealthCheckResponse_NOT_SERVING)
		grpcServer.Stop()
		_ = httpServer.Close()
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		value := os.Getenv(key)
		if value != "" {
			return value
		}
	}
	return ""
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
