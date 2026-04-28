package models

import (
	"strings"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
)

func definitionSourceAuthorityPriority(sourceID string) int {
	priority, ok := LookupSourceAuthorityPriority(sourceID)
	if !ok {
		return 0
	}
	return priority
}

func collectedEntryAuthorityPriority(item *modelservicev1.CollectedModelEntry) int {
	priority := 0
	for _, source := range item.GetSources() {
		if candidate := collectedSourcePriority(source); candidate > priority {
			priority = candidate
		}
	}
	return priority
}

func collectedEntryAuthoritySourceID(item *modelservicev1.CollectedModelEntry) string {
	bestPriority := -1
	bestSourceID := ""
	for _, source := range item.GetSources() {
		priority := collectedSourcePriority(source)
		sourceID := NormalizedVendorSlug(source.GetSourceId())
		if priority > bestPriority || (priority == bestPriority && compareSourceID(sourceID, bestSourceID) < 0) {
			bestPriority = priority
			bestSourceID = sourceID
		}
	}
	return bestSourceID
}

func compareCollectedEntryAuthority(left *modelservicev1.CollectedModelEntry, right *modelservicev1.CollectedModelEntry) int {
	leftPriority := collectedEntryAuthorityPriority(left)
	rightPriority := collectedEntryAuthorityPriority(right)
	if leftPriority != rightPriority {
		if leftPriority > rightPriority {
			return -1
		}
		return 1
	}
	return compareSourceID(collectedEntryAuthoritySourceID(left), collectedEntryAuthoritySourceID(right))
}

func compareSourceID(left string, right string) int {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	switch {
	case left == right:
		return 0
	case left == "":
		return 1
	case right == "":
		return -1
	default:
		return strings.Compare(left, right)
	}
}

func collectedSourcePriority(source *modelservicev1.CollectedModelSource) int {
	if source.GetAuthorityPriority() > 0 {
		return int(source.GetAuthorityPriority())
	}
	return definitionSourceAuthorityPriority(source.GetSourceId())
}
