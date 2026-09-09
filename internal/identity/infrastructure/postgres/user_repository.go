package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/google/uuid"
)

type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresUserRepository) Save(ctx context.Context, user *domain.User) error {
	return r.saveUser(ctx, r.db, user)
}

func (r *PostgresUserRepository) SaveTx(ctx context.Context, tx *sql.Tx, user *domain.User) error {
	return r.saveUser(ctx, tx, user)
}

func (r *PostgresUserRepository) saveUser(ctx context.Context, e sqlExecer, user *domain.User) error {
	query := `
		INSERT INTO users (id, organization_id, email, name, role_id, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := e.ExecContext(
		ctx, query,
		user.ID, user.OrganizationID, user.Email, user.Name,
		user.RoleID, user.PasswordHash,
		user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}
	return nil
}

func (r *PostgresUserRepository) FindByEmail(ctx context.Context, orgID uuid.UUID, email string) (*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, role_id, password_hash, created_at, updated_at
		FROM users
		WHERE organization_id = $1 AND email = $2
	`
	return r.scanUser(r.db.QueryRowContext(ctx, query, orgID, email))
}

func (r *PostgresUserRepository) FindByID(ctx context.Context, orgID, userID uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, role_id, password_hash, created_at, updated_at
		FROM users
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanUser(r.db.QueryRowContext(ctx, query, orgID, userID))
}

func (r *PostgresUserRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.User, error) {
	query := `
		SELECT id, organization_id, email, name, role_id, password_hash, created_at, updated_at
		FROM users
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*domain.User
	for rows.Next() {
		u, err := r.scanUserFromRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return users, nil
}

func (r *PostgresUserRepository) scanUser(row interface {
	Scan(dest ...any) error
}) (*domain.User, error) {
	var u domain.User
	if err := row.Scan(
		&u.ID, &u.OrganizationID, &u.Email, &u.Name,
		&u.RoleID, &u.PasswordHash,
		&u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}
	return &u, nil
}

func (r *PostgresUserRepository) scanUserFromRows(rows *sql.Rows) (*domain.User, error) {
	var u domain.User
	if err := rows.Scan(
		&u.ID, &u.OrganizationID, &u.Email, &u.Name,
		&u.RoleID, &u.PasswordHash,
		&u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}
	return &u, nil
}

func (r *PostgresUserRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	return r.countByOrganization(ctx, r.db, orgID)
}

func (r *PostgresUserRepository) CountByOrganizationTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) (int, error) {
	return r.countByOrganization(ctx, tx, orgID)
}

func (r *PostgresUserRepository) countByOrganization(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, orgID uuid.UUID) (int, error) {
	var count int
	err := q.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE organization_id = $1", orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

func (r *PostgresUserRepository) ScanRolePermissions(ctx context.Context, roleID uuid.UUID) ([]string, error) {
	if roleID == uuid.Nil {
		return nil, nil
	}
	var permsJSON []byte
	err := r.db.QueryRowContext(ctx,
		"SELECT permissions FROM roles WHERE id = $1", roleID).Scan(&permsJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query role permissions: %w", err)
	}

	var perms []string
	if len(permsJSON) > 0 {
		if err := json.Unmarshal(permsJSON, &perms); err != nil {
			return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
		}
	}
	return perms, nil
}
