package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// openAIModelsResponse is the standard OpenAI-compatible /v1/models envelope.
type openAIModelsResponse[T any] struct {
	Data []T `json:"data"`
}

// FetchOpenAIModels fetches model entries from an OpenAI-compatible /v1/models endpoint.
// This covers any provider that returns { "data": [...] } — e.g. Cerebras, NVIDIA, ModelScope,
// and provider probe targets. The caller provides the concrete Model type T.
func FetchOpenAIModels[T any](ctx context.Context, httpClient *http.Client, endpoint string) ([]T, error) {
	var payload openAIModelsResponse[T]
	if err := fetchJSON(ctx, httpClient, endpoint, &payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

// FetchJSONArray fetches model entries from an endpoint that returns a bare JSON array.
// This covers APIs like GitHub Models that respond with [...] rather than { "data": [...] }.
func FetchJSONArray[T any](ctx context.Context, httpClient *http.Client, endpoint string) ([]T, error) {
	var payload []T
	if err := fetchJSON(ctx, httpClient, endpoint, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func fetchJSON(ctx context.Context, httpClient *http.Client, endpoint string, target any) error {
	if httpClient == nil {
		return fmt.Errorf("source: http client is nil")
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return fmt.Errorf("source: models endpoint is required")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("source: build request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "code-code-platform-k8s-models")

	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("source: request models: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		// Drain body to allow connection reuse.
		_, _ = io.Copy(io.Discard, response.Body)
		return fmt.Errorf("source: models endpoint returned status %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("source: decode models response: %w", err)
	}
	return nil
}
