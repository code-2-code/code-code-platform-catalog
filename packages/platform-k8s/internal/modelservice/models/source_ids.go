package models

// NormalizeDefinitionSourceID returns the canonical form of a source ID.
func NormalizeDefinitionSourceID(sourceID string) string {
	return NormalizeSourceID(sourceID)
}

// NormalizeDefinitionSourceIDs normalizes and deduplicates source IDs.
func NormalizeDefinitionSourceIDs(sourceIDs []string) []string {
	if len(sourceIDs) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		normalized := NormalizeDefinitionSourceID(sourceID)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
