package source

import (
	"slices"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"code-code.internal/platform-k8s/internal/modelservice/modelidentity"
)

// NormalizeHostedModels is a generic helper for external hosted model sources (e.g. NVIDIA, ModelScope).
// The project function extracts owner, rawModelID, displayName, isDirect, ok from each item.
func NormalizeHostedModels[T any](
	sourceID string,
	items []T,
	ctx CollectionContext,
	project func(T) (owner string, rawModelID string, displayName string, isDirect bool, ok bool),
) map[string][]*CollectedEntry {
	byVendor := map[string]map[string]*CollectedEntry{}
	for _, item := range items {
		owner, rawModelID, displayName, isDirect, ok := project(item)
		if !ok {
			continue
		}
		vendorID, ok := ctx.ResolveVendor(owner)
		if !ok {
			continue
		}
		modelID, aliases, ok := modelidentity.NormalizeExternalModelIdentity(vendorID, rawModelID)
		if !ok {
			continue
		}
		entry := &CollectedEntry{
			Definition: &modelv1.ModelVersion{
				ModelId:     modelID,
				DisplayName: strings.TrimSpace(displayName),
				VendorId:    vendorID,
				Aliases:     aliases,
			},
			Sources: []*CollectedSource{{
				VendorId:    vendorID,
				ModelId:     rawModelID,
				SourceId:    sourceID,
				DisplayName: strings.TrimSpace(displayName),
				IsDirect:    isDirect,
				Kind:        modelservicev1.RegistryModelSourceKind_REGISTRY_MODEL_SOURCE_KIND_DISCOVERED,
			}},
		}
		if byVendor[vendorID] == nil {
			byVendor[vendorID] = map[string]*CollectedEntry{}
		}
		current, exists := byVendor[vendorID][modelID]
		if !exists {
			byVendor[vendorID][modelID] = entry
			continue
		}
		// Simple merge: keep primary, append sources
		current.Sources = append(current.Sources, entry.Sources...)
		byVendor[vendorID][modelID] = current
	}
	return SortByVendor(byVendor)
}

// SortByVendor sorts collected entries within each vendor by model ID.
func SortByVendor(byVendor map[string]map[string]*CollectedEntry) map[string][]*CollectedEntry {
	out := make(map[string][]*CollectedEntry, len(byVendor))
	for vendorID, models := range byVendor {
		entries := make([]*CollectedEntry, 0, len(models))
		for _, entry := range models {
			entries = append(entries, entry)
		}
		slices.SortFunc(entries, func(a, b *CollectedEntry) int {
			return strings.Compare(a.GetDefinition().GetModelId(), b.GetDefinition().GetModelId())
		})
		out[vendorID] = entries
	}
	return out
}

// RichModelProjection holds the output of a rich model projection callback.
type RichModelProjection struct {
	Owner         string
	RawModelID    string
	Definition    *modelv1.ModelVersion // may include capabilities, modalities, contextSpec
	Source        *CollectedSource
	Pricing       *modelservicev1.PricingSummary
}

// NormalizeRichModels is like NormalizeHostedModels but accepts a richer projection
// that can set capabilities, modalities, contextSpec, pricing, and source details.
// The callback returns a complete RichModelProjection; the helper resolves the vendor,
// normalizes the model identity, and merges aliases into the definition.
func NormalizeRichModels[T any](
	items []T,
	ctx CollectionContext,
	project func(T, CollectionContext) (RichModelProjection, bool),
) map[string][]*CollectedEntry {
	byVendor := map[string]map[string]*CollectedEntry{}
	for _, item := range items {
		proj, ok := project(item, ctx)
		if !ok {
			continue
		}
		vendorID, ok := ctx.ResolveVendor(proj.Owner)
		if !ok {
			continue
		}
		modelID, aliases, ok := modelidentity.NormalizeExternalModelIdentity(vendorID, proj.RawModelID)
		if !ok {
			continue
		}
		def := proj.Definition
		if def == nil {
			def = &modelv1.ModelVersion{}
		}
		def.VendorId = vendorID
		def.ModelId = modelID
		def.Aliases = aliases
		src := proj.Source
		if src != nil {
			src.VendorId = vendorID
		}
		entry := &CollectedEntry{
			Definition: def,
			Pricing:    proj.Pricing,
		}
		if src != nil {
			entry.Sources = []*CollectedSource{src}
		}
		if byVendor[vendorID] == nil {
			byVendor[vendorID] = map[string]*CollectedEntry{}
		}
		current, exists := byVendor[vendorID][modelID]
		if !exists {
			byVendor[vendorID][modelID] = entry
			continue
		}
		current.Sources = append(current.Sources, entry.Sources...)
		byVendor[vendorID][modelID] = current
	}
	return SortByVendor(byVendor)
}
