package supportservice

import (
	"context"
	"testing"

	supportv1 "code-code.internal/go-contract/platform/support/v1"
	providerv1 "code-code.internal/go-contract/provider/v1"
	"code-code.internal/platform-k8s/internal/platform/testutil"
)

func TestServerListCLIsReturnsSupportData(t *testing.T) {
	server, err := NewServer(Config{
		Reader:    testutil.NewEmptyClient(),
		Namespace: "code-code",
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	response, err := server.ListCLIs(context.Background(), &supportv1.ListCLIsRequest{})
	if err != nil {
		t.Fatalf("ListCLIs() error = %v", err)
	}
	claude := findCLI(response.GetItems(), "claude-code")
	if claude == nil {
		t.Fatal("claude-code support data not found")
	}
	if got, want := claude.GetOfficialVersionSource().GetNpmDistTag().GetPackageName(), "@anthropic-ai/claude-code"; got != want {
		t.Fatalf("claude-code version source = %q, want %q", got, want)
	}
	if got, want := claude.GetContainerImages()[0].GetImage(), "code-code/claude-code-agent:0.0.0"; got != want {
		t.Fatalf("claude-code image = %q, want %q", got, want)
	}
}

func TestServerListVendorsReturnsSupportData(t *testing.T) {
	server, err := NewServer(Config{
		Reader:    testutil.NewEmptyClient(),
		Namespace: "code-code",
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	response, err := server.ListVendors(context.Background(), &supportv1.ListVendorsRequest{})
	if err != nil {
		t.Fatalf("ListVendors() error = %v", err)
	}
	if len(response.GetItems()) == 0 {
		t.Fatal("ListVendors() returned no support data")
	}
}

func TestServerListProviderSurfacesReturnsSupportData(t *testing.T) {
	server, err := NewServer(Config{
		Reader:    testutil.NewEmptyClient(),
		Namespace: "code-code",
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	response, err := server.ListProviderSurfaces(context.Background(), &supportv1.ListProviderSurfacesRequest{})
	if err != nil {
		t.Fatalf("ListProviderSurfaces() error = %v", err)
	}
	if findSurface(response.GetItems(), "openai-compatible") == nil {
		t.Fatal("openai-compatible surface not found")
	}
}

func findCLI(items []*supportv1.CLI, cliID string) *supportv1.CLI {
	for _, item := range items {
		if item.GetCliId() == cliID {
			return item
		}
	}
	return nil
}

func findSurface(items []*providerv1.ProviderSurface, surfaceID string) *providerv1.ProviderSurface {
	for _, item := range items {
		if item.GetSurfaceId() == surfaceID {
			return item
		}
	}
	return nil
}
