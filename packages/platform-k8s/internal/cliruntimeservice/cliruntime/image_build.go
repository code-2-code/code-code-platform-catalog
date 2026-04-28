package cliruntime

import (
	"context"
	"fmt"
	"strings"

	cliversions "code-code.internal/platform-k8s/internal/cliruntimeservice/cliversions"
)

type imageBuildPlanner struct {
	imageReferencePlanner
	SourceContext  string
	SourceRevision string
}

type ImageBuildRequest struct {
	RequestID          string `json:"requestId"`
	CLIID              string `json:"cliId"`
	CLIVersion         string `json:"cliVersion"`
	PreviousCLIVersion string `json:"previousCliVersion,omitempty"`
	ExecutionClass     string `json:"executionClass"`
	BuildTarget        string `json:"buildTarget"`
	ImageRepository    string `json:"imageRepository"`
	ImageTag           string `json:"imageTag"`
	Image              string `json:"image"`
	SourceContext      string `json:"sourceContext,omitempty"`
	SourceRevision     string `json:"sourceRevision,omitempty"`
}

type ImageBuildDispatcher interface {
	DispatchImageBuild(context.Context, ImageBuildRequest) error
}

func newImageBuildPlanner(registryPrefix, sourceContext, sourceRevision string) (imageBuildPlanner, error) {
	sourceContext = strings.TrimSpace(sourceContext)
	sourceRevision = strings.TrimSpace(sourceRevision)
	if sourceContext == "" {
		return imageBuildPlanner{}, fmt.Errorf("platformk8s/cliruntime: image build source context is required")
	}
	if sourceRevision == "" {
		sourceRevision = "main"
	}
	references, err := newImageReferencePlanner(registryPrefix)
	if err != nil {
		return imageBuildPlanner{}, err
	}
	return imageBuildPlanner{
		imageReferencePlanner: references,
		SourceContext:         sourceContext,
		SourceRevision:        sourceRevision,
	}, nil
}

func (p imageBuildPlanner) RequestsForChanges(changes []cliversions.VersionChange) []ImageBuildRequest {
	requests := p.imageReferencePlanner.RequestsForChanges(changes)
	for i := range requests {
		requests[i].SourceContext = strings.TrimSpace(p.SourceContext)
		requests[i].SourceRevision = strings.TrimSpace(p.SourceRevision)
	}
	return requests
}

func imageBuildRequestID(cliID, version, buildTarget string) string {
	return fmt.Sprintf("cli-image-build:%s:%s:%s", cliID, version, buildTarget)
}

func imageBuildTag(version string) string {
	tag := "cli-" + strings.TrimSpace(version)
	var builder strings.Builder
	for _, r := range tag {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('-')
	}
	out := strings.Trim(builder.String(), ".-")
	if out == "" {
		return "cli-version"
	}
	if len(out) > 128 {
		out = out[:128]
	}
	return out
}

func imageRepository(image string) string {
	image = strings.TrimSpace(image)
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		return image[:lastColon]
	}
	return image
}

func buildTargetForRepository(repository string) string {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return ""
	}
	index := strings.LastIndex(repository, "/")
	if index < 0 {
		return repository
	}
	return repository[index+1:]
}

func applyRegistryPrefix(prefix, repository string) string {
	prefix = strings.TrimSpace(prefix)
	repository = strings.TrimLeft(strings.TrimSpace(repository), "/")
	if prefix == "" {
		return repository
	}
	return strings.TrimRight(prefix, "/") + "/" + repository
}
