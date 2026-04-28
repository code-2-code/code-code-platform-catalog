package modelservice

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

const (
	manualModelMaintenanceWorkflowID = "model-maintenance-manual"
	syncSubmissionAccepted           = "accepted"
	syncSubmissionAlreadyRunning     = "already_running"
)

type DefinitionSyncRuntime interface {
	SubmitSync(context.Context) (string, error)
}

type temporalWorkflowStarter interface {
	ExecuteWorkflow(context.Context, client.StartWorkflowOptions, any, ...any) (client.WorkflowRun, error)
}

type TemporalDefinitionSyncRuntime struct {
	starter   temporalWorkflowStarter
	taskQueue string
}

func NewTemporalDefinitionSyncRuntime(temporalClient client.Client, taskQueue string) (*TemporalDefinitionSyncRuntime, error) {
	if temporalClient == nil {
		return nil, fmt.Errorf("platformk8s/modelservice: temporal definition sync runtime client is nil")
	}
	taskQueue = strings.TrimSpace(taskQueue)
	if taskQueue == "" {
		taskQueue = TemporalTaskQueue
	}
	return &TemporalDefinitionSyncRuntime{
		starter:   temporalClient,
		taskQueue: taskQueue,
	}, nil
}

func (r *TemporalDefinitionSyncRuntime) SubmitSync(ctx context.Context) (string, error) {
	if r == nil || r.starter == nil {
		return "", fmt.Errorf("platformk8s/modelservice: temporal definition sync runtime is nil")
	}
	_, err := r.starter.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        manualModelMaintenanceWorkflowID,
		TaskQueue: r.taskQueue,
	}, ModelMaintenanceWorkflowName)
	if err == nil {
		return syncSubmissionAccepted, nil
	}
	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return syncSubmissionAlreadyRunning, nil
	}
	return "", fmt.Errorf("platformk8s/modelservice: submit definition sync workflow: %w", err)
}
