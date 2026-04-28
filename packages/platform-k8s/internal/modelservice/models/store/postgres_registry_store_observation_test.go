package store

import (
	"testing"

	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
)

func TestNormalizeRegistryObservationsDeduplicatesBySourceCallableIdentity(t *testing.T) {
	t.Parallel()

	input := []*modelservicev1.RegistryModelSource{
		{
			SourceId:      "github-models",
			IsDirect:      true,
			SourceModelId: "deepseek-r1",
		},
		{
			SourceId:      "github-models",
			IsDirect:      true,
			SourceModelId: "deepseek-r1-alt",
		},
		{
			SourceId:      "github-models",
			IsDirect:      false,
			SourceModelId: "deepseek/deepseek-r1",
		},
		{
			SourceId:      "openrouter",
			IsDirect:      false,
			SourceModelId: "deepseek/deepseek-r1:free",
		},
	}

	normalized := normalizeRegistryObservations(input)
	if got, want := len(normalized), 4; got != want {
		t.Fatalf("len(normalized) = %d, want %d", got, want)
	}
	if got, want := normalized[0].GetSourceModelId(), "deepseek-r1"; got != want {
		t.Fatalf("first github source_model_id = %q, want %q", got, want)
	}
	if got, want := normalized[1].GetSourceModelId(), "deepseek-r1-alt"; got != want {
		t.Fatalf("second github source_model_id = %q, want %q", got, want)
	}
	if got, want := normalized[2].GetIsDirect(), false; got != want {
		t.Fatalf("third github is_direct = %v, want %v", got, want)
	}
	if got, want := normalized[3].GetSourceId(), "openrouter"; got != want {
		t.Fatalf("fourth source_id = %q, want %q", got, want)
	}
}

func TestNormalizeModelDefinitionAliasesDeduplicatesByKindAndValue(t *testing.T) {
	t.Parallel()

	normalized := normalizeModelDefinitionAliases(&modelv1.ModelVersion{
		Aliases: []*modelv1.ModelAlias{
			{Kind: modelv1.AliasKind_ALIAS_KIND_SNAPSHOT, Value: "gpt-5-20250201"},
			{Kind: modelv1.AliasKind_ALIAS_KIND_SNAPSHOT, Value: " gpt-5-20250201 "},
			{Kind: modelv1.AliasKind_ALIAS_KIND_STABLE, Value: "gpt-5-20250201"},
			{Kind: modelv1.AliasKind_ALIAS_KIND_STABLE, Value: " "},
			nil,
		},
	})
	if got, want := len(normalized), 2; got != want {
		t.Fatalf("len(normalized) = %d, want %d", got, want)
	}
	if got, want := normalized[0].GetKind(), modelv1.AliasKind_ALIAS_KIND_SNAPSHOT; got != want {
		t.Fatalf("first alias kind = %v, want %v", got, want)
	}
	if got, want := normalized[1].GetKind(), modelv1.AliasKind_ALIAS_KIND_STABLE; got != want {
		t.Fatalf("second alias kind = %v, want %v", got, want)
	}
	if got, want := normalized[0].GetValue(), "gpt-5-20250201"; got != want {
		t.Fatalf("first alias value = %q, want %q", got, want)
	}
}
