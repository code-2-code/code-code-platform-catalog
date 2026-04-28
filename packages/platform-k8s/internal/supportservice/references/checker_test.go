package references

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	"code-code.internal/go-contract/domainerror"
	providerv1 "code-code.internal/go-contract/provider/v1"
	"google.golang.org/protobuf/proto"
)

func TestCheckCredentialReferencesUsesProviderAggregate(t *testing.T) {
	checker := NewResourceReferenceChecker(referenceProviderStore{items: []*providerv1.Provider{{
		ProviderId:  "provider-1",
		DisplayName: "Provider One",
		Surfaces: []*providerv1.ProviderSurfaceBinding{{
			SurfaceId:             "endpoint-1",
			ProviderCredentialRef: &providerv1.ProviderCredentialRef{ProviderCredentialId: "credential-1"},
			Runtime: &providerv1.ProviderSurfaceRuntime{
				DisplayName: "Surface One",
				Origin:      providerv1.ProviderSurfaceOrigin_PROVIDER_SURFACE_ORIGIN_MANUAL,
				Access: &providerv1.ProviderSurfaceRuntime_Api{
					Api: &providerv1.ProviderAPISurfaceRuntime{
						Protocol: apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE,
						BaseUrl:  "https://api.example.com/v1",
					},
				},
			},
		}},
	}}})
	err := checker.CheckCredentialReferences(t.Context(), "credential-1")
	if err == nil {
		t.Fatal("CheckCredentialReferences() error = nil, want conflict")
	}
	var conflictErr *domainerror.ReferenceConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("CheckCredentialReferences() error = %T, want ReferenceConflictError", err)
	}
}

type referenceProviderStore struct {
	items []*providerv1.Provider
}

func (s referenceProviderStore) List(context.Context) ([]*providerv1.Provider, error) {
	items := make([]*providerv1.Provider, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, proto.Clone(item).(*providerv1.Provider))
	}
	return items, nil
}

func (referenceProviderStore) Get(context.Context, string) (*providerv1.Provider, error) {
	return nil, fmt.Errorf("unexpected Get")
}

func (referenceProviderStore) Upsert(context.Context, *providerv1.Provider) (*providerv1.Provider, error) {
	return nil, fmt.Errorf("unexpected Upsert")
}

func (referenceProviderStore) Update(context.Context, string, func(*providerv1.Provider) error) (*providerv1.Provider, error) {
	return nil, fmt.Errorf("unexpected Update")
}

func (referenceProviderStore) Delete(context.Context, string) error {
	return fmt.Errorf("unexpected Delete")
}
