package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type FormSubmissionRepository interface {
	DB() *sql.DB

	Save(ctx context.Context, submission *FormSubmission) error
	SaveTx(ctx context.Context, tx *sql.Tx, submission *FormSubmission) error

	FindByID(ctx context.Context, tenantID, id uuid.UUID) (*FormSubmission, error)
	FindByIDTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) (*FormSubmission, error)
	FindByIDForUpdateTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) (*FormSubmission, error)

	FindByCaseAndFormVersion(ctx context.Context, tenantID, caseID, formVersionID uuid.UUID) (*FormSubmission, error)
	FindByCaseAndFormVersionTx(ctx context.Context, tx *sql.Tx, tenantID, caseID, formVersionID uuid.UUID) (*FormSubmission, error)
	FindByCaseAndFormVersionForUpdateTx(ctx context.Context, tx *sql.Tx, tenantID, caseID, formVersionID uuid.UUID) (*FormSubmission, error)

	ListByCase(ctx context.Context, tenantID, caseID uuid.UUID) ([]*FormSubmission, error)

	Update(ctx context.Context, submission *FormSubmission) error
	UpdateTx(ctx context.Context, tx *sql.Tx, submission *FormSubmission) error

	Delete(ctx context.Context, tenantID, id uuid.UUID) error
	DeleteTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) error
}
