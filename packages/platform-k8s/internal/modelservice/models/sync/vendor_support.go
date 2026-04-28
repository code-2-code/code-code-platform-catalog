package sync

import (
	"fmt"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"google.golang.org/protobuf/proto"
)

func vendorSupportDefinitionCandidate(vendorID string, source *modelv1.ModelVersion) (*modelv1.ModelVersion, error) {
	if source == nil {
		return nil, fmt.Errorf("platformk8s/models: vendor support model definition is nil")
	}
	definition := proto.Clone(source).(*modelv1.ModelVersion)
	canonicalID := strings.TrimSpace(definition.GetModelId())
	if canonicalID == "" {
		return nil, fmt.Errorf("platformk8s/models: vendor support model id is empty")
	}
	definition.ModelId = canonicalID

	supportVendorID := strings.TrimSpace(vendorID)
	sourceVendorID := strings.TrimSpace(definition.GetVendorId())
	switch {
	case sourceVendorID == "":
		definition.VendorId = supportVendorID
	case sourceVendorID != supportVendorID:
		return nil, fmt.Errorf(
			"platformk8s/models: vendor support model %q vendor_id %q does not match support vendor %q",
			canonicalID,
			sourceVendorID,
			supportVendorID,
		)
	}

	if strings.TrimSpace(definition.GetDisplayName()) == "" {
		definition.DisplayName = canonicalID
	}
	models.ApplyCreateDefaults(definition)
	if err := modelv1.ValidateDefinition(definition); err != nil {
		return nil, fmt.Errorf("platformk8s/models: invalid vendor support model definition %q: %w", canonicalID, err)
	}
	return definition, nil
}
