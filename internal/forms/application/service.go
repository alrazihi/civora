package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	"github.com/alrazihi/civora/internal/forms/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
)

var (
	ErrFormNotFound         = errors.New("form not found")
	ErrFormInvalidInput     = errors.New("invalid form input")
	ErrFormKeyExists        = errors.New("form key already exists")
	ErrFormVersionNotFound  = errors.New("form version not found")
	ErrFormFieldNotFound    = errors.New("form field not found")
	ErrFormInvalidStatus    = errors.New("invalid form status transition")
	ErrFormVersionStatus    = errors.New("invalid form version status")
	ErrFormFieldTypeInvalid = errors.New("invalid field type")
	ErrFormFieldDuplicate   = errors.New("duplicate field key")
	ErrFormFieldInvalid     = errors.New("invalid field input")
	ErrFormOptionInvalid    = errors.New("invalid option")
)

type FormService struct {
	repo        domain.FormRepository
	versionRepo domain.FormVersionRepository
	fieldRepo   domain.FormFieldRepository
	auditor     auditdomain.EventRecorder
}

func NewFormService(
	repo domain.FormRepository,
	versionRepo domain.FormVersionRepository,
	fieldRepo domain.FormFieldRepository,
	auditor auditdomain.EventRecorder,
) *FormService {
	return &FormService{
		repo:        repo,
		versionRepo: versionRepo,
		fieldRepo:   fieldRepo,
		auditor:     auditor,
	}
}

type CreateFormParams struct {
	OrganizationID uuid.UUID
	Key            string
	Name           string
	Description    string
	CreatedByID    uuid.UUID
}

func (s *FormService) CreateForm(ctx context.Context, params CreateFormParams) (*domain.Form, error) {
	if err := domain.ValidateFormKey(params.Key); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormInvalidInput, err)
	}
	if err := domain.ValidateFormName(params.Name); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormInvalidInput, err)
	}
	if err := domain.ValidateFormDescription(params.Description); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormInvalidInput, err)
	}

	form, err := domain.NewForm(params.OrganizationID, params.Key, params.Name, params.Description, params.CreatedByID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormInvalidInput, err)
	}

	var savedForm *domain.Form
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		existing, err := s.repo.FindByKeyTx(ctx, tx, params.OrganizationID, params.Key)
		if err != nil && !errors.Is(err, domain.ErrFormNotFound) {
			return fmt.Errorf("failed to check duplicate key: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("%w: key %s", ErrFormKeyExists, params.Key)
		}

		if err := s.repo.SaveTx(ctx, tx, form); err != nil {
			return fmt.Errorf("failed to save form: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: form.OrganizationID,
				ActorID:        &form.CreatedByID,
				Action:         "form.created",
				Resource:       "form",
				ResourceID:     shared.StrPtr(form.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":  form.Key,
					"name": form.Name,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		savedForm = form
		return nil
	})
	if err != nil {
		return nil, err
	}

	return savedForm, nil
}

type UpdateFormParams struct {
	OrganizationID uuid.UUID
	ID             uuid.UUID
	Name           string
	Description    string
	ActorID        uuid.UUID
}

func (s *FormService) UpdateForm(ctx context.Context, params UpdateFormParams) (*domain.Form, error) {
	form, err := s.repo.FindByID(ctx, params.OrganizationID, params.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormNotFound, err)
	}

	if form.Status == domain.FormStatusArchived {
		return nil, fmt.Errorf("%w: form is already archived", ErrFormInvalidStatus)
	}

	if params.Name != "" {
		if err := domain.ValidateFormName(params.Name); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFormInvalidInput, err)
		}
		form.Name = params.Name
	}
	if params.Description != "" {
		if err := domain.ValidateFormDescription(params.Description); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFormInvalidInput, err)
		}
		form.Description = params.Description
	}
	form.UpdatedAt = time.Now().UTC()

	var updatedForm *domain.Form
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateTx(ctx, tx, params.OrganizationID, params.ID, form); err != nil {
			return fmt.Errorf("failed to update form: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: form.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "form.updated",
				Resource:       "form",
				ResourceID:     shared.StrPtr(form.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		updatedForm = form
		return nil
	})
	if err != nil {
		return nil, err
	}

	return updatedForm, nil
}

type CreateVersionParams struct {
	OrganizationID uuid.UUID
	FormID         uuid.UUID
	ActorID        uuid.UUID
}

func (s *FormService) CreateVersion(ctx context.Context, params CreateVersionParams) (*domain.FormVersion, error) {
	form, err := s.repo.FindByID(ctx, params.OrganizationID, params.FormID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormNotFound, err)
	}

	existingVersion, err := s.versionRepo.FindLatest(ctx, params.OrganizationID, params.FormID)
	if err != nil && !errors.Is(err, domain.ErrFormVersionNotFound) {
		return nil, fmt.Errorf("failed to find latest version: %w", err)
	}

	version := domain.NewFormVersion(params.FormID, params.OrganizationID, params.ActorID)
	if existingVersion != nil {
		version.Version = existingVersion.Version + 1
	}

	var savedVersion *domain.FormVersion
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.versionRepo.SaveTx(ctx, tx, version); err != nil {
			return fmt.Errorf("failed to save version: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: form.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "form.version_created",
				Resource:       "form",
				ResourceID:     shared.StrPtr(form.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"version": version.Version,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		savedVersion = version
		return nil
	})
	if err != nil {
		return nil, err
	}

	return savedVersion, nil
}

type PublishVersionParams struct {
	OrganizationID uuid.UUID
	VersionID      uuid.UUID
	ActorID        uuid.UUID
}

func (s *FormService) PublishVersion(ctx context.Context, params PublishVersionParams) (*domain.FormVersion, error) {
	version, err := s.versionRepo.FindByID(ctx, params.OrganizationID, params.VersionID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormVersionNotFound, err)
	}

	form, err := s.repo.FindByID(ctx, params.OrganizationID, version.FormID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormNotFound, err)
	}

	if version.Status != domain.FormVersionStatusDraft {
		return nil, fmt.Errorf("%w: only draft versions can be published", ErrFormVersionStatus)
	}

	if err := domain.ValidateVersionStatusTransition(version.Status, domain.FormVersionStatusPublished); err != nil {
		return nil, err
	}

	activeVersion, err := s.versionRepo.FindActiveVersion(ctx, params.OrganizationID, version.FormID)
	if err != nil && !errors.Is(err, domain.ErrFormVersionNotFound) {
		return nil, fmt.Errorf("failed to check active version: %w", err)
	}
	if activeVersion != nil && activeVersion.Version > version.Version {
		return nil, fmt.Errorf("%w: cannot publish older version when a newer version is already published", ErrFormVersionStatus)
	}

	version.Status = domain.FormVersionStatusPublished
	now := time.Now().UTC()
	version.PublishedAt = &now
	version.UpdatedAt = now

	form.Status = domain.FormStatusActive
	form.UpdatedAt = now

	var publishedVersion *domain.FormVersion
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.versionRepo.SaveTx(ctx, tx, version); err != nil {
			return fmt.Errorf("failed to save published version: %w", err)
		}
		if err := s.repo.UpdateTx(ctx, tx, params.OrganizationID, version.FormID, form); err != nil {
			return fmt.Errorf("failed to update form status: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: form.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "form.published",
				Resource:       "form",
				ResourceID:     shared.StrPtr(form.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"version": version.Version,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		publishedVersion = version
		return nil
	})
	if err != nil {
		return nil, err
	}

	return publishedVersion, nil
}

type ArchiveFormParams struct {
	OrganizationID uuid.UUID
	FormID         uuid.UUID
	ActorID        uuid.UUID
}

func (s *FormService) ArchiveForm(ctx context.Context, params ArchiveFormParams) (*domain.Form, error) {
	form, err := s.repo.FindByID(ctx, params.OrganizationID, params.FormID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormNotFound, err)
	}

	if form.Status == domain.FormStatusArchived {
		return nil, fmt.Errorf("%w: form is already archived", ErrFormInvalidStatus)
	}

	if err := domain.ValidateFormStatusTransition(form.Status, domain.FormStatusArchived); err != nil {
		return nil, err
	}

	form.Status = domain.FormStatusArchived
	form.UpdatedAt = time.Now().UTC()

	var archivedForm *domain.Form
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateTx(ctx, tx, params.OrganizationID, params.FormID, form); err != nil {
			return fmt.Errorf("failed to archive form: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: form.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "form.archived",
				Resource:       "form",
				ResourceID:     shared.StrPtr(form.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		archivedForm = form
		return nil
	})
	if err != nil {
		return nil, err
	}

	return archivedForm, nil
}

type GetFormParams struct {
	OrganizationID uuid.UUID
	FormID         uuid.UUID
}

func (s *FormService) GetForm(ctx context.Context, params GetFormParams) (*domain.Form, error) {
	form, err := s.repo.FindByID(ctx, params.OrganizationID, params.FormID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormNotFound, err)
	}

	versions, err := s.versionRepo.ListByFormID(ctx, params.OrganizationID, params.FormID)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	for _, v := range versions {
		v.Fields, err = s.fieldRepo.FindByVersion(ctx, params.OrganizationID, v.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list fields: %w", err)
		}
		form.Versions = append(form.Versions, v)
	}

	return form, nil
}

type ListFormsParams struct {
	OrganizationID uuid.UUID
	Limit          int
	Offset         int
}

func (s *FormService) ListForms(ctx context.Context, params ListFormsParams) ([]*domain.Form, int, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	forms, total, err := s.repo.List(ctx, params.OrganizationID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list forms: %w", err)
	}

	return forms, total, nil
}

type AddFieldParams struct {
	OrganizationID uuid.UUID
	FormID         uuid.UUID
	VersionID      uuid.UUID
	Key            string
	Label          string
	Type           domain.FieldType
	Required       bool
	Description    string
	Placeholder    string
	DefaultValue   *string
	Validation     map[string]any
	Options        []domain.FormOption
	Order          int
	ActorID        uuid.UUID
}

func (s *FormService) AddField(ctx context.Context, params AddFieldParams) (*domain.FormField, error) {
	form, err := s.repo.FindByID(ctx, params.OrganizationID, params.FormID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormNotFound, err)
	}

	version, err := s.versionRepo.FindByID(ctx, params.OrganizationID, params.VersionID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormVersionNotFound, err)
	}

	if version.Status != domain.FormVersionStatusDraft {
		return nil, fmt.Errorf("%w: cannot add fields to non-draft version", ErrFormVersionStatus)
	}

	if err := domain.ValidateFieldKey(params.Key); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormFieldInvalid, err)
	}

	if err := domain.ValidateField(params.Type, params.Label, params.Required, params.Options, params.Validation); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormFieldInvalid, err)
	}

	existingFields, err := s.fieldRepo.FindByVersion(ctx, params.OrganizationID, params.VersionID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing fields: %w", err)
	}
	for _, f := range existingFields {
		if f.Key == params.Key {
			return nil, fmt.Errorf("%w: key %s", ErrFormFieldDuplicate, params.Key)
		}
	}

	field := domain.NewFormField(params.FormID, params.VersionID, params.OrganizationID, params.Key, params.Label, params.Type, params.Required, params.Description, params.Placeholder, params.DefaultValue, params.Validation, params.Options, params.Order)

	var savedField *domain.FormField
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.fieldRepo.SaveTx(ctx, tx, field); err != nil {
			return fmt.Errorf("failed to save field: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: form.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "field.created",
				Resource:       "form",
				ResourceID:     shared.StrPtr(form.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"field_key":  field.Key,
					"field_type": field.Type,
					"version":    version.Version,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		savedField = field
		return nil
	})
	if err != nil {
		return nil, err
	}

	return savedField, nil
}

type UpdateFieldParams struct {
	OrganizationID uuid.UUID
	FieldID        uuid.UUID
	Label          string
	Required       bool
	Description    string
	Placeholder    string
	DefaultValue   *string
	Validation     map[string]any
	Options        []domain.FormOption
	ActorID        uuid.UUID
}

func (s *FormService) UpdateField(ctx context.Context, params UpdateFieldParams) (*domain.FormField, error) {
	field, err := s.fieldRepo.FindByID(ctx, params.OrganizationID, params.FieldID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormFieldNotFound, err)
	}

	version, err := s.versionRepo.FindByID(ctx, params.OrganizationID, field.FormVersionID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormVersionNotFound, err)
	}

	if version.Status != domain.FormVersionStatusDraft {
		return nil, fmt.Errorf("%w: cannot modify fields in non-draft version", ErrFormVersionStatus)
	}

	if params.Label != "" {
		if err := domain.ValidateFieldLabel(params.Label); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFormFieldInvalid, err)
		}
		field.Label = params.Label
	}
	field.Required = params.Required
	if params.Description != "" {
		field.Description = params.Description
	}
	if params.Placeholder != "" {
		field.Placeholder = params.Placeholder
	}
	field.DefaultValue = params.DefaultValue
	if params.Validation != nil {
		if err := domain.ValidateFieldValidation(field.Type, params.Validation); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFormFieldInvalid, err)
		}
		field.Validation = params.Validation
	}
	if params.Options != nil {
		if err := domain.ValidateFieldOptions(field.Type, params.Options); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFormOptionInvalid, err)
		}
		field.Options = params.Options
	}
	field.UpdatedAt = time.Now().UTC()

	var updatedField *domain.FormField
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.fieldRepo.UpdateTx(ctx, tx, field); err != nil {
			return fmt.Errorf("failed to update field: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: field.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "field.updated",
				Resource:       "form",
				ResourceID:     shared.StrPtr(field.FormID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"field_key":  field.Key,
					"field_type": field.Type,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		updatedField = field
		return nil
	})
	if err != nil {
		return nil, err
	}

	return updatedField, nil
}

type DeleteFieldParams struct {
	OrganizationID uuid.UUID
	FieldID        uuid.UUID
	ActorID        uuid.UUID
}

func (s *FormService) DeleteField(ctx context.Context, params DeleteFieldParams) (*domain.FormField, error) {
	field, err := s.fieldRepo.FindByID(ctx, params.OrganizationID, params.FieldID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormFieldNotFound, err)
	}

	version, err := s.versionRepo.FindByID(ctx, params.OrganizationID, field.FormVersionID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormVersionNotFound, err)
	}

	if version.Status != domain.FormVersionStatusDraft {
		return nil, fmt.Errorf("%w: cannot delete fields from non-draft version", ErrFormVersionStatus)
	}

	var deletedField *domain.FormField
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.fieldRepo.DeleteTx(ctx, tx, params.OrganizationID, params.FieldID); err != nil {
			return fmt.Errorf("failed to delete field: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: field.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "field.deleted",
				Resource:       "form",
				ResourceID:     shared.StrPtr(field.FormID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"field_key": field.Key,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		deletedField = field
		return nil
	})
	if err != nil {
		return nil, err
	}

	return deletedField, nil
}

type GetActiveVersionParams struct {
	OrganizationID uuid.UUID
	FormID         uuid.UUID
}

func (s *FormService) GetActiveVersion(ctx context.Context, params GetActiveVersionParams) (*domain.FormVersion, error) {
	version, err := s.versionRepo.FindActiveVersion(ctx, params.OrganizationID, params.FormID)
	if err != nil {
		return nil, fmt.Errorf("failed to find active version: %w", err)
	}
	if version == nil {
		return nil, ErrFormVersionNotFound
	}
	return version, nil
}
