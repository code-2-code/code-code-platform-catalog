package cliruntime

import (
	"strings"

	domaineventv1 "code-code.internal/go-contract/platform/domain_event/v1"
)

func imageBuildRequestFromProto(input *domaineventv1.CLIImageBuildRequest) ImageBuildRequest {
	if input == nil {
		return ImageBuildRequest{}
	}
	return ImageBuildRequest{
		RequestID:          strings.TrimSpace(input.GetRequestId()),
		CLIID:              strings.TrimSpace(input.GetCliId()),
		CLIVersion:         strings.TrimSpace(input.GetCliVersion()),
		PreviousCLIVersion: strings.TrimSpace(input.GetPreviousCliVersion()),
		BuildTarget:        strings.TrimSpace(input.GetBuildTarget()),
		ImageRepository:    strings.TrimSpace(input.GetImageRepository()),
		ImageTag:           strings.TrimSpace(input.GetImageTag()),
		Image:              strings.TrimSpace(input.GetImage()),
		SourceContext:      strings.TrimSpace(input.GetSourceContext()),
		SourceRevision:     strings.TrimSpace(input.GetSourceRevision()),
	}
}
