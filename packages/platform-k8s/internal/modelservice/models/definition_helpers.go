package models

import (
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	"google.golang.org/protobuf/proto"
)

func collectDefinitionAliases(definition *modelv1.ModelVersion) []string {
	if definition == nil {
		return nil
	}
	out := make([]string, 0, len(definition.GetAliases()))
	seen := map[string]struct{}{}
	for _, alias := range definition.GetAliases() {
		value := strings.TrimSpace(alias.GetValue())
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func hasDefinitionAlias(definition *modelv1.ModelVersion, alias string) bool {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return false
	}
	for _, value := range collectDefinitionAliases(definition) {
		if value == alias {
			return true
		}
	}
	return false
}

func refForDefinition(definition *modelv1.ModelVersion) *modelv1.ModelRef {
	if definition == nil {
		return nil
	}
	return &modelv1.ModelRef{
		ModelId:  definition.GetModelId(),
		VendorId: definition.GetVendorId(),
	}
}

func cloneModelRef(ref *modelv1.ModelRef) *modelv1.ModelRef {
	if ref == nil {
		return nil
	}
	return proto.Clone(ref).(*modelv1.ModelRef)
}
