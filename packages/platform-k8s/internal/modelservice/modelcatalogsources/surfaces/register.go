package surfaces

import (
	"fmt"

	credentialv1 "code-code.internal/go-contract/credential/v1"
	providerv1 "code-code.internal/go-contract/provider/v1"
	"code-code.internal/platform-k8s/internal/modelservice/modelcatalogsources"
	"code-code.internal/platform-k8s/internal/providerservice/providers"
	surfaceregistry "code-code.internal/platform-k8s/internal/supportservice/providersurfaces/registry"
)

type RegisterConfig struct {
	Probe modelcatalogsources.ModelIDProbe
}

func Register(registry *modelcatalogsources.Registry, config RegisterConfig) error {
	if registry == nil {
		return fmt.Errorf("platformk8s/modelcatalogsources/surfaces: registry is nil")
	}
	for _, surface := range surfaceregistry.List() {
		if !supportsBestEffortCatalogProbe(surface) {
			continue
		}
		if err := registry.Register(&surfaceSource{
			ref: modelcatalogsources.ProbeRef(providers.SurfaceModelCatalogProbeID(surface.GetSurfaceId())),
		}); err != nil {
			return err
		}
	}
	return nil
}

type surfaceSource struct {
	ref modelcatalogsources.CapabilityRef
}

func (s *surfaceSource) CapabilityRef() modelcatalogsources.CapabilityRef {
	return s.ref
}

func supportsBestEffortCatalogProbe(surface *providerv1.ProviderSurface) bool {
	if surface == nil {
		return false
	}
	return surface.GetKind() == providerv1.ProviderSurfaceKind_PROVIDER_SURFACE_KIND_API &&
		supportsCredentialKind(surface.GetSupportedCredentialKinds(), credentialv1.CredentialKind_CREDENTIAL_KIND_API_KEY) &&
		(modelCatalogProbeMethod(surface) == providerv1.ProviderSurfaceModelCatalogProbeMethod_PROVIDER_SURFACE_MODEL_CATALOG_PROBE_METHOD_PROTOCOL_BEST_EFFORT ||
			modelCatalogProbeMethod(surface) == providerv1.ProviderSurfaceModelCatalogProbeMethod_PROVIDER_SURFACE_MODEL_CATALOG_PROBE_METHOD_STATIC ||
			modelCatalogProbeMethod(surface) == providerv1.ProviderSurfaceModelCatalogProbeMethod_PROVIDER_SURFACE_MODEL_CATALOG_PROBE_METHOD_ADAPTER)
}

func supportsCredentialKind(values []credentialv1.CredentialKind, want credentialv1.CredentialKind) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func modelCatalogProbeMethod(surface *providerv1.ProviderSurface) providerv1.ProviderSurfaceModelCatalogProbeMethod {
	if surface == nil || surface.GetProbes() == nil || surface.GetProbes().GetModelCatalog() == nil {
		return providerv1.ProviderSurfaceModelCatalogProbeMethod_PROVIDER_SURFACE_MODEL_CATALOG_PROBE_METHOD_UNSPECIFIED
	}
	return surface.GetProbes().GetModelCatalog().GetMethod()
}
