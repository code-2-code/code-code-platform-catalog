package modelservice

import "code-code.internal/platform-k8s/internal/platform/grpcerrors"

func grpcError(err error) error {
	return grpcerrors.MapError(err)
}
