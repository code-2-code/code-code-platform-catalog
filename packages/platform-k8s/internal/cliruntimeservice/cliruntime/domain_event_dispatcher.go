package cliruntime

import (
	"context"
	"fmt"

	domaineventv1 "code-code.internal/go-contract/platform/domain_event/v1"
	"code-code.internal/platform-k8s/internal/platform/domainevents"
)

type DomainEventOutbox interface {
	Enqueue(context.Context, *domaineventv1.DomainEvent) error
}

type DomainEventDispatcher struct {
	outbox DomainEventOutbox
}

func NewDomainEventDispatcher(outbox DomainEventOutbox) (*DomainEventDispatcher, error) {
	if outbox == nil {
		return nil, fmt.Errorf("platformk8s/cliruntime: domain event outbox is nil")
	}
	return &DomainEventDispatcher{outbox: outbox}, nil
}

func (d *DomainEventDispatcher) DispatchImageBuild(ctx context.Context, request ImageBuildRequest) error {
	if d == nil || d.outbox == nil {
		return fmt.Errorf("platformk8s/cliruntime: domain event dispatcher is nil")
	}
	return d.outbox.Enqueue(ctx, &domaineventv1.DomainEvent{
		EventId:       request.RequestID,
		EventType:     "image_build_requested",
		AggregateType: domainevents.AggregateCLIRuntime,
		AggregateId:   request.CLIID,
		Payload: &domaineventv1.DomainEvent_CliRuntime{CliRuntime: &domaineventv1.CLIRuntimeEvent{
			Type: domaineventv1.CLIRuntimeEventType_CLI_RUNTIME_EVENT_TYPE_IMAGE_BUILD_REQUESTED,
			ImageBuildRequest: &domaineventv1.CLIImageBuildRequest{
				RequestId:          request.RequestID,
				CliId:              request.CLIID,
				CliVersion:         request.CLIVersion,
				PreviousCliVersion: request.PreviousCLIVersion,
				BuildTarget:        request.BuildTarget,
				ImageRepository:    request.ImageRepository,
				ImageTag:           request.ImageTag,
				Image:              request.Image,
				SourceContext:      request.SourceContext,
				SourceRevision:     request.SourceRevision,
			},
		}},
	})
}
