package nvidia

import (
	"strings"

	"code-code.internal/platform-k8s/internal/modelservice/modelidentity"
	"code-code.internal/platform-k8s/internal/modelservice/models/source"
)

// SourceID is the canonical source identifier for NVIDIA Integrate.
const SourceID = "nvidia-integrate"

// Model represents one entry from the NVIDIA /v1/models API.
type Model struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
}

// Normalize transforms raw NVIDIA API models into grouped CollectedEntry maps.
func Normalize(items []Model, ctx source.CollectionContext) map[string][]*source.CollectedEntry {
	return source.NormalizeHostedModels(SourceID, items, ctx, func(item Model) (string, string, string, bool, bool) {
		owner, rawModelID, ok := strings.Cut(strings.TrimSpace(item.ID), "/")
		if !ok {
			return "", "", "", false, false
		}
		if ownedBy := strings.TrimSpace(item.OwnedBy); ownedBy != "" {
			owner = ownedBy
		}
		if shouldSkip(rawModelID) {
			return "", "", "", false, false
		}
		return owner, rawModelID, rawModelID, true, true
	})
}

func shouldSkip(rawModelID string) bool {
	if modelidentity.HasChannelToken(rawModelID) {
		return true
	}
	return modelidentity.HasModelToken(rawModelID,
		"asr",
		"calibration",
		"clip",
		"embed",
		"embedcode",
		"embedding",
		"embedqa",
		"guard",
		"moderation",
		"nemoguard",
		"nvclip",
		"parse",
		"pii",
		"rerank",
		"retriever",
		"reward",
		"safety",
		"transcribe",
		"translate",
		"tts",
	)
}
