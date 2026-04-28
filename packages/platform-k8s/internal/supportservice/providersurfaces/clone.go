package providersurfaces

import (
	providerv1 "code-code.internal/go-contract/provider/v1"
	"google.golang.org/protobuf/proto"
)

func cloneSurface(surface *providerv1.ProviderSurface) *providerv1.ProviderSurface {
	if surface == nil {
		return nil
	}
	return proto.Clone(surface).(*providerv1.ProviderSurface)
}

func cloneProviderCapabilities(capabilities *providerv1.ProviderCapabilities) *providerv1.ProviderCapabilities {
	if capabilities == nil {
		return nil
	}
	return proto.Clone(capabilities).(*providerv1.ProviderCapabilities)
}
