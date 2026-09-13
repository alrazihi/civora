package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/forms/domain"
	"github.com/google/uuid"
)

type PostgresFormVersionRepository struct {
	db *sql.DB
}

func NewPostgresFormVersionRepository(db *sql.DB) *PostgresFormVersionRepository {
	return &PostgresFormVersionRepository{db: db}
}

func (r *PostgresFormVersionRepository) Save(ctx context.Context, tx *sql.Tx, version *domain.FormVersion) error {
	return r.save(ctx, tx, version, false)
}

func (r *PostgresFormVersionRepository) SaveTx(ctx context.Context, tx *sql.Tx, version *domain.FormVersion) error {
	return r.save(ctx, tx, version, true)
}

func (r *PostgresFormVersionRepository) save(ctx context.Context, tx *sql.Tx, version *domain.FormVersion, inTx bool) error {
	query := `
		INSERT INTO form_versions (id, form_id, organization_id, version, status, created_by, created_at, published_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE
		SET status = EXCLUDED.status,
			published_at = EXCLUDED.published_at,
			updated_at = EXCLUDED.updated_at
	`
	publishedAt := time.Time{}
	if version.PublishedAt != nil {
		publishedAt = *version.PublishedAt
	}
	execer := r.getExecer(tx, inTx)
	_, err := execer.ExecContext(ctx, query,
		version.ID, version.FormID, version.OrganizationID, version.Version, version.Status,
		version.CreatedByID, version.CreatedAt, publishedAt, version.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save form version: %w", err)
	}
	return nil
}

func (r *PostgresFormVersionRepository) FindLatest(ctx context.Context, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	return r.FindLatestTx(ctx, nil, orgID, formID)
}

func (r *PostgresFormVersionRepository) FindLatestTx(ctx context.Context, tx *sql.Tx, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	query := `
		SELECT id, form_id, organization_id, version, status, created_by, created_at, published_at, updated_at
		FROM form_versions
		WHERE form_id = $1 AND organization_id = $2
		ORDER BY version DESC
		LIMIT 1
	`
	row := r.queryRow(ctx, tx, query, formID, orgID)
	return r.scanVersion(row)
}

func (r *PostgresFormVersionRepository) FindActiveVersion(ctx context.Context, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	return r.FindActiveVersionTx(ctx, nil, orgID, formID)
}

func (r *PostgresFormVersionRepository) FindActiveVersionTx(ctx context.Context, tx *sql.Tx, orgID, formID uuid.UUID) (*domain.FormVersion, error) {
	query := `
		SELECT id, form_id, organization_id, version, status, created_by, created_at, published_at, updated_at
		FROM form_versions
		WHERE form_id = $1 AND organization_id = $2 AND status = 'PUBLISHED'
		ORDER BY version DESC
		LIMIT 1
	`
	row := r.queryRow(ctx, tx, query, formID, orgID)
	return r.scanVersion(row)
}

func (r *PostgresFormVersionRepository) FindByID(ctx context.Context, orgID, versionID uuid.UUID) (*domain.FormVersion, error) {
	return r.FindByIDTx(ctx, nil, orgID, versionID)
}

func (r *PostgresFormVersionRepository) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID) (*domain.FormVersion, error) {
	query := `
		SELECT id, form_id, organization_id, version, status, created_by, created_at, published_at, updated_at
		FROM form_versions
		WHERE id = $1 AND organization_id = $2
	`
	row := r.queryRow(ctx, tx, query, versionID, orgID)
	return r.scanVersion(row)
}

func (r *PostgresFormVersionRepository) FindByIDForUpdateTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID) (*domain.FormVersion, error) {
	query := `
		SELECT id, form_id, organization_id, version, status, created_by, created_at, published_at, updated_at
		FROM form_versions
		WHERE id = $1 AND organization_id = $2
		FOR UPDATE
	`
	row := r.queryRow(ctx, tx, query, versionID, orgID)
	return r.scanVersion(row)
}

func (r *PostgresFormVersionRepository) FindByIDs(ctx context.Context, orgID uuid.UUID, versionIDs []uuid.UUID) ([]*domain.FormVersion, error) {
	if len(versionIDs) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, form_id, organization_id, version, status, created_by, created_at, published_at, updated_at
		FROM form_versions
		WHERE organization_id = $1 AND id = ANY($2::uuid[])
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, versionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch-fetch form versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var versions []*domain.FormVersion
	for rows.Next() {
		v, err := r.scanVersionFromRows(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return versions, nil
}

func (r *PostgresFormVersionRepository) ListByFormID(ctx context.Context, orgID, formID uuid.UUID) ([]*domain.FormVersion, error) {
	query := `
		SELECT id, form_id, organization_id, version, status, created_by, created_at, published_at, updated_at
		FROM form_versions
		WHERE form_id = $1 AND organization_id = $2
		ORDER BY version ASC
	`
	rows, err := r.db.QueryContext(ctx, query, formID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var versions []*domain.FormVersion
	for rows.Next() {
		v, err := r.scanVersionFromRows(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return versions, nil
}

func (r *PostgresFormVersionRepository) queryRow(ctx context.Context, tx *sql.Tx, query string, args ...any) interface{ Scan(dest ...any) error } {
	if tx != nil {
		return tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *PostgresFormVersionRepository) getExecer(tx *sql.Tx, inTx bool) interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
} {
	if inTx {
		return tx
	}
	return r.db
}

func (r *PostgresFormVersionRepository) scanVersion(row interface{ Scan(dest ...any) error }) (*domain.FormVersion, error) {
	var v domain.FormVersion
	var status string
	var publishedAt sql.NullTime
	if err := row.Scan(
		&v.ID, &v.FormID, &v.OrganizationID, &v.Version, &status,
		&v.CreatedByID, &v.CreatedAt, &publishedAt, &v.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrFormVersionNotFound
		}
		return nil, fmt.Errorf("failed to scan version: %w", err)
	}
	v.Status = domain.FormVersionStatus(status)
	if publishedAt.Valid {
		v.PublishedAt = &publishedAt.Time
	}
	return &v, nil
}

func (r *PostgresFormVersionRepository) scanVersionFromRows(rows *sql.Rows) (*domain.FormVersion, error) {
	var v domain.FormVersion
	var status string
	var publishedAt sql.NullTime
	if err := rows.Scan(
		&v.ID, &v.FormID, &v.OrganizationID, &v.Version, &status,
		&v.CreatedByID, &v.CreatedAt, &publishedAt, &v.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrFormVersionNotFound
		}
		return nil, fmt.Errorf("failed to scan version: %w", err)
	}
	v.Status = domain.FormVersionStatus(status)
	if publishedAt.Valid {
		v.PublishedAt = &publishedAt.Time
	}
	return &v, nil
}
