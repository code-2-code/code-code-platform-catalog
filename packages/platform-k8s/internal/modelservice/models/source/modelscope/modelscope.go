package modelscope

import (
	"strings"

	"code-code.internal/platform-k8s/internal/modelservice/modelidentity"
	"code-code.internal/platform-k8s/internal/modelservice/models/source"
)

// SourceID is the canonical source identifier for ModelScope.
const SourceID = "modelscope"

// Model represents one entry from the ModelScope /v1/models API.
type Model struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
}

// Normalize transforms raw ModelScope API models into grouped CollectedEntry maps.
func Normalize(items []Model, ctx source.CollectionContext) map[string][]*source.CollectedEntry {
	return source.NormalizeHostedModels(SourceID, items, ctx, func(item Model) (string, string, string, bool, bool) {
		vendorID, rawModelID, _, ok := resolveVendorModel(item, ctx)
		if !ok {
			return "", "", "", false, false
		}
		return vendorID, rawModelID, rawModelID, true, true
	})
}

func resolveVendorModel(item Model, ctx source.CollectionContext) (string, string, string, bool) {
	callableModelID := strings.TrimSpace(item.ID)
	owner, rawModelID, ok := strings.Cut(callableModelID, "/")
	if !ok {
		return "", "", "", false
	}
	vendorID, ok := resolveVendorID(strings.TrimSpace(owner), strings.TrimSpace(rawModelID), ctx)
	if !ok {
		return "", "", "", false
	}
	return vendorID, strings.TrimSpace(rawModelID), callableModelID, true
}

func resolveVendorID(owner string, rawModelID string, ctx source.CollectionContext) (string, bool) {
	if vendorID, ok := ctx.ResolveVendor(owner); ok {
		return vendorID, true
	}
	normalizedModelID := modelidentity.NormalizeExternalModelSlug(rawModelID)
	switch {
	case strings.HasPrefix(normalizedModelID, "c4ai-command-r"), strings.HasPrefix(normalizedModelID, "command-r"):
		return ctx.ResolveVendor("cohere")
	case strings.HasPrefix(normalizedModelID, "llama-"):
		return ctx.ResolveVendor("meta")
	case strings.HasPrefix(normalizedModelID, "gui-owl-"):
		return ctx.ResolveVendor("tongyi-lab")
	case strings.HasPrefix(normalizedModelID, "qwen-image-"):
		return ctx.ResolveVendor("qwen")
	default:
		return "", false
	}
}
