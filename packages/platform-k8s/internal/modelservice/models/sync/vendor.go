package sync

import (
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"slices"
	"strings"

)

type configuredVendorScope struct {
	canonicalIDs map[string]struct{}
	managedIDs   map[string]struct{}
	aliases      map[string]string
	rawAliases   map[string][]string
}

func newConfiguredVendorScope(vendors []configuredVendor) configuredVendorScope {
	scope := configuredVendorScope{
		canonicalIDs: map[string]struct{}{},
		managedIDs:   map[string]struct{}{},
		aliases:      map[string]string{},
		rawAliases:   map[string][]string{},
	}
	for _, vendor := range vendors {
		vendorID := models.NormalizedVendorSlug(vendor.vendorID)
		if vendorID == "" {
			continue
		}
		scope.aliases[vendorID] = vendorID
		for _, alias := range vendor.aliases {
			scope.rawAliases[vendorID] = appendUniqueString(scope.rawAliases[vendorID], alias)
			bindConfiguredVendorAlias(scope.aliases, alias, vendorID)
		}
		if !models.IsPresetSourceVendorID(vendorID) {
			scope.canonicalIDs[vendorID] = struct{}{}
			scope.managedIDs[vendorID] = struct{}{}
		}
	}
	return scope
}

func cloneConfiguredVendorScope(scope configuredVendorScope) configuredVendorScope {
	return configuredVendorScope{
		canonicalIDs: cloneStringSet(scope.canonicalIDs),
		managedIDs:   cloneStringSet(scope.managedIDs),
		aliases:      cloneStringMap(scope.aliases),
		rawAliases:   cloneStringSliceMap(scope.rawAliases),
	}
}

func bindConfiguredVendorAlias(out map[string]string, alias string, canonical string) {
	normalizedAlias := models.NormalizedVendorSlug(alias)
	canonical = models.NormalizedVendorSlug(canonical)
	if normalizedAlias == "" || canonical == "" {
		return
	}
	if current, ok := out[normalizedAlias]; ok && current != canonical {
		out[normalizedAlias] = ""
		return
	}
	out[normalizedAlias] = canonical
}

func (s configuredVendorScope) canonicalVendorID(raw string) (string, bool) {
	vendorID, ok := s.configuredVendorID(raw)
	if !ok {
		return "", false
	}
	if _, ok := s.canonicalIDs[vendorID]; ok {
		return vendorID, true
	}
	return "", false
}


func (s configuredVendorScope) configuredVendorID(raw string) (string, bool) {
	vendorID := models.NormalizedVendorSlug(raw)
	if canonical, ok := s.aliases[vendorID]; ok {
		if canonical == "" {
			return "", false
		}
		return canonical, true
	}
	return "", false
}

func (s configuredVendorScope) aliasCandidates(vendorID string) []string {
	vendorID = models.NormalizedVendorSlug(vendorID)
	if vendorID == "" {
		return nil
	}
	out := append([]string(nil), s.rawAliases[vendorID]...)
	slices.Sort(out)
	return out
}

func appendUniqueString(values []string, candidate string) []string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return values
	}
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

