package support

import (
	"slices"
	"testing"

	egressv1 "code-code.internal/go-contract/egress/v1"
)

func TestPresetExternalRuleSetsProjectToExternalAccessSets(t *testing.T) {
	sets := PresetExternalRuleSetAccessSets()
	if got, want := len(sets), 1; got != want {
		t.Fatalf("external rule sets = %d, want %d", got, want)
	}
	accessSet := sets[0]
	if got, want := accessSet.GetAccessSetId(), "support.external-rule-set.bootstrap"; got != want {
		t.Fatalf("access set id = %q, want %q", got, want)
	}
	if got, want := len(accessSet.GetExternalRules()), 4; got != want {
		t.Fatalf("external rules = %d, want %d", got, want)
	}
	if got, want := len(accessSet.GetServiceRules()), 4; got != want {
		t.Fatalf("service rules = %d, want %d", got, want)
	}
	if got, want := len(accessSet.GetHttpRoutes()), 0; got != want {
		t.Fatalf("http routes = %d, want %d", got, want)
	}
	rawRule := externalRuleByDestination(t, accessSet, "support.external-rule-set.bootstrap.github-raw-content")
	if got, want := rawRule.GetHostMatch().GetHostExact(), "raw.githubusercontent.com"; got != want {
		t.Fatalf("raw host exact = %q, want %q", got, want)
	}
	openaiRule := externalRuleByDestination(t, accessSet, "protocol.openai-compatible.api")
	if got, want := openaiRule.GetHostMatch().GetHostExact(), "api.openai.com"; got != want {
		t.Fatalf("openai host exact = %q, want %q", got, want)
	}
	mistralRule := externalRuleByDestination(t, accessSet, "vendor.mistral.api")
	if got, want := mistralRule.GetHostMatch().GetHostExact(), "api.mistral.ai"; got != want {
		t.Fatalf("mistral host exact = %q, want %q", got, want)
	}
	mistralServiceRule := serviceRuleByDestination(t, accessSet, "vendor.mistral.api")
	if got, want := mistralServiceRule.GetSourceServiceAccounts(), []string{
		"code-code/platform-agent-runtime-service",
		"code-code/platform-provider-service",
		"code-code/provider-host-blackbox-exporter",
	}; !slices.Equal(got, want) {
		t.Fatalf("mistral source service accounts = %v, want %v", got, want)
	}
	mistralConsoleRule := externalRuleByDestination(t, accessSet, "vendor.mistral.console")
	if got, want := mistralConsoleRule.GetHostMatch().GetHostExact(), "console.mistral.ai"; got != want {
		t.Fatalf("mistral console host exact = %q, want %q", got, want)
	}
	mistralConsoleServiceRule := serviceRuleByDestination(t, accessSet, "vendor.mistral.console")
	if got, want := mistralConsoleServiceRule.GetSourceServiceAccounts(), []string{"code-code/platform-provider-service"}; !slices.Equal(got, want) {
		t.Fatalf("mistral console source service accounts = %v, want %v", got, want)
	}
}

func TestL7SmokeRuleSetIsParsedButNotStartupSynced(t *testing.T) {
	var smoke externalRuleSetConfig
	found := false
	for _, ruleSet := range presetExternalRuleSets {
		if ruleSet.RuleSetID == "support.external-rule-set.l7-smoke" {
			smoke = ruleSet
			found = true
			break
		}
	}
	if !found {
		t.Fatal("l7 smoke rule set not found")
	}
	if startupSyncEnabled(smoke.StartupSync) {
		t.Fatal("l7 smoke rule set startupSync = true, want false")
	}
	accessSet := externalAccessSetFromRuleSet(smoke)
	if got, want := len(accessSet.GetExternalRules()), 1; got != want {
		t.Fatalf("external rules = %d, want %d", got, want)
	}
	rule := accessSet.GetExternalRules()[0]
	if got, want := rule.GetProtocol(), egressv1.EgressProtocol_EGRESS_PROTOCOL_HTTPS; got != want {
		t.Fatalf("protocol = %v, want %v", got, want)
	}
	if got, want := rule.GetHostMatch().GetHostExact(), "httpbin.org"; got != want {
		t.Fatalf("host exact = %q, want %q", got, want)
	}
	if got, want := len(accessSet.GetHttpRoutes()), 1; got != want {
		t.Fatalf("http routes = %d, want %d", got, want)
	}
	route := accessSet.GetHttpRoutes()[0]
	if got, want := route.GetDestinationId(), "support.external-rule-set.l7-smoke.httpbin-headers"; got != want {
		t.Fatalf("route destination = %q, want %q", got, want)
	}
	if got, want := len(route.GetRequestHeaders().GetSet()), 1; got != want {
		t.Fatalf("request header set = %d, want %d", got, want)
	}
	if got, want := len(route.GetResponseHeaders().GetSet()), 1; got != want {
		t.Fatalf("response header set = %d, want %d", got, want)
	}
	if route.GetDynamicHeaderAuthz() {
		t.Fatal("dynamic header authz = true, want false for static L7 smoke")
	}
}

func TestL7DynamicAuthzSmokeRuleSetIsParsedButNotStartupSynced(t *testing.T) {
	var smoke externalRuleSetConfig
	found := false
	for _, ruleSet := range presetExternalRuleSets {
		if ruleSet.RuleSetID == "support.external-rule-set.l7-dynamic-authz-smoke" {
			smoke = ruleSet
			found = true
			break
		}
	}
	if !found {
		t.Fatal("l7 dynamic authz smoke rule set not found")
	}
	if startupSyncEnabled(smoke.StartupSync) {
		t.Fatal("l7 dynamic authz smoke rule set startupSync = true, want false")
	}
	accessSet := externalAccessSetFromRuleSet(smoke)
	if got, want := len(accessSet.GetExternalRules()), 1; got != want {
		t.Fatalf("external rules = %d, want %d", got, want)
	}
	if got, want := len(accessSet.GetHttpRoutes()), 1; got != want {
		t.Fatalf("http routes = %d, want %d", got, want)
	}
	route := accessSet.GetHttpRoutes()[0]
	if !route.GetDynamicHeaderAuthz() {
		t.Fatal("dynamic header authz = false, want true")
	}
}

func TestPresetProxyProjectsToExternalAccessSet(t *testing.T) {
	sets := PresetProxyAccessSets()
	if got, want := len(sets), 1; got != want {
		t.Fatalf("proxy access sets = %d, want %d", got, want)
	}
	accessSet := sets[0]
	if got, want := accessSet.GetAccessSetId(), "support.proxy-preset.preset-proxy"; got != want {
		t.Fatalf("access set id = %q, want %q", got, want)
	}
	if got, want := len(accessSet.GetExternalRules()), 1; got != want {
		t.Fatalf("external rules = %d, want %d", got, want)
	}
	rule := accessSet.GetExternalRules()[0]
	if got, want := rule.GetHostMatch().GetHostExact(), "preset-proxy.local"; got != want {
		t.Fatalf("host exact = %q, want %q", got, want)
	}
	if got, want := rule.GetAddressCidr(), "192.168.0.126/32"; got != want {
		t.Fatalf("address cidr = %q, want %q", got, want)
	}
	if got, want := rule.GetProtocol(), egressv1.EgressProtocol_EGRESS_PROTOCOL_TCP; got != want {
		t.Fatalf("protocol = %v, want %v", got, want)
	}
	if got, want := rule.GetResolution(), egressv1.EgressResolution_EGRESS_RESOLUTION_NONE; got != want {
		t.Fatalf("resolution = %v, want %v", got, want)
	}
	if got, want := len(accessSet.GetServiceRules()), 1; got != want {
		t.Fatalf("service rules = %d, want %d", got, want)
	}
	serviceRule := accessSet.GetServiceRules()[0]
	if got, want := serviceRule.GetDestinationId(), "preset-proxy"; got != want {
		t.Fatalf("service rule destination = %q, want %q", got, want)
	}
	if got, want := serviceRule.GetSourceServiceAccounts(), []string{"code-code/platform-support-service"}; !slices.Equal(got, want) {
		t.Fatalf("source service accounts = %v, want %v", got, want)
	}
}

func TestStartupExternalAccessSetsIncludesOnlyNetworkOwnedSets(t *testing.T) {
	sets := StartupExternalAccessSets()
	if got, want := len(sets), 2; got != want {
		t.Fatalf("startup access sets = %d, want %d", got, want)
	}
	if got, want := sets[0].GetAccessSetId(), "support.external-rule-set.bootstrap"; got != want {
		t.Fatalf("first access set id = %q, want %q", got, want)
	}
	if got, want := sets[1].GetAccessSetId(), "support.proxy-preset.preset-proxy"; got != want {
		t.Fatalf("second access set id = %q, want %q", got, want)
	}
}

func externalRuleByDestination(t *testing.T, accessSet *egressv1.ExternalAccessSet, destinationID string) *egressv1.ExternalRule {
	t.Helper()
	for _, rule := range accessSet.GetExternalRules() {
		if rule.GetDestinationId() == destinationID {
			return rule
		}
	}
	t.Fatalf("external rule destination %q not found", destinationID)
	return nil
}

func serviceRuleByDestination(t *testing.T, accessSet *egressv1.ExternalAccessSet, destinationID string) *egressv1.ServiceRule {
	t.Helper()
	for _, rule := range accessSet.GetServiceRules() {
		if rule.GetDestinationId() == destinationID {
			return rule
		}
	}
	t.Fatalf("service rule destination %q not found", destinationID)
	return nil
}
