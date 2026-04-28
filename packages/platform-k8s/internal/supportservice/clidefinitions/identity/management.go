package identity

import (
	"context"
	"slices"
	"strings"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	clidefinitionv1 "code-code.internal/go-contract/cli_definition/v1"
	managementv1 "code-code.internal/go-contract/platform/management/v1"
	supportv1 "code-code.internal/go-contract/platform/support/v1"
	clisupport "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/support"
)

var registeredCLIContainerImages = map[string][]*clidefinitionv1.CLIContainerImage{
	"claude-code": {{
		ExecutionClass: "default",
		Image:          "code-code/claude-code-agent:0.0.0",
		CpuRequest:     "500m",
		MemoryRequest:  "1Gi",
	}},
	"gemini-cli": {{
		ExecutionClass: "default",
		Image:          "code-code/agent-cli-gemini:0.0.0",
		CpuRequest:     "500m",
		MemoryRequest:  "1Gi",
	}},
	"qwen-cli": {{
		ExecutionClass: "default",
		Image:          "code-code/agent-cli-qwen:0.0.0",
		CpuRequest:     "500m",
		MemoryRequest:  "1Gi",
	}},
}

// CLIDefinitionManagementService provides read-only access to CLI definitions.
type CLIDefinitionManagementService struct {
}

// NewCLIDefinitionManagementService creates one CLI definition management
// service.
func NewCLIDefinitionManagementService() (*CLIDefinitionManagementService, error) {
	return &CLIDefinitionManagementService{}, nil
}

// List returns all CLI definitions in UI-facing form.
func (s *CLIDefinitionManagementService) List(ctx context.Context) ([]*managementv1.CLIDefinitionView, error) {
	_ = ctx
	definitions, err := registeredCLIDefinitions()
	if err != nil {
		return nil, err
	}

	items := make([]*managementv1.CLIDefinitionView, 0, len(definitions))
	for _, def := range definitions {
		items = append(items, cliDefinitionToView(def))
	}
	slices.SortFunc(items, func(a, b *managementv1.CLIDefinitionView) int {
		if a.GetDisplayName() < b.GetDisplayName() {
			return -1
		}
		if a.GetDisplayName() > b.GetDisplayName() {
			return 1
		}
		return 0
	})
	return items, nil
}

func cliDefinitionToView(def *clidefinitionv1.CLIDefinition) *managementv1.CLIDefinitionView {
	view := &managementv1.CLIDefinitionView{
		CliId:       def.CliId,
		DisplayName: def.DisplayName,
		IconUrl:     def.IconUrl,
		WebsiteUrl:  def.WebsiteUrl,
		Description: def.Description,
	}
	view.ContainerImages = make([]*managementv1.CLIContainerImageView, 0, len(def.ContainerImages))
	for _, image := range def.ContainerImages {
		if image == nil {
			continue
		}
		view.ContainerImages = append(view.ContainerImages, &managementv1.CLIContainerImageView{
			ExecutionClass: image.ExecutionClass,
			Image:          image.Image,
			CpuRequest:     image.CpuRequest,
			MemoryRequest:  image.MemoryRequest,
		})
	}
	if def.Capabilities != nil {
		supportedProtocols := make([]string, 0, len(def.Capabilities.SupportedProtocols))
		for _, protocol := range def.Capabilities.SupportedProtocols {
			supportedProtocols = append(supportedProtocols, protocol.String())
		}
		view.Capabilities = &managementv1.CLIDefinitionCapabilityView{
			SupportsStreaming:    def.Capabilities.SupportsStreaming,
			SupportsApprovalMode: def.Capabilities.SupportsApprovalMode,
			SupportedProtocols:   supportedProtocols,
		}
	}
	return view
}

func registeredCLIDefinitions() ([]*clidefinitionv1.CLIDefinition, error) {
	clis, err := clisupport.RegisteredCLIs()
	if err != nil {
		return nil, err
	}
	items := make([]*clidefinitionv1.CLIDefinition, 0, len(clis))
	for _, cli := range clis {
		items = append(items, cliDefinitionFromSupport(cli))
	}
	return items, nil
}

func registeredCLIDefinition(cliID string) (*clidefinitionv1.CLIDefinition, error) {
	cli, err := clisupport.RegisteredCLI(cliID)
	if err != nil {
		return nil, err
	}
	return cliDefinitionFromSupport(cli), nil
}

// RegisteredContainerImages returns the runnable images registered for one CLI.
func RegisteredContainerImages(cliID string) []*clidefinitionv1.CLIContainerImage {
	return registeredContainerImages(cliID)
}

func cliDefinitionFromSupport(cli *supportv1.CLI) *clidefinitionv1.CLIDefinition {
	if cli == nil {
		return &clidefinitionv1.CLIDefinition{}
	}
	return &clidefinitionv1.CLIDefinition{
		CliId:           strings.TrimSpace(cli.GetCliId()),
		DisplayName:     strings.TrimSpace(cli.GetDisplayName()),
		IconUrl:         strings.TrimSpace(cli.GetIconUrl()),
		WebsiteUrl:      strings.TrimSpace(cli.GetWebsiteUrl()),
		Description:     strings.TrimSpace(cli.GetDescription()),
		ContainerImages: registeredContainerImages(cli.GetCliId()),
		Capabilities: &clidefinitionv1.CLICapabilities{
			SupportsStreaming:    true,
			SupportsApprovalMode: true,
			SupportedProtocols:   supportedProtocols(cli),
		},
	}
}

func registeredContainerImages(cliID string) []*clidefinitionv1.CLIContainerImage {
	images := registeredCLIContainerImages[strings.TrimSpace(cliID)]
	out := make([]*clidefinitionv1.CLIContainerImage, 0, len(images))
	for _, image := range images {
		if image == nil {
			continue
		}
		out = append(out, &clidefinitionv1.CLIContainerImage{
			ExecutionClass: strings.TrimSpace(image.GetExecutionClass()),
			Image:          strings.TrimSpace(image.GetImage()),
			CpuRequest:     strings.TrimSpace(image.GetCpuRequest()),
			MemoryRequest:  strings.TrimSpace(image.GetMemoryRequest()),
		})
	}
	return out
}

func supportedProtocols(cli *supportv1.CLI) []apiprotocolv1.Protocol {
	seen := map[apiprotocolv1.Protocol]struct{}{}
	out := []apiprotocolv1.Protocol{}
	for _, protocol := range cli.GetApiKeyProtocols() {
		value := protocol.GetProtocol()
		if value == apiprotocolv1.Protocol_PROTOCOL_UNSPECIFIED {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
