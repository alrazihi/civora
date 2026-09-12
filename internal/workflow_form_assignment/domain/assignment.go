package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type WorkflowStateFormAssignment struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	WorkflowDefinitionID uuid.UUID
	WorkflowStateKey     string
	FormID               uuid.UUID
	FormVersionID        uuid.UUID
	Required             bool
	DisplayOrder         int
	Active               bool
	CreatedBy            uuid.UUID
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func NewWorkflowStateFormAssignment(
	tenantID, workflowDefID, formID, formVersionID, createdBy uuid.UUID,
	stateKey string,
	required bool,
	displayOrder int,
) (*WorkflowStateFormAssignment, error) {
	if err := ValidateWorkflowStateKey(stateKey); err != nil {
		return nil, err
	}
	if displayOrder < 0 {
		return nil, fmt.Errorf("display order must be non-negative")
	}

	now := time.Now().UTC()
	return &WorkflowStateFormAssignment{
		ID:                   uuid.New(),
		TenantID:             tenantID,
		WorkflowDefinitionID: workflowDefID,
		WorkflowStateKey:     stateKey,
		FormID:               formID,
		FormVersionID:        formVersionID,
		Required:             required,
		DisplayOrder:         displayOrder,
		Active:               true,
		CreatedBy:            createdBy,
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

func ValidateWorkflowStateKey(key string) error {
	if key == "" {
		return fmt.Errorf("workflow state key is required")
	}
	return nil
}

func ValidateAssignment(tenantID, workflowDefID, formID, formVersionID uuid.UUID, stateKey string) error {
	if tenantID == uuid.Nil {
		return fmt.Errorf("tenant ID is required")
	}
	if workflowDefID == uuid.Nil {
		return fmt.Errorf("workflow definition ID is required")
	}
	if formID == uuid.Nil {
		return fmt.Errorf("form ID is required")
	}
	if formVersionID == uuid.Nil {
		return fmt.Errorf("form version ID is required")
	}
	if stateKey == "" {
		return ErrInvalidWorkflowState
	}
	return nil
}
