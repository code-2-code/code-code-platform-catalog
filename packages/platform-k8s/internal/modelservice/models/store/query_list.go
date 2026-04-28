package store

import (
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"context"
	"fmt"
	"strings"

	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
)

func (s *PostgresModelRegistry) List(ctx context.Context, request *modelservicev1.ListModelsRequest) (*modelservicev1.ListModelsResponse, error) {
	if request == nil {
		request = &modelservicev1.ListModelsRequest{}
	}
	filter, err := resolveListFilter(request)
	if err != nil {
		return nil, err
	}
	pageSize := normalizePageSize(request.GetPageSize())
	continueToken, offset := decodeDefinitionListPageToken(request.GetPageToken())
	if offset < 0 {
		offset = 0
	}
	predicate := newPostgresDefinitionListPredicate(s.namespace)
	if err := predicate.apply(filter); err != nil {
		return nil, err
	}
	continuationVendorID, continuationModelID, continuation := decodeDefinitionListContinueToken(continueToken)
	if continuation {
		predicate.add(fmt.Sprintf("(vendor_id, model_id) > (%s, %s)", predicate.arg(continuationVendorID), predicate.arg(continuationModelID)))
	}
	whereSQL, whereArgs := predicate.where()

	listArgs := append([]any(nil), whereArgs...)
	limitParam := appendPostgresArg(&listArgs, int64(pageSize+1))
	query := fmt.Sprintf(
		`select vendor_id, model_id, definition, source_ref_vendor_id, source_ref_model_id, badges, pricing
from %s
where %s
order by vendor_id, model_id
limit %s`,
		modelRegistryEntriesTable,
		whereSQL,
		limitParam,
	)
	if !continuation && offset > 0 {
		offsetParam := appendPostgresArg(&listArgs, offset)
		query += " offset " + offsetParam
	}
	rows, err := s.pool.Query(
		ctx,
		query,
		listArgs...,
	)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: list model registry entries: %w", err)
	}
	defer rows.Close()

	items := make([]*modelservicev1.ModelRegistryEntry, 0, pageSize+1)
	identities := make([]models.SurfaceIdentity, 0, pageSize+1)
	for rows.Next() {
		identity, row, err := s.scanRegistryEntryBase(rows.Scan)
		if err != nil {
			return nil, err
		}
		identities = append(identities, identity)
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
		identities = identities[:pageSize]
	}
	if len(identities) > 0 {
		sourcesByIdentity, err := s.listEntrySourcesBatch(ctx, identities)
		if err != nil {
			return nil, err
		}
		for idx, identity := range identities {
			items[idx].Sources = sourcesByIdentity[identity.Key()]
		}
	}
	nextOffset := offset + int64(len(items))
	nextPageToken := ""
	if hasMore && len(identities) > 0 {
		lastIdentity := identities[len(identities)-1]
		nextPageToken = encodeDefinitionListPageToken(
			encodeDefinitionListContinueToken(lastIdentity.VendorID, lastIdentity.ModelID),
			nextOffset,
		)
	}
	return &modelservicev1.ListModelsResponse{
		Items:         items,
		NextPageToken: nextPageToken,
		TotalCount:    estimateDefinitionListTotalCount(offset, int64(len(items)), nil, hasMore),
	}, nil
}

// postgresDefinitionListPredicate builds parameterized WHERE clauses for
// definition list queries.
type postgresDefinitionListPredicate struct {
	clauses []string
	args    []any
}

func newPostgresDefinitionListPredicate(namespace string) *postgresDefinitionListPredicate {
	p := &postgresDefinitionListPredicate{}
	p.add("namespace = " + p.arg(strings.TrimSpace(namespace)))
	return p
}

func (p *postgresDefinitionListPredicate) apply(filter *modelservicev1.ModelListFilter) error {
	if filter == nil {
		return nil
	}
	if len(filter.GetVendorIds()) > 0 {
		p.add("vendor_id = any(" + p.arg(normalizeVendorIDs(filter.GetVendorIds())) + "::text[])")
	}
	if strings.TrimSpace(filter.GetModelId()) != "" {
		p.add("model_id = " + p.arg(strings.TrimSpace(filter.GetModelId())))
	}
	if filter.GetBadge() != "" {
		badge := models.NormalizeDefinitionSourceBadge(filter.GetBadge())
		if badge != "" {
			raw, err := encodeStringSlice([]string{badge})
			if err != nil {
				return err
			}
			p.add("badges @> " + p.arg(string(raw)) + "::jsonb")
		}
	}
	if strings.TrimSpace(filter.GetModelIdQuery()) != "" {
		p.add("lower(model_id) like " + p.arg(likeContainsPattern(filter.GetModelIdQuery())) + " escape '\\'")
	}
	if len(filter.GetSourceIds()) > 0 {
		p.add(fmt.Sprintf(`exists (
select 1 from %s observations
where observations.namespace = %s.namespace
  and observations.vendor_id = %s.vendor_id
  and observations.model_id = %s.model_id
  and observations.source_id = any(%s::text[])
)`, modelRegistryObservationsTable, modelRegistryEntriesTable, modelRegistryEntriesTable, modelRegistryEntriesTable, p.arg(models.NormalizeDefinitionSourceIDs(filter.GetSourceIds()))))
	}
	if filter.GetCategory() != "" {
		categoryInt := categoryFilterToInt(filter.GetCategory())
		if categoryInt >= 0 {
			p.add(fmt.Sprintf("(definition->>'category')::int = %s", p.arg(categoryInt)))
		}
	}
	if len(filter.GetLifecycleStatusExclude()) > 0 {
		excludeInts := lifecycleStatusFilterToInts(filter.GetLifecycleStatusExclude())
		if len(excludeInts) > 0 {
			p.add(fmt.Sprintf("coalesce((definition->>'lifecycleStatus')::int, 0) != all(%s::int[])", p.arg(excludeInts)))
		}
	}
	return nil
}

func (p *postgresDefinitionListPredicate) add(clause string) {
	if strings.TrimSpace(clause) != "" {
		p.clauses = append(p.clauses, clause)
	}
}

func (p *postgresDefinitionListPredicate) arg(value any) string {
	p.args = append(p.args, value)
	return fmt.Sprintf("$%d", len(p.args))
}

func (p *postgresDefinitionListPredicate) where() (string, []any) {
	if len(p.clauses) == 0 {
		return "true", append([]any(nil), p.args...)
	}
	return strings.Join(p.clauses, " and "), append([]any(nil), p.args...)
}

func appendPostgresArg(args *[]any, value any) string {
	*args = append(*args, value)
	return fmt.Sprintf("$%d", len(*args))
}

func likeContainsPattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(strings.ToLower(strings.TrimSpace(value))) + "%"
}

func normalizeVendorIDs(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if normalized := models.NormalizedVendorSlug(value); normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}

func trimmedValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
