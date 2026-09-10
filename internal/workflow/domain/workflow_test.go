package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestValidateWorkflowDefinition_ValidDefinition(t *testing.T) {
	def := &WorkflowDefinition{
		Key:          "emergency_assistance",
		Name:         "Emergency Assistance",
		Version:      1,
		InitialState: "NEW",
		States: []WorkflowState{
			{Key: "NEW", Name: "New", Terminal: false},
			{Key: "OPEN", Name: "Open", Terminal: false},
			{Key: "CLOSED", Name: "Closed", Terminal: true},
		},
		Transitions: []WorkflowTransition{
			{Key: "open", FromState: "NEW", ToState: "OPEN"},
			{Key: "close", FromState: "OPEN", ToState: "CLOSED"},
		},
	}

	result := ValidateWorkflowDefinition(def)
	assert.False(t, result.HasErrors(), "expected no validation errors, got: %v", result.Errors)
}

func TestValidateWorkflowDefinition_MissingKey(t *testing.T) {
	def := &WorkflowDefinition{
		Name:         "Emergency Assistance",
		Version:      1,
		InitialState: "NEW",
		States:       []WorkflowState{{Key: "NEW"}},
		Transitions:  []WorkflowTransition{{Key: "open", FromState: "NEW", ToState: "CLOSED"}},
	}

	result := ValidateWorkflowDefinition(def)
	assert.True(t, result.HasErrors())
	assert.Contains(t, result.Errors[0].Field, "key")
}

func TestValidateWorkflowDefinition_DuplicateStateKey(t *testing.T) {
	def := &WorkflowDefinition{
		Key:          "test",
		Name:         "Test",
		Version:      1,
		InitialState: "NEW",
		States: []WorkflowState{
			{Key: "NEW"},
			{Key: "NEW"},
		},
		Transitions: []WorkflowTransition{{Key: "t1", FromState: "NEW", ToState: "NEW"}},
	}

	result := ValidateWorkflowDefinition(def)
	assert.True(t, result.HasErrors())
	var found bool
	for _, e := range result.Errors {
		if e.Field == "states[].key" {
			found = true
		}
	}
	assert.True(t, found, "expected duplicate state key error")
}

func TestValidateWorkflowDefinition_TerminalStateWithOutgoingTransitions(t *testing.T) {
	def := &WorkflowDefinition{
		Key:          "test",
		Name:         "Test",
		Version:      1,
		InitialState: "NEW",
		States: []WorkflowState{
			{Key: "NEW", Terminal: false},
			{Key: "DONE", Terminal: true},
		},
		Transitions: []WorkflowTransition{
			{Key: "done", FromState: "NEW", ToState: "DONE", Active: true},
			{Key: "again", FromState: "DONE", ToState: "NEW", Active: true},
		},
	}

	result := ValidateWorkflowDefinition(def)
	assert.True(t, result.HasErrors())
	var found bool
	for _, e := range result.Errors {
		if e.Field == "states[DONE].terminal" {
			found = true
		}
	}
	assert.True(t, found, "expected terminal state with outgoing transitions error")
}

func TestValidateWorkflowDefinition_TransitionReferencesUnknownState(t *testing.T) {
	def := &WorkflowDefinition{
		Key:          "test",
		Name:         "Test",
		Version:      1,
		InitialState: "NEW",
		States: []WorkflowState{
			{Key: "NEW"},
		},
		Transitions: []WorkflowTransition{
			{Key: "to_unknown", FromState: "NEW", ToState: "UNKNOWN"},
		},
	}

	result := ValidateWorkflowDefinition(def)
	assert.True(t, result.HasErrors())
	var found bool
	for _, e := range result.Errors {
		if e.Field == "transitions[].to_state" {
			found = true
		}
	}
	assert.True(t, found, "expected unknown to_state error")
}

func TestValidateWorkflowDefinition_InitialStateDoesNotExist(t *testing.T) {
	def := &WorkflowDefinition{
		Key:          "test",
		Name:         "Test",
		Version:      1,
		InitialState: "NONEXISTENT",
		States: []WorkflowState{
			{Key: "NEW"},
		},
		Transitions: []WorkflowTransition{},
	}

	result := ValidateWorkflowDefinition(def)
	assert.True(t, result.HasErrors())
}

func TestValidateWorkflowDefinition_DuplicateTransitionKey(t *testing.T) {
	def := &WorkflowDefinition{
		Key:          "test",
		Name:         "Test",
		Version:      1,
		InitialState: "NEW",
		States: []WorkflowState{
			{Key: "NEW"},
			{Key: "OPEN"},
		},
		Transitions: []WorkflowTransition{
			{Key: "dupe", FromState: "NEW", ToState: "OPEN"},
			{Key: "dupe", FromState: "NEW", ToState: "OPEN"},
		},
	}

	result := ValidateWorkflowDefinition(def)
	assert.True(t, result.HasErrors())
}

func TestValidateStateTransition_ValidTransition(t *testing.T) {
	def := &WorkflowDefinition{
		InitialState: "NEW",
		States: []WorkflowState{
			{Key: "NEW"},
			{Key: "OPEN"},
			{Key: "IN_REVIEW"},
		},
		Transitions: []WorkflowTransition{
			{Key: "open", FromState: "NEW", ToState: "OPEN", Active: true},
			{Key: "review", FromState: "OPEN", ToState: "IN_REVIEW", Active: true},
		},
	}

	result := ValidateStateTransition(def, "NEW", "OPEN", "open")
	assert.False(t, result.HasErrors())
}

func TestValidateStateTransition_InvalidTransition(t *testing.T) {
	def := &WorkflowDefinition{
		InitialState: "NEW",
		States: []WorkflowState{
			{Key: "NEW"},
			{Key: "OPEN"},
			{Key: "CLOSED"},
		},
		Transitions: []WorkflowTransition{
			{Key: "open", FromState: "NEW", ToState: "OPEN", Active: true},
		},
	}

	result := ValidateStateTransition(def, "NEW", "CLOSED", "open")
	assert.True(t, result.HasErrors())
}

func TestValidateStateTransition_TerminalStateBlocked(t *testing.T) {
	def := &WorkflowDefinition{
		InitialState: "NEW",
		States: []WorkflowState{
			{Key: "NEW"},
			{Key: "OPEN"},
			{Key: "CLOSED", Terminal: true},
		},
		Transitions: []WorkflowTransition{
			{Key: "open", FromState: "NEW", ToState: "OPEN", Active: true},
			{Key: "close", FromState: "OPEN", ToState: "CLOSED", Active: true},
			{Key: "reopen", FromState: "CLOSED", ToState: "OPEN", Active: true},
		},
	}

	result := ValidateStateTransition(def, "CLOSED", "OPEN", "reopen")
	assert.True(t, result.HasErrors())
}

func TestValidateStateTransition_SameStateTransition(t *testing.T) {
	def := &WorkflowDefinition{
		InitialState: "NEW",
		States: []WorkflowState{
			{Key: "NEW"},
			{Key: "OPEN"},
		},
		Transitions: []WorkflowTransition{
			{Key: "open", FromState: "NEW", ToState: "OPEN", Active: true},
		},
	}

	result := ValidateStateTransition(def, "NEW", "NEW", "open")
	assert.True(t, result.HasErrors())
}

func TestErrWorkflowInstanceNotFound_Is(t *testing.T) {
	err1 := ErrWorkflowInstanceNotFound{InstanceID: uuid.New()}
	err2 := ErrWorkflowInstanceNotFound{InstanceID: uuid.New()}
	assert.True(t, err1.Is(err2))
	assert.True(t, err1.Is(ErrWorkflowInstanceNotFound{}))
}

func TestErrWorkflowDefinitionNotFound_Is(t *testing.T) {
	err1 := ErrWorkflowDefinitionNotFound{DefID: uuid.New()}
	err2 := ErrWorkflowDefinitionNotFound{DefID: uuid.New()}
	assert.True(t, err1.Is(err2))
	assert.True(t, err1.Is(ErrWorkflowDefinitionNotFound{}))
}

func TestErrTenantViolation_Is(t *testing.T) {
	err := ErrTenantViolation{}
	assert.True(t, err.Is(ErrTenantViolation{}))
}

func TestTerminalStateError_Is(t *testing.T) {
	err := TerminalStateError{State: "CLOSED"}
	assert.True(t, err.Is(TerminalStateError{State: "CLOSED"}))
}

func TestErrTransitionNotFound_Is(t *testing.T) {
	err := ErrTransitionNotFound{FromState: "NEW", ToState: "CLOSED"}
	assert.True(t, err.Is(ErrTransitionNotFound{FromState: "ANY", ToState: "ANY"}))
}

func TestErrUnauthorizedTransition_Is(t *testing.T) {
	err := ErrUnauthorizedTransition{TransitionKey: "approve", AllowedRoles: []string{"admin"}}
	assert.True(t, err.Is(ErrUnauthorizedTransition{TransitionKey: "any", AllowedRoles: nil}))
}

func TestErrWorkflowInstanceExists_Is(t *testing.T) {
	err := ErrWorkflowInstanceExists{CaseID: uuid.New()}
	assert.True(t, err.Is(ErrWorkflowInstanceExists{CaseID: uuid.Nil}))
}
