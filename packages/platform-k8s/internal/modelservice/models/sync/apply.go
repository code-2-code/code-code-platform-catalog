package sync

import (
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"context"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (r *DefinitionSyncReconciler) sync(ctx context.Context, snapshot *collectedDefinitionsSnapshot) error {
	if snapshot == nil {
		return nil
	}

	definitions, err := r.store.ListManagedDefinitions(ctx)
	if err != nil {
		return err
	}
	existingByIdentity := make(map[string]models.StoredDefinition, len(definitions))
	for _, definition := range definitions {
		if definition.Definition == nil {
			continue
		}
		identity, err := models.IdentityFromDefinition(definition.Definition)
		if err != nil {
			return err
		}
		if _, ok := existingByIdentity[identity.Key()]; ok {
			return fmt.Errorf("platformk8s/models: duplicate model definition identity %q/%q", identity.VendorID, identity.ModelID)
		}
		existingByIdentity[identity.Key()] = definition
	}

	keys := make([]string, 0, len(snapshot.definitions))
	for key := range snapshot.definitions {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		candidate := snapshot.definitions[key]
		if candidate.GetDefinition() == nil {
			return fmt.Errorf("platformk8s/models: collected model definition %q is nil", key)
		}
		identity, err := models.IdentityFromDefinition(candidate.GetDefinition())
		if err != nil {
			return err
		}
		if err := r.store.UpsertManagedDefinition(
			ctx,
			models.NewModelRegistryEntry(candidate),
		); err != nil {
			return fmt.Errorf("platformk8s/models: upsert vendor-support model definition %q/%q: %w", identity.VendorID, identity.ModelID, err)
		}
	}

	for _, existing := range definitions {
		identity, err := models.IdentityFromDefinition(existing.Definition)
		if err != nil {
			return err
		}
		if _, ok := snapshot.definitions[identity.Key()]; ok {
			continue
		}
		if shouldKeepManagedDefinition(identity.VendorID, snapshot) {
			continue
		}
		if err := r.store.DeleteManagedDefinition(ctx, identity); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("platformk8s/models: delete stale vendor-support model definition %q/%q: %w", identity.VendorID, identity.ModelID, err)
		}
	}
	return nil
}

func shouldKeepManagedDefinition(vendorID string, snapshot *collectedDefinitionsSnapshot) bool {
	if snapshot == nil {
		return true
	}
	if _, configured := snapshot.managedVendorIDs[vendorID]; !configured {
		return false
	}
	if _, collected := snapshot.collectedVendorIDs[vendorID]; collected {
		return false
	}
	return true
}
