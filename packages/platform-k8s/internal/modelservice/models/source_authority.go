package models

// sourceAuthorityPriorities defines the authority priority for each known
// source identifier. Higher values indicate higher trust. Sync collectors
// must use matching source IDs and priorities.
var sourceAuthorityPriorities = map[string]int{
	SourceIDGitHubModels:    600,
	SourceIDCerebras:        500,
	SourceIDModelScope:      400,
	SourceIDNVIDIAIntegrate: 300,
	SourceIDHuggingFaceHub:  200,
	SourceIDOpenRouter:      100,
}

// presetSourceVendorIDs identifies sources that act as preset vendor providers.
var presetSourceVendorIDs = map[string]struct{}{
	SourceIDGitHubModels: {},
	SourceIDCerebras:     {},
	SourceIDModelScope:   {},
	SourceIDOpenRouter:   {},
}

// LookupSourceAuthorityPriority returns the authority priority for a source ID.
func LookupSourceAuthorityPriority(sourceID string) (int, bool) {
	priority, ok := sourceAuthorityPriorities[NormalizedVendorSlug(sourceID)]
	return priority, ok
}

// IsKnownSourceID returns true if the source ID matches a registered source.
func IsKnownSourceID(sourceID string) bool {
	_, ok := sourceAuthorityPriorities[NormalizedVendorSlug(sourceID)]
	return ok
}

// IsPresetSourceVendorID returns true if the vendor ID maps to a preset source.
func IsPresetSourceVendorID(vendorID string) bool {
	_, ok := presetSourceVendorIDs[NormalizedVendorSlug(vendorID)]
	return ok
}

// NormalizeSourceID returns the canonical form of a source ID, or empty if unknown.
func NormalizeSourceID(sourceID string) string {
	normalized := NormalizedVendorSlug(sourceID)
	if _, ok := sourceAuthorityPriorities[normalized]; ok {
		return normalized
	}
	return ""
}
