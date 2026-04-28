package sync

import "testing"

func TestShouldKeepManagedDefinitionKeepsConfiguredAggregateVendorWithoutFreshCollection(t *testing.T) {
	t.Parallel()

	if !shouldKeepManagedDefinition("openrouter", &collectedDefinitionsSnapshot{
		managedVendorIDs:   map[string]struct{}{"openrouter": {}},
		collectedVendorIDs: map[string]struct{}{},
	}) {
		t.Fatal("expected configured aggregate vendor to be kept without fresh collection")
	}
}

func TestShouldKeepManagedDefinitionDeletesUnconfiguredVendor(t *testing.T) {
	t.Parallel()

	if shouldKeepManagedDefinition("openrouter", &collectedDefinitionsSnapshot{
		managedVendorIDs:   map[string]struct{}{"openai": {}},
		collectedVendorIDs: map[string]struct{}{},
	}) {
		t.Fatal("expected unconfigured vendor to be deleted")
	}
}
