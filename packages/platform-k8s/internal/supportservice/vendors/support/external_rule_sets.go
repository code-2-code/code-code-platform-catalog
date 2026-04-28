package support

import (
	"bytes"
	"embed"
	"fmt"
	"slices"
	"strings"

	egressv1 "code-code.internal/go-contract/egress/v1"
	"gopkg.in/yaml.v3"
)

//go:embed external_rule_sets.yaml
var externalRuleSetFS embed.FS

type externalRuleSetFile struct {
	RuleSets []externalRuleSetConfig `yaml:"ruleSets"`
}

type externalRuleSetConfig struct {
	RuleSetID             string                 `yaml:"ruleSetId"`
	DisplayName           string                 `yaml:"displayName"`
	OwnerService          string                 `yaml:"ownerService"`
	PolicyID              string                 `yaml:"policyId"`
	StartupSync           *bool                  `yaml:"startupSync"`
	SourceServiceAccounts []string               `yaml:"sourceServiceAccounts"`
	Rules                 []externalRuleSetRule  `yaml:"rules"`
	HTTPRoutes            []externalRuleSetRoute `yaml:"httpRoutes"`
}

type externalRuleSetRule struct {
	RuleID                string   `yaml:"ruleId"`
	DestinationID         string   `yaml:"destinationId"`
	DisplayName           string   `yaml:"displayName"`
	HostExact             string   `yaml:"hostExact"`
	HostWildcard          string   `yaml:"hostWildcard"`
	Port                  int32    `yaml:"port"`
	Protocol              string   `yaml:"protocol"`
	Resolution            string   `yaml:"resolution"`
	SourceServiceAccounts []string `yaml:"sourceServiceAccounts"`
}

type externalRuleSetRoute struct {
	RouteID            string                      `yaml:"routeId"`
	DisplayName        string                      `yaml:"displayName"`
	DestinationID      string                      `yaml:"destinationId"`
	Matches            []externalRuleSetMatch      `yaml:"matches"`
	RequestHeaders     externalRuleSetHeaderPolicy `yaml:"requestHeaders"`
	ResponseHeaders    externalRuleSetHeaderPolicy `yaml:"responseHeaders"`
	AuthPolicyID       string                      `yaml:"authPolicyId"`
	DynamicHeaderAuthz bool                        `yaml:"dynamicHeaderAuthz"`
}

type externalRuleSetMatch struct {
	PathPrefixes []string `yaml:"pathPrefixes"`
	Methods      []string `yaml:"methods"`
}

type externalRuleSetHeaderPolicy struct {
	Add    []externalRuleSetHeaderValue `yaml:"add"`
	Set    []externalRuleSetHeaderValue `yaml:"set"`
	Remove []string                     `yaml:"remove"`
}

type externalRuleSetHeaderValue struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

var presetExternalRuleSets = mustLoadExternalRuleSets()

func StartupExternalAccessSets() []*egressv1.ExternalAccessSet {
	sets := PresetExternalRuleSetAccessSets()
	sets = append(sets, PresetProxyAccessSets()...)
	return sets
}

func PresetExternalRuleSetAccessSets() []*egressv1.ExternalAccessSet {
	out := make([]*egressv1.ExternalAccessSet, 0, len(presetExternalRuleSets))
	for _, ruleSet := range presetExternalRuleSets {
		if !startupSyncEnabled(ruleSet.StartupSync) {
			continue
		}
		out = append(out, externalAccessSetFromRuleSet(ruleSet))
	}
	return out
}

func ExternalRuleSetAccessSet(ruleSetID string) (*egressv1.ExternalAccessSet, bool) {
	ruleSetID = strings.TrimSpace(ruleSetID)
	for _, ruleSet := range presetExternalRuleSets {
		if ruleSet.RuleSetID == ruleSetID {
			return externalAccessSetFromRuleSet(ruleSet), true
		}
	}
	return nil, false
}

func mustLoadExternalRuleSets() []externalRuleSetConfig {
	payload, err := externalRuleSetFS.ReadFile("external_rule_sets.yaml")
	if err != nil {
		panic(fmt.Sprintf("platformk8s/vendors/support: read external rule sets: %v", err))
	}
	decoder := yaml.NewDecoder(bytes.NewReader(payload))
	decoder.KnownFields(true)
	var file externalRuleSetFile
	if err := decoder.Decode(&file); err != nil {
		panic(fmt.Sprintf("platformk8s/vendors/support: parse external rule sets: %v", err))
	}
	ruleSets, err := normalizeExternalRuleSetFile(file)
	if err != nil {
		panic(fmt.Sprintf("platformk8s/vendors/support: invalid external rule sets: %v", err))
	}
	return ruleSets
}

func normalizeExternalRuleSetFile(file externalRuleSetFile) ([]externalRuleSetConfig, error) {
	seenSets := map[string]struct{}{}
	ruleSets := make([]externalRuleSetConfig, 0, len(file.RuleSets))
	for setIndex, ruleSet := range file.RuleSets {
		normalizedSet := externalRuleSetConfig{
			RuleSetID:             strings.TrimSpace(ruleSet.RuleSetID),
			DisplayName:           strings.TrimSpace(ruleSet.DisplayName),
			OwnerService:          strings.TrimSpace(ruleSet.OwnerService),
			PolicyID:              strings.TrimSpace(ruleSet.PolicyID),
			StartupSync:           boolPtr(startupSyncEnabled(ruleSet.StartupSync)),
			SourceServiceAccounts: normalizeStringList(ruleSet.SourceServiceAccounts),
			Rules:                 make([]externalRuleSetRule, 0, len(ruleSet.Rules)),
			HTTPRoutes:            make([]externalRuleSetRoute, 0, len(ruleSet.HTTPRoutes)),
		}
		if normalizedSet.RuleSetID == "" {
			return nil, fmt.Errorf("ruleSets[%d].ruleSetId is required", setIndex)
		}
		if _, ok := seenSets[normalizedSet.RuleSetID]; ok {
			return nil, fmt.Errorf("duplicate ruleSetId %q", normalizedSet.RuleSetID)
		}
		seenSets[normalizedSet.RuleSetID] = struct{}{}
		if normalizedSet.DisplayName == "" {
			normalizedSet.DisplayName = normalizedSet.RuleSetID
		}
		if normalizedSet.OwnerService == "" {
			return nil, fmt.Errorf("ruleSet %q ownerService is required", normalizedSet.RuleSetID)
		}
		if len(normalizedSet.SourceServiceAccounts) == 0 {
			return nil, fmt.Errorf("ruleSet %q sourceServiceAccounts is required", normalizedSet.RuleSetID)
		}
		if len(ruleSet.Rules) == 0 {
			return nil, fmt.Errorf("ruleSet %q rules is required", normalizedSet.RuleSetID)
		}
		seenRules := map[string]struct{}{}
		for ruleIndex, rule := range ruleSet.Rules {
			normalizedRule, err := normalizeExternalRuleSetRule(normalizedSet.RuleSetID, ruleIndex, rule)
			if err != nil {
				return nil, fmt.Errorf("ruleSet %q: %w", normalizedSet.RuleSetID, err)
			}
			if _, ok := seenRules[normalizedRule.RuleID]; ok {
				return nil, fmt.Errorf("ruleSet %q duplicate ruleId %q", normalizedSet.RuleSetID, normalizedRule.RuleID)
			}
			seenRules[normalizedRule.RuleID] = struct{}{}
			normalizedSet.Rules = append(normalizedSet.Rules, normalizedRule)
		}
		slices.SortFunc(normalizedSet.Rules, func(left, right externalRuleSetRule) int {
			return strings.Compare(left.RuleID, right.RuleID)
		})
		knownDestinations := map[string]struct{}{}
		for _, rule := range normalizedSet.Rules {
			knownDestinations[rule.DestinationID] = struct{}{}
		}
		seenRoutes := map[string]struct{}{}
		for routeIndex, route := range ruleSet.HTTPRoutes {
			normalizedRoute, err := normalizeExternalRuleSetRoute(routeIndex, route, knownDestinations)
			if err != nil {
				return nil, fmt.Errorf("ruleSet %q: %w", normalizedSet.RuleSetID, err)
			}
			if _, ok := seenRoutes[normalizedRoute.RouteID]; ok {
				return nil, fmt.Errorf("ruleSet %q duplicate http route %q", normalizedSet.RuleSetID, normalizedRoute.RouteID)
			}
			seenRoutes[normalizedRoute.RouteID] = struct{}{}
			normalizedSet.HTTPRoutes = append(normalizedSet.HTTPRoutes, normalizedRoute)
		}
		slices.SortFunc(normalizedSet.HTTPRoutes, func(left, right externalRuleSetRoute) int {
			return strings.Compare(left.RouteID, right.RouteID)
		})
		ruleSets = append(ruleSets, normalizedSet)
	}
	slices.SortFunc(ruleSets, func(left, right externalRuleSetConfig) int {
		return strings.Compare(left.RuleSetID, right.RuleSetID)
	})
	return ruleSets, nil
}

func startupSyncEnabled(value *bool) bool {
	return value == nil || *value
}

func boolPtr(value bool) *bool {
	return &value
}

func normalizeExternalRuleSetRule(ruleSetID string, index int, rule externalRuleSetRule) (externalRuleSetRule, error) {
	ruleID := strings.TrimSpace(rule.RuleID)
	if ruleID == "" {
		return externalRuleSetRule{}, fmt.Errorf("rules[%d].ruleId is required", index)
	}
	hostExact := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(rule.HostExact), "."))
	hostWildcard := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(rule.HostWildcard), "."))
	if (hostExact == "") == (hostWildcard == "") {
		return externalRuleSetRule{}, fmt.Errorf("rule %q must set exactly one of hostExact or hostWildcard", ruleID)
	}
	destinationID := strings.TrimSpace(rule.DestinationID)
	if destinationID == "" {
		destinationID = ruleSetID + "." + ruleID
	}
	displayName := strings.TrimSpace(rule.DisplayName)
	if displayName == "" {
		displayName = ruleID
	}
	port := rule.Port
	if port == 0 {
		port = 443
	}
	if port < 1 || port > 65535 {
		return externalRuleSetRule{}, fmt.Errorf("rule %q port must be between 1 and 65535", ruleID)
	}
	return externalRuleSetRule{
		RuleID:                ruleID,
		DestinationID:         destinationID,
		DisplayName:           displayName,
		HostExact:             hostExact,
		HostWildcard:          hostWildcard,
		Port:                  port,
		Protocol:              strings.TrimSpace(rule.Protocol),
		Resolution:            strings.TrimSpace(rule.Resolution),
		SourceServiceAccounts: normalizeStringList(rule.SourceServiceAccounts),
	}, nil
}

func normalizeExternalRuleSetRoute(index int, route externalRuleSetRoute, knownDestinations map[string]struct{}) (externalRuleSetRoute, error) {
	routeID := strings.TrimSpace(route.RouteID)
	if routeID == "" {
		return externalRuleSetRoute{}, fmt.Errorf("httpRoutes[%d].routeId is required", index)
	}
	destinationID := strings.TrimSpace(route.DestinationID)
	if destinationID == "" {
		return externalRuleSetRoute{}, fmt.Errorf("http route %q destinationId is required", routeID)
	}
	if _, ok := knownDestinations[destinationID]; !ok {
		return externalRuleSetRoute{}, fmt.Errorf("http route %q references unknown destination %q", routeID, destinationID)
	}
	displayName := strings.TrimSpace(route.DisplayName)
	if displayName == "" {
		displayName = routeID
	}
	return externalRuleSetRoute{
		RouteID:            routeID,
		DisplayName:        displayName,
		DestinationID:      destinationID,
		Matches:            normalizeExternalRuleSetMatches(route.Matches),
		RequestHeaders:     normalizeExternalRuleSetHeaderPolicy(route.RequestHeaders),
		ResponseHeaders:    normalizeExternalRuleSetHeaderPolicy(route.ResponseHeaders),
		AuthPolicyID:       strings.TrimSpace(route.AuthPolicyID),
		DynamicHeaderAuthz: route.DynamicHeaderAuthz,
	}, nil
}

func normalizeExternalRuleSetMatches(matches []externalRuleSetMatch) []externalRuleSetMatch {
	out := make([]externalRuleSetMatch, 0, len(matches))
	for _, match := range matches {
		out = append(out, externalRuleSetMatch{
			PathPrefixes: normalizeStringList(match.PathPrefixes),
			Methods:      normalizeStringList(match.Methods),
		})
	}
	return out
}

func normalizeExternalRuleSetHeaderPolicy(policy externalRuleSetHeaderPolicy) externalRuleSetHeaderPolicy {
	return externalRuleSetHeaderPolicy{
		Add:    normalizeExternalRuleSetHeaderValues(policy.Add),
		Set:    normalizeExternalRuleSetHeaderValues(policy.Set),
		Remove: normalizeStringList(policy.Remove),
	}
}

func normalizeExternalRuleSetHeaderValues(values []externalRuleSetHeaderValue) []externalRuleSetHeaderValue {
	out := make([]externalRuleSetHeaderValue, 0, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value.Name)
		if name == "" {
			continue
		}
		out = append(out, externalRuleSetHeaderValue{Name: name, Value: value.Value})
	}
	return out
}

func externalAccessSetFromRuleSet(ruleSet externalRuleSetConfig) *egressv1.ExternalAccessSet {
	accessSet := &egressv1.ExternalAccessSet{
		AccessSetId:  ruleSet.RuleSetID,
		DisplayName:  ruleSet.DisplayName,
		OwnerService: ruleSet.OwnerService,
		PolicyId:     ruleSet.PolicyID,
	}
	for _, rule := range ruleSet.Rules {
		accessSet.ExternalRules = append(accessSet.ExternalRules, &egressv1.ExternalRule{
			ExternalRuleId: ruleSet.RuleSetID + "." + rule.RuleID,
			DestinationId:  rule.DestinationID,
			DisplayName:    rule.DisplayName,
			HostMatch:      hostMatch(rule),
			Port:           rule.Port,
			Protocol:       egressProtocol(rule.Protocol),
			Resolution:     egressResolution(rule.Resolution),
		})
		accessSet.ServiceRules = append(accessSet.ServiceRules, &egressv1.ServiceRule{
			ServiceRuleId:         rule.DestinationID + ".services",
			DestinationId:         rule.DestinationID,
			SourceServiceAccounts: sourceServiceAccounts(ruleSet.SourceServiceAccounts, rule.SourceServiceAccounts),
		})
	}
	for _, route := range ruleSet.HTTPRoutes {
		accessSet.HttpRoutes = append(accessSet.HttpRoutes, &egressv1.HttpEgressRoute{
			RouteId:            route.RouteID,
			DisplayName:        route.DisplayName,
			DestinationId:      route.DestinationID,
			Matches:            httpRouteMatches(route.Matches),
			RequestHeaders:     httpHeaderPolicy(route.RequestHeaders),
			ResponseHeaders:    httpHeaderPolicy(route.ResponseHeaders),
			AuthPolicyId:       route.AuthPolicyID,
			DynamicHeaderAuthz: route.DynamicHeaderAuthz,
		})
	}
	return accessSet
}

func hostMatch(rule externalRuleSetRule) *egressv1.HostMatch {
	if rule.HostWildcard != "" {
		return &egressv1.HostMatch{Kind: &egressv1.HostMatch_HostWildcard{HostWildcard: rule.HostWildcard}}
	}
	return &egressv1.HostMatch{Kind: &egressv1.HostMatch_HostExact{HostExact: rule.HostExact}}
}

func httpRouteMatches(matches []externalRuleSetMatch) []*egressv1.HttpRouteMatch {
	out := make([]*egressv1.HttpRouteMatch, 0, len(matches))
	for _, match := range matches {
		out = append(out, &egressv1.HttpRouteMatch{
			PathPrefixes: match.PathPrefixes,
			Methods:      match.Methods,
		})
	}
	return out
}

func httpHeaderPolicy(policy externalRuleSetHeaderPolicy) *egressv1.HttpHeaderPolicy {
	if len(policy.Add) == 0 && len(policy.Set) == 0 && len(policy.Remove) == 0 {
		return nil
	}
	return &egressv1.HttpHeaderPolicy{
		Add:    httpHeaderValues(policy.Add),
		Set:    httpHeaderValues(policy.Set),
		Remove: policy.Remove,
	}
}

func httpHeaderValues(values []externalRuleSetHeaderValue) []*egressv1.HttpHeaderValue {
	out := make([]*egressv1.HttpHeaderValue, 0, len(values))
	for _, value := range values {
		out = append(out, &egressv1.HttpHeaderValue{Name: value.Name, Value: value.Value})
	}
	return out
}

func egressProtocol(value string) egressv1.EgressProtocol {
	switch strings.ToLower(value) {
	case "http":
		return egressv1.EgressProtocol_EGRESS_PROTOCOL_HTTP
	case "https":
		return egressv1.EgressProtocol_EGRESS_PROTOCOL_HTTPS
	case "tcp":
		return egressv1.EgressProtocol_EGRESS_PROTOCOL_TCP
	default:
		return egressv1.EgressProtocol_EGRESS_PROTOCOL_TLS
	}
}

func egressResolution(value string) egressv1.EgressResolution {
	switch strings.ToLower(value) {
	case "dynamic-dns":
		return egressv1.EgressResolution_EGRESS_RESOLUTION_DYNAMIC_DNS
	case "none":
		return egressv1.EgressResolution_EGRESS_RESOLUTION_NONE
	default:
		return egressv1.EgressResolution_EGRESS_RESOLUTION_DNS
	}
}
