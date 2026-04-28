package observability

import (
	"context"
	"fmt"
	"testing"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	providerv1 "code-code.internal/go-contract/provider/v1"
	"google.golang.org/protobuf/proto"
)

func TestListCLIProviderSurfaceBindingBindings(t *testing.T) {
	store := observabilityProviderStore{items: []*providerv1.Provider{
		testProvider("codex-provider", "credential-codex", testCLIEndpoint("codex-provider", "codex-instance", "codex")),
		testProvider("api-provider", "credential-api", testAPIEndpoint("api-provider", "api-instance")),
	}}
	bindings, err := listCLIProviderSurfaceBindingBindings(context.Background(), store)
	if err != nil {
		t.Fatalf("listCLIProviderSurfaceBindingBindings() error = %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("len(bindings) = %d, want 1", len(bindings))
	}
	if got, want := bindings[0].ProviderSurfaceBindingID, "codex-instance"; got != want {
		t.Fatalf("ProviderSurfaceBindingID = %q, want %q", got, want)
	}
	if got, want := bindings[0].CliID, "codex"; got != want {
		t.Fatalf("CliID = %q, want %q", got, want)
	}
	if got, want := bindings[0].CredentialID, "credential-codex"; got != want {
		t.Fatalf("CredentialID = %q, want %q", got, want)
	}
}

type observabilityProviderStore struct {
	items []*providerv1.Provider
}

func (s observabilityProviderStore) List(context.Context) ([]*providerv1.Provider, error) {
	items := make([]*providerv1.Provider, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, proto.Clone(item).(*providerv1.Provider))
	}
	return items, nil
}

func (observabilityProviderStore) Get(context.Context, string) (*providerv1.Provider, error) {
	return nil, fmt.Errorf("unexpected Get")
}

func (observabilityProviderStore) Upsert(context.Context, *providerv1.Provider) (*providerv1.Provider, error) {
	return nil, fmt.Errorf("unexpected Upsert")
}

func (observabilityProviderStore) Update(context.Context, string, func(*providerv1.Provider) error) (*providerv1.Provider, error) {
	return nil, fmt.Errorf("unexpected Update")
}

func (observabilityProviderStore) Delete(context.Context, string) error {
	return fmt.Errorf("unexpected Delete")
}

func testProvider(providerID, credentialID string, endpoint *providerv1.ProviderSurfaceBinding) *providerv1.Provider {
	endpoint.ProviderCredentialRef = &providerv1.ProviderCredentialRef{ProviderCredentialId: credentialID}
	return &providerv1.Provider{
		ProviderId:  providerID,
		DisplayName: providerID,
		Surfaces:    []*providerv1.ProviderSurfaceBinding{endpoint},
	}
}

func testCLIEndpoint(_ string, surfaceID, cliID string) *providerv1.ProviderSurfaceBinding {
	return &providerv1.ProviderSurfaceBinding{
		SurfaceId: surfaceID,
		Runtime: &providerv1.ProviderSurfaceRuntime{
			DisplayName: surfaceID,
			Origin:      providerv1.ProviderSurfaceOrigin_PROVIDER_SURFACE_ORIGIN_MANUAL,
			Access: &providerv1.ProviderSurfaceRuntime_Cli{
				Cli: &providerv1.ProviderCLISurfaceRuntime{
					CliId: cliID,
				},
			},
		},
	}
}

func testAPIEndpoint(_ string, surfaceID string) *providerv1.ProviderSurfaceBinding {
	return &providerv1.ProviderSurfaceBinding{
		SurfaceId: surfaceID,
		Runtime: &providerv1.ProviderSurfaceRuntime{
			DisplayName: surfaceID,
			Origin:      providerv1.ProviderSurfaceOrigin_PROVIDER_SURFACE_ORIGIN_MANUAL,
			Access: &providerv1.ProviderSurfaceRuntime_Api{
				Api: &providerv1.ProviderAPISurfaceRuntime{
					Protocol: apiprotocolv1.Protocol_PROTOCOL_OPENAI_COMPATIBLE,
					BaseUrl:  "https://api.example.com/v1",
				},
			},
		},
	}
}
