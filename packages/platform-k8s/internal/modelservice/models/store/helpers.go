package store

import "strings"

// boolKey encodes a boolean as "0" or "1" for compound identity keys.
func boolKey(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

// identityKey builds a compound lookup key from vendor and model ID.
func identityKey(vendorID string, modelID string) string {
	return strings.TrimSpace(vendorID) + "\x00" + strings.TrimSpace(modelID)
}
