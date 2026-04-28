// Package modelidentity provides shared model and vendor identity normalization
// used by both model source collectors and provider catalog probes.
package modelidentity

import (
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
)

// NormalizedVendorSlug normalizes a vendor identifier to a lowercase, hyphen-separated slug.
func NormalizedVendorSlug(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	replacer := strings.NewReplacer(
		"_", "-",
		" ", "-",
		".", "-",
		"/", "-",
	)
	raw = replacer.Replace(raw)
	raw = strings.Trim(raw, "-")
	for strings.Contains(raw, "--") {
		raw = strings.ReplaceAll(raw, "--", "-")
	}
	return raw
}

// NormalizeExternalModelIdentity resolves a raw external model ID to a canonical
// model ID with optional aliases. It prefers the shortest candidate produced by
// stripping date/release suffixes so that snapshot versions collapse to the
// canonical family slug.
func NormalizeExternalModelIdentity(vendorID string, rawModelID string) (string, []*modelv1.ModelAlias, bool) {
	rawModelID = NormalizeExternalModelSlug(rawModelID)
	if rawModelID == "" || HasChannelToken(rawModelID) {
		return "", nil, false
	}

	candidates := ExternalModelCandidates(rawModelID)
	for _, candidate := range candidates {
		if candidate != rawModelID {
			return candidate, BuildExternalAliases(candidate, rawModelID), true
		}
	}
	return rawModelID, nil, true
}

// NormalizeExternalModelSlug normalizes a raw model slug to lowercase, trimmed form.
func NormalizeExternalModelSlug(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	replacer := strings.NewReplacer(
		"_", "-",
		" ", "-",
		"/", "-",
	)
	raw = replacer.Replace(raw)
	for strings.Contains(raw, "--") {
		raw = strings.ReplaceAll(raw, "--", "-")
	}
	raw = strings.Trim(raw, "-")
	switch {
	case strings.HasPrefix(raw, "qwen-3-"):
		return "qwen3-" + strings.TrimPrefix(raw, "qwen-3-")
	case strings.HasPrefix(raw, "qwen-3."):
		return "qwen3." + strings.TrimPrefix(raw, "qwen-3.")
	default:
		return raw
	}
}

// HasChannelToken returns true if the model ID contains channel tokens like
// "preview", "latest", or "experimental" that indicate non-canonical entries.
func HasChannelToken(value string) bool {
	for _, token := range strings.FieldsFunc(strings.ToLower(strings.TrimSpace(value)), splitModelToken) {
		switch token {
		case "preview", "latest", "experimental":
			return true
		}
	}
	return false
}

// HasModelToken checks if a model ID contains any of the specified tokens.
func HasModelToken(value string, tokens ...string) bool {
	if len(tokens) == 0 {
		return false
	}
	known := map[string]struct{}{}
	for _, token := range tokens {
		token = strings.TrimSpace(strings.ToLower(token))
		if token != "" {
			known[token] = struct{}{}
		}
	}
	for _, token := range strings.FieldsFunc(strings.ToLower(strings.TrimSpace(value)), splitModelToken) {
		if _, ok := known[token]; ok {
			return true
		}
	}
	return false
}

func splitModelToken(r rune) bool {
	switch r {
	case '-', '_', '.', '/', ':', ' ', '(', ')':
		return true
	default:
		return false
	}
}

// BuildExternalAliases creates model aliases when the canonical model ID
// differs from the raw source model ID.
func BuildExternalAliases(modelID string, raw string) []*modelv1.ModelAlias {
	modelID = strings.TrimSpace(modelID)
	raw = strings.TrimSpace(raw)
	if modelID == "" || raw == "" || modelID == raw {
		return nil
	}
	kind := modelv1.AliasKind_ALIAS_KIND_STABLE
	if HasSnapshotReleaseSuffix(raw) {
		kind = modelv1.AliasKind_ALIAS_KIND_SNAPSHOT
	}
	return []*modelv1.ModelAlias{{
		Kind:  kind,
		Value: raw,
	}}
}

// HasSnapshotReleaseSuffix returns true if the model ID has a snapshot-style release suffix.
func HasSnapshotReleaseSuffix(modelID string) bool {
	return HasPreciseDateSuffix(modelID) || HasCalendarReleaseSuffix(modelID) || HasReleaseSuffix(modelID)
}

// HasPreciseDateSuffix returns true if modelID ends with a YYYY-MM-DD or YYYYMMDD date suffix.
func HasPreciseDateSuffix(modelID string) bool {
	if base, _, ok := CutPreciseDateSuffix(modelID); ok {
		return strings.TrimSpace(base) != ""
	}
	return false
}

// CutPreciseDateSuffix separates a date suffix from the model ID.
func CutPreciseDateSuffix(modelID string) (string, string, bool) {
	modelID = strings.TrimSpace(modelID)
	if len(modelID) > len("-YYYY-MM-DD") {
		if year, month, day, ok := trailingDateParts(modelID, 10, true); ok {
			_ = year
			_ = month
			_ = day
			return strings.TrimSpace(modelID[:len(modelID)-11]), modelID[len(modelID)-10:], true
		}
	}
	if len(modelID) > len("-YYYYMMDD") {
		if year, month, day, ok := trailingDateParts(modelID, 8, false); ok {
			_ = year
			_ = month
			_ = day
			return strings.TrimSpace(modelID[:len(modelID)-9]), modelID[len(modelID)-8:], true
		}
	}
	return "", "", false
}

// HasCalendarReleaseSuffix returns true if modelID ends with MM-YYYY or YYYY-MM suffix.
func HasCalendarReleaseSuffix(modelID string) bool {
	base, _, ok := CutCalendarReleaseSuffix(modelID)
	return ok && strings.TrimSpace(base) != ""
}

// CutCalendarReleaseSuffix separates a calendar release suffix from the model ID.
func CutCalendarReleaseSuffix(modelID string) (string, string, bool) {
	modelID = strings.TrimSpace(modelID)
	if len(modelID) > len("-MM-YYYY") {
		if _, _, ok := trailingMonthYearSuffix(modelID); ok {
			return strings.TrimSpace(modelID[:len(modelID)-8]), modelID[len(modelID)-7:], true
		}
	}
	if len(modelID) > len("-YYYY-MM") {
		if _, _, ok := trailingYearMonthSuffix(modelID); ok {
			return strings.TrimSpace(modelID[:len(modelID)-8]), modelID[len(modelID)-7:], true
		}
	}
	return "", "", false
}

// HasReleaseSuffix returns true if modelID ends with a 4-digit release suffix.
func HasReleaseSuffix(modelID string) bool {
	base, _, ok := CutReleaseSuffix(modelID)
	return ok && strings.TrimSpace(base) != ""
}

// CutReleaseSuffix separates a 4-digit release suffix from the model ID.
func CutReleaseSuffix(modelID string) (string, string, bool) {
	modelID = strings.TrimSpace(modelID)
	if len(modelID) <= len("-0000") {
		return "", "", false
	}
	lastDash := strings.LastIndex(modelID, "-")
	if lastDash <= 0 || lastDash == len(modelID)-1 {
		return "", "", false
	}
	suffix := modelID[lastDash+1:]
	if len(suffix) != 4 || !AllDigits(suffix) {
		return "", "", false
	}
	return strings.TrimSpace(modelID[:lastDash]), suffix, true
}

// ExternalModelCandidates generates canonical model ID candidates from a raw model ID.
func ExternalModelCandidates(raw string) []string {
	out := make([]string, 0, 4)
	appendCandidate := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		for _, existing := range out {
			if existing == candidate {
				return
			}
		}
		out = append(out, candidate)
	}
	if base, _, ok := CutPreciseDateSuffix(raw); ok {
		appendCandidate(base)
	}
	if base, _, ok := CutCalendarReleaseSuffix(raw); ok {
		appendCandidate(base)
		appendCandidate(strings.TrimSuffix(base, "-instruct"))
	}
	if base, _, ok := CutReleaseSuffix(raw); ok {
		appendCandidate(base)
		appendCandidate(strings.TrimSuffix(base, "-instruct"))
	}
	appendCandidate(raw)
	return out
}

// AllDigits returns true if value is non-empty and contains only ASCII digits.
func AllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func trailingMonthYearSuffix(value string) (string, string, bool) {
	if len(value) < len("-MM-YYYY") || value[len(value)-8] != '-' {
		return "", "", false
	}
	suffix := value[len(value)-7:]
	if suffix[2] != '-' {
		return "", "", false
	}
	month := suffix[:2]
	year := suffix[3:]
	return year, month, isValidMonthParts(year, month)
}

func trailingYearMonthSuffix(value string) (string, string, bool) {
	if len(value) < len("-YYYY-MM") || value[len(value)-8] != '-' {
		return "", "", false
	}
	suffix := value[len(value)-7:]
	if suffix[4] != '-' {
		return "", "", false
	}
	year := suffix[:4]
	month := suffix[5:]
	return year, month, isValidMonthParts(year, month)
}

func isValidMonthParts(year string, month string) bool {
	return len(year) == 4 && len(month) == 2 &&
		AllDigits(year) && AllDigits(month) &&
		month >= "01" && month <= "12"
}

func trailingDateParts(value string, digits int, dashed bool) (string, string, string, bool) {
	if dashed {
		if len(value) < digits+1 || value[len(value)-11] != '-' {
			return "", "", "", false
		}
		date := value[len(value)-10:]
		if date[4] != '-' || date[7] != '-' {
			return "", "", "", false
		}
		year, month, day := date[:4], date[5:7], date[8:10]
		return year, month, day, isValidDateParts(year, month, day)
	}
	if len(value) < digits+1 || value[len(value)-9] != '-' {
		return "", "", "", false
	}
	date := value[len(value)-8:]
	year, month, day := date[:4], date[4:6], date[6:8]
	return year, month, day, isValidDateParts(year, month, day)
}

func isValidDateParts(year string, month string, day string) bool {
	return len(year) == 4 && len(month) == 2 && len(day) == 2 &&
		AllDigits(year) && AllDigits(month) && AllDigits(day) &&
		month >= "01" && month <= "12" &&
		day >= "01" && day <= "31"
}
