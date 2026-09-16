package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type EvidenceRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, e *Evidence) error
	SaveTx(ctx context.Context, tx *sql.Tx, e *Evidence) error
	UpdateVerification(ctx context.Context, e *Evidence) error
	UpdateVerificationTx(ctx context.Context, tx *sql.Tx, e *Evidence) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Evidence, error)
	FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*Evidence, error)
	CountByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (int, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Evidence, error)

	SaveVerificationHistory(ctx context.Context, rec *VerificationRecord) error
	SaveVerificationHistoryTx(ctx context.Context, tx *sql.Tx, rec *VerificationRecord) error
	FindVerificationHistory(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*VerificationRecord, error)
	CountVerificationHistory(ctx context.Context, orgID, evidenceID uuid.UUID) (int, error)

	SaveDocument(ctx context.Context, d *Document) error
	SaveDocumentTx(ctx context.Context, tx *sql.Tx, d *Document) error
	FindDocumentByID(ctx context.Context, orgID, id uuid.UUID) (*Document, error)
	FindDocumentByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID) (*Document, error)
	ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*Document, int, error)
	CountDocuments(ctx context.Context, orgID uuid.UUID) (int, error)
	DeleteDocument(ctx context.Context, orgID, id uuid.UUID) error
}
