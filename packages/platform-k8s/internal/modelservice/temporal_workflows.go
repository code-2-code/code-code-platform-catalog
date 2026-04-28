package modelservice

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
	TemporalTaskQueue = "platform-model-service"

	ModelMaintenanceWorkflowName     = "platform.model.maintenance"
	syncModelDefinitionsActivityName = "platform.model.syncDefinitions"
)

func RegisterTemporalWorkflows(worker worker.Worker, server *Server) error {
	if worker == nil {
		return fmt.Errorf("platformk8s/modelservice: temporal worker is nil")
	}
	if server == nil {
		return fmt.Errorf("platformk8s/modelservice: server is nil")
	}
	worker.RegisterWorkflowWithOptions(ModelMaintenanceWorkflow, workflow.RegisterOptions{Name: ModelMaintenanceWorkflowName})
	activities := &TemporalActivities{server: server}
	worker.RegisterActivityWithOptions(activities.SyncModelDefinitions, activity.RegisterOptions{Name: syncModelDefinitionsActivityName})
	return nil
}

func EnsureTemporalSchedules(ctx context.Context, client client.Client, taskQueue string) error {
	return temporalruntime.EnsureIntervalSchedule(ctx, client, temporalruntime.IntervalSchedule{
		ID:         "model-maintenance",
		WorkflowID: "model-maintenance",
		Workflow:   ModelMaintenanceWorkflowName,
		TaskQueue:  taskQueue,
		Every:      5 * time.Minute,
		Offset:     2 * time.Minute,
	})
}

func ModelMaintenanceWorkflow(ctx workflow.Context) error {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 4 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumInterval: time.Minute,
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	return workflow.ExecuteActivity(ctx, syncModelDefinitionsActivityName).Get(ctx, nil)
}

type TemporalActivities struct {
	server *Server
}

func (a *TemporalActivities) SyncModelDefinitions(ctx context.Context) error {
	if a == nil || a.server == nil {
		return fmt.Errorf("platformk8s/modelservice: temporal activity server is nil")
	}
	return a.server.runModelDefinitionSync(ctx)
}
