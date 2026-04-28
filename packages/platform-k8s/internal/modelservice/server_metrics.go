package modelservice

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"code-code.internal/go-contract/domainerror"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

const (
	registryQueryRunsMetricName      = "gen_ai.model_registry.query.runs.total"
	registryQueryDurationMetricName  = "gen_ai.model_registry.query.duration.seconds"
	definitionSyncRunsMetricName     = "gen_ai.model_registry.sync.runs.total"
	definitionSyncDurationMetricName = "gen_ai.model_registry.sync.duration.seconds"
)

type serverMetrics struct {
	registryQueryRuns      otelmetric.Int64Counter
	registryQueryDuration  otelmetric.Float64Histogram
	definitionSyncRuns     otelmetric.Int64Counter
	definitionSyncDuration otelmetric.Float64Histogram
}

var (
	registerServerMetricsOnce sync.Once
	registeredServerMetrics   *serverMetrics
	registerServerMetricsErr  error
)

func registerServerMetrics() (*serverMetrics, error) {
	registerServerMetricsOnce.Do(func() {
		meter := otel.Meter("platform-k8s/modelservice")
		queryRuns, err := meter.Int64Counter(
			registryQueryRunsMetricName,
			otelmetric.WithDescription("Count of model registry query RPC calls."),
		)
		if err != nil {
			registerServerMetricsErr = err
			return
		}
		queryDuration, err := meter.Float64Histogram(
			registryQueryDurationMetricName,
			otelmetric.WithUnit("s"),
			otelmetric.WithDescription("Duration of model registry query RPC calls."),
		)
		if err != nil {
			registerServerMetricsErr = err
			return
		}
		syncRuns, err := meter.Int64Counter(
			definitionSyncRunsMetricName,
			otelmetric.WithDescription("Count of model definition sync submissions."),
		)
		if err != nil {
			registerServerMetricsErr = err
			return
		}
		syncDuration, err := meter.Float64Histogram(
			definitionSyncDurationMetricName,
			otelmetric.WithUnit("s"),
			otelmetric.WithDescription("Duration of model definition sync submissions."),
		)
		if err != nil {
			registerServerMetricsErr = err
			return
		}
		registeredServerMetrics = &serverMetrics{
			registryQueryRuns:      queryRuns,
			registryQueryDuration:  queryDuration,
			definitionSyncRuns:     syncRuns,
			definitionSyncDuration: syncDuration,
		}
	})
	if registerServerMetricsErr != nil {
		return nil, registerServerMetricsErr
	}
	return registeredServerMetrics, nil
}

func (m *serverMetrics) recordRegistryQuery(method string, started time.Time, err error) {
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{attribute.String("method", strings.TrimSpace(method))}
	m.recordOp(m.registryQueryRuns, m.registryQueryDuration, started, err, attrs)
}

func (m *serverMetrics) recordDefinitionSync(started time.Time, err error) {
	if m == nil {
		return
	}
	m.recordOp(m.definitionSyncRuns, m.definitionSyncDuration, started, err, nil)
}

func (m *serverMetrics) recordOp(counter otelmetric.Int64Counter, histogram otelmetric.Float64Histogram, started time.Time, err error, extra []attribute.KeyValue) {
	attrs := append(extra, attribute.String("outcome", metricsOutcome(err)))
	if kind := metricsErrorKind(err); kind != "" {
		attrs = append(attrs, attribute.String("error_kind", kind))
	}
	options := otelmetric.WithAttributes(attrs...)
	ctx := context.Background()
	counter.Add(ctx, 1, options)
	histogram.Record(ctx, time.Since(started).Seconds(), options)
}


func metricsOutcome(err error) string {
	if err == nil {
		return "success"
	}
	return "failed"
}

func metricsErrorKind(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	}
	var validationErr *domainerror.ValidationError
	if errors.As(err, &validationErr) {
		return "validation"
	}
	var notFoundErr *domainerror.NotFoundError
	if errors.As(err, &notFoundErr) {
		return "not_found"
	}
	return "error"
}
