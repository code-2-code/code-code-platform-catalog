package supportservice

import (
	"context"
	"fmt"
	"strings"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	credentialv1 "code-code.internal/go-contract/credential/v1"
	supportv1 "code-code.internal/go-contract/platform/support/v1"
	clisupport "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/support"
	vendorsupport "code-code.internal/platform-k8s/internal/supportservice/vendors/support"
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
	case *supportv1.ResolveProviderCapabilitiesRequest_Runtime:
		return s.resolveRuntimeCapabilitySubject(ctx, subject.Runtime)
	default:
		return nil, fmt.Errorf("platformk8s/supportservice: capability subject is required")
	}
}

func (s *Server) resolveRuntimeCapabilitySubject(ctx context.Context, subject *supportv1.RuntimeCapabilitySubject) (*supportv1.ResolveProviderCapabilitiesResponse, error) {
	if subject == nil {
		return nil, fmt.Errorf("platformk8s/supportservice: runtime capability subject is required")
	}
	response, err := s.resolveProviderCapabilitySubject(ctx, &supportv1.ProviderCapabilitySubject{
		ProviderId:       subject.GetProviderId(),
		SurfaceId:        subject.GetSurfaceId(),
		Protocol:         subject.GetProtocol(),
		Model:            subject.GetModel(),
		CredentialKind:   subject.GetCredentialKind(),
		RuntimeUrl:       subject.GetRuntimeUrl(),
		ExecutionContext: subject.GetExecutionContext(),
	})
	if err != nil {
		return nil, err
	}
	if key := strings.TrimSpace(subject.GetAuthMaterializationKey()); key != "" {
		response.AuthMaterializationKey = key
	}
	return response, nil
}

func (s *Server) resolveProviderCapabilitySubject(ctx context.Context, subject *supportv1.ProviderCapabilitySubject) (*supportv1.ResolveProviderCapabilitiesResponse, error) {
	if subject == nil {
		return nil, fmt.Errorf("platformk8s/supportservice: provider capability subject is required")
	}
	providerID := strings.TrimSpace(subject.GetProviderId())
	surfaceID := strings.TrimSpace(subject.GetSurfaceId())
	protocol := subject.GetProtocol()
	credentialKind := subject.GetCredentialKind()
	if providerID == "" && surfaceID == "" {
		return s.resolveProtocolCapability(protocol, credentialKind), nil
	}
	if cli, ok, err := s.cliByID(ctx, providerID); err != nil {
		return nil, err
	} else if ok {
		return s.resolveCLICapability(cli, protocol, credentialKind), nil
	}
	if vendor, binding, ok, err := s.vendorBinding(ctx, providerID, surfaceID, protocol); err != nil {
		return nil, err
	} else if ok {
		return resolveVendorSurfaceCapability(vendor, binding), nil
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

func (s *Server) vendorBinding(ctx context.Context, providerID string, surfaceID string, protocol apiprotocolv1.Protocol) (*supportv1.Vendor, *supportv1.VendorProviderBinding, bool, error) {
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
		if binding := selectVendorBinding(vendor, surfaceID, protocol); binding != nil {
			return vendor, binding, true, nil
		}
	}
	if providerID != "" {
		for _, vendor := range items {
			if binding := selectVendorBinding(vendor, surfaceID, protocol); binding != nil {
				return vendor, binding, true, nil
			}
		}
	}
	return nil, nil, false, nil
}

func selectVendorBinding(vendor *supportv1.Vendor, surfaceID string, protocol apiprotocolv1.Protocol) *supportv1.VendorProviderBinding {
	for _, binding := range vendor.GetProviderBindings() {
		if surfaceID != "" && vendorsupport.BindingSurfaceID(binding) != surfaceID {
			continue
		}
		if protocol != apiprotocolv1.Protocol_PROTOCOL_UNSPECIFIED && !vendorBindingSupportsProtocol(binding, protocol) {
			continue
		}
		return binding
	}
	return nil
}

func vendorBindingSupportsProtocol(binding *supportv1.VendorProviderBinding, protocol apiprotocolv1.Protocol) bool {
	for _, template := range binding.GetSurfaceTemplates() {
		if template.GetRuntime().GetApi().GetProtocol() == protocol {
			return true
		}
	}
	return false
}

func resolveVendorSurfaceCapability(vendor *supportv1.Vendor, binding *supportv1.VendorProviderBinding) *supportv1.ResolveProviderCapabilitiesResponse {
	config := binding.GetProviderBinding()
	policyID := strings.TrimSpace(config.GetEgressPolicyId())
	observability := passiveHTTPObservability(binding.GetObservability())
	return &supportv1.ResolveProviderCapabilitiesResponse{
		EgressPolicyId:      policyID,
		AuthPolicyId:        strings.TrimSpace(config.GetHeaderRewritePolicyId()),
		Observability:       observability,
		ModelCatalogProbeId: strings.TrimSpace(config.GetModelCatalogProbeId()),
		QuotaProbeId:        strings.TrimSpace(config.GetQuotaProbeId()),
		ResourceRef:         strings.TrimSpace(vendor.GetVendor().GetVendorId()),
	}
}

func (s *Server) resolveCLICapability(cli *supportv1.CLI, protocol apiprotocolv1.Protocol, credentialKind credentialv1.CredentialKind) *supportv1.ResolveProviderCapabilitiesResponse {
	if credentialKind == credentialv1.CredentialKind_CREDENTIAL_KIND_OAUTH ||
		(credentialKind == credentialv1.CredentialKind_CREDENTIAL_KIND_UNSPECIFIED && cli.GetOauth() != nil) {
		materialization, _ := clisupport.ResolveAuthMaterialization(cli, credentialv1.CredentialKind_CREDENTIAL_KIND_OAUTH, protocol)
		policyID := clisupport.OAuthEgressPolicyID(cli)
		return &supportv1.ResolveProviderCapabilitiesResponse{
			EgressPolicyId:         policyID,
			AuthPolicyId:           clisupport.OAuthHeaderRewritePolicyID(cli),
			Observability:          passiveHTTPObservability(cli.GetOauth().GetObservability()),
			ModelCatalogProbeId:    clisupport.OAuthModelCatalogProbeID(cli),
			QuotaProbeId:           clisupport.OAuthQuotaProbeID(cli),
			AuthMaterializationKey: strings.TrimSpace(materialization.GetMaterializationKey()),
			ResourceRef:            strings.TrimSpace(cli.GetCliId()),
		}
	}
	return s.resolveProtocolCapability(protocol, credentialKind)
}

func (s *Server) resolveProtocolCapability(protocol apiprotocolv1.Protocol, credentialKind credentialv1.CredentialKind) *supportv1.ResolveProviderCapabilitiesResponse {
	protocolID := protocolPolicyID(protocol)
	if credentialKind == credentialv1.CredentialKind_CREDENTIAL_KIND_OAUTH {
		return &supportv1.ResolveProviderCapabilitiesResponse{
			EgressPolicyId:         protocolID,
			AuthPolicyId:           protocolID + ".oauth",
			Observability:          s.protocolRuntimeTelemetry(protocol),
			ModelCatalogProbeId:    protocolSurfaceID(protocol),
			AuthMaterializationKey: protocolID + ".oauth",
		}
	}
	return &supportv1.ResolveProviderCapabilitiesResponse{
		EgressPolicyId:         protocolID,
		AuthPolicyId:           protocolID + ".api-key",
		Observability:          s.protocolRuntimeTelemetry(protocol),
		ModelCatalogProbeId:    protocolSurfaceID(protocol),
		AuthMaterializationKey: protocolID + ".api-key",
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
	response.ResourceRef = "custom.api"
	return response
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
