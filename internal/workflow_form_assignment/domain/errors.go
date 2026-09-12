package domain

import (
	"errors"
)

var (
	ErrAssignmentNotFound         = errors.New("assignment not found")
	ErrAssignmentExists           = errors.New("assignment already exists for this state and form version")
	ErrInvalidWorkflowState       = errors.New("invalid workflow state key")
	ErrInvalidFormVersion         = errors.New("invalid form version")
	ErrFormNotPublished           = errors.New("form version is not published")
	ErrTenantMismatch             = errors.New("tenant mismatch")
	ErrDuplicateDisplayOrder      = errors.New("display order already in use for this state")
	ErrWorkflowDefinitionNotFound = errors.New("workflow definition not found")
)
