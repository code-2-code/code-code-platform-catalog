package references

import (
	"context"
	"fmt"
	"strings"

	"code-code.internal/go-contract/domainerror"
	provideraggregates "code-code.internal/platform-k8s/internal/providerservice/providers"
)

// ResourceReferenceChecker verifies whether a given resource is still
// referenced by other resources before allowing deletion.
type ResourceReferenceChecker struct {
	providers provideraggregates.Store
}

// NewResourceReferenceChecker creates one reference checker.
func NewResourceReferenceChecker(providers provideraggregates.Store) *ResourceReferenceChecker {
	return &ResourceReferenceChecker{providers: providers}
}

// CheckCredentialReferences returns an error if the credential is referenced
// by any provider credential slot.
func (c *ResourceReferenceChecker) CheckCredentialReferences(ctx context.Context, credentialID string) error {
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return domainerror.NewValidation("platformk8s: credential id is empty")
	}

	providerID, err := c.firstProviderReferencingCredential(ctx, credentialID)
	if err != nil {
		return err
	}
	if providerID != "" {
		return domainerror.NewReferenceConflict("platformk8s: credential %q is referenced by provider %q", credentialID, providerID)
	}

	return nil
}

func (c *ResourceReferenceChecker) firstProviderReferencingCredential(ctx context.Context, credentialID string) (string, error) {
	if c == nil || c.providers == nil {
		return "", fmt.Errorf("platformk8s: provider repository is nil")
	}
	providers, err := c.providers.List(ctx)
	if err != nil {
		return "", fmt.Errorf("platformk8s: list providers for reference check: %w", err)
	}
	for _, provider := range providers {
		for _, surface := range provider.GetSurfaces() {
			if strings.TrimSpace(surface.GetProviderCredentialRef().GetProviderCredentialId()) == credentialID {
				return provider.GetProviderId(), nil
			}
		}
	}
	return "", nil
}

// Compile-time verifications.
var (
	_ = (*ResourceReferenceChecker)(nil).CheckCredentialReferences
)
