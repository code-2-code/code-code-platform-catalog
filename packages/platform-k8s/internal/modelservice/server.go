package modelservice

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"code-code.internal/platform-k8s/internal/modelservice/models/store"
	modelsync "code-code.internal/platform-k8s/internal/modelservice/models/sync"
	"code-code.internal/platform-k8s/internal/platform/domainevents"
	"github.com/jackc/pgx/v5/pgxpool"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	Client    ctrlclient.Client
	Reader    ctrlclient.Reader
	StatePool *pgxpool.Pool
	Outbox    *domainevents.Outbox
	Namespace string
	Logger    *slog.Logger
}

type Server struct {
	modelservicev1.UnimplementedModelServiceServer

	registry       modelRegistry
	modelCardStore *store.PostgresModelCardStore
	syncer         *modelsync.DefinitionSyncReconciler
	metrics        *serverMetrics
	logger         *slog.Logger
}

type modelRegistry interface {
	List(context.Context, *modelservicev1.ListModelsRequest) (*modelservicev1.ListModelsResponse, error)
	Resolve(context.Context, *modelservicev1.ResolveModelRefRequest) (*modelservicev1.ResolveModelRefResponse, error)
	Get(context.Context, *modelservicev1.GetModelVersionRequest) (*modelservicev1.GetModelVersionResponse, error)
}

func NewServer(config Config) (*Server, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("platformk8s/modelservice: client is nil")
	}
	if config.Reader == nil {
		config.Reader = config.Client
	}
	if strings.TrimSpace(config.Namespace) == "" {
		return nil, fmt.Errorf("platformk8s/modelservice: namespace is empty")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	metrics, err := registerServerMetrics()
	if err != nil {
		return nil, err
	}
	registry, err := store.NewPostgresModelRegistry(config.StatePool, config.Outbox, config.Namespace)
	if err != nil {
		return nil, err
	}
	syncer, err := modelsync.NewReconciler(modelsync.ReconcilerConfig{
		Client:    config.Client,
		Store:     registry,
		Namespace: registry.Namespace(),
		Logger:    config.Logger,
	})
	if err != nil {
		return nil, err
	}
	var modelCardStore *store.PostgresModelCardStore
	if config.StatePool != nil {
		var mcErr error
		modelCardStore, mcErr = store.NewPostgresModelCardStore(config.StatePool, config.Namespace)
		if mcErr != nil {
			return nil, mcErr
		}
	}
	return &Server{
		registry:       registry,
		modelCardStore: modelCardStore,
		syncer:         syncer,
		metrics:        metrics,
		logger:         config.Logger,
	}, nil
}

func (s *Server) ListModels(ctx context.Context, request *modelservicev1.ListModelsRequest) (*modelservicev1.ListModelsResponse, error) {
	started := time.Now()
	metrics := s.metricsOrNil()
	response, err := s.registry.List(ctx, request)
	metrics.recordRegistryQuery("ListModels", started, err)
	if err != nil {
		return nil, grpcError(err)
	}
	return response, nil
}

func (s *Server) ResolveModelRef(ctx context.Context, request *modelservicev1.ResolveModelRefRequest) (*modelservicev1.ResolveModelRefResponse, error) {
	started := time.Now()
	metrics := s.metricsOrNil()
	response, err := s.registry.Resolve(ctx, request)
	metrics.recordRegistryQuery("ResolveModelRef", started, err)
	if err != nil {
		return nil, grpcError(err)
	}
	return response, nil
}

func (s *Server) GetModelVersion(ctx context.Context, request *modelservicev1.GetModelVersionRequest) (*modelservicev1.GetModelVersionResponse, error) {
	started := time.Now()
	metrics := s.metricsOrNil()
	response, err := s.registry.Get(ctx, request)
	metrics.recordRegistryQuery("GetModelVersion", started, err)
	if err != nil {
		return nil, grpcError(err)
	}
	return response, nil
}

func (s *Server) GetModelCard(ctx context.Context, request *modelservicev1.GetModelCardRequest) (*modelservicev1.GetModelCardResponse, error) {
	if s.modelCardStore == nil {
		return s.UnimplementedModelServiceServer.GetModelCard(ctx, request)
	}
	started := time.Now()
	metrics := s.metricsOrNil()
	card, err := s.modelCardStore.Get(ctx, request.GetRef())
	metrics.recordRegistryQuery("GetModelCard", started, err)
	if err != nil {
		return nil, grpcError(err)
	}
	return &modelservicev1.GetModelCardResponse{Card: card}, nil
}

func (s *Server) SyncModelDefinitions(ctx context.Context, _ *modelservicev1.SyncModelDefinitionsRequest) (*modelservicev1.SyncModelDefinitionsResponse, error) {
	started := time.Now()
	metrics := s.metricsOrNil()
	if s == nil || s.syncer == nil {
		err := fmt.Errorf("platformk8s/modelservice: model definition syncer is not initialized")
		metrics.recordDefinitionSync(started, err)
		return nil, grpcError(err)
	}
	err := s.syncer.SyncNow(ctx)
	metrics.recordDefinitionSync(started, err)
	if err != nil {
		return nil, grpcError(err)
	}
	return &modelservicev1.SyncModelDefinitionsResponse{Status: "completed"}, nil
}

func (s *Server) metricsOrNil() *serverMetrics {
	if s == nil {
		return nil
	}
	return s.metrics
}
