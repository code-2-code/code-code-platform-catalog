package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"code-code.internal/go-contract/domainerror"
	modelservicev1 "code-code.internal/go-contract/platform/model/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const organizationsTable = "platform_organizations"

// PostgresOrganizationStore reads and writes model vendor organizations.
type PostgresOrganizationStore struct {
	pool *pgxpool.Pool
}

// NewPostgresOrganizationStore creates a DB-backed organization store.
func NewPostgresOrganizationStore(pool *pgxpool.Pool) (*PostgresOrganizationStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("platformk8s/models: postgres pool is nil")
	}
	return &PostgresOrganizationStore{pool: pool}, nil
}

// Get returns an organization by its org_code.
func (s *PostgresOrganizationStore) Get(ctx context.Context, orgCode string) (*modelservicev1.Organization, error) {
	orgCode = strings.TrimSpace(orgCode)
	if orgCode == "" {
		return nil, domainerror.NewValidation("platformk8s/models: org_code is required")
	}
	var rawRoles []byte
	var org modelservicev1.Organization
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
select org_code, org_name, org_roles, country, website, status
from %s
where org_code = $1`, organizationsTable), orgCode,
	).Scan(
		&org.OrgCode,
		&org.OrgName,
		&rawRoles,
		&org.Country,
		&org.Website,
		&org.Status,
	)
	if err == pgx.ErrNoRows {
		return nil, domainerror.NewNotFound("platformk8s/models: organization %q not found", orgCode)
	}
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: get organization %q: %w", orgCode, err)
	}
	if len(rawRoles) > 0 && string(rawRoles) != "null" {
		_ = json.Unmarshal(rawRoles, &org.OrgRoles)
	}
	return &org, nil
}

// Upsert creates or updates an organization.
func (s *PostgresOrganizationStore) Upsert(ctx context.Context, org *modelservicev1.Organization) error {
	if org == nil {
		return fmt.Errorf("platformk8s/models: organization is nil")
	}
	orgCode := strings.TrimSpace(org.GetOrgCode())
	if orgCode == "" {
		return domainerror.NewValidation("platformk8s/models: org_code is required for upsert")
	}
	rolesJSON, err := json.Marshal(org.GetOrgRoles())
	if err != nil {
		return fmt.Errorf("platformk8s/models: encode org_roles: %w", err)
	}
	_, err = s.pool.Exec(ctx, fmt.Sprintf(`
insert into %s (
	org_code, org_name, org_roles, country, website, status, created_at, updated_at
) values ($1, $2, $3::jsonb, $4, $5, $6, now(), now())
on conflict (org_code) do update set
	org_name = excluded.org_name,
	org_roles = excluded.org_roles,
	country = excluded.country,
	website = excluded.website,
	status = excluded.status,
	updated_at = now()`, organizationsTable),
		orgCode,
		strings.TrimSpace(org.GetOrgName()),
		string(rolesJSON),
		strings.TrimSpace(org.GetCountry()),
		strings.TrimSpace(org.GetWebsite()),
		strings.TrimSpace(org.GetStatus()),
	)
	if err != nil {
		return fmt.Errorf("platformk8s/models: upsert organization %q: %w", orgCode, err)
	}
	return nil
}

// List returns all organizations ordered by org_code.
func (s *PostgresOrganizationStore) List(ctx context.Context) ([]*modelservicev1.Organization, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
select org_code, org_name, org_roles, country, website, status
from %s
order by org_code`, organizationsTable))
	if err != nil {
		return nil, fmt.Errorf("platformk8s/models: list organizations: %w", err)
	}
	defer rows.Close()

	var out []*modelservicev1.Organization
	for rows.Next() {
		var org modelservicev1.Organization
		var rawRoles []byte
		if err := rows.Scan(&org.OrgCode, &org.OrgName, &rawRoles, &org.Country, &org.Website, &org.Status); err != nil {
			return nil, fmt.Errorf("platformk8s/models: scan organization: %w", err)
		}
		if len(rawRoles) > 0 && string(rawRoles) != "null" {
			_ = json.Unmarshal(rawRoles, &org.OrgRoles)
		}
		out = append(out, &org)
	}
	return out, rows.Err()
}
