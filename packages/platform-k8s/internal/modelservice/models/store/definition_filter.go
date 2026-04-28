package store

import (
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
)

// categoryFilterToInt maps a category filter string to the proto enum int value.
// Returns -1 if the category name is not recognized.
func categoryFilterToInt(category string) int {
	switch strings.TrimSpace(strings.ToLower(category)) {
	case "chat", "1":
		return int(modelv1.ModelCategory_MODEL_CATEGORY_CHAT)
	case "embedding", "2":
		return int(modelv1.ModelCategory_MODEL_CATEGORY_EMBEDDING)
	case "rerank", "3":
		return int(modelv1.ModelCategory_MODEL_CATEGORY_RERANK)
	case "image_gen", "4":
		return int(modelv1.ModelCategory_MODEL_CATEGORY_IMAGE_GEN)
	case "audio", "5":
		return int(modelv1.ModelCategory_MODEL_CATEGORY_AUDIO)
	case "video", "6":
		return int(modelv1.ModelCategory_MODEL_CATEGORY_VIDEO)
	case "moderation", "7":
		return int(modelv1.ModelCategory_MODEL_CATEGORY_MODERATION)
	default:
		return -1
	}
}

// lifecycleStatusFilterToInts maps lifecycle status filter strings to proto enum int values.
func lifecycleStatusFilterToInts(statuses []string) []int {
	out := make([]int, 0, len(statuses))
	for _, status := range statuses {
		v := lifecycleStatusToInt(strings.TrimSpace(strings.ToLower(status)))
		if v >= 0 {
			out = append(out, v)
		}
	}
	return out
}

func lifecycleStatusToInt(status string) int {
	switch status {
	case "active", "1":
		return int(modelv1.ModelLifecycleStatus_MODEL_LIFECYCLE_STATUS_ACTIVE)
	case "legacy", "2":
		return int(modelv1.ModelLifecycleStatus_MODEL_LIFECYCLE_STATUS_LEGACY)
	case "deprecated", "3":
		return int(modelv1.ModelLifecycleStatus_MODEL_LIFECYCLE_STATUS_DEPRECATED)
	case "eol", "4":
		return int(modelv1.ModelLifecycleStatus_MODEL_LIFECYCLE_STATUS_EOL)
	case "blocked", "5":
		return int(modelv1.ModelLifecycleStatus_MODEL_LIFECYCLE_STATUS_BLOCKED)
	default:
		return -1
	}
}
