package huggingface

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	gosync "sync"

	"code-code.internal/platform-k8s/internal/modelservice/modelidentity"
	"code-code.internal/platform-k8s/internal/modelservice/models/source"
)

const maxConcurrency = 4

// SourceID is the canonical source identifier for HuggingFace Hub.
const SourceID = "huggingface-hub"

const pageSize = "100"

// Model represents one entry from the HuggingFace Hub API.
type Model struct {
	ID          string   `json:"id"`
	ModelID     string   `json:"modelId"`
	PipelineTag string   `json:"pipeline_tag"`
	Tags        []string `json:"tags"`
}

// FetchAllModels fetches models across all vendor authors with bounded concurrency.
// vendorIDs are the configured vendor identifiers; aliasCandidates maps a vendor ID
// to its alias candidates for author resolution. isUnavailable tests whether an error
// indicates the endpoint is down (to cancel remaining probes early).
func FetchAllModels(
	ctx context.Context,
	httpClient *http.Client,
	endpoint string,
	vendorIDs []string,
	aliasCandidates func(string) []string,
	isUnavailable func(error) bool,
	logger *slog.Logger,
) []Model {
	type fetchTask struct {
		vendorID string
		author   string
	}
	sorted := make([]string, len(vendorIDs))
	copy(sorted, vendorIDs)
	slices.Sort(sorted)

	var tasks []fetchTask
	for _, vendorID := range sorted {
		for _, author := range AuthorCandidates(vendorID, aliasCandidates) {
			tasks = append(tasks, fetchTask{vendorID: vendorID, author: author})
		}
	}
	if len(tasks) == 0 {
		return nil
	}

	fetchCtx, fetchCancel := context.WithCancel(ctx)
	defer fetchCancel()

	var mu gosync.Mutex
	items := make([]Model, 0, 64)
	sem := make(chan struct{}, maxConcurrency)
	var wg gosync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t fetchTask) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-fetchCtx.Done():
				return
			}
			page, err := FetchModels(fetchCtx, httpClient, endpoint, t.author)
			if err != nil {
				if isUnavailable != nil && isUnavailable(err) {
					logger.Warn("huggingface endpoint unavailable; cancel remaining author probes",
						"vendor_id", t.vendorID, "author", t.author, "error", err)
					fetchCancel()
					return
				}
				logger.Warn("skip huggingface author collection",
					"vendor_id", t.vendorID, "author", t.author, "error", err)
				return
			}
			mu.Lock()
			items = append(items, page...)
			mu.Unlock()
		}(task)
	}
	wg.Wait()
	return items
}

// FetchModels retrieves the model catalog from the HuggingFace API for a given author.
func FetchModels(ctx context.Context, httpClient *http.Client, endpoint string, author string) ([]Model, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("huggingface: http client is nil")
	}
	author = strings.TrimSpace(author)
	if author == "" {
		return nil, nil
	}
	nextURL, err := buildModelsURL(endpoint, author)
	if err != nil {
		return nil, err
	}

	var out []Model
	for nextURL != "" {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("huggingface: build request: %w", err)
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", "code-code-platform-k8s-models")

		response, err := httpClient.Do(request)
		if err != nil {
			return nil, fmt.Errorf("huggingface: request models: %w", err)
		}
		var page []Model
		decodeErr := json.NewDecoder(response.Body).Decode(&page)
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("huggingface: models status %d", response.StatusCode)
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("huggingface: decode models: %w", decodeErr)
		}
		out = append(out, page...)
		nextURL = nextLinkURL(response.Header.Get("Link"))
	}
	return out, nil
}

// Normalize transforms raw HuggingFace models into grouped CollectedEntry maps.
func Normalize(items []Model, ctx source.CollectionContext) map[string][]*source.CollectedEntry {
	return source.NormalizeHostedModels(SourceID, items, ctx, func(item Model) (string, string, string, bool, bool) {
		modelID := strings.TrimSpace(item.ModelID)
		if modelID == "" {
			modelID = strings.TrimSpace(item.ID)
		}
		owner, rawModelID, ok := strings.Cut(modelID, "/")
		if !ok {
			return "", "", "", false, false
		}
		if shouldSkip(item, rawModelID) {
			return "", "", "", false, false
		}
		return owner, rawModelID, rawModelID, true, true
	})
}

// AuthorCandidates returns candidate HuggingFace author names for a vendor.
func AuthorCandidates(vendorID string, aliasCandidates func(string) []string) []string {
	candidates := []string{vendorID}
	if aliasCandidates != nil {
		candidates = append(candidates, aliasCandidates(vendorID)...)
	}
	candidates = append(candidates, titleCaseVendorID(vendorID))

	out := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	slices.Sort(out)
	return out
}

func shouldSkip(item Model, rawModelID string) bool {
	if strings.TrimSpace(strings.ToLower(item.PipelineTag)) != "text-generation" {
		return true
	}
	if modelidentity.HasChannelToken(rawModelID) || modelidentity.HasModelToken(rawModelID, "awq", "gguf", "gptq", "mlx", "onnx") {
		return true
	}
	for _, tag := range item.Tags {
		normalized := strings.TrimSpace(strings.ToLower(tag))
		switch normalized {
		case "awq", "gguf", "gptq", "mlx", "onnx":
			return true
		}
		if strings.HasPrefix(normalized, "base_model:quantized:") {
			return true
		}
	}
	return false
}

func titleCaseVendorID(vendorID string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(vendorID), func(r rune) bool {
		return r == '-' || r == '_'
	})
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "-")
}

func buildModelsURL(endpoint string, author string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", fmt.Errorf("huggingface: models endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("huggingface: parse endpoint: %w", err)
	}
	query := parsed.Query()
	query.Set("author", strings.TrimSpace(author))
	query.Set("pipeline_tag", "text-generation")
	query.Set("limit", pageSize)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func nextLinkURL(linkHeader string) string {
	for _, part := range strings.Split(strings.TrimSpace(linkHeader), ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start >= 0 && end > start+1 {
			return strings.TrimSpace(part[start+1 : end])
		}
	}
	return ""
}
