package models

import (
	"testing"

	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
)

func TestMergeCollectedEntriesPrefersHigherAuthorityMetadata(t *testing.T) {
	t.Parallel()

	merged := MergeCollectedEntries(
		&modelservicev1.CollectedModelEntry{
			Definition: &modelv1.ModelVersion{
				ModelId:     "gpt-5",
				DisplayName: "GPT 5",
				VendorId:    "openai",
				ContextSpec: &modelv1.ContextSpec{MaxContextTokens: 128000},
			},
			Sources: []*modelservicev1.CollectedModelSource{{VendorId: "openai", ModelId: "gpt-5", SourceId: SourceIDOpenRouter}},
		},
		&modelservicev1.CollectedModelEntry{
			Definition: &modelv1.ModelVersion{
				ModelId:     "gpt-5",
				DisplayName: "OpenAI GPT-5",
				VendorId:    "openai",
				ContextSpec: &modelv1.ContextSpec{MaxContextTokens: 200000, MaxOutputTokens: 100000},
			},
			Sources: []*modelservicev1.CollectedModelSource{{VendorId: "openai", ModelId: "gpt-5", SourceId: SourceIDGitHubModels, IsDirect: true}},
		},
	)

	if got, want := merged.GetDefinition().GetDisplayName(), "OpenAI GPT-5"; got != want {
		t.Fatalf("display_name = %q, want %q", got, want)
	}
	if got, want := merged.GetDefinition().GetContextSpec().GetMaxContextTokens(), int64(200000); got != want {
		t.Fatalf("context_window_tokens = %d, want %d", got, want)
	}
	if got, want := merged.GetDefinition().GetContextSpec().GetMaxOutputTokens(), int64(100000); got != want {
		t.Fatalf("max_output_tokens = %d, want %d", got, want)
	}
	if got, want := merged.GetSources()[0].GetSourceId(), SourceIDGitHubModels; got != want {
		t.Fatalf("primary source = %q, want %q", got, want)
	}
}
