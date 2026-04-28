package sync

import (
	"slices"

	models "code-code.internal/platform-k8s/internal/modelservice/models"
)

func testIdentityKey(vendorID, modelID string) string {
	id, _ := models.NewSurfaceIdentity(vendorID, modelID)
	return id.Key()
}

func equalStrings(a []string, b []string) bool {
	return slices.Equal(a, b)
}
