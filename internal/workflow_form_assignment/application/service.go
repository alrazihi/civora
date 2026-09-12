package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	"github.com/alrazihi/civora/internal/forms/domain"
	"github.com/alrazihi/civora/internal/shared"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	assignmentdomain "github.com/alrazihi/civora/internal/workflow_form_assignment/domain"
)

var (
	ErrWorkflowNotFound        = fmt.Errorf("%w: workflow definition not found", assignmentdomain.ErrWorkflowDefinitionNotFound)
	ErrAssignmentNotFound      = fmt.Errorf("%w: assignment not found", assignmentdomain.ErrAssignmentNotFound)
	ErrAssignmentAlreadyExists = fmt.Errorf("%w: assignment already exists for this state and form version", assignmentdomain.ErrAssignmentExists)
	ErrInvalidWorkflowState    = fmt.Errorf("%w: workflow state key does not exist", assignmentdomain.ErrInvalidWorkflowState)
	ErrInvalidFormVersion      = fmt.Errorf("%w: form version not found or not published", assignmentdomain.ErrInvalidFormVersion)
	ErrTenantMismatch          = assignmentdomain.ErrTenantMismatch
	ErrFormArchived            = fmt.Errorf("form is archived and cannot be assigned")
	ErrDuplicateDisplayOrder   = fmt.Errorf("%w: display order already in use for this state", assignmentdomain.ErrDuplicateDisplayOrder)
)

type WorkflowStateFormAssignmentService struct {
	defRepo         workflowdomain.WorkflowDefinitionRepository
	formRepo        domain.FormRepository
	formVersionRepo domain.FormVersionRepository
	assignmentRepo  assignmentdomain.WorkflowStateFormAssignmentRepository
	auditor         auditdomain.EventRecorder
}

func NewWorkflowStateFormAssignmentService(
	defRepo workflowdomain.WorkflowDefinitionRepository,
	formRepo domain.FormRepository,
	formVersionRepo domain.FormVersionRepository,
	assignmentRepo assignmentdomain.WorkflowStateFormAssignmentRepository,
	auditor auditdomain.EventRecorder,
) *WorkflowStateFormAssignmentService {
	return &WorkflowStateFormAssignmentService{
		defRepo:         defRepo,
		formRepo:        formRepo,
		formVersionRepo: formVersionRepo,
		assignmentRepo:  assignmentRepo,
		auditor:         auditor,
	}
}

type CreateAssignmentParams struct {
	TenantID             uuid.UUID
	WorkflowDefinitionID uuid.UUID
	WorkflowStateKey     string
	FormID               uuid.UUID
	FormVersionID        uuid.UUID
	Required             bool
	DisplayOrder         int
	Active               bool
	CreatedBy            uuid.UUID
}

func (s *WorkflowStateFormAssignmentService) CreateAssignment(ctx context.Context, params CreateAssignmentParams) (*assignmentdomain.WorkflowStateFormAssignment, error) {
	if err := assignmentdomain.ValidateAssignment(params.TenantID, params.WorkflowDefinitionID, params.FormID, params.FormVersionID, params.WorkflowStateKey); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidWorkflowState, err)
	}

	workflowDef, err := s.defRepo.FindByID(ctx, params.TenantID, params.WorkflowDefinitionID)
	if err != nil {
		return nil, ErrWorkflowNotFound
	}

	if !stateExistsInWorkflow(workflowDef, params.WorkflowStateKey) {
		return nil, ErrInvalidWorkflowState
	}

	form, err := s.formRepo.FindByID(ctx, params.TenantID, params.FormID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFormVersion, err)
	}

	if form.Status == domain.FormStatusArchived {
		return nil, ErrFormArchived
	}

	formVersion, err := s.formVersionRepo.FindByID(ctx, params.TenantID, params.FormVersionID)
	if err != nil {
		return nil, ErrInvalidFormVersion
	}

	if formVersion.Status != domain.FormVersionStatusPublished {
		return nil, ErrInvalidFormVersion
	}

	if formVersion.FormID != params.FormID {
		return nil, ErrInvalidFormVersion
	}

	existingAssignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, params.TenantID, params.WorkflowDefinitionID, params.WorkflowStateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing assignments: %w", err)
	}

	for _, a := range existingAssignments {
		if a.FormVersionID == params.FormVersionID {
			return nil, ErrAssignmentAlreadyExists
		}
		if a.Active && a.DisplayOrder == params.DisplayOrder {
			return nil, ErrDuplicateDisplayOrder
		}
	}

	assignment, err := assignmentdomain.NewWorkflowStateFormAssignment(
		params.TenantID,
		params.WorkflowDefinitionID,
		params.FormID,
		params.FormVersionID,
		params.CreatedBy,
		params.WorkflowStateKey,
		params.Required,
		params.DisplayOrder,
	)
	if err != nil {
		return nil, err
	}

	assignment.Active = params.Active

	var savedAssignment *assignmentdomain.WorkflowStateFormAssignment
	err = database.InTransaction(ctx, s.defRepo.DB(), func(tx *sql.Tx) error {
		if err := s.assignmentRepo.SaveTx(ctx, tx, assignment); err != nil {
			return fmt.Errorf("failed to save assignment: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.TenantID,
				ActorID:        &params.CreatedBy,
				Action:         "workflow_form_assignment.created",
				Resource:       "workflow_form_assignment",
				ResourceID:     shared.StrPtr(assignment.ID.String()),
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"workflow_definition_id": params.WorkflowDefinitionID.String(),
					"workflow_state_key":     params.WorkflowStateKey,
					"form_id":                params.FormID.String(),
					"form_version_id":        params.FormVersionID.String(),
					"required":               params.Required,
					"display_order":          params.DisplayOrder,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		savedAssignment = assignment
		return nil
	})
	if err != nil {
		return nil, err
	}

	return savedAssignment, nil
}

type UpdateAssignmentParams struct {
	TenantID     uuid.UUID
	AssignmentID uuid.UUID
	Required     *bool
	DisplayOrder *int
	Active       *bool
}

func (s *WorkflowStateFormAssignmentService) UpdateAssignment(ctx context.Context, params UpdateAssignmentParams) (*assignmentdomain.WorkflowStateFormAssignment, error) {
	assignment, err := s.assignmentRepo.FindByID(ctx, params.TenantID, params.AssignmentID)
	if err != nil {
		return nil, ErrAssignmentNotFound
	}

	if params.Required != nil {
		assignment.Required = *params.Required
	}
	if params.Active != nil {
		assignment.Active = *params.Active
	}

	if params.DisplayOrder != nil {
		if *params.DisplayOrder < 0 {
			return nil, fmt.Errorf("display order must be non-negative")
		}

		existingAssignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, params.TenantID, assignment.WorkflowDefinitionID, assignment.WorkflowStateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing assignments: %w", err)
		}

		for _, a := range existingAssignments {
			if a.ID != assignment.ID && a.Active && a.DisplayOrder == *params.DisplayOrder {
				return nil, ErrDuplicateDisplayOrder
			}
		}

		assignment.DisplayOrder = *params.DisplayOrder
	}

	assignment.UpdatedAt = time.Now().UTC()

	var updatedAssignment *assignmentdomain.WorkflowStateFormAssignment
	err = database.InTransaction(ctx, s.defRepo.DB(), func(tx *sql.Tx) error {
		if err := s.assignmentRepo.UpdateTx(ctx, tx, assignment); err != nil {
			return fmt.Errorf("failed to update assignment: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.TenantID,
				ActorID:        nil,
				Action:         "workflow_form_assignment.updated",
				Resource:       "workflow_form_assignment",
				ResourceID:     shared.StrPtr(assignment.ID.String()),
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"workflow_definition_id": assignment.WorkflowDefinitionID.String(),
					"workflow_state_key":     assignment.WorkflowStateKey,
					"form_version_id":        assignment.FormVersionID.String(),
					"required":               assignment.Required,
					"display_order":          assignment.DisplayOrder,
					"active":                 assignment.Active,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		updatedAssignment = assignment
		return nil
	})
	if err != nil {
		return nil, err
	}

	return updatedAssignment, nil
}

func (s *WorkflowStateFormAssignmentService) DeleteAssignment(ctx context.Context, tenantID, id, actorID uuid.UUID) error {
	assignment, err := s.assignmentRepo.FindByID(ctx, tenantID, id)
	if err != nil {
		return ErrAssignmentNotFound
	}

	err = database.InTransaction(ctx, s.defRepo.DB(), func(tx *sql.Tx) error {
		if err := s.assignmentRepo.DeleteTx(ctx, tx, tenantID, id); err != nil {
			return fmt.Errorf("failed to delete assignment: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: tenantID,
				ActorID:        &actorID,
				Action:         "workflow_form_assignment.deleted",
				Resource:       "workflow_form_assignment",
				ResourceID:     shared.StrPtr(assignment.ID.String()),
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"workflow_definition_id": assignment.WorkflowDefinitionID.String(),
					"workflow_state_key":     assignment.WorkflowStateKey,
					"form_version_id":        assignment.FormVersionID.String(),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *WorkflowStateFormAssignmentService) GetAssignment(ctx context.Context, tenantID, id uuid.UUID) (*assignmentdomain.WorkflowStateFormAssignment, error) {
	assignment, err := s.assignmentRepo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, ErrAssignmentNotFound
	}
	return assignment, nil
}

func (s *WorkflowStateFormAssignmentService) ListAssignmentsByWorkflow(ctx context.Context, tenantID, workflowDefID uuid.UUID) ([]*assignmentdomain.WorkflowStateFormAssignment, error) {
	_, err := s.defRepo.FindByID(ctx, tenantID, workflowDefID)
	if err != nil {
		return nil, ErrWorkflowNotFound
	}

	assignments, err := s.assignmentRepo.ListByWorkflow(ctx, tenantID, workflowDefID)
	if err != nil {
		return nil, fmt.Errorf("failed to list assignments: %w", err)
	}
	return assignments, nil
}

func (s *WorkflowStateFormAssignmentService) GetAssignmentsForState(ctx context.Context, tenantID, workflowDefID uuid.UUID, stateKey string) ([]*assignmentdomain.WorkflowStateFormAssignment, error) {
	_, err := s.defRepo.FindByID(ctx, tenantID, workflowDefID)
	if err != nil {
		return nil, ErrWorkflowNotFound
	}

	assignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, tenantID, workflowDefID, stateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get assignments for state: %w", err)
	}
	return assignments, nil
}

func (s *WorkflowStateFormAssignmentService) GetFormsForCaseState(ctx context.Context, tenantID, workflowDefID uuid.UUID, stateKey string) ([]*assignmentdomain.WorkflowStateFormAssignment, error) {
	return s.GetAssignmentsForState(ctx, tenantID, workflowDefID, stateKey)
}

func stateExistsInWorkflow(def *workflowdomain.WorkflowDefinition, stateKey string) bool {
	for _, state := range def.States {
		if state.Key == stateKey {
			return true
		}
	}
	return false
}
