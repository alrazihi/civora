package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type RuleSetRepository interface {
	DB() *sql.DB
	SaveTx(ctx context.Context, tx *sql.Tx, rs *RuleSet) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*RuleSet, error)
	FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*RuleSet, error)
	FindByIDForUpdateTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*RuleSet, error)
	FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*RuleSet, error)
	FindByKeyAndVersion(ctx context.Context, orgID uuid.UUID, key string, version int) (*RuleSet, error)
	List(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*RuleSet, int, error)
	ListByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*RuleSet, int, error)
	ListVersions(ctx context.Context, orgID uuid.UUID, key string, limit, offset int) ([]*RuleSet, int, error)
	FindPublished(ctx context.Context, orgID uuid.UUID, key string, caseID *uuid.UUID) (*RuleSet, error)
	UpdateTx(ctx context.Context, tx *sql.Tx, rs *RuleSet) error
	UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, from, to RuleSetStatus) error
	DeleteTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) error
	CountByKey(ctx context.Context, orgID uuid.UUID, key string) (int, error)
}

type RuleTemplateRepository interface {
	DB() *sql.DB
	SaveTx(ctx context.Context, tx *sql.Tx, rt *RuleTemplate) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*RuleTemplate, error)
	FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*RuleTemplate, error)
	List(ctx context.Context, orgID uuid.UUID, scope RuleTemplateScope, category string, limit, offset int) ([]*RuleTemplate, int, error)
	ListGlobal(ctx context.Context, category string, limit, offset int) ([]*RuleTemplate, int, error)
	DeleteTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) error
	CountByKey(ctx context.Context, orgID uuid.UUID, key string) (int, error)
}

type EvaluationRepository interface {
	DB() *sql.DB
	SaveTx(ctx context.Context, tx *sql.Tx, ev *Evaluation) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Evaluation, error)
	ListByRuleSet(ctx context.Context, orgID, ruleSetID uuid.UUID, limit, offset int) ([]*Evaluation, int, error)
	ListByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*Evaluation, int, error)
}
