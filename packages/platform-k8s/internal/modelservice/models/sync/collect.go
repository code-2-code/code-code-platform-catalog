package sync

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"code-code.internal/platform-k8s/internal/platform/outboundhttp"
	vendorsupport "code-code.internal/platform-k8s/internal/supportservice/vendors/support"
)

const defaultDefinitionSourceCollectorTimeout = 10 * time.Second

type configuredVendor struct {
	name     string
	vendorID string
	aliases  []string
}

// sourceResult holds the output of a single source collector goroutine.
type sourceResult struct {
	sourceID          string
	authorityPriority int
	grouped           map[string][]*modelservicev1.CollectedModelEntry
	duration          time.Duration
	count             int
	err               error
}

func (r *DefinitionSyncReconciler) collectAuthoritativeDefinitions(ctx context.Context) (*collectedDefinitionsSnapshot, error) {
	vendors, err := r.listVendors(ctx)
	if err != nil {
		return nil, err
	}

	scope := newConfiguredVendorScope(vendors)
	if len(scope.managedIDs) == 0 {
		return newCollectedDefinitionsSnapshot(scope), nil
	}

	httpClient, err := r.newHTTPClient(ctx)
	if err != nil {
		return nil, err
	}

	collectors := orderedDefinitionSourceCollectors()
	results := make(chan sourceResult, len(collectors))

	// Fan-out: each collector runs independently in its own goroutine.
	for _, collector := range collectors {
		go func(c definitionSourceCollectorSpec) {
			results <- r.collectDefinitionSourceIsolated(ctx, c, httpClient, scope)
		}(collector)
	}

	// Fan-in: collect all results, record metrics.
	collected := make([]sourceResult, 0, len(collectors))
	for range collectors {
		result := <-results
		r.metrics.recordCollectorRun(result.sourceID, result.duration, result.count, result.err)
		collected = append(collected, result)
	}

	// Sort by authorityPriority descending so high-priority sources merge first.
	sort.Slice(collected, func(i, j int) bool {
		if collected[i].authorityPriority != collected[j].authorityPriority {
			return collected[i].authorityPriority > collected[j].authorityPriority
		}
		return collected[i].sourceID < collected[j].sourceID
	})

	snapshot := newCollectedDefinitionsSnapshot(scope)
	for _, result := range collected {
		if result.err != nil {
			r.logger.Warn("skip source collection due to error",
				"source_id", result.sourceID,
				"error", result.err,
			)
			continue
		}
		if result.grouped != nil {
			mergeCollectedDefinitionsSnapshot(snapshot, result.grouped, r.logger)
		}
	}

	if len(snapshot.definitions) == 0 {
		r.logger.Warn(
			"model definition collection returned empty snapshot; preserve existing managed definitions until upstream sources recover",
			"configured_vendor_count", len(snapshot.managedVendorIDs),
		)
	}
	return snapshot, nil
}

// collectDefinitionSourceIsolated runs a single collector with its own local
// snapshot and returns the grouped results. It does not write to any shared
// state, making it safe to call concurrently.
func (r *DefinitionSyncReconciler) collectDefinitionSourceIsolated(
	ctx context.Context,
	collector definitionSourceCollectorSpec,
	httpClient *http.Client,
	scope configuredVendorScope,
) sourceResult {
	if collector.collect == nil {
		return sourceResult{sourceID: collector.sourceID, authorityPriority: collector.authorityPriority}
	}

	timeout := collector.timeout
	if timeout <= 0 {
		timeout = defaultDefinitionSourceCollectorTimeout
	}
	sourceCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use an isolated HTTP client per source to avoid connection pool interference.
	sourceHTTPClient := &http.Client{
		Timeout:   timeout,
		Transport: httpClient.Transport,
	}

	// Each collector writes into its own local snapshot.
	localSnapshot := newCollectedDefinitionsSnapshot(scope)
	started := time.Now()
	collector.collect(sourceCtx, r, sourceHTTPClient, localSnapshot)
	elapsed := time.Since(started)

	count := countCollectedModels(localSnapshot)
	return sourceResult{
		sourceID:          collector.sourceID,
		authorityPriority: collector.authorityPriority,
		grouped:           extractGroupedDefinitions(localSnapshot),
		duration:          elapsed,
		count:             count,
	}
}

// countCollectedModels returns the total number of model definitions in a local snapshot.
func countCollectedModels(snapshot *collectedDefinitionsSnapshot) int {
	if snapshot == nil {
		return 0
	}
	return len(snapshot.definitions)
}

// extractGroupedDefinitions converts a local snapshot back to grouped form
// for merging into the global snapshot.
func extractGroupedDefinitions(snapshot *collectedDefinitionsSnapshot) map[string][]*modelservicev1.CollectedModelEntry {
	if snapshot == nil || len(snapshot.definitions) == 0 {
		return nil
	}
	byVendor := map[string][]*modelservicev1.CollectedModelEntry{}
	for _, def := range snapshot.definitions {
		if def.GetDefinition() == nil {
			continue
		}
		vendorID := strings.TrimSpace(def.GetDefinition().GetVendorId())
		if vendorID == "" {
			continue
		}
		byVendor[vendorID] = append(byVendor[vendorID], def)
	}
	return byVendor
}

// listConfiguredVendorsDefault queries Kubernetes for registered vendor support packages.
func (r *DefinitionSyncReconciler) listConfiguredVendorsDefault(ctx context.Context) ([]configuredVendor, error) {
	registry, err := vendorsupport.NewManagementService()
	if err != nil {
		return nil, err
	}
	registered, err := registry.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: list registered vendors for collection refresh: %w", err)
	}

	vendors := make([]configuredVendor, 0, len(registered))
	for _, vendor := range registered {
		if vendor == nil || vendor.GetVendor() == nil {
			continue
		}
		vendorID := strings.TrimSpace(vendor.GetVendor().GetVendorId())
		if vendorID == "" {
			r.logger.Warn("skip registered vendor support without vendor id")
			continue
		}
		vendors = append(vendors, configuredVendor{
			name:     vendorID,
			vendorID: vendorID,
			aliases:  append([]string(nil), vendor.GetVendor().GetAliases()...),
		})
	}
	slices.SortFunc(vendors, func(left, right configuredVendor) int {
		if left.vendorID == right.vendorID {
			return strings.Compare(left.name, right.name)
		}
		return strings.Compare(left.vendorID, right.vendorID)
	})
	return vendors, nil
}

// newCollectionHTTPClientDefault creates an outbound HTTP client for vendor API collection.
func (r *DefinitionSyncReconciler) newCollectionHTTPClientDefault(ctx context.Context) (*http.Client, error) {
	return outboundhttp.NewClientFactory().NewClient(ctx)
}
