package store

import (
	"strings"

)

const (
	defaultListPageSize = 50
	maxListPageSize     = 100
)

func parseFilterValues(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func normalizePageSize(pageSize int32) int {
	if pageSize <= 0 {
		return defaultListPageSize
	}
	if pageSize > maxListPageSize {
		return maxListPageSize
	}
	return int(pageSize)
}
