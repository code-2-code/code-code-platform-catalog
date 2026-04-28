package modelcatalogsources

import (
	"fmt"
	"strings"
)

type CapabilityRef struct {
	ID string
}

func ProbeRef(probeID string) CapabilityRef {
	return CapabilityRef{ID: strings.TrimSpace(probeID)}
}

func (r CapabilityRef) Key() (string, error) {
	id := strings.TrimSpace(r.ID)
	if id == "" {
		return "", fmt.Errorf("platformk8s/modelcatalogsources: probe id is empty")
	}
	return id, nil
}
