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
	}, {
		CLIID: "antigravity",
		Current: cliversions.Snapshot{
			Version: "1.0.0",
		},
	}})

	if got, want := len(requests), 2; got != want {
		t.Fatalf("requests = %d, want %d", got, want)
	}
	if got, want := requests[0].RequestID, "cli-image-build:gemini-cli:0.9.0:agent-cli-gemini"; got != want {
		t.Fatalf("requests[0] requestID = %q, want %q", got, want)
	}
	if got, want := requests[1].RequestID, "cli-image-build:codex:0.121.0:agent-cli-codex"; got != want {
		t.Fatalf("requests[1] requestID = %q, want %q", got, want)
	}
	if got, want := requests[1].Image, "registry.internal/platform/code-code/agent-cli-codex:cli-0.121.0"; got != want {
		t.Fatalf("requests[1] image = %q, want %q", got, want)
	}
}

func TestImageBuildTagSanitizesVersion(t *testing.T) {
	if got, want := imageBuildTag("1.2.3+build/7"), "cli-1.2.3-build-7"; got != want {
		t.Fatalf("tag = %q, want %q", got, want)
	}
}

func TestImageBuildJobUsesDockerfileContents(t *testing.T) {
	expected := []string{
		"DOCKERFILE_CONTENTS",
		"buildctl-daemonless.sh build",
		"--frontend dockerfile.v0",
		"--local dockerfile=",
	}
	for _, fragment := range expected {
		if !strings.Contains(buildAndPushScript, fragment) {
			t.Fatalf("buildAndPushScript missing %q", fragment)
		}
	}
}

func TestImageBuildPlannerAppliesRegistry(t *testing.T) {
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
}

func TestNewImageBuildPlannerRequiresRegistry(t *testing.T) {
	if _, err := newImageBuildPlanner(""); err == nil {
		t.Fatalf("newImageBuildPlanner() expected registry error")
	}
}

func TestBuildDockerfileContentsForAllTargets(t *testing.T) {
	for _, target := range []string{"claude-code-agent", "agent-cli-gemini", "agent-cli-qwen", "agent-cli-codex"} {
		content, err := buildDockerfileContents(target)
		if err != nil {
			t.Fatalf("buildDockerfileContents(%q) error = %v", target, err)
		}
		if !strings.Contains(content, "FROM node:24-bookworm-slim") {
			t.Fatalf("dockerfile for %q missing base image", target)
		}
		if !strings.Contains(content, "agent-entrypoint.sh") {
			t.Fatalf("dockerfile for %q missing entrypoint install", target)
		}
	}
}

func mustImageBuildPlanner(t *testing.T) imageBuildPlanner {
	t.Helper()
	planner, err := newImageBuildPlanner("registry.internal/platform")
	if err != nil {
		t.Fatalf("newImageBuildPlanner() error = %v", err)
	}
	return planner
}
