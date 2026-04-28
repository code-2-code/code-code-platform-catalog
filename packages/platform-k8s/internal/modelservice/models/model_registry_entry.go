package models

import (
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"google.golang.org/protobuf/proto"
)

type StoredDefinition struct {
	Identity   SurfaceIdentity
	Definition *modelv1.ModelVersion
}

func NewModelRegistryEntry(
	entry *modelservicev1.CollectedModelEntry,
) *modelservicev1.ModelRegistryEntry {
	if entry == nil {
		return nil
	}
	return &modelservicev1.ModelRegistryEntry{
		Definition: CloneModelDefinition(entry.GetDefinition()),
		Badges:     NormalizeDefinitionSourceBadges(entry.GetBadges()),
		Pricing:    NormalizePricingSummary(entry.GetPricing()),
		Sources:    protoRegistrySourcesFromCollectedSources(entry.GetSources()),
	}
}

func protoRegistrySourcesFromCollectedSources(sources []*modelservicev1.CollectedModelSource) []*modelservicev1.RegistryModelSource {
	sources = NormalizeCollectedSources(sources)
	if len(sources) == 0 {
		return nil
	}
	out := make([]*modelservicev1.RegistryModelSource, 0, len(sources))
	for _, source := range sources {
		source = NormalizeCollectedSource(source)
		if source == nil {
			continue
		}
		definition := specSourceDefinition(source)
		out = append(out, NormalizeRegistryModelSource(&modelservicev1.RegistryModelSource{
			SourceId:      source.GetSourceId(),
			Kind:          source.GetKind(),
			IsDirect:      source.GetIsDirect(),
			SourceModelId: specSourceModelID(source, definition),
			Definition:    definition,
			Badges:        NormalizeDefinitionSourceBadges(source.GetBadges()),
			Pricing:       NormalizePricingSummary(source.GetPricing()),
		}))
	}
	return out
}

func NormalizeRegistryModelSource(source *modelservicev1.RegistryModelSource) *modelservicev1.RegistryModelSource {
	if source == nil {
		return nil
	}
	out := proto.Clone(source).(*modelservicev1.RegistryModelSource)
	out.SourceId = NormalizedVendorSlug(out.GetSourceId())

	out.SourceModelId = strings.TrimSpace(out.GetSourceModelId())
	out.Badges = NormalizeDefinitionSourceBadges(out.GetBadges())
	if out.Pricing != nil {
		out.Pricing = NormalizePricingSummary(out.GetPricing())
	}
	if out.Definition != nil {
		out.Definition = CloneModelDefinition(out.GetDefinition())
	}
	return out
}

func specSourceDefinition(source *modelservicev1.CollectedModelSource) *modelv1.ModelVersion {
	if source.GetDefinition() != nil {
		return CloneModelDefinition(source.GetDefinition())
	}
	vendorID := NormalizedVendorSlug(source.GetVendorId())
	modelID := strings.TrimSpace(source.GetModelId())
	if vendorID == "" || modelID == "" {
		return nil
	}
	definition := &modelv1.ModelVersion{
		VendorId: vendorID,
		ModelId:  modelID,
	}
	if displayName := strings.TrimSpace(source.GetDisplayName()); displayName != "" {
		definition.DisplayName = displayName
	}
	return definition
}

func specSourceModelID(source *modelservicev1.CollectedModelSource, definition *modelv1.ModelVersion) string {
	if sourceModelID := strings.TrimSpace(source.GetSourceModelId()); sourceModelID != "" {
		return sourceModelID
	}
	if definition != nil {
		return strings.TrimSpace(definition.GetModelId())
	}
	return strings.TrimSpace(source.GetModelId())
}
