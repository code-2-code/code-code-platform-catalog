package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code-code.internal/go-contract/domainerror"
	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresModelRegistry) Get(ctx context.Context, request *modelservicev1.GetModelVersionRequest) (*modelservicev1.GetModelVersionResponse, error) {
	ref := request.GetRef()
	if ref == nil {
		return nil, domainerror.NewValidation("platformk8s/models: model ref is required")
	}
	if strings.TrimSpace(ref.GetVendorId()) == "" || strings.TrimSpace(ref.GetModelId()) == "" {
		return nil, domainerror.NewValidation("platformk8s/models: model ref is required")
	}
	item, err := s.getRegistryEntry(ctx, ref.GetVendorId(), ref.GetModelId())
	if err != nil {
		return nil, err
	}
	return &modelservicev1.GetModelVersionResponse{Item: item}, nil
}

func (s *PostgresModelRegistry) Resolve(ctx context.Context, request *modelservicev1.ResolveModelRefRequest) (*modelservicev1.ResolveModelRefResponse, error) {
	modelIDOrAlias := strings.TrimSpace(request.GetModelIdOrAlias())
	if modelIDOrAlias == "" {
		return nil, domainerror.NewValidation("platformk8s/models: model_id_or_alias is required")
	}
	vendorID := strings.TrimSpace(request.GetVendorId())
	if vendorID != "" {
		if _, err := s.getRegistryEntry(ctx, vendorID, modelIDOrAlias); err == nil {
			return &modelservicev1.ResolveModelRefResponse{
				Ref: &modelv1.ModelRef{VendorId: vendorID, ModelId: modelIDOrAlias},
			}, nil
		} else if !isDomainNotFound(err) {
			return nil, err
		}
	}

	exactMatches, err := s.queryResolveRefsByModelID(ctx, vendorID, modelIDOrAlias)
	if err != nil {
		return nil, err
	}
	if ref, err := resolveUniqueModelRef(exactMatches, vendorID, modelIDOrAlias, "model_id"); err != nil || ref != nil {
		if err != nil {
			return nil, err
		}
		return &modelservicev1.ResolveModelRefResponse{Ref: ref}, nil
	}

	aliasMatches, err := s.queryResolveRefsByAlias(ctx, vendorID, modelIDOrAlias)
	if err != nil {
		return nil, err
	}
	ref, err := resolveUniqueModelRef(aliasMatches, vendorID, modelIDOrAlias, "alias")
	if err != nil {
		return nil, err
	}
	if ref == nil {
		return nil, domainerror.NewNotFound("platformk8s/models: model %q not found for vendor %q", modelIDOrAlias, vendorID)
	}
	return &modelservicev1.ResolveModelRefResponse{Ref: ref}, nil
}

func (s *PostgresModelRegistry) queryResolveRefsByModelID(ctx context.Context, vendorID string, modelID string) ([]*modelv1.ModelRef, error) {
	query := fmt.Sprintf(`select vendor_id, model_id from %s where namespace = $1 and model_id = $2`, modelRegistryEntriesTable)
	args := []any{s.namespace, strings.TrimSpace(modelID)}
	if strings.TrimSpace(vendorID) != "" {
		query += " and vendor_id = $3"
		args = append(args, strings.TrimSpace(vendorID))
	}
	query += " order by vendor_id, model_id"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: resolve model_id %q/%q: %w", vendorID, modelID, err)
	}
	defer rows.Close()
	return scanResolveModelRefs(rows)
}

func (s *PostgresModelRegistry) queryResolveRefsByAlias(ctx context.Context, vendorID string, alias string) ([]*modelv1.ModelRef, error) {
	query := fmt.Sprintf(`select distinct vendor_id, model_id from %s where namespace = $1 and alias_value = $2`, modelRegistryAliasesTable)
	args := []any{s.namespace, strings.TrimSpace(alias)}
	if strings.TrimSpace(vendorID) != "" {
		query += " and vendor_id = $3"
		args = append(args, strings.TrimSpace(vendorID))
	}
	query += " order by vendor_id, model_id"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: resolve alias %q/%q: %w", vendorID, alias, err)
	}
	defer rows.Close()
	return scanResolveModelRefs(rows)
}

func scanResolveModelRefs(rows pgx.Rows) ([]*modelv1.ModelRef, error) {
	refs := []*modelv1.ModelRef{}
	for rows.Next() {
		var vendorID string
		var modelID string
		if err := rows.Scan(&vendorID, &modelID); err != nil {
			return nil, fmt.Errorf("platformk8s/models: scan model registry ref: %w", err)
		}
		refs = appendUniqueModelRefs(refs, &modelv1.ModelRef{
			VendorId: vendorID,
			ModelId:  modelID,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return refs, nil
}

func appendUniqueModelRefs(items []*modelv1.ModelRef, ref *modelv1.ModelRef) []*modelv1.ModelRef {
	if ref == nil {
		return items
	}
	key := identityKey(ref.GetVendorId(), ref.GetModelId())
	for _, item := range items {
		if identityKey(item.GetVendorId(), item.GetModelId()) == key {
			return items
		}
	}
	return append(items, ref)
}

func resolveUniqueModelRef(matches []*modelv1.ModelRef, vendorID string, modelIDOrAlias string, field string) (*modelv1.ModelRef, error) {
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return matches[0], nil
	default:
		return nil, domainerror.NewValidation(
			"platformk8s/models: %s %q is ambiguous for vendor %q",
			field,
			modelIDOrAlias,
			vendorID,
		)
	}
}

func isDomainNotFound(err error) bool {
	var notFound *domainerror.NotFoundError
	return err != nil && errors.As(err, &notFound)
}
