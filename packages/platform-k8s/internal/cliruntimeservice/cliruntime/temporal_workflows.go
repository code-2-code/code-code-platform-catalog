package cliruntime

import (
	"context"
	"fmt"
	"time"

	"code-code.internal/platform-k8s/internal/platform/temporalruntime"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	TemporalTaskQueue = "platform-support-service"

	CLIVersionSyncWorkflowName   = "platform.cliRuntime.versionSync"
	CLIImageBuildWorkflowName    = "platform.cliRuntime.imageBuild"
	syncCLIVersionsActivityName  = "platform.cliRuntime.syncVersions"
	runCLIImageBuildActivityName = "platform.cliRuntime.runImageBuild"
)

func RegisterTemporalWorkflows(worker worker.Worker, service *Service, imageBuilds *ImageBuildJobRunner) error {
	if worker == nil {
		return fmt.Errorf("platformk8s/cliruntime: temporal worker is nil")
	}
	if service == nil {
		return fmt.Errorf("platformk8s/cliruntime: service is nil")
	}
	worker.RegisterWorkflowWithOptions(CLIVersionSyncWorkflow, workflow.RegisterOptions{Name: CLIVersionSyncWorkflowName})
	worker.RegisterWorkflowWithOptions(CLIImageBuildWorkflow, workflow.RegisterOptions{Name: CLIImageBuildWorkflowName})
	activities := &TemporalActivities{service: service}
	worker.RegisterActivityWithOptions(activities.SyncCLIVersions, activity.RegisterOptions{Name: syncCLIVersionsActivityName})
	if imageBuilds != nil {
		worker.RegisterActivityWithOptions(imageBuilds.RunImageBuild, activity.RegisterOptions{Name: runCLIImageBuildActivityName})
	}
	return nil
}

func EnsureTemporalSchedules(ctx context.Context, client client.Client, taskQueue string) error {
	return temporalruntime.EnsureIntervalSchedule(ctx, client, temporalruntime.IntervalSchedule{
		ID:         "cli-runtime-version-sync",
		WorkflowID: "cli-runtime-version-sync",
		Workflow:   CLIVersionSyncWorkflowName,
		TaskQueue:  taskQueue,
		Every:      time.Hour,
	})
}

func CLIVersionSyncWorkflow(ctx workflow.Context) error {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumInterval: time.Minute,
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	return workflow.ExecuteActivity(ctx, syncCLIVersionsActivityName).Get(ctx, nil)
}

func CLIImageBuildWorkflow(ctx workflow.Context, request ImageBuildRequest) error {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 70 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute,
			MaximumInterval: 5 * time.Minute,
			MaximumAttempts: 12,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	return workflow.ExecuteActivity(ctx, runCLIImageBuildActivityName, request).Get(ctx, nil)
}

type TemporalActivities struct {
	service *Service
}

func (a *TemporalActivities) SyncCLIVersions(ctx context.Context) error {
	if a == nil || a.service == nil {
		return fmt.Errorf("platformk8s/cliruntime: temporal activity service is nil")
	}
	_, err := a.service.SyncCLIVersions(ctx)
	return err
}
