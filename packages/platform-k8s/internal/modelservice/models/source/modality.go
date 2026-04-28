package source

import (
	"slices"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
)

// ParseModalities converts string modality names to proto enum values.
func ParseModalities(values []string) []modelv1.Modality {
	set := map[modelv1.Modality]struct{}{}
	for _, value := range values {
		switch strings.TrimSpace(value) {
		case "text":
			set[modelv1.Modality_MODALITY_TEXT] = struct{}{}
		case "image":
			set[modelv1.Modality_MODALITY_IMAGE] = struct{}{}
		case "audio":
			set[modelv1.Modality_MODALITY_AUDIO] = struct{}{}
		case "video":
			set[modelv1.Modality_MODALITY_VIDEO] = struct{}{}
		}
	}
	out := make([]modelv1.Modality, 0, len(set))
	for modality := range set {
		out = append(out, modality)
	}
	slices.Sort(out)
	return out
}
