package observability

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	credentialv1 "code-code.internal/go-contract/credential/v1"
	providerv1 "code-code.internal/go-contract/provider/v1"
	platformv1alpha1 "code-code.internal/platform-k8s/api/v1alpha1"
	"code-code.internal/platform-k8s/internal/authservice/credentials"
	"code-code.internal/platform-k8s/internal/providerservice/providers"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	sessionStartsMetric        = "gen_ai.provider.cli.oauth.session.starts.total"
	sessionTerminalMetric      = "gen_ai.provider.cli.oauth.session.terminal.total"
	sessionInflightMetric      = "gen_ai.provider.cli.oauth.session.inflight"
	sessionDurationMetric      = "gen_ai.provider.cli.oauth.session.duration.seconds"
	credentialExpiryMetric     = "gen_ai.provider.cli.oauth.credential.expiry.timestamp.seconds"
	nextRefreshMetric          = "gen_ai.provider.cli.oauth.next.refresh.timestamp.seconds"
	lastRefreshedMetric        = "gen_ai.provider.cli.oauth.last.refreshed.timestamp.seconds"
	credentialGenerationMetric = "gen_ai.provider.cli.oauth.credential.generation"
	refreshReadyMetric         = "gen_ai.provider.cli.oauth.refresh.ready"
	refreshAttemptsMetric      = "gen_ai.provider.cli.oauth.refresh.attempts.total"

	conditionOAuthRefreshReady = "OAuthRefreshReady"
)

type Observer struct {
	client    ctrlclient.Client
	namespace string
	providers providers.Store

	sessionStarts     otelmetric.Int64Counter
	sessionTerminal   otelmetric.Int64Counter
	sessionDuration   otelmetric.Float64Histogram
	refreshAttempts   otelmetric.Int64Counter
	stateRegistration otelmetric.Registration
}

type stateObserver struct {
	client          ctrlclient.Client
	namespace       string
	providers       providers.Store
	credentialStore credentials.ResourceStore
	materialStore   credentials.CredentialMaterialStore

	sessionInflight      otelmetric.Float64ObservableGauge
	credentialExpiry     otelmetric.Float64ObservableGauge
	nextRefresh          otelmetric.Float64ObservableGauge
	lastRefreshed        otelmetric.Float64ObservableGauge
	credentialGeneration otelmetric.Float64ObservableGauge
	refreshReady         otelmetric.Float64ObservableGauge
}

type providerSurfaceBindingBinding struct {
	ProviderSurfaceBindingID string
	ProviderID               string
	CliID                    string
	CredentialID             string
}

type materialExpiryObservation struct {
	expiresAt time.Time
	ok        bool
}

var (
	registerMetricsOnce sync.Once
	registeredObserver  *Observer
	registerMetricsErr  error
)

func RegisterWithCredentialStore(
	client ctrlclient.Client,
	namespace string,
	providerRepository providers.Store,
	credentialStore credentials.ResourceStore,
	materialStore credentials.CredentialMaterialStore,
) (*Observer, error) {
	registerMetricsOnce.Do(func() {
		if client == nil {
			registerMetricsErr = fmt.Errorf("platformk8s/clidefinitions/observability: client is nil")
			return
		}
		if providerRepository == nil {
			registerMetricsErr = fmt.Errorf("platformk8s/clidefinitions/observability: provider repository is nil")
			return
		}
		if materialStore == nil {
			registerMetricsErr = fmt.Errorf("platformk8s/clidefinitions/observability: credential material store is nil")
			return
		}
		namespace = strings.TrimSpace(namespace)
		if namespace == "" {
			registerMetricsErr = fmt.Errorf("platformk8s/clidefinitions/observability: namespace is empty")
			return
		}

		meter := otel.Meter("platform-k8s/clidefinitions/observability")
		sessionStarts, err := newObserverCounter(
			meter,
			sessionStartsMetric,
			"Count of started CLI OAuth authorization sessions.",
		)
		if err != nil {
			registerMetricsErr = err
			return
		}
		sessionTerminal, err := newObserverCounter(
			meter,
			sessionTerminalMetric,
			"Count of terminal CLI OAuth authorization sessions.",
		)
		if err != nil {
			registerMetricsErr = err
			return
		}
		sessionDuration, err := meter.Float64Histogram(
			sessionDurationMetric,
			otelmetric.WithDescription("Duration of terminal CLI OAuth authorization sessions."),
			otelmetric.WithUnit("s"),
		)
		if err != nil {
			registerMetricsErr = fmt.Errorf("platformk8s/clidefinitions/observability: create histogram %q: %w", sessionDurationMetric, err)
			return
		}
		refreshAttempts, err := newObserverCounter(
			meter,
			refreshAttemptsMetric,
			"Count of CLI OAuth refresh attempts scoped to bound providers.",
		)
		if err != nil {
			registerMetricsErr = err
			return
		}
		state := &stateObserver{
			client:          client,
			namespace:       namespace,
			providers:       providerRepository,
			credentialStore: credentialStore,
			materialStore:   materialStore,
		}
		state.sessionInflight, err = newObserverObservableGauge(
			meter,
			sessionInflightMetric,
			"Current number of non-terminal CLI OAuth authorization sessions.",
		)
		if err != nil {
			registerMetricsErr = err
			return
		}
		state.credentialExpiry, err = newObserverObservableGauge(
			meter,
			credentialExpiryMetric,
			"Provider-scoped OAuth access token expiry timestamp.",
		)
		if err != nil {
			registerMetricsErr = err
			return
		}
		state.nextRefresh, err = newObserverObservableGauge(
			meter,
			nextRefreshMetric,
			"Provider-scoped OAuth next refresh timestamp.",
		)
		if err != nil {
			registerMetricsErr = err
			return
		}
		state.lastRefreshed, err = newObserverObservableGauge(
			meter,
			lastRefreshedMetric,
			"Provider-scoped OAuth last refreshed timestamp.",
		)
		if err != nil {
			registerMetricsErr = err
			return
		}
		state.credentialGeneration, err = newObserverObservableGauge(
			meter,
			credentialGenerationMetric,
			"Provider-scoped OAuth credential generation.",
		)
		if err != nil {
			registerMetricsErr = err
			return
		}
		state.refreshReady, err = newObserverObservableGauge(
			meter,
			refreshReadyMetric,
			"Whether provider OAuth refresh status is ready (1=true, 0=false).",
		)
		if err != nil {
			registerMetricsErr = err
			return
		}
		registration, err := meter.RegisterCallback(
			state.observe,
			state.sessionInflight,
			state.credentialExpiry,
			state.nextRefresh,
			state.lastRefreshed,
			state.credentialGeneration,
			state.refreshReady,
		)
		if err != nil {
			registerMetricsErr = fmt.Errorf("platformk8s/clidefinitions/observability: register state observer: %w", err)
			return
		}

		registeredObserver = &Observer{
			client:            client,
			namespace:         namespace,
			providers:         providerRepository,
			sessionStarts:     sessionStarts,
			sessionTerminal:   sessionTerminal,
			sessionDuration:   sessionDuration,
			refreshAttempts:   refreshAttempts,
			stateRegistration: registration,
		}
	})
	if registerMetricsErr != nil {
		return nil, registerMetricsErr
	}
	return registeredObserver, nil
}

func newObserverCounter(meter otelmetric.Meter, name string, description string) (otelmetric.Int64Counter, error) {
	counter, err := meter.Int64Counter(name, otelmetric.WithDescription(description), otelmetric.WithUnit("1"))
	if err != nil {
		return nil, fmt.Errorf("platformk8s/clidefinitions/observability: create counter %q: %w", name, err)
	}
	return counter, nil
}

func newObserverObservableGauge(meter otelmetric.Meter, name string, description string) (otelmetric.Float64ObservableGauge, error) {
	gauge, err := meter.Float64ObservableGauge(name, otelmetric.WithDescription(description))
	if err != nil {
		return nil, fmt.Errorf("platformk8s/clidefinitions/observability: create observable gauge %q: %w", name, err)
	}
	return gauge, nil
}

func (o *Observer) RecordSessionStarted(cliID string, flow credentialv1.OAuthAuthorizationFlow) {
	if o == nil {
		return
	}
	o.sessionStarts.Add(context.Background(), 1, otelmetric.WithAttributes(
		attribute.String("cli_id", strings.TrimSpace(cliID)),
		attribute.String("flow", normalizeFlowFromProto(flow)),
	))
}

func (o *Observer) RecordSessionTerminal(cliID string, flow platformv1alpha1.OAuthAuthorizationSessionFlow, phase platformv1alpha1.OAuthAuthorizationSessionPhase, startedAt, endedAt time.Time) {
	if o == nil {
		return
	}
	labels := []string{
		strings.TrimSpace(cliID),
		normalizeFlow(flow),
		normalizeTerminalPhase(phase),
	}
	ctx := context.Background()
	attrs := otelmetric.WithAttributes(
		attribute.String("cli_id", labels[0]),
		attribute.String("flow", labels[1]),
		attribute.String("terminal_phase", labels[2]),
	)
	o.sessionTerminal.Add(ctx, 1, attrs)
	if !startedAt.IsZero() && !endedAt.IsZero() && endedAt.After(startedAt) {
		o.sessionDuration.Record(ctx, endedAt.Sub(startedAt).Seconds(), attrs)
	}
}

func (o *Observer) RecordRefreshAttempt(cliID, credentialID, result string) {
	if o == nil {
		return
	}
	trimmedCredentialID := strings.TrimSpace(credentialID)
	trimmedResult := strings.TrimSpace(result)
	if trimmedCredentialID == "" || trimmedResult == "" {
		return
	}
	bindings, err := listCLIProviderSurfaceBindingBindings(context.Background(), o.providers)
	if err != nil {
		return
	}
	trimmedCliID := strings.TrimSpace(cliID)
	for _, binding := range bindings {
		if binding.CredentialID != trimmedCredentialID {
			continue
		}
		if trimmedCliID != "" && binding.CliID != trimmedCliID {
			continue
		}
		o.refreshAttempts.Add(context.Background(), 1, otelmetric.WithAttributes(
			attribute.String("cli_id", binding.CliID),
			attribute.String("provider_surface_binding_id", binding.ProviderSurfaceBindingID),
			attribute.String("provider_id", binding.ProviderID),
			attribute.String("result", trimmedResult),
		))
	}
}

func (c *stateObserver) observe(ctx context.Context, observer otelmetric.Observer) error {
	c.observeSessions(ctx, observer)
	c.observeCredentials(ctx, observer)
	return nil
}

func (c *stateObserver) observeSessions(ctx context.Context, observer otelmetric.Observer) {
	list := &platformv1alpha1.OAuthAuthorizationSessionResourceList{}
	if err := c.client.List(ctx, list, ctrlclient.InNamespace(c.namespace)); err != nil {
		return
	}
	counts := map[string]float64{}
	for i := range list.Items {
		item := &list.Items[i]
		if isTerminalSessionPhase(item.Status.Phase) {
			continue
		}
		key := strings.TrimSpace(item.Spec.CliID) + "|" + normalizeFlow(item.Spec.Flow)
		counts[key]++
	}
	for key, value := range counts {
		parts := strings.SplitN(key, "|", 2)
		observer.ObserveFloat64(c.sessionInflight, value, otelmetric.WithAttributes(
			attribute.String("cli_id", parts[0]),
			attribute.String("flow", parts[1]),
		))
	}
}

func (c *stateObserver) observeCredentials(ctx context.Context, observer otelmetric.Observer) {
	bindings, err := listCLIProviderSurfaceBindingBindings(ctx, c.providers)
	if err != nil || len(bindings) == 0 {
		return
	}

	items, err := c.listCredentialResources(ctx)
	if err != nil {
		return
	}
	credentials := make(map[string]*platformv1alpha1.CredentialDefinitionResource, len(items))
	for i := range items {
		resource := &items[i]
		definition := resource.Spec.Definition
		if definition == nil || definition.GetKind() != credentialv1.CredentialKind_CREDENTIAL_KIND_OAUTH {
			continue
		}
		credentialID := strings.TrimSpace(definition.GetCredentialId())
		if credentialID == "" {
			credentialID = resource.Name
		}
		credentials[credentialID] = resource
	}

	materialExpiries := map[string]materialExpiryObservation{}
	for _, binding := range bindings {
		resource := credentials[binding.CredentialID]
		if resource == nil {
			continue
		}
		labels := []string{
			binding.CliID,
			binding.ProviderSurfaceBindingID,
			binding.ProviderID,
		}
		attrs := otelmetric.WithAttributes(
			attribute.String("cli_id", labels[0]),
			attribute.String("provider_surface_binding_id", labels[1]),
			attribute.String("provider_id", labels[2]),
		)
		if expiresAt, ok := c.cachedMaterialExpiry(ctx, binding.CredentialID, materialExpiries); ok {
			observer.ObserveFloat64(c.credentialExpiry, float64(expiresAt.Unix()), attrs)
		}
		if status := resource.Status.OAuth; status != nil {
			observer.ObserveFloat64(c.credentialGeneration, float64(status.CredentialGeneration), attrs)
			if status.NextRefreshAfter != nil && !status.NextRefreshAfter.IsZero() {
				observer.ObserveFloat64(c.nextRefresh, float64(status.NextRefreshAfter.Unix()), attrs)
			}
			if status.LastRefreshedAt != nil && !status.LastRefreshedAt.IsZero() {
				observer.ObserveFloat64(c.lastRefreshed, float64(status.LastRefreshedAt.Unix()), attrs)
			}
		}
		observer.ObserveFloat64(c.refreshReady, conditionGauge(resource.Status.Conditions, conditionOAuthRefreshReady), attrs)
	}
}

func (c *stateObserver) cachedMaterialExpiry(ctx context.Context, credentialID string, cache map[string]materialExpiryObservation) (time.Time, bool) {
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return time.Time{}, false
	}
	if observation, exists := cache[credentialID]; exists {
		return observation.expiresAt, observation.ok
	}
	expiresAt, ok := c.materialExpiry(ctx, credentialID)
	cache[credentialID] = materialExpiryObservation{
		expiresAt: expiresAt,
		ok:        ok,
	}
	return expiresAt, ok
}

func (c *stateObserver) listCredentialResources(ctx context.Context) ([]platformv1alpha1.CredentialDefinitionResource, error) {
	if c.credentialStore != nil {
		return c.credentialStore.List(ctx)
	}
	list := &platformv1alpha1.CredentialDefinitionResourceList{}
	if err := c.client.List(ctx, list, ctrlclient.InNamespace(c.namespace)); err != nil {
		return nil, err
	}
	return append([]platformv1alpha1.CredentialDefinitionResource(nil), list.Items...), nil
}

func (c *stateObserver) materialExpiry(ctx context.Context, credentialID string) (time.Time, bool) {
	if c.materialStore == nil || strings.TrimSpace(credentialID) == "" {
		return time.Time{}, false
	}
	values, err := c.materialStore.ReadValues(ctx, credentialID)
	if err != nil {
		return time.Time{}, false
	}
	expiresAt, err := credentials.ExpiresAtFromMaterialValues(values)
	if err != nil {
		return time.Time{}, false
	}
	if expiresAt == nil {
		return time.Time{}, false
	}
	return expiresAt.UTC(), true
}

func listCLIProviderSurfaceBindingBindings(ctx context.Context, providerRepository providers.Store) ([]providerSurfaceBindingBinding, error) {
	projections, err := providers.ListSurfaceBindingProjections(ctx, providerRepository)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/clidefinitions/observability: list provider surface bindings: %w", err)
	}

	bindingsByID := make(map[string]providerSurfaceBindingBinding, len(projections))
	for _, projection := range projections {
		surface := projection.Surface
		if surface == nil || surface.GetRuntime() == nil {
			continue
		}
		runtime := surface.GetRuntime()
		if providerv1.RuntimeKind(runtime) != providerv1.ProviderSurfaceKind_PROVIDER_SURFACE_KIND_CLI {
			continue
		}
		providerSurfaceBindingID := strings.TrimSpace(surface.GetSurfaceId())
		providerID := strings.TrimSpace(projection.Provider.GetProviderId())
		cliID := strings.TrimSpace(providerv1.RuntimeCLIID(runtime))
		credentialID := strings.TrimSpace(surface.GetProviderCredentialRef().GetProviderCredentialId())
		if providerID == "" || providerSurfaceBindingID == "" || cliID == "" || credentialID == "" {
			continue
		}
		bindingsByID[providerID] = providerSurfaceBindingBinding{
			ProviderSurfaceBindingID: providerSurfaceBindingID,
			ProviderID:               providerID,
			CliID:                    cliID,
			CredentialID:             credentialID,
		}
	}

	bindings := make([]providerSurfaceBindingBinding, 0, len(bindingsByID))
	for _, binding := range bindingsByID {
		bindings = append(bindings, binding)
	}
	slices.SortFunc(bindings, func(left, right providerSurfaceBindingBinding) int {
		if left.ProviderSurfaceBindingID < right.ProviderSurfaceBindingID {
			return -1
		}
		if left.ProviderSurfaceBindingID > right.ProviderSurfaceBindingID {
			return 1
		}
		return 0
	})
	return bindings, nil
}

func conditionGauge(conditions []metav1.Condition, conditionType string) float64 {
	condition := meta.FindStatusCondition(conditions, conditionType)
	if condition == nil {
		return 0
	}
	if condition.Status == metav1.ConditionTrue {
		return 1
	}
	return 0
}

func normalizeFlow(flow platformv1alpha1.OAuthAuthorizationSessionFlow) string {
	switch flow {
	case platformv1alpha1.OAuthAuthorizationSessionFlowCode:
		return "code"
	case platformv1alpha1.OAuthAuthorizationSessionFlowDevice:
		return "device"
	default:
		return strings.ToLower(strings.TrimSpace(string(flow)))
	}
}

func normalizeFlowFromProto(flow credentialv1.OAuthAuthorizationFlow) string {
	switch flow {
	case credentialv1.OAuthAuthorizationFlow_O_AUTH_AUTHORIZATION_FLOW_CODE:
		return "code"
	case credentialv1.OAuthAuthorizationFlow_O_AUTH_AUTHORIZATION_FLOW_DEVICE:
		return "device"
	default:
		return strings.ToLower(strings.TrimSpace(flow.String()))
	}
}

func normalizeTerminalPhase(phase platformv1alpha1.OAuthAuthorizationSessionPhase) string {
	return strings.ToLower(strings.TrimSpace(string(phase)))
}

func isTerminalSessionPhase(phase platformv1alpha1.OAuthAuthorizationSessionPhase) bool {
	switch phase {
	case platformv1alpha1.OAuthAuthorizationSessionPhaseSucceeded,
		platformv1alpha1.OAuthAuthorizationSessionPhaseFailed,
		platformv1alpha1.OAuthAuthorizationSessionPhaseExpired,
		platformv1alpha1.OAuthAuthorizationSessionPhaseCanceled:
		return true
	default:
		return false
	}
}
