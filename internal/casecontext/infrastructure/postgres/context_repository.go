package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alrazihi/civora/internal/casecontext/domain"
	"github.com/google/uuid"
)

type CaseContextRepository struct {
	db *sql.DB
}

func NewCaseContextRepository(db *sql.DB) *CaseContextRepository {
	return &CaseContextRepository{db: db}
}

func (r *CaseContextRepository) DB() *sql.DB {
	return r.db
}

func (r *CaseContextRepository) Save(ctx context.Context, c *domain.CaseContext) error {
	return r.save(ctx, r.db, c)
}

func (r *CaseContextRepository) SaveTx(ctx context.Context, tx *sql.Tx, c *domain.CaseContext) error {
	return r.save(ctx, tx, c)
}

func (r *CaseContextRepository) save(ctx context.Context, exec sqlExecutor, c *domain.CaseContext) error {
	factsJSON, err := json.Marshal(c.Facts)
	if err != nil {
		return fmt.Errorf("failed to marshal facts: %w", err)
	}

	authJSON, err := json.Marshal(c.Authorization)
	if err != nil {
		return fmt.Errorf("failed to marshal authorization: %w", err)
	}

	query := `
		INSERT INTO case_contexts (
			id, organization_id, case_id, actor_id, facts, token_estimate,
			created_at, expires_at, authorization
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			facts = EXCLUDED.facts,
			token_estimate = EXCLUDED.token_estimate,
			authorization = EXCLUDED.authorization
	`

	_, err = exec.ExecContext(ctx, query,
		c.ID, c.OrganizationID, c.CaseID, c.ActorID,
		factsJSON, c.TokenEstimate, c.CreatedAt, c.ExpiresAt, authJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to save case context: %w", err)
	}

	return nil
}

func (r *CaseContextRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.CaseContext, error) {
	query := `
		SELECT id, organization_id, case_id, actor_id, facts, token_estimate,
		       created_at, expires_at, authorization
		FROM case_contexts
		WHERE id = $1 AND organization_id = $2
	`

	row := r.db.QueryRowContext(ctx, query, id, orgID)
	return r.scanContext(row)
}

func (r *CaseContextRepository) FindLatestByCase(ctx context.Context, orgID, caseID uuid.UUID) (*domain.CaseContext, error) {
	query := `
		SELECT id, organization_id, case_id, actor_id, facts, token_estimate,
		       created_at, expires_at, authorization
		FROM case_contexts
		WHERE case_id = $1 AND organization_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, caseID, orgID)
	return r.scanContext(row)
}

func (r *CaseContextRepository) DeleteExpired(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `
		DELETE FROM case_contexts
		WHERE organization_id = $1 AND expires_at < NOW()
	`

	result, err := r.db.ExecContext(ctx, query, orgID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired contexts: %w", err)
	}

	count, err := result.RowsAffected()
	return int(count), err
}

func (r *CaseContextRepository) scanContext(scanner sqlScanner) (*domain.CaseContext, error) {
	var c domain.CaseContext
	var factsJSON, authJSON []byte

	err := scanner.Scan(
		&c.ID, &c.OrganizationID, &c.CaseID, &c.ActorID,
		&factsJSON, &c.TokenEstimate, &c.CreatedAt, &c.ExpiresAt, &authJSON,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrContextNotFound
		}
		return nil, fmt.Errorf("failed to scan context: %w", err)
	}

	if err := json.Unmarshal(factsJSON, &c.Facts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal facts: %w", err)
	}

	if err := json.Unmarshal(authJSON, &c.Authorization); err != nil {
		return nil, fmt.Errorf("failed to unmarshal authorization: %w", err)
	}

	return &c, nil
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type sqlScanner interface {
	Scan(dest ...interface{}) error
}
