package modelservice

import (
	"context"
	"errors"
	"testing"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

func TestTemporalDefinitionSyncRuntimeSubmitSyncAccepted(t *testing.T) {
	t.Parallel()

	starter := &workflowStarterStub{}
	runtime := &TemporalDefinitionSyncRuntime{
		starter:   starter,
		taskQueue: "platform-model-service",
	}
	status, err := runtime.SubmitSync(context.Background())
	if err != nil {
		t.Fatalf("SubmitSync() error = %v", err)
	}
	if got, want := status, "accepted"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if got, want := starter.workflowName, ModelMaintenanceWorkflowName; got != want {
		t.Fatalf("workflow name = %q, want %q", got, want)
	}
	if got, want := starter.options.ID, manualModelMaintenanceWorkflowID; got != want {
		t.Fatalf("workflow id = %q, want %q", got, want)
	}
}

func TestTemporalDefinitionSyncRuntimeSubmitSyncAlreadyRunning(t *testing.T) {
	t.Parallel()

	runtime := &TemporalDefinitionSyncRuntime{
		starter: &workflowStarterStub{
			err: serviceerror.NewWorkflowExecutionAlreadyStarted("started", "req-1", "run-1"),
		},
		taskQueue: "platform-model-service",
	}
	status, err := runtime.SubmitSync(context.Background())
	if err != nil {
		t.Fatalf("SubmitSync() error = %v", err)
	}
	if got, want := status, "already_running"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestTemporalDefinitionSyncRuntimeSubmitSyncError(t *testing.T) {
	t.Parallel()

	runtime := &TemporalDefinitionSyncRuntime{
		starter: &workflowStarterStub{err: errors.New("dial timeout")},
	}
	_, err := runtime.SubmitSync(context.Background())
	if err == nil {
		t.Fatal("SubmitSync() error = nil, want non-nil")
	}
}

type workflowStarterStub struct {
	options      client.StartWorkflowOptions
	workflowName string
	err          error
}

func (s *workflowStarterStub) ExecuteWorkflow(_ context.Context, options client.StartWorkflowOptions, workflow any, _ ...any) (client.WorkflowRun, error) {
	s.options = options
	if name, ok := workflow.(string); ok {
		s.workflowName = name
	}
	if s.err != nil {
		return nil, s.err
	}
	return workflowRunStub{id: options.ID, runID: "run-123"}, nil
}

type workflowRunStub struct {
	id    string
	runID string
}

func (w workflowRunStub) GetID() string {
	return w.id
}

func (w workflowRunStub) GetRunID() string {
	return w.runID
}

func (w workflowRunStub) Get(context.Context, any) error {
	return nil
}

func (w workflowRunStub) GetWithOptions(context.Context, any, client.WorkflowRunGetOptions) error {
	return nil
}
