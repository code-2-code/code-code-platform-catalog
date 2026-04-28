package supportservice

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	egressservicev1 "code-code.internal/go-contract/platform/egress/v1"
	managementv1 "code-code.internal/go-contract/platform/management/v1"
	"google.golang.org/grpc"
)

func TestSyncStartupExternalAccessSetsRetriesUntilSuccess(t *testing.T) {
	client := &recordingEgressClient{failuresRemaining: 2}

	if err := syncStartupExternalAccessSets(context.Background(), client, time.Millisecond); err != nil {
		t.Fatalf("syncStartupExternalAccessSets() error = %v", err)
	}
	if got, want := client.calls(), 4; got != want {
		t.Fatalf("calls = %d, want %d", got, want)
	}
	if got, want := client.accessSetIDs(), []string{
		"support.external-rule-set.bootstrap",
		"support.external-rule-set.bootstrap",
		"support.external-rule-set.bootstrap",
		"support.proxy-preset.preset-proxy",
	}; !equalStrings(got, want) {
		t.Fatalf("access set ids = %v, want %v", got, want)
	}
}

func TestSyncStartupExternalAccessSetsStopsOnContextCancel(t *testing.T) {
	client := &recordingEgressClient{alwaysFail: true}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Millisecond)
	defer cancel()

	err := syncStartupExternalAccessSets(ctx, client, time.Millisecond)
	if err == nil {
		t.Fatal("syncStartupExternalAccessSets() error is nil, want cancel error")
	}
	if got := client.calls(); got == 0 {
		t.Fatal("calls = 0, want at least one retry attempt")
	}
}

type recordingEgressClient struct {
	mu                sync.Mutex
	callCount         int
	failuresRemaining int
	alwaysFail        bool
	last              *egressservicev1.ApplyExternalAccessSetRequest
	ids               []string
}

func (c *recordingEgressClient) ApplyExternalAccessSet(_ context.Context, request *egressservicev1.ApplyExternalAccessSetRequest, _ ...grpc.CallOption) (*egressservicev1.ApplyExternalAccessSetResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callCount++
	c.last = request
	c.ids = append(c.ids, request.GetAccessSet().GetAccessSetId())
	if c.alwaysFail {
		return nil, fmt.Errorf("egress unavailable")
	}
	if c.failuresRemaining > 0 {
		c.failuresRemaining--
		return nil, fmt.Errorf("egress unavailable")
	}
	return &egressservicev1.ApplyExternalAccessSetResponse{}, nil
}

func (c *recordingEgressClient) ListEgressPolicies(context.Context, *managementv1.ListEgressPoliciesRequest, ...grpc.CallOption) (*managementv1.ListEgressPoliciesResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *recordingEgressClient) UpdateEgressPolicy(context.Context, *managementv1.UpdateEgressPolicyRequest, ...grpc.CallOption) (*managementv1.UpdateEgressPolicyResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *recordingEgressClient) DeleteExternalAccessSet(context.Context, *egressservicev1.DeleteExternalAccessSetRequest, ...grpc.CallOption) (*egressservicev1.DeleteExternalAccessSetResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *recordingEgressClient) GetEgressRuntimePolicy(context.Context, *egressservicev1.GetEgressRuntimePolicyRequest, ...grpc.CallOption) (*egressservicev1.GetEgressRuntimePolicyResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *recordingEgressClient) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.callCount
}

func (c *recordingEgressClient) lastAccessSetID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last.GetAccessSet().GetAccessSetId()
}

func (c *recordingEgressClient) accessSetIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.ids...)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
