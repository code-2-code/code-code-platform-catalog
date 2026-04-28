package sync

import (
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"code-code.internal/platform-k8s/internal/modelservice/models/source"
	"code-code.internal/platform-k8s/internal/modelservice/models/source/cerebras"
	"code-code.internal/platform-k8s/internal/modelservice/models/source/github"
	"code-code.internal/platform-k8s/internal/modelservice/models/source/huggingface"
	"code-code.internal/platform-k8s/internal/modelservice/models/source/modelscope"
	"code-code.internal/platform-k8s/internal/modelservice/models/source/nvidia"
	"code-code.internal/platform-k8s/internal/modelservice/models/source/openrouter"
)

type definitionSourceCollectorSpec struct {
	sourceID          string
	collectionOrder   int
	authorityPriority int
	presetVendor      bool
	endpoint          string
	timeout           time.Duration
	collect           func(context.Context, *DefinitionSyncReconciler, *http.Client, *collectedDefinitionsSnapshot)
}

// definitionSourceCollectors is the canonical collector index. Built lazily on
// first access — no init()-time registration, no panics. Tests may replace
// individual endpoint URLs for fakes.
var (
	definitionSourceCollectors     map[string]definitionSourceCollectorSpec
	definitionSourceCollectorsOnce sync.Once
)

func ensureDefinitionSourceCollectors() map[string]definitionSourceCollectorSpec {
	definitionSourceCollectorsOnce.Do(func() {
		definitionSourceCollectors = buildDefinitionSourceCollectors()
	})
	return definitionSourceCollectors
}

func buildDefinitionSourceCollectors() map[string]definitionSourceCollectorSpec {
	specs := []definitionSourceCollectorSpec{
		{
			sourceID:          models.SourceIDGitHubModels,
			collectionOrder:   100,
			authorityPriority: 600,
			presetVendor:      true,
			endpoint:          "https://models.inference.ai.azure.com/models",
			timeout:           6 * time.Second,
		collect: func(ctx context.Context, r *DefinitionSyncReconciler, httpClient *http.Client, snapshot *collectedDefinitionsSnapshot) {
				items, err := source.FetchJSONArray[github.Model](ctx, httpClient, r.definitionSourceEndpoint(models.SourceIDGitHubModels))
				if err != nil {
					r.logger.Warn("skip github models collection", "error", err)
					return
				}
				mergeCollectedDefinitionsSnapshot(snapshot, github.Normalize(items, newCollectionContext(snapshot.vendorScope)), r.logger)
			},
		},
		{
			sourceID:          models.SourceIDCerebras,
			collectionOrder:   200,
			authorityPriority: 500,
			presetVendor:      true,
			endpoint:          "https://api.cerebras.ai/public/v1/models",
			timeout:           6 * time.Second,
			collect: func(ctx context.Context, r *DefinitionSyncReconciler, httpClient *http.Client, snapshot *collectedDefinitionsSnapshot) {
				items, err := source.FetchOpenAIModels[cerebras.Model](ctx, httpClient, r.definitionSourceEndpoint(models.SourceIDCerebras))
				if err != nil {
					r.logger.Warn("skip cerebras model collection", "error", err)
					return
				}
				mergeCollectedDefinitionsSnapshot(snapshot, cerebras.Normalize(items, newCollectionContext(snapshot.vendorScope)), r.logger)
			},
		},
		{
			sourceID:          models.SourceIDNVIDIAIntegrate,
			collectionOrder:   300,
			authorityPriority: 300,
			endpoint:          "https://integrate.api.nvidia.com/v1/models",
			timeout:           6 * time.Second,
			collect: func(ctx context.Context, r *DefinitionSyncReconciler, httpClient *http.Client, snapshot *collectedDefinitionsSnapshot) {
				items, err := source.FetchOpenAIModels[nvidia.Model](ctx, httpClient, r.definitionSourceEndpoint(models.SourceIDNVIDIAIntegrate))
				if err != nil {
					r.logger.Warn("skip nvidia integrate model collection", "error", err)
					return
				}
				mergeCollectedDefinitionsSnapshot(snapshot, nvidia.Normalize(items, newCollectionContext(snapshot.vendorScope)), r.logger)
			},
		},
		{
			sourceID:          models.SourceIDHuggingFaceHub,
			collectionOrder:   400,
			authorityPriority: 200,
			endpoint:          "https://huggingface.co/api/models",
			timeout:           12 * time.Second,
			collect: func(ctx context.Context, r *DefinitionSyncReconciler, httpClient *http.Client, snapshot *collectedDefinitionsSnapshot) {
				vendorIDs := make([]string, 0, len(snapshot.configuredVendorIDs))
				for vendorID := range snapshot.configuredVendorIDs {
					vendorIDs = append(vendorIDs, vendorID)
				}
				items := huggingface.FetchAllModels(ctx, httpClient,
					r.definitionSourceEndpoint(models.SourceIDHuggingFaceHub),
					vendorIDs, snapshot.vendorScope.aliasCandidates,
					definitionSourceEndpointUnavailable, r.logger)
				if len(items) == 0 {
					return
				}
				mergeCollectedDefinitionsSnapshot(snapshot, huggingface.Normalize(items, newCollectionContext(snapshot.vendorScope)), r.logger)
			},
		},
		{
			sourceID:          models.SourceIDModelScope,
			collectionOrder:   500,
			authorityPriority: 400,
			presetVendor:      true,
			endpoint:          "https://api-inference.modelscope.cn/v1/models",
			timeout:           6 * time.Second,
			collect: func(ctx context.Context, r *DefinitionSyncReconciler, httpClient *http.Client, snapshot *collectedDefinitionsSnapshot) {
				items, err := source.FetchOpenAIModels[modelscope.Model](ctx, httpClient, r.definitionSourceEndpoint(models.SourceIDModelScope))
				if err != nil {
					r.logger.Warn("skip modelscope model collection", "error", err)
					return
				}
				mergeCollectedDefinitionsSnapshot(snapshot, modelscope.Normalize(items, newCollectionContext(snapshot.vendorScope)), r.logger)
			},
		},
		{
			sourceID:          models.SourceIDOpenRouter,
			collectionOrder:   600,
			authorityPriority: 100,
			presetVendor:      true,
			endpoint:          "https://openrouter.ai/api/v1/models?output_modalities=text",
			timeout:           15 * time.Second,
			collect: func(ctx context.Context, r *DefinitionSyncReconciler, httpClient *http.Client, snapshot *collectedDefinitionsSnapshot) {
				items, err := openrouter.FetchModels(ctx, httpClient, r.definitionSourceEndpoint(models.SourceIDOpenRouter))
				if err != nil {
					r.logger.Warn("skip openrouter model collection", "error", err)
					return
				}
				mergeCollectedDefinitionsSnapshot(snapshot, openrouter.Normalize(items, newCollectionContext(snapshot.vendorScope)), r.logger)
			},
		},
	}
	index := make(map[string]definitionSourceCollectorSpec, len(specs))
	for _, spec := range specs {
		spec.sourceID = models.NormalizedVendorSlug(spec.sourceID)
		spec.endpoint = strings.TrimSpace(spec.endpoint)
		index[spec.sourceID] = spec
	}
	return index
}

// orderedDefinitionSourceCollectors returns collectors sorted by collection order.
func orderedDefinitionSourceCollectors() []definitionSourceCollectorSpec {
	collectors := ensureDefinitionSourceCollectors()
	specs := make([]definitionSourceCollectorSpec, 0, len(collectors))
	for _, spec := range collectors {
		specs = append(specs, spec)
	}
	slices.SortFunc(specs, func(left, right definitionSourceCollectorSpec) int {
		if left.collectionOrder == right.collectionOrder {
			return strings.Compare(left.sourceID, right.sourceID)
		}
		if left.collectionOrder < right.collectionOrder {
			return -1
		}
		return 1
	})
	return specs
}

func lookupDefinitionSourceCollector(sourceID string) (definitionSourceCollectorSpec, bool) {
	spec, ok := ensureDefinitionSourceCollectors()[models.NormalizedVendorSlug(sourceID)]
	return spec, ok
}

func (r *DefinitionSyncReconciler) definitionSourceEndpoint(sourceID string) string {
	spec, ok := lookupDefinitionSourceCollector(sourceID)
	if !ok {
		return ""
	}
	return strings.TrimSpace(spec.endpoint)
}
