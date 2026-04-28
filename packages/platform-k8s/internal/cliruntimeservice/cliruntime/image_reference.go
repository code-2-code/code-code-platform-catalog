package cliruntime

import (
	"fmt"
	"strings"

	"code-code.internal/platform-k8s/internal/cliruntimeservice/cliversions"
	cliidentity "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/identity"
)

type imageReferencePlanner struct {
	RegistryPrefix string
}

func newImageReferencePlanner(registryPrefix string) (imageReferencePlanner, error) {
	registryPrefix = strings.TrimSpace(registryPrefix)
	if registryPrefix == "" {
		return imageReferencePlanner{}, fmt.Errorf("platformk8s/cliruntime: image registry is required")
	}
	return imageReferencePlanner{RegistryPrefix: registryPrefix}, nil
}

func (p imageReferencePlanner) RequestsForChanges(changes []cliversions.VersionChange) []ImageBuildRequest {
	requests := []ImageBuildRequest{}
	for _, change := range changes {
		cliID := strings.TrimSpace(change.CLIID)
		version := strings.TrimSpace(change.Current.Version)
		if cliID == "" || version == "" {
			continue
		}
		for _, image := range cliidentity.RegisteredContainerImages(cliID) {
			repository := imageRepository(image.GetImage())
			buildTarget := buildTargetForRepository(repository)
			if repository == "" || buildTarget == "" {
				continue
			}
			repository = applyRegistryPrefix(p.RegistryPrefix, repository)
			tag := imageBuildTag(version)
			requests = append(requests, ImageBuildRequest{
				RequestID:          imageBuildRequestID(cliID, version, buildTarget),
				CLIID:              cliID,
				CLIVersion:         version,
				PreviousCLIVersion: strings.TrimSpace(change.Previous.Version),
				ExecutionClass:     strings.TrimSpace(image.GetExecutionClass()),
				BuildTarget:        buildTarget,
				ImageRepository:    repository,
				ImageTag:           tag,
				Image:              repository + ":" + tag,
			})
		}
	}
	return requests
}
