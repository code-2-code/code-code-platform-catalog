package sync

import (
	"testing"

	models "code-code.internal/platform-k8s/internal/modelservice/models"
)

func testConfiguredVendorScope(input map[string][]string) configuredVendorScope {
	vendors := make([]configuredVendor, 0, len(input))
	for vendorID, aliases := range input {
		vendors = append(vendors, configuredVendor{
			vendorID: vendorID,
			aliases:  append([]string(nil), aliases...),
		})
	}
	return newConfiguredVendorScope(vendors)
}

func TestConfiguredVendorScopeCanonicalVendorIDUsesConfiguredAliases(t *testing.T) {
	t.Parallel()

	scope := testConfiguredVendorScope(map[string][]string{
		"mistral": {"mistralai", "mistral-ai"},
	})
	if got, ok := scope.canonicalVendorID("mistralai"); !ok || got != "mistral" {
		t.Fatalf("canonicalVendorID(mistralai) = %q, %v; want mistral, true", got, ok)
	}
	if got, ok := scope.canonicalVendorID("mistral-ai"); !ok || got != "mistral" {
		t.Fatalf("canonicalVendorID(mistral-ai) = %q, %v; want mistral, true", got, ok)
	}
}

func TestConfiguredVendorScopeConfiguredVendorIDUsesConfiguredAliases(t *testing.T) {
	t.Parallel()

	scope := testConfiguredVendorScope(map[string][]string{
		"github": {"github-models"},
	})
	if got, ok := scope.configuredVendorID(models.SourceIDGitHubModels); !ok || got != "github" {
		t.Fatalf("configuredVendorID(github-models) = %q, %v; want github, true", got, ok)
	}
}
