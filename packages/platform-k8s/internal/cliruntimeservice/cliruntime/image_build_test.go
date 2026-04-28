package cliruntime

import (
	"strings"
	"testing"
	"time"

	cliversions "code-code.internal/platform-k8s/internal/cliruntimeservice/cliversions"
)

func TestImageBuildPlannerUsesRunnableCLIImages(t *testing.T) {
	planner := mustImageBuildPlanner(t)
	requests := planner.RequestsForChanges([]cliversions.VersionChange{{
		CLIID: "gemini-cli",
		Previous: cliversions.Snapshot{
			Version: "0.8.0",
		},
		Current: cliversions.Snapshot{
			Version:   "0.9.0",
			UpdatedAt: time.Unix(1713480000, 0),
		},
	}, {
		CLIID: "codex",
		Current: cliversions.Snapshot{
			Version: "0.121.0",
		},
	}})

	if got, want := len(requests), 1; got != want {
		t.Fatalf("requests = %d, want %d", got, want)
	}
	request := requests[0]
	if got, want := request.RequestID, "cli-image-build:gemini-cli:0.9.0:agent-cli-gemini"; got != want {
		t.Fatalf("requestID = %q, want %q", got, want)
	}
	if got, want := request.Image, "registry.internal/platform/code-code/agent-cli-gemini:cli-0.9.0"; got != want {
		t.Fatalf("image = %q, want %q", got, want)
	}
	if got, want := request.PreviousCLIVersion, "0.8.0"; got != want {
		t.Fatalf("previous version = %q, want %q", got, want)
	}
}

func TestImageBuildTagSanitizesVersion(t *testing.T) {
	if got, want := imageBuildTag("1.2.3+build/7"), "cli-1.2.3-build-7"; got != want {
		t.Fatalf("tag = %q, want %q", got, want)
	}
}

func TestImageBuildJobUsesSharedNodeCLIImageDockerfile(t *testing.T) {
	expected := []string{
		`filename="deploy/images/release/node-cli-agent.Dockerfile"`,
		`build-arg:AGENT_DIR="${agent_dir}"`,
		`build-arg:CLI_PACKAGE="${cli_package}"`,
		`build-arg:CLI_VERSION="${CLI_VERSION}"`,
		`claude-code-agent) agent_dir="claude-code"; cli_package="@anthropic-ai/claude-code"`,
		`agent-cli-qwen) agent_dir="qwen-cli"; cli_package="@qwen-code/qwen-code"`,
		`agent-cli-gemini) agent_dir="gemini-cli"; cli_package="@google/gemini-cli"`,
	}
	for _, fragment := range expected {
		if !strings.Contains(buildAndPushScript, fragment) {
			t.Fatalf("buildAndPushScript missing %q", fragment)
		}
	}
}

func TestImageBuildPlannerAppliesRegistryAndSource(t *testing.T) {
	planner := mustImageBuildPlanner(t)
	requests := planner.RequestsForChanges([]cliversions.VersionChange{{
		CLIID: "gemini-cli",
		Current: cliversions.Snapshot{
			Version: "0.9.0",
		},
	}})
	if got, want := requests[0].Image, "registry.internal/platform/code-code/agent-cli-gemini:cli-0.9.0"; got != want {
		t.Fatalf("image = %q, want %q", got, want)
	}
	if got, want := requests[0].SourceContext, "https://git.example.com/code-code.git"; got != want {
		t.Fatalf("sourceContext = %q, want %q", got, want)
	}
}

func TestNewImageBuildPlannerRequiresRegistryAndSource(t *testing.T) {
	if _, err := newImageBuildPlanner("", "https://git.example.com/code-code.git", "main"); err == nil {
		t.Fatalf("newImageBuildPlanner() expected registry error")
	}
	if _, err := newImageBuildPlanner("registry.internal/platform", "", "main"); err == nil {
		t.Fatalf("newImageBuildPlanner() expected source context error")
	}
}

func mustImageBuildPlanner(t *testing.T) imageBuildPlanner {
	t.Helper()
	planner, err := newImageBuildPlanner(
		"registry.internal/platform",
		"https://git.example.com/code-code.git",
		"main",
	)
	if err != nil {
		t.Fatalf("newImageBuildPlanner() error = %v", err)
	}
	return planner
}
