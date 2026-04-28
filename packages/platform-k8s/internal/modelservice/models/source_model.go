package models

import (
	"slices"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"google.golang.org/protobuf/proto"
)

const (
	SourceIDOpenRouter      = "openrouter"
	SourceIDModelScope      = "modelscope"
	SourceIDCerebras        = "cerebras"
	SourceIDGitHubModels    = "github-models"
	SourceIDNVIDIAIntegrate = "nvidia-integrate"
	SourceIDHuggingFaceHub  = "huggingface-hub"
)

func NormalizeCollectedSources(sources []*modelservicev1.CollectedModelSource) []*modelservicev1.CollectedModelSource {
	if len(sources) == 0 {
		return nil
	}
	merged := map[string]*modelservicev1.CollectedModelSource{}
	for _, source := range sources {
		source = NormalizeCollectedSource(source)
		key := collectedSourceIdentityKey(source)
		if key == "" {
			continue
		}
		if current, ok := merged[key]; ok {
			merged[key] = mergeCollectedSource(current, source)
			continue
		}
		merged[key] = source
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(left, right string) int {
		return CompareCollectedSource(merged[left], merged[right])
	})
	out := make([]*modelservicev1.CollectedModelSource, 0, len(keys))
	for _, key := range keys {
		out = append(out, merged[key])
	}
	return out
}

func CloneCollectedSources(sources []*modelservicev1.CollectedModelSource) []*modelservicev1.CollectedModelSource {
	sources = NormalizeCollectedSources(sources)
	if len(sources) == 0 {
		return nil
	}
	out := make([]*modelservicev1.CollectedModelSource, 0, len(sources))
	for _, source := range sources {
		out = append(out, proto.Clone(source).(*modelservicev1.CollectedModelSource))
	}
	return out
}

func MergeCollectedSourceSlices(left []*modelservicev1.CollectedModelSource, right []*modelservicev1.CollectedModelSource) []*modelservicev1.CollectedModelSource {
	combined := make([]*modelservicev1.CollectedModelSource, 0, len(left)+len(right))
	combined = append(combined, left...)
	combined = append(combined, right...)
	return NormalizeCollectedSources(combined)
}

func NormalizeCollectedSource(source *modelservicev1.CollectedModelSource) *modelservicev1.CollectedModelSource {
	if source == nil {
		return nil
	}
	out := proto.Clone(source).(*modelservicev1.CollectedModelSource)
	out.VendorId = NormalizedVendorSlug(out.GetVendorId())
	out.ModelId = strings.TrimSpace(out.GetModelId())
	out.SourceId = NormalizedVendorSlug(out.GetSourceId())
	out.SourceModelId = strings.TrimSpace(out.GetSourceModelId())
	out.DisplayName = strings.TrimSpace(out.GetDisplayName())
	out.Definition = CloneModelDefinition(out.GetDefinition())
	out.Badges = NormalizeDefinitionSourceBadges(out.GetBadges())
	out.Pricing = NormalizePricingSummary(out.GetPricing())
	if out.GetVendorId() == "" || out.GetModelId() == "" || out.GetSourceId() == "" {
		return nil
	}
	return out
}

func collectedSourceIdentityKey(source *modelservicev1.CollectedModelSource) string {
	source = NormalizeCollectedSource(source)
	if source == nil {
		return ""
	}
	return source.GetVendorId() + "\x00" + source.GetModelId() + "\x00" + source.GetSourceId() + "\x00" + source.GetSourceModelId() + "\x00" + boolKey(source.GetIsDirect())
}

func boolKey(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func mergeCollectedSource(primary *modelservicev1.CollectedModelSource, fallback *modelservicev1.CollectedModelSource) *modelservicev1.CollectedModelSource {
	primary = NormalizeCollectedSource(primary)
	fallback = NormalizeCollectedSource(fallback)
	if primary == nil {
		return fallback
	}
	if fallback == nil {
		return primary
	}
	if primary.GetSourceModelId() == "" {
		primary.SourceModelId = fallback.GetSourceModelId()
	}
	if primary.GetDefinition() == nil {
		primary.Definition = CloneModelDefinition(fallback.GetDefinition())
	} else {
		primary.Definition = mergeDefinitionMetadata(primary.GetDefinition(), fallback.GetDefinition())
	}
	if fallback.GetAuthorityPriority() > primary.GetAuthorityPriority() {
		primary.AuthorityPriority = fallback.GetAuthorityPriority()
	}
	primary.Badges = MergeDefinitionSourceBadges(primary.GetBadges(), fallback.GetBadges())
	primary.Pricing = MergePricingSummary(primary.GetPricing(), fallback.GetPricing())
	return primary
}

func CompareCollectedSource(left *modelservicev1.CollectedModelSource, right *modelservicev1.CollectedModelSource) int {
	// Direct sources come first.
	if left.GetIsDirect() != right.GetIsDirect() {
		if left.GetIsDirect() {
			return -1
		}
		return 1
	}
	// Higher authority priority first.
	if left.GetAuthorityPriority() != right.GetAuthorityPriority() {
		return int(right.GetAuthorityPriority() - left.GetAuthorityPriority())
	}
	if v := strings.Compare(left.GetSourceId(), right.GetSourceId()); v != 0 {
		return v
	}
	if v := strings.Compare(left.GetVendorId(), right.GetVendorId()); v != 0 {
		return v
	}
	if v := strings.Compare(left.GetModelId(), right.GetModelId()); v != 0 {
		return v
	}
	return strings.Compare(left.GetSourceModelId(), right.GetSourceModelId())
}


func CloneModelDefinition(in *modelv1.ModelVersion) *modelv1.ModelVersion {
	if in == nil {
		return nil
	}
	return proto.Clone(in).(*modelv1.ModelVersion)
}
