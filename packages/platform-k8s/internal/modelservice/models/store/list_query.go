package store

import (
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"fmt"
	"strings"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
)

// resolveListFilter returns a ModelListFilter from the request, preferring
// the structured_filter field when present and falling back to the legacy
// string filter for backward compatibility.
func resolveListFilter(request *modelservicev1.ListModelsRequest) (*modelservicev1.ModelListFilter, error) {
	if request.GetStructuredFilter() != nil {
		return request.GetStructuredFilter(), nil
	}
	return parseDefinitionListFilter(request.GetFilter())
}

type definitionListFilter = modelservicev1.ModelListFilter

// parseDefinitionListFilter parses the legacy "field=value AND ..." string
// filter format into a structured ModelListFilter.
func parseDefinitionListFilter(raw string) (*modelservicev1.ModelListFilter, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return &modelservicev1.ModelListFilter{}, nil
	}

	filter := &modelservicev1.ModelListFilter{}
	clauses := strings.Split(raw, "AND")
	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		parts := strings.SplitN(clause, "=", 2)
		if len(parts) != 2 {
			return nil, validationError("unsupported filter clause %q", clause)
		}
		field := strings.TrimSpace(parts[0])
		values := parseFilterValues(parts[1])
		if len(values) == 0 {
			return nil, validationError("filter values are empty for %q", field)
		}
		switch field {
		case "vendor_id":
			filter.VendorIds = append([]string(nil), values...)
		case "model_id":
			if len(values) != 1 {
				return nil, validationError("model_id filter does not support multiple values")
			}
			filter.ModelId = values[0]
		case "model_id_query", "name_query":
			if len(values) != 1 {
				return nil, validationError("model_id_query filter does not support multiple values")
			}
			filter.ModelIdQuery = values[0]
		case "source_id":
			normalized := models.NormalizeDefinitionSourceIDs(values)
			if len(normalized) == 0 {
				return nil, validationError("unsupported source_id values")
			}
			filter.SourceIds = normalized
		case "badge":
			if len(values) != 1 {
				return nil, validationError("badge filter does not support multiple values")
			}
			badge := models.NormalizeDefinitionSourceBadge(values[0])
			if badge == "" {
				return nil, validationError("unsupported badge %q", values[0])
			}
			filter.Badge = badge
		case "category":
			if len(values) != 1 {
				return nil, validationError("category filter does not support multiple values")
			}
			filter.Category = strings.TrimSpace(values[0])
		case "lifecycle_status_exclude":
			filter.LifecycleStatusExclude = trimmedValues(values)
		default:
			return nil, validationError("unsupported filter field %q", field)
		}
	}

	return filter, nil
}

func validationError(format string, args ...any) error {
	return fmt.Errorf("platformk8s/models: "+format, args...)
}
