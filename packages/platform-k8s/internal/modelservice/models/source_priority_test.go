package models

import (
	"testing"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
)

func TestNormalizeCollectedSourcesOrdersByAuthorityPriority(t *testing.T) {
	t.Parallel()

	sources := NormalizeCollectedSources([]*modelservicev1.CollectedModelSource{
		{VendorId: "openai", ModelId: "gpt-5", SourceId: SourceIDOpenRouter},
		{VendorId: "openai", ModelId: "gpt-5", SourceId: SourceIDHuggingFaceHub},
		{VendorId: "openai", ModelId: "gpt-5", SourceId: SourceIDGitHubModels, IsDirect: true},
	})

	if got, want := sources[0].GetSourceId(), SourceIDGitHubModels; got != want {
		t.Fatalf("sources[0] = %q, want %q", got, want)
	}
	if got, want := sources[1].GetSourceId(), SourceIDHuggingFaceHub; got != want {
		t.Fatalf("sources[1] = %q, want %q", got, want)
	}
	if got, want := sources[2].GetSourceId(), SourceIDOpenRouter; got != want {
		t.Fatalf("sources[2] = %q, want %q", got, want)
	}
}
