package store

import (
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"context"
	"fmt"
	"strings"

	"code-code.internal/go-contract/domainerror"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"github.com/jackc/pgx/v5"
)

type postgresScanFunc func(...any) error

func (s *PostgresModelRegistry) scanRegistryEntry(ctx context.Context, scan postgresScanFunc) (*modelservicev1.ModelRegistryEntry, error) {
	identity, item, err := s.scanRegistryEntryBase(scan)
	if err != nil {
		return nil, err
	}
	sources, err := s.listEntrySources(ctx, identity.VendorID, identity.ModelID)
	if err != nil {
		return nil, err
	}
	item.Sources = sources
	return item, nil
}

func (s *PostgresModelRegistry) scanRegistryEntryBase(scan postgresScanFunc) (models.SurfaceIdentity, *modelservicev1.ModelRegistryEntry, error) {
	var vendorID string
	var modelID string
	var rawDefinition []byte
	var sourceRefVendorID *string // read for DB column compat; discarded
	var sourceRefModelID *string  // read for DB column compat; discarded
	var rawBadges []byte
	var rawPricing []byte
	if err := scan(&vendorID, &modelID, &rawDefinition, &sourceRefVendorID, &sourceRefModelID, &rawBadges, &rawPricing); err != nil {
		return models.SurfaceIdentity{}, nil, fmt.Errorf("platformk8s/models: scan model registry entry: %w", err)
	}
	identity, err := models.NewSurfaceIdentity(vendorID, modelID)
	if err != nil {
		return models.SurfaceIdentity{}, nil, err
	}
	definition, err := decodeModelDefinition(rawDefinition)
	if err != nil {
		return models.SurfaceIdentity{}, nil, err
	}
	badges, err := decodeStringSlice(rawBadges)
	if err != nil {
		return models.SurfaceIdentity{}, nil, err
	}
	pricing, err := decodePricing(rawPricing)
	if err != nil {
		return models.SurfaceIdentity{}, nil, err
	}
	return identity, &modelservicev1.ModelRegistryEntry{
		Definition: definition,
		Badges:     badges,
		Pricing:    pricing,
	}, nil
}

func (s *PostgresModelRegistry) getRegistryEntry(ctx context.Context, vendorID string, modelID string) (*modelservicev1.ModelRegistryEntry, error) {
	vendorID = strings.TrimSpace(vendorID)
	modelID = strings.TrimSpace(modelID)
	if vendorID == "" || modelID == "" {
		return nil, domainerror.NewValidation("platformk8s/models: vendor_id and model_id are required")
	}
	row := s.pool.QueryRow(ctx, fmt.Sprintf(`
select vendor_id, model_id, definition, source_ref_vendor_id, source_ref_model_id, badges, pricing
from %s
where namespace = $1 and vendor_id = $2 and model_id = $3`, modelRegistryEntriesTable), s.namespace, vendorID, modelID)
	item, err := s.scanRegistryEntry(ctx, row.Scan)
	if err == pgx.ErrNoRows {
		return nil, domainerror.NewNotFound("platformk8s/models: model %q/%q not found", vendorID, modelID)
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *PostgresModelRegistry) listEntrySources(ctx context.Context, vendorID string, modelID string) ([]*modelservicev1.RegistryModelSource, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
select source_id, is_direct, kind, source_model_id, definition, badges, pricing
from %s
where namespace = $1 and vendor_id = $2 and model_id = $3
order by is_direct desc, source_id, source_model_id`, modelRegistryObservationsTable), s.namespace, vendorID, modelID)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: list model registry observations %q/%q: %w", vendorID, modelID, err)
	}
	defer rows.Close()

	out := []*modelservicev1.RegistryModelSource{}
	for rows.Next() {
		var sourceID string
		var isDirect bool
		var kind string
		var sourceModelID *string
		var rawDefinition []byte
		var rawBadges []byte
		var rawPricing []byte
		if err := rows.Scan(&sourceID, &isDirect, &kind, &sourceModelID, &rawDefinition, &rawBadges, &rawPricing); err != nil {
			return nil, fmt.Errorf("platformk8s/models: scan model registry observation: %w", err)
		}
		definition, err := decodeModelDefinition(rawDefinition)
		if err != nil {
			return nil, err
		}
		badges, err := decodeStringSlice(rawBadges)
		if err != nil {
			return nil, err
		}
		pricing, err := decodePricing(rawPricing)
		if err != nil {
			return nil, err
		}
		modelID := ""
		if sourceModelID != nil {
			modelID = *sourceModelID
		}
		out = append(out, models.NormalizeRegistryModelSource(&modelservicev1.RegistryModelSource{
			SourceId:      sourceID,
			Kind:          registryModelSourceKindFromDBString(kind),
			IsDirect:      isDirect,
			SourceModelId: modelID,
			Definition:    definition,
			Badges:        badges,
			Pricing:       pricing,
		}))
	}
	return out, rows.Err()
}

func (s *PostgresModelRegistry) listEntrySourcesBatch(ctx context.Context, identities []models.SurfaceIdentity) (map[string][]*modelservicev1.RegistryModelSource, error) {
	out := map[string][]*modelservicev1.RegistryModelSource{}
	if len(identities) == 0 {
		return out, nil
	}

	seen := map[string]struct{}{}
	vendorIDs := make([]string, 0, len(identities))
	modelIDs := make([]string, 0, len(identities))
	for _, identity := range identities {
		key := identity.Key()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		vendorIDs = append(vendorIDs, identity.VendorID)
		modelIDs = append(modelIDs, identity.ModelID)
		out[key] = nil
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
with selected(vendor_id, model_id) as (
	select * from unnest($2::text[], $3::text[])
)
select observations.vendor_id, observations.model_id, observations.source_id, observations.is_direct,
       observations.kind, observations.source_model_id, observations.definition, observations.badges, observations.pricing
from %s observations
join selected
  on selected.vendor_id = observations.vendor_id
 and selected.model_id = observations.model_id
where observations.namespace = $1
order by observations.vendor_id, observations.model_id, observations.is_direct desc, observations.source_id, observations.source_model_id`,
		modelRegistryObservationsTable), s.namespace, vendorIDs, modelIDs)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: list model registry observations batch: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var vendorID string
		var modelID string
		var sourceID string
		var isDirect bool
		var kind string
		var sourceModelID string
		var rawDefinition []byte
		var rawBadges []byte
		var rawPricing []byte
		if err := rows.Scan(&vendorID, &modelID, &sourceID, &isDirect, &kind, &sourceModelID, &rawDefinition, &rawBadges, &rawPricing); err != nil {
			return nil, fmt.Errorf("platformk8s/models: scan model registry observation batch: %w", err)
		}
		definition, err := decodeModelDefinition(rawDefinition)
		if err != nil {
			return nil, err
		}
		badges, err := decodeStringSlice(rawBadges)
		if err != nil {
			return nil, err
		}
		pricing, err := decodePricing(rawPricing)
		if err != nil {
			return nil, err
		}
		key := identityKey(vendorID, modelID)
		out[key] = append(out[key], models.NormalizeRegistryModelSource(&modelservicev1.RegistryModelSource{
			SourceId:      sourceID,
			Kind:          registryModelSourceKindFromDBString(kind),
			IsDirect:      isDirect,
			SourceModelId: sourceModelID,
			Definition:    definition,
			Badges:        badges,
			Pricing:       pricing,
		}))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
