package cerebras

import (
	"testing"

	modelv1 "code-code.internal/go-contract/model/v1"
	"code-code.internal/platform-k8s/internal/modelservice/modelidentity"
	"code-code.internal/platform-k8s/internal/modelservice/models/source"
)

func testCollectionContext(vendorAliases map[string][]string) source.CollectionContext {
	aliases := map[string]string{}
	for vendorID, aliasList := range vendorAliases {
		aliases[vendorID] = vendorID
		for _, alias := range aliasList {
			aliases[modelidentity.NormalizedVendorSlug(alias)] = vendorID
		}
	}
	return source.CollectionContext{
		ResolveVendor: func(raw string) (string, bool) {
			slug := modelidentity.NormalizedVendorSlug(raw)
			canonical, ok := aliases[slug]
			return canonical, ok
		},
		AliasCandidates: func(vendorID string) []string { return nil },
	}
}

func TestNormalizeUsesHuggingFaceIdentityAndPricing(t *testing.T) {
	t.Parallel()

	definitions := Normalize([]Model{{
		ID:            "qwen-3-235b-a22b-instruct-2507",
		OwnedBy:       "Qwen",
		Name:          "Qwen 3 235B Instruct",
		HuggingFaceID: "Qwen/Qwen3-235B-A22B-Instruct-2507",
		Pricing: struct {
			Prompt     string `json:"prompt"`
			Completion string `json:"completion"`
		}{
			Prompt:     "0.0000006",
			Completion: "0.0000012",
		},
	}}, testCollectionContext(map[string][]string{
		"qwen": nil,
	}))

	qwenEntries := definitions["qwen"]
	if len(qwenEntries) == 0 {
		t.Fatal("expected qwen entries")
	}
	// Without KnownModelIDs, suffix-stripping produces the canonical family slug.
	// "qwen3-235b-a22b-instruct-2507" → strips release suffix "-2507" → "qwen3-235b-a22b-instruct"
	if got, want := qwenEntries[0].GetDefinition().GetModelId(), "qwen3-235b-a22b-instruct"; got != want {
		t.Fatalf("qwen model id = %q, want %q", got, want)
	}
	assertAlias(t, qwenEntries[0].GetDefinition(), modelv1.AliasKind_ALIAS_KIND_SNAPSHOT, "qwen3-235b-a22b-instruct-2507")
	if got, want := qwenEntries[0].GetSources()[0].GetPricing().GetInput(), "0.0000006"; got != want {
		t.Fatalf("prompt pricing = %q, want %q", got, want)
	}
	if got, want := qwenEntries[0].GetSources()[0].GetSourceId(), SourceID; got != want {
		t.Fatalf("source id = %q, want %q", got, want)
	}
	if got, want := qwenEntries[0].GetSources()[0].GetSourceModelId(), "qwen-3-235b-a22b-instruct-2507"; got != want {
		t.Fatalf("source model id = %q, want %q", got, want)
	}
}

func TestNormalizeOnlyProducesCanonicalEntries(t *testing.T) {
	t.Parallel()

	definitions := Normalize([]Model{{
		ID:      "gpt-oss-120b",
		OwnedBy: "openai",
		Name:    "GPT OSS 120B",
	}}, testCollectionContext(map[string][]string{
		"openai":   nil,
		"cerebras": nil,
	}))

	if got, want := len(definitions["openai"]), 1; got != want {
		t.Fatalf("len(openai) = %d, want %d", got, want)
	}
	if _, ok := definitions["cerebras"]; ok {
		t.Fatalf("unexpected cerebras aggregate definitions, want canonical only")
	}
}

func assertAlias(t *testing.T, definition *modelv1.ModelVersion, kind modelv1.AliasKind, value string) {
	t.Helper()
	for _, alias := range definition.GetAliases() {
		if alias.GetKind() == kind && alias.GetValue() == value {
			return
		}
	}
	t.Fatalf("expected alias %v/%q, got %v", kind, value, definition.GetAliases())
}
