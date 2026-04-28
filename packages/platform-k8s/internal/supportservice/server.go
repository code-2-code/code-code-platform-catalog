package supportservice

import (
	"context"
	"fmt"
	"slices"
	"strings"

	apiprotocolv1 "code-code.internal/go-contract/api_protocol/v1"
	clidefinitionv1 "code-code.internal/go-contract/cli_definition/v1"
	observabilityv1 "code-code.internal/go-contract/observability/v1"
	supportv1 "code-code.internal/go-contract/platform/support/v1"
	cliidentity "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/identity"
	clisupport "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/support"
	"code-code.internal/platform-k8s/internal/supportservice/providersurfaces"
	vendorsupport "code-code.internal/platform-k8s/internal/supportservice/vendors/support"
	"google.golang.org/protobuf/proto"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	Reader    ctrlclient.Reader
	Namespace string
}

type Server struct {
	supportv1.UnimplementedSupportServiceServer

	vendors          *vendorsupport.ManagementService
	clis             *clisupport.ManagementService
	surfaces         *providersurfaces.Service
	runtimeTelemetry *observabilityv1.ObservabilityCapability
}

func NewServer(config Config) (*Server, error) {
	if config.Reader == nil {
		return nil, fmt.Errorf("platformk8s/supportservice: reader is nil")
	}
	if strings.TrimSpace(config.Namespace) == "" {
		return nil, fmt.Errorf("platformk8s/supportservice: namespace is empty")
	}
	vendors, err := vendorsupport.NewManagementService()
	if err != nil {
		return nil, err
	}
	clis, err := clisupport.NewManagementService()
	if err != nil {
		return nil, err
	}
	surfaces, err := providersurfaces.NewService(clis, vendors)
	if err != nil {
		return nil, err
	}
	runtimeTelemetry, err := loadRuntimeTelemetryProfiles()
	if err != nil {
		return nil, err
	}
	return &Server{
		vendors:          vendors,
		clis:             clis,
		surfaces:         surfaces,
		runtimeTelemetry: runtimeTelemetry,
	}, nil
}

func (s *Server) ListVendors(ctx context.Context, _ *supportv1.ListVendorsRequest) (*supportv1.ListVendorsResponse, error) {
	items, err := s.vendors.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*supportv1.Vendor, 0, len(items))
	for _, item := range items {
		out = append(out, sanitizeVendor(item))
	}
	return &supportv1.ListVendorsResponse{Items: out}, nil
}

func (s *Server) GetVendor(ctx context.Context, request *supportv1.GetVendorRequest) (*supportv1.GetVendorResponse, error) {
	item, err := s.vendors.Get(ctx, request.GetVendorId())
	if err != nil {
		return nil, err
	}
	return &supportv1.GetVendorResponse{Item: sanitizeVendor(item)}, nil
}

func (s *Server) ListProviderSurfaces(ctx context.Context, _ *supportv1.ListProviderSurfacesRequest) (*supportv1.ListProviderSurfacesResponse, error) {
	items, err := s.surfaces.List(ctx)
	if err != nil {
		return nil, err
	}
	return &supportv1.ListProviderSurfacesResponse{Items: items}, nil
}

func (s *Server) GetProviderSurface(ctx context.Context, request *supportv1.GetProviderSurfaceRequest) (*supportv1.GetProviderSurfaceResponse, error) {
	item, err := s.surfaces.Get(ctx, request.GetSurfaceId())
	if err != nil {
		return nil, err
	}
	return &supportv1.GetProviderSurfaceResponse{Item: item}, nil
}

func (s *Server) ListCLIs(ctx context.Context, _ *supportv1.ListCLIsRequest) (*supportv1.ListCLIsResponse, error) {
	items, err := s.clis.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*supportv1.CLI, 0, len(items))
	for _, item := range items {
		out = append(out, sanitizeCLI(enrichCLI(item)))
	}
	slices.SortFunc(out, func(left, right *supportv1.CLI) int {
		return strings.Compare(cliDisplayName(left), cliDisplayName(right))
	})
	return &supportv1.ListCLIsResponse{Items: out}, nil
}

func (s *Server) GetCLI(ctx context.Context, request *supportv1.GetCLIRequest) (*supportv1.GetCLIResponse, error) {
	item, err := s.clis.Get(ctx, request.GetCliId())
	if err != nil {
		return nil, err
	}
	return &supportv1.GetCLIResponse{Item: sanitizeCLI(enrichCLI(item))}, nil
}

func enrichCLI(in *supportv1.CLI) *supportv1.CLI {
	if in == nil {
		return &supportv1.CLI{}
	}
	next := proto.Clone(in).(*supportv1.CLI)
	next.ContainerImages = supportContainerImages(cliidentity.RegisteredContainerImages(next.GetCliId()))
	next.Capability = &supportv1.CLICapability{
		SupportsStreaming:    true,
		SupportsApprovalMode: true,
		SupportedProtocols:   supportProtocols(next),
	}
	return next
}

func supportContainerImages(images []*clidefinitionv1.CLIContainerImage) []*supportv1.CLIContainerImage {
	out := make([]*supportv1.CLIContainerImage, 0, len(images))
	for _, image := range images {
		if image == nil {
			continue
		}
		out = append(out, &supportv1.CLIContainerImage{
			ExecutionClass: strings.TrimSpace(image.GetExecutionClass()),
			Image:          strings.TrimSpace(image.GetImage()),
			CpuRequest:     strings.TrimSpace(image.GetCpuRequest()),
			MemoryRequest:  strings.TrimSpace(image.GetMemoryRequest()),
		})
	}
	return out
}

func supportProtocols(cli *supportv1.CLI) []apiprotocolv1.Protocol {
	seen := map[apiprotocolv1.Protocol]struct{}{}
	out := []apiprotocolv1.Protocol{}
	for _, item := range cli.GetApiKeyProtocols() {
		protocol := item.GetProtocol()
		if protocol == apiprotocolv1.Protocol_PROTOCOL_UNSPECIFIED {
			continue
		}
		if _, ok := seen[protocol]; ok {
			continue
		}
		seen[protocol] = struct{}{}
		out = append(out, protocol)
	}
	return out
}

func cliDisplayName(cli *supportv1.CLI) string {
	if cli == nil {
		return ""
	}
	if displayName := strings.TrimSpace(cli.GetDisplayName()); displayName != "" {
		return displayName
	}
	return strings.TrimSpace(cli.GetCliId())
}
