package store

import (
	"strings"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
)

// registryModelSourceKindToDBString converts the proto enum to its database
// persistence string.
func registryModelSourceKindToDBString(kind modelservicev1.RegistryModelSourceKind) string {
	if kind == modelservicev1.RegistryModelSourceKind_REGISTRY_MODEL_SOURCE_KIND_DISCOVERED {
		return "discovered"
	}
	return "preset"
}

// registryModelSourceKindFromDBString parses the database string back to the
// proto enum.
func registryModelSourceKindFromDBString(kind string) modelservicev1.RegistryModelSourceKind {
	if strings.TrimSpace(strings.ToLower(kind)) == "discovered" {
		return modelservicev1.RegistryModelSourceKind_REGISTRY_MODEL_SOURCE_KIND_DISCOVERED
	}
	return modelservicev1.RegistryModelSourceKind_REGISTRY_MODEL_SOURCE_KIND_PRESET
}
