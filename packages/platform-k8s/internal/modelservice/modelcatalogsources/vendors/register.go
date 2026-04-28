package vendors

import (
	"context"
	"fmt"
	"strings"

	supportv1 "code-code.internal/go-contract/platform/support/v1"
	"code-code.internal/platform-k8s/internal/modelservice/modelcatalogsources"
	vendorsupport "code-code.internal/platform-k8s/internal/supportservice/vendors/support"
	"google.golang.org/protobuf/proto"
)

type VendorSupportReader interface {
	List(context.Context) ([]*supportv1.Vendor, error)
}

type RegisterConfig struct {
	Support VendorSupportReader
	Probe   modelcatalogsources.ModelIDProbe
}

func Register(ctx context.Context, registry *modelcatalogsources.Registry, config RegisterConfig) error {
	if registry == nil {
		return fmt.Errorf("platformk8s/modelcatalogsources/vendors: registry is nil")
	}
	if config.Support == nil {
		return fmt.Errorf("platformk8s/modelcatalogsources/vendors: vendor support reader is nil")
	}
	vendors, err := config.Support.List(ctx)
	if err != nil {
		return err
	}
	for _, vendor := range vendors {
		if !hasCatalogCapability(vendor) {
			continue
		}
		vendorID := strings.TrimSpace(vendor.GetVendor().GetVendorId())
		if vendorID == "" {
			return fmt.Errorf("platformk8s/modelcatalogsources/vendors: vendor support id is empty")
		}
		if err := registry.Register(&vendorSource{
			ref:    modelcatalogsources.ProbeRef("vendor." + vendorID),
			vendor: proto.Clone(vendor).(*supportv1.Vendor),
		}); err != nil {
			return err
		}
	}
	return nil
}

func hasCatalogCapability(vendor *supportv1.Vendor) bool {
	if vendor == nil {
		return false
	}
	for _, binding := range vendor.GetProviderBindings() {
		if vendorsupport.SupportsModelCatalogProbe(binding) {
			return true
		}
	}
	return false
}

type vendorSource struct {
	ref    modelcatalogsources.CapabilityRef
	vendor *supportv1.Vendor
}

func (s *vendorSource) CapabilityRef() modelcatalogsources.CapabilityRef {
	return s.ref
}
