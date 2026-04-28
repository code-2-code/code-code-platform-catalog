package openrouter

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	openroutersdk "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
)

// SourceID is the canonical source identifier for OpenRouter.
const SourceID = "openrouter"

// FetchModels retrieves the model catalog from the OpenRouter API.
func FetchModels(ctx context.Context, httpClient *http.Client, endpoint string) ([]components.Model, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("openrouter: http client is nil")
	}

	opts := []openroutersdk.SDKOption{
		openroutersdk.WithClient(httpClient),
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("openrouter: models endpoint is required")
	}
	base := endpoint
	if strings.HasSuffix(base, "/models") {
		base = strings.TrimSuffix(base, "/models")
	} else if strings.Contains(base, "/models?") {
		parts := strings.SplitN(base, "/models?", 2)
		base = parts[0]
	}
	opts = append(opts, openroutersdk.WithServerURL(base))

	client := openroutersdk.New(opts...)
	res, err := client.Models.List(ctx, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("openrouter: request models: %w", err)
	}
	if res == nil {
		return nil, fmt.Errorf("openrouter: models returned nil response")
	}

	return res.Data, nil
}
