package models

import (
	"slices"
	"testing"
)

func TestNormalizeDefinitionSourceIDs(t *testing.T) {
	t.Parallel()

	got := NormalizeDefinitionSourceIDs([]string{
		" NVIDIA-INTEGRATE ",
		"openrouter",
		"openrouter",
		"unknown",
	})

	want := []string{SourceIDNVIDIAIntegrate, SourceIDOpenRouter}
	if !slices.Equal(got, want) {
		t.Fatalf("NormalizeDefinitionSourceIDs() = %#v, want %#v", got, want)
	}
}
