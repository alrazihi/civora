package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/google/uuid"
)

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// TransitionObserver is an optional generic extension point for applications
// that need to react to a transition before it commits.
type TransitionObserver interface {
	OnTransition(ctx context.Context, tx *sql.Tx, instance *domain.WorkflowInstance, transition *domain.WorkflowTransition) error
}

type TransitionObserverRegistrar interface {
	SetTransitionObserver(observer TransitionObserver)
}

// WorkflowService manages workflow definitions, instances, and transitions.
type WorkflowService struct {
	defRepo        domain.WorkflowDefinitionRepository
	stateRepo      domain.WorkflowStateRepository
	transitionRepo domain.WorkflowTransitionRepository
	instanceRepo   domain.WorkflowInstanceRepository
	historyRepo    domain.WorkflowTransitionHistoryRepository
	auditor        auditdomain.EventRecorder
	observer       TransitionObserver
}

func NewWorkflowService(
	defRepo domain.WorkflowDefinitionRepository,
	stateRepo domain.WorkflowStateRepository,
	transitionRepo domain.WorkflowTransitionRepository,
	instanceRepo domain.WorkflowInstanceRepository,
	historyRepo domain.WorkflowTransitionHistoryRepository,
	auditor auditdomain.EventRecorder,
) *WorkflowService {
	return &WorkflowService{
		defRepo:        defRepo,
		stateRepo:      stateRepo,
		transitionRepo: transitionRepo,
		instanceRepo:   instanceRepo,
		historyRepo:    historyRepo,
		auditor:        auditor,
	}
}

// SetTransitionObserver registers an optional observer invoked before a
// transition commits. Observers run in the same transaction as the transition.
func (s *WorkflowService) SetTransitionObserver(observer TransitionObserver) {
	s.observer = observer
}

// CreateWorkflowDefinitionParams holds parameters for creating a workflow definition.
type CreateWorkflowDefinitionParams struct {
	TenantID     uuid.UUID
	ActorID      uuid.UUID
	Key          string
	Name         string
	Description  string
	Version      int
	InitialState string
	States       []domain.WorkflowState
	Transitions  []domain.WorkflowTransition
	Metadata     map[string]interface{}
}

// UpdateWorkflowDefinitionParams holds parameters for updating a workflow definition.
type UpdateWorkflowDefinitionParams struct {
	TenantID     uuid.UUID
	ID           uuid.UUID
	ActorID      uuid.UUID
	Key          string
	Name         string
	Description  string
	Version      int
	InitialState string
	States       []domain.WorkflowState
	Transitions  []domain.WorkflowTransition
	Metadata     map[string]interface{}
}

// ExecuteTransitionParams holds parameters for executing a workflow transition.
type ExecuteTransitionParams struct {
	TenantID      uuid.UUID
	InstanceID    uuid.UUID
	TransitionKey string
	ActorID       uuid.UUID
	ActorRole     string
	Reason        string
}

// CreateWorkflowDefinition creates a new draft workflow definition with states and transitions.
func (s *WorkflowService) CreateWorkflowDefinition(ctx context.Context, params CreateWorkflowDefinitionParams) (*domain.WorkflowDefinition, error) {
	now := time.Now().UTC()
	def := &domain.WorkflowDefinition{
		ID:           uuid.New(),
		TenantID:     params.TenantID,
		Key:          params.Key,
		Name:         params.Name,
		Description:  params.Description,
		Version:      params.Version,
		Status:       domain.WorkflowStatusDraft,
		InitialState: params.InitialState,
		States:       params.States,
		Transitions:  params.Transitions,
		Metadata:     params.Metadata,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	validation := domain.ValidateWorkflowDefinition(def)
	if validation.HasErrors() {
		return nil, fmt.Errorf("invalid workflow definition: %v", validation.Errors)
	}

	if err := database.InTransaction(ctx, s.defRepo.DB(), func(tx *sql.Tx) error {
		existing, err := s.defRepo.FindByKeyTx(ctx, tx, params.TenantID, params.Key)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to check duplicate key: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("workflow definition with key %s already exists", params.Key)
		}

		if err := s.defRepo.SaveTx(ctx, tx, def); err != nil {
			return fmt.Errorf("failed to save workflow definition: %w", err)
		}
		for i := range def.States {
			if def.States[i].ID == uuid.Nil {
				def.States[i].ID = uuid.New()
			}
			def.States[i].WorkflowDefID = def.ID
			def.States[i].TenantID = def.TenantID
			if def.States[i].CreatedAt.IsZero() {
				def.States[i].CreatedAt = now
			}
		}
		if err := s.stateRepo.SaveBatchTx(ctx, tx, def.States); err != nil {
			return fmt.Errorf("failed to save workflow states: %w", err)
		}
		for i := range def.Transitions {
			if def.Transitions[i].ID == uuid.Nil {
				def.Transitions[i].ID = uuid.New()
			}
			def.Transitions[i].WorkflowDefID = def.ID
			def.Transitions[i].TenantID = def.TenantID
			if def.Transitions[i].CreatedAt.IsZero() {
				def.Transitions[i].CreatedAt = now
			}
		}
		if err := s.transitionRepo.SaveBatchTx(ctx, tx, def.Transitions); err != nil {
			return fmt.Errorf("failed to save workflow transitions: %w", err)
		}

		if s.auditor != nil {
			defIDStr := def.ID.String()
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: def.TenantID,
				ActorID:        &params.ActorID,
				Action:         "workflow.definition_created",
				Resource:       "workflow_definition",
				ResourceID:     &defIDStr,
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":     def.Key,
					"version": def.Version,
					"status":  string(def.Status),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return def, nil
}

// ActivateWorkflowDefinition activates a draft workflow definition.
func (s *WorkflowService) ActivateWorkflowDefinition(ctx context.Context, tenantID, id uuid.UUID, actorID uuid.UUID) error {
	if err := database.InTransaction(ctx, s.defRepo.DB(), func(tx *sql.Tx) error {
		def, err := s.defRepo.FindByIDTx(ctx, tx, tenantID, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domain.ErrWorkflowDefinitionNotFound{DefID: id}
			}
			return fmt.Errorf("workflow definition not found: %w", err)
		}
		if def.Status != domain.WorkflowStatusDraft {
			return fmt.Errorf("only draft definitions can be activated")
		}

		activeDef, err := s.defRepo.FindLatestActiveByKeyTx(ctx, tx, tenantID, def.Key)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to check active definitions: %w", err)
		}
		if activeDef != nil && activeDef.ID != def.ID {
			return fmt.Errorf("another active version (%d) already exists for key %s", activeDef.Version, def.Key)
		}

		def.Status = domain.WorkflowStatusActive
		def.UpdatedAt = time.Now().UTC()

		if err := s.defRepo.UpdateStatusTx(ctx, tx, tenantID, id, domain.WorkflowStatusActive, def.Version); err != nil {
			return fmt.Errorf("failed to activate workflow definition: %w", err)
		}

		if s.auditor != nil {
			defIDStr := def.ID.String()
			_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
				OrganizationID: tenantID,
				ActorID:        &actorID,
				Action:         "workflow.definition_activated",
				Resource:       "workflow_definition",
				ResourceID:     &defIDStr,
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":     def.Key,
					"version": def.Version,
				},
			})
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// ArchiveWorkflowDefinition archives an active workflow definition.
func (s *WorkflowService) ArchiveWorkflowDefinition(ctx context.Context, tenantID, id uuid.UUID, actorID uuid.UUID) error {
	if err := database.InTransaction(ctx, s.defRepo.DB(), func(tx *sql.Tx) error {
		def, err := s.defRepo.FindByIDTx(ctx, tx, tenantID, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domain.ErrWorkflowDefinitionNotFound{DefID: id}
			}
			return fmt.Errorf("workflow definition not found: %w", err)
		}
		if def.Status != domain.WorkflowStatusActive {
			return fmt.Errorf("only active definitions can be archived")
		}

		activeCount, err := s.instanceRepo.CountActiveByDefinitionID(ctx, tenantID, def.ID)
		if err != nil {
			return fmt.Errorf("failed to count active instances: %w", err)
		}
		if activeCount > 0 {
			return fmt.Errorf("cannot archive workflow definition with %d active instances", activeCount)
		}

		def.Status = domain.WorkflowStatusArchived
		def.UpdatedAt = time.Now().UTC()

		if err := s.defRepo.UpdateStatusTx(ctx, tx, tenantID, id, domain.WorkflowStatusArchived, def.Version); err != nil {
			return fmt.Errorf("failed to archive workflow definition: %w", err)
		}

		if s.auditor != nil {
			defIDStr := def.ID.String()
			_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
				OrganizationID: tenantID,
				ActorID:        &actorID,
				Action:         "workflow.definition_archived",
				Resource:       "workflow_definition",
				ResourceID:     &defIDStr,
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":     def.Key,
					"version": def.Version,
				},
			})
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// DeleteWorkflowDefinition deletes a draft workflow definition.
func (s *WorkflowService) DeleteWorkflowDefinition(ctx context.Context, tenantID, id uuid.UUID, actorID uuid.UUID) error {
	if err := database.InTransaction(ctx, s.defRepo.DB(), func(tx *sql.Tx) error {
		def, err := s.defRepo.FindByIDTx(ctx, tx, tenantID, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domain.ErrWorkflowDefinitionNotFound{DefID: id}
			}
			return fmt.Errorf("workflow definition not found: %w", err)
		}
		if def.Status != domain.WorkflowStatusDraft {
			return fmt.Errorf("only draft definitions can be deleted")
		}

		// Delete transitions first (foreign key constraint)
		if err := s.transitionRepo.DeleteBatchByDefinitionIDTx(ctx, tx, tenantID, def.ID); err != nil {
			return fmt.Errorf("failed to delete workflow transitions: %w", err)
		}

		// Delete states second (foreign key constraint)
		if err := s.stateRepo.DeleteBatchByDefinitionIDTx(ctx, tx, tenantID, def.ID); err != nil {
			return fmt.Errorf("failed to delete workflow states: %w", err)
		}

		// Delete the definition
		if err := s.defRepo.DeleteTx(ctx, tx, tenantID, id); err != nil {
			return fmt.Errorf("failed to delete workflow definition: %w", err)
		}

		// Record audit event
		if s.auditor != nil {
			defIDStr := def.ID.String()
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: tenantID,
				ActorID:        &actorID,
				Action:         "workflow.definition_deleted",
				Resource:       "workflow_definition",
				ResourceID:     &defIDStr,
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":     def.Key,
					"version": def.Version,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// UpdateWorkflowDefinition updates a draft workflow definition.
func (s *WorkflowService) UpdateWorkflowDefinition(ctx context.Context, params UpdateWorkflowDefinitionParams) (*domain.WorkflowDefinition, error) {
	var def *domain.WorkflowDefinition
	if err := database.InTransaction(ctx, s.defRepo.DB(), func(tx *sql.Tx) error {
		var err error
		def, err = s.defRepo.FindByIDTx(ctx, tx, params.TenantID, params.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domain.ErrWorkflowDefinitionNotFound{DefID: params.ID}
			}
			return fmt.Errorf("workflow definition not found: %w", err)
		}
		if def.Status != domain.WorkflowStatusDraft {
			return fmt.Errorf("only draft definitions can be updated")
		}

		// Check for duplicate key if key is being changed
		if params.Key != def.Key {
			existing, err := s.defRepo.FindByKeyTx(ctx, tx, params.TenantID, params.Key)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("failed to check duplicate key: %w", err)
			}
			if existing != nil && existing.ID != def.ID {
				return fmt.Errorf("workflow definition with key %s already exists", params.Key)
			}
		}

		// Update definition fields
		def.Key = params.Key
		def.Name = params.Name
		def.Description = params.Description
		def.Version = params.Version
		def.InitialState = params.InitialState
		if params.Metadata != nil {
			def.Metadata = params.Metadata
		}
		def.UpdatedAt = time.Now().UTC()

		// Validate the updated definition
		if validation := domain.ValidateWorkflowDefinition(def); validation.HasErrors() {
			return fmt.Errorf("invalid workflow definition: %v", validation.Errors)
		}

		// Save the updated definition
		if err := s.defRepo.UpdateTx(ctx, tx, def); err != nil {
			return fmt.Errorf("failed to save workflow definition: %w", err)
		}

		// Update states
		for i := range params.States {
			if params.States[i].ID == uuid.Nil {
				params.States[i].ID = uuid.New()
			}
			params.States[i].WorkflowDefID = def.ID
			params.States[i].TenantID = def.TenantID
		}
		if err := s.stateRepo.SaveBatchTx(ctx, tx, params.States); err != nil {
			return fmt.Errorf("failed to save workflow states: %w", err)
		}

		// Update transitions
		for i := range params.Transitions {
			if params.Transitions[i].ID == uuid.Nil {
				params.Transitions[i].ID = uuid.New()
			}
			params.Transitions[i].WorkflowDefID = def.ID
			params.Transitions[i].TenantID = def.TenantID
		}
		if err := s.transitionRepo.SaveBatchTx(ctx, tx, params.Transitions); err != nil {
			return fmt.Errorf("failed to save workflow transitions: %w", err)
		}

		// Record audit event
		if s.auditor != nil {
			defIDStr := def.ID.String()
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: def.TenantID,
				ActorID:        &params.ActorID,
				Action:         "workflow.definition_updated",
				Resource:       "workflow_definition",
				ResourceID:     &defIDStr,
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":     def.Key,
					"version": def.Version,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// Return the updated definition with states and transitions
	def.States = params.States
	def.Transitions = params.Transitions
	return def, nil
}

// GetWorkflowDefinition retrieves a workflow definition with its states and transitions.
func (s *WorkflowService) GetWorkflowDefinition(ctx context.Context, tenantID, id uuid.UUID) (*domain.WorkflowDefinition, error) {
	def, err := s.defRepo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("workflow definition not found: %w", err)
	}
	states, err := s.stateRepo.FindByDefinitionID(ctx, tenantID, def.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow states: %w", err)
	}
	def.States = states
	transitions, err := s.transitionRepo.FindByDefinitionID(ctx, tenantID, def.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow transitions: %w", err)
	}
	def.Transitions = transitions
	return def, nil
}

// ListWorkflowDefinitions lists workflow definitions for an organization.
func (s *WorkflowService) ListWorkflowDefinitions(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*domain.WorkflowDefinition, int, error) {
	definitions, total, err := s.defRepo.ListByOrganization(ctx, tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list workflow definitions: %w", err)
	}
	for _, def := range definitions {
		states, err := s.stateRepo.FindByDefinitionID(ctx, tenantID, def.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to load workflow states: %w", err)
		}
		def.States = states
		transitions, err := s.transitionRepo.FindByDefinitionID(ctx, tenantID, def.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to load workflow transitions: %w", err)
		}
		def.Transitions = transitions
	}
	return definitions, total, nil
}

// FindLatestActiveByKey finds the latest active workflow definition by key.
func (s *WorkflowService) FindLatestActiveByKey(ctx context.Context, tenantID uuid.UUID, key string) (*domain.WorkflowDefinition, error) {
	def, err := s.defRepo.FindLatestActiveByKey(ctx, tenantID, key)
	if err != nil {
		return nil, err
	}
	states, err := s.stateRepo.FindByDefinitionID(ctx, tenantID, def.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow states: %w", err)
	}
	def.States = states
	transitions, err := s.transitionRepo.FindByDefinitionID(ctx, tenantID, def.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow transitions: %w", err)
	}
	def.Transitions = transitions
	return def, nil
}

// CreateInstanceForCase creates a workflow instance for a case.
func (s *WorkflowService) CreateInstanceForCase(ctx context.Context, tenantID, caseID uuid.UUID, workflowDefKey string, actorID uuid.UUID) (*domain.WorkflowInstance, error) {
	var instance *domain.WorkflowInstance
	err := database.InTransaction(ctx, s.instanceRepo.DB(), func(tx *sql.Tx) error {
		var err error
		instance, err = s.createInstanceForCase(ctx, tx, tenantID, caseID, workflowDefKey, actorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return instance, nil
}

// CreateInstanceForCaseTx creates a workflow instance within an existing transaction.
func (s *WorkflowService) CreateInstanceForCaseTx(ctx context.Context, tx *sql.Tx, tenantID, caseID uuid.UUID, workflowDefKey string, actorID uuid.UUID) (*domain.WorkflowInstance, error) {
	return s.createInstanceForCase(ctx, tx, tenantID, caseID, workflowDefKey, actorID)
}

func (s *WorkflowService) createInstanceForCase(ctx context.Context, tx *sql.Tx, tenantID, caseID uuid.UUID, workflowDefKey string, actorID uuid.UUID) (*domain.WorkflowInstance, error) {
	def, err := s.FindLatestActiveByKey(ctx, tenantID, workflowDefKey)
	if err != nil {
		return nil, fmt.Errorf("workflow definition not found for key %s: %w", workflowDefKey, err)
	}

	now := time.Now().UTC()
	instance := &domain.WorkflowInstance{
		ID:                 uuid.New(),
		TenantID:           tenantID,
		WorkflowDefID:      def.ID,
		WorkflowDefVersion: def.Version,
		CaseID:             caseID,
		CurrentState:       def.InitialState,
		StartedAt:          now,
		Metadata:           map[string]interface{}{},
		Version:            1,
	}

	if err := s.instanceRepo.SaveTx(ctx, tx, instance); err != nil {
		return nil, fmt.Errorf("failed to create workflow instance: %w", err)
	}

	if s.auditor != nil {
		instanceIDStr := instance.ID.String()
		if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
			OrganizationID: tenantID,
			ActorID:        &actorID,
			Action:         "workflow.instance_created",
			Resource:       "workflow_instance",
			ResourceID:     &instanceIDStr,
			Outcome:        "success",
			RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
			Metadata: map[string]interface{}{
				"workflow_definition_id":      def.ID.String(),
				"workflow_definition_version": def.Version,
				"case_id":                     caseID.String(),
				"initial_state":               def.InitialState,
			},
		}); err != nil {
			return nil, fmt.Errorf("failed to record audit event: %w", err)
		}
	}

	instance.Definition = def
	return instance, nil
}

// ExecuteTransition executes a workflow transition by key.
func (s *WorkflowService) ExecuteTransition(ctx context.Context, params ExecuteTransitionParams) (*domain.WorkflowInstance, error) {
	return s.executeTransition(ctx, nil, params)
}

// ExecuteTransitionInTx executes a workflow transition within an existing transaction.
func (s *WorkflowService) ExecuteTransitionInTx(ctx context.Context, tx *sql.Tx, params ExecuteTransitionParams) (*domain.WorkflowInstance, error) {
	return s.executeTransition(ctx, tx, params)
}

func (s *WorkflowService) executeTransition(ctx context.Context, tx *sql.Tx, params ExecuteTransitionParams) (*domain.WorkflowInstance, error) {
	instance, err := s.instanceRepo.FindByID(ctx, params.TenantID, params.InstanceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrWorkflowInstanceNotFound{InstanceID: params.InstanceID}
		}
		return nil, fmt.Errorf("workflow instance not found: %w", err)
	}

	def, err := s.defRepo.FindByID(ctx, params.TenantID, instance.WorkflowDefID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrWorkflowDefinitionNotFound{DefID: instance.WorkflowDefID}
		}
		return nil, fmt.Errorf("workflow definition not found: %w", err)
	}
	if def.TenantID != params.TenantID {
		return nil, domain.ErrTenantViolation{}
	}

	states, err := s.stateRepo.FindByDefinitionID(ctx, params.TenantID, def.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow states: %w", err)
	}
	def.States = states

	transitions, err := s.transitionRepo.FindByDefinitionID(ctx, params.TenantID, def.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow transitions: %w", err)
	}
	def.Transitions = transitions

	statesMap := make(map[string]*domain.WorkflowState)
	for i := range def.States {
		statesMap[def.States[i].Key] = &def.States[i]
	}

	currentState, exists := statesMap[instance.CurrentState]
	if !exists {
		return nil, fmt.Errorf("instance in unknown state: %s", instance.CurrentState)
	}

	if currentState.Terminal {
		return nil, domain.TerminalStateError{State: instance.CurrentState}
	}

	var transition *domain.WorkflowTransition
	for i := range def.Transitions {
		t := &def.Transitions[i]
		if t.FromState == instance.CurrentState && t.Key == params.TransitionKey && t.Active {
			transition = t
			break
		}
	}
	if transition == nil {
		return nil, domain.ErrTransitionNotFound{FromState: instance.CurrentState, ToState: ""}
	}

	if len(transition.AllowedRoles) > 0 && params.ActorRole != "" {
		allowed := false
		for _, role := range transition.AllowedRoles {
			if role == params.ActorRole {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, domain.ErrUnauthorizedTransition{TransitionKey: transition.Key, AllowedRoles: transition.AllowedRoles}
		}
	}

	targetState, exists := statesMap[transition.ToState]
	if !exists {
		return nil, fmt.Errorf("transition targets unknown state: %s", transition.ToState)
	}

	now := time.Now().UTC()
	history := &domain.WorkflowTransitionHistory{
		ID:                 uuid.New(),
		TenantID:           params.TenantID,
		WorkflowInstanceID: instance.ID,
		CaseID:             instance.CaseID,
		FromState:          instance.CurrentState,
		ToState:            transition.ToState,
		TransitionKey:      transition.Key,
		ActorID:            &params.ActorID,
		OccurredAt:         now,
		Reason:             params.Reason,
		Metadata:           map[string]interface{}{},
	}

	auditParams := auditdomain.RecordEventParams{
		OrganizationID: params.TenantID,
		ActorID:        &params.ActorID,
		Action:         "workflow.transition",
		Resource:       "workflow_instance",
		ResourceID:     shared.StrPtr(instance.ID.String()),
		Outcome:        "success",
		RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
		Metadata: map[string]interface{}{
			"workflow_instance_id": instance.ID.String(),
			"transition":           transition.Key,
			"from_state":           history.FromState,
			"to_state":             history.ToState,
		},
	}
	if params.Reason != "" {
		auditParams.Metadata["reason"] = params.Reason
	}

	if tx != nil {
		if err := s.instanceRepo.UpdateStateTx(ctx, tx, params.TenantID, instance.ID, transition.ToState, instance.Version); err != nil {
			return nil, fmt.Errorf("failed to update workflow instance state: %w", err)
		}
		if err := s.historyRepo.SaveTx(ctx, tx, history); err != nil {
			return nil, fmt.Errorf("failed to save transition history: %w", err)
		}
		if targetState.Terminal {
			if err := s.instanceRepo.CompleteTx(ctx, tx, params.TenantID, instance.ID, instance.Version+1); err != nil {
				return nil, fmt.Errorf("failed to complete workflow instance: %w", err)
			}
		}
		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditParams); err != nil {
				return nil, fmt.Errorf("failed to record audit event: %w", err)
			}
		}
		if s.observer != nil {
			if err := s.observer.OnTransition(ctx, tx, instance, transition); err != nil {
				return nil, fmt.Errorf("workflow transition observer failed: %w", err)
			}
		}
	} else {
		if err := database.InTransaction(ctx, s.instanceRepo.DB(), func(tx *sql.Tx) error {
			if err := s.instanceRepo.UpdateStateTx(ctx, tx, params.TenantID, instance.ID, transition.ToState, instance.Version); err != nil {
				return fmt.Errorf("failed to update workflow instance state: %w", err)
			}
			if err := s.historyRepo.SaveTx(ctx, tx, history); err != nil {
				return fmt.Errorf("failed to save transition history: %w", err)
			}
			if targetState.Terminal {
				if err := s.instanceRepo.CompleteTx(ctx, tx, params.TenantID, instance.ID, instance.Version+1); err != nil {
					return fmt.Errorf("failed to complete workflow instance: %w", err)
				}
			}
			if s.auditor != nil {
				if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditParams); err != nil {
					return fmt.Errorf("failed to record audit event: %w", err)
				}
			}
			if s.observer != nil {
				if err := s.observer.OnTransition(ctx, tx, instance, transition); err != nil {
					return fmt.Errorf("workflow transition observer failed: %w", err)
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	instance.CurrentState = transition.ToState
	instance.Version++
	instance.Definition = def

	return instance, nil
}

// GetInstanceByCaseID retrieves the workflow instance for a case.
func (s *WorkflowService) GetInstanceByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) (*domain.WorkflowInstance, error) {
	instance, err := s.instanceRepo.FindByCaseID(ctx, tenantID, caseID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrWorkflowInstanceNotFound{InstanceID: caseID}, err)
	}
	return instance, nil
}

// GetValidTransitions returns all valid transitions from the current state of a workflow instance.
func (s *WorkflowService) GetValidTransitions(ctx context.Context, tenantID, instanceID uuid.UUID) ([]domain.WorkflowTransition, error) {
	instance, err := s.instanceRepo.FindByID(ctx, tenantID, instanceID)
	if err != nil {
		return nil, domain.ErrWorkflowInstanceNotFound{InstanceID: instanceID}
	}

	def, err := s.defRepo.FindByID(ctx, tenantID, instance.WorkflowDefID)
	if err != nil {
		return nil, domain.ErrWorkflowDefinitionNotFound{DefID: instance.WorkflowDefID}
	}

	states, err := s.stateRepo.FindByDefinitionID(ctx, tenantID, def.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow states: %w", err)
	}
	def.States = states

	transitions, err := s.transitionRepo.FindByFromState(ctx, tenantID, def.ID, instance.CurrentState)
	if err != nil {
		return nil, fmt.Errorf("failed to load transitions: %w", err)
	}

	statesMap := make(map[string]*domain.WorkflowState)
	for i := range def.States {
		statesMap[def.States[i].Key] = &def.States[i]
	}

	var valid []domain.WorkflowTransition
	for _, t := range transitions {
		if target, ok := statesMap[t.ToState]; ok {
			if !target.Terminal || instance.CurrentState == t.FromState {
				valid = append(valid, t)
			}
		}
	}
	return valid, nil
}

// GetWorkflowHistoryByCaseID returns workflow transition history for a case.
func (s *WorkflowService) GetWorkflowHistoryByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) ([]domain.WorkflowTransitionHistory, error) {
	histories, err := s.historyRepo.FindByCaseID(ctx, tenantID, caseID, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow history: %w", err)
	}
	return histories, nil
}
