package providersurfaces

import (
	"slices"
	"testing"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	credentialv1 "code-code.internal/go-contract/credential/v1"
	providerv1 "code-code.internal/go-contract/provider/v1"
	clisupport "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/support"
	vendorsupport "code-code.internal/platform-k8s/internal/supportservice/vendors/support"
)

func TestServiceListReturnsBuiltinSurfaces(t *testing.T) {
	t.Parallel()

	cliSupport, err := clisupport.NewManagementService()
	if err != nil {
		t.Fatalf("NewManagementService() error = %v", err)
	}
	vendorSupport, err := vendorsupport.NewManagementService()
	if err != nil {
		t.Fatalf("NewManagementService() vendor error = %v", err)
	}
	service, err := NewService(cliSupport, vendorSupport)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	items, err := service.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) < 3 {
		t.Fatalf("List() len = %d, want at least 3", len(items))
	}
	if got, want := items[0].GetSurfaceId(), "anthropic"; got != want {
		t.Fatalf("first surface = %q, want %q", got, want)
	}
	openAICompatibleSurface, err := service.Get(t.Context(), "openai-compatible")
	if err != nil {
		t.Fatalf("Get() openai-compatible error = %v", err)
	}
	if got, want := openAICompatibleSurface.GetSupportedCredentialKinds()[0], credentialv1.CredentialKind_CREDENTIAL_KIND_API_KEY; got != want {
		t.Fatalf("surface credential kind = %v, want %v", got, want)
	}
	if !slices.Contains(openAICompatibleSurface.GetApi().GetSupportedProtocols(), apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE) {
		t.Fatalf("surface api protocols = %v, want contains %v", openAICompatibleSurface.GetApi().GetSupportedProtocols(), apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE)
	}
	if got, want := openAICompatibleSurface.GetKind(), providerv1.ProviderSurfaceKind_PROVIDER_SURFACE_KIND_API; got != want {
		t.Fatalf("surface kind = %v, want %v", got, want)
	}
	if !openAICompatibleSurface.GetCapabilities().GetSupportsQuotaProbe() {
		t.Fatal("openai-compatible supports_quota_probe = false, want true")
	}
	geminiSurface, err := service.Get(t.Context(), "gemini")
	if err != nil {
		t.Fatalf("Get() gemini error = %v", err)
	}
	if !geminiSurface.GetCapabilities().GetSupportsModelCatalogProbe() {
		t.Fatal("gemini supports_model_catalog_probe = false, want true")
	}
	anthropicSurface, err := service.Get(t.Context(), "anthropic")
	if err != nil {
		t.Fatalf("Get() vendor error = %v", err)
	}
	if !anthropicSurface.GetCapabilities().GetSupportsModelCatalogProbe() {
		t.Fatal("anthropic supports_model_catalog_probe = false, want true")
	}
}
