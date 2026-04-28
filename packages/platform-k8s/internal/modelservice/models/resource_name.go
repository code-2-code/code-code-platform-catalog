package models

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	modelDefinitionNamePrefix = "model-"
	modelDefinitionHashLength = 12
	maxKubernetesNameLength   = 253
)

// ResourceNameForVendorModelID returns one stable Kubernetes-safe resource
// name for one canonical vendor model identity.
func ResourceNameForVendorModelID(vendorID string, modelID string) string {
	canonical := strings.TrimSpace(vendorID) + "-" + strings.TrimSpace(modelID)
	if validation.IsDNS1123Subdomain(canonical) == nil {
		return canonical
	}

	slug := buildModelIDSlug(canonical)
	hash := stableModelIDHash(canonical)
	maxSlugLength := maxKubernetesNameLength - len(modelDefinitionNamePrefix) - len(hash) - 1
	if maxSlugLength < 1 {
		maxSlugLength = 1
	}
	if len(slug) > maxSlugLength {
		slug = strings.Trim(slug[:maxSlugLength], "-.")
	}
	if slug == "" {
		slug = "model"
	}
	return modelDefinitionNamePrefix + slug + "-" + hash
}

// ResourceNameForModelID preserves the historical helper shape for callers
// that still operate on a model id without vendor context.
func ResourceNameForModelID(modelID string) string {
	return ResourceNameForVendorModelID("", modelID)
}

func buildModelIDSlug(modelID string) string {
	var b strings.Builder
	lastSeparator := false
	for _, r := range strings.ToLower(modelID) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastSeparator = false
		case r == '-' || r == '.':
			if lastSeparator {
				continue
			}
			b.WriteRune(r)
			lastSeparator = true
		default:
			if lastSeparator {
				continue
			}
			b.WriteByte('-')
			lastSeparator = true
		}
	}
	return strings.Trim(b.String(), "-.")
}

func stableModelIDHash(modelID string) string {
	sum := sha256.Sum256([]byte(modelID))
	return hex.EncodeToString(sum[:])[:modelDefinitionHashLength]
}
