package supportservice

import (
	"context"
	"testing"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	credentialv1 "code-code.internal/go-contract/credential/v1"
	supportv1 "code-code.internal/go-contract/platform/support/v1"
	providerv1 "code-code.internal/go-contract/provider/v1"
	"code-code.internal/platform-k8s/internal/platform/testutil"
)

func TestResolveProviderCapabilitiesReturnsProtocolPolicyIDs(t *testing.T) {
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
	if got, want := response.GetAuthPolicyId(), "protocol.openai-compatible.api-key"; got != want {
		t.Fatalf("auth_policy_id = %q, want %q", got, want)
	}
	if got, want := response.GetEgressPolicyId(), "custom.api"; got != want {
		t.Fatalf("egress_policy_id = %q, want %q", got, want)
	}
	if got, want := response.GetModelCatalogProbeId(), "surface.openai-compatible"; got != want {
		t.Fatalf("model_catalog_probe_id = %q, want %q", got, want)
	}
	if got := response.GetQuotaProbeId(); got != "" {
		t.Fatalf("quota_probe_id = %q, want empty", got)
	}
}

func TestResolveProviderCapabilitiesReturnsCustomAPIPolicyForCustomSurface(t *testing.T) {
	server := newTestSupportServer(t)

	response, err := server.ResolveProviderCapabilities(context.Background(), &supportv1.ResolveProviderCapabilitiesRequest{
		Subject: &supportv1.ResolveProviderCapabilitiesRequest_Provider{Provider: &supportv1.ProviderCapabilitySubject{
			ProviderId: "custom-provider",
			SurfaceId:  "custom.api",
			Endpoint: &providerv1.ProviderEndpoint{
				Type: providerv1.ProviderEndpointType_PROVIDER_ENDPOINT_TYPE_API,
				Shape: &providerv1.ProviderEndpoint_Api{Api: &providerv1.ProviderApiEndpoint{
					Protocol: apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE,
					BaseUrl:  "https://api.custom.example/v1",
				}},
			},
			CredentialKind: credentialv1.CredentialKind_CREDENTIAL_KIND_API_KEY,
		}},
	})
	if err != nil {
		t.Fatalf("ResolveProviderCapabilities() error = %v", err)
	}
	if got, want := response.GetSurfaceId(), "custom.api"; got != want {
		t.Fatalf("surface_id = %q, want %q", got, want)
	}
	if got, want := response.GetEgressPolicyId(), "custom.api"; got != want {
		t.Fatalf("egress_policy_id = %q, want %q", got, want)
	}
	if got, want := response.GetModelCatalogProbeId(), "surface.openai-compatible"; got != want {
		t.Fatalf("model_catalog_probe_id = %q, want %q", got, want)
	}
	if got := response.GetQuotaProbeId(); got != "" {
		t.Fatalf("quota_probe_id = %q, want empty", got)
	}
}

func TestResolveProviderCapabilitiesReturnsVendorSurfacePolicyIDs(t *testing.T) {
	server := newTestSupportServer(t)

	response, err := server.ResolveProviderCapabilities(context.Background(), &supportv1.ResolveProviderCapabilitiesRequest{
		Subject: &supportv1.ResolveProviderCapabilitiesRequest_Provider{Provider: &supportv1.ProviderCapabilitySubject{
			ProviderId: "openrouter",
			SurfaceId:  "openai-compatible",
			Endpoint: &providerv1.ProviderEndpoint{
				Type: providerv1.ProviderEndpointType_PROVIDER_ENDPOINT_TYPE_API,
				Shape: &providerv1.ProviderEndpoint_Api{Api: &providerv1.ProviderApiEndpoint{
					Protocol: apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE,
					BaseUrl:  "https://openrouter.ai/api/v1",
				}},
			},
			CredentialKind: credentialv1.CredentialKind_CREDENTIAL_KIND_API_KEY,
		}},
	})
	if err != nil {
		t.Fatalf("ResolveProviderCapabilities() error = %v", err)
	}
	if response.GetAuthPolicyId() == "" {
		t.Fatal("auth_policy_id is empty")
	}
	if response.GetEgressPolicyId() == "" {
		t.Fatal("egress_policy_id is empty")
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
