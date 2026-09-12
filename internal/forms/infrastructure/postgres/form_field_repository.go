package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/alrazihi/civora/internal/forms/domain"
	"github.com/google/uuid"
)

type PostgresFormFieldRepository struct {
	db *sql.DB
}

func NewPostgresFormFieldRepository(db *sql.DB) *PostgresFormFieldRepository {
	return &PostgresFormFieldRepository{db: db}
}

func (r *PostgresFormFieldRepository) Save(ctx context.Context, tx *sql.Tx, field *domain.FormField) error {
	return r.save(ctx, tx, field, false)
}

func (r *PostgresFormFieldRepository) SaveTx(ctx context.Context, tx *sql.Tx, field *domain.FormField) error {
	return r.save(ctx, tx, field, true)
}

func (r *PostgresFormFieldRepository) save(ctx context.Context, tx *sql.Tx, field *domain.FormField, inTx bool) error {
	optionsJSON, err := json.Marshal(field.Options)
	if err != nil {
		return fmt.Errorf("failed to marshal options: %w", err)
	}

	validationJSON, err := json.Marshal(field.Validation)
	if err != nil {
		return fmt.Errorf("failed to marshal validation: %w", err)
	}

	defaultValue := ""
	if field.DefaultValue != nil {
		defaultValue = *field.DefaultValue
	}

	query := `
		INSERT INTO form_fields (id, form_id, form_version_id, organization_id, key, label, type, required, description, placeholder, default_value, validation, options_json, "order", created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	execer := r.getExecer(tx, inTx)
	_, err = execer.ExecContext(ctx, query,
		field.ID, field.FormID, field.FormVersionID, field.OrganizationID,
		field.Key, field.Label, field.Type, field.Required, field.Description,
		field.Placeholder, defaultValue, validationJSON, optionsJSON, field.Order,
		field.CreatedAt, field.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save field: %w", err)
	}
	return nil
}

func (r *PostgresFormFieldRepository) FindByVersion(ctx context.Context, orgID, versionID uuid.UUID) ([]*domain.FormField, error) {
	return r.FindByVersionTx(ctx, nil, orgID, versionID)
}

func (r *PostgresFormFieldRepository) FindByVersionTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID) ([]*domain.FormField, error) {
	query := `
		SELECT id, form_id, form_version_id, organization_id, key, label, type, required, description, placeholder, default_value, validation, options_json, "order", created_at, updated_at
		FROM form_fields
		WHERE form_version_id = $1 AND organization_id = $2
		ORDER BY "order" ASC, id ASC
	`
	rows, err := r.query(ctx, tx, query, versionID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query fields: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var fields []*domain.FormField
	for rows.Next() {
		f, err := r.scanFieldFromRows(rows)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return fields, nil
}

func (r *PostgresFormFieldRepository) FindByVersionForUpdateTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID) ([]*domain.FormField, error) {
	query := `
		SELECT id, form_id, form_version_id, organization_id, key, label, type, required, description, placeholder, default_value, validation, options_json, "order", created_at, updated_at
		FROM form_fields
		WHERE form_version_id = $1 AND organization_id = $2
		FOR UPDATE
		ORDER BY "order" ASC, id ASC
	`
	rows, err := r.query(ctx, tx, query, versionID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query fields for update: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var fields []*domain.FormField
	for rows.Next() {
		f, err := r.scanFieldFromRows(rows)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return fields, nil
}

func (r *PostgresFormFieldRepository) FindFieldsByVersionIDs(ctx context.Context, orgID uuid.UUID, versionIDs []uuid.UUID) ([]*domain.FormField, error) {
	if len(versionIDs) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, form_id, form_version_id, organization_id, key, label, type, required, description, placeholder, default_value, validation, options_json, "order", created_at, updated_at
		FROM form_fields
		WHERE organization_id = $1 AND form_version_id = ANY($2::uuid[])
		ORDER BY "order" ASC, id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, versionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch-fetch form fields: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var fields []*domain.FormField
	for rows.Next() {
		f, err := r.scanFieldFromRows(rows)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return fields, nil
}

func (r *PostgresFormFieldRepository) FindByID(ctx context.Context, orgID, fieldID uuid.UUID) (*domain.FormField, error) {
	return r.FindByIDTx(ctx, nil, orgID, fieldID)
}

func (r *PostgresFormFieldRepository) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, fieldID uuid.UUID) (*domain.FormField, error) {
	query := `
		SELECT id, form_id, form_version_id, organization_id, key, label, type, required, description, placeholder, default_value, validation, options_json, "order", created_at, updated_at
		FROM form_fields
		WHERE id = $1 AND organization_id = $2
	`
	row := r.queryRow(ctx, tx, query, fieldID, orgID)
	return r.scanField(row)
}

func (r *PostgresFormFieldRepository) UpdateTx(ctx context.Context, tx *sql.Tx, field *domain.FormField) error {
	optionsJSON, err := json.Marshal(field.Options)
	if err != nil {
		return fmt.Errorf("failed to marshal options: %w", err)
	}
	validationJSON, err := json.Marshal(field.Validation)
	if err != nil {
		return fmt.Errorf("failed to marshal validation: %w", err)
	}

	defaultValue := ""
	if field.DefaultValue != nil {
		defaultValue = *field.DefaultValue
	}

	query := `
		UPDATE form_fields
		SET label = $1, type = $2, required = $3, description = $4, placeholder = $5, default_value = $6, validation = $7, options_json = $8, "order" = $9, updated_at = now()
		WHERE id = $10 AND form_version_id = $11
	`
	result, err := tx.ExecContext(ctx, query,
		field.Label, field.Type, field.Required, field.Description,
		field.Placeholder, defaultValue, validationJSON, optionsJSON, field.Order,
		field.ID, field.FormVersionID,
	)
	if err != nil {
		return fmt.Errorf("failed to update field: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrFormFieldNotFound
	}
	return nil
}

func (r *PostgresFormFieldRepository) DeleteTx(ctx context.Context, tx *sql.Tx, orgID, fieldID uuid.UUID) error {
	query := `DELETE FROM form_fields WHERE id = $1 AND organization_id = $2`
	result, err := tx.ExecContext(ctx, query, fieldID, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete field: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrFormFieldNotFound
	}
	return nil
}

func (r *PostgresFormFieldRepository) UpdateOrderTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID, fields []*domain.FormField) error {
	for _, f := range fields {
		_, err := tx.ExecContext(ctx, `UPDATE form_fields SET "order" = $1, updated_at = now() WHERE id = $2 AND organization_id = $3`, f.Order, f.ID, orgID)
		if err != nil {
			return fmt.Errorf("failed to update field order: %w", err)
		}
	}
	return nil
}

func (r *PostgresFormFieldRepository) getExecer(tx *sql.Tx, inTx bool) interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
} {
	if inTx {
		return tx
	}
	return r.db
}

func (r *PostgresFormFieldRepository) query(ctx context.Context, tx *sql.Tx, query string, args ...any) (*sql.Rows, error) {
	if tx != nil {
		return tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *PostgresFormFieldRepository) queryRow(ctx context.Context, tx *sql.Tx, query string, args ...any) interface{ Scan(dest ...any) error } {
	if tx != nil {
		return tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *PostgresFormFieldRepository) scanField(row interface{ Scan(dest ...any) error }) (*domain.FormField, error) {
	var f domain.FormField
	var defaultValue sql.NullString
	var validationJSON []byte
	var optionsJSON []byte
	if err := row.Scan(
		&f.ID, &f.FormID, &f.FormVersionID, &f.OrganizationID,
		&f.Key, &f.Label, &f.Type, &f.Required, &f.Description,
		&f.Placeholder, &defaultValue, &validationJSON, &optionsJSON,
		&f.Order, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan field: %w", err)
	}
	if defaultValue.Valid {
		f.DefaultValue = &defaultValue.String
	}
	if len(validationJSON) > 0 {
		_ = json.Unmarshal(validationJSON, &f.Validation)
	}
	if f.Validation == nil {
		f.Validation = map[string]any{}
	}
	if len(optionsJSON) > 0 {
		_ = json.Unmarshal(optionsJSON, &f.Options)
	}
	if f.Options == nil {
		f.Options = []domain.FormOption{}
	}
	return &f, nil
}

func (r *PostgresFormFieldRepository) scanFieldFromRows(rows *sql.Rows) (*domain.FormField, error) {
	var f domain.FormField
	var defaultValue sql.NullString
	var validationJSON []byte
	var optionsJSON []byte
	if err := rows.Scan(
		&f.ID, &f.FormID, &f.FormVersionID, &f.OrganizationID,
		&f.Key, &f.Label, &f.Type, &f.Required, &f.Description,
		&f.Placeholder, &defaultValue, &validationJSON, &optionsJSON,
		&f.Order, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan field: %w", err)
	}
	if defaultValue.Valid {
		f.DefaultValue = &defaultValue.String
	}
	if len(validationJSON) > 0 {
		_ = json.Unmarshal(validationJSON, &f.Validation)
	}
	if f.Validation == nil {
		f.Validation = map[string]any{}
	}
	if len(optionsJSON) > 0 {
		_ = json.Unmarshal(optionsJSON, &f.Options)
	}
	if f.Options == nil {
		f.Options = []domain.FormOption{}
	}
	return &f, nil
}
