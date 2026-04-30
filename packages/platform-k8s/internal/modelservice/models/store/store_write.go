package store

import (
	"context"
	"fmt"
	"strings"

	modelv1 "code-code.internal/go-contract/model/v1"
	domaineventv1 "code-code.internal/go-contract/platform/domain_event/v1"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	models "code-code.internal/platform-k8s/internal/modelservice/models"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

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
