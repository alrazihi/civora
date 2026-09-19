package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type RetentionPolicy struct {
	DefaultRetentionDays       int
	MinRetentionDays           int
	EvidenceRetentionDays      int
	ObservationRetentionDays   int
	SummarizationRetentionDays int
}

func NewDefaultRetentionPolicy() *RetentionPolicy {
	return &RetentionPolicy{
		DefaultRetentionDays:       365 * 5,
		MinRetentionDays:           30,
		EvidenceRetentionDays:      365 * 7,
		ObservationRetentionDays:   365 * 2,
		SummarizationRetentionDays: 365,
	}
}

type RetentionRecord struct {
	RecordID       uuid.UUID
	RecordType     string
	OrganizationID uuid.UUID
	RecordedAt     time.Time
	ExpiresAt      time.Time
	IsExpired      bool
}

type RetentionService interface {
	GetRetentionEpoch(ctx context.Context, orgID uuid.UUID) time.Time
	MarkForDeletion(ctx context.Context, record RetentionRecord) error
	IsDeleted(ctx context.Context, record RetentionRecord) (bool, error)
	DeleteExpired(ctx context.Context, orgID uuid.UUID, maxItems int) ([]uuid.UUID, error)
}

type RetentionStatus string

const (
	RetentionStatusActive          RetentionStatus = "ACTIVE"
	RetentionStatusPendingDeletion RetentionStatus = "PENDING_DELETION"
	RetentionStatusDeleted         RetentionStatus = "DELETED"
	RetentionStatusArchived        RetentionStatus = "ARCHIVED"
)

func (r RetentionRecord) AgeInDays(now time.Time) int {
	return int(now.Sub(r.RecordedAt).Hours() / 24)
}
