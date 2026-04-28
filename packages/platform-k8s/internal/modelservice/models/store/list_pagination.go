package store

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

type definitionListPageToken struct {
	Continue string `json:"continue,omitempty"`
	Offset   int64  `json:"offset,omitempty"`
}

type definitionListContinueToken struct {
	VendorID string `json:"vendor_id,omitempty"`
	ModelID  string `json:"model_id,omitempty"`
}

func decodeDefinitionListPageToken(raw string) (string, int64) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return raw, 0
	}
	token := &definitionListPageToken{}
	if err := json.Unmarshal(payload, token); err != nil {
		return raw, 0
	}
	if token.Offset < 0 {
		return strings.TrimSpace(token.Continue), 0
	}
	return strings.TrimSpace(token.Continue), token.Offset
}

func encodeDefinitionListPageToken(continueToken string, offset int64) string {
	continueToken = strings.TrimSpace(continueToken)
	if continueToken == "" {
		return ""
	}
	if offset < 0 {
		offset = 0
	}
	payload, err := json.Marshal(definitionListPageToken{
		Continue: continueToken,
		Offset:   offset,
	})
	if err != nil {
		return continueToken
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func encodeDefinitionListContinueToken(vendorID string, modelID string) string {
	vendorID = strings.TrimSpace(vendorID)
	modelID = strings.TrimSpace(modelID)
	if vendorID == "" || modelID == "" {
		return ""
	}
	payload, err := json.Marshal(definitionListContinueToken{
		VendorID: vendorID,
		ModelID:  modelID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeDefinitionListContinueToken(raw string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", "", false
	}
	token := &definitionListContinueToken{}
	if err := json.Unmarshal(payload, token); err != nil {
		return "", "", false
	}
	vendorID := strings.TrimSpace(token.VendorID)
	modelID := strings.TrimSpace(token.ModelID)
	if vendorID == "" || modelID == "" {
		return "", "", false
	}
	return vendorID, modelID, true
}

func estimateDefinitionListTotalCount(offset int64, itemCount int64, remaining *int64, hasMore bool) int64 {
	if offset < 0 {
		offset = 0
	}
	if itemCount < 0 {
		itemCount = 0
	}
	if remaining == nil {
		if hasMore {
			return 0
		}
		return offset + itemCount
	}
	if *remaining < 0 {
		if hasMore {
			return 0
		}
		return offset + itemCount
	}
	return offset + itemCount + *remaining
}
