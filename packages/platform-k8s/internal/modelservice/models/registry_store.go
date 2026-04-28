package models

import (
	"context"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
)

// DefinitionSyncStore is the store contract used by DefinitionSyncReconciler.
type DefinitionSyncStore interface {
	ListManagedDefinitions(context.Context) ([]StoredDefinition, error)
	UpsertManagedDefinition(context.Context, *modelservicev1.ModelRegistryEntry) error
	DeleteManagedDefinition(context.Context, SurfaceIdentity) error
}
