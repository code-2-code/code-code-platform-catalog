package cliruntime

import (
	"context"
	"testing"

	domaineventv1 "code-code.internal/go-contract/platform/domain_event/v1"
	"code-code.internal/platform-k8s/internal/platform/domainevents"
)

type recordingOutbox struct {
	events []*domaineventv1.DomainEvent
}

func (o *recordingOutbox) Enqueue(_ context.Context, event *domaineventv1.DomainEvent) error {
	o.events = append(o.events, event)
	return nil
}

func TestDomainEventDispatcherEnqueuesCLIImageBuildRequest(t *testing.T) {
	outbox := &recordingOutbox{}
	dispatcher, err := NewDomainEventDispatcher(outbox)
	if err != nil {
		t.Fatalf("NewDomainEventDispatcher() error = %v", err)
	}

	err = dispatcher.DispatchImageBuild(context.Background(), ImageBuildRequest{
		RequestID:       "cli-image-build:gemini-cli:0.9.0:agent-cli-gemini",
		CLIID:           "gemini-cli",
		CLIVersion:      "0.9.0",
		BuildTarget:     "agent-cli-gemini",
		ImageRepository: "code-code/agent-cli-gemini",
		ImageTag:        "cli-0.9.0",
		Image:           "code-code/agent-cli-gemini:cli-0.9.0",
	})
	if err != nil {
		t.Fatalf("DispatchImageBuild() error = %v", err)
	}
	if got, want := len(outbox.events), 1; got != want {
		t.Fatalf("events = %d, want %d", got, want)
	}
	event := outbox.events[0]
	if got, want := event.GetAggregateType(), domainevents.AggregateCLIRuntime; got != want {
		t.Fatalf("aggregateType = %q, want %q", got, want)
	}
	if got, want := event.GetCliRuntime().GetImageBuildRequest().GetImage(), "code-code/agent-cli-gemini:cli-0.9.0"; got != want {
		t.Fatalf("image = %q, want %q", got, want)
	}
}
