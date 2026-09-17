package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/alrazihi/civora/internal/documentintelligence/domain"
	"github.com/google/uuid"
)

type PostgresAnalysisRepository struct {
	db *sql.DB
}

func NewPostgresAnalysisRepository(db *sql.DB) *PostgresAnalysisRepository {
	return &PostgresAnalysisRepository{db: db}
}

func (r *PostgresAnalysisRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresAnalysisRepository) Save(ctx context.Context, a *domain.DocumentAnalysis) error {
	return r.SaveTx(ctx, nil, a)
}

func (r *PostgresAnalysisRepository) SaveTx(ctx context.Context, tx *sql.Tx, a *domain.DocumentAnalysis) error {
	query := `
		INSERT INTO document_analyses (
			id, organization_id, document_id, evidence_id, type, source, status,
			model, content, confidence, input_hash, output_hash, schema_version,
			created_at, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			reviewed_at = EXCLUDED.reviewed_at,
			reviewed_by = EXCLUDED.reviewed_by,
			review_notes = EXCLUDED.review_notes
	`

	contentJSON, err := json.Marshal(a.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	modelJSON, err := json.Marshal(a.Model)
	if err != nil {
		return fmt.Errorf("failed to marshal model: %w", err)
	}

	var exec interface {
		ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	} = r.db
	if tx != nil {
		exec = tx
	}

	_, err = exec.ExecContext(ctx, query,
		a.ID,
		a.OrganizationID,
		a.DocumentID,
		a.EvidenceID,
		a.Type,
		a.Source,
		a.Status,
		modelJSON,
		contentJSON,
		a.Confidence,
		a.InputHash,
		a.OutputHash,
		a.SchemaVersion,
		a.CreatedAt,
		a.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to save analysis: %w", err)
	}
	return nil
}

func (r *PostgresAnalysisRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.DocumentAnalysis, error) {
	query := `
		SELECT id, organization_id, document_id, evidence_id, type, source, status,
		       model, content, confidence, input_hash, output_hash, schema_version,
		       created_at, created_by, reviewed_at, reviewed_by, review_notes
		FROM document_analyses
		WHERE id = $1 AND organization_id = $2
	`
	var a domain.DocumentAnalysis
	var modelJSON, contentJSON []byte

	err := r.db.QueryRowContext(ctx, query, id, orgID).Scan(
		&a.ID,
		&a.OrganizationID,
		&a.DocumentID,
		&a.EvidenceID,
		&a.Type,
		&a.Source,
		&a.Status,
		&modelJSON,
		&contentJSON,
		&a.Confidence,
		&a.InputHash,
		&a.OutputHash,
		&a.SchemaVersion,
		&a.CreatedAt,
		&a.CreatedBy,
		&a.ReviewedAt,
		&a.ReviewedBy,
		&a.ReviewNotes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrAnalysisNotFound
		}
		return nil, fmt.Errorf("failed to find analysis: %w", err)
	}

	if len(modelJSON) > 0 {
		if err := json.Unmarshal(modelJSON, &a.Model); err != nil {
			return nil, fmt.Errorf("failed to unmarshal model: %w", err)
		}
	}
	if len(contentJSON) > 0 {
		if err := json.Unmarshal(contentJSON, &a.Content); err != nil {
			return nil, fmt.Errorf("failed to unmarshal content: %w", err)
		}
	}

	return &a, nil
}

func (r *PostgresAnalysisRepository) FindByDocument(ctx context.Context, orgID, documentID uuid.UUID, limit, offset int) ([]*domain.DocumentAnalysis, int, error) {
	countQuery := `SELECT COUNT(*) FROM document_analyses WHERE document_id = $1 AND organization_id = $2`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, documentID, orgID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count analyses: %w", err)
	}

	query := `
		SELECT id, organization_id, document_id, evidence_id, type, source, status,
		       model, content, confidence, input_hash, output_hash, schema_version,
		       created_at, created_by, reviewed_at, reviewed_by, review_notes
		FROM document_analyses
		WHERE document_id = $1 AND organization_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.QueryContext(ctx, query, documentID, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query analyses: %w", err)
	}
	defer rows.Close()

	analyses := make([]*domain.DocumentAnalysis, 0, limit)
	for rows.Next() {
		var a domain.DocumentAnalysis
		var modelJSON, contentJSON []byte
		if err := rows.Scan(
			&a.ID,
			&a.OrganizationID,
			&a.DocumentID,
			&a.EvidenceID,
			&a.Type,
			&a.Source,
			&a.Status,
			&modelJSON,
			&contentJSON,
			&a.Confidence,
			&a.InputHash,
			&a.OutputHash,
			&a.SchemaVersion,
			&a.CreatedAt,
			&a.CreatedBy,
			&a.ReviewedAt,
			&a.ReviewedBy,
			&a.ReviewNotes,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan analysis: %w", err)
		}
		if len(modelJSON) > 0 {
			if err := json.Unmarshal(modelJSON, &a.Model); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal model: %w", err)
			}
		}
		if len(contentJSON) > 0 {
			if err := json.Unmarshal(contentJSON, &a.Content); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal content: %w", err)
			}
		}
		analyses = append(analyses, &a)
	}

	return analyses, total, nil
}

func (r *PostgresAnalysisRepository) FindByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*domain.DocumentAnalysis, int, error) {
	countQuery := `SELECT COUNT(*) FROM document_analyses WHERE evidence_id = $1 AND organization_id = $2`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, evidenceID, orgID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count analyses: %w", err)
	}

	query := `
		SELECT id, organization_id, document_id, evidence_id, type, source, status,
		       model, content, confidence, input_hash, output_hash, schema_version,
		       created_at, created_by, reviewed_at, reviewed_by, review_notes
		FROM document_analyses
		WHERE evidence_id = $1 AND organization_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.QueryContext(ctx, query, evidenceID, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query analyses: %w", err)
	}
	defer rows.Close()

	analyses := make([]*domain.DocumentAnalysis, 0, limit)
	for rows.Next() {
		var a domain.DocumentAnalysis
		var modelJSON, contentJSON []byte
		if err := rows.Scan(
			&a.ID,
			&a.OrganizationID,
			&a.DocumentID,
			&a.EvidenceID,
			&a.Type,
			&a.Source,
			&a.Status,
			&modelJSON,
			&contentJSON,
			&a.Confidence,
			&a.InputHash,
			&a.OutputHash,
			&a.SchemaVersion,
			&a.CreatedAt,
			&a.CreatedBy,
			&a.ReviewedAt,
			&a.ReviewedBy,
			&a.ReviewNotes,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan analysis: %w", err)
		}
		if len(modelJSON) > 0 {
			if err := json.Unmarshal(modelJSON, &a.Model); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal model: %w", err)
			}
		}
		if len(contentJSON) > 0 {
			if err := json.Unmarshal(contentJSON, &a.Content); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal content: %w", err)
			}
		}
		analyses = append(analyses, &a)
	}

	return analyses, total, nil
}

func (r *PostgresAnalysisRepository) UpdateStatus(ctx context.Context, orgID, analysisID uuid.UUID, status domain.AnalysisStatus, reviewerID *uuid.UUID, notes string) error {
	return r.UpdateStatusTx(ctx, nil, orgID, analysisID, status, reviewerID, notes)
}

func (r *PostgresAnalysisRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, analysisID uuid.UUID, status domain.AnalysisStatus, reviewerID *uuid.UUID, notes string) error {
	query := `
		UPDATE document_analyses
		SET status = $1, reviewed_at = NOW(), reviewed_by = $2, review_notes = $3
		WHERE id = $4 AND organization_id = $5
	`

	var exec interface {
		ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	} = r.db
	if tx != nil {
		exec = tx
	}

	res, err := exec.ExecContext(ctx, query, status, reviewerID, notes, analysisID, orgID)
	if err != nil {
		return fmt.Errorf("failed to update analysis status: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrAnalysisNotFound
	}
	return nil
}

func (r *PostgresAnalysisRepository) CountByDocument(ctx context.Context, orgID, documentID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM document_analyses WHERE document_id = $1 AND organization_id = $2`
	var count int
	err := r.db.QueryRowContext(ctx, query, documentID, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count analyses: %w", err)
	}
	return count, nil
}

var _ domain.DocumentAnalysisRepository = (*PostgresAnalysisRepository)(nil)
