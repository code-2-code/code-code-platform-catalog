package models

import (
	"context"
	"fmt"
	"strings"

	"code-code.internal/go-contract/domainerror"
	modelv1 "code-code.internal/go-contract/model/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const modelCardsTable = "platform_model_cards"

// PostgresModelCardStore reads and writes model cards for registry entries.
type PostgresModelCardStore struct {
	pool      *pgxpool.Pool
	namespace string
}

// NewPostgresModelCardStore creates a DB-backed model card store.
func NewPostgresModelCardStore(pool *pgxpool.Pool, namespace string) (*PostgresModelCardStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("platformk8s/models: postgres pool is nil")
	}
	if strings.TrimSpace(namespace) == "" {
		return nil, fmt.Errorf("platformk8s/models: namespace is empty")
	}
	return &PostgresModelCardStore{pool: pool, namespace: strings.TrimSpace(namespace)}, nil
}

// Get returns the model card for a given model ref.
func (s *PostgresModelCardStore) Get(ctx context.Context, ref *modelv1.ModelRef) (*modelservicev1.ModelCard, error) {
	vendorID := strings.TrimSpace(ref.GetVendorId())
	modelID := strings.TrimSpace(ref.GetModelId())
	if vendorID == "" || modelID == "" {
		return nil, domainerror.NewValidation("platformk8s/models: model ref is required for model card")
	}
	var card modelservicev1.ModelCard
	var metadataJSON []byte
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
select schema_version, metadata_json, markdown_body, source_type, source_url, review_status, reviewer
from %s
where namespace = $1 and vendor_id = $2 and model_id = $3`, modelCardsTable),
		s.namespace, vendorID, modelID,
	).Scan(
		&card.SchemaVersion,
		&metadataJSON,
		&card.MarkdownBody,
		&card.SourceType,
		&card.SourceUrl,
		&card.ReviewStatus,
		&card.Reviewer,
	)
	if err == pgx.ErrNoRows {
		return nil, domainerror.NewNotFound("platformk8s/models: model card %q/%q not found", vendorID, modelID)
	}
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: get model card %q/%q: %w", vendorID, modelID, err)
	}
	if len(metadataJSON) > 0 && string(metadataJSON) != "null" {
		card.MetadataJson = string(metadataJSON)
	}
	return &card, nil
}

// Upsert creates or updates a model card for a specific model ref.
func (s *PostgresModelCardStore) Upsert(ctx context.Context, ref *modelv1.ModelRef, card *modelservicev1.ModelCard) error {
	if card == nil {
		return fmt.Errorf("platformk8s/models: model card is nil")
	}
	vendorID := strings.TrimSpace(ref.GetVendorId())
	modelID := strings.TrimSpace(ref.GetModelId())
	if vendorID == "" || modelID == "" {
		return domainerror.NewValidation("platformk8s/models: model ref is required for model card upsert")
	}
	var metadataJSON *string
	if strings.TrimSpace(card.GetMetadataJson()) != "" {
		v := strings.TrimSpace(card.GetMetadataJson())
		metadataJSON = &v
	}
	_, err := s.pool.Exec(ctx, fmt.Sprintf(`
insert into %s (
	namespace, vendor_id, model_id, schema_version, metadata_json, markdown_body,
	source_type, source_url, review_status, reviewer, created_at, updated_at
) values ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $9, $10, now(), now())
on conflict (namespace, vendor_id, model_id) do update set
	schema_version = excluded.schema_version,
	metadata_json = excluded.metadata_json,
	markdown_body = excluded.markdown_body,
	source_type = excluded.source_type,
	source_url = excluded.source_url,
	review_status = excluded.review_status,
	reviewer = excluded.reviewer,
	updated_at = now()`, modelCardsTable),
		s.namespace, vendorID, modelID,
		normalizeModelCardSchemaVersion(card.GetSchemaVersion()),
		metadataJSON,
		strings.TrimSpace(card.GetMarkdownBody()),
		strings.TrimSpace(card.GetSourceType()),
		strings.TrimSpace(card.GetSourceUrl()),
		normalizeModelCardReviewStatus(card.GetReviewStatus()),
		strings.TrimSpace(card.GetReviewer()),
	)
	if err != nil {
		return fmt.Errorf("platformk8s/models: upsert model card %q/%q: %w", vendorID, modelID, err)
	}
	return nil
}

func normalizeModelCardSchemaVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "v1"
	}
	return v
}

func normalizeModelCardReviewStatus(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unreviewed"
	}
	return v
}
