package identity

import (
	"context"
	"testing"

	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveContainerImageReturnsRegisteredVariant(t *testing.T) {
	client := ctrlclientfake.NewClientBuilder().Build()

	image, err := ResolveContainerImage(context.Background(), client, "code-code", "qwen-cli", "default")
	if err != nil {
		t.Fatalf("ResolveContainerImage() error = %v", err)
	}
	if got, want := image.GetImage(), "code-code/agent-cli-qwen:0.0.0"; got != want {
		t.Fatalf("image = %q, want %q", got, want)
	}
}
