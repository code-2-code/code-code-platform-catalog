package models

import (
	"slices"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
)

func MergeCollectedEntries(primary *modelservicev1.CollectedModelEntry, fallback *modelservicev1.CollectedModelEntry) *modelservicev1.CollectedModelEntry {
	if compareCollectedEntryAuthority(fallback, primary) < 0 {
		primary, fallback = fallback, primary
	}
	return &modelservicev1.CollectedModelEntry{
		Definition: mergeDefinitionMetadata(primary.GetDefinition(), fallback.GetDefinition()),
		Badges:     MergeDefinitionSourceBadges(primary.GetBadges(), fallback.GetBadges()),
		Pricing:    MergePricingSummary(primary.GetPricing(), fallback.GetPricing()),
		Sources:    MergeCollectedSourceSlices(primary.GetSources(), fallback.GetSources()),
	}
}

func mergeDefinitionMetadata(primary *modelv1.ModelVersion, fallback *modelv1.ModelVersion) *modelv1.ModelVersion {
	if primary == nil {
		return fallback
	}
	if fallback == nil {
		return primary
	}
	// Clone primary so we do not mutate the input.
	merged := &modelv1.ModelVersion{
		VendorId:         primary.GetVendorId(),
		ModelId:          primary.GetModelId(),
		DisplayName:      primary.GetDisplayName(),
		PrimaryShape:     primary.GetPrimaryShape(),
		SupportedShapes:  append([]modelv1.ModelShape(nil), primary.GetSupportedShapes()...),
		Capabilities:     append([]modelv1.ModelCapability(nil), primary.GetCapabilities()...),
		InputModalities:  append([]modelv1.Modality(nil), primary.GetInputModalities()...),
		OutputModalities: append([]modelv1.Modality(nil), primary.GetOutputModalities()...),
	}
	if primary.GetContextSpec() != nil {
		merged.ContextSpec = &modelv1.ContextSpec{
			MaxContextTokens: primary.GetContextSpec().GetMaxContextTokens(),
			MaxOutputTokens:  primary.GetContextSpec().GetMaxOutputTokens(),
		}
	}

	// Fill gaps from fallback.
	if strings.TrimSpace(merged.GetDisplayName()) == "" && strings.TrimSpace(fallback.GetDisplayName()) != "" {
		merged.DisplayName = fallback.GetDisplayName()
	}
	if merged.GetContextSpec().GetMaxContextTokens() == 0 && fallback.GetContextSpec().GetMaxContextTokens() != 0 {
		if merged.ContextSpec == nil {
			merged.ContextSpec = &modelv1.ContextSpec{}
		}
		merged.ContextSpec.MaxContextTokens = fallback.GetContextSpec().GetMaxContextTokens()
	}
	if merged.GetContextSpec().GetMaxOutputTokens() == 0 && fallback.GetContextSpec().GetMaxOutputTokens() != 0 {
		if merged.ContextSpec == nil {
			merged.ContextSpec = &modelv1.ContextSpec{}
		}
		merged.ContextSpec.MaxOutputTokens = fallback.GetContextSpec().GetMaxOutputTokens()
	}
	if merged.GetPrimaryShape() == modelv1.ModelShape_MODEL_SHAPE_UNSPECIFIED && fallback.GetPrimaryShape() != modelv1.ModelShape_MODEL_SHAPE_UNSPECIFIED {
		merged.PrimaryShape = fallback.GetPrimaryShape()
	}
	if len(merged.GetSupportedShapes()) == 0 && len(fallback.GetSupportedShapes()) > 0 {
		merged.SupportedShapes = append([]modelv1.ModelShape(nil), fallback.GetSupportedShapes()...)
	}
	if len(merged.GetCapabilities()) == 0 && len(fallback.GetCapabilities()) > 0 {
		merged.Capabilities = append([]modelv1.ModelCapability(nil), fallback.GetCapabilities()...)
	}
	if len(merged.GetInputModalities()) == 0 && len(fallback.GetInputModalities()) > 0 {
		merged.InputModalities = append([]modelv1.Modality(nil), fallback.GetInputModalities()...)
	}
	if len(merged.GetOutputModalities()) == 0 && len(fallback.GetOutputModalities()) > 0 {
		merged.OutputModalities = append([]modelv1.Modality(nil), fallback.GetOutputModalities()...)
	}
	merged.Aliases = mergeDefinitionAliases(primary.GetAliases(), fallback.GetAliases())
	return merged
}

func mergeDefinitionAliases(left []*modelv1.ModelAlias, right []*modelv1.ModelAlias) []*modelv1.ModelAlias {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	out := make([]*modelv1.ModelAlias, 0, len(left)+len(right))
	seen := map[string]struct{}{}
	appendAlias := func(alias *modelv1.ModelAlias) {
		if alias == nil {
			return
		}
		value := strings.TrimSpace(alias.GetValue())
		if value == "" {
			return
		}
		key := alias.GetKind().String() + "\x00" + value
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, &modelv1.ModelAlias{
			Kind:  alias.GetKind(),
			Value: value,
		})
	}
	for _, alias := range left {
		appendAlias(alias)
	}
	for _, alias := range right {
		appendAlias(alias)
	}
	slices.SortFunc(out, func(left *modelv1.ModelAlias, right *modelv1.ModelAlias) int {
		if left.GetKind() != right.GetKind() {
			return int(left.GetKind()) - int(right.GetKind())
		}
		return strings.Compare(left.GetValue(), right.GetValue())
	})
	return out
}
