package support

import (
	"strings"

	observabilityv1 "code-code.internal/go-contract/observability/v1"
	supportv1 "code-code.internal/go-contract/platform/support/v1"
	providerv1 "code-code.internal/go-contract/provider/v1"
	"google.golang.org/protobuf/proto"
)

func normalizeProviderBindings(vendor *supportv1.Vendor) {
	if vendor == nil {
		return
	}
	for _, binding := range vendor.GetProviderBindings() {
		normalizeProviderBinding(vendor, binding)
	}
}

func normalizeProviderBinding(vendor *supportv1.Vendor, binding *supportv1.VendorProviderBinding) {
	if vendor == nil || binding == nil {
		return
	}
	current := binding.GetProviderBinding()
	if current == nil {
		current = &supportv1.ProviderSurfaceBinding{}
		binding.ProviderBinding = current
	}
	if strings.TrimSpace(current.GetSurfaceId()) == "" {
		current.SurfaceId = BindingSurfaceID(binding)
	}
	if strings.TrimSpace(current.GetModelCatalogProbeId()) == "" {
		current.ModelCatalogProbeId = defaultModelCatalogProbeID(vendor, binding)
	}
	if strings.TrimSpace(current.GetQuotaProbeId()) == "" {
		current.QuotaProbeId = defaultQuotaProbeID(binding)
	}
	if strings.TrimSpace(current.GetEgressPolicyId()) == "" {
		current.EgressPolicyId = defaultEgressPolicyID(vendor)
	}
	if strings.TrimSpace(current.GetHeaderRewritePolicyId()) == "" {
		current.HeaderRewritePolicyId = current.GetEgressPolicyId()
	}
}

func cloneProviderBindings(vendor *supportv1.Vendor) []*supportv1.VendorProviderBinding {
	if vendor == nil || len(vendor.GetProviderBindings()) == 0 {
		return nil
	}
	out := make([]*supportv1.VendorProviderBinding, 0, len(vendor.GetProviderBindings()))
	for _, binding := range vendor.GetProviderBindings() {
		if binding == nil {
			continue
		}
		out = append(out, proto.Clone(binding).(*supportv1.VendorProviderBinding))
	}
	return out
}

func BindingForSurfaceID(vendor *supportv1.Vendor, surfaceID string) (*supportv1.VendorProviderBinding, bool) {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" || vendor == nil {
		return nil, false
	}
	for _, binding := range vendor.GetProviderBindings() {
		if BindingSurfaceID(binding) == surfaceID {
			return proto.Clone(binding).(*supportv1.VendorProviderBinding), true
		}
		for _, template := range binding.GetSurfaceTemplates() {
			if strings.TrimSpace(template.GetSurfaceId()) == surfaceID {
				return proto.Clone(binding).(*supportv1.VendorProviderBinding), true
			}
		}
	}
	return nil, false
}

type MaterializedSurfaceTarget struct {
	Runtime          *providerv1.ProviderSurfaceRuntime
	BootstrapCatalog *providerv1.ProviderModelCatalog
}

func MaterializeSurfaceTargets(vendor *supportv1.Vendor) []*MaterializedSurfaceTarget {
	if vendor == nil {
		return nil
	}
	out := []*MaterializedSurfaceTarget{}
	for _, binding := range vendor.GetProviderBindings() {
		if binding == nil {
			continue
		}
		bindingConfig := binding.GetProviderBinding()
		for _, template := range binding.GetSurfaceTemplates() {
			if template == nil {
				continue
			}
			runtime := template.GetRuntime()
			if runtime == nil {
				runtime = &providerv1.ProviderSurfaceRuntime{}
			} else {
				runtime = proto.Clone(runtime).(*providerv1.ProviderSurfaceRuntime)
			}
			if strings.TrimSpace(runtime.GetDisplayName()) == "" {
				runtime.DisplayName = strings.TrimSpace(template.GetSurfaceId())
			}
			if strings.TrimSpace(runtime.GetModelCatalogProbeId()) == "" {
				runtime.ModelCatalogProbeId = strings.TrimSpace(bindingConfig.GetModelCatalogProbeId())
			}
			if strings.TrimSpace(runtime.GetQuotaProbeId()) == "" {
				runtime.QuotaProbeId = strings.TrimSpace(bindingConfig.GetQuotaProbeId())
			}
			if strings.TrimSpace(runtime.GetEgressRulesetId()) == "" {
				runtime.EgressRulesetId = strings.TrimSpace(bindingConfig.GetEgressPolicyId())
			}
			if strings.TrimSpace(runtime.GetMitmId()) == "" {
				runtime.MitmId = strings.TrimSpace(bindingConfig.GetHeaderRewritePolicyId())
			}
			runtime.Origin = providerv1.ProviderSurfaceOrigin_PROVIDER_SURFACE_ORIGIN_DERIVED
			out = append(out, &MaterializedSurfaceTarget{
				Runtime:          runtime,
				BootstrapCatalog: cloneBootstrapCatalog(template.GetBootstrapCatalog()),
			})
		}
	}
	return out
}

func MaterializeSurfaces(vendor *supportv1.Vendor) []*providerv1.ProviderSurfaceRuntime {
	out := []*providerv1.ProviderSurfaceRuntime{}
	for _, target := range MaterializeSurfaceTargets(vendor) {
		if target == nil || target.Runtime == nil {
			continue
		}
		out = append(out, proto.Clone(target.Runtime).(*providerv1.ProviderSurfaceRuntime))
	}
	return out
}

func MaterializeObservability(vendor *supportv1.Vendor, surfaceID string) *observabilityv1.ObservabilityCapability {
	if vendor == nil {
		return nil
	}
	bindings := selectBindings(vendor, surfaceID)
	if len(bindings) == 0 {
		return nil
	}
	if len(bindings) == 1 {
		return cloneObservability(bindings[0].GetObservability())
	}
	profiles := make([]*observabilityv1.ObservabilityProfile, 0)
	seen := map[string]struct{}{}
	for _, binding := range bindings {
		for _, profile := range binding.GetObservability().GetProfiles() {
			if profile == nil {
				continue
			}
			key := strings.TrimSpace(profile.GetProfileId())
			if key == "" {
				key = strings.TrimSpace(profile.GetDisplayName())
			}
			if key != "" {
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
			}
			profiles = append(profiles, proto.Clone(profile).(*observabilityv1.ObservabilityProfile))
		}
	}
	if len(profiles) == 0 {
		return nil
	}
	return &observabilityv1.ObservabilityCapability{Profiles: profiles}
}

func SupportsModelCatalogProbe(binding *supportv1.VendorProviderBinding) bool {
	if binding == nil {
		return false
	}
	if binding.GetModelDiscovery() != nil && binding.GetModelDiscovery().GetActiveDiscovery() != nil {
		return true
	}
	for _, template := range binding.GetSurfaceTemplates() {
		if len(template.GetBootstrapCatalog().GetModels()) > 0 {
			return true
		}
	}
	return false
}

func SupportsQuotaProbe(binding *supportv1.VendorProviderBinding) bool {
	if binding == nil {
		return false
	}
	return observabilityHasActiveQuery(binding.GetObservability())
}

func BindingSurfaceID(binding *supportv1.VendorProviderBinding) string {
	if binding == nil {
		return ""
	}
	if current := strings.TrimSpace(binding.GetProviderBinding().GetSurfaceId()); current != "" {
		return current
	}
	value := ""
	for _, template := range binding.GetSurfaceTemplates() {
		surfaceID := strings.TrimSpace(template.GetSurfaceId())
		if surfaceID == "" {
			continue
		}
		if value == "" {
			value = surfaceID
			continue
		}
		if value != surfaceID {
			return ""
		}
	}
	return value
}

func defaultModelCatalogProbeID(vendor *supportv1.Vendor, binding *supportv1.VendorProviderBinding) string {
	if surfaceID := strings.TrimSpace(BindingSurfaceID(binding)); surfaceID != "" {
		return surfaceID
	}
	if vendorID := strings.TrimSpace(vendor.GetVendor().GetVendorId()); vendorID != "" {
		return vendorID
	}
	return ""
}

func defaultQuotaProbeID(binding *supportv1.VendorProviderBinding) string {
	if collectorID := firstActiveQueryCollectorID(binding.GetObservability()); collectorID != "" {
		return collectorID
	}
	return ""
}

func defaultEgressPolicyID(vendor *supportv1.Vendor) string {
	if vendorID := strings.TrimSpace(vendor.GetVendor().GetVendorId()); vendorID != "" {
		return "vendor." + vendorID
	}
	return ""
}

func firstActiveQueryCollectorID(capability *observabilityv1.ObservabilityCapability) string {
	for _, profile := range capability.GetProfiles() {
		activeQuery := profile.GetActiveQuery()
		if activeQuery == nil {
			continue
		}
		if collectorID := strings.TrimSpace(activeQuery.GetCollectorId()); collectorID != "" {
			return collectorID
		}
	}
	return ""
}

func cloneBootstrapCatalog(catalog *providerv1.ProviderModelCatalog) *providerv1.ProviderModelCatalog {
	if catalog == nil {
		return nil
	}
	return proto.Clone(catalog).(*providerv1.ProviderModelCatalog)
}

func cloneObservability(capability *observabilityv1.ObservabilityCapability) *observabilityv1.ObservabilityCapability {
	if capability == nil {
		return nil
	}
	return proto.Clone(capability).(*observabilityv1.ObservabilityCapability)
}

func selectBindings(
	vendor *supportv1.Vendor,
	surfaceID string,
) []*supportv1.VendorProviderBinding {
	if vendor == nil {
		return nil
	}
	surfaceID = strings.TrimSpace(surfaceID)
	out := make([]*supportv1.VendorProviderBinding, 0, len(vendor.GetProviderBindings()))
	for _, binding := range vendor.GetProviderBindings() {
		if binding == nil {
			continue
		}
		if surfaceID != "" && !bindingHasSurfaceID(binding, surfaceID) {
			continue
		}
		if surfaceID != "" && BindingSurfaceID(binding) != surfaceID {
			continue
		}
		out = append(out, binding)
	}
	return out
}

func bindingHasSurfaceID(binding *supportv1.VendorProviderBinding, surfaceID string) bool {
	for _, template := range binding.GetSurfaceTemplates() {
		if strings.TrimSpace(template.GetSurfaceId()) == surfaceID {
			return true
		}
	}
	return false
}

func AnyProviderCardEnabled(vendor *supportv1.Vendor) bool {
	if vendor == nil {
		return false
	}
	for _, binding := range vendor.GetProviderBindings() {
		if binding != nil && binding.GetProviderCard().GetEnabled() {
			return true
		}
	}
	return false
}
