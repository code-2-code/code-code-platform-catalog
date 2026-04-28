package cliruntime

import (
	"context"
	"sync"
	"testing"
	"time"

	cliruntimev1 "code-code.internal/go-contract/platform/cli_runtime/v1"
	"code-code.internal/platform-k8s/internal/cliruntimeservice/cliversions"
)

func TestServerListCLIRuntimeRecordsReturnsVersionsAndImages(t *testing.T) {
	versionStore := &staticVersionStore{state: &cliversions.State{
		Versions: map[string]cliversions.Snapshot{
			"gemini-cli": {
				Version:   "0.9.0",
				UpdatedAt: time.Date(2024, 4, 13, 8, 0, 0, 0, time.UTC),
			},
		},
	}}
	server, err := NewServer(ServerConfig{
		Versions:      versionStore,
		ImageRegistry: "registry.internal/platform",
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	response, err := server.ListCLIRuntimeRecords(context.Background(), &cliruntimev1.ListCLIRuntimeRecordsRequest{
		CliId: "gemini-cli",
	})
	if err != nil {
		t.Fatalf("ListCLIRuntimeRecords() error = %v", err)
	}
	if got, want := len(response.GetItems()), 1; got != want {
		t.Fatalf("items = %d, want %d", got, want)
	}
	item := response.GetItems()[0]
	if got, want := item.GetVersion().GetVersion(), "0.9.0"; got != want {
		t.Fatalf("version = %q, want %q", got, want)
	}
	if got, want := item.GetImages()[0].GetImage(), "registry.internal/platform/code-code/agent-cli-gemini:cli-0.9.0"; got != want {
		t.Fatalf("image = %q, want %q", got, want)
	}
	if got, want := item.GetImages()[0].GetBuildTarget(), "agent-cli-gemini"; got != want {
		t.Fatalf("build target = %q, want %q", got, want)
	}
}

func TestServerGetLatestAvailableCLIRuntimeImagesReadsRegistryTags(t *testing.T) {
	server, err := NewServer(ServerConfig{
		Versions:      &staticVersionStore{state: &cliversions.State{Versions: map[string]cliversions.Snapshot{}}},
		Registry:      fakeRegistryTags{"registry.internal/platform/code-code/agent-cli-gemini": {"cli-0.8.0", "buildcache", "cli-0.10.0", "cli-0.9.0"}},
		ImageRegistry: "registry.internal/platform",
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	response, err := server.GetLatestAvailableCLIRuntimeImages(context.Background(), &cliruntimev1.GetLatestAvailableCLIRuntimeImagesRequest{
		CliId: "gemini-cli",
	})
	if err != nil {
		t.Fatalf("GetLatestAvailableCLIRuntimeImages() error = %v", err)
	}
	if got, want := len(response.GetItems()), 1; got != want {
		t.Fatalf("items = %d, want %d", got, want)
	}
	item := response.GetItems()[0]
	if got, want := item.GetCliVersion(), "0.10.0"; got != want {
		t.Fatalf("cli version = %q, want %q", got, want)
	}
	if got, want := item.GetImage(), "registry.internal/platform/code-code/agent-cli-gemini:cli-0.10.0"; got != want {
		t.Fatalf("image = %q, want %q", got, want)
	}
}

func TestServerGetLatestAvailableCLIRuntimeImagesSplitsLookupAndPullRegistry(t *testing.T) {
	server, err := NewServer(ServerConfig{
		Versions:            &staticVersionStore{state: &cliversions.State{Versions: map[string]cliversions.Snapshot{}}},
		Registry:            fakeRegistryTags{"registry.internal-api/platform/code-code/agent-cli-qwen": {"cli-0.14.5"}},
		ImageRegistry:       "registry.internal-pull/platform",
		ImageRegistryLookup: "registry.internal-api/platform",
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	response, err := server.GetLatestAvailableCLIRuntimeImages(context.Background(), &cliruntimev1.GetLatestAvailableCLIRuntimeImagesRequest{
		CliId: "qwen-cli",
	})
	if err != nil {
		t.Fatalf("GetLatestAvailableCLIRuntimeImages() error = %v", err)
	}
	if got, want := len(response.GetItems()), 1; got != want {
		t.Fatalf("items = %d, want %d", got, want)
	}
	if got, want := response.GetItems()[0].GetImage(), "registry.internal-pull/platform/code-code/agent-cli-qwen:cli-0.14.5"; got != want {
		t.Fatalf("image = %q, want %q", got, want)
	}
}

func TestServerAllowsMissingImageRegistry(t *testing.T) {
	versionStore := &staticVersionStore{state: &cliversions.State{
		Versions: map[string]cliversions.Snapshot{
			"gemini-cli": {
				Version: "0.9.0",
			},
		},
	}}
	server, err := NewServer(ServerConfig{
		Versions: versionStore,
		Registry: fakeRegistryTags{
			"registry.internal/platform/code-code/agent-cli-gemini": {"cli-0.9.0"},
		},
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	records, err := server.ListCLIRuntimeRecords(context.Background(), &cliruntimev1.ListCLIRuntimeRecordsRequest{
		CliId: "gemini-cli",
	})
	if err != nil {
		t.Fatalf("ListCLIRuntimeRecords() error = %v", err)
	}
	if got, want := len(records.GetItems()), 1; got != want {
		t.Fatalf("items = %d, want %d", got, want)
	}
	if got, want := len(records.GetItems()[0].GetImages()), 0; got != want {
		t.Fatalf("images = %d, want %d", got, want)
	}

	images, err := server.GetLatestAvailableCLIRuntimeImages(context.Background(), &cliruntimev1.GetLatestAvailableCLIRuntimeImagesRequest{
		CliId: "gemini-cli",
	})
	if err != nil {
		t.Fatalf("GetLatestAvailableCLIRuntimeImages() error = %v", err)
	}
	if got, want := len(images.GetItems()), 0; got != want {
		t.Fatalf("latest images = %d, want %d", got, want)
	}
}

func TestServerGetLatestAvailableCLIRuntimeImagesTimesOutRegistry(t *testing.T) {
	started := make(chan struct{})
	server, err := NewServer(ServerConfig{
		Versions:        &staticVersionStore{state: &cliversions.State{Versions: map[string]cliversions.Snapshot{}}},
		Registry:        &blockingRegistry{started: started},
		ImageRegistry:   "registry.internal/platform",
		RegistryTimeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := server.GetLatestAvailableCLIRuntimeImages(context.Background(), &cliruntimev1.GetLatestAvailableCLIRuntimeImagesRequest{
			CliId: "gemini-cli",
		})
		done <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("registry ListTags was not called")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("GetLatestAvailableCLIRuntimeImages() error = nil, want timeout")
		}
	case <-time.After(time.Second):
		t.Fatal("GetLatestAvailableCLIRuntimeImages() did not return after registry timeout")
	}
}

type staticVersionStore struct {
	state *cliversions.State
}

func (s *staticVersionStore) Load(context.Context) (*cliversions.State, error) {
	return s.state, nil
}

func (s *staticVersionStore) Save(context.Context, *cliversions.State) error {
	return nil
}

type fakeRegistryTags map[string][]string

func (f fakeRegistryTags) ListTags(_ context.Context, repository string) ([]string, error) {
	return append([]string(nil), f[repository]...), nil
}

type blockingRegistry struct {
	started chan<- struct{}
	once    sync.Once
}

func (b *blockingRegistry) ListTags(ctx context.Context, _ string) ([]string, error) {
	b.once.Do(func() { close(b.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}
