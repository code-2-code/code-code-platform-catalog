package models

import (
	"slices"
	"strings"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"google.golang.org/protobuf/proto"
)

const (
	SourceBadgeExacto   = "exacto"
	SourceBadgeExtended = "extended"
	SourceBadgeFloor    = "floor"
	SourceBadgeFree     = "free"
	SourceBadgeNitro    = "nitro"
	SourceBadgeOnline   = "online"
	SourceBadgeThinking = "thinking"
)

func NormalizeDefinitionSourceBadges(badges []string) []string {
	if len(badges) == 0 {
		return nil
	}
	out := make([]string, 0, len(badges))
	seen := map[string]struct{}{}
	for _, badge := range badges {
		badge = NormalizeDefinitionSourceBadge(badge)
		if badge == "" {
			continue
		}
		if _, ok := seen[badge]; ok {
			continue
		}
		seen[badge] = struct{}{}
		out = append(out, badge)
	}
	slices.Sort(out)
	return out
}

func NormalizeDefinitionSourceBadge(badge string) string {
	switch strings.TrimSpace(strings.ToLower(badge)) {
	case SourceBadgeExacto:
		return SourceBadgeExacto
	case SourceBadgeExtended:
		return SourceBadgeExtended
	case SourceBadgeFloor:
		return SourceBadgeFloor
	case SourceBadgeFree:
		return SourceBadgeFree
	case SourceBadgeNitro:
		return SourceBadgeNitro
	case SourceBadgeOnline:
		return SourceBadgeOnline
	case SourceBadgeThinking:
		return SourceBadgeThinking
	default:
		return ""
	}
}

func MergeDefinitionSourceBadges(left []string, right []string) []string {
	return NormalizeDefinitionSourceBadges(append(append([]string(nil), left...), right...))
}

func NormalizePricingSummary(pricing *modelservicev1.PricingSummary) *modelservicev1.PricingSummary {
	if pricing == nil {
		return nil
	}
	normalized := &modelservicev1.PricingSummary{
		Input:           strings.TrimSpace(pricing.GetInput()),
		Output:          strings.TrimSpace(pricing.GetOutput()),
		CacheReadInput:  strings.TrimSpace(pricing.GetCacheReadInput()),
		CacheWriteInput: strings.TrimSpace(pricing.GetCacheWriteInput()),
		Reasoning:       strings.TrimSpace(pricing.GetReasoning()),
		ImageInput:      strings.TrimSpace(pricing.GetImageInput()),
		AudioInput:      strings.TrimSpace(pricing.GetAudioInput()),
		AudioOutput:     strings.TrimSpace(pricing.GetAudioOutput()),
		Request:         strings.TrimSpace(pricing.GetRequest()),
		Currency:        strings.TrimSpace(pricing.GetCurrency()),
		PriceType:       pricing.GetPriceType(),
	}
	if normalized.Input == "" && normalized.Output == "" && normalized.CacheReadInput == "" && normalized.CacheWriteInput == "" &&
		normalized.Reasoning == "" && normalized.ImageInput == "" && normalized.AudioInput == "" && normalized.AudioOutput == "" && normalized.Request == "" {
		return nil
	}
	return normalized
}

func ClonePricingSummary(pricing *modelservicev1.PricingSummary) *modelservicev1.PricingSummary {
	pricing = NormalizePricingSummary(pricing)
	if pricing == nil {
		return nil
	}
	return proto.Clone(pricing).(*modelservicev1.PricingSummary)
}

func MergePricingSummary(primary *modelservicev1.PricingSummary, fallback *modelservicev1.PricingSummary) *modelservicev1.PricingSummary {
	primary = ClonePricingSummary(primary)
	fallback = NormalizePricingSummary(fallback)
	if primary == nil {
		return ClonePricingSummary(fallback)
	}
	if fallback == nil {
		return primary
	}
	fillEmpty := func(dst *string, src string) {
		if *dst == "" {
			*dst = src
		}
	}
	fillEmpty(&primary.Input, fallback.Input)
	fillEmpty(&primary.Output, fallback.Output)
	fillEmpty(&primary.CacheReadInput, fallback.CacheReadInput)
	fillEmpty(&primary.CacheWriteInput, fallback.CacheWriteInput)
	fillEmpty(&primary.Reasoning, fallback.Reasoning)
	fillEmpty(&primary.ImageInput, fallback.ImageInput)
	fillEmpty(&primary.AudioInput, fallback.AudioInput)
	fillEmpty(&primary.AudioOutput, fallback.AudioOutput)
	fillEmpty(&primary.Request, fallback.Request)
	fillEmpty(&primary.Currency, fallback.Currency)
	if primary.PriceType == modelservicev1.PriceType_PRICE_TYPE_UNSPECIFIED {
		primary.PriceType = fallback.PriceType
	}
	return NormalizePricingSummary(primary)
}

