package cliruntime

import (
	"context"
	"fmt"
	"strings"

	domaineventv1 "code-code.internal/go-contract/platform/domain_event/v1"
	"code-code.internal/platform-k8s/internal/platform/temporalruntime"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

type TemporalImageBuildDispatcher struct {
	client    client.Client
	taskQueue string
}

func NewTemporalImageBuildDispatcher(client client.Client, taskQueue string) (*TemporalImageBuildDispatcher, error) {
	if client == nil {
		return nil, fmt.Errorf("platformk8s/cliruntime: temporal client is nil")
	}
	taskQueue = strings.TrimSpace(taskQueue)
	if taskQueue == "" {
		return nil, fmt.Errorf("platformk8s/cliruntime: temporal task queue is empty")
	}
	return &TemporalImageBuildDispatcher{client: client, taskQueue: taskQueue}, nil
}

func (d *TemporalImageBuildDispatcher) DispatchImageBuild(ctx context.Context, request ImageBuildRequest) error {
	if d == nil || d.client == nil {
		return fmt.Errorf("platformk8s/cliruntime: temporal image build dispatcher is nil")
	}
	_, err := d.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       "cli-runtime-image-build-" + temporalruntime.IDPart(request.RequestID, "build"),
		TaskQueue:                d.taskQueue,
		WorkflowIDConflictPolicy: enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
	}, CLIImageBuildWorkflowName, request)
	return err
}

func (d *TemporalImageBuildDispatcher) HandleDomainEvent(ctx context.Context, event *domaineventv1.DomainEvent) error {
	payload := event.GetCliRuntime()
	if payload.GetType() != domaineventv1.CLIRuntimeEventType_CLI_RUNTIME_EVENT_TYPE_IMAGE_BUILD_REQUESTED {
		return nil
	}
	request := imageBuildRequestFromProto(payload.GetImageBuildRequest())
	if request.RequestID == "" {
		request.RequestID = event.GetEventId()
	}
	return d.DispatchImageBuild(ctx, request)
}
