package store

import (
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"encoding/json"
	"fmt"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

var modelDefinitionJSON = protojson.MarshalOptions{UseProtoNames: true}

func encodeModelDefinition(definition *modelv1.ModelVersion) ([]byte, error) {
	if definition == nil {
		return []byte("null"), nil
	}
	raw, err := modelDefinitionJSON.Marshal(definition)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: encode model definition: %w", err)
	}
	return raw, nil
}

var modelDefinitionUnmarshalOpts = protojson.UnmarshalOptions{DiscardUnknown: true}

func decodeModelDefinition(raw []byte) (*modelv1.ModelVersion, error) {
	definition := &modelv1.ModelVersion{}
	if len(raw) == 0 || string(raw) == "null" {
		return definition, nil
	}
	if err := modelDefinitionUnmarshalOpts.Unmarshal(raw, definition); err != nil {
		return nil, fmt.Errorf("platformk8s/models: decode model definition: %w", err)
	}
	return definition, nil
}

func encodeStringSlice(values []string) ([]byte, error) {
	raw, err := json.Marshal(models.NormalizeDefinitionSourceBadges(values))
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: encode string slice: %w", err)
	}
	return raw, nil
}

func decodeStringSlice(raw []byte) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("platformk8s/models: decode string slice: %w", err)
	}
	return models.NormalizeDefinitionSourceBadges(values), nil
}

func encodePricing(pricing *modelservicev1.PricingSummary) ([]byte, error) {
	pricing = models.NormalizePricingSummary(pricing)
	if pricing == nil {
		return []byte("null"), nil
	}
	raw, err := protojson.Marshal(pricing)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: encode pricing: %w", err)
	}
	return raw, nil
}

func decodePricing(raw []byte) (*modelservicev1.PricingSummary, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var pricing modelservicev1.PricingSummary
	if err := protojson.Unmarshal(raw, &pricing); err != nil {
		return nil, fmt.Errorf("platformk8s/models: decode pricing: %w", err)
	}
	return models.NormalizePricingSummary(&pricing), nil
}

func normalizeRegistryObservations(observations []*modelservicev1.RegistryModelSource) []*modelservicev1.RegistryModelSource {
	if len(observations) == 0 {
		return nil
	}
	out := make([]*modelservicev1.RegistryModelSource, 0, len(observations))
	seen := map[string]struct{}{}
	for _, observation := range observations {
		normalized := models.NormalizeRegistryModelSource(observation)
		if normalized == nil || normalized.GetSourceId() == "" {
			continue
		}
		key := normalized.GetSourceId() + "\x00" + normalized.GetSourceModelId() + "\x00" + boolKey(normalized.GetIsDirect())
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeModelDefinitionAliases(definition *modelv1.ModelVersion) []*modelv1.ModelAlias {
	if definition == nil {
		return nil
	}
	out := make([]*modelv1.ModelAlias, 0, len(definition.GetAliases()))
	seen := map[string]struct{}{}
	for _, alias := range definition.GetAliases() {
		if alias == nil {
			continue
		}
		value := strings.TrimSpace(alias.GetValue())
		if value == "" {
			continue
		}
		key := alias.GetKind().String() + "\x00" + value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, &modelv1.ModelAlias{
			Kind:  alias.GetKind(),
			Value: value,
		})
	}
	return out
}
