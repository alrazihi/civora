package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	intmid "github.com/alrazihi/civora/internal/middleware"
	rulesdomain "github.com/alrazihi/civora/internal/rules/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrRuleAssignmentConflict = errors.New("rule assignment conflict")
)

// RuleAssignmentService binds rule sets to workflow states so eligibility rules
// can be configured per workflow state without editing application source.
type RuleAssignmentService struct {
	repo        rulesdomain.RuleAssignmentRepository
	ruleSetRepo rulesdomain.RuleSetRepository
	auditor     auditdomain.EventRecorder
}

func NewRuleAssignmentService(
	repo rulesdomain.RuleAssignmentRepository,
	ruleSetRepo rulesdomain.RuleSetRepository,
	auditor auditdomain.EventRecorder,
) *RuleAssignmentService {
	return &RuleAssignmentService{repo: repo, ruleSetRepo: ruleSetRepo, auditor: auditor}
}

type CreateRuleAssignmentParams struct {
	OrganizationID   uuid.UUID
	WorkflowDefID    uuid.UUID
	WorkflowStateKey string
	RuleSetID        uuid.UUID
	Required         bool
	Active           bool
	DisplayOrder     int
	ActorID          uuid.UUID
}

func (s *RuleAssignmentService) CreateRuleAssignment(ctx context.Context, params CreateRuleAssignmentParams) (*rulesdomain.WorkflowStateRuleAssignment, error) {
	if params.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrRuleSetInvalid)
	}
	if params.WorkflowDefID == uuid.Nil {
		return nil, fmt.Errorf("%w: workflow_definition_id is required", ErrRuleSetInvalid)
	}
	if params.WorkflowStateKey == "" {
		return nil, fmt.Errorf("%w: workflow_state_key is required", ErrRuleSetInvalid)
	}
	if params.RuleSetID == uuid.Nil {
		return nil, fmt.Errorf("%w: rule_set_id is required", ErrRuleSetInvalid)
	}
	if params.DisplayOrder < 0 {
		return nil, fmt.Errorf("%w: display_order must be non-negative", ErrRuleSetInvalid)
	}

	// Enforce cross-tenant boundary: the referenced rule set must belong to
	// this organization.
	if _, err := s.ruleSetRepo.FindByID(ctx, params.OrganizationID, params.RuleSetID); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetNotFound, err)
	}

	assignment := &rulesdomain.WorkflowStateRuleAssignment{
		ID:               uuid.New(),
		OrganizationID:   params.OrganizationID,
		WorkflowDefID:    params.WorkflowDefID,
		WorkflowStateKey: params.WorkflowStateKey,
		RuleSetID:        params.RuleSetID,
		Required:         params.Required,
		Active:           params.Active,
		DisplayOrder:     params.DisplayOrder,
	}
	if err := assignment.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetInvalid, err)
	}

	now := time.Now().UTC()
	assignment.CreatedAt = now
	assignment.UpdatedAt = now

	var saved *rulesdomain.WorkflowStateRuleAssignment
	err := database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, assignment); err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: %v", ErrRuleAssignmentConflict, err)
			}
			return fmt.Errorf("failed to save rule assignment: %w", err)
		}
		if err := s.recordAudit(ctx, tx, params.ActorID, assignment.OrganizationID, "rule_assignment.created", assignment.ID, map[string]interface{}{
			"workflow_definition_id": assignment.WorkflowDefID.String(),
			"workflow_state_key":     assignment.WorkflowStateKey,
			"rule_set_id":            assignment.RuleSetID.String(),
		}); err != nil {
			return err
		}
		saved = assignment
		return nil
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

type UpdateRuleAssignmentParams struct {
	OrganizationID uuid.UUID
	ID             uuid.UUID
	RuleSetID      *uuid.UUID
	Required       *bool
	Active         *bool
	DisplayOrder   *int
	ActorID        uuid.UUID
}

func (s *RuleAssignmentService) UpdateRuleAssignment(ctx context.Context, params UpdateRuleAssignmentParams) (*rulesdomain.WorkflowStateRuleAssignment, error) {
	if params.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrRuleSetInvalid)
	}
	if params.ID == uuid.Nil {
		return nil, fmt.Errorf("%w: id is required", ErrRuleSetInvalid)
	}

	assignment, err := s.repo.FindByID(ctx, params.OrganizationID, params.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleAssignmentNotFound, err)
	}

	if params.RuleSetID != nil {
		// Cross-tenant boundary: new rule set must belong to this org.
		if _, err := s.ruleSetRepo.FindByID(ctx, params.OrganizationID, *params.RuleSetID); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRuleSetNotFound, err)
		}
		assignment.RuleSetID = *params.RuleSetID
	}
	if params.Required != nil {
		assignment.Required = *params.Required
	}
	if params.Active != nil {
		assignment.Active = *params.Active
	}
	if params.DisplayOrder != nil {
		assignment.DisplayOrder = *params.DisplayOrder
	}
	assignment.UpdatedAt = time.Now().UTC()
	if err := assignment.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetInvalid, err)
	}

	var updated *rulesdomain.WorkflowStateRuleAssignment
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateTx(ctx, tx, assignment); err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: %v", ErrRuleAssignmentConflict, err)
			}
			return fmt.Errorf("failed to update rule assignment: %w", err)
		}
		if err := s.recordAudit(ctx, tx, params.ActorID, assignment.OrganizationID, "rule_assignment.updated", assignment.ID, map[string]interface{}{
			"rule_set_id":   assignment.RuleSetID.String(),
			"display_order": assignment.DisplayOrder,
		}); err != nil {
			return err
		}
		updated = assignment
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *RuleAssignmentService) GetRuleAssignment(ctx context.Context, orgID, id uuid.UUID) (*rulesdomain.WorkflowStateRuleAssignment, error) {
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrRuleSetInvalid)
	}
	a, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleAssignmentNotFound, err)
	}
	return a, nil
}

func (s *RuleAssignmentService) ListRuleAssignments(ctx context.Context, orgID, workflowDefID uuid.UUID) ([]*rulesdomain.WorkflowStateRuleAssignment, error) {
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrRuleSetInvalid)
	}
	if workflowDefID == uuid.Nil {
		return nil, fmt.Errorf("%w: workflow_definition_id is required", ErrRuleSetInvalid)
	}
	return s.repo.ListByWorkflow(ctx, orgID, workflowDefID)
}

func (s *RuleAssignmentService) DeleteRuleAssignment(ctx context.Context, orgID, id, actorID uuid.UUID) error {
	if orgID == uuid.Nil {
		return fmt.Errorf("%w: organization_id is required", ErrRuleSetInvalid)
	}
	if id == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrRuleSetInvalid)
	}
	if _, err := s.repo.FindByID(ctx, orgID, id); err != nil {
		return fmt.Errorf("%w: %v", ErrRuleAssignmentNotFound, err)
	}

	return database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.DeleteTx(ctx, tx, orgID, id); err != nil {
			return fmt.Errorf("failed to delete rule assignment: %w", err)
		}
		return s.recordAudit(ctx, tx, actorID, orgID, "rule_assignment.deleted", id, nil)
	})
}

func (s *RuleAssignmentService) recordAudit(ctx context.Context, tx *sql.Tx, actorID, orgID uuid.UUID, action string, resourceID uuid.UUID, metadata map[string]interface{}) error {
	if s.auditor == nil {
		return nil
	}
	return shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
		OrganizationID: orgID,
		ActorID:        &actorID,
		Action:         action,
		Resource:       "rule_assignment",
		ResourceID:     shared.StrPtr(resourceID.String()),
		Outcome:        "success",
		RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
		Metadata:       metadata,
	})
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "23505")
}
