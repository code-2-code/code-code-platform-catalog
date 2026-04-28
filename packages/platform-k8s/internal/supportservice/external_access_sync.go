package supportservice

import (
	"context"
	"fmt"
	"time"

	egressservicev1 "code-code.internal/go-contract/platform/egress/v1"
	vendorsupport "code-code.internal/platform-k8s/internal/supportservice/vendors/support"
	"google.golang.org/grpc"
)

const startupExternalAccessSyncRetryInterval = 5 * time.Second

type externalAccessSetApplier interface {
	ApplyExternalAccessSet(context.Context, *egressservicev1.ApplyExternalAccessSetRequest, ...grpc.CallOption) (*egressservicev1.ApplyExternalAccessSetResponse, error)
}

func SyncStartupExternalAccessSets(ctx context.Context, client externalAccessSetApplier) error {
	return syncStartupExternalAccessSets(ctx, client, startupExternalAccessSyncRetryInterval)
}

func syncStartupExternalAccessSets(ctx context.Context, client externalAccessSetApplier, retryInterval time.Duration) error {
	if client == nil {
		return fmt.Errorf("platformk8s/supportservice: egress client is nil")
	}
	if retryInterval <= 0 {
		retryInterval = startupExternalAccessSyncRetryInterval
	}
	accessSets := vendorsupport.StartupExternalAccessSets()
	var lastErr error
	for {
		lastErr = nil
		for _, accessSet := range accessSets {
			request := &egressservicev1.ApplyExternalAccessSetRequest{AccessSet: accessSet}
			if _, err := client.ApplyExternalAccessSet(ctx, request); err != nil {
				lastErr = err
				break
			}
		}
		if lastErr == nil {
			return nil
		}
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("platformk8s/supportservice: sync startup external access sets: %w", lastErr)
		case <-timer.C:
		}
	}
}
