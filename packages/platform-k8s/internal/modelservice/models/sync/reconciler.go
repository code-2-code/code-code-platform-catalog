package sync

import (
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"context"
	"fmt"
	"log/slog"
	"net/http"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// DefinitionSyncReconciler canonicalizes collected vendor model metadata into
// the model-service registry read model.
type DefinitionSyncReconciler struct {
	client    ctrlclient.Client
	namespace string
	store     models.DefinitionSyncStore
	logger    *slog.Logger
	metrics   *collectorMetrics

	// Infrastructure dependencies — injected for testability.
	listVendors   func(ctx context.Context) ([]configuredVendor, error)
	newHTTPClient func(ctx context.Context) (*http.Client, error)
}

// ReconcilerConfig groups dependencies for DefinitionSyncReconciler.
type ReconcilerConfig struct {
	Client    ctrlclient.Client
	Store     models.DefinitionSyncStore
	Namespace string
	Logger    *slog.Logger
}

// NewReconciler creates one model sync reconciler.
func NewReconciler(config ReconcilerConfig) (*DefinitionSyncReconciler, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("platformk8s/models: sync client is nil")
	}
	if config.Store == nil {
		return nil, fmt.Errorf("platformk8s/models: sync store is nil")
	}
	if config.Namespace == "" {
		return nil, fmt.Errorf("platformk8s/models: sync namespace is empty")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	metrics, err := registerCollectorMetrics()
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: register collector metrics: %w", err)
	}
	r := &DefinitionSyncReconciler{
		client:    config.Client,
		namespace: config.Namespace,
		store:     config.Store,
		logger:    config.Logger,
		metrics:   metrics,
	}
	r.listVendors = r.listConfiguredVendorsDefault
	r.newHTTPClient = r.newCollectionHTTPClientDefault
	return r, nil
}

// SyncNow refreshes authoritative collections immediately and applies them to
// managed model registry entries.
func (r *DefinitionSyncReconciler) SyncNow(ctx context.Context) error {
	snapshot, err := r.collectAuthoritativeDefinitions(ctx)
	if err != nil {
		return err
	}
	return r.sync(ctx, snapshot)
}
