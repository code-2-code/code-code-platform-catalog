package modelcatalogdiscovery

import (
	"encoding/json"
	"net/http"
	"testing"

	"code-code.internal/platform-k8s/internal/egressauth"
)

func TestApplyEnvoyAuthHeadersUsesExplicitHeaderFromSimpleRule(t *testing.T) {
	headers, err := applyEnvoyAuthHeaders(http.Header{}, "https://example.test/v1/models", &EnvoyAuthContext{
		ProviderSurfaceBindingID: "surface-1",
		SimpleReplacementRules: []egressauth.SimpleReplacementRule{{
			Mode:       egressauth.SimpleReplacementModeCookie,
			HeaderName: "cookie",
		}},
	})
	if err != nil {
		t.Fatalf("applyEnvoyAuthHeaders() error = %v", err)
	}
	if got := headers.Get("cookie"); got != "" {
		t.Fatalf("Cookie header = %q, want empty", got)
	}
	if got, want := headers.Get(egressauth.HeaderRequestHeaderNames), "cookie"; got != want {
		t.Fatalf("%s = %q, want %q", egressauth.HeaderRequestHeaderNames, got, want)
	}
	if got, want := headers.Get(egressauth.HeaderProviderSurfaceBindingID), "surface-1"; got != want {
		t.Fatalf("%s = %q, want %q", egressauth.HeaderProviderSurfaceBindingID, got, want)
	}
	var rules []egressauth.SimpleReplacementRule
	if err := json.Unmarshal([]byte(headers.Get(egressauth.HeaderRequestHeaderRulesJSON)), &rules); err != nil {
		t.Fatalf("rules JSON is invalid: %v", err)
	}
	if len(rules) != 1 || rules[0].Mode != egressauth.SimpleReplacementModeCookie || rules[0].HeaderName != "cookie" {
		t.Fatalf("rules = %#v", rules)
	}
}
