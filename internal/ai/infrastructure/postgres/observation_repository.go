package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/ai/domain"
	"github.com/google/uuid"
)

type PostgresObservationRepository struct {
	db *sql.DB
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func NewPostgresObservationRepository(db *sql.DB) *PostgresObservationRepository {
	return &PostgresObservationRepository{db: db}
}

func (r *PostgresObservationRepository) Save(ctx context.Context, o *domain.Observation) error {
	return r.save(ctx, r.db, o)
}

func (r *PostgresObservationRepository) SaveTx(ctx context.Context, tx *sql.Tx, o *domain.Observation) error {
	return r.save(ctx, tx, o)
}

func (r *PostgresObservationRepository) save(ctx context.Context, ex sqlExecutor, o *domain.Observation) error {
	contentJSON, err := json.Marshal(o.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal observation content: %w", err)
	}
	if contentJSON == nil {
		contentJSON = []byte("{}")
	}

	var modelJSON []byte
	if o.Model != nil {
		modelJSON, err = json.Marshal(o.Model)
		if err != nil {
			return fmt.Errorf("failed to marshal model info: %w", err)
		}
	}

	query := `
		INSERT INTO ai_observations (
			id, organization_id, evidence_id, observation_type, observation_source,
			status, model, content, confidence, input_hash, output_hash,
			created_at, created_by, reviewed_at, reviewed_by, review_notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	_, err = ex.ExecContext(ctx, query,
		o.ID, o.OrganizationID, o.EvidenceID, o.Type, o.Source,
		o.Status, modelJSON, contentJSON, o.Confidence,
		o.InputHash, o.OutputHash, o.CreatedAt, o.CreatedBy,
		o.ReviewedAt, o.ReviewedBy, o.ReviewNotes,
	)
	if err != nil {
		return fmt.Errorf("failed to insert observation: %w", err)
	}
	return nil
}

func (r *PostgresObservationRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Observation, error) {
	query := `
		SELECT id, organization_id, evidence_id, observation_type, observation_source,
			   status, model, content, confidence, input_hash, output_hash,
			   created_at, created_by, reviewed_at, reviewed_by, review_notes
		FROM ai_observations
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanObservation(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresObservationRepository) FindByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*domain.Observation, int, error) {
	query := `
		SELECT id, organization_id, evidence_id, observation_type, observation_source,
			   status, model, content, confidence, input_hash, output_hash,
			   created_at, created_by, reviewed_at, reviewed_by, review_notes
		FROM ai_observations
		WHERE organization_id = $1 AND evidence_id = $2
		ORDER BY created_at ASC
		LIMIT $3 OFFSET $4
	`
	countQuery := `SELECT COUNT(*) FROM ai_observations WHERE organization_id = $1 AND evidence_id = $2`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, orgID, evidenceID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count observations: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, orgID, evidenceID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query observations: %w", err)
	}
	defer rows.Close()

	var items []*domain.Observation
	for rows.Next() {
		obs, err := r.scanObservation(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, obs)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, total, nil
}

func (r *PostgresObservationRepository) CountByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM ai_observations WHERE organization_id = $1 AND evidence_id = $2`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID, evidenceID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count observations: %w", err)
	}
	return total, nil
}

func (r *PostgresObservationRepository) UpdateStatus(ctx context.Context, orgID, observationID uuid.UUID, status domain.ObservationStatus, reviewerID *uuid.UUID, notes string) error {
	query := `
		UPDATE ai_observations SET
			status = $1,
			reviewed_at = $2,
			reviewed_by = $3,
			review_notes = $4
		WHERE organization_id = $5 AND id = $6
	`
	now := sql.NullTime{Time: readTime(), Valid: true}
	_, err := r.db.ExecContext(ctx, query, status, now, reviewerID, notes, orgID, observationID)
	if err != nil {
		return fmt.Errorf("failed to update observation status: %w", err)
	}
	return nil
}

func (r *PostgresObservationRepository) scanObservation(row interface {
	Scan(dest ...any) error
}) (*domain.Observation, error) {
	var o domain.Observation
	var contentJSON []byte
	var modelJSON []byte

	if err := row.Scan(
		&o.ID, &o.OrganizationID, &o.EvidenceID, &o.Type, &o.Source,
		&o.Status, &modelJSON, &contentJSON, &o.Confidence,
		&o.InputHash, &o.OutputHash, &o.CreatedAt, &o.CreatedBy,
		&o.ReviewedAt, &o.ReviewedBy, &o.ReviewNotes,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrObservationNotFound
		}
		return nil, fmt.Errorf("failed to scan observation: %w", err)
	}

	if len(contentJSON) > 0 {
		_ = json.Unmarshal(contentJSON, &o.Content)
	}
	if len(modelJSON) > 0 {
		var mi domain.ModelInfo
		if err := json.Unmarshal(modelJSON, &mi); err == nil {
			o.Model = &mi
		}
	}

	return &o, nil
}

func readTime() (t time.Time) {
	return time.Now().UTC()
}
