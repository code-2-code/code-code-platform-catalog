package models

import (
	"fmt"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
)

type SurfaceIdentity struct {
	VendorID string
	ModelID  string
}

func NewSurfaceIdentity(vendorID string, modelID string) (SurfaceIdentity, error) {
	vendorID = strings.TrimSpace(vendorID)
	modelID = strings.TrimSpace(modelID)
	if vendorID == "" {
		return SurfaceIdentity{}, fmt.Errorf("platformk8s/models: vendor id is empty")
	}
	if modelID == "" {
		return SurfaceIdentity{}, fmt.Errorf("platformk8s/models: model id is empty")
	}
	return SurfaceIdentity{
		VendorID: vendorID,
		ModelID:  modelID,
	}, nil
}

func IdentityFromDefinition(definition *modelv1.ModelVersion) (SurfaceIdentity, error) {
	if definition == nil {
		return SurfaceIdentity{}, fmt.Errorf("platformk8s/models: model definition is nil")
	}
	return NewSurfaceIdentity(definition.GetVendorId(), definition.GetModelId())
}

func identityKey(vendorID string, modelID string) string {
	return strings.TrimSpace(vendorID) + "\x00" + strings.TrimSpace(modelID)
}

func (i SurfaceIdentity) Key() string {
	return identityKey(i.VendorID, i.ModelID)
}
