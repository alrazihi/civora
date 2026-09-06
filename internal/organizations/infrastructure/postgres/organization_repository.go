package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alrazihi/civora/internal/organizations/domain"
	"github.com/google/uuid"
)

type PostgresOrganizationRepository struct {
	db *sql.DB
}

func NewPostgresOrganizationRepository(db *sql.DB) *PostgresOrganizationRepository {
	return &PostgresOrganizationRepository{db: db}
}

func (r *PostgresOrganizationRepository) Save(ctx context.Context, org *domain.Organization) error {
	query := `
		INSERT INTO organizations (id, name, description, slug, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query,
		org.ID, org.Name, org.Description, org.Slug, org.CreatedAt, org.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert organization: %w", err)
	}
	return nil
}

func (r *PostgresOrganizationRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	query := `
		SELECT id, name, description, slug, created_at, updated_at
		FROM organizations
		WHERE id = $1
	`
	return r.scanOrganization(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresOrganizationRepository) FindBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	query := `
		SELECT id, name, description, slug, created_at, updated_at
		FROM organizations
		WHERE slug = $1
	`
	return r.scanOrganization(r.db.QueryRowContext(ctx, query, slug))
}

func (r *PostgresOrganizationRepository) scanOrganization(row interface {
	Scan(dest ...any) error
}) (*domain.Organization, error) {
	var org domain.Organization
	err := row.Scan(&org.ID, &org.Name, &org.Description, &org.Slug, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan organization: %w", err)
	}
	return &org, nil
}
