package supportservice

import (
	supportv1 "code-code.internal/go-contract/platform/support/v1"
	"google.golang.org/protobuf/proto"
)

func sanitizeVendor(in *supportv1.Vendor) *supportv1.Vendor {
	if in == nil {
		return &supportv1.Vendor{}
	}
	next := proto.Clone(in).(*supportv1.Vendor)
	for _, binding := range next.GetProviderBindings() {
		if binding == nil {
			continue
		}
		binding.EgressPolicy = nil
		binding.Observability = nil
		binding.ModelDiscovery = nil
	}
	return next
}

func sanitizeCLI(in *supportv1.CLI) *supportv1.CLI {
	if in == nil {
		return &supportv1.CLI{}
	}
	next := proto.Clone(in).(*supportv1.CLI)
	if oauth := next.GetOauth(); oauth != nil {
		oauth.AuthMaterialization = nil
		oauth.Observability = nil
		oauth.ModelCatalog = nil
	}
	for _, support := range next.GetApiKeyProtocols() {
		if support != nil {
			support.AuthMaterialization = nil
		}
	}
	next.EgressPolicy = nil
	return next
}
