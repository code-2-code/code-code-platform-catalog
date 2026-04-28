package modelcatalogdiscovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	"code-code.internal/go-contract/domainerror"
	modelcatalogdiscoveryv1 "code-code.internal/go-contract/model_catalog_discovery/v1"
	"code-code.internal/platform-k8s/internal/egressauth"
)

type EnvoyAuthContext struct {
	ProviderSurfaceBindingID string
	RequestHeaderNames       []string
	HeaderValuePrefix        string
	SimpleReplacementRules   []egressauth.SimpleReplacementRule
}

func EnvoyAuthContextForOperation(
	protocol apiprotocolv1.Protocol,
	providerSurfaceBindingID string,
	operation *modelcatalogdiscoveryv1.ModelCatalogDiscoveryOperation,
) (*EnvoyAuthContext, http.Header, error) {
	if auth := envoyAuthContextFromOperation(providerSurfaceBindingID, operation); auth != nil {
		return auth, nil, nil
	}
	if !operationAllowsDefaultAPIKeyAuth(operation) {
		return nil, nil, domainerror.NewValidation("platformk8s/modelcatalogdiscovery: auth-bound discovery requires explicit placeholder request headers")
	}
	headers := http.Header{}
	auth := &EnvoyAuthContext{
		ProviderSurfaceBindingID: strings.TrimSpace(providerSurfaceBindingID),
	}
	switch protocol {
	case apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE:
		auth.RequestHeaderNames = []string{"authorization"}
		auth.HeaderValuePrefix = "Bearer"
		auth.SimpleReplacementRules = []egressauth.SimpleReplacementRule{{
			Mode:       egressauth.SimpleReplacementModeBearer,
			HeaderName: "authorization",
		}}
	case apiprotocolv1.Protocol_PROTOCOL_GEMINI:
		auth.RequestHeaderNames = []string{"x-goog-api-key"}
		auth.SimpleReplacementRules = []egressauth.SimpleReplacementRule{{
			Mode:       egressauth.SimpleReplacementModeGoogleAPIKey,
			HeaderName: "x-goog-api-key",
		}}
	case apiprotocolv1.Protocol_PROTOCOL_ANTHROPIC:
		auth.RequestHeaderNames = []string{"x-api-key"}
		auth.SimpleReplacementRules = []egressauth.SimpleReplacementRule{{
			Mode:       egressauth.SimpleReplacementModeXAPIKey,
			HeaderName: "x-api-key",
		}}
		headers.Set("anthropic-version", anthropicVersionHeaderValue)
	default:
		return nil, nil, domainerror.NewValidation("platformk8s/modelcatalogdiscovery: unsupported api key auth protocol %s", protocol.String())
	}
	return auth, headers, nil
}

func operationAllowsDefaultAPIKeyAuth(operation *modelcatalogdiscoveryv1.ModelCatalogDiscoveryOperation) bool {
	if operation == nil || len(operation.GetSecurity()) == 0 {
		return true
	}
	for _, requirement := range operation.GetSecurity() {
		for _, scheme := range requirement.GetSchemes() {
			if scheme == modelcatalogdiscoveryv1.ModelCatalogDiscoverySecurityScheme_MODEL_CATALOG_DISCOVERY_SECURITY_SCHEME_API_KEY {
				return true
			}
		}
	}
	return false
}

func envoyAuthContextFromOperation(
	providerSurfaceBindingID string,
	operation *modelcatalogdiscoveryv1.ModelCatalogDiscoveryOperation,
) *EnvoyAuthContext {
	if operation == nil {
		return nil
	}
	var headerNames []string
	prefix := ""
	for _, parameter := range operation.GetRequestHeaders() {
		literal := strings.TrimSpace(parameter.GetLiteral())
		if literal == "" || !strings.Contains(literal, egressauth.Placeholder) {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(parameter.GetName()))
		if name == "" {
			continue
		}
		headerNames = append(headerNames, name)
		nextPrefix := strings.TrimSpace(strings.TrimSuffix(literal, egressauth.Placeholder))
		if prefix == "" {
			prefix = nextPrefix
			continue
		}
		if nextPrefix != "" && nextPrefix != prefix {
			return nil
		}
	}
	if len(headerNames) == 0 {
		return nil
	}
	return &EnvoyAuthContext{
		ProviderSurfaceBindingID: strings.TrimSpace(providerSurfaceBindingID),
		RequestHeaderNames:       headerNames,
		HeaderValuePrefix:        prefix,
		SimpleReplacementRules:   simpleRulesForHeaders(headerNames, prefix),
	}
}

func applyEnvoyAuthHeaders(headers http.Header, rawTargetURL string, auth *EnvoyAuthContext) (http.Header, error) {
	if auth == nil {
		return headers, nil
	}
	providerSurfaceBindingID := strings.TrimSpace(auth.ProviderSurfaceBindingID)
	if providerSurfaceBindingID == "" {
		return nil, domainerror.NewValidation("platformk8s/modelcatalogdiscovery: envoy auth provider surface binding id is required")
	}
	target, err := url.Parse(rawTargetURL)
	if err != nil || target == nil || strings.TrimSpace(target.Host) == "" {
		return nil, domainerror.NewValidation("platformk8s/modelcatalogdiscovery: envoy auth target URL is invalid")
	}
	out := cloneHeaders(headers)
	headerNames := normalizedHeaderNames(auth.RequestHeaderNames)
	rules := normalizedSimpleRules(auth.SimpleReplacementRules, headerNames, auth.HeaderValuePrefix)
	if len(headerNames) == 0 {
		for _, rule := range rules {
			if name := strings.ToLower(strings.TrimSpace(rule.HeaderName)); name != "" {
				headerNames = append(headerNames, name)
			}
		}
		headerNames = normalizedHeaderNames(headerNames)
	}
	if len(headerNames) == 0 {
		return nil, domainerror.NewValidation("platformk8s/modelcatalogdiscovery: envoy auth request headers are required")
	}
	for _, name := range headerNames {
		out.Del(textproto.CanonicalMIMEHeaderKey(name))
	}
	out.Set(egressauth.HeaderProviderSurfaceBindingID, providerSurfaceBindingID)
	out.Set(egressauth.HeaderTargetHosts, target.Host)
	out.Set(egressauth.HeaderRequestHeaderNames, strings.Join(headerNames, ","))
	if prefix := strings.TrimSpace(auth.HeaderValuePrefix); prefix != "" {
		out.Set(egressauth.HeaderHeaderValuePrefix, prefix)
	}
	if len(rules) > 0 {
		payload, err := json.Marshal(rules)
		if err != nil {
			return nil, fmt.Errorf("platformk8s/modelcatalogdiscovery: marshal envoy auth rules: %w", err)
		}
		out.Set(egressauth.HeaderRequestHeaderRulesJSON, string(payload))
	}
	return out, nil
}

func normalizedHeaderNames(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func simpleRulesForHeaders(headerNames []string, prefix string) []egressauth.SimpleReplacementRule {
	out := make([]egressauth.SimpleReplacementRule, 0, len(headerNames))
	for _, name := range normalizedHeaderNames(headerNames) {
		rule := egressauth.SimpleReplacementRule{HeaderName: name}
		switch name {
		case "authorization":
			rule.Mode = egressauth.SimpleReplacementModeBearer
			if strings.TrimSpace(prefix) != "" {
				rule.HeaderValuePrefix = strings.TrimSpace(prefix)
			}
		case "x-goog-api-key":
			rule.Mode = egressauth.SimpleReplacementModeGoogleAPIKey
		case "x-api-key":
			rule.Mode = egressauth.SimpleReplacementModeXAPIKey
		case "cookie":
			rule.Mode = egressauth.SimpleReplacementModeCookie
		default:
			continue
		}
		out = append(out, egressauth.NormalizeSimpleReplacementRule(rule))
	}
	return out
}

func normalizedSimpleRules(
	rules []egressauth.SimpleReplacementRule,
	headerNames []string,
	prefix string,
) []egressauth.SimpleReplacementRule {
	if len(rules) == 0 {
		return simpleRulesForHeaders(headerNames, prefix)
	}
	out := make([]egressauth.SimpleReplacementRule, 0, len(rules))
	for _, rule := range rules {
		rule = egressauth.NormalizeSimpleReplacementRule(rule)
		if strings.TrimSpace(rule.HeaderName) == "" {
			continue
		}
		out = append(out, rule)
	}
	return out
}
