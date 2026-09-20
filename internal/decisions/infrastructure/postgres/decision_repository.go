package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alrazihi/civora/internal/decisions/domain"
	"github.com/google/uuid"
)

type PostgresDecisionRepository struct {
	db *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func scanUUIDSlice(data []byte) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	if len(data) == 0 {
		return []uuid.UUID{}, nil
	}
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("failed to unmarshal uuid slice: %w", err)
	}
	return ids, nil
}

func NewPostgresDecisionRepository(db *sql.DB) *PostgresDecisionRepository {
	return &PostgresDecisionRepository{db: db}
}

func (r *PostgresDecisionRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresDecisionRepository) Save(ctx context.Context, d *domain.Decision) error {
	return r.saveDecision(ctx, r.db, d)
}

func (r *PostgresDecisionRepository) SaveTx(ctx context.Context, tx *sql.Tx, d *domain.Decision) error {
	return r.saveDecision(ctx, tx, d)
}

func (r *PostgresDecisionRepository) UpdateTx(ctx context.Context, tx *sql.Tx, d *domain.Decision) error {
	return r.updateDecision(ctx, tx, d.OrganizationID, d)
}

func (r *PostgresDecisionRepository) saveDecision(ctx context.Context, ex sqlExecer, d *domain.Decision) error {
	ruleEvalIDsJSON, err := json.Marshal(d.RuleEvaluationIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal rule evaluation IDs: %w", err)
	}
	evidenceIDsJSON, err := json.Marshal(d.EvidenceIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal evidence IDs: %w", err)
	}
	query := `
		INSERT INTO decisions (
			id, organization_id, service_request_id, decision, reason,
			decision_maker, decided_at, created_at, workflow_state,
			rule_evaluation_ids, evidence_ids, form_submission_id, review_queue_entry_id,
			version, superseded_by_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	var reviewQueueEntryIDArg interface{}
	if d.ReviewQueueEntryID != nil && *d.ReviewQueueEntryID != uuid.Nil {
		reviewQueueEntryIDArg = *d.ReviewQueueEntryID
	}
	_, err = ex.ExecContext(ctx, query,
		d.ID, d.OrganizationID, d.ServiceRequestID, d.Decision, d.Reason,
		d.DecisionMaker, d.DecidedAt, d.CreatedAt, d.WorkflowState,
		ruleEvalIDsJSON, evidenceIDsJSON, d.FormSubmissionID, reviewQueueEntryIDArg,
		d.Version, d.SupersededByID,
	)
	if err != nil {
		return fmt.Errorf("failed to insert decision: %w", err)
	}
	return nil
}

func (r *PostgresDecisionRepository) updateDecision(ctx context.Context, ex sqlExecer, orgID uuid.UUID, d *domain.Decision) error {
	query := `
		UPDATE decisions SET superseded_by_id = $1 WHERE id = $2 AND organization_id = $3
	`
	var supersededByIDArg interface{}
	if d.SupersededByID != nil && *d.SupersededByID != uuid.Nil {
		supersededByIDArg = *d.SupersededByID
	}
	_, err := ex.ExecContext(ctx, query, supersededByIDArg, d.ID, orgID)
	if err != nil {
		return fmt.Errorf("failed to update decision: %w", err)
	}
	return nil
}

func (r *PostgresDecisionRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Decision, error) {
	query := `
		SELECT id, organization_id, service_request_id, decision, reason,
			   decision_maker, decided_at, created_at, workflow_state,
			   rule_evaluation_ids, evidence_ids, form_submission_id, review_queue_entry_id,
			   version, superseded_by_id
		FROM decisions
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanDecision(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresDecisionRepository) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*domain.Decision, error) {
	query := `
		SELECT id, organization_id, service_request_id, decision, reason,
			   decision_maker, decided_at, created_at, workflow_state,
			   rule_evaluation_ids, evidence_ids, form_submission_id, review_queue_entry_id,
			   version, superseded_by_id
		FROM decisions
		WHERE organization_id = $1 AND service_request_id = $2
		ORDER BY version DESC
		LIMIT 1
	`
	return r.scanDecision(r.db.QueryRowContext(ctx, query, orgID, serviceRequestID))
}

func (r *PostgresDecisionRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Decision, error) {
	query := `
		SELECT id, organization_id, service_request_id, decision, reason,
			   decision_maker, decided_at, created_at, workflow_state,
			   rule_evaluation_ids, evidence_ids, form_submission_id, review_queue_entry_id,
			   version, superseded_by_id
		FROM decisions
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query decisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*domain.Decision
	for rows.Next() {
		d, err := r.scanDecisionFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, nil
}

func (r *PostgresDecisionRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM decisions WHERE organization_id = $1`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count decisions: %w", err)
	}
	return total, nil
}

func (r *PostgresDecisionRepository) ListByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) ([]*domain.Decision, error) {
	query := `
		SELECT id, organization_id, service_request_id, decision, reason,
			   decision_maker, decided_at, created_at, workflow_state,
			   rule_evaluation_ids, evidence_ids, form_submission_id, review_queue_entry_id,
			   version, superseded_by_id
		FROM decisions
		WHERE organization_id = $1 AND service_request_id = $2
		ORDER BY version ASC
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, serviceRequestID)
	if err != nil {
		return nil, fmt.Errorf("failed to query decisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*domain.Decision
	for rows.Next() {
		d, err := r.scanDecisionFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, nil
}

func (r *PostgresDecisionRepository) scanDecision(row interface {
	Scan(dest ...any) error
}) (*domain.Decision, error) {
	var d domain.Decision
	var supersededByID sql.NullString
	var formSubmissionID sql.NullString
	var reviewQueueEntryID sql.NullString
	var ruleEvalIDsJSON []byte
	var evidenceIDsJSON []byte
	if err := row.Scan(
		&d.ID, &d.OrganizationID, &d.ServiceRequestID, &d.Decision, &d.Reason,
		&d.DecisionMaker, &d.DecidedAt, &d.CreatedAt, &d.WorkflowState,
		&ruleEvalIDsJSON, &evidenceIDsJSON, &formSubmissionID, &reviewQueueEntryID, &d.Version,
		&supersededByID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrDecisionNotFound
		}
		return nil, fmt.Errorf("failed to scan decision: %w", err)
	}
	d.RuleEvaluationIDs, _ = scanUUIDSlice(ruleEvalIDsJSON)
	d.EvidenceIDs, _ = scanUUIDSlice(evidenceIDsJSON)
	if supersededByID.Valid {
		id, err := uuid.Parse(supersededByID.String)
		if err == nil {
			d.SupersededByID = &id
		}
	}
	if formSubmissionID.Valid {
		id, err := uuid.Parse(formSubmissionID.String)
		if err == nil {
			d.FormSubmissionID = &id
		}
	}
	if reviewQueueEntryID.Valid {
		id, err := uuid.Parse(reviewQueueEntryID.String)
		if err == nil {
			d.ReviewQueueEntryID = &id
		}
	}
	return &d, nil
}

func (r *PostgresDecisionRepository) scanDecisionFromRows(rows *sql.Rows) (*domain.Decision, error) {
	var d domain.Decision
	var supersededByID sql.NullString
	var formSubmissionID sql.NullString
	var reviewQueueEntryID sql.NullString
	var ruleEvalIDsJSON []byte
	var evidenceIDsJSON []byte
	if err := rows.Scan(
		&d.ID, &d.OrganizationID, &d.ServiceRequestID, &d.Decision, &d.Reason,
		&d.DecisionMaker, &d.DecidedAt, &d.CreatedAt, &d.WorkflowState,
		&ruleEvalIDsJSON, &evidenceIDsJSON, &formSubmissionID, &reviewQueueEntryID, &d.Version,
		&supersededByID,
	); err != nil {
		return nil, fmt.Errorf("failed to scan decision: %w", err)
	}
	d.RuleEvaluationIDs, _ = scanUUIDSlice(ruleEvalIDsJSON)
	d.EvidenceIDs, _ = scanUUIDSlice(evidenceIDsJSON)
	if supersededByID.Valid {
		id, err := uuid.Parse(supersededByID.String)
		if err == nil {
			d.SupersededByID = &id
		}
	}
	if formSubmissionID.Valid {
		id, err := uuid.Parse(formSubmissionID.String)
		if err == nil {
			d.FormSubmissionID = &id
		}
	}
	if reviewQueueEntryID.Valid {
		id, err := uuid.Parse(reviewQueueEntryID.String)
		if err == nil {
			d.ReviewQueueEntryID = &id
		}
	}
	return &d, nil
}
