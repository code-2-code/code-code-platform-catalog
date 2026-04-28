package sync

import (
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"log/slog"
	"maps"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"code-code.internal/platform-k8s/internal/modelservice/models/source"
)

// newCollectionContext creates a CollectionContext from the internal vendor scope.
func newCollectionContext(scope configuredVendorScope) source.CollectionContext {
	return source.CollectionContext{
		ResolveVendor:   func(raw string) (string, bool) { return scope.canonicalVendorID(raw) },
		AliasCandidates: scope.aliasCandidates,
	}
}

type collectedDefinitionsSnapshot struct {
	vendorScope         configuredVendorScope
	configuredVendorIDs map[string]struct{}
	managedVendorIDs    map[string]struct{}
	collectedVendorIDs  map[string]struct{}
	definitions         map[string]*modelservicev1.CollectedModelEntry
}

func newCollectedDefinitionsSnapshot(scope configuredVendorScope) *collectedDefinitionsSnapshot {
	return &collectedDefinitionsSnapshot{
		vendorScope:         cloneConfiguredVendorScope(scope),
		configuredVendorIDs: cloneStringSet(scope.canonicalIDs),
		managedVendorIDs:    cloneStringSet(scope.managedIDs),
		collectedVendorIDs:  map[string]struct{}{},
		definitions:         map[string]*modelservicev1.CollectedModelEntry{},
	}
}

func mergeCollectedDefinitionsSnapshot(snapshot *collectedDefinitionsSnapshot, grouped map[string][]*modelservicev1.CollectedModelEntry, logger *slog.Logger) *collectedDefinitionsSnapshot {
	if logger == nil {
		logger = slog.Default()
	}
	if snapshot == nil {
		snapshot = newCollectedDefinitionsSnapshot(configuredVendorScope{})
	}

	for vendorID, items := range grouped {
		if len(items) == 0 {
			continue
		}
		snapshot.collectedVendorIDs[vendorID] = struct{}{}
		for index, item := range items {
			candidate, err := vendorSupportDefinitionCandidate(vendorID, item.GetDefinition())
			if err != nil {
				logger.Warn("skip invalid collected model definition",
					"vendor_id", vendorID,
					"source_index", index,
					"error", err,
				)
				continue
			}
			identity, err := models.IdentityFromDefinition(candidate)
			if err != nil {
				logger.Warn("skip collected model definition with invalid identity",
					"vendor_id", vendorID,
					"model_id", candidate.GetModelId(),
					"error", err,
				)
				continue
			}
			collected := &modelservicev1.CollectedModelEntry{
				Definition: candidate,
				Badges:     models.NormalizeDefinitionSourceBadges(item.GetBadges()),
				Pricing:    models.ClonePricingSummary(item.GetPricing()),
				Sources:    models.CloneCollectedSources(item.GetSources()),
			}
			if current, ok := snapshot.definitions[identity.Key()]; ok {
				collected = models.MergeCollectedEntries(current, collected)
			}
			snapshot.definitions[identity.Key()] = collected
		}
	}

	return snapshot
}



func cloneStringSet(values map[string]struct{}) map[string]struct{} {
	if len(values) == 0 {
		return map[string]struct{}{}
	}
	return maps.Clone(values)
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	return maps.Clone(values)
}

func cloneStringSliceMap(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(values))
	for key, items := range values {
		out[key] = append([]string(nil), items...)
	}
	return out
}



