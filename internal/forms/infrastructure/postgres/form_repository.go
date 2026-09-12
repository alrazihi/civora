package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alrazihi/civora/internal/forms/domain"
	"github.com/google/uuid"
)

type PostgresFormRepository struct {
	db *sql.DB
}

func NewPostgresFormRepository(db *sql.DB) *PostgresFormRepository {
	return &PostgresFormRepository{db: db}
}

func (r *PostgresFormRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresFormRepository) SaveTx(ctx context.Context, tx *sql.Tx, form *domain.Form) error {
	query := `
		INSERT INTO forms (id, organization_id, key, name, description, status, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := tx.ExecContext(ctx, query,
		form.ID, form.OrganizationID, form.Key, form.Name, form.Description,
		form.Status, form.CreatedByID, form.CreatedAt, form.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save form: %w", err)
	}
	return nil
}

func (r *PostgresFormRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Form, error) {
	return r.FindByIDTx(ctx, nil, orgID, id)
}

func (r *PostgresFormRepository) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*domain.Form, error) {
	query := `
		SELECT id, organization_id, key, name, description, status, created_by, created_at, updated_at
		FROM forms
		WHERE organization_id = $1 AND id = $2
	`
	row := r.queryRow(ctx, tx, query, orgID, id)
	return r.scanForm(row)
}

func (r *PostgresFormRepository) FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*domain.Form, error) {
	return r.FindByKeyTx(ctx, nil, orgID, key)
}

func (r *PostgresFormRepository) FindByKeyTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, key string) (*domain.Form, error) {
	query := `
		SELECT id, organization_id, key, name, description, status, created_by, created_at, updated_at
		FROM forms
		WHERE organization_id = $1 AND key = $2
	`
	row := r.queryRow(ctx, tx, query, orgID, key)
	return r.scanForm(row)
}

func (r *PostgresFormRepository) List(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Form, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM forms WHERE organization_id = $1", orgID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count forms: %w", err)
	}

	query := `
		SELECT id, organization_id, key, name, description, status, created_by, created_at, updated_at
		FROM forms
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list forms: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var forms []*domain.Form
	for rows.Next() {
		f, err := r.scanFormFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		forms = append(forms, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	return forms, total, nil
}

func (r *PostgresFormRepository) Update(ctx context.Context, orgID, id uuid.UUID, form *domain.Form) error {
	return r.UpdateTx(ctx, nil, orgID, id, form)
}

func (r *PostgresFormRepository) UpdateTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, form *domain.Form) error {
	query := `
		UPDATE forms
		SET name = $1, description = $2, status = $3, updated_at = now()
		WHERE organization_id = $4 AND id = $5
	`
	result, err := r.exec(ctx, tx, query, form.Name, form.Description, form.Status, orgID, id)
	if err != nil {
		return err
	}
	if result == 0 {
		return domain.ErrFormNotFound
	}
	return nil
}

func (r *PostgresFormRepository) queryRow(ctx context.Context, tx *sql.Tx, query string, args ...any) interface{ Scan(dest ...any) error } {
	if tx != nil {
		return tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *PostgresFormRepository) exec(ctx context.Context, tx *sql.Tx, query string, args ...any) (int64, error) {
	var result sql.Result
	var err error
	if tx != nil {
		result, err = tx.ExecContext(ctx, query, args...)
	} else {
		result, err = r.db.ExecContext(ctx, query, args...)
	}
	if err != nil {
		return 0, fmt.Errorf("query execution error: %w", err)
	}
	return result.RowsAffected()
}

func (r *PostgresFormRepository) scanForm(row interface{ Scan(dest ...any) error }) (*domain.Form, error) {
	var f domain.Form
	var status string
	if err := row.Scan(
		&f.ID, &f.OrganizationID, &f.Key, &f.Name, &f.Description,
		&status, &f.CreatedByID, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan form: %w", err)
	}
	f.Status = domain.FormStatus(status)
	return &f, nil
}

func (r *PostgresFormRepository) scanFormFromRows(rows *sql.Rows) (*domain.Form, error) {
	var f domain.Form
	var status string
	if err := rows.Scan(
		&f.ID, &f.OrganizationID, &f.Key, &f.Name, &f.Description,
		&status, &f.CreatedByID, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan form: %w", err)
	}
	f.Status = domain.FormStatus(status)
	return &f, nil
}
