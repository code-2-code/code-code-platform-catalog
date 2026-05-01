package supportservice

import (
	"context"
	"fmt"
	"strings"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	credentialv1 "code-code.internal/go-contract/credential/v1"
	supportv1 "code-code.internal/go-contract/platform/support/v1"
	"code-code.internal/platform-k8s/internal/platform/providersurfaces/registry"
	vendorsupport "code-code.internal/platform-k8s/internal/platform/vendors/support"
)

func (s *Server) ResolveProviderCapabilities(ctx context.Context, request *supportv1.ResolveProviderCapabilitiesRequest) (*supportv1.ResolveProviderCapabilitiesResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("platformk8s/supportservice: capability subject is required")
	}
	switch subject := request.GetSubject().(type) {
	case *supportv1.ResolveProviderCapabilitiesRequest_Provider:
		return s.resolveProviderCapabilitySubject(ctx, subject.Provider)
	case *supportv1.ResolveProviderCapabilitiesRequest_CustomApi:
		return s.resolveCustomAPICapabilitySubject(subject.CustomApi), nil
	default:
		return nil, fmt.Errorf("platformk8s/supportservice: capability subject is required")
	}
}

func (s *Server) resolveProviderCapabilitySubject(ctx context.Context, subject *supportv1.ProviderCapabilitySubject) (*supportv1.ResolveProviderCapabilitiesResponse, error) {
	if subject == nil {
		return nil, fmt.Errorf("platformk8s/supportservice: provider capability subject is required")
	}
	providerID := strings.TrimSpace(subject.GetProviderId())
	surfaceID := strings.TrimSpace(subject.GetSurfaceId())
	protocol := subject.GetEndpoint().GetApi().GetProtocol()
	credentialKind := subject.GetCredentialKind()
	if surfaceID == registry.SurfaceIDCustomAPIKey {
		return s.resolveCustomAPICapabilitySubject(&supportv1.CustomAPICapabilitySubject{
			BaseUrl:          subject.GetEndpoint().GetApi().GetBaseUrl(),
			Protocol:         protocol,
			CredentialKind:   credentialKind,
			ExecutionContext: subject.GetExecutionContext(),
		}), nil
	}
	if providerID == "" && surfaceID == "" {
		return s.resolveProtocolCapability(protocol, credentialKind), nil
	}
	if cli, ok, err := s.cliByID(ctx, providerID); err != nil {
		return nil, err
	} else if ok {
		_ = cli
	}
	if vendor, surface, ok, err := s.vendorSurface(ctx, providerID, surfaceID, protocol); err != nil {
		return nil, err
	} else if ok {
		return resolveVendorSurfaceCapability(vendor, surface), nil
	}
	return s.resolveProtocolCapability(protocol, credentialKind), nil
}

func (s *Server) cliByID(ctx context.Context, cliID string) (*supportv1.CLI, bool, error) {
	cliID = strings.TrimSpace(cliID)
	if cliID == "" {
		return nil, false, nil
	}
	cli, err := s.clis.Get(ctx, cliID)
	if err == nil {
		return cli, true, nil
	}
	items, listErr := s.clis.List(ctx)
	if listErr != nil {
		return nil, false, listErr
	}
	for _, item := range items {
		if strings.TrimSpace(item.GetCliId()) == cliID {
			return item, true, nil
		}
	}
	return nil, false, nil
}

func (s *Server) vendorSurface(ctx context.Context, providerID string, surfaceID string, protocol apiprotocolv1.Protocol) (*supportv1.Vendor, *supportv1.Surface, bool, error) {
	items, err := s.vendors.List(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	providerID = strings.TrimSpace(providerID)
	surfaceID = strings.TrimSpace(surfaceID)
	for _, vendor := range items {
		if providerID != "" && providerID != strings.TrimSpace(vendor.GetVendor().GetVendorId()) {
			continue
		}
		if surface := selectVendorSurface(vendor, surfaceID, protocol); surface != nil {
			return vendor, surface, true, nil
		}
	}
	if providerID != "" {
		for _, vendor := range items {
			if surface := selectVendorSurface(vendor, surfaceID, protocol); surface != nil {
				return vendor, surface, true, nil
			}
		}
	}
	return nil, nil, false, nil
}

func selectVendorSurface(vendor *supportv1.Vendor, surfaceID string, protocol apiprotocolv1.Protocol) *supportv1.Surface {
	for _, surface := range vendor.GetSurfaces() {
		if surfaceID != "" && strings.TrimSpace(surface.GetSurfaceId()) != surfaceID {
			continue
		}
		if !vendorsupport.SurfaceSupportsProtocol(surface, protocol) {
			continue
		}
		return surface
	}
	return nil
}

func resolveVendorSurfaceCapability(vendor *supportv1.Vendor, surface *supportv1.Surface) *supportv1.ResolveProviderCapabilitiesResponse {
	policyID := vendorsupport.SurfaceEgressPolicyID(vendor, surface)
	return &supportv1.ResolveProviderCapabilitiesResponse{
		EgressPolicyId:        policyID,
		AuthPolicyId:          vendorsupport.SurfaceAuthPolicyID(vendor, surface),
		ObservabilityPolicyId: vendorsupport.SurfaceObservabilityPolicyID(surface),
		ModelCatalogProbeId:   vendorsupport.SurfaceModelCatalogProbeID(surface),
		QuotaProbeId:          vendorsupport.SurfaceQuotaProbeID(surface),
		SurfaceId:             strings.TrimSpace(surface.GetSurfaceId()),
	}
}

func (s *Server) resolveProtocolCapability(protocol apiprotocolv1.Protocol, credentialKind credentialv1.CredentialKind) *supportv1.ResolveProviderCapabilitiesResponse {
	protocolID := protocolPolicyID(protocol)
	if credentialKind == credentialv1.CredentialKind_CREDENTIAL_KIND_OAUTH {
		return &supportv1.ResolveProviderCapabilitiesResponse{
			EgressPolicyId:        protocolID,
			AuthPolicyId:          protocolID + ".oauth",
			ObservabilityPolicyId: protocolID,
			ModelCatalogProbeId:   protocolSurfaceID(protocol),
			SurfaceId:             protocolSurfaceID(protocol),
		}
	}
	return &supportv1.ResolveProviderCapabilitiesResponse{
		EgressPolicyId:        protocolID,
		AuthPolicyId:          protocolID + ".api-key",
		ObservabilityPolicyId: protocolID,
		ModelCatalogProbeId:   protocolSurfaceID(protocol),
		SurfaceId:             protocolSurfaceID(protocol),
	}
}

func (s *Server) resolveCustomAPICapabilitySubject(subject *supportv1.CustomAPICapabilitySubject) *supportv1.ResolveProviderCapabilitiesResponse {
	protocol := apiprotocolv1.Protocol_PROTOCOL_UNSPECIFIED
	credentialKind := credentialv1.CredentialKind_CREDENTIAL_KIND_API_KEY
	if subject != nil {
		protocol = subject.GetProtocol()
		credentialKind = subject.GetCredentialKind()
	}
	response := s.resolveProtocolCapability(protocol, credentialKind)
	response.EgressPolicyId = "custom.api"
	response.SurfaceId = registry.SurfaceIDCustomAPIKey
	response.ModelCatalogProbeId = customAPIModelCatalogProbeID(protocol)
	response.QuotaProbeId = ""
	return response
}

func customAPIModelCatalogProbeID(protocol apiprotocolv1.Protocol) string {
	switch protocol {
	case apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE, apiprotocolv1.Protocol_PROTOCOL_OPENAI_RESPONSES:
		return "surface.openai-compatible"
	default:
		return ""
	}
}

func protocolPolicyID(protocol apiprotocolv1.Protocol) string {
	switch protocol {
	case apiprotocolv1.Protocol_PROTOCOL_ANTHROPIC:
		return "protocol.anthropic"
	case apiprotocolv1.Protocol_PROTOCOL_GEMINI:
		return "protocol.gemini"
	case apiprotocolv1.Protocol_PROTOCOL_OPENAI_RESPONSES:
		return "protocol.openai-responses"
	case apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE:
		return "protocol.openai-compatible"
	default:
		return "protocol.default"
	}
}

func protocolSurfaceID(protocol apiprotocolv1.Protocol) string {
	switch protocol {
	case apiprotocolv1.Protocol_PROTOCOL_ANTHROPIC:
		return "anthropic"
	case apiprotocolv1.Protocol_PROTOCOL_GEMINI:
		return "gemini"
	case apiprotocolv1.Protocol_PROTOCOL_OPENAI_RESPONSES, apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE:
		return "openai-compatible"
	default:
		return ""
	}
}
