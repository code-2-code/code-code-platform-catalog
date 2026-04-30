package cliruntime

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

const imageBuildMetricsNamespace = "gen_ai.cli_runtime.image_build"

type imageBuildMetrics struct {
	runs     otelmetric.Int64Counter
	duration otelmetric.Float64Histogram
}

func registerImageBuildMetrics() (*imageBuildMetrics, error) {
	meter := otel.Meter("platform-support-service")

	runs, err := meter.Int64Counter(
		imageBuildMetricsNamespace+".runs.total",
		otelmetric.WithDescription("Number of image build runs"),
	)
	if err != nil {
		return nil, err
	}
	duration, err := meter.Float64Histogram(
		imageBuildMetricsNamespace+".duration.seconds",
		otelmetric.WithDescription("Duration of image builds in seconds"),
		otelmetric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}
	return &imageBuildMetrics{
		runs:     runs,
		duration: duration,
	}, nil
}

func (m *imageBuildMetrics) recordImageBuildRun(buildTarget string, elapsed time.Duration, err error) {
	if m == nil {
		return
	}
	ctx := context.Background()
	outcome := "success"
	if err != nil {
		outcome = "failed"
	}
	attrs := otelmetric.WithAttributes(
		attribute.String("build_target", buildTarget),
		attribute.String("outcome", outcome),
	)
	m.runs.Add(ctx, 1, attrs)
	m.duration.Record(ctx, elapsed.Seconds(), attrs)
}
