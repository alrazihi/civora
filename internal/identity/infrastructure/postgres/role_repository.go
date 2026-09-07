package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/google/uuid"
)

type PostgresRoleRepository struct {
	db *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewPostgresRoleRepository(db *sql.DB) *PostgresRoleRepository {
	return &PostgresRoleRepository{db: db}
}

func (r *PostgresRoleRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresRoleRepository) Save(ctx context.Context, role *domain.Role) error {
	return r.saveRole(ctx, r.db, role)
}

func (r *PostgresRoleRepository) SaveTx(ctx context.Context, tx *sql.Tx, role *domain.Role) error {
	return r.saveRole(ctx, tx, role)
}

func (r *PostgresRoleRepository) saveRole(ctx context.Context, e sqlExecer, role *domain.Role) error {
	query := `
		INSERT INTO roles (id, organization_id, name, description, permissions, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := e.ExecContext(
		ctx, query,
		role.ID, role.OrganizationID, role.Name, role.Description,
		role.Permissions, role.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert role: %w", err)
	}
	return nil
}

func (r *PostgresRoleRepository) FindByID(ctx context.Context, orgID, roleID uuid.UUID) (*domain.Role, error) {
	query := `
		SELECT id, organization_id, name, description, permissions, created_at
		FROM roles
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanRole(r.db.QueryRowContext(ctx, query, orgID, roleID))
}

func (r *PostgresRoleRepository) FindByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.Role, error) {
	query := `
		SELECT id, organization_id, name, description, permissions, created_at
		FROM roles
		WHERE organization_id = $1 AND name = $2
	`
	return r.scanRole(r.db.QueryRowContext(ctx, query, orgID, name))
}

func (r *PostgresRoleRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.Role, error) {
	query := `
		SELECT id, organization_id, name, description, permissions, created_at
		FROM roles
		WHERE organization_id = $1
		ORDER BY name
	`
	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query roles: %w", err)
	}
	defer rows.Close()

	var roles []*domain.Role
	for rows.Next() {
		role, err := r.scanRoleFromRows(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func (r *PostgresRoleRepository) scanRole(row interface {
	Scan(dest ...any) error
}) (*domain.Role, error) {
	var r2 domain.Role
	var permsJSON []byte

	if err := row.Scan(
		&r2.ID, &r2.OrganizationID, &r2.Name, &r2.Description,
		&permsJSON, &r2.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan role: %w", err)
	}

	if len(permsJSON) > 0 {
		_ = json.Unmarshal(permsJSON, &r2.Permissions)
	}

	return &r2, nil
}

func (r *PostgresRoleRepository) scanRoleFromRows(rows *sql.Rows) (*domain.Role, error) {
	var r2 domain.Role
	var permsJSON []byte

	if err := rows.Scan(
		&r2.ID, &r2.OrganizationID, &r2.Name, &r2.Description,
		&permsJSON, &r2.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan role: %w", err)
	}

	if len(permsJSON) > 0 {
		_ = json.Unmarshal(permsJSON, &r2.Permissions)
	}

	return &r2, nil
}
