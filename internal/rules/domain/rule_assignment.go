package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type WorkflowStateRuleAssignment struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	WorkflowDefID    uuid.UUID
	WorkflowStateKey string
	RuleSetID        uuid.UUID
	Required         bool
	Active           bool
	DisplayOrder     int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (w *WorkflowStateRuleAssignment) Validate() error {
	if w.OrganizationID == uuid.Nil {
		return fmt.Errorf("organization_id is required")
	}
	if w.WorkflowDefID == uuid.Nil {
		return fmt.Errorf("workflow_definition_id is required")
	}
	if w.WorkflowStateKey == "" {
		return fmt.Errorf("workflow_state_key is required")
	}
	if w.RuleSetID == uuid.Nil {
		return fmt.Errorf("rule_set_id is required")
	}
	if w.DisplayOrder < 0 {
		return fmt.Errorf("display_order must be non-negative")
	}
	return nil
}
