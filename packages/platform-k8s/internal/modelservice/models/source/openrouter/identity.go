package openrouter

import (
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	"code-code.internal/platform-k8s/internal/modelservice/modelidentity"
	"code-code.internal/platform-k8s/internal/modelservice/models/source"
)

// Identity holds a resolved OpenRouter model identity.
type Identity struct {
	ModelID string
	Aliases []*modelv1.ModelAlias
}

// ResolveIdentity normalizes an OpenRouter sourceID (e.g. "meta/llama-3.1-8b-instruct")
// to a canonical vendor/model identity.
func ResolveIdentity(
	sourceID string,
	canonicalSourceID string,
	ctx source.CollectionContext,
) (string, Identity, bool) {
	sourcePrefix, sourceModelID, ok := ParseVendorModelID(sourceID)
	if !ok {
		return "", Identity{}, false
	}
	vendorID, ok := resolveVendorID(sourcePrefix, ctx)
	if !ok {
		return "", Identity{}, false
	}
	sourceModelID, _ = SplitRouteVariant(sourceModelID)
	if sourceModelID == "" || modelidentity.HasChannelToken(sourceModelID) {
		return "", Identity{}, false
	}
	modelID, aliases, ok := modelidentity.NormalizeExternalModelIdentity(vendorID, sourceModelID)
	if !ok {
		return "", Identity{}, false
	}

	canonicalModelID := ""
	if canonicalPrefix, candidateModelID, ok := ParseVendorModelID(canonicalSourceID); ok {
		if canonicalVendorID, vendorOK := resolveVendorID(canonicalPrefix, ctx); vendorOK && canonicalVendorID == vendorID {
			canonicalModelID = StripRouteVariant(candidateModelID)
		}
	}

	return vendorID, Identity{
		ModelID: modelID,
		Aliases: mergeAliases(aliases, buildAliases(modelID, canonicalModelID)),
	}, true
}

// ParseVendorModelID splits "vendor/model" format.
func ParseVendorModelID(value string) (string, string, bool) {
	prefix, modelID, ok := strings.Cut(strings.TrimSpace(value), "/")
	if !ok {
		return "", "", false
	}
	prefix = strings.TrimSpace(prefix)
	modelID = strings.TrimSpace(modelID)
	if prefix == "" || modelID == "" {
		return "", "", false
	}
	return prefix, modelID, true
}

// StripRouteVariant removes the route variant suffix (after ":").
func StripRouteVariant(modelID string) string {
	base, _ := SplitRouteVariant(modelID)
	return base
}

// SplitRouteVariant splits model ID at the route variant separator.
func SplitRouteVariant(modelID string) (string, string) {
	base, variant, ok := strings.Cut(strings.TrimSpace(modelID), ":")
	if !ok {
		return strings.TrimSpace(modelID), ""
	}
	return strings.TrimSpace(base), strings.TrimSpace(variant)
}

func resolveVendorID(prefix string, ctx source.CollectionContext) (string, bool) {
	if vendorID, ok := ctx.ResolveVendor(prefix); ok {
		return vendorID, true
	}
	normalizedPrefix := modelidentity.NormalizedVendorSlug(prefix)
	if normalizedPrefix != SourceID {
		return "", false
	}
	// OpenRouter self-publishes under its own prefix
	if vendorID, ok := ctx.ResolveVendor(normalizedPrefix); ok && vendorID == SourceID {
		return vendorID, true
	}
	return "", false
}

func buildAliases(modelID string, canonicalModelID string) []*modelv1.ModelAlias {
	if strings.TrimSpace(canonicalModelID) == "" || strings.TrimSpace(canonicalModelID) == strings.TrimSpace(modelID) {
		return nil
	}
	kind := modelv1.AliasKind_ALIAS_KIND_STABLE
	if modelidentity.HasSnapshotReleaseSuffix(canonicalModelID) {
		kind = modelv1.AliasKind_ALIAS_KIND_SNAPSHOT
	}
	return []*modelv1.ModelAlias{{
		Kind:  kind,
		Value: canonicalModelID,
	}}
}

func mergeAliases(left []*modelv1.ModelAlias, right []*modelv1.ModelAlias) []*modelv1.ModelAlias {
	if len(right) == 0 {
		return left
	}
	seen := map[string]struct{}{}
	for _, alias := range left {
		seen[strings.TrimSpace(alias.GetValue())] = struct{}{}
	}
	out := append([]*modelv1.ModelAlias(nil), left...)
	for _, alias := range right {
		value := strings.TrimSpace(alias.GetValue())
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, alias)
	}
	return out
}
