package modelservice

import (
	"context"
	"fmt"
)

func (s *Server) runModelDefinitionSync(ctx context.Context) error {
	if s == nil || s.syncer == nil {
		return fmt.Errorf("platformk8s/modelservice: model definition syncer is not initialized")
	}
	return s.syncer.SyncNow(ctx)
}
