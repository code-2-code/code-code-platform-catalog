package sync

import (
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"code-code.internal/platform-k8s/internal/platform/testutil"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDefinitionSyncReconcilerCollectsAuthoritativeDefinitionsFromOpenRouter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if got, want := r.URL.Path, "/models"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		_, _ = w.Write([]byte(`{"data":[
			{"id":"openai/gpt-5:free","canonical_slug":"openai/gpt-5-20250201","name":"GPT-5","context_length":400000,"pricing":{"prompt":"0","completion":"0"},"supported_parameters":["tools"],"architecture":{"input_modalities":["text"],"output_modalities":["text"]},"top_provider":{"max_completion_tokens":16000}},
			{"id":"mistralai/mistral-medium-3","canonical_slug":"mistralai/mistral-medium-3","name":"Mistral Medium 3","context_length":128000,"architecture":{"input_modalities":["text"],"output_modalities":["text"]},"top_provider":{"max_completion_tokens":8000}}
		]}`))
	}))
	defer server.Close()

	client := ctrlclientfake.NewClientBuilder().
		WithScheme(testutil.NewScheme()).
		Build()

	withDefinitionSourceCollectorEndpoints(t, map[string]string{
		models.SourceIDGitHubModels:    "://bad-url",
		models.SourceIDOpenRouter:      server.URL + "/models",
		models.SourceIDModelScope:      "://bad-url",
		models.SourceIDCerebras:        "://bad-url",
		models.SourceIDNVIDIAIntegrate: "://bad-url",
		models.SourceIDHuggingFaceHub:  "://bad-url",
	})
	reconciler := newCollectionTestReconciler(client)

	snapshot, err := reconciler.collectAuthoritativeDefinitions(context.Background())
	if err != nil {
		t.Fatalf("collectAuthoritativeDefinitions() error = %v", err)
	}

	if got, want := len(snapshot.definitions), 2; got != want {
		t.Fatalf("len(definitions) = %d, want %d", got, want)
	}
	openaiDefinition := snapshot.definitions[testIdentityKey("openai", "gpt-5")]
	if got, want := openaiDefinition.GetDefinition().GetVendorId(), "openai"; got != want {
		t.Fatalf("openai vendor id = %q, want %q", got, want)
	}
	if got, want := openaiDefinition.GetDefinition().GetAliases()[0].GetValue(), "gpt-5-20250201"; got != want {
		t.Fatalf("openai snapshot alias = %q, want %q", got, want)
	}
	if got := len(openaiDefinition.GetSources()); got == 0 {
		t.Fatal("sources = 0, want at least 1")
	}
	if got, want := openaiDefinition.GetSources()[0].GetSourceId(), models.SourceIDOpenRouter; got != want {
		t.Fatalf("source id = %q, want %q", got, want)
	}
	if !equalStrings(openaiDefinition.GetSources()[0].GetBadges(), nil) {
		t.Fatalf("source badges = %#v, want nil", openaiDefinition.GetSources()[0].GetBadges())
	}
	if openaiDefinition.GetPricing() != nil {
		t.Fatalf("openai pricing = %#v, want nil", openaiDefinition.GetPricing())
	}
	if got, want := snapshot.definitions[testIdentityKey("mistral", "mistral-medium-3")].GetDefinition().GetModelId(), "mistral-medium-3"; got != want {
		t.Fatalf("mistral model id = %q, want %q", got, want)
	}
	if _, ok := snapshot.definitions[testIdentityKey("openrouter", "openai/gpt-5:free")]; ok {
		t.Fatal("unexpected openrouter proxy definition for third-party model")
	}

}

func TestDefinitionSyncReconcilerPrefersGitHubModelsMetadataOverOpenRouter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/github":
			_, _ = w.Write([]byte(`[
				{"id":"openai/gpt-5","name":"OpenAI gpt-5","supported_input_modalities":["text","image"],"supported_output_modalities":["text"],"capabilities":["tool-calling","streaming"],"limits":{"max_input_tokens":200000,"max_output_tokens":100000}}
			]`))
		case "/openrouter/models":
			_, _ = w.Write([]byte(`{"data":[
				{"id":"openai/gpt-5","canonical_slug":"openai/gpt-5-2025-08-07","name":"GPT-5","context_length":400000,"pricing":{"prompt":"0.00000125","completion":"0.00001"},"supported_parameters":["tools"],"architecture":{"input_modalities":["text"],"output_modalities":["text"]},"top_provider":{"max_completion_tokens":16000}}
			]}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := ctrlclientfake.NewClientBuilder().
		WithScheme(testutil.NewScheme()).
		Build()

	withDefinitionSourceCollectorEndpoints(t, map[string]string{
		models.SourceIDGitHubModels:    server.URL + "/github",
		models.SourceIDOpenRouter:      server.URL + "/openrouter",
		models.SourceIDModelScope:      "://bad-url",
		models.SourceIDCerebras:        "://bad-url",
		models.SourceIDNVIDIAIntegrate: "://bad-url",
		models.SourceIDHuggingFaceHub:  "://bad-url",
	})
	reconciler := newCollectionTestReconciler(client)

	snapshot, err := reconciler.collectAuthoritativeDefinitions(context.Background())
	if err != nil {
		t.Fatalf("collectAuthoritativeDefinitions() error = %v", err)
	}

	definition := snapshot.definitions[testIdentityKey("openai", "gpt-5")]
	if got, want := definition.GetDefinition().GetContextSpec().GetMaxContextTokens(), int64(200000); got != want {
		t.Fatalf("context_spec.max_context_tokens = %d, want %d", got, want)
	}
	if got, want := definition.GetDefinition().GetContextSpec().GetMaxOutputTokens(), int64(100000); got != want {
		t.Fatalf("context_spec.max_output_tokens = %d, want %d", got, want)
	}
	if got, want := len(definition.GetSources()), 2; got != want {
		t.Fatalf("len(sources) = %d, want %d", got, want)
	}
	if got, want := definition.GetSources()[0].GetSourceId(), models.SourceIDGitHubModels; got != want {
		t.Fatalf("primary source id = %q, want %q", got, want)
	}
	if got, want := definition.GetSources()[1].GetSourceId(), models.SourceIDOpenRouter; got != want {
		t.Fatalf("secondary source id = %q, want %q", got, want)
	}
	if definition.GetPricing() != nil {
		t.Fatalf("pricing = %#v, want nil (catalog does not include proxy pricing)", definition.GetPricing())
	}
}

func TestDefinitionSyncReconcilerAllowsEmptySnapshotWhenAllSourcesUnavailable(t *testing.T) {
	client := ctrlclientfake.NewClientBuilder().
		WithScheme(testutil.NewScheme()).
		Build()

	withDefinitionSourceCollectorEndpoints(t, map[string]string{
		models.SourceIDGitHubModels:    "://bad-url",
		models.SourceIDOpenRouter:      "://bad-url",
		models.SourceIDModelScope:      "://bad-url",
		models.SourceIDCerebras:        "://bad-url",
		models.SourceIDNVIDIAIntegrate: "://bad-url",
		models.SourceIDHuggingFaceHub:  "://bad-url",
	})
	reconciler := newCollectionTestReconciler(client)

	snapshot, err := reconciler.collectAuthoritativeDefinitions(context.Background())
	if err != nil {
		t.Fatalf("collectAuthoritativeDefinitions() error = %v, want nil", err)
	}
	if snapshot == nil {
		t.Fatal("snapshot = nil, want non-nil")
	}
	if got := len(snapshot.definitions); got != 0 {
		t.Fatalf("len(definitions) = %d, want 0", got)
	}
}

func newCollectionTestReconciler(client ctrlclient.Client) *DefinitionSyncReconciler {
	r := &DefinitionSyncReconciler{
		client:    client,
		namespace: "code-code",
		logger:    slog.Default(),
	}
	r.listVendors = r.listConfiguredVendorsDefault
	r.newHTTPClient = r.newCollectionHTTPClientDefault
	return r
}

func withDefinitionSourceCollectorEndpoints(t *testing.T, endpoints map[string]string) {
	t.Helper()
	ensureDefinitionSourceCollectors()
	previous := map[string]definitionSourceCollectorSpec{}
	for sourceID, endpoint := range endpoints {
		sourceID = models.NormalizedVendorSlug(sourceID)
		spec, ok := definitionSourceCollectors[sourceID]
		if !ok {
			t.Fatalf("unknown definition source collector %q", sourceID)
		}
		if _, ok := previous[sourceID]; !ok {
			previous[sourceID] = spec
		}
		spec.endpoint = strings.TrimSpace(endpoint)
		definitionSourceCollectors[sourceID] = spec
	}
	t.Cleanup(func() {
		for sourceID, spec := range previous {
			definitionSourceCollectors[sourceID] = spec
		}
	})
}
