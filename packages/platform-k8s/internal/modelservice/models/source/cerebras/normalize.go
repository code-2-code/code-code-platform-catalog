package cerebras

import (
	"slices"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"code-code.internal/platform-k8s/internal/modelservice/models/source"
)

// SourceID is the canonical source identifier for Cerebras.
const SourceID = "cerebras"

// Normalize transforms raw Cerebras API models into grouped CollectedEntry maps
// keyed by canonical vendor ID.
func Normalize(
	items []Model,
	ctx source.CollectionContext,
) map[string][]*source.CollectedEntry {
	return source.NormalizeRichModels(items, ctx, projectModel)
}

func projectModel(item Model, ctx source.CollectionContext) (source.RichModelProjection, bool) {
	owner := strings.TrimSpace(item.OwnedBy)
	rawModelID := strings.TrimSpace(item.ID)
	callableModelID := rawModelID
	if hfOwner, hfModelID, ok := strings.Cut(strings.TrimSpace(item.HuggingFaceID), "/"); ok {
		owner = hfOwner
		rawModelID = hfModelID
	}
	if owner == "" || rawModelID == "" {
		return source.RichModelProjection{}, false
	}
	pricing := normalizePricing(item)
	return source.RichModelProjection{
		Owner:      owner,
		RawModelID: rawModelID,
		Definition: &modelv1.ModelVersion{
			DisplayName:      strings.TrimSpace(item.Name),
			Capabilities:     capabilities(item),
			InputModalities:  inputModalities(item),
			OutputModalities: []modelv1.Modality{modelv1.Modality_MODALITY_TEXT},
			ContextSpec: &modelv1.ContextSpec{
				MaxContextTokens: item.Limits.MaxContextLength,
				MaxOutputTokens:  item.Limits.MaxCompletionTokens,
			},
		},
		Source: &source.CollectedSource{
			ModelId:       rawModelID,
			SourceId:      SourceID,
			SourceModelId: callableModelID,
			DisplayName:   strings.TrimSpace(item.Name),
			IsDirect:      true,
			Kind:          modelservicev1.RegistryModelSourceKind_REGISTRY_MODEL_SOURCE_KIND_DISCOVERED,
			Pricing:       pricing,
		},
		Pricing: pricing,
	}, true
}

func normalizePricing(item Model) *modelservicev1.PricingSummary {
	if item.Pricing.Prompt == "" && item.Pricing.Completion == "" {
		return nil
	}
	return &modelservicev1.PricingSummary{
		Input:  item.Pricing.Prompt,
		Output: item.Pricing.Completion,
	}
}

func capabilities(item Model) []modelv1.ModelCapability {
	out := make([]modelv1.ModelCapability, 0, 4)
	if item.Capabilities.FunctionCalling {
		out = append(out, modelv1.ModelCapability_MODEL_CAPABILITY_TOOL_CALLING)
	}
	if item.Capabilities.StructuredOutputs {
		out = append(out, modelv1.ModelCapability_MODEL_CAPABILITY_STRUCTURED_OUTPUT)
	}
	if item.Capabilities.Vision {
		out = append(out, modelv1.ModelCapability_MODEL_CAPABILITY_IMAGE_INPUT)
	}
	slices.Sort(out)
	return out
}

func inputModalities(item Model) []modelv1.Modality {
	if item.Capabilities.Vision {
		return []modelv1.Modality{
			modelv1.Modality_MODALITY_IMAGE,
			modelv1.Modality_MODALITY_TEXT,
		}
	}
	return []modelv1.Modality{modelv1.Modality_MODALITY_TEXT}
}
