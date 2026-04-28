package openrouter

import (
	"slices"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"code-code.internal/platform-k8s/internal/modelservice/models/source"
	"github.com/OpenRouterTeam/go-sdk/models/components"
)

type definitionCandidate struct {
	sourceID          string
	canonicalSourceID string
	definition        *modelv1.ModelVersion
	sources           []*source.CollectedSource
}

// Normalize transforms raw OpenRouter API models into grouped CollectedEntry maps.
func Normalize(
	items []components.Model,
	ctx source.CollectionContext,
) map[string][]*source.CollectedEntry {
	byVendor := map[string]map[string]definitionCandidate{}
	for _, item := range items {
		vendorID, candidate, ok := normalizeModel(item, ctx)
		if !ok {
			continue
		}
		if byVendor[vendorID] == nil {
			byVendor[vendorID] = map[string]definitionCandidate{}
		}
		modelID := candidate.definition.GetModelId()
		current, exists := byVendor[vendorID][modelID]
		if !exists {
			byVendor[vendorID][modelID] = candidate
			continue
		}
		byVendor[vendorID][modelID] = mergeDefinitions(current, candidate)
	}

	out := make(map[string][]*source.CollectedEntry, len(byVendor))
	for vendorID, models := range byVendor {
		entries := make([]*source.CollectedEntry, 0, len(models))
		for _, candidate := range models {
			entries = append(entries, &source.CollectedEntry{
				Definition: candidate.definition,
				Sources:    append([]*source.CollectedSource(nil), candidate.sources...),
			})
		}
		slices.SortFunc(entries, func(a, b *source.CollectedEntry) int {
			return strings.Compare(a.GetDefinition().GetModelId(), b.GetDefinition().GetModelId())
		})
		out[vendorID] = entries
	}
	return out
}

func normalizeModel(
	item components.Model,
	ctx source.CollectionContext,
) (string, definitionCandidate, bool) {
	sourceID := strings.TrimSpace(item.ID)
	canonicalSourceID := strings.TrimSpace(item.CanonicalSlug)
	if canonicalSourceID == "" {
		canonicalSourceID = sourceID
	}
	_, sourceModelID, ok := ParseVendorModelID(sourceID)
	if !ok {
		return "", definitionCandidate{}, false
	}

	vendorID, identity, ok := ResolveIdentity(sourceID, canonicalSourceID, ctx)
	if !ok {
		return "", definitionCandidate{}, false
	}
	var maxOutputTokens int64
	if item.TopProvider.MaxCompletionTokens != nil {
		maxOutputTokens = *item.TopProvider.MaxCompletionTokens
	}
	definition := &modelv1.ModelVersion{
		ModelId:          identity.ModelID,
		DisplayName:      strings.TrimSpace(item.Name),
		VendorId:         vendorID,
		Aliases:          identity.Aliases,
		Capabilities:     parseCapabilities(item.SupportedParameters, item.Architecture.InputModalities),
		InputModalities:  parseInputModalities(item.Architecture.InputModalities),
		OutputModalities: parseOutputModalities(item.Architecture.OutputModalities),
		ContextSpec: &modelv1.ContextSpec{
			MaxContextTokens: item.ContextLength,
			MaxOutputTokens:  maxOutputTokens,
		},
	}
	src := &source.CollectedSource{
		VendorId:      vendorID,
		ModelId:       sourceModelID,
		SourceId:      SourceID,
		SourceModelId: sourceID,
		DisplayName:   strings.TrimSpace(item.Name),
		IsDirect:      false,
		Kind:          modelservicev1.RegistryModelSourceKind_REGISTRY_MODEL_SOURCE_KIND_DISCOVERED,
	}
	return vendorID, definitionCandidate{
		sourceID:          sourceID,
		canonicalSourceID: canonicalSourceID,
		definition:        definition,
		sources:           []*source.CollectedSource{src},
	}, true
}

func mergeDefinitions(current definitionCandidate, candidate definitionCandidate) definitionCandidate {
	preferred := current
	fallback := candidate
	if candidatePriority(candidate) < candidatePriority(current) ||
		(candidatePriority(candidate) == candidatePriority(current) && strings.Compare(candidate.sourceID, current.sourceID) < 0) {
		preferred = candidate
		fallback = current
	}
	definition := preferred.definition
	if definition == nil {
		return fallback
	}
	if fallback.definition == nil {
		return preferred
	}
	if strings.TrimSpace(definition.GetDisplayName()) == "" && strings.TrimSpace(fallback.definition.GetDisplayName()) != "" {
		definition.DisplayName = fallback.definition.GetDisplayName()
	}
	if definition.GetContextSpec().GetMaxContextTokens() == 0 && fallback.definition.GetContextSpec().GetMaxContextTokens() != 0 {
		if definition.ContextSpec == nil {
			definition.ContextSpec = &modelv1.ContextSpec{}
		}
		definition.ContextSpec.MaxContextTokens = fallback.definition.GetContextSpec().GetMaxContextTokens()
	}
	if definition.GetContextSpec().GetMaxOutputTokens() == 0 && fallback.definition.GetContextSpec().GetMaxOutputTokens() != 0 {
		if definition.ContextSpec == nil {
			definition.ContextSpec = &modelv1.ContextSpec{}
		}
		definition.ContextSpec.MaxOutputTokens = fallback.definition.GetContextSpec().GetMaxOutputTokens()
	}
	definition.Aliases = mergeAliases(definition.GetAliases(), fallback.definition.GetAliases())
	if len(definition.GetCapabilities()) == 0 && len(fallback.definition.GetCapabilities()) > 0 {
		definition.Capabilities = append([]modelv1.ModelCapability(nil), fallback.definition.GetCapabilities()...)
	}
	if len(definition.GetInputModalities()) == 0 && len(fallback.definition.GetInputModalities()) > 0 {
		definition.InputModalities = append([]modelv1.Modality(nil), fallback.definition.GetInputModalities()...)
	}
	if len(definition.GetOutputModalities()) == 0 && len(fallback.definition.GetOutputModalities()) > 0 {
		definition.OutputModalities = append([]modelv1.Modality(nil), fallback.definition.GetOutputModalities()...)
	}
	preferred.definition = definition
	preferred.sources = mergeSources(preferred.sources, fallback.sources)
	return preferred
}

func candidatePriority(c definitionCandidate) int {
	switch {
	case c.sourceID == c.canonicalSourceID:
		return 0
	case !strings.Contains(c.sourceID, ":"):
		return 1
	default:
		return 2
	}
}

func mergeSources(primary []*source.CollectedSource, fallback []*source.CollectedSource) []*source.CollectedSource {
	seen := map[string]struct{}{}
	for _, s := range primary {
		seen[s.GetSourceModelId()] = struct{}{}
	}
	out := append([]*source.CollectedSource(nil), primary...)
	for _, s := range fallback {
		if _, ok := seen[s.GetSourceModelId()]; ok {
			continue
		}
		seen[s.GetSourceModelId()] = struct{}{}
		out = append(out, s)
	}
	return out
}

func parseCapabilities(parameters []components.Parameter, inputModalities []components.InputModality) []modelv1.ModelCapability {
	set := map[modelv1.ModelCapability]struct{}{}
	for _, parameter := range parameters {
		switch strings.TrimSpace(string(parameter)) {
		case "tools", "tool_choice":
			set[modelv1.ModelCapability_MODEL_CAPABILITY_TOOL_CALLING] = struct{}{}
		case "structured_outputs", "response_format":
			set[modelv1.ModelCapability_MODEL_CAPABILITY_STRUCTURED_OUTPUT] = struct{}{}
		}
	}
	for _, modality := range inputModalities {
		if strings.TrimSpace(string(modality)) == "image" {
			set[modelv1.ModelCapability_MODEL_CAPABILITY_IMAGE_INPUT] = struct{}{}
		}
	}
	out := make([]modelv1.ModelCapability, 0, len(set))
	for capability := range set {
		out = append(out, capability)
	}
	slices.Sort(out)
	return out
}

func parseInputModalities(values []components.InputModality) []modelv1.Modality {
	raw := make([]string, len(values))
	for i, v := range values {
		raw[i] = string(v)
	}
	return source.ParseModalities(raw)
}

func parseOutputModalities(values []components.OutputModality) []modelv1.Modality {
	raw := make([]string, len(values))
	for i, v := range values {
		raw[i] = string(v)
	}
	return source.ParseModalities(raw)
}

