package domain

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
)

type FormRepository interface {
	DB() *sql.DB
	SaveTx(ctx context.Context, tx *sql.Tx, form *Form) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Form, error)
	FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*Form, error)
	FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*Form, error)
	FindByKeyTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, key string) (*Form, error)
	List(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Form, int, error)
	Update(ctx context.Context, orgID, id uuid.UUID, form *Form) error
	UpdateTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, form *Form) error
}

type FormVersionRepository interface {
	Save(ctx context.Context, tx *sql.Tx, version *FormVersion) error
	SaveTx(ctx context.Context, tx *sql.Tx, version *FormVersion) error
	FindLatest(ctx context.Context, orgID, formID uuid.UUID) (*FormVersion, error)
	FindLatestTx(ctx context.Context, tx *sql.Tx, orgID, formID uuid.UUID) (*FormVersion, error)
	FindActiveVersion(ctx context.Context, orgID, formID uuid.UUID) (*FormVersion, error)
	FindActiveVersionTx(ctx context.Context, tx *sql.Tx, orgID, formID uuid.UUID) (*FormVersion, error)
	FindByID(ctx context.Context, orgID, versionID uuid.UUID) (*FormVersion, error)
	FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID) (*FormVersion, error)
	ListByFormID(ctx context.Context, orgID, formID uuid.UUID) ([]*FormVersion, error)
}

type FormFieldRepository interface {
	Save(ctx context.Context, tx *sql.Tx, field *FormField) error
	SaveTx(ctx context.Context, tx *sql.Tx, field *FormField) error
	FindByVersion(ctx context.Context, orgID, versionID uuid.UUID) ([]*FormField, error)
	FindByVersionTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID) ([]*FormField, error)
	FindByID(ctx context.Context, orgID, fieldID uuid.UUID) (*FormField, error)
	FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, fieldID uuid.UUID) (*FormField, error)
	UpdateTx(ctx context.Context, tx *sql.Tx, field *FormField) error
	DeleteTx(ctx context.Context, tx *sql.Tx, orgID, fieldID uuid.UUID) error
	UpdateOrderTx(ctx context.Context, tx *sql.Tx, orgID, versionID uuid.UUID, fields []*FormField) error
}
