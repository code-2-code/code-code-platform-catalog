package sync

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

const collectorMetricsNamespace = "gen_ai.model_registry.collector"

// collectorMetrics records per-source OTel metrics for model collection runs.
type collectorMetrics struct {
	runs     otelmetric.Int64Counter
	duration otelmetric.Float64Histogram
	models   otelmetric.Int64Gauge
}

func registerCollectorMetrics() (*collectorMetrics, error) {
	meter := otel.Meter("platform-model-service")

	runs, err := meter.Int64Counter(
		collectorMetricsNamespace+".runs.total",
		otelmetric.WithDescription("Number of model collection runs per source"),
	)
	if err != nil {
		return nil, err
	}
	duration, err := meter.Float64Histogram(
		collectorMetricsNamespace+".duration.seconds",
		otelmetric.WithDescription("Duration of model collection per source in seconds"),
		otelmetric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}
	models, err := meter.Int64Gauge(
		collectorMetricsNamespace+".models.count",
		otelmetric.WithDescription("Number of models collected per source in the last run"),
	)
	if err != nil {
		return nil, err
	}
	return &collectorMetrics{
		runs:     runs,
		duration: duration,
		models:   models,
	}, nil
}

func (m *collectorMetrics) recordCollectorRun(sourceID string, elapsed time.Duration, modelCount int, err error) {
	if m == nil {
		return
	}
	ctx := context.Background()
	outcome := collectorOutcome(err)
	attrs := otelmetric.WithAttributes(
		attribute.String("source_id", sourceID),
		attribute.String("outcome", outcome),
	)
	m.runs.Add(ctx, 1, attrs)
	m.duration.Record(ctx, elapsed.Seconds(), attrs)
	m.models.Record(ctx, int64(modelCount), otelmetric.WithAttributes(
		attribute.String("source_id", sourceID),
	))
}

func collectorOutcome(err error) string {
	if err == nil {
		return "success"
	}
	return "failed"
}
