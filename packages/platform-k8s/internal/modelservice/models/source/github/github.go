package github

import (
	"slices"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"code-code.internal/platform-k8s/internal/modelservice/models/source"
)

// SourceID is the canonical source identifier for GitHub Models.
const SourceID = "github-models"

// Model represents one entry from the GitHub Models catalog API.
type Model struct {
	ID                        string   `json:"id"`
	Name                      string   `json:"name"`
	SupportedInputModalities  []string `json:"supported_input_modalities"`
	SupportedOutputModalities []string `json:"supported_output_modalities"`
	Capabilities              []string `json:"capabilities"`
	Limits                    struct {
		MaxInputTokens  int64 `json:"max_input_tokens"`
		MaxOutputTokens int64 `json:"max_output_tokens"`
	} `json:"limits"`
}

// Normalize transforms raw GitHub Models API models into grouped CollectedEntry maps.
func Normalize(items []Model, ctx source.CollectionContext) map[string][]*source.CollectedEntry {
	return source.NormalizeRichModels(items, ctx, projectModel)
}

func projectModel(item Model, _ source.CollectionContext) (source.RichModelProjection, bool) {
	callableModelID := strings.TrimSpace(item.ID)
	if shouldSkip(item) || callableModelID == "" {
		return source.RichModelProjection{}, false
	}
	owner, rawModelID, ok := strings.Cut(callableModelID, "/")
	if !ok {
		return source.RichModelProjection{}, false
	}
	return source.RichModelProjection{
		Owner:      owner,
		RawModelID: rawModelID,
		Definition: &modelv1.ModelVersion{
			DisplayName:      strings.TrimSpace(item.Name),
			PrimaryShape:     modelv1.ModelShape_MODEL_SHAPE_CHAT_COMPLETIONS,
			SupportedShapes:  []modelv1.ModelShape{modelv1.ModelShape_MODEL_SHAPE_CHAT_COMPLETIONS},
			Capabilities:     capabilities(item),
			InputModalities:  source.ParseModalities(item.SupportedInputModalities),
			OutputModalities: source.ParseModalities(item.SupportedOutputModalities),
			ContextSpec: &modelv1.ContextSpec{
				MaxContextTokens: item.Limits.MaxInputTokens,
				MaxOutputTokens:  item.Limits.MaxOutputTokens,
			},
		},
		Source: &source.CollectedSource{
			ModelId:       rawModelID,
			SourceId:      SourceID,
			SourceModelId: callableModelID,
			DisplayName:   strings.TrimSpace(item.Name),
			IsDirect:      true,
			Kind:          modelservicev1.RegistryModelSourceKind_REGISTRY_MODEL_SOURCE_KIND_DISCOVERED,
		},
	}, true
}

func shouldSkip(item Model) bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(item.ID)), "preview") || strings.Contains(strings.ToLower(strings.TrimSpace(item.Name)), "preview") {
		return true
	}
	for _, modality := range item.SupportedOutputModalities {
		if strings.TrimSpace(strings.ToLower(modality)) == "embeddings" {
			return true
		}
	}
	return len(source.ParseModalities(item.SupportedOutputModalities)) == 0
}

func capabilities(item Model) []modelv1.ModelCapability {
	set := map[modelv1.ModelCapability]struct{}{}
	for _, capability := range item.Capabilities {
		switch strings.TrimSpace(strings.ToLower(capability)) {
		case "tool-calling":
			set[modelv1.ModelCapability_MODEL_CAPABILITY_TOOL_CALLING] = struct{}{}
		case "structured-output", "structured-outputs":
			set[modelv1.ModelCapability_MODEL_CAPABILITY_STRUCTURED_OUTPUT] = struct{}{}
		}
	}
	for _, modality := range item.SupportedInputModalities {
		if strings.TrimSpace(strings.ToLower(modality)) == "image" {
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
