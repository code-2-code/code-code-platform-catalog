package templates

import (
	"errors"
	"testing"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	"code-code.internal/go-contract/domainerror"
	managementv1 "code-code.internal/go-contract/platform/management/v1"
	providerv1 "code-code.internal/go-contract/provider/v1"
)

func TestLoadTemplatesReturnsViews(t *testing.T) {
	templates, err := loadTemplates()
	if err != nil {
		t.Fatalf("loadTemplates() error = %v", err)
	}
	expectedIDs := []string{
		"minimax-anthropic",
		"minimax-openai-compatible",
		"mistral-openai-compatible",
	}
	if got, want := len(templates), len(expectedIDs); got < want {
		t.Fatalf("loadTemplates() count = %d, want at least %d", got, want)
	}
	for _, templateID := range expectedIDs {
		item, ok := templates[templateID]
		if !ok {
			t.Fatalf("template %q missing from loadTemplates()", templateID)
		}
		if item.view.GetTemplateId() != templateID {
			t.Fatalf("template %q view.TemplateId = %q", templateID, item.view.GetTemplateId())
		}
		if item.view.GetProtocol() == "" {
			t.Fatalf("template %q protocol is empty", templateID)
		}
		if item.view.GetDefaultBaseUrl() == "" {
			t.Fatalf("template %q default base URL is empty", templateID)
		}
		if got := len(item.view.GetDefaultModels()); got == 0 {
			t.Fatalf("template %q default models = 0, want non-empty", templateID)
		}
	}
}

func TestBuildTemplateInstanceAppliesOverrides(t *testing.T) {
	item := manifestTemplate{
		view: &managementv1.TemplateView{
			TemplateId: "vendor-protocol",
		},
		provider: &providerv1.Provider{
			ProviderId:  "provider-account-template",
			DisplayName: "Vendor Protocol",
			Surfaces: []*providerv1.ProviderSurfaceBinding{{
				SurfaceId: "vendor-protocol",
				Runtime: &providerv1.ProviderSurfaceRuntime{
					DisplayName: "vendor-protocol",
					Origin:      providerv1.ProviderSurfaceOrigin_PROVIDER_SURFACE_ORIGIN_DERIVED,
					Access: &providerv1.ProviderSurfaceRuntime_Api{
						Api: &providerv1.ProviderAPISurfaceRuntime{
							Protocol: apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE,
							BaseUrl:  "https://api.example.com/v1",
						},
					},
					Catalog: &providerv1.ProviderModelCatalog{
						Source: providerv1.CatalogSource_CATALOG_SOURCE_FALLBACK_CONFIG,
						Models: []*providerv1.ProviderModelCatalogEntry{{ProviderModelId: "model-a"}},
					},
				},
				ProviderCredentialRef: &providerv1.ProviderCredentialRef{ProviderCredentialId: "existing"},
			}},
		},
	}

	provider, err := buildTemplateProvider(item, &managementv1.ApplyTemplateRequest{
		Namespace:       "code-code",
		DisplayName:     "Vendor Protocol",
		ProviderId:      "custom-provider",
		AllowedModelIds: []string{"model-b", "model-c"},
	}, []string{"model-b", "model-c"})
	if err != nil {
		t.Fatalf("buildTemplateProvider() error = %v", err)
	}
	if provider.ProviderId != "custom-provider" {
		t.Fatalf("ProviderId = %q, want custom-provider", provider.ProviderId)
	}
	endpoint := provider.GetSurfaces()[0]
	if endpoint.GetProviderCredentialRef() != nil {
		t.Fatalf("ProviderCredentialRef = %#v, want nil when request omits provider credential", endpoint.GetProviderCredentialRef())
	}
	if got := len(endpoint.GetRuntime().GetCatalog().GetModels()); got != 2 {
		t.Fatalf("Catalog.Models len = %d, want 2", got)
	}
}

func TestDedupeModelIDsRejectsDuplicateEntries(t *testing.T) {
	t.Parallel()

	_, err := dedupeModelIDs([]string{"model-a", "model-a"})
	if err == nil {
		t.Fatal("dedupeModelIDs() error = nil, want duplicate error")
	}
	var validationErr *domainerror.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("dedupeModelIDs() error = %T, want ValidationError", err)
	}
}

func TestTemplateApplyRejectsUnknownTemplate(t *testing.T) {
	t.Parallel()

	service := &TemplateManagementService{
		templates: map[string]manifestTemplate{},
	}
	_, err := service.Apply(t.Context(), &managementv1.ApplyTemplateRequest{
		TemplateId:  "missing",
		Namespace:   "code-code",
		DisplayName: "Missing",
	})
	if err == nil {
		t.Fatal("Apply() error = nil, want not found error")
	}
	var notFoundErr *domainerror.NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("Apply() error = %T, want NotFoundError", err)
	}
}
