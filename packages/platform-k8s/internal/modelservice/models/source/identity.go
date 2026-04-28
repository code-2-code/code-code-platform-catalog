package source

import (
	modelv1 "code-code.internal/go-contract/model/v1"
	"code-code.internal/platform-k8s/internal/modelservice/modelidentity"
)

// NormalizedVendorSlug normalizes a vendor identifier to a lowercase, hyphen-separated slug.
func NormalizedVendorSlug(raw string) string {
	return modelidentity.NormalizedVendorSlug(raw)
}

// NormalizeExternalModelIdentity resolves a raw external model ID to a canonical
// model ID with optional aliases.
func NormalizeExternalModelIdentity(vendorID string, rawModelID string, knownModelIDs map[string]struct{}) (string, []*modelv1.ModelAlias, bool) {
	return modelidentity.NormalizeExternalModelIdentity(vendorID, rawModelID)
}

// NormalizeExternalModelSlug normalizes a raw model slug to lowercase, trimmed form.
func NormalizeExternalModelSlug(raw string) string {
	return modelidentity.NormalizeExternalModelSlug(raw)
}

// HasChannelToken returns true if the model ID contains channel tokens like
// "preview", "latest", or "experimental" that indicate non-canonical entries.
func HasChannelToken(value string) bool {
	return modelidentity.HasChannelToken(value)
}

// HasModelToken checks if a model ID contains any of the specified tokens.
func HasModelToken(value string, tokens ...string) bool {
	return modelidentity.HasModelToken(value, tokens...)
}

// BuildExternalAliases creates model aliases when the canonical model ID
// differs from the raw source model ID.
func BuildExternalAliases(modelID string, raw string) []*modelv1.ModelAlias {
	return modelidentity.BuildExternalAliases(modelID, raw)
}

// HasSnapshotReleaseSuffix returns true if the model ID has a snapshot-style release suffix.
func HasSnapshotReleaseSuffix(modelID string) bool {
	return modelidentity.HasSnapshotReleaseSuffix(modelID)
}

// HasPreciseDateSuffix returns true if modelID ends with a YYYY-MM-DD or YYYYMMDD date suffix.
func HasPreciseDateSuffix(modelID string) bool {
	return modelidentity.HasPreciseDateSuffix(modelID)
}

// CutPreciseDateSuffix separates a date suffix from the model ID.
func CutPreciseDateSuffix(modelID string) (string, string, bool) {
	return modelidentity.CutPreciseDateSuffix(modelID)
}

// HasCalendarReleaseSuffix returns true if modelID ends with MM-YYYY or YYYY-MM suffix.
func HasCalendarReleaseSuffix(modelID string) bool {
	return modelidentity.HasCalendarReleaseSuffix(modelID)
}

// CutCalendarReleaseSuffix separates a calendar release suffix from the model ID.
func CutCalendarReleaseSuffix(modelID string) (string, string, bool) {
	return modelidentity.CutCalendarReleaseSuffix(modelID)
}

// HasReleaseSuffix returns true if modelID ends with a 4-digit release suffix.
func HasReleaseSuffix(modelID string) bool {
	return modelidentity.HasReleaseSuffix(modelID)
}

// CutReleaseSuffix separates a 4-digit release suffix from the model ID.
func CutReleaseSuffix(modelID string) (string, string, bool) {
	return modelidentity.CutReleaseSuffix(modelID)
}

// ExternalModelCandidates generates canonical model ID candidates from a raw model ID.
func ExternalModelCandidates(raw string) []string {
	return modelidentity.ExternalModelCandidates(raw)
}

// AllDigits returns true if value is non-empty and contains only ASCII digits.
func AllDigits(value string) bool {
	return modelidentity.AllDigits(value)
}
