package store

import "slices"

func equalStrings(a []string, b []string) bool {
	return slices.Equal(a, b)
}
