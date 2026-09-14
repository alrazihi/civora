package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alrazihi/civora/internal/rules/domain"
	"github.com/google/uuid"
)

type RuleTemplateRepository struct {
	db *sql.DB
}

func NewRuleTemplateRepository(db *sql.DB) *RuleTemplateRepository {
	return &RuleTemplateRepository{db: db}
}

func (r *RuleTemplateRepository) DB() *sql.DB {
	return r.db
}

func (r *RuleTemplateRepository) SaveTx(ctx context.Context, tx *sql.Tx, rt *domain.RuleTemplate) error {
	var orgID *string
	if rt.OrganizationID != nil {
		id := rt.OrganizationID.String()
		orgID = &id
	}
	ruleJSON, err := domain.MarshalRuleTemplateRule(rt.Rule)
	if err != nil {
		return fmt.Errorf("failed to marshal rule: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO rules.rule_templates (
			id, scope, organization_id, key, name, description, category, rule_json, created_by, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, rt.ID, string(rt.Scope), orgID, rt.Key, rt.Name, rt.Description, rt.Category, ruleJSON, rt.CreatedBy, rt.CreatedAt, rt.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to save rule template: %w", err)
	}
	return nil
}

func (r *RuleTemplateRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.RuleTemplate, error) {
	var rt domain.RuleTemplate
	var orgIDStr *string
	var ruleJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, scope, organization_id, key, name, description, category, rule_json, created_by, created_at, updated_at
		FROM rules.rule_templates
		WHERE id = $1 AND (organization_id = $2 OR organization_id IS NULL)
	`, id, orgID).Scan(
		&rt.ID, &rt.Scope, &orgIDStr, &rt.Key, &rt.Name, &rt.Description, &rt.Category, &ruleJSON, &rt.CreatedBy, &rt.CreatedAt, &rt.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrRuleSetNotFound
		}
		return nil, fmt.Errorf("failed to find rule template: %w", err)
	}
	if orgIDStr != nil {
		id, _ := uuid.Parse(*orgIDStr)
		rt.OrganizationID = &id
	}
	rule, err := domain.UnmarshalRuleTemplateRule(ruleJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal rule: %w", err)
	}
	rt.Rule = rule
	return &rt, nil
}

func (r *RuleTemplateRepository) FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*domain.RuleTemplate, error) {
	var rt domain.RuleTemplate
	var orgIDStr *string
	var ruleJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, scope, organization_id, key, name, description, category, rule_json, created_by, created_at, updated_at
		FROM rules.rule_templates
		WHERE key = $1 AND (organization_id = $2 OR organization_id IS NULL)
	`, key, orgID).Scan(
		&rt.ID, &rt.Scope, &orgIDStr, &rt.Key, &rt.Name, &rt.Description, &rt.Category, &ruleJSON, &rt.CreatedBy, &rt.CreatedAt, &rt.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrRuleSetNotFound
		}
		return nil, fmt.Errorf("failed to find rule template by key: %w", err)
	}
	if orgIDStr != nil {
		id, _ := uuid.Parse(*orgIDStr)
		rt.OrganizationID = &id
	}
	rule, err := domain.UnmarshalRuleTemplateRule(ruleJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal rule: %w", err)
	}
	rt.Rule = rule
	return &rt, nil
}

func (r *RuleTemplateRepository) List(ctx context.Context, orgID uuid.UUID, scope domain.RuleTemplateScope, category string, limit, offset int) ([]*domain.RuleTemplate, int, error) {
	where := "WHERE (organization_id = $1 OR organization_id IS NULL)"
	args := []interface{}{orgID}
	argIdx := 2

	if scope != "" {
		where += fmt.Sprintf(" AND scope = $%d", argIdx)
		args = append(args, string(scope))
		argIdx++
	}
	if category != "" {
		where += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, category)
		argIdx++
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM rules.rule_templates %s", where)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count rule templates: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, scope, organization_id, key, name, description, category, rule_json, created_by, created_at, updated_at
		FROM rules.rule_templates
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list rule templates: %w", err)
	}
	defer rows.Close()

	templates := make([]*domain.RuleTemplate, 0)
	for rows.Next() {
		var rt domain.RuleTemplate
		var orgIDStr *string
		var ruleJSON []byte
		if err := rows.Scan(
			&rt.ID, &rt.Scope, &orgIDStr, &rt.Key, &rt.Name, &rt.Description, &rt.Category, &ruleJSON, &rt.CreatedBy, &rt.CreatedAt, &rt.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan rule template: %w", err)
		}
		if orgIDStr != nil {
			id, _ := uuid.Parse(*orgIDStr)
			rt.OrganizationID = &id
		}
		rule, err := domain.UnmarshalRuleTemplateRule(ruleJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal rule: %w", err)
		}
		rt.Rule = rule
		templates = append(templates, &rt)
	}
	return templates, total, nil
}

func (r *RuleTemplateRepository) ListGlobal(ctx context.Context, category string, limit, offset int) ([]*domain.RuleTemplate, int, error) {
	where := "WHERE organization_id IS NULL"
	args := []interface{}{}
	argIdx := 1

	if category != "" {
		where += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, category)
		argIdx++
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM rules.rule_templates %s", where)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count global rule templates: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, scope, organization_id, key, name, description, category, rule_json, created_by, created_at, updated_at
		FROM rules.rule_templates
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list global rule templates: %w", err)
	}
	defer rows.Close()

	templates := make([]*domain.RuleTemplate, 0)
	for rows.Next() {
		var rt domain.RuleTemplate
		var orgIDStr *string
		var ruleJSON []byte
		if err := rows.Scan(
			&rt.ID, &rt.Scope, &orgIDStr, &rt.Key, &rt.Name, &rt.Description, &rt.Category, &ruleJSON, &rt.CreatedBy, &rt.CreatedAt, &rt.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan rule template: %w", err)
		}
		rule, err := domain.UnmarshalRuleTemplateRule(ruleJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal rule: %w", err)
		}
		rt.Rule = rule
		templates = append(templates, &rt)
	}
	return templates, total, nil
}

func (r *RuleTemplateRepository) DeleteTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM rules.rule_templates WHERE id = $1 AND (organization_id = $2 OR organization_id IS NULL)
	`, id, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete rule template: %w", err)
	}
	return nil
}

func (r *RuleTemplateRepository) CountByKey(ctx context.Context, orgID uuid.UUID, key string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM rules.rule_templates WHERE key = $1 AND (organization_id = $2 OR organization_id IS NULL)
	`, key, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count rule templates by key: %w", err)
	}
	return count, nil
}
