package providersurfaces

import (
	"context"
	"fmt"
	"slices"
	"strings"

	providerv1 "code-code.internal/go-contract/provider/v1"
	clisupport "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/support"
	surfaceregistry "code-code.internal/platform-k8s/internal/supportservice/providersurfaces/registry"
	vendorsupport "code-code.internal/platform-k8s/internal/supportservice/vendors/support"
)

// Service exposes the effective provider surface read path.
type Service struct {
	builtins   map[string]*providerv1.ProviderSurface
	cliSupport *clisupport.ManagementService
	vendors    *vendorsupport.ManagementService
}

// NewService creates one provider surface service.
func NewService(
	cliSupport *clisupport.ManagementService,
	vendorSupport *vendorsupport.ManagementService,
) (*Service, error) {
	if cliSupport == nil {
		return nil, fmt.Errorf("platformk8s/providersurfaces: cli support service is nil")
	}
	if vendorSupport == nil {
		return nil, fmt.Errorf("platformk8s/providersurfaces: vendor support service is nil")
	}
	builtins := make(map[string]*providerv1.ProviderSurface)
	for _, item := range surfaceregistry.List() {
		builtins[item.GetSurfaceId()] = cloneSurface(item)
	}
	return &Service{
		builtins:   builtins,
		cliSupport: cliSupport,
		vendors:    vendorSupport,
	}, nil
}

// Get returns one effective provider surface by stable identity.
func (s *Service) Get(ctx context.Context, surfaceID string) (*providerv1.ProviderSurface, error) {
	surface, ok := s.builtins[surfaceID]
	if !ok {
		return nil, fmt.Errorf("platformk8s/providersurfaces: provider surface %q not found", surfaceID)
	}
	return s.withMeta(ctx, cloneSurface(surface))
}

// List returns all effective provider surfaces.
func (s *Service) List(ctx context.Context) ([]*providerv1.ProviderSurface, error) {
	items := make([]*providerv1.ProviderSurface, 0, len(s.builtins))
	for _, item := range s.builtins {
		items = append(items, cloneSurface(item))
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
	index, err := s.loadMetaIndex(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*providerv1.ProviderSurface, 0, len(items))
	for _, item := range items {
		out = append(out, attachSurfaceMeta(item, index))
	}
	return out, nil
}

type surfaceMetaIndex struct {
	modelCatalogProbeableBySurface map[string]bool
	quotaProbeableBySurface        map[string]bool
}

func (s *Service) withMeta(ctx context.Context, surface *providerv1.ProviderSurface) (*providerv1.ProviderSurface, error) {
	index, err := s.loadMetaIndex(ctx)
	if err != nil {
		return nil, err
	}
	return attachSurfaceMeta(surface, index), nil
}

func (s *Service) loadMetaIndex(ctx context.Context) (*surfaceMetaIndex, error) {
	index := &surfaceMetaIndex{
		modelCatalogProbeableBySurface: map[string]bool{},
		quotaProbeableBySurface:        map[string]bool{},
	}
	cliItems, err := s.cliSupport.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/providersurfaces: list cli-backed provider surfaces: %w", err)
	}
	for _, item := range cliItems {
		if item == nil || item.GetOauth() == nil {
			continue
		}
		surfaceID := strings.TrimSpace(clisupport.OAuthProviderSurfaceID(item))
		cliID := strings.TrimSpace(item.GetCliId())
		if surfaceID == "" || cliID == "" {
			continue
		}
		if clisupport.OAuthSupportsModelCatalogProbe(item) {
			index.modelCatalogProbeableBySurface[surfaceID] = true
		}
		if clisupport.OAuthSupportsQuotaProbe(item) {
			index.quotaProbeableBySurface[surfaceID] = true
		}
	}
	vendorItems, err := s.vendors.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/providersurfaces: list vendor-backed provider surfaces: %w", err)
	}
	for _, item := range vendorItems {
		vendorID := strings.TrimSpace(item.GetVendor().GetVendorId())
		if vendorID == "" {
			continue
		}
		for _, binding := range item.GetProviderBindings() {
			surfaceID := strings.TrimSpace(vendorsupport.BindingSurfaceID(binding))
			if surfaceID == "" {
				continue
			}
			if vendorsupport.SupportsModelCatalogProbe(binding) {
				index.modelCatalogProbeableBySurface[surfaceID] = true
			}
			if vendorsupport.SupportsQuotaProbe(binding) {
				index.quotaProbeableBySurface[surfaceID] = true
			}
		}
	}
	return index, nil
}

func attachSurfaceMeta(
	surface *providerv1.ProviderSurface,
	index *surfaceMetaIndex,
) *providerv1.ProviderSurface {
	next := cloneSurface(surface)
	if next == nil {
		return nil
	}
	attachSurfaceProbeCapabilities(next, index)
	return next
}

func attachSurfaceProbeCapabilities(
	surface *providerv1.ProviderSurface,
	index *surfaceMetaIndex,
) {
	if surface == nil || index == nil {
		return
	}
	current := surface.GetCapabilities()
	if current == nil {
		current = &providerv1.ProviderCapabilities{}
	}
	next := cloneCapabilities(current)
	surfaceID := strings.TrimSpace(surface.GetSurfaceId())
	if index.modelCatalogProbeableBySurface[surfaceID] {
		next.SupportsModelCatalogProbe = true
	}
	if index.quotaProbeableBySurface[surfaceID] {
		next.SupportsQuotaProbe = true
	}
	surface.Capabilities = next
}

func cloneCapabilities(capabilities *providerv1.ProviderCapabilities) *providerv1.ProviderCapabilities {
	if capabilities == nil {
		return nil
	}
	return cloneProviderCapabilities(capabilities)
}
