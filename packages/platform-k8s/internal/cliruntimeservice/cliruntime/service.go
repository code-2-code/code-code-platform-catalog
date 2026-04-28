package cliruntime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	cliversions "code-code.internal/platform-k8s/internal/cliruntimeservice/cliversions"
)

const cliVersionSyncTimeout = 2 * time.Minute

type VersionSyncer interface {
	Sync(context.Context) (*cliversions.SyncResult, error)
}

type Config struct {
	VersionSyncer  VersionSyncer
	Dispatcher     ImageBuildDispatcher
	ImageRegistry  string
	SourceContext  string
	SourceRevision string
	Logger         *slog.Logger
	SyncTimeout    time.Duration
}

type Service struct {
	versionSyncer VersionSyncer
	dispatcher    ImageBuildDispatcher
	planner       *imageBuildPlanner
	logger        *slog.Logger
	syncTimeout   time.Duration
}

type SyncCLIVersionsResult struct {
	Status             string              `json:"status"`
	VersionChangeCount int                 `json:"versionChangeCount"`
	ImageBuildRequests []ImageBuildRequest `json:"imageBuildRequests"`
}

func NewService(config Config) (*Service, error) {
	if config.VersionSyncer == nil {
		return nil, fmt.Errorf("platformk8s/cliruntime: version syncer is nil")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.SyncTimeout <= 0 {
		config.SyncTimeout = cliVersionSyncTimeout
	}
	var planner *imageBuildPlanner
	imageBuildConfigured := strings.TrimSpace(config.ImageRegistry) != "" || strings.TrimSpace(config.SourceContext) != ""
	if imageBuildConfigured {
		if config.Dispatcher == nil {
			return nil, fmt.Errorf("platformk8s/cliruntime: image build dispatcher is nil")
		}
		imageBuildPlanner, err := newImageBuildPlanner(config.ImageRegistry, config.SourceContext, config.SourceRevision)
		if err != nil {
			return nil, err
		}
		planner = &imageBuildPlanner
	} else {
		config.Logger.Warn("cli image build disabled", "reason", "image registry and source context are not configured")
	}
	return &Service{
		versionSyncer: config.VersionSyncer,
		dispatcher:    config.Dispatcher,
		planner:       planner,
		logger:        config.Logger,
		syncTimeout:   config.SyncTimeout,
	}, nil
}

func (s *Service) SyncCLIVersions(ctx context.Context) (*SyncCLIVersionsResult, error) {
	if s == nil {
		return nil, fmt.Errorf("platformk8s/cliruntime: service is nil")
	}
	syncCtx, cancel := context.WithTimeout(ctx, s.syncTimeout)
	defer cancel()
	syncResult, syncErr := s.versionSyncer.Sync(syncCtx)
	if syncResult == nil {
		syncResult = &cliversions.SyncResult{}
	}
	requests := []ImageBuildRequest{}
	if s.planner != nil {
		requests = s.planner.RequestsForChanges(syncResult.Changes)
	} else if len(syncResult.Changes) > 0 {
		s.logger.Warn("cli image build skipped", "version_changes", len(syncResult.Changes))
	}
	var errs []error
	if syncErr != nil {
		errs = append(errs, syncErr)
	}
	for _, request := range requests {
		if err := s.dispatcher.DispatchImageBuild(syncCtx, request); err != nil {
			errs = append(errs, err)
			continue
		}
		s.logger.Info("cli image build requested", "cli_id", request.CLIID, "version", request.CLIVersion, "target", request.BuildTarget, "image", request.Image)
	}
	return &SyncCLIVersionsResult{
		Status:             "synced",
		VersionChangeCount: len(syncResult.Changes),
		ImageBuildRequests: requests,
	}, errors.Join(errs...)
}
