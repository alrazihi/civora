package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database"
	submissiondomain "github.com/alrazihi/civora/internal/form_submission/domain"
	formdomain "github.com/alrazihi/civora/internal/forms/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	assignmentdomain "github.com/alrazihi/civora/internal/workflow_form_assignment/domain"
	"github.com/google/uuid"
)

var (
	ErrCaseNotFound             = errors.New("case not found")
	ErrCaseInvalidInput         = errors.New("invalid input")
	ErrCaseTransition           = errors.New("invalid state transition")
	ErrCaseUserNotFound         = errors.New("assignee not found")
	ErrCaseTenantViolation      = errors.New("user does not belong to organization")
	ErrPersonNotFound           = errors.New("person not found")
	ErrPersonTenantViolation    = errors.New("person does not belong to organization")
	ErrWorkflowNotAvailable     = errors.New("workflow engine not available for this operation")
	ErrWorkflowInstanceNotFound = errors.New("workflow instance not found")
	ErrFormNotAssigned          = errors.New("form is not assigned to the current workflow state")
	ErrFormNotPublished         = errors.New("form version is not published")
	ErrFormArchived             = errors.New("form is archived")
	ErrSubmissionExists         = errors.New("submission already exists for this case and form version")
	ErrUnknownFields            = errors.New("submission contains unknown fields")
	ErrFieldValidationFailed    = errors.New("field validation failed")
	ErrOptionValidationFailed   = errors.New("option validation failed")
	ErrInvalidSubmissionData    = errors.New("invalid submission data")
	ErrRequiredFormsIncomplete  = errors.New("required forms are incomplete")
)

// WorkflowTransitionExecutor abstracts the workflow engine for the case service.
type WorkflowTransitionExecutor interface {
	CreateInstanceForCase(ctx context.Context, tenantID, caseID uuid.UUID, workflowDefKey string, actorID uuid.UUID) (*workflowdomain.WorkflowInstance, error)
	CreateInstanceForCaseTx(ctx context.Context, tx *sql.Tx, tenantID, caseID uuid.UUID, workflowDefKey string, actorID uuid.UUID) (*workflowdomain.WorkflowInstance, error)
	CreateInstanceForCaseByDefID(ctx context.Context, tenantID, caseID uuid.UUID, workflowDefID uuid.UUID, actorID uuid.UUID) (*workflowdomain.WorkflowInstance, error)
	CreateInstanceForCaseByDefIDTx(ctx context.Context, tx *sql.Tx, tenantID, caseID uuid.UUID, workflowDefID uuid.UUID, actorID uuid.UUID) (*workflowdomain.WorkflowInstance, error)
	GetInstanceByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) (*workflowdomain.WorkflowInstance, error)
	ExecuteTransition(ctx context.Context, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error)
	ExecuteTransitionInTx(ctx context.Context, tx *sql.Tx, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error)
	GetValidTransitions(ctx context.Context, tenantID, instanceID uuid.UUID) ([]workflowdomain.WorkflowTransition, error)
	ListActiveForSelection(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*workflowdomain.WorkflowDefinition, int, error)
}

type UserChecker interface {
	BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
}

type CaseService struct {
	repo                 domain.CaseRepository
	personFinder         shared.PersonFinder
	userChecker          UserChecker
	auditor              auditdomain.EventRecorder
	auditRepo            auditdomain.AuditRepository
	workflowSvc          WorkflowTransitionExecutor
	workflowDefRepo      workflowdomain.WorkflowDefinitionRepository
	workflowInstanceRepo workflowdomain.WorkflowInstanceRepository
	formRepo             formdomain.FormRepository
	formVersionRepo      formdomain.FormVersionRepository
	formFieldRepo        formdomain.FormFieldRepository
	assignmentRepo       assignmentdomain.WorkflowStateFormAssignmentRepository
	submissionRepo       submissiondomain.FormSubmissionRepository
}

func NewCaseService(
	repo domain.CaseRepository,
	personFinder shared.PersonFinder,
	userChecker UserChecker,
	auditor auditdomain.EventRecorder,
	auditRepo auditdomain.AuditRepository,
	workflowSvc ...WorkflowTransitionExecutor,
) *CaseService {
	var wf WorkflowTransitionExecutor
	if len(workflowSvc) > 0 {
		wf = workflowSvc[0]
	}
	svc := &CaseService{
		repo:                 repo,
		personFinder:         personFinder,
		userChecker:          userChecker,
		auditor:              auditor,
		auditRepo:            auditRepo,
		workflowSvc:          wf,
		workflowDefRepo:      nil,
		workflowInstanceRepo: nil,
		formRepo:             nil,
		formVersionRepo:      nil,
		formFieldRepo:        nil,
		assignmentRepo:       nil,
		submissionRepo:       nil,
	}
	if ws, ok := wf.(workflowapp.TransitionObserverRegistrar); ok {
		ws.SetTransitionObserver(svc)
	}
	return svc
}

type CreateCaseParams struct {
	OrganizationID uuid.UUID
	Title          string
	Description    string
	ServiceType    domain.ServiceType
	Priority       domain.Priority
	PersonID       *uuid.UUID
	CreatedByID    uuid.UUID
	WorkflowID     *uuid.UUID
}

type CaseFormAvailability struct {
	AssignmentID    uuid.UUID               `json:"assignment_id"`
	FormID          uuid.UUID               `json:"form_id"`
	FormVersionID   uuid.UUID               `json:"form_version_id"`
	FormKey         string                  `json:"form_key"`
	FormName        string                  `json:"form_name"`
	FormDescription string                  `json:"form_description"`
	Version         int                     `json:"version"`
	Required        bool                    `json:"required"`
	DisplayOrder    int                     `json:"display_order"`
	Active          bool                    `json:"active"`
	Fields          []FormFieldAvailability `json:"fields"`
}

type FormFieldAvailability struct {
	Key         string                  `json:"key"`
	Label       string                  `json:"label"`
	Type        formdomain.FieldType    `json:"type"`
	Required    bool                    `json:"required"`
	Placeholder string                  `json:"placeholder"`
	Options     []formdomain.FormOption `json:"options,omitempty"`
	Validation  map[string]interface{}  `json:"validation,omitempty"`
}

type SubmissionResponse struct {
	ID            uuid.UUID                         `json:"id"`
	CaseID        uuid.UUID                         `json:"case_id"`
	FormID        uuid.UUID                         `json:"form_id"`
	FormVersionID uuid.UUID                         `json:"form_version_id"`
	Status        submissiondomain.SubmissionStatus `json:"status"`
	Data          map[string]interface{}            `json:"data"`
	SubmittedBy   uuid.UUID                         `json:"submitted_by"`
	SubmittedAt   time.Time                         `json:"submitted_at"`
	UpdatedAt     time.Time                         `json:"updated_at"`
}

type WorkflowRequirements struct {
	CaseID          uuid.UUID               `json:"case_id"`
	CurrentState    string                  `json:"current_state"`
	TotalRequired   int                     `json:"total_required"`
	SubmittedCount  int                     `json:"submitted_count"`
	MissingRequired []*CaseFormAvailability `json:"missing_required"`
	SubmittedForms  []*SubmissionResponse   `json:"submitted_forms"`
	CanProceed      bool                    `json:"can_proceed"`
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

func (s *CaseService) CreateCase(ctx context.Context, params CreateCaseParams) (*domain.Case, error) {
	if params.Title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrCaseInvalidInput)
	}

	if params.PersonID != nil {
		p, err := s.personFinder.FindByID(ctx, params.OrganizationID, *params.PersonID)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPersonNotFound, err)
		}
		if p.OrganizationID != params.OrganizationID {
			return nil, ErrPersonTenantViolation
		}
	}

	c, err := domain.NewCase(params.OrganizationID, params.CreatedByID, params.Title, params.Description, params.ServiceType, params.Priority, params.PersonID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaseInvalidInput, err)
	}

	var workflowDefKey string
	if params.WorkflowID != nil {
		if *params.WorkflowID == uuid.Nil {
			return nil, fmt.Errorf("%w: invalid workflow ID", ErrCaseInvalidInput)
		}
		c.WorkflowID = params.WorkflowID
	} else {
		key := domain.WorkflowKeyForServiceType(c.ServiceType)
		c.WorkflowKey = key
		workflowDefKey = key
	}

	const maxRetries = 3
	var result *domain.Case
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		for attempts := 0; ; attempts++ {
			if err := s.repo.SaveTx(ctx, tx, c); err != nil {
				if errors.Is(err, domain.ErrCaseNumberConflict) && attempts < maxRetries-1 {
					c.RegenerateCaseNumber()
					continue
				}
				return fmt.Errorf("failed to save case: %w", err)
			}

			if s.auditor != nil {
				if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
					OrganizationID: c.OrganizationID,
					ActorID:        &c.CreatedByID,
					Action:         "case.created",
					Resource:       "case",
					ResourceID:     shared.StrPtr(c.ID.String()),
					Outcome:        "success",
					RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				}); err != nil {
					return fmt.Errorf("failed to record audit event: %w", err)
				}
			}

			if s.workflowSvc != nil {
				var instance *workflowdomain.WorkflowInstance
				var instanceErr error
				if params.WorkflowID != nil {
					instance, instanceErr = s.workflowSvc.CreateInstanceForCaseByDefIDTx(ctx, tx, c.OrganizationID, c.ID, *params.WorkflowID, c.CreatedByID)
				} else {
					instance, instanceErr = s.workflowSvc.CreateInstanceForCaseTx(ctx, tx, c.OrganizationID, c.ID, workflowDefKey, c.CreatedByID)
				}
				if instanceErr != nil {
					return fmt.Errorf("failed to create workflow instance for case: %w", instanceErr)
				}
				instanceID := instance.ID
				c.WorkflowInstanceID = &instanceID
				c.WorkflowState = instance.CurrentState
				if err := c.SyncStatusFromWorkflow(instance.CurrentState); err != nil {
					return fmt.Errorf("failed to sync case status from workflow: %w", err)
				}
				if err := s.repo.UpdateWorkflowInstanceIDTx(ctx, tx, c.OrganizationID, c.ID, instanceID); err != nil {
					return fmt.Errorf("failed to link workflow instance: %w", err)
				}
				if err := s.repo.UpdateStatusTx(ctx, tx, c.OrganizationID, c.ID, c.Status, c.Version); err != nil {
					return fmt.Errorf("failed to update case status: %w", err)
				}
				c.Version++
			}

			result = c
			return nil
		}
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

type ChangeCaseStatusParams struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	Status         domain.CaseStatus
	ActorID        uuid.UUID
	ActorRole      string
}

func (s *CaseService) ChangeStatus(ctx context.Context, params ChangeCaseStatusParams) (*domain.Case, error) {
	if s.workflowSvc == nil {
		return nil, ErrWorkflowNotAvailable
	}
	return s.changeStatusViaWorkflow(ctx, params)
}

func (s *CaseService) changeStatusViaWorkflow(ctx context.Context, params ChangeCaseStatusParams) (*domain.Case, error) {
	if s.workflowSvc == nil {
		return nil, ErrWorkflowNotAvailable
	}

	c, err := s.repo.FindByID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	instance, err := s.workflowSvc.GetInstanceByCaseID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		return nil, fmt.Errorf("workflow instance not found: %w", err)
	}

	validTransitions, err := s.workflowSvc.GetValidTransitions(ctx, params.OrganizationID, instance.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get valid transitions: %w", err)
	}

	var targetTransition *workflowdomain.WorkflowTransition
	for i := range validTransitions {
		if validTransitions[i].ToState == string(params.Status) {
			targetTransition = &validTransitions[i]
			break
		}
	}
	if targetTransition == nil {
		return nil, fmt.Errorf("%w: no transition leads to status %s from %s", ErrCaseTransition, params.Status, instance.CurrentState)
	}

	var result *domain.Case
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.checkRequiredFormsCompleteInTx(ctx, tx, params.OrganizationID, params.CaseID, instance.WorkflowDefID, instance.CurrentState); err != nil {
			return err
		}

		_, err = s.workflowSvc.ExecuteTransitionInTx(ctx, tx, workflowapp.ExecuteTransitionParams{
			TenantID:      params.OrganizationID,
			InstanceID:    instance.ID,
			TransitionKey: targetTransition.Key,
			ActorID:       params.ActorID,
			ActorRole:     params.ActorRole,
			Reason:        "",
		})
		if err != nil {
			return fmt.Errorf("workflow transition failed: %w", err)
		}

		updatedCase, err := s.repo.FindByIDTx(ctx, tx, params.OrganizationID, params.CaseID)
		if err != nil {
			return fmt.Errorf("failed to reload case after workflow transition: %w", err)
		}

		c.Status = updatedCase.Status
		c.WorkflowState = updatedCase.WorkflowState
		c.Version = updatedCase.Version
		if domain.IsClosed(updatedCase.Status) {
			now := time.Now().UTC()
			c.ClosedAt = &now
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: c.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "case.transition",
				Resource:       "case",
				ResourceID:     shared.StrPtr(c.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"from": string(instance.CurrentState),
					"to":   string(updatedCase.Status),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = c
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *CaseService) checkRequiredFormsCompleteInTx(ctx context.Context, tx *sql.Tx, tenantID, caseID uuid.UUID, workflowDefID uuid.UUID, currentState string) error {
	if s.assignmentRepo == nil || s.submissionRepo == nil {
		return nil
	}

	assignments, err := s.assignmentRepo.FindByWorkflowAndStateForUpdateTx(ctx, tx, tenantID, workflowDefID, currentState)
	if err != nil {
		return fmt.Errorf("failed to load form assignments: %w", err)
	}

	var missingFormVersions []uuid.UUID
	for _, assignment := range assignments {
		if !assignment.Active || !assignment.Required {
			continue
		}

		_, err := s.submissionRepo.FindByCaseAndFormVersionForUpdateTx(ctx, tx, tenantID, caseID, assignment.FormVersionID)
		if err == submissiondomain.ErrSubmissionNotFound {
			missingFormVersions = append(missingFormVersions, assignment.FormVersionID)
		}
	}

	if len(missingFormVersions) > 0 {
		return fmt.Errorf("%w: missing form versions %v", ErrRequiredFormsIncomplete, missingFormVersions)
	}

	return nil
}

type AssignCaseParams struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	UserID         uuid.UUID
	ActorID        uuid.UUID
}

func (s *CaseService) AssignCase(ctx context.Context, params AssignCaseParams) (*domain.Case, error) {
	c, err := s.repo.FindByID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if s.userChecker != nil {
		valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate assignee: %w", err)
		}
		if !valid {
			return nil, ErrCaseTenantViolation
		}
	}

	c.AssignTo(params.UserID)

	var result *domain.Case
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.AssignTx(ctx, tx, params.OrganizationID, params.CaseID, params.UserID); err != nil {
			return fmt.Errorf("failed to assign case: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: c.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "case.assigned",
				Resource:       "case",
				ResourceID:     shared.StrPtr(c.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"assigned_to": params.UserID.String(),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = c
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *CaseService) GetCase(ctx context.Context, orgID, caseID uuid.UUID) (*domain.Case, error) {
	c, err := s.repo.FindByID(ctx, orgID, caseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}
	return c, nil
}

func (s *CaseService) ListCases(ctx context.Context, orgID uuid.UUID, limit, offset int, filter domain.CaseFilter) ([]*domain.Case, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.CountByOrganization(ctx, orgID, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count cases: %w", err)
	}

	cases, err := s.repo.FindByOrganizationWithFilter(ctx, orgID, limit, offset, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list cases: %w", err)
	}

	return cases, total, nil
}

func (s *CaseService) GetStatistics(ctx context.Context, orgID uuid.UUID) (*domain.CaseStatistics, error) {
	stats, err := s.repo.Statistics(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate case statistics: %w", err)
	}
	return stats, nil
}

type TimelineEvent struct {
	ID        uuid.UUID              `json:"id"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	Outcome   string                 `json:"outcome"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

func (s *CaseService) GetCaseTimeline(ctx context.Context, orgID, caseID uuid.UUID) ([]*TimelineEvent, error) {
	events, err := s.auditRepo.FindByResource(ctx, orgID, caseID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get case timeline: %w", err)
	}

	var timeline []*TimelineEvent
	for _, ev := range events {
		timeline = append(timeline, &TimelineEvent{
			ID:        ev.ID,
			Action:    ev.Action,
			Resource:  ev.Resource,
			Outcome:   ev.Outcome,
			Timestamp: ev.Timestamp,
			Metadata:  ev.Metadata,
		})
	}

	return timeline, nil
}

// GetWorkflowService returns the underlying workflow service for testing
// and advanced use cases.
func (s *CaseService) GetWorkflowService() WorkflowTransitionExecutor {
	return s.workflowSvc
}

// GetWorkflowInstance returns the workflow instance associated with a case.
func (s *CaseService) GetWorkflowInstance(ctx context.Context, tenantID, caseID uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
	if s.workflowSvc == nil {
		return nil, ErrWorkflowNotAvailable
	}
	return s.workflowSvc.GetInstanceByCaseID(ctx, tenantID, caseID)
}

// SyncCaseStatus updates the case status to match the given workflow state.
// It implements workflowapp.CaseStatusSyncer so the workflow engine can keep
// the denormalized case status in sync after each transition.
func (s *CaseService) SyncCaseStatus(ctx context.Context, tenantID, caseID uuid.UUID, stateKey string) error {
	c, err := s.repo.FindByID(ctx, tenantID, caseID)
	if err != nil {
		return err
	}
	if err := c.SyncStatusFromWorkflow(stateKey); err != nil {
		return err
	}
	c.WorkflowState = stateKey
	return s.repo.UpdateStatus(ctx, tenantID, caseID, c.Status, c.Version)
}

// OnTransition implements workflowapp.TransitionObserver. It is invoked by
// the workflow engine within the transition transaction to keep the
// denormalized case status in sync with the authoritative workflow state.
// Because the observer runs inside the same transaction, a failure here
// rolls back the entire transition atomically.
func (s *CaseService) OnTransition(ctx context.Context, tx *sql.Tx, instance *workflowdomain.WorkflowInstance, transition *workflowdomain.WorkflowTransition, targetTerminal bool) error {
	if instance == nil {
		return nil
	}
	c, err := s.repo.FindByIDTx(ctx, tx, instance.TenantID, instance.CaseID)
	if err != nil {
		return err
	}
	if err := c.SyncStatusFromWorkflow(transition.ToState); err != nil {
		return err
	}
	if targetTerminal {
		now := time.Now().UTC()
		c.ClosedAt = &now
	}
	c.WorkflowState = transition.ToState
	if err := s.repo.UpdateWorkflowStateTx(ctx, tx, instance.TenantID, instance.CaseID, transition.ToState, c.Version, targetTerminal); err != nil {
		return err
	}
	c.Version++
	return nil
}

func (s *CaseService) SetFormRepos(
	workflowDefRepo workflowdomain.WorkflowDefinitionRepository,
	workflowInstanceRepo workflowdomain.WorkflowInstanceRepository,
	formRepo formdomain.FormRepository,
	formVersionRepo formdomain.FormVersionRepository,
	formFieldRepo formdomain.FormFieldRepository,
	assignmentRepo assignmentdomain.WorkflowStateFormAssignmentRepository,
	submissionRepo submissiondomain.FormSubmissionRepository,
) {
	s.workflowDefRepo = workflowDefRepo
	s.workflowInstanceRepo = workflowInstanceRepo
	s.formRepo = formRepo
	s.formVersionRepo = formVersionRepo
	s.formFieldRepo = formFieldRepo
	s.assignmentRepo = assignmentRepo
	s.submissionRepo = submissionRepo
}

func (s *CaseService) GetCaseForms(ctx context.Context, orgID, caseID uuid.UUID) ([]*CaseFormAvailability, error) {
	if s.workflowDefRepo == nil || s.workflowInstanceRepo == nil || s.formRepo == nil || s.formVersionRepo == nil || s.formFieldRepo == nil || s.assignmentRepo == nil || s.submissionRepo == nil {
		return nil, fmt.Errorf("form services not initialized")
	}

	caseEntity, err := s.repo.FindByID(ctx, orgID, caseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if caseEntity.WorkflowInstanceID == nil {
		return []*CaseFormAvailability{}, nil
	}

	instance, err := s.workflowInstanceRepo.FindByID(ctx, orgID, *caseEntity.WorkflowInstanceID)
	if err != nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	assignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, orgID, instance.WorkflowDefID, instance.CurrentState)
	if err != nil {
		return nil, fmt.Errorf("failed to get form assignments: %w", err)
	}

	var activeAssignments []*assignmentdomain.WorkflowStateFormAssignment
	var formIDs []uuid.UUID
	var versionIDs []uuid.UUID
	for _, assignment := range assignments {
		if !assignment.Active {
			continue
		}
		activeAssignments = append(activeAssignments, assignment)
		formIDs = append(formIDs, assignment.FormID)
		versionIDs = append(versionIDs, assignment.FormVersionID)
	}

	forms, err := s.formRepo.FindByIDs(ctx, orgID, formIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch-fetch forms: %w", err)
	}

	formByID := make(map[uuid.UUID]*formdomain.Form)
	for _, f := range forms {
		formByID[f.ID] = f
	}

	versions, err := s.formVersionRepo.FindByIDs(ctx, orgID, versionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch-fetch form versions: %w", err)
	}

	versionByID := make(map[uuid.UUID]*formdomain.FormVersion)
	for _, v := range versions {
		versionByID[v.ID] = v
	}

	var activeVersionIDs []uuid.UUID
	for _, v := range versions {
		if v.Status == formdomain.FormVersionStatusPublished {
			activeVersionIDs = append(activeVersionIDs, v.ID)
		}
	}

	fieldsByVersionID := make(map[uuid.UUID][]*formdomain.FormField)
	if len(activeVersionIDs) > 0 {
		allFields, err := s.formFieldRepo.FindFieldsByVersionIDs(ctx, orgID, activeVersionIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to batch-fetch form fields: %w", err)
		}
		for _, f := range allFields {
			fieldsByVersionID[f.FormVersionID] = append(fieldsByVersionID[f.FormVersionID], f)
		}
	}

	var result []*CaseFormAvailability
	for _, assignment := range activeAssignments {
		form, ok := formByID[assignment.FormID]
		if !ok || form.Status == formdomain.FormStatusArchived {
			continue
		}

		formVersion, ok := versionByID[assignment.FormVersionID]
		if !ok || formVersion.Status != formdomain.FormVersionStatusPublished {
			continue
		}

		fields := fieldsByVersionID[formVersion.ID]
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

func (s *CaseService) SubmitForm(ctx context.Context, orgID, caseID, submittedBy, formVersionID uuid.UUID, data map[string]interface{}) (*SubmissionResponse, error) {
	if s.workflowInstanceRepo == nil || s.formVersionRepo == nil || s.formRepo == nil || s.assignmentRepo == nil || s.submissionRepo == nil || s.formFieldRepo == nil {
		return nil, fmt.Errorf("form services not initialized")
	}

	caseEntity, err := s.repo.FindByID(ctx, orgID, caseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if caseEntity.WorkflowInstanceID == nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	instance, err := s.workflowInstanceRepo.FindByID(ctx, orgID, *caseEntity.WorkflowInstanceID)
	if err != nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	fields, err := s.formFieldRepo.FindByVersion(ctx, orgID, formVersionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get form fields: %w", err)
	}

	if err := validateSubmissionData(data, fields); err != nil {
		return nil, err
	}

	submission, err := submissiondomain.NewFormSubmission(
		orgID,
		caseID,
		uuid.Nil,
		formVersionID,
		submittedBy,
		data,
		submissiondomain.SubmissionStatusSubmitted,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create submission: %w", err)
	}

	var savedSubmission *submissiondomain.FormSubmission
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		lockedForm, err := s.formRepo.FindByIDForUpdateTx(ctx, tx, orgID, formVersionID)
		if err != nil {
			return ErrFormNotPublished
		}
		if lockedForm.Status == formdomain.FormStatusArchived {
			return ErrFormArchived
		}

		lockedVersion, err := s.formVersionRepo.FindByIDForUpdateTx(ctx, tx, orgID, formVersionID)
		if err != nil {
			return ErrFormNotPublished
		}
		if lockedVersion.Status != formdomain.FormVersionStatusPublished {
			return ErrFormNotPublished
		}

		submission.FormID = lockedForm.ID

		lockedAssignments, err := s.assignmentRepo.FindByWorkflowAndStateForUpdateTx(ctx, tx, orgID, instance.WorkflowDefID, instance.CurrentState)
		if err != nil {
			return fmt.Errorf("failed to check form assignments: %w", err)
		}

		assignmentFound := false
		for _, assignment := range lockedAssignments {
			if assignment.FormVersionID == formVersionID && assignment.Active {
				assignmentFound = true
				break
			}
		}
		if !assignmentFound {
			return ErrFormNotAssigned
		}

		existingSubmission, err := s.submissionRepo.FindByCaseAndFormVersionForUpdateTx(ctx, tx, orgID, caseID, formVersionID)
		if err != nil && err != submissiondomain.ErrSubmissionNotFound {
			return fmt.Errorf("failed to check existing submission: %w", err)
		}
		if existingSubmission != nil {
			return ErrSubmissionExists
		}

		lockedFields, err := s.formFieldRepo.FindByVersionForUpdateTx(ctx, tx, orgID, formVersionID)
		if err != nil {
			return fmt.Errorf("failed to get form fields: %w", err)
		}
		if err := validateSubmissionData(data, lockedFields); err != nil {
			return err
		}

		if err := s.submissionRepo.SaveTx(ctx, tx, submission); err != nil {
			return fmt.Errorf("failed to save submission: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: orgID,
				ActorID:        &submittedBy,
				Action:         "form.submitted",
				Resource:       "form_submission",
				ResourceID:     shared.StrPtr(submission.ID.String()),
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"case_id":         caseID.String(),
					"form_id":         lockedForm.ID.String(),
					"form_version_id": formVersionID.String(),
					"field_count":     len(data),
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

func (s *CaseService) SubmitFormByKey(ctx context.Context, orgID, caseID, submittedBy uuid.UUID, formKey string, data map[string]interface{}) (*SubmissionResponse, error) {
	caseEntity, err := s.repo.FindByID(ctx, orgID, caseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if caseEntity.WorkflowInstanceID == nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	instance, err := s.workflowInstanceRepo.FindByID(ctx, orgID, *caseEntity.WorkflowInstanceID)
	if err != nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	assignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, orgID, instance.WorkflowDefID, instance.CurrentState)
	if err != nil {
		return nil, fmt.Errorf("failed to get form assignments: %w", err)
	}

	var targetFormVersionID uuid.UUID
	found := false
	for _, assignment := range assignments {
		if !assignment.Active {
			continue
		}
		form, err := s.formRepo.FindByID(ctx, orgID, assignment.FormID)
		if err != nil {
			continue
		}
		if form.Key == formKey {
			targetFormVersionID = assignment.FormVersionID
			found = true
			break
		}
	}
	if !found {
		return nil, ErrFormNotAssigned
	}

	return s.SubmitForm(ctx, orgID, caseID, submittedBy, targetFormVersionID, data)
}

func (s *CaseService) GetFormSubmissionByKey(ctx context.Context, orgID, caseID uuid.UUID, formKey string) (*SubmissionResponse, error) {
	if s.submissionRepo == nil || s.assignmentRepo == nil {
		return nil, fmt.Errorf("form services not initialized")
	}

	caseEntity, err := s.repo.FindByID(ctx, orgID, caseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if caseEntity.WorkflowInstanceID == nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	instance, err := s.workflowInstanceRepo.FindByID(ctx, orgID, *caseEntity.WorkflowInstanceID)
	if err != nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	assignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, orgID, instance.WorkflowDefID, instance.CurrentState)
	if err != nil {
		return nil, fmt.Errorf("failed to get form assignments: %w", err)
	}

	var targetFormVersionID uuid.UUID
	found := false
	for _, assignment := range assignments {
		if !assignment.Active {
			continue
		}
		form, err := s.formRepo.FindByID(ctx, orgID, assignment.FormID)
		if err != nil {
			continue
		}
		if form.Key == formKey {
			targetFormVersionID = assignment.FormVersionID
			found = true
			break
		}
	}
	if !found {
		return nil, ErrFormNotAssigned
	}

	submission, err := s.submissionRepo.FindByCaseAndFormVersion(ctx, orgID, caseID, targetFormVersionID)
	if err != nil {
		return nil, err
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

func (s *CaseService) GetCaseFormSubmissions(ctx context.Context, orgID, caseID uuid.UUID) (map[string]*SubmissionResponse, error) {
	if s.submissionRepo == nil || s.assignmentRepo == nil || s.workflowInstanceRepo == nil {
		return nil, fmt.Errorf("form services not initialized")
	}

	caseEntity, err := s.repo.FindByID(ctx, orgID, caseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if caseEntity.WorkflowInstanceID == nil {
		return map[string]*SubmissionResponse{}, nil
	}

	instance, err := s.workflowInstanceRepo.FindByID(ctx, orgID, *caseEntity.WorkflowInstanceID)
	if err != nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	assignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, orgID, instance.WorkflowDefID, instance.CurrentState)
	if err != nil {
		return nil, fmt.Errorf("failed to get form assignments: %w", err)
	}

	submissions, err := s.submissionRepo.ListByCase(ctx, orgID, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed to list submissions: %w", err)
	}

	submissionsByVersion := make(map[uuid.UUID]*SubmissionResponse)
	for _, sub := range submissions {
		submissionsByVersion[sub.FormVersionID] = &SubmissionResponse{
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

	result := make(map[string]*SubmissionResponse)
	for _, assignment := range assignments {
		if !assignment.Active {
			continue
		}
		form, err := s.formRepo.FindByID(ctx, orgID, assignment.FormID)
		if err != nil {
			continue
		}
		if sub, ok := submissionsByVersion[assignment.FormVersionID]; ok {
			result[form.Key] = sub
		} else {
			result[form.Key] = nil
		}
	}

	return result, nil
}

func (s *CaseService) GetSubmission(ctx context.Context, orgID, caseID, submissionID uuid.UUID) (*SubmissionResponse, error) {
	if s.submissionRepo == nil {
		return nil, fmt.Errorf("form services not initialized")
	}

	submission, err := s.submissionRepo.FindByID(ctx, orgID, submissionID)
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

func (s *CaseService) ListSubmissions(ctx context.Context, orgID, caseID uuid.UUID) ([]*SubmissionResponse, error) {
	if s.submissionRepo == nil {
		return nil, fmt.Errorf("form services not initialized")
	}

	submissions, err := s.submissionRepo.ListByCase(ctx, orgID, caseID)
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

func (s *CaseService) GetWorkflowRequirements(ctx context.Context, orgID, caseID uuid.UUID) (*WorkflowRequirements, error) {
	if s.workflowInstanceRepo == nil || s.workflowDefRepo == nil || s.formRepo == nil || s.formVersionRepo == nil || s.formFieldRepo == nil || s.assignmentRepo == nil || s.submissionRepo == nil {
		return nil, fmt.Errorf("form services not initialized")
	}

	caseEntity, err := s.repo.FindByID(ctx, orgID, caseID)
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

	instance, err := s.workflowInstanceRepo.FindByID(ctx, orgID, *caseEntity.WorkflowInstanceID)
	if err != nil {
		return nil, ErrWorkflowInstanceNotFound
	}

	assignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, orgID, instance.WorkflowDefID, instance.CurrentState)
	if err != nil {
		return nil, fmt.Errorf("failed to get form assignments: %w", err)
	}

	var requiredAssignments []*assignmentdomain.WorkflowStateFormAssignment
	var formIDs []uuid.UUID
	var versionIDs []uuid.UUID
	for _, assignment := range assignments {
		if !assignment.Active || !assignment.Required {
			continue
		}
		requiredAssignments = append(requiredAssignments, assignment)
		formIDs = append(formIDs, assignment.FormID)
		versionIDs = append(versionIDs, assignment.FormVersionID)
	}

	forms, err := s.formRepo.FindByIDs(ctx, orgID, formIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch-fetch forms: %w", err)
	}

	formByID := make(map[uuid.UUID]*formdomain.Form)
	for _, f := range forms {
		formByID[f.ID] = f
	}

	versions, err := s.formVersionRepo.FindByIDs(ctx, orgID, versionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch-fetch form versions: %w", err)
	}

	versionByID := make(map[uuid.UUID]*formdomain.FormVersion)
	for _, v := range versions {
		versionByID[v.ID] = v
	}

	var publishedVersionIDs []uuid.UUID
	for _, v := range versions {
		if v.Status == formdomain.FormVersionStatusPublished {
			publishedVersionIDs = append(publishedVersionIDs, v.ID)
		}
	}

	fieldsByVersionID := make(map[uuid.UUID][]*formdomain.FormField)
	if len(publishedVersionIDs) > 0 {
		allFields, err := s.formFieldRepo.FindFieldsByVersionIDs(ctx, orgID, publishedVersionIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to batch-fetch form fields: %w", err)
		}
		for _, f := range allFields {
			fieldsByVersionID[f.FormVersionID] = append(fieldsByVersionID[f.FormVersionID], f)
		}
	}

	var requiredForms []*CaseFormAvailability
	for _, assignment := range requiredAssignments {
		form, ok := formByID[assignment.FormID]
		if !ok || form.Status == formdomain.FormStatusArchived {
			continue
		}

		formVersion, ok := versionByID[assignment.FormVersionID]
		if !ok || formVersion.Status != formdomain.FormVersionStatusPublished {
			continue
		}

		fields := fieldsByVersionID[formVersion.ID]
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

	allSubmissions, err := s.submissionRepo.ListByCase(ctx, orgID, caseID)
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

	return &WorkflowRequirements{
		CaseID:          caseID,
		CurrentState:    instance.CurrentState,
		TotalRequired:   len(requiredForms),
		SubmittedCount:  len(submittedResponses),
		MissingRequired: missingRequired,
		SubmittedForms:  submittedResponses,
		CanProceed:      len(missingRequired) == 0,
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
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}
