package sync

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
)

func TestDefinitionSourceEndpointUnavailableDetectsTransportFailures(t *testing.T) {
	t.Parallel()

	cases := []error{
		context.DeadlineExceeded,
		context.Canceled,
		errors.New("Get \"https://example.com\": context deadline exceeded"),
		errors.New("Get \"https://example.com\": read tcp 10.0.0.1:1234->1.1.1.1:443: read: connection reset by peer"),
		errors.New("Get \"https://example.com\": EOF"),
		&url.Error{Err: context.DeadlineExceeded},
	}
	for _, err := range cases {
		if !definitionSourceEndpointUnavailable(err) {
			t.Fatalf("definitionSourceEndpointUnavailable(%v) = false, want true", err)
		}
	}
}

func TestDefinitionSourceEndpointUnavailableIgnoresNonTransportErrors(t *testing.T) {
	t.Parallel()

	cases := []error{
		errors.New("platformk8s/models: decode openrouter models: invalid character 'x'"),
		errors.New("platformk8s/models: huggingface models status 429"),
		fmt.Errorf("wrapped: %w", errors.New("platformk8s/models: malformed catalog payload")),
	}
	for _, err := range cases {
		if definitionSourceEndpointUnavailable(err) {
			t.Fatalf("definitionSourceEndpointUnavailable(%v) = true, want false", err)
		}
	}
}
