package supportservice

import (
	"context"
	"testing"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	credentialv1 "code-code.internal/go-contract/credential/v1"
	observabilityv1 "code-code.internal/go-contract/observability/v1"
	supportv1 "code-code.internal/go-contract/platform/support/v1"
	"code-code.internal/platform-k8s/internal/platform/testutil"
)

func TestResolveProviderCapabilitiesReturnsProtocolPassiveHTTPObservability(t *testing.T) {
	server := newTestSupportServer(t)

	response, err := server.ResolveProviderCapabilities(context.Background(), &supportv1.ResolveProviderCapabilitiesRequest{
		Subject: &supportv1.ResolveProviderCapabilitiesRequest_CustomApi{CustomApi: &supportv1.CustomAPICapabilitySubject{
			Protocol:       apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE,
			CredentialKind: credentialv1.CredentialKind_CREDENTIAL_KIND_API_KEY,
		}},
	})
	if err != nil {
		t.Fatalf("ResolveProviderCapabilities() error = %v", err)
	}
	if !capabilityHasHeader(response.GetObservability(), "x-ratelimit-remaining-requests") {
		t.Fatalf("observability passive http headers = %#v, want x-ratelimit-remaining-requests", response.GetObservability())
	}
}

func TestResolveProviderCapabilitiesReturnsVendorPassiveHTTPObservability(t *testing.T) {
	server := newTestSupportServer(t)

	response, err := server.ResolveProviderCapabilities(context.Background(), &supportv1.ResolveProviderCapabilitiesRequest{
		Subject: &supportv1.ResolveProviderCapabilitiesRequest_Provider{Provider: &supportv1.ProviderCapabilitySubject{
			ProviderId:     "openrouter",
			SurfaceId:      "openai-compatible",
			Protocol:       apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE,
			CredentialKind: credentialv1.CredentialKind_CREDENTIAL_KIND_API_KEY,
		}},
	})
	if err != nil {
		t.Fatalf("ResolveProviderCapabilities() error = %v", err)
	}
	if !capabilityHasHeader(response.GetObservability(), "x-ratelimit-remaining-tokens") {
		t.Fatalf("observability passive http headers = %#v, want vendor token rate-limit header", response.GetObservability())
	}
}

func TestResolveProviderCapabilitiesReturnsCLIOAuthPassiveHTTPObservability(t *testing.T) {
	server := newTestSupportServer(t)

	response, err := server.ResolveProviderCapabilities(context.Background(), &supportv1.ResolveProviderCapabilitiesRequest{
		Subject: &supportv1.ResolveProviderCapabilitiesRequest_Provider{Provider: &supportv1.ProviderCapabilitySubject{
			ProviderId:     "codex",
			Protocol:       apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE,
			CredentialKind: credentialv1.CredentialKind_CREDENTIAL_KIND_OAUTH,
		}},
	})
	if err != nil {
		t.Fatalf("ResolveProviderCapabilities() error = %v", err)
	}
	if !capabilityHasHeader(response.GetObservability(), "retry-after") {
		t.Fatalf("observability passive http headers = %#v, want retry-after", response.GetObservability())
	}
}

func newTestSupportServer(t *testing.T) *Server {
	t.Helper()
	server, err := NewServer(Config{
		Reader:    testutil.NewEmptyClient(),
		Namespace: "code-code",
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return server
}

func capabilityHasHeader(capability *observabilityv1.ObservabilityCapability, header string) bool {
	for _, profile := range capability.GetProfiles() {
		for _, transform := range profile.GetPassiveHttp().GetTransforms() {
			if transform.GetHeaderName() == header {
				return true
			}
		}
	}
	return false
}
