package domain

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// WorkflowDefinitionStatus represents the lifecycle state of a workflow definition.
type WorkflowDefinitionStatus string

const (
	WorkflowStatusDraft    WorkflowDefinitionStatus = "DRAFT"
	WorkflowStatusActive   WorkflowDefinitionStatus = "ACTIVE"
	WorkflowStatusArchived WorkflowDefinitionStatus = "ARCHIVED"
)

// WorkflowDefinition describes how a service-delivery process operates.
type WorkflowDefinition struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Key          string
	Name         string
	Description  string
	Version      int
	Status       WorkflowDefinitionStatus
	InitialState string
	States       []WorkflowState
	Transitions  []WorkflowTransition
	Metadata     map[string]interface{}
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// WorkflowState represents a state within a workflow definition.
type WorkflowState struct {
	ID              uuid.UUID
	WorkflowDefID   uuid.UUID
	TenantID        uuid.UUID
	Key             string
	Name            string
	Description     string
	Category        string
	Terminal        bool
	DisplayOrder    int `json:"display_order"`
	ResponsibleRole string
	CreatedAt       time.Time
}

// WorkflowTransition represents a transition between two states.
type WorkflowTransition struct {
	ID            uuid.UUID
	WorkflowDefID uuid.UUID
	TenantID      uuid.UUID
	Key           string
	Name          string
	FromState     string `json:"from_state"`
	ToState       string `json:"to_state"`
	Description   string
	Conditions    []TransitionCondition
	AllowedRoles  []string
	Active        bool
	CreatedAt     time.Time
}

// TransitionCondition represents a condition that must be satisfied for a transition.
type TransitionCondition struct {
	Type     string
	Field    string
	Operator string
	Value    interface{}
}

// WorkflowInstance represents the execution of a workflow definition for a specific case.
type WorkflowInstance struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	WorkflowDefID      uuid.UUID
	WorkflowDefVersion int
	CaseID             uuid.UUID
	CurrentState       string
	StartedAt          time.Time
	CompletedAt        *time.Time
	Metadata           map[string]interface{}
	Version            int
	Definition         *WorkflowDefinition
}

// WorkflowTransitionHistory records a single transition execution.
type WorkflowTransitionHistory struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	WorkflowInstanceID uuid.UUID
	CaseID             uuid.UUID
	FromState          string
	ToState            string
	TransitionKey      string
	ActorID            *uuid.UUID
	OccurredAt         time.Time
	Reason             string
	Metadata           map[string]interface{}
}

// ValidationResult holds validation errors for a workflow definition.
type ValidationResult struct {
	Errors []WorkflowValidationError
}

func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// WorkflowValidationError describes a single validation failure.
type WorkflowValidationError struct {
	Field   string
	Message string
}

func (e WorkflowValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// Transition errors
var (
	ErrEmptyKey                = errors.New("key is required")
	ErrEmptyName               = errors.New("name is required")
	ErrInvalidVersion          = errors.New("version must be >= 1")
	ErrEmptyInitialState       = errors.New("initial_state is required")
	ErrNoStates                = errors.New("at least one state is required")
	ErrStateKeyRequired        = errors.New("state key is required")
	ErrTransitionKeyRequired   = errors.New("transition key is required")
	ErrDuplicateStateKey       = errors.New("duplicate state key")
	ErrDuplicateTransitionKey  = errors.New("duplicate transition key")
	ErrTransitionSourceMissing = errors.New("transition references unknown from_state")
	ErrTransitionTargetMissing = errors.New("transition references unknown to_state")
	ErrTerminalHasOutgoing     = errors.New("terminal state cannot have outgoing transitions")
	ErrDuplicateTransition     = errors.New("duplicate transition")
)

// Workflow instance errors
type (
	ErrWorkflowInstanceNotFound   struct{ InstanceID uuid.UUID }
	ErrWorkflowDefinitionNotFound struct{ DefID uuid.UUID }
	ErrTenantViolation            struct{}
	TerminalStateError            struct{ State string }
	ErrWorkflowInstanceExists     struct{ CaseID uuid.UUID }
	ErrConcurrentModification     struct{}
	ErrTransitionNotFound         struct{ FromState, ToState string }
	ErrUnauthorizedTransition     struct {
		TransitionKey string
		AllowedRoles  []string
	}
	ErrWorkflowDefinitionNotActive struct{ DefID uuid.UUID }
)

func (e ErrWorkflowInstanceNotFound) Error() string {
	return "workflow instance not found: " + e.InstanceID.String()
}

func (e ErrWorkflowInstanceNotFound) Is(target error) bool {
	_, ok := target.(ErrWorkflowInstanceNotFound)
	return ok
}

func (e ErrWorkflowDefinitionNotFound) Error() string {
	return "workflow definition not found: " + e.DefID.String()
}

func (e ErrWorkflowDefinitionNotFound) Is(target error) bool {
	_, ok := target.(ErrWorkflowDefinitionNotFound)
	return ok
}

func (e ErrTenantViolation) Error() string {
	return "tenant violation: resource belongs to a different organization"
}

func (e ErrTenantViolation) Is(target error) bool {
	_, ok := target.(ErrTenantViolation)
	return ok
}

func (e TerminalStateError) Error() string {
	return "cannot transition from terminal state: " + e.State
}

func (e TerminalStateError) Is(target error) bool {
	_, ok := target.(TerminalStateError)
	return ok
}

func (e ErrWorkflowDefinitionNotActive) Error() string {
	return "workflow definition not active: " + e.DefID.String()
}

func (e ErrWorkflowDefinitionNotActive) Is(target error) bool {
	_, ok := target.(ErrWorkflowDefinitionNotActive)
	return ok
}

func (e ErrWorkflowInstanceExists) Error() string {
	return "workflow instance already exists for case: " + e.CaseID.String()
}

func (e ErrWorkflowInstanceExists) Is(target error) bool {
	_, ok := target.(ErrWorkflowInstanceExists)
	return ok
}

func (ErrConcurrentModification) Error() string {
	return "workflow instance was modified concurrently"
}

func (ErrConcurrentModification) Is(target error) bool {
	_, ok := target.(ErrConcurrentModification)
	return ok
}

func (e ErrTransitionNotFound) Error() string {
	return "no valid transition from " + e.FromState + " to " + e.ToState
}

func (e ErrTransitionNotFound) Is(target error) bool {
	_, ok := target.(ErrTransitionNotFound)
	return ok
}

func (e ErrUnauthorizedTransition) Error() string {
	return "unauthorized for transition " + e.TransitionKey + ": required roles " + fmt.Sprintf("%v", e.AllowedRoles)
}

func (e ErrUnauthorizedTransition) Is(target error) bool {
	_, ok := target.(ErrUnauthorizedTransition)
	return ok
}

// ValidateWorkflowDefinition validates a workflow definition for internal consistency.
func ValidateWorkflowDefinition(def *WorkflowDefinition) *ValidationResult {
	result := &ValidationResult{}

	if def.Key == "" {
		result.Errors = append(result.Errors, WorkflowValidationError{Field: "key", Message: ErrEmptyKey.Error()})
	}
	if def.Name == "" {
		result.Errors = append(result.Errors, WorkflowValidationError{Field: "name", Message: ErrEmptyName.Error()})
	}
	if def.Version < 1 {
		result.Errors = append(result.Errors, WorkflowValidationError{Field: "version", Message: ErrInvalidVersion.Error()})
	}
	if def.InitialState == "" {
		result.Errors = append(result.Errors, WorkflowValidationError{Field: "initial_state", Message: ErrEmptyInitialState.Error()})
	}
	if len(def.States) == 0 {
		result.Errors = append(result.Errors, WorkflowValidationError{Field: "states", Message: ErrNoStates.Error()})
	}

	if len(result.Errors) > 0 {
		return result
	}

	stateKeys := make(map[string]bool)
	for _, state := range def.States {
		if state.Key == "" {
			result.Errors = append(result.Errors, WorkflowValidationError{Field: "states[].key", Message: ErrStateKeyRequired.Error()})
			continue
		}
		if stateKeys[state.Key] {
			result.Errors = append(result.Errors, WorkflowValidationError{Field: "states[].key", Message: fmt.Sprintf("%s: %s", ErrDuplicateStateKey.Error(), state.Key)})
		}
		stateKeys[state.Key] = true
	}

	transitionKeys := make(map[string]bool)
	seenTransitions := make(map[string]bool)
	for _, t := range def.Transitions {
		if t.Key == "" {
			result.Errors = append(result.Errors, WorkflowValidationError{Field: "transitions[].key", Message: ErrTransitionKeyRequired.Error()})
			continue
		}
		if transitionKeys[t.Key] {
			result.Errors = append(result.Errors, WorkflowValidationError{Field: "transitions[].key", Message: fmt.Sprintf("%s: %s", ErrDuplicateTransitionKey.Error(), t.Key)})
		}
		transitionKeys[t.Key] = true

		if !stateKeys[t.FromState] {
			result.Errors = append(result.Errors, WorkflowValidationError{Field: "transitions[].from_state", Message: fmt.Sprintf("%s: %s", ErrTransitionSourceMissing.Error(), t.FromState)})
		}
		if !stateKeys[t.ToState] {
			result.Errors = append(result.Errors, WorkflowValidationError{Field: "transitions[].to_state", Message: fmt.Sprintf("%s: %s", ErrTransitionTargetMissing.Error(), t.ToState)})
		}

		tk := t.FromState + "->" + t.ToState + "(" + t.Key + ")"
		if seenTransitions[tk] {
			result.Errors = append(result.Errors, WorkflowValidationError{Field: "transitions", Message: fmt.Sprintf("%s: %s", ErrDuplicateTransition.Error(), tk)})
		}
		seenTransitions[tk] = true
	}

	if !stateKeys[def.InitialState] {
		result.Errors = append(result.Errors, WorkflowValidationError{Field: "initial_state", Message: fmt.Sprintf("initial_state %q does not match any state key", def.InitialState)})
	}

	for _, state := range def.States {
		if state.Terminal {
			outgoing := 0
			for _, t := range def.Transitions {
				if t.FromState == state.Key && t.Active {
					outgoing++
				}
			}
			if outgoing > 0 {
				result.Errors = append(result.Errors, WorkflowValidationError{Field: "states[" + state.Key + "].terminal", Message: fmt.Sprintf("%s: %s has %d outgoing transition(s)", ErrTerminalHasOutgoing.Error(), state.Key, outgoing)})
			}
		}
	}

	return result
}

// ValidateStateTransition validates a single state transition against a definition.
func ValidateStateTransition(def *WorkflowDefinition, fromState, toState, transitionKey string) *ValidationResult {
	result := &ValidationResult{}

	if fromState == toState {
		result.Errors = append(result.Errors, WorkflowValidationError{Field: "transition", Message: "cannot transition to the same state"})
		return result
	}

	var from *WorkflowState
	for i := range def.States {
		if def.States[i].Key == fromState {
			from = &def.States[i]
			break
		}
	}
	if from == nil {
		result.Errors = append(result.Errors, WorkflowValidationError{Field: "from_state", Message: "unknown state: " + fromState})
		return result
	}

	if from.Terminal {
		result.Errors = append(result.Errors, WorkflowValidationError{Field: "from_state", Message: TerminalStateError{State: fromState}.Error()})
		return result
	}

	var transition *WorkflowTransition
	for i := range def.Transitions {
		t := &def.Transitions[i]
		if t.FromState == fromState && t.ToState == toState && t.Key == transitionKey && t.Active {
			transition = t
			break
		}
	}
	if transition == nil {
		result.Errors = append(result.Errors, WorkflowValidationError{Field: "transition", Message: ErrTransitionNotFound{FromState: fromState, ToState: toState}.Error()})
		return result
	}

	return result
}

// SortStatesByDisplayOrder sorts states by their display order.
func SortStatesByDisplayOrder(states []WorkflowState) {
	sort.Slice(states, func(i, j int) bool {
		if states[i].DisplayOrder != states[j].DisplayOrder {
			return states[i].DisplayOrder < states[j].DisplayOrder
		}
		return states[i].Key < states[j].Key
	})
}
