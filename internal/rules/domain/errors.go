package domain

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrRuleSetNotFound       = errors.New("rule set not found")
	ErrRuleSetInvalid        = errors.New("rule set is invalid")
	ErrRuleSetNotPublished   = errors.New("rule set is not published")
	ErrRuleSetArchived       = errors.New("rule set is archived")
	ErrRuleSetKeyExists      = errors.New("rule set key already exists")
	ErrRuleSetVersionExists  = errors.New("rule set version already exists")
	ErrRuleSetNoActive       = errors.New("no active (published) rule set found")
	ErrRuleSetNotDraft       = errors.New("rule set is not in DRAFT status")
	ErrEvaluationFailed      = errors.New("evaluation failed")
	ErrInvalidCondition      = errors.New("invalid condition")
	ErrInvalidOperator       = errors.New("invalid operator")
	ErrTypeMismatch          = errors.New("type mismatch in condition")
	ErrMaxDepthExceeded      = errors.New("maximum condition nesting depth exceeded")
	ErrTooManyConditions     = errors.New("too many conditions")
	ErrDuplicateRulePriority = errors.New("duplicate rule priority")
	ErrTenantViolation       = errors.New("tenant violation")
	ErrInvalidFactPath       = errors.New("invalid fact path")
)

// NewErrRuleSetNotFound returns a not-found error annotated with the id/version.
func NewErrRuleSetNotFound(id uuid.UUID, version int) error {
	return fmt.Errorf("%w: id %s version %d", ErrRuleSetNotFound, id, version)
}
