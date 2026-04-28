package registry

import (
	"fmt"
	"slices"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	credentialv1 "code-code.internal/go-contract/credential/v1"
	providerv1 "code-code.internal/go-contract/provider/v1"
	"google.golang.org/protobuf/proto"
)

const (
	SurfaceIDOpenAICompatible = "openai-compatible"
	SurfaceIDAnthropic        = "anthropic"
	SurfaceIDGemini           = "gemini"
)

var builtinSurfaces = map[string]*providerv1.ProviderSurface{
	SurfaceIDOpenAICompatible: {
		SurfaceId:   SurfaceIDOpenAICompatible,
		DisplayName: "OpenAI Compatible",
		SupportedCredentialKinds: []credentialv1.CredentialKind{
			credentialv1.CredentialKind_CREDENTIAL_KIND_API_KEY,
			credentialv1.CredentialKind_CREDENTIAL_KIND_OAUTH,
		},
		Kind: providerv1.ProviderSurfaceKind_PROVIDER_SURFACE_KIND_API,
		Api: &providerv1.ProviderSurfaceAPISpec{
			SupportedProtocols: []apiprotocolv1.Protocol{
				apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE,
				apiprotocolv1.Protocol_PROTOCOL_OPENAI_RESPONSES,
			},
		},
		Probes: &providerv1.ProviderSurfaceProbes{
			ModelCatalog: &providerv1.ProviderSurfaceModelCatalogProbe{
				Method: providerv1.ProviderSurfaceModelCatalogProbeMethod_PROVIDER_SURFACE_MODEL_CATALOG_PROBE_METHOD_PROTOCOL_BEST_EFFORT,
			},
			Quota: &providerv1.ProviderSurfaceQuotaProbe{
				SchemaId:    "openai-compatible-session",
				DisplayName: "Quota Session",
				Description: "Optional session token used by OpenAI-compatible active quota collectors that do not accept the primary API key.",
				Args: []*providerv1.ProviderSurfaceProbeArg{{
					ArgId:       "session_token",
					DisplayName: "Session Token",
					Description: "Paste the browser or console session token when quota probing requires it.",
					Placeholder: "Paste session token",
					Sensitive:   true,
				}},
			},
		},
		Capabilities: &providerv1.ProviderCapabilities{
			SupportsModelOverride:     false,
			SupportsModelCatalogProbe: true,
		},
	},
	SurfaceIDAnthropic: {
		SurfaceId:   SurfaceIDAnthropic,
		DisplayName: "Anthropic",
		SupportedCredentialKinds: []credentialv1.CredentialKind{
			credentialv1.CredentialKind_CREDENTIAL_KIND_API_KEY,
			credentialv1.CredentialKind_CREDENTIAL_KIND_OAUTH,
		},
		Kind: providerv1.ProviderSurfaceKind_PROVIDER_SURFACE_KIND_API,
		Api: &providerv1.ProviderSurfaceAPISpec{
			SupportedProtocols: []apiprotocolv1.Protocol{apiprotocolv1.Protocol_PROTOCOL_ANTHROPIC},
		},
		Probes: &providerv1.ProviderSurfaceProbes{
			ModelCatalog: &providerv1.ProviderSurfaceModelCatalogProbe{
				Method: providerv1.ProviderSurfaceModelCatalogProbeMethod_PROVIDER_SURFACE_MODEL_CATALOG_PROBE_METHOD_PROTOCOL_BEST_EFFORT,
			},
		},
		Capabilities: &providerv1.ProviderCapabilities{
			SupportsModelOverride:     true,
			SupportsModelCatalogProbe: true,
		},
	},
	SurfaceIDGemini: {
		SurfaceId:   SurfaceIDGemini,
		DisplayName: "Gemini",
		SupportedCredentialKinds: []credentialv1.CredentialKind{
			credentialv1.CredentialKind_CREDENTIAL_KIND_API_KEY,
			credentialv1.CredentialKind_CREDENTIAL_KIND_OAUTH,
		},
		Kind: providerv1.ProviderSurfaceKind_PROVIDER_SURFACE_KIND_API,
		Api: &providerv1.ProviderSurfaceAPISpec{
			SupportedProtocols: []apiprotocolv1.Protocol{apiprotocolv1.Protocol_PROTOCOL_GEMINI},
		},
		Probes: &providerv1.ProviderSurfaceProbes{
			ModelCatalog: &providerv1.ProviderSurfaceModelCatalogProbe{
				Method: providerv1.ProviderSurfaceModelCatalogProbeMethod_PROVIDER_SURFACE_MODEL_CATALOG_PROBE_METHOD_PROTOCOL_BEST_EFFORT,
			},
			Quota: &providerv1.ProviderSurfaceQuotaProbe{
				SchemaId:    "google-ai-studio-session",
				DisplayName: "AI Studio Session",
				Description: "Paste Google AI Studio browser session fields used by active quota queries.",
				Args: []*providerv1.ProviderSurfaceProbeArg{
					{
						ArgId:       "cookie",
						DisplayName: "Request Cookie",
						Description: "Copy the Cookie request header from ListModelRateLimits.",
						Placeholder: "SID=...; HSID=...; SSID=...; SAPISID=...",
						Required:    true,
						Sensitive:   true,
						Multiline:   true,
					},
					{
						ArgId:       "response_set_cookie",
						DisplayName: "Response Set-Cookie",
						Description: "Optional. Paste Set-Cookie response headers from the same request, one per line.",
						Placeholder: "Set-Cookie: SID=...\nSet-Cookie: HSID=...",
						Multiline:   true,
					},
					{
						ArgId:       "authorization",
						DisplayName: "Request Authorization",
						Description: "Optional. Copy the Authorization request header from the same ListModelRateLimits request.",
						Placeholder: "SAPISIDHASH ...",
						Sensitive:   true,
					},
					{
						ArgId:       "page_api_key",
						DisplayName: "X-Goog-Api-Key",
						Placeholder: "AIzaSy...",
						Required:    true,
					},
					{
						ArgId:       "project_id",
						DisplayName: "Project Number",
						Description: "Use the projects/<number> value or plain project number from the ListModelRateLimits request body.",
						Placeholder: "946397203396 or projects/946397203396",
						Required:    true,
					},
				},
			},
		},
		Capabilities: &providerv1.ProviderCapabilities{
			SupportsModelOverride:     false,
			SupportsModelCatalogProbe: true,
		},
	},
}

func Get(surfaceID string) (*providerv1.ProviderSurface, error) {
	item, ok := builtinSurfaces[surfaceID]
	if !ok {
		return nil, fmt.Errorf("platformk8s/providersurfaces/registry: provider surface %q not found", surfaceID)
	}
	return clone(item), nil
}

func MustGet(surfaceID string) *providerv1.ProviderSurface {
	item, err := Get(surfaceID)
	if err != nil {
		panic(err)
	}
	return item
}

func List() []*providerv1.ProviderSurface {
	items := make([]*providerv1.ProviderSurface, 0, len(builtinSurfaces))
	for _, item := range builtinSurfaces {
		items = append(items, clone(item))
	}
	slices.SortFunc(items, func(left, right *providerv1.ProviderSurface) int {
		switch {
		case left.GetSurfaceId() < right.GetSurfaceId():
			return -1
		case left.GetSurfaceId() > right.GetSurfaceId():
			return 1
		default:
			return 0
		}
	})
	return items
}

func clone(surface *providerv1.ProviderSurface) *providerv1.ProviderSurface {
	if surface == nil {
		return nil
	}
	return proto.Clone(surface).(*providerv1.ProviderSurface)
}
