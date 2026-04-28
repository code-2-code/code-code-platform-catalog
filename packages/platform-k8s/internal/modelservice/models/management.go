package models

import (
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
)

const (
	defaultListPageSize = 50
	maxListPageSize     = 100
)

func parseFilterValues(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func applyCreateDefaults(definition *modelv1.ModelVersion) {
	if definition.PrimaryShape == modelv1.ModelShape_MODEL_SHAPE_UNSPECIFIED {
		definition.PrimaryShape = modelv1.ModelShape_MODEL_SHAPE_CHAT_COMPLETIONS
	}
	if len(definition.SupportedShapes) == 0 {
		definition.SupportedShapes = []modelv1.ModelShape{definition.PrimaryShape}
	}
	if len(definition.InputModalities) == 0 {
		definition.InputModalities = []modelv1.Modality{modelv1.Modality_MODALITY_TEXT}
	}
	if len(definition.OutputModalities) == 0 {
		definition.OutputModalities = []modelv1.Modality{modelv1.Modality_MODALITY_TEXT}
	}
}

func normalizePageSize(pageSize int32) int {
	if pageSize <= 0 {
		return defaultListPageSize
	}
	if pageSize > maxListPageSize {
		return maxListPageSize
	}
	return int(pageSize)
}
