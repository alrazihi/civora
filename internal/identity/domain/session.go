package domain

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSessionNotFound = errors.New("session not found")
)

type Session struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	OrganizationID   uuid.UUID
	RefreshTokenHash string
	TokenFamily      uuid.UUID
	UserModifiedAt   time.Time
	IssuedAt         time.Time
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	LastUsedAt       *time.Time
	UserAgent        *string
	IPAddress        *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

type SessionRepository interface {
	DB() *sql.DB
	Create(ctx context.Context, session *Session) error
	CreateTx(ctx context.Context, tx *sql.Tx, session *Session) error
	Update(ctx context.Context, session *Session) error
	UpdateTx(ctx context.Context, tx *sql.Tx, session *Session) error
	FindByRefreshTokenHash(ctx context.Context, hash string) (*Session, error)
	FindByID(ctx context.Context, sessionID string) (*Session, error)
	FindByIDForOrganization(ctx context.Context, sessionID string, organizationID uuid.UUID) (*Session, error)
	FindActiveByUserID(ctx context.Context, userID uuid.UUID) ([]*Session, error)
	FindActiveByUserIDForOrganization(ctx context.Context, userID uuid.UUID, organizationID uuid.UUID) ([]*Session, error)
	Revoke(ctx context.Context, sessionID string) error
	RevokeForOrganization(ctx context.Context, sessionID string, organizationID uuid.UUID) error
	RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error
	RevokeAllByUserIDForOrganization(ctx context.Context, userID uuid.UUID, organizationID uuid.UUID) error
	RevokeAllByUserIDTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error
	RevokeAllByUserIDTxForOrganization(ctx context.Context, tx *sql.Tx, userID uuid.UUID, organizationID uuid.UUID) error
	MarkUsed(ctx context.Context, sessionID string) error
	MarkUsedForOrganization(ctx context.Context, sessionID string, organizationID uuid.UUID) error
}
