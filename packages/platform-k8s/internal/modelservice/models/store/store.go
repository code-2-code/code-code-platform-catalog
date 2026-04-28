package store

import (
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"context"
	"fmt"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	domaineventv1 "code-code.internal/go-contract/platform/domain_event/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"code-code.internal/platform-k8s/internal/platform/domainevents"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
)

const (
	modelRegistryEntriesTable      = "platform_model_registry_entries"
	modelRegistryObservationsTable = "platform_model_registry_observations"
	modelRegistryAliasesTable      = "platform_model_registry_aliases"
)


// PostgresModelRegistry is the unified read/write store for model registry
// entries. It replaces the former split between PostgresManagementService
// (read) and PostgresRegistryStore (write).
type PostgresModelRegistry struct {
	pool      *pgxpool.Pool
	outbox    *domainevents.Outbox
	namespace string
}

// NewPostgresModelRegistry creates a unified read/write model registry.
func NewPostgresModelRegistry(pool *pgxpool.Pool, outbox *domainevents.Outbox, namespace string) (*PostgresModelRegistry, error) {
	return newPostgresModelRegistry(pool, outbox, namespace)
}

func newPostgresModelRegistry(pool *pgxpool.Pool, outbox *domainevents.Outbox, namespace string) (*PostgresModelRegistry, error) {
	if pool == nil {
		return nil, fmt.Errorf("platformk8s/models: postgres pool is nil")
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, fmt.Errorf("platformk8s/models: namespace is empty")
	}
	return &PostgresModelRegistry{
		pool:      pool,
		outbox:    outbox,
		namespace: namespace,
	}, nil
}

// Namespace returns the configured namespace for this registry.
func (s *PostgresModelRegistry) Namespace() string {
	return s.namespace
}

func (s *PostgresModelRegistry) ListManagedDefinitions(ctx context.Context) ([]models.StoredDefinition, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
select vendor_id, model_id, definition
from %s
where namespace = $1
order by vendor_id, model_id`, modelRegistryEntriesTable), s.namespace)
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: list managed model registry entries: %w", err)
	}
	defer rows.Close()

	out := []models.StoredDefinition{}
	for rows.Next() {
		var vendorID string
		var modelID string
		var rawDefinition []byte
		if err := rows.Scan(&vendorID, &modelID, &rawDefinition); err != nil {
			return nil, fmt.Errorf("platformk8s/models: scan model registry entry: %w", err)
		}
		definition, err := decodeModelDefinition(rawDefinition)
		if err != nil {
			return nil, err
		}
		identity, err := models.NewSurfaceIdentity(vendorID, modelID)
		if err != nil {
			return nil, err
		}
		out = append(out, models.StoredDefinition{Identity: identity, Definition: definition})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresModelRegistry) UpsertManagedDefinition(ctx context.Context, entry *modelservicev1.ModelRegistryEntry) error {
	if entry == nil {
		return fmt.Errorf("platformk8s/models: model registry entry is nil")
	}
	identity, err := models.IdentityFromDefinition(entry.GetDefinition())
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	generation, mutation, err := s.nextGeneration(ctx, tx, identity)
	if err != nil {
		return err
	}
	definitionJSON, err := encodeModelDefinition(entry.GetDefinition())
	if err != nil {
		return err
	}
	badgesJSON, err := encodeStringSlice(entry.GetBadges())
	if err != nil {
		return err
	}
	pricingJSON, err := encodePricing(entry.GetPricing())
	if err != nil {
		return err
	}

	if mutation == "created" {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
insert into %s (
	namespace, vendor_id, model_id, definition, source_ref_vendor_id,
	source_ref_model_id, badges, pricing, generation, created_at, updated_at
) values ($1, $2, $3, $4::jsonb, null, null, $5::jsonb, $6::jsonb, $7, now(), now())`,
			modelRegistryEntriesTable),
			s.namespace, identity.VendorID, identity.ModelID, string(definitionJSON),
			string(badgesJSON), string(pricingJSON), generation,
		); err != nil {
			return fmt.Errorf("platformk8s/models: insert model registry entry %q/%q: %w", identity.VendorID, identity.ModelID, err)
		}
	} else {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
update %s
set definition = $4::jsonb,
    source_ref_vendor_id = null,
    source_ref_model_id = null,
    badges = $5::jsonb,
    pricing = $6::jsonb,
    generation = $7,
    updated_at = now()
where namespace = $1 and vendor_id = $2 and model_id = $3`,
			modelRegistryEntriesTable),
			s.namespace, identity.VendorID, identity.ModelID, string(definitionJSON),
			string(badgesJSON), string(pricingJSON), generation,
		); err != nil {
			return fmt.Errorf("platformk8s/models: update model registry entry %q/%q: %w", identity.VendorID, identity.ModelID, err)
		}
	}
	if err := s.replaceAliases(ctx, tx, identity, entry.GetDefinition()); err != nil {
		return err
	}
	if err := s.replaceObservations(ctx, tx, identity, entry.GetSources()); err != nil {
		return err
	}
	if err := s.enqueueModelDefinitionEvent(ctx, tx, entry.GetDefinition(), mutation, generation); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresModelRegistry) DeleteManagedDefinition(ctx context.Context, identity models.SurfaceIdentity) error {
	if strings.TrimSpace(identity.VendorID) == "" || strings.TrimSpace(identity.ModelID) == "" {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	definition, generation, err := s.getDefinitionForUpdate(ctx, tx, identity)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
delete from %s where namespace = $1 and vendor_id = $2 and model_id = $3`,
		modelRegistryObservationsTable), s.namespace, identity.VendorID, identity.ModelID); err != nil {
		return fmt.Errorf("platformk8s/models: delete model registry observations %q/%q: %w", identity.VendorID, identity.ModelID, err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
delete from %s where namespace = $1 and vendor_id = $2 and model_id = $3`,
		modelRegistryEntriesTable), s.namespace, identity.VendorID, identity.ModelID); err != nil {
		return fmt.Errorf("platformk8s/models: delete model registry entry %q/%q: %w", identity.VendorID, identity.ModelID, err)
	}
	if err := s.enqueueModelDefinitionEvent(ctx, tx, definition, "deleted", generation); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresModelRegistry) nextGeneration(ctx context.Context, tx pgx.Tx, identity models.SurfaceIdentity) (int64, string, error) {
	_, generation, err := s.getDefinitionForUpdate(ctx, tx, identity)
	if err == pgx.ErrNoRows {
		return 1, "created", nil
	}
	if err != nil {
		return 0, "", err
	}
	return generation + 1, "updated", nil
}

func (s *PostgresModelRegistry) getDefinitionForUpdate(ctx context.Context, tx pgx.Tx, identity models.SurfaceIdentity) (*modelv1.ModelVersion, int64, error) {
	var rawDefinition []byte
	var generation int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
select definition, generation
from %s
where namespace = $1 and vendor_id = $2 and model_id = $3
for update`, modelRegistryEntriesTable), s.namespace, identity.VendorID, identity.ModelID).Scan(&rawDefinition, &generation); err != nil {
		return nil, 0, err
	}
	definition, err := decodeModelDefinition(rawDefinition)
	if err != nil {
		return nil, 0, err
	}
	return definition, generation, nil
}

func (s *PostgresModelRegistry) replaceObservations(ctx context.Context, tx pgx.Tx, identity models.SurfaceIdentity, observations []*modelservicev1.RegistryModelSource) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
delete from %s where namespace = $1 and vendor_id = $2 and model_id = $3`,
		modelRegistryObservationsTable), s.namespace, identity.VendorID, identity.ModelID); err != nil {
		return fmt.Errorf("platformk8s/models: delete model registry observations %q/%q: %w", identity.VendorID, identity.ModelID, err)
	}
	for _, observation := range normalizeRegistryObservations(observations) {
		if observation.GetSourceId() == "" {
			continue
		}
		definitionJSON, err := encodeModelDefinition(observation.GetDefinition())
		if err != nil {
			return err
		}
		badgesJSON, err := encodeStringSlice(observation.GetBadges())
		if err != nil {
			return err
		}
		pricingJSON, err := encodePricing(observation.GetPricing())
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
insert into %s (
	namespace, vendor_id, model_id, source_id, is_direct, kind,
	source_model_id, definition, badges, pricing, created_at, updated_at
) values ($1, $2, $3, $4, $5, $6, nullif($7, ''), $8::jsonb, $9::jsonb, $10::jsonb, now(), now())`,
			modelRegistryObservationsTable),
			s.namespace, identity.VendorID, identity.ModelID, observation.GetSourceId(),
			observation.GetIsDirect(), registryModelSourceKindToDBString(observation.GetKind()), observation.GetSourceModelId(),
			string(definitionJSON), string(badgesJSON), string(pricingJSON),
		); err != nil {
			return fmt.Errorf("platformk8s/models: insert model registry observation %q/%q/%q: %w", identity.VendorID, identity.ModelID, observation.GetSourceId(), err)
		}
	}
	return nil
}

func (s *PostgresModelRegistry) replaceAliases(ctx context.Context, tx pgx.Tx, identity models.SurfaceIdentity, definition *modelv1.ModelVersion) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
delete from %s where namespace = $1 and vendor_id = $2 and model_id = $3`,
		modelRegistryAliasesTable), s.namespace, identity.VendorID, identity.ModelID); err != nil {
		return fmt.Errorf("platformk8s/models: delete model registry aliases %q/%q: %w", identity.VendorID, identity.ModelID, err)
	}
	for _, alias := range normalizeModelDefinitionAliases(definition) {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
insert into %s (
	namespace, vendor_id, model_id, alias_kind, alias_value, created_at, updated_at
) values ($1, $2, $3, $4, $5, now(), now())`,
			modelRegistryAliasesTable),
			s.namespace, identity.VendorID, identity.ModelID, alias.GetKind().String(), alias.GetValue(),
		); err != nil {
			return fmt.Errorf("platformk8s/models: insert model registry alias %q/%q/%q: %w", identity.VendorID, identity.ModelID, alias.GetValue(), err)
		}
	}
	return nil
}

func (s *PostgresModelRegistry) enqueueModelDefinitionEvent(ctx context.Context, tx pgx.Tx, definition *modelv1.ModelVersion, mutation string, generation int64) error {
	if s.outbox == nil {
		return nil
	}
	identity, err := models.IdentityFromDefinition(definition)
	if err != nil {
		return err
	}
	event := &domaineventv1.DomainEvent{
		EventType:        mutation,
		AggregateType:    "catalog",
		AggregateId:      identity.VendorID + "/" + identity.ModelID,
		AggregateVersion: generation,
		Payload: &domaineventv1.DomainEvent_Catalog{Catalog: &domaineventv1.CatalogEvent{
			Mutation:  modelDefinitionDomainMutation(mutation),
			Kind:      domaineventv1.CatalogKind_CATALOG_KIND_MODEL_DEFINITION,
			CatalogId: identity.VendorID + "/" + identity.ModelID,
			Definition: &domaineventv1.CatalogEvent_ModelVersion{
				ModelVersion: proto.Clone(definition).(*modelv1.ModelVersion),
			},
		}},
	}
	return s.outbox.EnqueueTx(ctx, tx, event)
}

func modelDefinitionDomainMutation(mutation string) domaineventv1.DomainMutation {
	switch strings.TrimSpace(mutation) {
	case "created":
		return domaineventv1.DomainMutation_DOMAIN_MUTATION_CREATED
	case "deleted":
		return domaineventv1.DomainMutation_DOMAIN_MUTATION_DELETED
	default:
		return domaineventv1.DomainMutation_DOMAIN_MUTATION_UPDATED
	}
}
