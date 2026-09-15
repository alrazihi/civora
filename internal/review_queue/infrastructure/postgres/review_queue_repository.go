package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	casesdomain "github.com/alrazihi/civora/internal/cases/domain"
	reviewdomain "github.com/alrazihi/civora/internal/review_queue/domain"
	"github.com/google/uuid"
)

type PostgresReviewQueueRepository struct {
	db *sql.DB
}

func NewPostgresReviewQueueRepository(db *sql.DB) *PostgresReviewQueueRepository {
	return &PostgresReviewQueueRepository{db: db}
}

func (r *PostgresReviewQueueRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresReviewQueueRepository) Save(ctx context.Context, entry *reviewdomain.ReviewQueueEntry) error {
	return r.saveReview(ctx, r.db, entry)
}

func (r *PostgresReviewQueueRepository) SaveTx(ctx context.Context, tx *sql.Tx, entry *reviewdomain.ReviewQueueEntry) error {
	return r.saveReview(ctx, tx, entry)
}

func (r *PostgresReviewQueueRepository) saveReview(ctx context.Context, ex interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}, entry *reviewdomain.ReviewQueueEntry) error {
	ruleEvalIDsJSON, err := json.Marshal(entry.RuleEvaluationIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal rule evaluation IDs: %w", err)
	}
	missingInfoJSON, err := json.Marshal(entry.MissingInformation)
	if err != nil {
		return fmt.Errorf("failed to marshal missing information: %w", err)
	}
	var assignedToIDArg any = entry.AssignedToID
	if entry.AssignedToID == nil {
		assignedToIDArg = nil
	}
	var completedAtArg any = entry.CompletedAt
	if entry.CompletedAt == nil {
		completedAtArg = nil
	}
	query := `
		INSERT INTO review_queue (
			id, organization_id, case_id, workflow_instance_id, status,
			assigned_to, priority, workflow_state, rule_evaluation_ids,
			missing_information, created_at, updated_at, completed_at, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
	`
	_, err = ex.ExecContext(ctx, query,
		entry.ID, entry.OrganizationID, entry.CaseID, entry.WorkflowInstanceID,
		entry.Status, assignedToIDArg, string(entry.Priority), entry.WorkflowState,
		ruleEvalIDsJSON, missingInfoJSON,
		entry.CreatedAt, entry.UpdatedAt, completedAtArg, entry.Metadata,
	)
	if err != nil {
		return fmt.Errorf("failed to insert review queue entry: %w", err)
	}
	return nil
}

func (r *PostgresReviewQueueRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*reviewdomain.ReviewQueueEntry, error) {
	query := `
		SELECT id, organization_id, case_id, workflow_instance_id, status,
			   assigned_to, priority, workflow_state, rule_evaluation_ids,
			   missing_information, created_at, updated_at, completed_at, metadata
		FROM review_queue
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanReview(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresReviewQueueRepository) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*reviewdomain.ReviewQueueEntry, error) {
	query := `
		SELECT id, organization_id, case_id, workflow_instance_id, status,
			   assigned_to, priority, workflow_state, rule_evaluation_ids,
			   missing_information, created_at, updated_at, completed_at, metadata
		FROM review_queue
		WHERE organization_id = $1 AND id = $2
		FOR UPDATE SKIP LOCKED
	`
	return r.scanReview(tx.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresReviewQueueRepository) FindByCaseID(ctx context.Context, orgID, caseID uuid.UUID) (*reviewdomain.ReviewQueueEntry, error) {
	query := `
		SELECT id, organization_id, case_id, workflow_instance_id, status,
			   assigned_to, priority, workflow_state, rule_evaluation_ids,
			   missing_information, created_at, updated_at, completed_at, metadata
		FROM review_queue
		WHERE organization_id = $1 AND case_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`
	return r.scanReview(r.db.QueryRowContext(ctx, query, orgID, caseID))
}

func (r *PostgresReviewQueueRepository) ListByOrganization(ctx context.Context, orgID uuid.UUID, status *reviewdomain.ReviewStatus, assignedToID *uuid.UUID, limit, offset int) ([]*reviewdomain.ReviewQueueEntry, int, error) {
	var where []string
	var args []any
	argIdx := 1

	where = append(where, fmt.Sprintf("organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	if status != nil {
		where = append(where, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*status))
		argIdx++
	}

	if assignedToID != nil {
		where = append(where, fmt.Sprintf("assigned_to = $%d", argIdx))
		args = append(args, *assignedToID)
		argIdx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + where[0]
		for i := 1; i < len(where); i++ {
			whereClause += " AND " + where[i]
		}
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM review_queue %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count review queue entries: %w", err)
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, organization_id, case_id, workflow_instance_id, status,
			   assigned_to, priority, workflow_state, rule_evaluation_ids,
			   missing_information, created_at, updated_at, completed_at, metadata
		FROM review_queue
		%s
		ORDER BY 
			CASE priority 
				WHEN 'URGENT' THEN 1 
				WHEN 'HIGH' THEN 2 
				WHEN 'NORMAL' THEN 3 
				WHEN 'LOW' THEN 4 
				ELSE 5 
			END,
			created_at ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query review queue: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*reviewdomain.ReviewQueueEntry
	for rows.Next() {
		entry, err := r.scanReviewFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, total, nil
}

func (r *PostgresReviewQueueRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, status reviewdomain.ReviewStatus, assignedToID *uuid.UUID) error {
	query := `
		UPDATE review_queue
		SET status = $1, assigned_to = $2, updated_at = $3
		WHERE organization_id = $4 AND id = $5
	`
	_, err := tx.ExecContext(ctx, query, string(status), assignedToID, time.Now().UTC(), orgID, id)
	if err != nil {
		return fmt.Errorf("failed to update review queue status: %w", err)
	}
	return nil
}

func (r *PostgresReviewQueueRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	query := `DELETE FROM review_queue WHERE organization_id = $1 AND id = $2`
	_, err := r.db.ExecContext(ctx, query, orgID, id)
	if err != nil {
		return fmt.Errorf("failed to delete review queue entry: %w", err)
	}
	return nil
}

func (r *PostgresReviewQueueRepository) DeleteTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) error {
	query := `DELETE FROM review_queue WHERE organization_id = $1 AND id = $2`
	_, err := tx.ExecContext(ctx, query, orgID, id)
	if err != nil {
		return fmt.Errorf("failed to delete review queue entry: %w", err)
	}
	return nil
}

func (r *PostgresReviewQueueRepository) scanReview(row interface {
	Scan(dest ...any) error
}) (*reviewdomain.ReviewQueueEntry, error) {
	var entry reviewdomain.ReviewQueueEntry
	var priorityStr string
	var assignedToStr sql.NullString
	var completedAt sql.NullTime
	var ruleEvalIDsJSON []byte
	var missingInfoJSON []byte
	var metadataJSON []byte
	if err := row.Scan(
		&entry.ID, &entry.OrganizationID, &entry.CaseID, &entry.WorkflowInstanceID,
		&entry.Status, &assignedToStr, &priorityStr, &entry.WorkflowState,
		&ruleEvalIDsJSON, &missingInfoJSON,
		&entry.CreatedAt, &entry.UpdatedAt, &completedAt, &metadataJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, reviewdomain.ErrReviewNotFound
		}
		return nil, fmt.Errorf("failed to scan review queue entry: %w", err)
	}
	entry.Priority = casesdomain.Priority(priorityStr)
	if assignedToStr.Valid {
		id, err := uuid.Parse(assignedToStr.String)
		if err == nil {
			entry.AssignedToID = &id
		}
	}
	if completedAt.Valid {
		t := completedAt.Time
		entry.CompletedAt = &t
	}
	if err := json.Unmarshal(ruleEvalIDsJSON, &entry.RuleEvaluationIDs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rule evaluation IDs: %w", err)
	}
	if err := json.Unmarshal(missingInfoJSON, &entry.MissingInformation); err != nil {
		return nil, fmt.Errorf("failed to unmarshal missing information: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &entry.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	return &entry, nil
}

func (r *PostgresReviewQueueRepository) scanReviewFromRows(rows *sql.Rows) (*reviewdomain.ReviewQueueEntry, error) {
	var entry reviewdomain.ReviewQueueEntry
	var priorityStr string
	var assignedToStr sql.NullString
	var completedAt sql.NullTime
	var ruleEvalIDsJSON []byte
	var missingInfoJSON []byte
	var metadataJSON []byte
	if err := rows.Scan(
		&entry.ID, &entry.OrganizationID, &entry.CaseID, &entry.WorkflowInstanceID,
		&entry.Status, &assignedToStr, &priorityStr, &entry.WorkflowState,
		&ruleEvalIDsJSON, &missingInfoJSON,
		&entry.CreatedAt, &entry.UpdatedAt, &completedAt, &metadataJSON,
	); err != nil {
		return nil, fmt.Errorf("failed to scan review queue entry: %w", err)
	}
	entry.Priority = casesdomain.Priority(priorityStr)
	if assignedToStr.Valid {
		id, err := uuid.Parse(assignedToStr.String)
		if err == nil {
			entry.AssignedToID = &id
		}
	}
	if completedAt.Valid {
		t := completedAt.Time
		entry.CompletedAt = &t
	}
	if err := json.Unmarshal(ruleEvalIDsJSON, &entry.RuleEvaluationIDs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rule evaluation IDs: %w", err)
	}
	if err := json.Unmarshal(missingInfoJSON, &entry.MissingInformation); err != nil {
		return nil, fmt.Errorf("failed to unmarshal missing information: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &entry.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	return &entry, nil
}
