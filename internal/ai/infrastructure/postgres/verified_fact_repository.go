package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/alrazihi/civora/internal/ai/domain"
	"github.com/google/uuid"
)

type PostgresVerifiedFactRepository struct {
	db *sql.DB
}

func NewPostgresVerifiedFactRepository(db *sql.DB) *PostgresVerifiedFactRepository {
	return &PostgresVerifiedFactRepository{db: db}
}

func (r *PostgresVerifiedFactRepository) Save(ctx context.Context, f *domain.VerifiedFact) error {
	return r.save(ctx, r.db, f)
}

func (r *PostgresVerifiedFactRepository) SaveTx(ctx context.Context, tx *sql.Tx, f *domain.VerifiedFact) error {
	return r.save(ctx, tx, f)
}

func (r *PostgresVerifiedFactRepository) save(ctx context.Context, ex sqlExecutor, f *domain.VerifiedFact) error {
	valueJSON, err := json.Marshal(f.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal verified fact value: %w", err)
	}
	if valueJSON == nil {
		valueJSON = []byte("{}")
	}

	originalValueJSON, err := json.Marshal(f.OriginalValue)
	if err != nil {
		return fmt.Errorf("failed to marshal original value: %w", err)
	}
	if originalValueJSON == nil {
		originalValueJSON = []byte("{}")
	}

	var correctedValueJSON []byte
	if f.CorrectedValue != nil {
		correctedValueJSON, err = json.Marshal(f.CorrectedValue)
		if err != nil {
			return fmt.Errorf("failed to marshal corrected value: %w", err)
		}
	} else {
		correctedValueJSON = nil
	}

	provenanceJSON, err := json.Marshal(f.Provenance)
	if err != nil {
		return fmt.Errorf("failed to marshal provenance: %w", err)
	}
	if provenanceJSON == nil {
		provenanceJSON = []byte("{}")
	}

	var modelJSON []byte
	if f.Model != nil {
		modelJSON, err = json.Marshal(f.Model)
		if err != nil {
			return fmt.Errorf("failed to marshal model info: %w", err)
		}
	}

	var caseID interface{} = nil
	if f.CaseID != nil {
		caseID = *f.CaseID
	}

	query := `
		INSERT INTO ai_verified_facts (
			id, organization_id, case_id, observation_id, observation_type,
			value, original_value, corrected_value, provenance,
			review_action, reviewer_id, review_notes,
			created_at, verified_at, source, model,
			input_hash, output_hash
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (observation_id) DO UPDATE SET
			value = EXCLUDED.value,
			corrected_value = EXCLUDED.corrected_value,
			provenance = EXCLUDED.provenance,
			review_action = EXCLUDED.review_action,
			reviewer_id = EXCLUDED.reviewer_id,
			review_notes = EXCLUDED.review_notes,
			verified_at = EXCLUDED.verified_at,
			source = EXCLUDED.source,
			model = EXCLUDED.model,
			input_hash = EXCLUDED.input_hash,
			output_hash = EXCLUDED.output_hash
	`
	_, err = ex.ExecContext(ctx, query,
		f.ID, f.OrganizationID, caseID, f.ObservationID, f.Type,
		valueJSON, originalValueJSON, correctedValueJSON, provenanceJSON,
		f.ReviewAction, f.ReviewerID, f.ReviewNotes,
		f.CreatedAt, f.VerifiedAt, f.Source, modelJSON,
		f.InputHash, f.OutputHash,
	)
	if err != nil {
		return fmt.Errorf("failed to insert verified fact: %w", err)
	}
	return nil
}

func (r *PostgresVerifiedFactRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.VerifiedFact, error) {
	query := `
		SELECT id, organization_id, case_id, observation_id, observation_type,
			   value, original_value, corrected_value, provenance,
			   review_action, reviewer_id, review_notes,
			   created_at, verified_at, source, model,
			   input_hash, output_hash
		FROM ai_verified_facts
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanVerifiedFact(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresVerifiedFactRepository) FindByObservation(ctx context.Context, orgID, observationID uuid.UUID) (*domain.VerifiedFact, error) {
	query := `
		SELECT id, organization_id, case_id, observation_id, observation_type,
			   value, original_value, corrected_value, provenance,
			   review_action, reviewer_id, review_notes,
			   created_at, verified_at, source, model,
			   input_hash, output_hash
		FROM ai_verified_facts
		WHERE organization_id = $1 AND observation_id = $2
	`
	return r.scanVerifiedFact(r.db.QueryRowContext(ctx, query, orgID, observationID))
}

func (r *PostgresVerifiedFactRepository) FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*domain.VerifiedFact, int, error) {
	query := `
		SELECT id, organization_id, case_id, observation_id, observation_type,
			   value, original_value, corrected_value, provenance,
			   review_action, reviewer_id, review_notes,
			   created_at, verified_at, source, model,
			   input_hash, output_hash
		FROM ai_verified_facts
		WHERE organization_id = $1 AND case_id = $2
		ORDER BY verified_at DESC
		LIMIT $3 OFFSET $4
	`
	countQuery := `SELECT COUNT(*) FROM ai_verified_facts WHERE organization_id = $1 AND case_id = $2`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, orgID, caseID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count verified facts: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, orgID, caseID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query verified facts: %w", err)
	}
	defer rows.Close()

	var items []*domain.VerifiedFact
	for rows.Next() {
		f, err := r.scanVerifiedFact(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, total, nil
}

func (r *PostgresVerifiedFactRepository) CountByCase(ctx context.Context, orgID, caseID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM ai_verified_facts WHERE organization_id = $1 AND case_id = $2`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID, caseID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count verified facts: %w", err)
	}
	return total, nil
}

func (r *PostgresVerifiedFactRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	query := `DELETE FROM ai_verified_facts WHERE organization_id = $1 AND id = $2`
	_, err := r.db.ExecContext(ctx, query, orgID, id)
	if err != nil {
		return fmt.Errorf("failed to delete verified fact: %w", err)
	}
	return nil
}

func (r *PostgresVerifiedFactRepository) scanVerifiedFact(row interface{ Scan(dest ...any) error }) (*domain.VerifiedFact, error) {
	var f domain.VerifiedFact
	var valueJSON []byte
	var originalValueJSON []byte
	var correctedValueJSON []byte
	var provenanceJSON []byte
	var modelJSON []byte
	var caseID sql.NullString

	if err := row.Scan(
		&f.ID, &f.OrganizationID, &caseID, &f.ObservationID, &f.Type,
		&valueJSON, &originalValueJSON, &correctedValueJSON, &provenanceJSON,
		&f.ReviewAction, &f.ReviewerID, &f.ReviewNotes,
		&f.CreatedAt, &f.VerifiedAt, &f.Source, &modelJSON,
		&f.InputHash, &f.OutputHash,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrVerifiedFactNotFound
		}
		return nil, fmt.Errorf("failed to scan verified fact: %w", err)
	}

	if caseID.Valid {
		id, err := uuid.Parse(caseID.String)
		if err == nil {
			f.CaseID = &id
		}
	}

	if len(valueJSON) > 0 {
		_ = json.Unmarshal(valueJSON, &f.Value)
	}
	if len(originalValueJSON) > 0 {
		_ = json.Unmarshal(originalValueJSON, &f.OriginalValue)
	}
	if len(correctedValueJSON) > 0 && string(correctedValueJSON) != "null" {
		_ = json.Unmarshal(correctedValueJSON, &f.CorrectedValue)
	}
	if len(provenanceJSON) > 0 {
		_ = json.Unmarshal(provenanceJSON, &f.Provenance)
	}
	if len(modelJSON) > 0 {
		var mi domain.ModelInfo
		if err := json.Unmarshal(modelJSON, &mi); err == nil {
			f.Model = &mi
		}
	}

	return &f, nil
}
