package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	casedomain "github.com/alrazihi/civora/internal/cases/domain"
	formdomain "github.com/alrazihi/civora/internal/forms/domain"
	assignmentdomain "github.com/alrazihi/civora/internal/workflow_form_assignment/domain"
	submissiondomain "github.com/alrazihi/civora/internal/form_submission/domain"
	"github.com/alrazihi/civora/internal/database"
	"github.com/alrazihi/civora/internal/shared"
	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
)

var (
	ErrCaseNotFound            = fmt.Errorf("case not found")
	ErrWorkflowInstanceNotFound = fmt.Errorf("workflow instance not found")
	ErrFormNotAssigned         = fmt.Errorf("form is not assigned to the current workflow state")
	ErrFormVersionMismatch     = fmt.Errorf("form version mismatch")
	ErrSubmissionExists        = fmt.Errorf("submission already exists for this case and form version")
	ErrUnknownFields           = fmt.Errorf("submission contains unknown fields")
	ErrFieldValidationFailed   = fmt.Errorf("field validation failed")
	ErrOptionValidationFailed  = fmt.Errorf("option validation failed")
	ErrInvalidSubmissionData   = fmt.Errorf("invalid submission data")
	ErrFormNotPublished        = fmt.Errorf("form version is not published")
	ErrFormArchived            = fmt.Errorf("form is archived")
)

type CaseFormService struct {
	caseRepo             casedomain.CaseRepository
	workflowDefRepo      workflowdomain.WorkflowDefinitionRepository
	workflowInstanceRepo workflowdomain.WorkflowInstanceRepository
	formRepo             formdomain.FormRepository
	formVersionRepo      formdomain.FormVersionRepository
	formFieldRepo        formdomain.FormFieldRepository
	assignmentRepo       assignmentdomain.WorkflowStateFormAssignmentRepository
	submissionRepo       submissiondomain.FormSubmissionRepository
	auditor              auditdomain.EventRecorder
}

func NewCaseFormService(
	caseRepo casedomain.CaseRepository,
	workflowDefRepo workflowdomain.WorkflowDefinitionRepository,
	workflowInstanceRepo workflowdomain.WorkflowInstanceRepository,
	formRepo formdomain.FormRepository,
	formVersionRepo formdomain.FormVersionRepository,
	formFieldRepo formdomain.FormFieldRepository,
	assignmentRepo assignmentdomain.WorkflowStateFormAssignmentRepository,
		submissionRepo       submissiondomain.FormSubmissionRepository,
	auditor auditdomain.EventRecorder,
) *CaseFormService {
	return &CaseFormService{
		caseRepo:             caseRepo,
		workflowDefRepo:      workflowDefRepo,
		workflowInstanceRepo: workflowInstanceRepo,
		formRepo:             formRepo,
		formVersionRepo:      formVersionRepo,
		formFieldRepo:        formFieldRepo,
		assignmentRepo:       assignmentRepo,
		submissionRepo:       submissionRepo,
		auditor:              auditor,
	}
}

type CaseFormAvailability struct {
	AssignmentID    uuid.UUID                       `json:"assignment_id"`
	FormID          uuid.UUID                       `json:"form_id"`
	FormVersionID   uuid.UUID                       `json:"form_version_id"`
	FormKey         string                          `json:"form_key"`
	FormName        string                          `json:"form_name"`
	FormDescription string                          `json:"form_description"`
	Version         int                             `json:"version"`
	Required        bool                            `json:"required"`
	DisplayOrder    int                             `json:"display_order"`
	Active          bool                            `json:"active"`
	Fields          []FormFieldAvailability         `json:"fields"`
}

type FormFieldAvailability struct {
	Key         string                           `json:"key"`
	Label       string                           `json:"label"`
	Type        formdomain.FieldType             `json:"type"`
	Required    bool                             `json:"required"`
	Placeholder string                           `json:"placeholder"`
	Options     []formdomain.FormOption          `json:"options,omitempty"`
	Validation  map[string]interface{}           `json:"validation,omitempty"`
}

type SubmissionResponse struct {
	ID            uuid.UUID                       `json:"id"`
	CaseID        uuid.UUID                       `json:"case_id"`
	FormID        uuid.UUID                       `json:"form_id"`
	FormVersionID uuid.UUID                       `json:"form_version_id"`
	Status        submissiondomain.SubmissionStatus `json:"status"`
	Data          map[string]interface{}           `json:"data"`
	SubmittedBy   uuid.UUID                       `json:"submitted_by"`
	SubmittedAt   time.Time                       `json:"submitted_at"`
	UpdatedAt     time.Time                       `json:"updated_at"`
}

type GetCaseFormsParams struct {
	TenantID uuid.UUID
	CaseID   uuid.UUID
}

type SubmitFormParams struct {
	TenantID      uuid.UUID
	CaseID        uuid.UUID
	FormVersionID uuid.UUID
	SubmittedBy   uuid.UUID
	Data          map[string]interface{}
}

type WorkflowRequirements struct {
	CaseID           uuid.UUID                        `json:"case_id"`
	CurrentState     string                           `json:"current_state"`
	TotalRequired    int                              `json:"total_required"`
	SubmittedCount   int                              `json:"submitted_count"`
	MissingRequired  []*CaseFormAvailability          `json:"missing_required"`
	SubmittedForms   []*SubmissionResponse            `json:"submitted_forms"`
	CanProceed       bool                             `json:"can_proceed"`
}

func (s *CaseFormService) GetCaseForms(ctx context.Context, params GetCaseFormsParams) ([]*CaseFormAvailability, error) {
	caseEntity, err := s.caseRepo.FindByID(ctx, params.TenantID, params.CaseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if caseEntity.WorkflowInstanceID == nil {
		return []*CaseFormAvailability{}, nil
	}

	instance, err := s.workflowInstanceRepo.FindByID(ctx, params.TenantID, *caseEntity.WorkflowInstanceID)
	if err != nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	assignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, params.TenantID, instance.WorkflowDefID, instance.CurrentState)
	if err != nil {
		return nil, fmt.Errorf("failed to get form assignments: %w", err)
	}

	var result []*CaseFormAvailability
	for _, assignment := range assignments {
		if !assignment.Active {
			continue
		}

		form, err := s.formRepo.FindByID(ctx, params.TenantID, assignment.FormID)
		if err != nil {
			continue
		}

		if form.Status == formdomain.FormStatusArchived {
			continue
		}

		formVersion, err := s.formVersionRepo.FindByID(ctx, params.TenantID, assignment.FormVersionID)
		if err != nil {
			continue
		}

		if formVersion.Status != formdomain.FormVersionStatusPublished {
			continue
		}

		fields, err := s.formFieldRepo.FindByVersion(ctx, params.TenantID, formVersion.ID)
		if err != nil {
			continue
		}

		fieldAvailabilities := make([]FormFieldAvailability, len(fields))
		for i, field := range fields {
			fieldAvailabilities[i] = FormFieldAvailability{
				Key:         field.Key,
				Label:       field.Label,
				Type:        field.Type,
				Required:    field.Required,
				Placeholder: field.Placeholder,
				Options:     field.Options,
				Validation:  field.Validation,
			}
		}

		submission, _ := s.submissionRepo.FindByCaseAndFormVersion(ctx, params.TenantID, params.CaseID, formVersion.ID)
		submissionStatus := ""
		if submission != nil {
			submissionStatus = string(submission.Status)
		}
		_ = submissionStatus

		result = append(result, &CaseFormAvailability{
			AssignmentID:    assignment.ID,
			FormID:          form.ID,
			FormVersionID:   formVersion.ID,
			FormKey:         form.Key,
			FormName:        form.Name,
			FormDescription: form.Description,
			Version:         formVersion.Version,
			Required:        assignment.Required,
			DisplayOrder:    assignment.DisplayOrder,
			Active:          assignment.Active,
			Fields:          fieldAvailabilities,
		})
	}

	return result, nil
}

func (s *CaseFormService) SubmitForm(ctx context.Context, params SubmitFormParams) (*SubmissionResponse, error) {
	caseEntity, err := s.caseRepo.FindByID(ctx, params.TenantID, params.CaseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if caseEntity.WorkflowInstanceID == nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	instance, err := s.workflowInstanceRepo.FindByID(ctx, params.TenantID, *caseEntity.WorkflowInstanceID)
	if err != nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	formVersion, err := s.formVersionRepo.FindByID(ctx, params.TenantID, params.FormVersionID)
	if err != nil {
		return nil, ErrFormNotPublished
	}

	if formVersion.Status != formdomain.FormVersionStatusPublished {
		return nil, ErrFormNotPublished
	}

	form, err := s.formRepo.FindByID(ctx, params.TenantID, formVersion.FormID)
	if err != nil {
		return nil, ErrFormNotPublished
	}

	if form.Status == formdomain.FormStatusArchived {
		return nil, ErrFormArchived
	}

	assignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, params.TenantID, instance.WorkflowDefID, instance.CurrentState)
	if err != nil {
		return nil, fmt.Errorf("failed to check form assignments: %w", err)
	}

	assignmentFound := false
	for _, assignment := range assignments {
		if assignment.FormVersionID == params.FormVersionID && assignment.Active {
			assignmentFound = true
			break
		}
	}
	if !assignmentFound {
		return nil, ErrFormNotAssigned
	}

	existingSubmission, err := s.submissionRepo.FindByCaseAndFormVersion(ctx, params.TenantID, params.CaseID, params.FormVersionID)
	if err != nil && err != submissiondomain.ErrSubmissionNotFound {
		return nil, fmt.Errorf("failed to check existing submission: %w", err)
	}
	if existingSubmission != nil {
		return nil, ErrSubmissionExists
	}

	fields, err := s.formFieldRepo.FindByVersion(ctx, params.TenantID, params.FormVersionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get form fields: %w", err)
	}

	if err := validateSubmissionData(params.Data, fields); err != nil {
		return nil, err
	}

	submission, err := submissiondomain.NewFormSubmission(
		params.TenantID,
		params.CaseID,
		form.ID,
		params.FormVersionID,
		params.SubmittedBy,
		params.Data,
		submissiondomain.SubmissionStatusSubmitted,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create submission: %w", err)
	}

	var savedSubmission *submissiondomain.FormSubmission
	err = database.InTransaction(ctx, s.caseRepo.DB(), func(tx *sql.Tx) error {
		if err := s.submissionRepo.SaveTx(ctx, tx, submission); err != nil {
			return fmt.Errorf("failed to save submission: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.TenantID,
				ActorID:        &params.SubmittedBy,
				Action:         "form.submitted",
				Resource:       "form_submission",
				ResourceID:     shared.StrPtr(submission.ID.String()),
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"case_id":         params.CaseID.String(),
					"form_id":         form.ID.String(),
					"form_version_id": params.FormVersionID.String(),
					"field_count":     len(params.Data),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		savedSubmission = submission
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &SubmissionResponse{
		ID:            savedSubmission.ID,
		CaseID:        savedSubmission.CaseID,
		FormID:        savedSubmission.FormID,
		FormVersionID: savedSubmission.FormVersionID,
		Status:        savedSubmission.Status,
		Data:          savedSubmission.Data,
		SubmittedBy:   savedSubmission.SubmittedBy,
		SubmittedAt:   savedSubmission.SubmittedAt,
		UpdatedAt:     savedSubmission.UpdatedAt,
	}, nil
}

func (s *CaseFormService) GetSubmission(ctx context.Context, tenantID, caseID, submissionID uuid.UUID) (*SubmissionResponse, error) {
	submission, err := s.submissionRepo.FindByID(ctx, tenantID, submissionID)
	if err != nil {
		return nil, submissiondomain.ErrSubmissionNotFound
	}

	if submission.CaseID != caseID {
		return nil, submissiondomain.ErrSubmissionNotFound
	}

	return &SubmissionResponse{
		ID:            submission.ID,
		CaseID:        submission.CaseID,
		FormID:        submission.FormID,
		FormVersionID: submission.FormVersionID,
		Status:        submission.Status,
		Data:          submission.Data,
		SubmittedBy:   submission.SubmittedBy,
		SubmittedAt:   submission.SubmittedAt,
		UpdatedAt:     submission.UpdatedAt,
	}, nil
}

func (s *CaseFormService) ListSubmissions(ctx context.Context, tenantID, caseID uuid.UUID) ([]*SubmissionResponse, error) {
	submissions, err := s.submissionRepo.ListByCase(ctx, tenantID, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed to list submissions: %w", err)
	}

	result := make([]*SubmissionResponse, len(submissions))
	for i, sub := range submissions {
		result[i] = &SubmissionResponse{
			ID:            sub.ID,
			CaseID:        sub.CaseID,
			FormID:        sub.FormID,
			FormVersionID: sub.FormVersionID,
			Status:        sub.Status,
			Data:          sub.Data,
			SubmittedBy:   sub.SubmittedBy,
			SubmittedAt:   sub.SubmittedAt,
			UpdatedAt:     sub.UpdatedAt,
		}
	}
	return result, nil
}

func (s *CaseFormService) GetWorkflowRequirements(ctx context.Context, tenantID, caseID uuid.UUID) (*WorkflowRequirements, error) {
	caseEntity, err := s.caseRepo.FindByID(ctx, tenantID, caseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if caseEntity.WorkflowInstanceID == nil {
		return &WorkflowRequirements{
			CaseID:          caseID,
			CurrentState:    caseEntity.WorkflowState,
			TotalRequired:   0,
			SubmittedCount:  0,
			MissingRequired: []*CaseFormAvailability{},
			SubmittedForms:  []*SubmissionResponse{},
			CanProceed:      true,
		}, nil
	}

	instance, err := s.workflowInstanceRepo.FindByID(ctx, tenantID, *caseEntity.WorkflowInstanceID)
	if err != nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	assignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, tenantID, instance.WorkflowDefID, instance.CurrentState)
	if err != nil {
		return nil, fmt.Errorf("failed to get form assignments: %w", err)
	}

	var requiredForms []*CaseFormAvailability
	for _, assignment := range assignments {
		if !assignment.Active || !assignment.Required {
			continue
		}

		form, err := s.formRepo.FindByID(ctx, tenantID, assignment.FormID)
		if err != nil {
			continue
		}

		if form.Status == formdomain.FormStatusArchived {
			continue
		}

		formVersion, err := s.formVersionRepo.FindByID(ctx, tenantID, assignment.FormVersionID)
		if err != nil {
			continue
		}

		if formVersion.Status != formdomain.FormVersionStatusPublished {
			continue
		}

		fields, err := s.formFieldRepo.FindByVersion(ctx, tenantID, formVersion.ID)
		if err != nil {
			continue
		}

		fieldAvailabilities := make([]FormFieldAvailability, len(fields))
		for i, field := range fields {
			fieldAvailabilities[i] = FormFieldAvailability{
				Key:         field.Key,
				Label:       field.Label,
				Type:        field.Type,
				Required:    field.Required,
				Placeholder: field.Placeholder,
				Options:     field.Options,
				Validation:  field.Validation,
			}
		}

		requiredForms = append(requiredForms, &CaseFormAvailability{
			AssignmentID:    assignment.ID,
			FormID:          form.ID,
			FormVersionID:   formVersion.ID,
			FormKey:         form.Key,
			FormName:        form.Name,
			FormDescription: form.Description,
			Version:         formVersion.Version,
			Required:        assignment.Required,
			DisplayOrder:    assignment.DisplayOrder,
			Active:          assignment.Active,
			Fields:          fieldAvailabilities,
		})
	}

	allSubmissions, err := s.submissionRepo.ListByCase(ctx, tenantID, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed to list submissions: %w", err)
	}

	submittedFormVersionIDs := make(map[uuid.UUID]bool)
	var submittedResponses []*SubmissionResponse
	for _, sub := range allSubmissions {
		submittedFormVersionIDs[sub.FormVersionID] = true
		submittedResponses = append(submittedResponses, &SubmissionResponse{
			ID:            sub.ID,
			CaseID:        sub.CaseID,
			FormID:        sub.FormID,
			FormVersionID: sub.FormVersionID,
			Status:        sub.Status,
			Data:          sub.Data,
			SubmittedBy:   sub.SubmittedBy,
			SubmittedAt:   sub.SubmittedAt,
			UpdatedAt:     sub.UpdatedAt,
		})
	}

	var missingRequired []*CaseFormAvailability
	for _, form := range requiredForms {
		if !submittedFormVersionIDs[form.FormVersionID] {
			missingRequired = append(missingRequired, form)
		}
	}

	canProceed := len(missingRequired) == 0

	return &WorkflowRequirements{
		CaseID:           caseID,
		CurrentState:     instance.CurrentState,
		TotalRequired:    len(requiredForms),
		SubmittedCount:   len(submittedResponses),
		MissingRequired:  missingRequired,
		SubmittedForms:   submittedResponses,
		CanProceed:       canProceed,
	}, nil
}

func validateSubmissionData(data map[string]interface{}, fields []*formdomain.FormField) error {
	fieldKeys := make(map[string]bool)
	for _, field := range fields {
		fieldKeys[field.Key] = true
	}

	for key := range data {
		if !fieldKeys[key] {
			return fmt.Errorf("%w: unknown field %q", ErrUnknownFields, key)
		}
	}

	for _, field := range fields {
		value, exists := data[field.Key]
		if !exists {
			if field.Required {
				return fmt.Errorf("%w: required field %q is missing", ErrFieldValidationFailed, field.Key)
			}
			continue
		}

		if err := validateFieldValue(field, value); err != nil {
			return err
		}
	}

	return nil
}

func validateFieldValue(field *formdomain.FormField, value interface{}) error {
	if field.Required && (value == nil || value == "") {
		return fmt.Errorf("%w: field %q is required", ErrFieldValidationFailed, field.Key)
	}

	if value == nil || value == "" {
		return nil
	}

	minLength := 0
	maxLength := 0
	if ml, ok := field.Validation["minLength"].(float64); ok {
		minLength = int(ml)
	}
	if ml, ok := field.Validation["maxLength"].(float64); ok {
		maxLength = int(ml)
	}

	switch field.Type {
	case formdomain.FieldTypeText, formdomain.FieldTypeTextarea, formdomain.FieldTypeEmail, formdomain.FieldTypePhone:
		strVal, ok := value.(string)
		if !ok {
			return fmt.Errorf("%w: field %q expected string", ErrFieldValidationFailed, field.Key)
		}
		if minLength > 0 && len(strVal) < minLength {
			return fmt.Errorf("%w: field %q is too short", ErrFieldValidationFailed, field.Key)
		}
		if maxLength > 0 && len(strVal) > maxLength {
			return fmt.Errorf("%w: field %q is too long", ErrFieldValidationFailed, field.Key)
		}
		if field.Type == formdomain.FieldTypeEmail {
			if !strings.Contains(strVal, "@") || !strings.Contains(strVal, ".") {
				return fmt.Errorf("%w: field %q is not a valid email", ErrFieldValidationFailed, field.Key)
			}
		}

	case formdomain.FieldTypeNumber, formdomain.FieldTypeDecimal:
		numVal, err := getFloat(value)
		if err != nil {
			return fmt.Errorf("%w: field %q expected number", ErrFieldValidationFailed, field.Key)
		}
		if minVal, ok := field.Validation["minValue"].(float64); ok && numVal < minVal {
			return fmt.Errorf("%w: field %q is below minimum", ErrFieldValidationFailed, field.Key)
		}
		if maxVal, ok := field.Validation["maxValue"].(float64); ok && numVal > maxVal {
			return fmt.Errorf("%w: field %q is above maximum", ErrFieldValidationFailed, field.Key)
		}

	case formdomain.FieldTypeDate, formdomain.FieldTypeDatetime:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%w: field %q expected date string", ErrFieldValidationFailed, field.Key)
		}

	case formdomain.FieldTypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%w: field %q expected boolean", ErrFieldValidationFailed, field.Key)
		}

	case formdomain.FieldTypeSelect, formdomain.FieldTypeRadio:
		strVal, ok := value.(string)
		if !ok {
			return fmt.Errorf("%w: field %q expected string", ErrFieldValidationFailed, field.Key)
		}
		validValues := make(map[string]bool)
		for _, opt := range field.Options {
			validValues[opt.Value] = true
		}
		if !validValues[strVal] {
			return fmt.Errorf("%w: field %q has invalid option %q", ErrOptionValidationFailed, field.Key, strVal)
		}

	case formdomain.FieldTypeMultiSelect:
		arrVal, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("%w: field %q expected array", ErrFieldValidationFailed, field.Key)
		}
		validValues := make(map[string]bool)
		for _, opt := range field.Options {
			validValues[opt.Value] = true
		}
		for _, v := range arrVal {
			strV, ok := v.(string)
			if !ok || !validValues[strV] {
				return fmt.Errorf("%w: field %q has invalid option", ErrOptionValidationFailed, field.Key)
			}
		}

	case formdomain.FieldTypeCheckbox:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%w: field %q expected boolean", ErrFieldValidationFailed, field.Key)
		}
	}

	return nil
}

func getFloat(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}
