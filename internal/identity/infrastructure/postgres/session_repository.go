package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/google/uuid"
)

type PostgresSessionRepository struct {
	db *sql.DB
}

func NewPostgresSessionRepository(db *sql.DB) *PostgresSessionRepository {
	return &PostgresSessionRepository{db: db}
}

func (r *PostgresSessionRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresSessionRepository) Create(ctx context.Context, session *domain.Session) error {
	return r.createSession(ctx, r.db, session)
}

func (r *PostgresSessionRepository) CreateTx(ctx context.Context, tx *sql.Tx, session *domain.Session) error {
	return r.createSession(ctx, tx, session)
}

func (r *PostgresSessionRepository) Update(ctx context.Context, session *domain.Session) error {
	return r.updateSession(ctx, r.db, session)
}

func (r *PostgresSessionRepository) UpdateTx(ctx context.Context, tx *sql.Tx, session *domain.Session) error {
	return r.updateSession(ctx, tx, session)
}

func (r *PostgresSessionRepository) createSession(ctx context.Context, e sqlExecer, session *domain.Session) error {
	query := `
		INSERT INTO auth_sessions (
			id, user_id, organization_id, refresh_token_hash, token_family,
			user_modified_at, issued_at, expires_at, revoked_at, last_used_at,
			user_agent, ip_address, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := e.ExecContext(ctx, query,
		session.ID,
		session.UserID,
		session.OrganizationID,
		session.RefreshTokenHash,
		session.TokenFamily,
		session.UserModifiedAt,
		session.IssuedAt,
		session.ExpiresAt,
		session.RevokedAt,
		session.LastUsedAt,
		session.UserAgent,
		session.IPAddress,
		session.CreatedAt,
		session.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert session: %w", err)
	}
	return nil
}

func (r *PostgresSessionRepository) updateSession(ctx context.Context, e sqlExecer, session *domain.Session) error {
	query := `
		UPDATE auth_sessions
		SET refresh_token_hash = $1, token_family = $2, expires_at = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := e.ExecContext(ctx, query,
		session.RefreshTokenHash,
		session.TokenFamily,
		session.ExpiresAt,
		session.UpdatedAt,
		session.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	return nil
}

func (r *PostgresSessionRepository) FindByRefreshTokenHash(ctx context.Context, hash string) (*domain.Session, error) {
	query := `
		SELECT id, user_id, organization_id, refresh_token_hash, token_family,
			user_modified_at, issued_at, expires_at, revoked_at, last_used_at,
			user_agent, ip_address, created_at, updated_at
		FROM auth_sessions
		WHERE refresh_token_hash = $1
		ORDER BY issued_at DESC
		LIMIT 1
	`
	return r.scanSession(r.db.QueryRowContext(ctx, query, hash))
}

func (r *PostgresSessionRepository) FindByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	query := `
		SELECT id, user_id, organization_id, refresh_token_hash, token_family,
			user_modified_at, issued_at, expires_at, revoked_at, last_used_at,
			user_agent, ip_address, created_at, updated_at
		FROM auth_sessions
		WHERE id = $1
	`
	return r.scanSession(r.db.QueryRowContext(ctx, query, sessionID))
}

func (r *PostgresSessionRepository) FindActiveByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Session, error) {
	query := `
		SELECT id, user_id, organization_id, refresh_token_hash, token_family,
			user_modified_at, issued_at, expires_at, revoked_at, last_used_at,
			user_agent, ip_address, created_at, updated_at
		FROM auth_sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
		ORDER BY issued_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []*domain.Session
	for rows.Next() {
		s, err := r.scanSessionFromRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return sessions, nil
}

func (r *PostgresSessionRepository) Revoke(ctx context.Context, sessionID string) error {
	query := `
		UPDATE auth_sessions
		SET revoked_at = now(), updated_at = now()
		WHERE id = $1 AND revoked_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}
	return nil
}

func (r *PostgresSessionRepository) RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE auth_sessions
		SET revoked_at = now(), updated_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL
	`
	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all sessions: %w", err)
	}
	return nil
}

func (r *PostgresSessionRepository) RevokeAllByUserIDTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	query := `
		UPDATE auth_sessions
		SET revoked_at = now(), updated_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL
	`
	_, err := tx.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all sessions in tx: %w", err)
	}
	return nil
}

func (r *PostgresSessionRepository) MarkUsed(ctx context.Context, sessionID string) error {
	query := `
		UPDATE auth_sessions
		SET last_used_at = now(), updated_at = now()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to mark session used: %w", err)
	}
	return nil
}

func (r *PostgresSessionRepository) scanSession(row interface {
	Scan(dest ...any) error
}) (*domain.Session, error) {
	var s domain.Session
	if err := row.Scan(
		&s.ID,
		&s.UserID,
		&s.OrganizationID,
		&s.RefreshTokenHash,
		&s.TokenFamily,
		&s.UserModifiedAt,
		&s.IssuedAt,
		&s.ExpiresAt,
		&s.RevokedAt,
		&s.LastUsedAt,
		&s.UserAgent,
		&s.IPAddress,
		&s.CreatedAt,
		&s.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to scan session: %w", err)
	}
	return &s, nil
}

func (r *PostgresSessionRepository) scanSessionFromRows(rows *sql.Rows) (*domain.Session, error) {
	var s domain.Session
	if err := rows.Scan(
		&s.ID,
		&s.UserID,
		&s.OrganizationID,
		&s.RefreshTokenHash,
		&s.TokenFamily,
		&s.UserModifiedAt,
		&s.IssuedAt,
		&s.ExpiresAt,
		&s.RevokedAt,
		&s.LastUsedAt,
		&s.UserAgent,
		&s.IPAddress,
		&s.CreatedAt,
		&s.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan session: %w", err)
	}
	return &s, nil
}
