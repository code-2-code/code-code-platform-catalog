// Package source defines shared types for model source sub-packages.
// Both the root models package and individual source vendor packages import this.
package source

import (
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
)

// CollectionContext provides vendor resolution context to source sub-packages
// during model collection.
type CollectionContext struct {
	// ResolveVendor maps a raw vendor name (e.g. "mistralai") to its canonical ID.
	ResolveVendor func(raw string) (canonical string, ok bool)
	// AliasCandidates returns known aliases for a vendor ID.
	AliasCandidates func(vendorID string) []string
}

// CollectedEntry is a type alias for the proto-generated CollectedModelEntry.
// Source sub-packages produce grouped entries by vendor.
type CollectedEntry = modelservicev1.CollectedModelEntry

// CollectedSource is a type alias for the proto-generated CollectedModelSource.
type CollectedSource = modelservicev1.CollectedModelSource
