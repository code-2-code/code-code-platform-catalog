package cliruntime

import (
	"context"
	"sort"
	"strings"

	cliidentity "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/identity"
	clisupport "code-code.internal/platform-k8s/internal/supportservice/clidefinitions/support"
	"golang.org/x/mod/semver"
)

type runtimeImageLookup struct {
	CLIID            string
	ExecutionClass   string
	ImageRepository  string
	LookupRepository string
	BuildTarget      string
}

func registeredRunnableImageLookups(cliIDFilter, imageRegistryPrefix, lookupRegistryPrefix string) ([]runtimeImageLookup, error) {
	cliIDs, err := registeredRunnableCLIIDs(cliIDFilter)
	if err != nil {
		return nil, err
	}
	lookups := []runtimeImageLookup{}
	for _, cliID := range cliIDs {
		for _, image := range cliidentity.RegisteredContainerImages(cliID) {
			baseRepository := imageRepository(image.GetImage())
			imageRepository := applyRegistryPrefix(imageRegistryPrefix, baseRepository)
			lookupRepository := applyRegistryPrefix(lookupRegistryPrefix, baseRepository)
			buildTarget := buildTargetForRepository(baseRepository)
			if imageRepository == "" || lookupRepository == "" || buildTarget == "" {
				continue
			}
			lookups = append(lookups, runtimeImageLookup{
				CLIID:            cliID,
				ExecutionClass:   strings.TrimSpace(image.GetExecutionClass()),
				ImageRepository:  imageRepository,
				LookupRepository: lookupRepository,
				BuildTarget:      buildTarget,
			})
		}
	}
	return lookups, nil
}

func registeredRunnableCLIIDs(cliIDFilter string) ([]string, error) {
	if cliIDFilter != "" {
		if len(cliidentity.RegisteredContainerImages(cliIDFilter)) == 0 {
			return nil, nil
		}
		return []string{cliIDFilter}, nil
	}
	clis, err := clisupport.RegisteredCLIs()
	if err != nil {
		return nil, err
	}
	cliIDs := []string{}
	for _, cli := range clis {
		cliID := strings.TrimSpace(cli.GetCliId())
		if cliID == "" || len(cliidentity.RegisteredContainerImages(cliID)) == 0 {
			continue
		}
		cliIDs = append(cliIDs, cliID)
	}
	sort.Strings(cliIDs)
	return cliIDs, nil
}

func (s *Server) latestAvailableTag(ctx context.Context, repository string) (string, error) {
	listCtx, cancel := context.WithTimeout(ctx, s.registryTimeout)
	defer cancel()
	tags, err := s.registry.ListTags(listCtx, repository)
	if err != nil {
		return "", err
	}
	latest := ""
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if !strings.HasPrefix(tag, "cli-") {
			continue
		}
		if latest == "" || compareCLITags(tag, latest) > 0 {
			latest = tag
		}
	}
	return latest, nil
}

func compareCLITags(left, right string) int {
	leftVersion := normalizeSemverTag(left)
	rightVersion := normalizeSemverTag(right)
	if semver.IsValid(leftVersion) && semver.IsValid(rightVersion) {
		return semver.Compare(leftVersion, rightVersion)
	}
	return strings.Compare(left, right)
}

func normalizeSemverTag(tag string) string {
	version := strings.TrimPrefix(strings.TrimSpace(tag), "cli-")
	version = strings.TrimPrefix(version, "v")
	if version == "" {
		return ""
	}
	return "v" + version
}
