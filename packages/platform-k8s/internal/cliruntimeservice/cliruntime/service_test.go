package cliruntime

import (
	"context"
	"testing"

	cliversions "code-code.internal/platform-k8s/internal/cliruntimeservice/cliversions"
)

type stubVersionSyncer struct {
	result *cliversions.SyncResult
	err    error
}

func (s stubVersionSyncer) Sync(context.Context) (*cliversions.SyncResult, error) {
	return s.result, s.err
}

type recordingDispatcher struct {
	requests []ImageBuildRequest
	err      error
}

func (d *recordingDispatcher) DispatchImageBuild(_ context.Context, request ImageBuildRequest) error {
	d.requests = append(d.requests, request)
	return d.err
}

func TestServiceSyncCLIVersionsDispatchesBuildRequests(t *testing.T) {
	dispatcher := &recordingDispatcher{}
	service, err := NewService(Config{
		VersionSyncer: stubVersionSyncer{result: &cliversions.SyncResult{
			Changes: []cliversions.VersionChange{{
				CLIID: "gemini-cli",
				Current: cliversions.Snapshot{
					Version: "0.9.0",
				},
			}},
		}},
		Dispatcher:     dispatcher,
		ImageRegistry:  "registry.internal/platform",
		SourceContext:  "https://git.example.com/code-code.git",
		SourceRevision: "main",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.SyncCLIVersions(context.Background())
	if err != nil {
		t.Fatalf("SyncCLIVersions() error = %v", err)
	}
	if got, want := len(dispatcher.requests), 1; got != want {
		t.Fatalf("dispatched requests = %d, want %d", got, want)
	}
	if got, want := result.VersionChangeCount, 1; got != want {
		t.Fatalf("versionChangeCount = %d, want %d", got, want)
	}
	if got, want := len(result.ImageBuildRequests), 1; got != want {
		t.Fatalf("imageBuildRequests = %d, want %d", got, want)
	}
}

func TestServiceAllowsImageBuildDisabled(t *testing.T) {
	dispatcher := &recordingDispatcher{}
	service, err := NewService(Config{
		VersionSyncer: stubVersionSyncer{result: &cliversions.SyncResult{
			Changes: []cliversions.VersionChange{{
				CLIID: "gemini-cli",
				Current: cliversions.Snapshot{
					Version: "0.9.0",
				},
			}},
		}},
		Dispatcher: dispatcher,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.SyncCLIVersions(context.Background())
	if err != nil {
		t.Fatalf("SyncCLIVersions() error = %v", err)
	}
	if got, want := len(dispatcher.requests), 0; got != want {
		t.Fatalf("dispatched requests = %d, want %d", got, want)
	}
	if got, want := result.VersionChangeCount, 1; got != want {
		t.Fatalf("versionChangeCount = %d, want %d", got, want)
	}
	if got, want := len(result.ImageBuildRequests), 0; got != want {
		t.Fatalf("imageBuildRequests = %d, want %d", got, want)
	}
}
