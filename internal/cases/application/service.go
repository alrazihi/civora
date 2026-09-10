package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/google/uuid"
)

var (
	ErrCaseNotFound          = errors.New("case not found")
	ErrCaseInvalidInput      = errors.New("invalid input")
	ErrCaseTransition        = errors.New("invalid state transition")
	ErrCaseUserNotFound      = errors.New("assignee not found")
	ErrCaseTenantViolation   = errors.New("user does not belong to organization")
	ErrPersonNotFound        = errors.New("person not found")
	ErrPersonTenantViolation = errors.New("person does not belong to organization")
	ErrWorkflowNotAvailable  = errors.New("workflow engine not available for this operation")
)

// WorkflowTransitionExecutor abstracts the workflow engine for the case service.
type WorkflowTransitionExecutor interface {
	CreateInstanceForCase(ctx context.Context, tenantID, caseID uuid.UUID, workflowDefKey string, actorID uuid.UUID) (*workflowdomain.WorkflowInstance, error)
	GetInstanceByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) (*workflowdomain.WorkflowInstance, error)
	ExecuteTransition(ctx context.Context, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error)
	ExecuteTransitionInTx(ctx context.Context, tx *sql.Tx, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error)
	GetValidTransitions(ctx context.Context, tenantID, instanceID uuid.UUID) ([]workflowdomain.WorkflowTransition, error)
}

type UserChecker interface {
	BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
}

type CaseService struct {
	repo         domain.CaseRepository
	personFinder shared.PersonFinder
	userChecker  UserChecker
	auditor      auditdomain.EventRecorder
	auditRepo    auditdomain.AuditRepository
	workflowSvc  WorkflowTransitionExecutor
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
		repo:         repo,
		personFinder: personFinder,
		userChecker:  userChecker,
		auditor:      auditor,
		auditRepo:    auditRepo,
		workflowSvc:  wf,
	}
	if ws, ok := wf.(*workflowapp.WorkflowService); ok {
		ws.SetCaseStatusSyncer(svc)
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

			result = c
			return nil
		}
	})
	if err != nil {
		return nil, err
	}

	if s.workflowSvc != nil {
		workflowKey := "emergency_assistance"
		instance, err := s.workflowSvc.CreateInstanceForCase(ctx, c.OrganizationID, c.ID, workflowKey, c.CreatedByID)
		if err != nil {
			return nil, fmt.Errorf("failed to create workflow instance for case: %w", err)
		}
		instanceID := instance.ID
		c.WorkflowInstanceID = &instanceID
		// Sync case status to the workflow initial state so the denormalized
		// status field always reflects the authoritative workflow state.
		if err := c.SyncStatusFromWorkflow(instance.CurrentState); err != nil {
			return nil, fmt.Errorf("failed to sync case status from workflow: %w", err)
		}
		// Persist the workflow instance link and synced status atomically.
		if err := database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
			if err := s.repo.UpdateWorkflowInstanceIDTx(ctx, tx, c.OrganizationID, c.ID, instanceID); err != nil {
				return fmt.Errorf("failed to link workflow instance: %w", err)
			}
			if err := s.repo.UpdateStatusTx(ctx, tx, c.OrganizationID, c.ID, c.Status, c.Version); err != nil {
				return fmt.Errorf("failed to update case status: %w", err)
			}
			c.Version++
			return nil
		}); err != nil {
			return nil, err
		}
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
	c, err := s.repo.FindByID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if s.workflowSvc != nil {
		return s.changeStatusViaWorkflow(ctx, params, c)
	}

	oldStatus := c.Status
	if err := c.TransitionTo(params.Status); err != nil {
		if errors.Is(err, domain.ErrInvalidStateTransition) {
			return nil, fmt.Errorf("%w: from %s to %s", ErrCaseTransition, c.Status, params.Status)
		}
		return nil, err
	}

	var result *domain.Case
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatusTx(ctx, tx, params.OrganizationID, params.CaseID, params.Status, c.Version); err != nil {
			return fmt.Errorf("failed to update case status: %w", err)
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
					"from": string(oldStatus),
					"to":   string(params.Status),
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

func (s *CaseService) changeStatusViaWorkflow(ctx context.Context, params ChangeCaseStatusParams, c *domain.Case) (*domain.Case, error) {
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

	newCaseStatus := domain.CaseStatus(targetTransition.ToState)

	var result *domain.Case
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		_, err := s.workflowSvc.ExecuteTransitionInTx(ctx, tx, workflowapp.ExecuteTransitionParams{
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

		if err := s.repo.UpdateStatusTx(ctx, tx, params.OrganizationID, params.CaseID, newCaseStatus, c.Version); err != nil {
			return fmt.Errorf("failed to update case status: %w", err)
		}

		result = c
		return nil
	})
	if err != nil {
		return nil, err
	}

	result.Status = newCaseStatus
	if newCaseStatus == domain.CaseStatusClosed {
		now := time.Now().UTC()
		result.ClosedAt = &now
	}

	return result, nil
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
	return s.repo.UpdateStatus(ctx, tenantID, caseID, c.Status, c.Version)
}
