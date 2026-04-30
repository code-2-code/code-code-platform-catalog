package supportservice

import (
	"context"
	"testing"

	supportv1 "code-code.internal/go-contract/platform/support/v1"
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
	codex := findCLI(response.GetItems(), "codex")
	if codex == nil {
		t.Fatal("codex support data not found")
	}
	if codex.GetOauth().GetObservability() == nil {
		t.Fatal("codex oauth observability = nil, want support observability profiles")
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
	google := findVendor(response.GetItems(), "google")
	if google == nil {
		t.Fatal("google vendor support data not found")
	}
	if len(google.GetProviderBindings()) == 0 || google.GetProviderBindings()[0].GetObservability() == nil {
		t.Fatal("google vendor observability = nil, want support observability profiles")
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



func findVendor(items []*supportv1.Vendor, vendorID string) *supportv1.Vendor {
	for _, item := range items {
		if item.GetVendor().GetVendorId() == vendorID {
			return item
		}
	}
	return nil
}
