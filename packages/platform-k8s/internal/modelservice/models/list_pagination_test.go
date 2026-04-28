package models

import "testing"

func TestDefinitionListContinueTokenRoundTrip(t *testing.T) {
	t.Parallel()

	encoded := encodeDefinitionListContinueToken("openai", "gpt-5")
	if encoded == "" {
		t.Fatal("encodeDefinitionListContinueToken() returned empty token")
	}
	vendorID, modelID, ok := decodeDefinitionListContinueToken(encoded)
	if !ok {
		t.Fatal("decodeDefinitionListContinueToken() failed")
	}
	if vendorID != "openai" || modelID != "gpt-5" {
		t.Fatalf("decoded token = %q/%q, want openai/gpt-5", vendorID, modelID)
	}
}

func TestDefinitionListContinueTokenRejectsInvalid(t *testing.T) {
	t.Parallel()

	for _, token := range []string{"", "raw", "!!not-base64!!"} {
		if _, _, ok := decodeDefinitionListContinueToken(token); ok {
			t.Fatalf("decodeDefinitionListContinueToken(%q) unexpectedly succeeded", token)
		}
	}
}

func TestDefinitionListPageTokenRoundTripWithContinue(t *testing.T) {
	t.Parallel()

	continueToken := encodeDefinitionListContinueToken("anthropic", "claude-sonnet-4")
	pageToken := encodeDefinitionListPageToken(continueToken, 200)
	decodedContinue, decodedOffset := decodeDefinitionListPageToken(pageToken)
	if decodedContinue != continueToken {
		t.Fatalf("decoded continue token mismatch: got %q want %q", decodedContinue, continueToken)
	}
	if decodedOffset != 200 {
		t.Fatalf("decoded offset = %d, want 200", decodedOffset)
	}
}

func BenchmarkDefinitionListContinueTokenRoundTrip(b *testing.B) {
	for i := 0; i < b.N; i++ {
		token := encodeDefinitionListContinueToken("openai", "gpt-5")
		if _, _, ok := decodeDefinitionListContinueToken(token); !ok {
			b.Fatal("round-trip decode failed")
		}
	}
}
