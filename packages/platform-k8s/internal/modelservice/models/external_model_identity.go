package models

import (
	modelv1 "code-code.internal/go-contract/model/v1"
	"code-code.internal/platform-k8s/internal/modelservice/models/source"
)

// Delegate to source package for shared identity resolution.

func normalizeExternalModelIdentity(vendorID string, rawModelID string, knownCanonicalModelIDs map[string]struct{}) (string, []*modelv1.ModelAlias, bool) {
	return source.NormalizeExternalModelIdentity(vendorID, rawModelID, knownCanonicalModelIDs)
}

func normalizeExternalModelSlug(raw string) string {
	return source.NormalizeExternalModelSlug(raw)
}

func hasChannelToken(value string) bool {
	return source.HasChannelToken(value)
}

func hasSnapshotReleaseSuffix(modelID string) bool {
	return source.HasSnapshotReleaseSuffix(modelID)
}

func hasPreciseDateSuffix(modelID string) bool {
	return source.HasPreciseDateSuffix(modelID)
}

func cutPreciseDateSuffix(modelID string) (string, string, bool) {
	return source.CutPreciseDateSuffix(modelID)
}

func cutCalendarReleaseSuffix(modelID string) (string, string, bool) {
	return source.CutCalendarReleaseSuffix(modelID)
}

func hasCalendarReleaseSuffix(modelID string) bool {
	return source.HasCalendarReleaseSuffix(modelID)
}

func cutReleaseSuffix(modelID string) (string, string, bool) {
	return source.CutReleaseSuffix(modelID)
}

func hasReleaseSuffix(modelID string) bool {
	return source.HasReleaseSuffix(modelID)
}

func externalModelCandidates(raw string) []string {
	return source.ExternalModelCandidates(raw)
}

func buildExternalAliases(modelID string, raw string) []*modelv1.ModelAlias {
	return source.BuildExternalAliases(modelID, raw)
}

func allDigits(value string) bool {
	return source.AllDigits(value)
}
