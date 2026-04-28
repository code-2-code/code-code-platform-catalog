package identity

import (
	"context"
	"testing"

	managementv1 "code-code.internal/go-contract/platform/management/v1"
	clisupport "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/support"
)

func TestListReturnsRegisteredCLIDefinitions(t *testing.T) {
	svc, err := NewCLIDefinitionManagementService()
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	items, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	clis, err := clisupport.RegisteredCLIs()
	if err != nil {
		t.Fatalf("registered clis: %v", err)
	}
	if got, want := len(items), len(clis); got != want {
		t.Fatalf("items = %d, want %d", got, want)
	}
	claude := findCLI(items, "claude-code")
	if claude == nil {
		t.Fatal("claude-code cli definition not found")
	}
	if claude.GetDisplayName() != "Claude Code" {
		t.Fatalf("display_name = %q, want Claude Code", claude.GetDisplayName())
	}
	if got, want := claude.GetIconUrl(), "https://code.claude.com/docs/favicon.ico"; got != want {
		t.Fatalf("icon_url = %q, want %q", got, want)
	}
	if len(claude.GetContainerImages()) != 1 {
		t.Fatalf("container_images = %d, want 1", len(claude.GetContainerImages()))
	}
	if got, want := claude.GetContainerImages()[0].GetImage(), "code-code/claude-code-agent:0.0.0"; got != want {
		t.Fatalf("image = %q, want %q", got, want)
	}
}

func findCLI(items []*managementv1.CLIDefinitionView, cliID string) *managementv1.CLIDefinitionView {
	for _, item := range items {
		if item.GetCliId() == cliID {
			return item
		}
	}
	return nil
}
