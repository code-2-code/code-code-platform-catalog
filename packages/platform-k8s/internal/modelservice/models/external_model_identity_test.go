package models

import (
	"testing"

	modelv1 "code-code.internal/go-contract/model/v1"
	"code-code.internal/platform-k8s/internal/modelservice/modelidentity"
)

func TestNormalizeExternalModelIdentityNormalizesCalendarReleaseSuffixes(t *testing.T) {
	t.Parallel()

	modelID, aliases, ok := modelidentity.NormalizeExternalModelIdentity("cohere", "command-r-08-2024")
	if !ok {
		t.Fatal("NormalizeExternalModelIdentity() ok = false, want true")
	}
	if got, want := modelID, "command-r"; got != want {
		t.Fatalf("model id = %q, want %q", got, want)
	}
	if got, want := len(aliases), 1; got != want {
		t.Fatalf("alias count = %d, want %d", got, want)
	}
	if got, want := aliases[0].GetKind(), modelv1.AliasKind_ALIAS_KIND_SNAPSHOT; got != want {
		t.Fatalf("alias kind = %v, want %v", got, want)
	}
	if got, want := aliases[0].GetValue(), "command-r-08-2024"; got != want {
		t.Fatalf("alias value = %q, want %q", got, want)
	}
}

