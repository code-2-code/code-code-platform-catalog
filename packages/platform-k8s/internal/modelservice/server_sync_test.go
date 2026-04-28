package modelservice

import (
	"context"
	"testing"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSyncModelDefinitionsRequiresSyncer(t *testing.T) {
	t.Parallel()

	server := &Server{}
	_, err := server.SyncModelDefinitions(context.Background(), &modelservicev1.SyncModelDefinitionsRequest{})
	if err == nil {
		t.Fatal("SyncModelDefinitions() error = nil, want non-nil")
	}
	if got, want := status.Code(err), codes.Internal; got != want {
		t.Fatalf("status code = %s, want %s", got, want)
	}
}
