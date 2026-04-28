package clis

import (
	"context"
	"fmt"
	"strings"

	supportv1 "code-code.internal/go-contract/platform/support/v1"
	"code-code.internal/platform-k8s/internal/cliruntimeservice/cliversions"
	"code-code.internal/platform-k8s/internal/modelservice/modelcatalogsources"
	clioauth "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/oauth"
	clisupport "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/support"
	"google.golang.org/protobuf/proto"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type CLISupportReader interface {
	List(context.Context) ([]*supportv1.CLI, error)
}

type RegisterConfig struct {
	Support      CLISupportReader
	Probe        modelcatalogsources.ModelIDProbe
	Reader       ctrlclient.Reader
	VersionStore cliversions.Store
	Namespace    string
}

func Register(ctx context.Context, registry *modelcatalogsources.Registry, config RegisterConfig) error {
	if registry == nil {
		return fmt.Errorf("platformk8s/modelcatalogsources/clis: registry is nil")
	}
	if config.Support == nil {
		return fmt.Errorf("platformk8s/modelcatalogsources/clis: cli support reader is nil")
	}
	clis, err := config.Support.List(ctx)
	if err != nil {
		return err
	}
	for _, cli := range clis {
		cliID := strings.TrimSpace(cli.GetCliId())
		if cliID == "" {
			return fmt.Errorf("platformk8s/modelcatalogsources/clis: cli support id is empty")
		}
		if hasDefaultCatalog(cli) {
			if err := registry.Register(&cliSource{
				ref: modelcatalogsources.ProbeRef("cli." + cliID),
			}); err != nil {
				return err
			}
		}
		if _, operation, err := clioauth.ResolveOAuthModelCatalogDiscovery(cli); err != nil {
			return err
		} else if operation != nil && config.Probe != nil {
			probeID := strings.TrimSpace(clisupport.OAuthModelCatalogProbeID(cli))
			if probeID == "" {
				continue
			}
			if err := registry.Register(&cliSource{
				ref: modelcatalogsources.ProbeRef(probeID),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasDefaultCatalog(cli *supportv1.CLI) bool {
	if cli == nil || cli.GetOauth() == nil || cli.GetOauth().GetModelCatalog() == nil {
		return false
	}
	return cli.GetOauth().GetModelCatalog().GetDefaultCatalog() != nil
}

type cliSource struct {
	ref modelcatalogsources.CapabilityRef
}

func (s *cliSource) CapabilityRef() modelcatalogsources.CapabilityRef {
	return s.ref
}

// cloneCLI creates a deep copy of a CLI support definition.
func cloneCLI(cli *supportv1.CLI) *supportv1.CLI {
	if cli == nil {
		return nil
	}
	return proto.Clone(cli).(*supportv1.CLI)
}
