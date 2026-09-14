package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	rulesdomain "github.com/alrazihi/civora/internal/rules/domain"
	"github.com/google/uuid"
)

type PostgresRuleSetRepository struct {
	db *sql.DB
}

func NewPostgresRuleSetRepository(db *sql.DB) *PostgresRuleSetRepository {
	return &PostgresRuleSetRepository{db: db}
}

func (r *PostgresRuleSetRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresRuleSetRepository) SaveTx(ctx context.Context, tx *sql.Tx, rs *rulesdomain.RuleSet) error {
	rulesJSON, err := json.Marshal(rs.Rules)
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	triggersJSON, err := json.Marshal(rs.Triggers)
	if err != nil {
		return fmt.Errorf("failed to marshal triggers: %w", err)
	}

	var caseID uuid.UUID
	hasCaseID := rs.CaseID != nil && *rs.CaseID != uuid.Nil
	if hasCaseID {
		caseID = *rs.CaseID
	}

	query := `
		INSERT INTO rules.rule_sets (id, organization_id, case_id, key, name, description, version, status, default_outcome, rules, created_by, created_at, updated_at, triggers)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	execer := r.getExecer(tx)
	_, err = execer.ExecContext(ctx, query,
		rs.ID, rs.OrganizationID, caseID, rs.Key, rs.Name, rs.Description,
		rs.Version, rs.Status, rs.DefaultOutcome, rulesJSON, rs.CreatedBy,
		rs.CreatedAt, rs.UpdatedAt, triggersJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to save rule set: %w", err)
	}
	return nil
}

func (r *PostgresRuleSetRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*rulesdomain.RuleSet, error) {
	return r.FindByIDTx(ctx, nil, orgID, id)
}

func (r *PostgresRuleSetRepository) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*rulesdomain.RuleSet, error) {
	query := `
		SELECT id, organization_id, case_id, key, name, description, version, status, default_outcome, rules, created_by, created_at, updated_at, triggers
		FROM rules.rule_sets
		WHERE organization_id = $1 AND id = $2
	`
	row := r.queryRow(ctx, tx, query, orgID, id)
	return r.scanRuleSet(row)
}

func (r *PostgresRuleSetRepository) FindByIDForUpdateTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*rulesdomain.RuleSet, error) {
	query := `
		SELECT id, organization_id, case_id, key, name, description, version, status, default_outcome, rules, created_by, created_at, updated_at, triggers
		FROM rules.rule_sets
		WHERE organization_id = $1 AND id = $2
		FOR UPDATE
	`
	row := r.queryRow(ctx, tx, query, orgID, id)
	return r.scanRuleSet(row)
}

func (r *PostgresRuleSetRepository) FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*rulesdomain.RuleSet, error) {
	return r.FindByKeyTx(ctx, nil, orgID, key)
}

func (r *PostgresRuleSetRepository) FindByKeyTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, key string) (*rulesdomain.RuleSet, error) {
	query := `
		SELECT id, organization_id, case_id, key, name, description, version, status, default_outcome, rules, created_by, created_at, updated_at, triggers
		FROM rules.rule_sets
		WHERE organization_id = $1 AND key = $2
		ORDER BY version DESC
		LIMIT 1
	`
	row := r.queryRow(ctx, tx, query, orgID, key)
	return r.scanRuleSet(row)
}

func (r *PostgresRuleSetRepository) FindByKeyAndVersion(ctx context.Context, orgID uuid.UUID, key string, version int) (*rulesdomain.RuleSet, error) {
	query := `
		SELECT id, organization_id, case_id, key, name, description, version, status, default_outcome, rules, created_by, created_at, updated_at, triggers
		FROM rules.rule_sets
		WHERE organization_id = $1 AND key = $2 AND version = $3
	`
	row := r.queryRow(ctx, nil, query, orgID, key, version)
	return r.scanRuleSet(row)
}

func (r *PostgresRuleSetRepository) List(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*rulesdomain.RuleSet, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rules.rule_sets WHERE organization_id = $1", orgID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count rule sets: %w", err)
	}

	query := `
		SELECT id, organization_id, case_id, key, name, description, version, status, default_outcome, rules, created_by, created_at, updated_at, triggers
		FROM rules.rule_sets
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list rule sets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sets []*rulesdomain.RuleSet
	for rows.Next() {
		rs, err := r.scanRuleSet(rows)
		if err != nil {
			return nil, 0, err
		}
		sets = append(sets, rs)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}
	return sets, total, nil
}

func (r *PostgresRuleSetRepository) ListByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*rulesdomain.RuleSet, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rules.rule_sets WHERE organization_id = $1 AND case_id = $2", orgID, caseID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count rule sets by case: %w", err)
	}

	query := `
		SELECT id, organization_id, case_id, key, name, description, version, status, default_outcome, rules, created_by, created_at, updated_at, triggers
		FROM rules.rule_sets
		WHERE organization_id = $1 AND case_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, caseID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list rule sets by case: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sets []*rulesdomain.RuleSet
	for rows.Next() {
		rs, err := r.scanRuleSet(rows)
		if err != nil {
			return nil, 0, err
		}
		sets = append(sets, rs)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}
	return sets, total, nil
}

func (r *PostgresRuleSetRepository) ListVersions(ctx context.Context, orgID uuid.UUID, key string, limit, offset int) ([]*rulesdomain.RuleSet, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rules.rule_sets WHERE organization_id = $1 AND key = $2", orgID, key).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count versions: %w", err)
	}

	query := `
		SELECT id, organization_id, case_id, key, name, description, version, status, default_outcome, rules, created_by, created_at, updated_at, triggers
		FROM rules.rule_sets
		WHERE organization_id = $1 AND key = $2
		ORDER BY version DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, key, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sets []*rulesdomain.RuleSet
	for rows.Next() {
		rs, err := r.scanRuleSet(rows)
		if err != nil {
			return nil, 0, err
		}
		sets = append(sets, rs)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}
	return sets, total, nil
}

func (r *PostgresRuleSetRepository) FindPublished(ctx context.Context, orgID uuid.UUID, key string, caseID *uuid.UUID) (*rulesdomain.RuleSet, error) {
	var caseIDVal uuid.UUID
	hasCaseID := caseID != nil && *caseID != uuid.Nil
	if hasCaseID {
		caseIDVal = *caseID
	}

	query := `
		SELECT id, organization_id, case_id, key, name, description, version, status, default_outcome, rules, created_by, created_at, updated_at, triggers
		FROM rules.rule_sets
		WHERE organization_id = $1 AND key = $2 AND status = 'PUBLISHED'
	`
	if hasCaseID {
		query += " AND case_id = $3"
	} else {
		query += " AND case_id IS NULL"
	}
	query += " ORDER BY version DESC LIMIT 1"

	row := r.queryRow(ctx, nil, query, orgID, key, caseIDVal)
	return r.scanRuleSet(row)
}

func (r *PostgresRuleSetRepository) UpdateTx(ctx context.Context, tx *sql.Tx, rs *rulesdomain.RuleSet) error {
	rulesJSON, err := json.Marshal(rs.Rules)
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}
	triggersJSON, err := json.Marshal(rs.Triggers)
	if err != nil {
		return fmt.Errorf("failed to marshal triggers: %w", err)
	}

	query := `
		UPDATE rules.rule_sets
		SET name = $1, description = $2, version = $3, status = $4, default_outcome = $5, rules = $6, updated_at = now(), triggers = $7
		WHERE organization_id = $8 AND id = $9
	`
	result, err := r.exec(ctx, tx, query,
		rs.Name, rs.Description, rs.Version, rs.Status, rs.DefaultOutcome,
		rulesJSON, triggersJSON, rs.OrganizationID, rs.ID,
	)
	if err != nil {
		return err
	}
	if result == 0 {
		return rulesdomain.ErrRuleSetNotFound
	}
	return nil
}

// UpdateStatusTx transitions a rule set's status only if it currently has the
// expected status. Returning errRuleSetNotFound on zero rows makes status
// transitions safe under concurrency (no TOCTOU between the read and write),
// enforcing immutable active/published rule sets.
func (r *PostgresRuleSetRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, from, to rulesdomain.RuleSetStatus) error {
	result, err := r.exec(ctx, tx,
		`UPDATE rules.rule_sets SET status = $1, updated_at = now() WHERE organization_id = $2 AND id = $3 AND status = $4`,
		to, orgID, id, from,
	)
	if err != nil {
		return fmt.Errorf("failed to update rule set status: %w", err)
	}
	if result == 0 {
		return rulesdomain.ErrRuleSetNotFound
	}
	return nil
}

func (r *PostgresRuleSetRepository) DeleteTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) error {
	query := "DELETE FROM rules.rule_sets WHERE organization_id = $1 AND id = $2"
	result, err := r.exec(ctx, tx, query, orgID, id)
	if err != nil {
		return fmt.Errorf("failed to delete rule set: %w", err)
	}
	if result == 0 {
		return rulesdomain.ErrRuleSetNotFound
	}
	return nil
}

func (r *PostgresRuleSetRepository) CountByKey(ctx context.Context, orgID uuid.UUID, key string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rules.rule_sets WHERE organization_id = $1 AND key = $2", orgID, key).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count rule sets by key: %w", err)
	}
	return count, nil
}

func (r *PostgresRuleSetRepository) queryRow(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) *sql.Row {
	if tx != nil {
		return tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *PostgresRuleSetRepository) exec(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) (int64, error) {
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

func (r *PostgresRuleSetRepository) getExecer(tx *sql.Tx) interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
} {
	if tx != nil {
		return tx
	}
	return r.db
}

func (r *PostgresRuleSetRepository) scanRuleSet(row interface {
	Scan(dest ...interface{}) error
}) (*rulesdomain.RuleSet, error) {
	var rs rulesdomain.RuleSet
	var status string
	var outcome string
	var caseID uuid.UUID
	var hasCaseID bool
	var rulesJSON []byte
	var triggersJSON []byte

	err := row.Scan(
		&rs.ID, &rs.OrganizationID, &caseID, &rs.Key, &rs.Name, &rs.Description,
		&rs.Version, &status, &outcome, &rulesJSON, &rs.CreatedBy,
		&rs.CreatedAt, &rs.UpdatedAt, &triggersJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, rulesdomain.ErrRuleSetNotFound
		}
		return nil, fmt.Errorf("failed to scan rule set: %w", err)
	}

	rs.Status = rulesdomain.RuleSetStatus(status)
	rs.DefaultOutcome = rulesdomain.Outcome(outcome)
	if caseID != uuid.Nil {
		rs.CaseID = &caseID
		hasCaseID = true
	} else {
		rs.CaseID = nil
		hasCaseID = false
	}

	if len(rulesJSON) > 0 {
		dec := json.NewDecoder(bytes.NewReader(rulesJSON))
		dec.UseNumber()
		if err := dec.Decode(&rs.Rules); err != nil {
			return nil, fmt.Errorf("failed to decode rules JSON: %w", err)
		}
	}

	if len(triggersJSON) > 0 {
		dec := json.NewDecoder(bytes.NewReader(triggersJSON))
		if err := dec.Decode(&rs.Triggers); err != nil {
			return nil, fmt.Errorf("failed to decode triggers JSON: %w", err)
		}
	}
	if rs.Triggers == nil {
		rs.Triggers = []string{}
	}

	_ = hasCaseID
	return &rs, nil
}

type PostgresEvaluationRepository struct {
	db *sql.DB
}

func NewPostgresEvaluationRepository(db *sql.DB) *PostgresEvaluationRepository {
	return &PostgresEvaluationRepository{db: db}
}

func (r *PostgresEvaluationRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresEvaluationRepository) SaveTx(ctx context.Context, tx *sql.Tx, ev *rulesdomain.Evaluation) error {
	traceJSON, err := json.Marshal(ev.Trace)
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}
	factsJSON, err := json.Marshal(ev.FactsSnapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal facts snapshot: %w", err)
	}

	var caseID uuid.UUID
	hasCaseID := ev.CaseID != nil && *ev.CaseID != uuid.Nil
	if hasCaseID {
		caseID = *ev.CaseID
	}

	var matchedRuleID uuid.UUID
	hasMatchedRuleID := ev.MatchedRuleID != nil && *ev.MatchedRuleID != uuid.Nil
	if hasMatchedRuleID {
		matchedRuleID = *ev.MatchedRuleID
	}

	var evalBy uuid.UUID
	hasEvalBy := ev.EvaluatedBy != nil && *ev.EvaluatedBy != uuid.Nil
	if hasEvalBy {
		evalBy = *ev.EvaluatedBy
	}

	var reasonStr string
	if ev.Reason != nil {
		reasonStr = *ev.Reason
	}

	query := `
		INSERT INTO rules.evaluations (id, rule_set_id, rule_set_version, organization_id, case_id, status, outcome, reason, matched_rule_id, trace, trigger, performed_by, evaluated_at, facts_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	execer := r.getExecer(tx)
	_, err = execer.ExecContext(ctx, query,
		ev.ID, ev.RuleSetID, ev.RuleSetVersion, ev.OrganizationID, caseID,
		string(ev.Status), string(ev.Outcome), reasonStr, matchedRuleID,
		traceJSON, string(ev.Trigger), evalBy, ev.EvaluatedAt, factsJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to save evaluation: %w", err)
	}
	return nil
}

func (r *PostgresEvaluationRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*rulesdomain.Evaluation, error) {
	return r.FindByIDTx(ctx, nil, orgID, id)
}

func (r *PostgresEvaluationRepository) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*rulesdomain.Evaluation, error) {
	query := `
		SELECT id, rule_set_id, rule_set_version, organization_id, case_id, status, outcome, reason, matched_rule_id, trace, trigger, performed_by, evaluated_at, facts_snapshot
		FROM rules.evaluations
		WHERE organization_id = $1 AND id = $2
	`
	row := r.queryRow(ctx, tx, query, orgID, id)
	return r.scanEvaluation(row)
}

func (r *PostgresEvaluationRepository) ListByRuleSet(ctx context.Context, orgID, ruleSetID uuid.UUID, limit, offset int) ([]*rulesdomain.Evaluation, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rules.evaluations WHERE organization_id = $1 AND rule_set_id = $2", orgID, ruleSetID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count evaluations: %w", err)
	}

	query := `
		SELECT id, rule_set_id, rule_set_version, organization_id, case_id, status, outcome, reason, matched_rule_id, trace, trigger, performed_by, evaluated_at, facts_snapshot
		FROM rules.evaluations
		WHERE organization_id = $1 AND rule_set_id = $2
		ORDER BY evaluated_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, ruleSetID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list evaluations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var evals []*rulesdomain.Evaluation
	for rows.Next() {
		ev, err := r.scanEvaluation(rows)
		if err != nil {
			return nil, 0, err
		}
		evals = append(evals, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}
	return evals, total, nil
}

func (r *PostgresEvaluationRepository) ListByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*rulesdomain.Evaluation, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rules.evaluations WHERE organization_id = $1 AND case_id = $2", orgID, caseID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count evaluations by case: %w", err)
	}

	query := `
		SELECT id, rule_set_id, rule_set_version, organization_id, case_id, status, outcome, reason, matched_rule_id, trace, trigger, performed_by, evaluated_at, facts_snapshot
		FROM rules.evaluations
		WHERE organization_id = $1 AND case_id = $2
		ORDER BY evaluated_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, caseID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list evaluations by case: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var evals []*rulesdomain.Evaluation
	for rows.Next() {
		ev, err := r.scanEvaluation(rows)
		if err != nil {
			return nil, 0, err
		}
		evals = append(evals, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}
	return evals, total, nil
}

func (r *PostgresEvaluationRepository) queryRow(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) *sql.Row {
	if tx != nil {
		return tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *PostgresEvaluationRepository) exec(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) (int64, error) {
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

func (r *PostgresEvaluationRepository) getExecer(tx *sql.Tx) interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
} {
	if tx != nil {
		return tx
	}
	return r.db
}

func (r *PostgresEvaluationRepository) scanEvaluation(row interface {
	Scan(dest ...interface{}) error
}) (*rulesdomain.Evaluation, error) {
	var ev rulesdomain.Evaluation
	var status string
	var outcome string
	var trigger string
	var caseID uuid.UUID
	var hasCaseID bool
	var matchedRuleID uuid.UUID
	var hasMatchedRuleID bool
	var evalBy uuid.UUID
	var hasEvalBy bool
	var traceJSON []byte
	var factsJSON []byte
	var reasonStr sql.NullString

	err := row.Scan(
		&ev.ID, &ev.RuleSetID, &ev.RuleSetVersion, &ev.OrganizationID, &caseID,
		&status, &outcome, &reasonStr, &matchedRuleID, &traceJSON, &trigger,
		&evalBy, &ev.EvaluatedAt, &factsJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, rulesdomain.ErrRuleSetNotFound
		}
		return nil, fmt.Errorf("failed to scan evaluation: %w", err)
	}

	ev.Status = rulesdomain.EvaluationStatus(status)
	ev.Outcome = rulesdomain.Outcome(outcome)
	ev.Trigger = rulesdomain.Trigger(trigger)

	if caseID != uuid.Nil {
		ev.CaseID = &caseID
		hasCaseID = true
	} else {
		ev.CaseID = nil
		hasCaseID = false
	}

	if matchedRuleID != uuid.Nil {
		ev.MatchedRuleID = &matchedRuleID
		hasMatchedRuleID = true
	} else {
		ev.MatchedRuleID = nil
		hasMatchedRuleID = false
	}

	if evalBy != uuid.Nil {
		ev.EvaluatedBy = &evalBy
		hasEvalBy = true
	} else {
		ev.EvaluatedBy = nil
		hasEvalBy = false
	}

	_ = hasCaseID
	_ = hasMatchedRuleID
	_ = hasEvalBy

	if reasonStr.Valid {
		ev.Reason = &reasonStr.String
	} else {
		ev.Reason = nil
	}

	if len(traceJSON) > 0 {
		dec := json.NewDecoder(bytes.NewReader(traceJSON))
		dec.UseNumber()
		if err := dec.Decode(&ev.Trace); err != nil {
			return nil, fmt.Errorf("failed to decode trace JSON: %w", err)
		}
	} else {
		ev.Trace = []rulesdomain.TraceNode{}
	}

	if len(factsJSON) > 0 {
		dec := json.NewDecoder(bytes.NewReader(factsJSON))
		dec.UseNumber()
		if err := dec.Decode(&ev.FactsSnapshot); err != nil {
			return nil, fmt.Errorf("failed to decode facts JSON: %w", err)
		}
	} else {
		ev.FactsSnapshot = map[string]interface{}{}
	}

	return &ev, nil
}
