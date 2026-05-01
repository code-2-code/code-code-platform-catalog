package surfaces

import (
	"fmt"

	"code-code.internal/platform-k8s/internal/modelservice/modelcatalogsources"
)

type RegisterConfig struct {
	Probe modelcatalogsources.ModelIDProbe
}

func Register(registry *modelcatalogsources.Registry, config RegisterConfig) error {
	if registry == nil {
		return fmt.Errorf("platformk8s/modelcatalogsources/surfaces: registry is nil")
	}
	return nil
}

type surfaceSource struct {
	ref modelcatalogsources.CapabilityRef
}

func (s *surfaceSource) CapabilityRef() modelcatalogsources.CapabilityRef {
	return s.ref
}
