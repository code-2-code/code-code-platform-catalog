package models

import "code-code.internal/platform-k8s/internal/modelservice/modelidentity"

// NormalizedVendorSlug normalizes a raw vendor identifier to its canonical slug form.
func NormalizedVendorSlug(raw string) string {
	return modelidentity.NormalizedVendorSlug(raw)
}
