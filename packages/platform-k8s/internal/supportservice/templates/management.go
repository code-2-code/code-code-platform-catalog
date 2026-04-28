package templates

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"slices"
	"strings"

	"code-code.internal/go-contract/domainerror"
	managementv1 "code-code.internal/go-contract/platform/management/v1"
	providerv1 "code-code.internal/go-contract/provider/v1"
	platformv1alpha1 "code-code.internal/platform-k8s/api/v1alpha1"
	"code-code.internal/platform-k8s/internal/platform/resourcemeta"
	"code-code.internal/platform-k8s/internal/providerservice/providers"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	templateKindCredentialDefinition = "CredentialDefinition"
	templateKindProvider             = "Provider"
)

// TemplateManagementService manages quick template listing and apply
// operations.
type TemplateManagementService struct {
	client    ctrlclient.Client
	providers providers.Store
	templates map[string]manifestTemplate
}

// NewTemplateManagementService creates one template management service.
func NewTemplateManagementService(client ctrlclient.Client, providerStore providers.Store) (*TemplateManagementService, error) {
	if client == nil {
		return nil, fmt.Errorf("platformk8s: client is nil")
	}
	if providerStore == nil {
		return nil, fmt.Errorf("platformk8s: provider store is nil")
	}
	templates, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	return &TemplateManagementService{client: client, providers: providerStore, templates: templates}, nil
}

// List returns the available quick templates.
func (s *TemplateManagementService) List() []*managementv1.TemplateView {
	items := make([]*managementv1.TemplateView, 0, len(s.templates))
	for _, item := range s.templates {
		items = append(items, cloneTemplateView(item.view))
	}
	slices.SortFunc(items, func(a, b *managementv1.TemplateView) int {
		if a.GetTemplateId() < b.GetTemplateId() {
			return -1
		}
		if a.GetTemplateId() > b.GetTemplateId() {
			return 1
		}
		return 0
	})
	return items
}

// Apply materializes one template into platform state.
func (s *TemplateManagementService) Apply(ctx context.Context, request *managementv1.ApplyTemplateRequest) (*managementv1.ApplyTemplateResult, error) {
	if request == nil {
		return nil, domainerror.NewValidation("platformk8s: template apply request is nil")
	}
	item, ok := s.templates[request.GetTemplateId()]
	if !ok {
		return nil, domainerror.NewNotFound("platformk8s: template %q not found", request.GetTemplateId())
	}
	if request.GetNamespace() == "" {
		return nil, domainerror.NewValidation("platformk8s: namespace is required")
	}
	if request.GetDisplayName() == "" {
		return nil, domainerror.NewValidation("platformk8s: display name is required")
	}
	providerID, err := resourcemeta.EnsureResourceID(request.GetProviderId(), request.GetDisplayName(), request.GetTemplateId())
	if err != nil {
		return nil, err
	}
	request.ProviderId = providerID
	allowedModelIDs, err := dedupeModelIDs(request.GetAllowedModelIds())
	if err != nil {
		return nil, err
	}
	if len(allowedModelIDs) == 0 {
		allowedModelIDs = append([]string(nil), item.view.GetDefaultModels()...)
	}

	provider, err := buildTemplateProvider(item, request, allowedModelIDs)
	if err != nil {
		return nil, err
	}
	if err := s.applyProvider(ctx, request.GetNamespace(), request.GetDisplayName(), provider); err != nil {
		return nil, err
	}

	return &managementv1.ApplyTemplateResult{
		TemplateId:   request.GetTemplateId(),
		Namespace:    request.GetNamespace(),
		DisplayName:  request.GetDisplayName(),
		ProviderId:   request.GetProviderId(),
		AppliedKinds: []string{templateKindProvider},
	}, nil
}

type manifestTemplate struct {
	view     *managementv1.TemplateView
	provider *providerv1.Provider
}

func loadTemplates() (map[string]manifestTemplate, error) {
	assets, err := TemplateAssets()
	if err != nil {
		return nil, err
	}
	out := make(map[string]manifestTemplate, len(assets))
	for _, asset := range assets {
		item, err := parseTemplateAsset(asset)
		if err != nil {
			log.Printf("platformk8s/templates: skip invalid template %q: %v", asset.ID, err)
			continue
		}
		out[item.view.GetTemplateId()] = item
	}
	return out, nil
}

func parseTemplateAsset(asset TemplateAsset) (manifestTemplate, error) {
	documents, err := decodeManifestDocuments(asset.Manifest)
	if err != nil {
		return manifestTemplate{}, fmt.Errorf("platformk8s: parse template %q: %w", asset.ID, err)
	}
	item := manifestTemplate{}
	for _, document := range documents {
		switch document.GetKind() {
		case templateKindProvider:
			provider := &providerv1.Provider{}
			if err = platformv1alpha1.UnmarshalSpecProto(document, "provider", provider); err == nil {
				item.provider = provider
			}
		}
		if err != nil {
			return manifestTemplate{}, err
		}
	}
	if item.provider == nil || len(item.provider.GetSurfaces()) == 0 {
		return manifestTemplate{}, fmt.Errorf("platformk8s: template %q is missing provider resource", asset.ID)
	}
	surface := item.provider.GetSurfaces()[0]
	defaultModels := surfaceModelIDs(surface.GetRuntime())
	item.view = &managementv1.TemplateView{
		TemplateId:         asset.ID,
		DisplayName:        humanizeTemplateID(asset.ID),
		Vendor:             humanizeToken(strings.Split(asset.ID, "-")[0]),
		Protocol:           providerv1.RuntimeProtocol(surface.GetRuntime()).String(),
		DefaultBaseUrl:     providerv1.RuntimeBaseURL(surface.GetRuntime()),
		DefaultModels:      defaultModels,
		RequiresCredential: surface.ProviderCredentialRef != nil || manifestContainsCredentialDefinition(documents),
	}
	return item, nil
}

func decodeManifestDocuments(raw []byte) ([]*unstructured.Unstructured, error) {
	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(raw), 4096)
	documents := make([]*unstructured.Unstructured, 0, 4)
	for {
		payload := make(map[string]any)
		if err := decoder.Decode(&payload); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(payload) == 0 {
			continue
		}
		documents = append(documents, &unstructured.Unstructured{Object: payload})
	}
	return documents, nil
}

func manifestContainsCredentialDefinition(documents []*unstructured.Unstructured) bool {
	for _, document := range documents {
		if document.GetKind() == templateKindCredentialDefinition {
			return true
		}
	}
	return false
}

func humanizeTemplateID(templateID string) string {
	parts := strings.Split(templateID, "-")
	words := make([]string, 0, len(parts))
	for _, part := range parts {
		words = append(words, humanizeToken(part))
	}
	return strings.Join(words, " ")
}

func humanizeToken(token string) string {
	if displayName := templateTokenDisplayName(token); displayName != "" {
		return displayName
	}
	if token == "" {
		return ""
	}
	return strings.ToUpper(token[:1]) + token[1:]
}

func (s *TemplateManagementService) applyProvider(ctx context.Context, namespace string, displayName string, provider *providerv1.Provider) error {
	provider.DisplayName = strings.TrimSpace(displayName)
	_, err := s.providers.Upsert(ctx, provider)
	return err
}

func buildTemplateProvider(item manifestTemplate, request *managementv1.ApplyTemplateRequest, allowedModelIDs []string) (*providerv1.Provider, error) {
	provider := proto.Clone(item.provider).(*providerv1.Provider)
	provider.ProviderId = request.GetProviderId()
	provider.DisplayName = request.GetDisplayName()
	if len(provider.GetSurfaces()) == 0 || provider.Surfaces[0].GetRuntime() == nil {
		return nil, domainerror.NewValidation("platformk8s: template %q is missing provider surface", item.view.GetTemplateId())
	}
	surface := provider.Surfaces[0]
	if request.GetProviderCredentialId() != "" {
		surface.ProviderCredentialRef = &providerv1.ProviderCredentialRef{ProviderCredentialId: request.GetProviderCredentialId()}
	} else {
		surface.ProviderCredentialRef = nil
	}
	if surface.Runtime.Catalog == nil {
		surface.Runtime.Catalog = &providerv1.ProviderModelCatalog{}
	}
	surface.Runtime.Catalog.Models = make([]*providerv1.ProviderModelCatalogEntry, 0, len(allowedModelIDs))
	for _, modelID := range allowedModelIDs {
		surface.Runtime.Catalog.Models = append(surface.Runtime.Catalog.Models, &providerv1.ProviderModelCatalogEntry{
			ProviderModelId: modelID,
		})
	}
	if surface.Runtime.Catalog.Source == providerv1.CatalogSource_CATALOG_SOURCE_UNSPECIFIED {
		surface.Runtime.Catalog.Source = providerv1.CatalogSource_CATALOG_SOURCE_FALLBACK_CONFIG
	}
	if err := providerv1.ValidateProvider(provider); err != nil {
		return nil, domainerror.NewValidation("platformk8s: invalid provider from template %q: %v", item.view.GetTemplateId(), err)
	}
	return provider, nil
}

func cloneTemplateView(view *managementv1.TemplateView) *managementv1.TemplateView {
	if view == nil {
		return nil
	}
	return &managementv1.TemplateView{
		TemplateId:         view.GetTemplateId(),
		DisplayName:        view.GetDisplayName(),
		Vendor:             view.GetVendor(),
		Protocol:           view.GetProtocol(),
		DefaultBaseUrl:     view.GetDefaultBaseUrl(),
		DefaultModels:      append([]string(nil), view.GetDefaultModels()...),
		RequiresCredential: view.GetRequiresCredential(),
	}
}

func surfaceModelIDs(surface *providerv1.ProviderSurfaceRuntime) []string {
	if surface == nil || surface.GetCatalog() == nil {
		return nil
	}
	models := surface.GetCatalog().GetModels()
	out := make([]string, 0, len(models))
	for _, model := range models {
		if model == nil || model.GetProviderModelId() == "" {
			continue
		}
		out = append(out, model.GetProviderModelId())
	}
	return out
}

func dedupeModelIDs(modelIDs []string) ([]string, error) {
	out, err := dedupeNonEmpty(modelIDs, "model id")
	if err != nil {
		return nil, err
	}
	slices.Sort(out)
	return out, nil
}

func dedupeNonEmpty(values []string, label string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			return nil, domainerror.NewValidation("platformk8s: duplicate %s entry %q", label, value)
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}
