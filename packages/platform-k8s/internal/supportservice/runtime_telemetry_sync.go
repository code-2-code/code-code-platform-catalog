package supportservice

import (
	"context"
	"fmt"
	"time"

	egressservicev1 "code-code.internal/go-contract/platform/egress/v1"
	"google.golang.org/grpc"
)

const startupRuntimeTelemetrySyncRetryInterval = 5 * time.Second

type runtimeTelemetryProfileSetApplier interface {
	ApplyRuntimeTelemetryProfileSet(context.Context, *egressservicev1.ApplyRuntimeTelemetryProfileSetRequest, ...grpc.CallOption) (*egressservicev1.ApplyRuntimeTelemetryProfileSetResponse, error)
}

func SyncStartupRuntimeTelemetryProfiles(ctx context.Context, server *Server, client runtimeTelemetryProfileSetApplier) error {
	return syncStartupRuntimeTelemetryProfiles(ctx, server, client, startupRuntimeTelemetrySyncRetryInterval)
}

func syncStartupRuntimeTelemetryProfiles(ctx context.Context, server *Server, client runtimeTelemetryProfileSetApplier, retryInterval time.Duration) error {
	if server == nil {
		return fmt.Errorf("platformk8s/supportservice: support server is nil")
	}
	if client == nil {
		return fmt.Errorf("platformk8s/supportservice: egress client is nil")
	}
	if retryInterval <= 0 {
		retryInterval = startupRuntimeTelemetrySyncRetryInterval
	}
	capability := server.RuntimeTelemetryProfiles()
	if capability == nil || len(capability.GetProfiles()) == 0 {
		return nil
	}
	request := &egressservicev1.ApplyRuntimeTelemetryProfileSetRequest{
		ProfileSetId: "support.runtime-http-telemetry",
		Capability:   capability,
	}
	var lastErr error
	for {
		_, lastErr = client.ApplyRuntimeTelemetryProfileSet(ctx, request)
		if lastErr == nil {
			return nil
		}
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("platformk8s/supportservice: sync startup runtime telemetry profiles: %w", lastErr)
		case <-timer.C:
		}
	}
}
