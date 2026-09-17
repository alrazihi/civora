package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/casesummary/domain"
	"github.com/google/uuid"
)

type PostgresCaseSummaryRepository struct {
	db *sql.DB
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func NewPostgresCaseSummaryRepository(db *sql.DB) *PostgresCaseSummaryRepository {
	return &PostgresCaseSummaryRepository{db: db}
}

func (r *PostgresCaseSummaryRepository) Save(ctx context.Context, s *domain.CaseSummary) error {
	return r.save(ctx, r.db, s)
}

func (r *PostgresCaseSummaryRepository) SaveTx(ctx context.Context, tx *sql.Tx, s *domain.CaseSummary) error {
	return r.save(ctx, tx, s)
}

func (r *PostgresCaseSummaryRepository) save(ctx context.Context, ex sqlExecutor, s *domain.CaseSummary) error {
	contentJSON, err := json.Marshal(s.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal summary content: %w", err)
	}

	var modelJSON []byte
	if s.Model != nil {
		modelJSON, err = json.Marshal(s.Model)
		if err != nil {
			return fmt.Errorf("failed to marshal model info: %w", err)
		}
	}

	query := `
		INSERT INTO case_summaries (
			id, organization_id, case_id, summary_type, source, status,
			model, content, confidence, input_hash, output_hash, schema_version, token_estimate,
			created_at, created_by, reviewed_at, reviewed_by, review_notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`
	_, err = ex.ExecContext(ctx, query,
		s.ID, s.OrganizationID, s.CaseID, s.Type, s.Source, s.Status,
		modelJSON, contentJSON, s.Confidence, s.InputHash, s.OutputHash,
		s.SchemaVersion, s.TokenEstimate, s.CreatedAt, s.CreatedBy,
		s.ReviewedAt, s.ReviewedBy, s.ReviewNotes,
	)
	if err != nil {
		return fmt.Errorf("failed to insert case summary: %w", err)
	}
	return nil
}

func (r *PostgresCaseSummaryRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.CaseSummary, error) {
	query := `
		SELECT id, organization_id, case_id, summary_type, source, status,
			   model, content, confidence, input_hash, output_hash, schema_version, token_estimate,
			   created_at, created_by, reviewed_at, reviewed_by, review_notes
		FROM case_summaries
		WHERE organization_id = $1 AND id = $2
	`
	row := r.db.QueryRowContext(ctx, query, orgID, id)
	return r.scanSummary(row)
}

func (r *PostgresCaseSummaryRepository) FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*domain.CaseSummary, int, error) {
	query := `
		SELECT id, organization_id, case_id, summary_type, source, status,
			   model, content, confidence, input_hash, output_hash, schema_version, token_estimate,
			   created_at, created_by, reviewed_at, reviewed_by, review_notes
		FROM case_summaries
		WHERE organization_id = $1 AND case_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	countQuery := `SELECT COUNT(*) FROM case_summaries WHERE organization_id = $1 AND case_id = $2`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, orgID, caseID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count summaries: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, orgID, caseID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query summaries: %w", err)
	}
	defer rows.Close()

	var items []*domain.CaseSummary
	for rows.Next() {
		summary, err := r.scanSummary(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, total, nil
}

func (r *PostgresCaseSummaryRepository) UpdateStatus(ctx context.Context, orgID, summaryID uuid.UUID, status domain.SummaryStatus, reviewerID *uuid.UUID, notes string) error {
	return r.updateStatus(ctx, r.db, orgID, summaryID, status, reviewerID, notes)
}

func (r *PostgresCaseSummaryRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, summaryID uuid.UUID, status domain.SummaryStatus, reviewerID *uuid.UUID, notes string) error {
	return r.updateStatus(ctx, tx, orgID, summaryID, status, reviewerID, notes)
}

func (r *PostgresCaseSummaryRepository) updateStatus(ctx context.Context, ex sqlExecutor, orgID, summaryID uuid.UUID, status domain.SummaryStatus, reviewerID *uuid.UUID, notes string) error {
	query := `
		UPDATE case_summaries SET
			status = $1,
			reviewed_at = $2,
			reviewed_by = $3,
			review_notes = $4
		WHERE organization_id = $5 AND id = $6
	`
	now := sql.NullTime{Time: time.Now(), Valid: true}
	_, err := ex.ExecContext(ctx, query, status, now, reviewerID, notes, orgID, summaryID)
	if err != nil {
		return fmt.Errorf("failed to update summary status: %w", err)
	}
	return nil
}

func (r *PostgresCaseSummaryRepository) CountByCase(ctx context.Context, orgID, caseID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM case_summaries WHERE organization_id = $1 AND case_id = $2`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID, caseID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count summaries: %w", err)
	}
	return total, nil
}

func (r *PostgresCaseSummaryRepository) scanSummary(row interface {
	Scan(dest ...any) error
}) (*domain.CaseSummary, error) {
	var s domain.CaseSummary
	var contentJSON []byte
	var modelJSON []byte

	if err := row.Scan(
		&s.ID, &s.OrganizationID, &s.CaseID, &s.Type, &s.Source, &s.Status,
		&modelJSON, &contentJSON, &s.Confidence, &s.InputHash, &s.OutputHash,
		&s.SchemaVersion, &s.TokenEstimate, &s.CreatedAt, &s.CreatedBy,
		&s.ReviewedAt, &s.ReviewedBy, &s.ReviewNotes,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSummaryNotFound
		}
		return nil, fmt.Errorf("failed to scan case summary: %w", err)
	}

	if len(contentJSON) > 0 {
		if err := json.Unmarshal(contentJSON, &s.Content); err != nil {
			return nil, fmt.Errorf("failed to unmarshal content: %w", err)
		}
	}
	if len(modelJSON) > 0 {
		var mi domain.ModelInfo
		if err := json.Unmarshal(modelJSON, &mi); err == nil {
			s.Model = &mi
		}
	}

	return &s, nil
}
