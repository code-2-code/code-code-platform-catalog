package cliruntime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	cliruntimev1 "code-code.internal/go-contract/platform/cli_runtime/v1"
	"code-code.internal/platform-k8s/internal/cliruntimeservice/cliversions"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultRegistryListTimeout     = 10 * time.Second
	defaultRegistryListConcurrency = 4
)

type ServerConfig struct {
	Versions            cliversions.Store
	Registry            RegistryTagLister
	ImageRegistry       string
	ImageRegistryLookup string
	RegistryInsecure    bool
	RegistryTimeout     time.Duration
	RegistryConcurrency int
}

type Server struct {
	cliruntimev1.UnimplementedCLIRuntimeServiceServer

	versions             cliversions.Store
	registry             RegistryTagLister
	references           *imageReferencePlanner
	lookupRegistryPrefix string
	registryTimeout      time.Duration
	registryConcurrency  int
}

func NewServer(config ServerConfig) (*Server, error) {
	if config.Versions == nil {
		return nil, fmt.Errorf("platformk8s/cliruntime: version store is nil")
	}
	if config.Registry == nil {
		config.Registry = RemoteRegistryTagLister{Insecure: config.RegistryInsecure}
	}
	if config.RegistryTimeout <= 0 {
		config.RegistryTimeout = defaultRegistryListTimeout
	}
	if config.RegistryConcurrency <= 0 {
		config.RegistryConcurrency = defaultRegistryListConcurrency
	}
	var references *imageReferencePlanner
	registryPrefix := strings.TrimSpace(config.ImageRegistry)
	if registryPrefix != "" {
		imageReferences, err := newImageReferencePlanner(registryPrefix)
		if err != nil {
			return nil, err
		}
		references = &imageReferences
	}
	lookupRegistry := strings.TrimSpace(config.ImageRegistryLookup)
	if lookupRegistry == "" {
		lookupRegistry = registryPrefix
	}
	lookupRegistryPrefix := ""
	if lookupRegistry != "" {
		lookupReferences, err := newImageReferencePlanner(lookupRegistry)
		if err != nil {
			return nil, err
		}
		lookupRegistryPrefix = lookupReferences.RegistryPrefix
	}
	return &Server{
		versions:             config.Versions,
		registry:             config.Registry,
		references:           references,
		lookupRegistryPrefix: lookupRegistryPrefix,
		registryTimeout:      config.RegistryTimeout,
		registryConcurrency:  config.RegistryConcurrency,
	}, nil
}

func (s *Server) ListCLIRuntimeRecords(ctx context.Context, request *cliruntimev1.ListCLIRuntimeRecordsRequest) (*cliruntimev1.ListCLIRuntimeRecordsResponse, error) {
	cliIDFilter := strings.TrimSpace(request.GetCliId())
	state, err := s.versions.Load(ctx)
	if err != nil {
		return nil, err
	}

	records := map[string]*cliruntimev1.CLIRuntimeRecord{}
	for cliID, snapshot := range state.Versions {
		cliID = strings.TrimSpace(cliID)
		if cliID == "" || cliIDFilter != "" && cliID != cliIDFilter {
			continue
		}
		records[cliID] = &cliruntimev1.CLIRuntimeRecord{
			Version: &cliruntimev1.CLIVersionSnapshot{
				CliId:     cliID,
				Version:   strings.TrimSpace(snapshot.Version),
				UpdatedAt: timestampOrNil(snapshot.UpdatedAt),
			},
		}
		if s.references != nil {
			records[cliID].Images = protoImagesFromRequests(s.references.RequestsForChanges([]cliversions.VersionChange{{
				CLIID: cliID,
				Current: cliversions.Snapshot{
					Version: snapshot.Version,
				},
			}}))
		}
	}

	cliIDs := make([]string, 0, len(records))
	for cliID := range records {
		cliIDs = append(cliIDs, cliID)
	}
	sort.Strings(cliIDs)
	response := &cliruntimev1.ListCLIRuntimeRecordsResponse{
		Items: make([]*cliruntimev1.CLIRuntimeRecord, 0, len(cliIDs)),
	}
	for _, cliID := range cliIDs {
		sort.Slice(records[cliID].Images, func(i, j int) bool {
			left := records[cliID].Images[i]
			right := records[cliID].Images[j]
			if left.GetCliVersion() != right.GetCliVersion() {
				return left.GetCliVersion() > right.GetCliVersion()
			}
			return left.GetBuildTarget() < right.GetBuildTarget()
		})
		response.Items = append(response.Items, records[cliID])
	}
	return response, nil
}

func (s *Server) GetLatestAvailableCLIRuntimeImages(ctx context.Context, request *cliruntimev1.GetLatestAvailableCLIRuntimeImagesRequest) (*cliruntimev1.GetLatestAvailableCLIRuntimeImagesResponse, error) {
	if s.references == nil {
		return &cliruntimev1.GetLatestAvailableCLIRuntimeImagesResponse{}, nil
	}
	lookups, err := registeredRunnableImageLookups(strings.TrimSpace(request.GetCliId()), s.references.RegistryPrefix, s.lookupRegistryPrefix)
	if err != nil {
		return nil, err
	}
	response := &cliruntimev1.GetLatestAvailableCLIRuntimeImagesResponse{}
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(s.registryConcurrency)
	var mutex sync.Mutex
	for _, lookup := range lookups {
		lookup := lookup
		group.Go(func() error {
			tag, err := s.latestAvailableTag(groupCtx, lookup.LookupRepository)
			if err != nil {
				return err
			}
			if tag == "" {
				return nil
			}
			item := &cliruntimev1.CLIRuntimeImage{
				CliId:           lookup.CLIID,
				CliVersion:      strings.TrimPrefix(tag, "cli-"),
				ExecutionClass:  lookup.ExecutionClass,
				BuildTarget:     lookup.BuildTarget,
				ImageRepository: lookup.ImageRepository,
				ImageTag:        tag,
				Image:           lookup.ImageRepository + ":" + tag,
			}
			mutex.Lock()
			response.Items = append(response.Items, item)
			mutex.Unlock()
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	sort.Slice(response.Items, func(i, j int) bool {
		left := response.Items[i]
		right := response.Items[j]
		if left.GetCliId() != right.GetCliId() {
			return left.GetCliId() < right.GetCliId()
		}
		return left.GetBuildTarget() < right.GetBuildTarget()
	})
	return response, nil
}

func protoImagesFromRequests(requests []ImageBuildRequest) []*cliruntimev1.CLIRuntimeImage {
	images := make([]*cliruntimev1.CLIRuntimeImage, 0, len(requests))
	for _, request := range requests {
		images = append(images, &cliruntimev1.CLIRuntimeImage{
			CliId:           strings.TrimSpace(request.CLIID),
			CliVersion:      strings.TrimSpace(request.CLIVersion),
			ExecutionClass:  strings.TrimSpace(request.ExecutionClass),
			BuildTarget:     strings.TrimSpace(request.BuildTarget),
			ImageRepository: strings.TrimSpace(request.ImageRepository),
			ImageTag:        strings.TrimSpace(request.ImageTag),
			Image:           strings.TrimSpace(request.Image),
		})
	}
	return images
}

func timestampOrNil(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value.UTC())
}
