package application

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/audit/application"
	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/config"
	"github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubDefRepo struct {
	defs map[uuid.UUID]*domain.WorkflowDefinition
}

func (s *stubDefRepo) DB() *sql.DB { return nil }
func (s *stubDefRepo) Save(ctx context.Context, def *domain.WorkflowDefinition) error {
	s.defs[def.ID] = def
	return nil
}
func (s *stubDefRepo) SaveTx(ctx context.Context, tx *sql.Tx, def *domain.WorkflowDefinition) error {
	return s.Save(ctx, def)
}
func (s *stubDefRepo) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.WorkflowDefinition, error) {
	if d, ok := s.defs[id]; ok {
		return d, nil
	}
	return nil, domain.ErrWorkflowDefinitionNotFound{DefID: id}
}
func (s *stubDefRepo) FindByKeyAndVersion(ctx context.Context, tenantID uuid.UUID, key string, version int) (*domain.WorkflowDefinition, error) {
	for _, d := range s.defs {
		if d.TenantID == tenantID && d.Key == key && d.Version == version {
			return d, nil
		}
	}
	return nil, domain.ErrWorkflowDefinitionNotFound{DefID: uuid.Nil}
}
func (s *stubDefRepo) FindLatestActiveByKey(ctx context.Context, tenantID uuid.UUID, key string) (*domain.WorkflowDefinition, error) {
	for _, d := range s.defs {
		if d.TenantID == tenantID && d.Key == key && d.Status == domain.WorkflowStatusActive {
			return d, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (s *stubDefRepo) FindLatestActiveByKeyTx(ctx context.Context, tx *sql.Tx, tenantID uuid.UUID, key string) (*domain.WorkflowDefinition, error) {
	return s.FindLatestActiveByKey(ctx, tenantID, key)
}
func (s *stubDefRepo) FindByKey(ctx context.Context, tenantID uuid.UUID, key string) (*domain.WorkflowDefinition, error) {
	for _, d := range s.defs {
		if d.TenantID == tenantID && d.Key == key {
			return d, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (s *stubDefRepo) FindByKeyTx(ctx context.Context, tx *sql.Tx, tenantID uuid.UUID, key string) (*domain.WorkflowDefinition, error) {
	return s.FindByKey(ctx, tenantID, key)
}
func (s *stubDefRepo) ListByOrganization(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*domain.WorkflowDefinition, int, error) {
	return nil, 0, nil
}
func (s *stubDefRepo) ListActiveByOrganization(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*domain.WorkflowDefinition, int, error) {
	return nil, 0, nil
}
func (s *stubDefRepo) GetActiveByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.WorkflowDefinition, error) {
	if d, ok := s.defs[id]; ok && d.TenantID == tenantID && d.Status == domain.WorkflowStatusActive {
		return d, nil
	}
	return nil, domain.ErrWorkflowDefinitionNotFound{DefID: id}
}
func (s *stubDefRepo) UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status domain.WorkflowDefinitionStatus, version int) error {
	if d, ok := s.defs[id]; ok {
		d.Status = status
		return nil
	}
	return domain.ErrWorkflowDefinitionNotFound{DefID: id}
}
func (s *stubDefRepo) UpdateStatusTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID, status domain.WorkflowDefinitionStatus, version int) error {
	return s.UpdateStatus(ctx, tenantID, id, status, version)
}
func (s *stubDefRepo) FindByIDTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) (*domain.WorkflowDefinition, error) {
	return s.FindByID(ctx, tenantID, id)
}

// Update updates a workflow definition.
func (s *stubDefRepo) Update(ctx context.Context, def *domain.WorkflowDefinition) error {
	if _, ok := s.defs[def.ID]; ok {
		s.defs[def.ID] = def
		return nil
	}
	return domain.ErrWorkflowDefinitionNotFound{DefID: def.ID}
}

// UpdateTx updates a workflow definition within a transaction.
func (s *stubDefRepo) UpdateTx(ctx context.Context, tx *sql.Tx, def *domain.WorkflowDefinition) error {
	return s.Update(ctx, def)
}

// Delete deletes a workflow definition.
func (s *stubDefRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if _, ok := s.defs[id]; ok {
		delete(s.defs, id)
		return nil
	}
	return domain.ErrWorkflowDefinitionNotFound{DefID: id}
}

// DeleteTx deletes a workflow definition within a transaction.
func (s *stubDefRepo) DeleteTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) error {
	return s.Delete(ctx, tenantID, id)
}

type stubStateRepo struct {
	states map[uuid.UUID][]domain.WorkflowState
}

func (s *stubStateRepo) DB() *sql.DB { return nil }
func (s *stubStateRepo) SaveBatch(ctx context.Context, states []domain.WorkflowState) error {
	for _, st := range states {
		s.states[st.WorkflowDefID] = append(s.states[st.WorkflowDefID], st)
	}
	return nil
}
func (s *stubStateRepo) SaveBatchTx(ctx context.Context, tx *sql.Tx, states []domain.WorkflowState) error {
	return s.SaveBatch(ctx, states)
}
func (s *stubStateRepo) FindByDefinitionID(ctx context.Context, tenantID uuid.UUID, defID uuid.UUID) ([]domain.WorkflowState, error) {
	return s.states[defID], nil
}

// DeleteBatchByDefinitionID deletes all states for a workflow definition.
func (s *stubStateRepo) DeleteBatchByDefinitionID(ctx context.Context, tenantID uuid.UUID, defID uuid.UUID) error {
	delete(s.states, defID)
	return nil
}

// DeleteBatchByDefinitionIDTx deletes all states for a workflow definition within a transaction.
func (s *stubStateRepo) DeleteBatchByDefinitionIDTx(ctx context.Context, tx *sql.Tx, tenantID uuid.UUID, defID uuid.UUID) error {
	return s.DeleteBatchByDefinitionID(ctx, tenantID, defID)
}

type stubTransitionRepo struct {
	transitions map[uuid.UUID][]domain.WorkflowTransition
}

func (s *stubTransitionRepo) DB() *sql.DB { return nil }
func (s *stubTransitionRepo) SaveBatch(ctx context.Context, transitions []domain.WorkflowTransition) error {
	for _, t := range transitions {
		s.transitions[t.WorkflowDefID] = append(s.transitions[t.WorkflowDefID], t)
	}
	return nil
}
func (s *stubTransitionRepo) SaveBatchTx(ctx context.Context, tx *sql.Tx, transitions []domain.WorkflowTransition) error {
	return s.SaveBatch(ctx, transitions)
}
func (s *stubTransitionRepo) FindByDefinitionID(ctx context.Context, tenantID, defID uuid.UUID) ([]domain.WorkflowTransition, error) {
	return s.transitions[defID], nil
}
func (s *stubTransitionRepo) FindByFromState(ctx context.Context, tenantID uuid.UUID, defID uuid.UUID, fromState string) ([]domain.WorkflowTransition, error) {
	var result []domain.WorkflowTransition
	for _, t := range s.transitions[defID] {
		if t.FromState == fromState {
			result = append(result, t)
		}
	}
	return result, nil
}

// DeleteBatchByDefinitionID deletes all transitions for a workflow definition.
func (s *stubTransitionRepo) DeleteBatchByDefinitionID(ctx context.Context, tenantID uuid.UUID, defID uuid.UUID) error {
	delete(s.transitions, defID)
	return nil
}

// DeleteBatchByDefinitionIDTx deletes all transitions for a workflow definition within a transaction.
func (s *stubTransitionRepo) DeleteBatchByDefinitionIDTx(ctx context.Context, tx *sql.Tx, tenantID uuid.UUID, defID uuid.UUID) error {
	return s.DeleteBatchByDefinitionID(ctx, tenantID, defID)
}

type stubInstanceRepo struct {
	instances map[uuid.UUID]*domain.WorkflowInstance
	byCase    map[uuid.UUID]*domain.WorkflowInstance
}

func (s *stubInstanceRepo) DB() *sql.DB { return nil }
func (s *stubInstanceRepo) Save(ctx context.Context, instance *domain.WorkflowInstance) error {
	s.instances[instance.ID] = instance
	s.byCase[instance.CaseID] = instance
	return nil
}
func (s *stubInstanceRepo) SaveTx(ctx context.Context, tx *sql.Tx, instance *domain.WorkflowInstance) error {
	return s.Save(ctx, instance)
}
func (s *stubInstanceRepo) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.WorkflowInstance, error) {
	if i, ok := s.instances[id]; ok {
		return i, nil
	}
	return nil, domain.ErrWorkflowInstanceNotFound{InstanceID: id}
}
func (s *stubInstanceRepo) FindByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) (*domain.WorkflowInstance, error) {
	if i, ok := s.byCase[caseID]; ok {
		return i, nil
	}
	return nil, domain.ErrWorkflowInstanceNotFound{InstanceID: caseID}
}
func (s *stubInstanceRepo) FindByDefinitionID(ctx context.Context, tenantID, defID uuid.UUID, limit, offset int) ([]*domain.WorkflowInstance, int, error) {
	return nil, 0, nil
}
func (s *stubInstanceRepo) CountActiveByDefinitionID(ctx context.Context, tenantID, defID uuid.UUID) (int, error) {
	count := 0
	for _, i := range s.instances {
		if i.TenantID == tenantID && i.WorkflowDefID == defID && i.CompletedAt == nil {
			count++
		}
	}
	return count, nil
}
func (s *stubInstanceRepo) UpdateState(ctx context.Context, tenantID, id uuid.UUID, state string, version int) error {
	if i, ok := s.instances[id]; ok {
		i.CurrentState = state
		i.Version = version
		return nil
	}
	return domain.ErrWorkflowInstanceNotFound{InstanceID: id}
}
func (s *stubInstanceRepo) UpdateStateTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID, state string, version int) error {
	return s.UpdateState(ctx, tenantID, id, state, version)
}
func (s *stubInstanceRepo) Complete(ctx context.Context, tenantID, id uuid.UUID, version int) error {
	if i, ok := s.instances[id]; ok {
		now := time.Now().UTC()
		i.CompletedAt = &now
		i.Version = version
		return nil
	}
	return domain.ErrWorkflowInstanceNotFound{InstanceID: id}
}
func (s *stubInstanceRepo) CompleteTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID, version int) error {
	return s.Complete(ctx, tenantID, id, version)
}

type stubHistoryRepo struct {
	histories []domain.WorkflowTransitionHistory
}

func (s *stubHistoryRepo) DB() *sql.DB { return nil }
func (s *stubHistoryRepo) Save(ctx context.Context, history *domain.WorkflowTransitionHistory) error {
	s.histories = append(s.histories, *history)
	return nil
}
func (s *stubHistoryRepo) SaveTx(ctx context.Context, tx *sql.Tx, history *domain.WorkflowTransitionHistory) error {
	return s.Save(ctx, history)
}
func (s *stubHistoryRepo) FindByInstanceID(ctx context.Context, tenantID, instanceID uuid.UUID, limit, offset int) ([]domain.WorkflowTransitionHistory, error) {
	var result []domain.WorkflowTransitionHistory
	for _, h := range s.histories {
		if h.WorkflowInstanceID == instanceID {
			result = append(result, h)
		}
	}
	return result, nil
}
func (s *stubHistoryRepo) FindByCaseID(ctx context.Context, tenantID, caseID uuid.UUID, limit, offset int) ([]domain.WorkflowTransitionHistory, error) {
	var result []domain.WorkflowTransitionHistory
	for _, h := range s.histories {
		if h.CaseID == caseID {
			result = append(result, h)
		}
	}
	return result, nil
}

type stubAuditRepo struct {
	events []*auditdomain.AuditEvent
}

func (s *stubAuditRepo) DB() *sql.DB { return nil }
func (s *stubAuditRepo) Save(ctx context.Context, event *auditdomain.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}
func (s *stubAuditRepo) SaveTx(ctx context.Context, tx *sql.Tx, event *auditdomain.AuditEvent) error {
	return s.Save(ctx, event)
}
func (s *stubAuditRepo) GetLastHash(ctx context.Context, orgID uuid.UUID) (*string, error) {
	return nil, nil
}
func (s *stubAuditRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*auditdomain.AuditEvent, error) {
	return nil, nil
}
func (s *stubAuditRepo) FindByResource(ctx context.Context, orgID uuid.UUID, resourceID string) ([]*auditdomain.AuditEvent, error) {
	return nil, nil
}
func (s *stubAuditRepo) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	return 0, nil
}
func (s *stubAuditRepo) RecordEvent(ctx context.Context, orgID uuid.UUID, event *auditdomain.AuditEvent) error {
	return s.Save(ctx, event)
}
func (s *stubAuditRepo) RecordEventTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, event *auditdomain.AuditEvent) error {
	return s.RecordEvent(ctx, orgID, event)
}
func (s *stubAuditRepo) PurgeOld(ctx context.Context, olderThan time.Time) (int, error) {
	return 0, nil
}
func (s *stubAuditRepo) AllOrganizationIDs(ctx context.Context) ([]uuid.UUID, error) {
	return nil, nil
}

func TestWorkflowEngine_SecondWorkflowWithoutEngineChanges(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	tenantID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	defRepo := &stubDefRepo{defs: make(map[uuid.UUID]*domain.WorkflowDefinition)}
	stateRepo := &stubStateRepo{states: make(map[uuid.UUID][]domain.WorkflowState)}
	transitionRepo := &stubTransitionRepo{transitions: make(map[uuid.UUID][]domain.WorkflowTransition)}
	instanceRepo := &stubInstanceRepo{instances: make(map[uuid.UUID]*domain.WorkflowInstance), byCase: make(map[uuid.UUID]*domain.WorkflowInstance)}
	historyRepo := &stubHistoryRepo{histories: nil}
	auditRepo := &stubAuditRepo{}

	auditService := application.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})
	svc := NewWorkflowService(defRepo, stateRepo, transitionRepo, instanceRepo, historyRepo, auditService)

	// Define a completely different workflow: Education Assistance
	states := []domain.WorkflowState{
		{ID: uuid.New(), TenantID: tenantID, Key: "APPLICATION", Name: "Application Submitted", Terminal: false, DisplayOrder: 0, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "ELIGIBILITY_REVIEW", Name: "Eligibility Review", Terminal: false, DisplayOrder: 1, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "DOCUMENT_REVIEW", Name: "Document Review", Terminal: false, DisplayOrder: 2, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "ASSESSMENT", Name: "Assessment", Terminal: false, DisplayOrder: 3, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "DECISION", Name: "Decision", Terminal: false, DisplayOrder: 4, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "GRANT_PROCESSING", Name: "Grant Processing", Terminal: false, DisplayOrder: 5, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "FOLLOW_UP", Name: "Follow-up", Terminal: false, DisplayOrder: 6, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "CLOSED", Name: "Closed", Terminal: true, DisplayOrder: 7, CreatedAt: now},
	}

	transitions := []domain.WorkflowTransition{
		{ID: uuid.New(), TenantID: tenantID, Key: "start_review", Name: "Start Review", FromState: "APPLICATION", ToState: "ELIGIBILITY_REVIEW", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "review_docs", Name: "Review Documents", FromState: "ELIGIBILITY_REVIEW", ToState: "DOCUMENT_REVIEW", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "assess", Name: "Assess", FromState: "DOCUMENT_REVIEW", ToState: "ASSESSMENT", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "decide", Name: "Decide", FromState: "ASSESSMENT", ToState: "DECISION", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "process_grant", Name: "Process Grant", FromState: "DECISION", ToState: "GRANT_PROCESSING", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "follow_up", Name: "Follow Up", FromState: "GRANT_PROCESSING", ToState: "FOLLOW_UP", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "close", Name: "Close", FromState: "FOLLOW_UP", ToState: "CLOSED", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: tenantID, Key: "reject", Name: "Reject", FromState: "DECISION", ToState: "CLOSED", Active: true, CreatedAt: now},
	}

	def, err := svc.CreateWorkflowDefinition(ctx, CreateWorkflowDefinitionParams{
		TenantID:     tenantID,
		Key:          "education_assistance",
		Name:         "Education Assistance",
		Description:  "Education grant application workflow",
		Version:      1,
		InitialState: "APPLICATION",
		States:       states,
		Transitions:  transitions,
		Metadata:     map[string]interface{}{},
	})
	require.NoError(t, err)

	// Activate the definition
	err = svc.ActivateWorkflowDefinition(ctx, tenantID, def.ID, uuid.Nil)
	require.NoError(t, err)

	// Create an instance for a case
	instance, err := svc.CreateInstanceForCase(ctx, tenantID, caseID, "education_assistance", uuid.Nil)
	require.NoError(t, err)
	assert.Equal(t, "APPLICATION", instance.CurrentState)

	// Execute transitions through the generic engine
	steps := []struct {
		transition string
		expected   string
	}{
		{"start_review", "ELIGIBILITY_REVIEW"},
		{"review_docs", "DOCUMENT_REVIEW"},
		{"assess", "ASSESSMENT"},
		{"decide", "DECISION"},
		{"process_grant", "GRANT_PROCESSING"},
		{"follow_up", "FOLLOW_UP"},
		{"close", "CLOSED"},
	}

	for _, step := range steps {
		instance, err = svc.ExecuteTransition(ctx, ExecuteTransitionParams{
			TenantID:      tenantID,
			InstanceID:    instance.ID,
			TransitionKey: step.transition,
			ActorID:       actorID,
			ActorRole:     "staff",
		})
		require.NoError(t, err)
		assert.Equal(t, step.expected, instance.CurrentState)
	}

	// Verify terminal state prevents further transitions
	_, err = svc.ExecuteTransition(ctx, ExecuteTransitionParams{
		TenantID:      tenantID,
		InstanceID:    instance.ID,
		TransitionKey: "start_review",
		ActorID:       actorID,
		ActorRole:     "staff",
	})
	require.Error(t, err)
	var terminalErr domain.TerminalStateError
	require.ErrorAs(t, err, &terminalErr)

	// Verify history was recorded
	history, err := historyRepo.FindByCaseID(ctx, tenantID, caseID, 100, 0)
	require.NoError(t, err)
	assert.Len(t, history, 7)
}

func TestCreateInstanceForCaseByDefID_Validation(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	otherTenant := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	defRepo := &stubDefRepo{defs: make(map[uuid.UUID]*domain.WorkflowDefinition)}
	stateRepo := &stubStateRepo{states: make(map[uuid.UUID][]domain.WorkflowState)}
	transitionRepo := &stubTransitionRepo{transitions: make(map[uuid.UUID][]domain.WorkflowTransition)}
	instanceRepo := &stubInstanceRepo{instances: make(map[uuid.UUID]*domain.WorkflowInstance), byCase: make(map[uuid.UUID]*domain.WorkflowInstance)}
	historyRepo := &stubHistoryRepo{histories: nil}
	auditRepo := &stubAuditRepo{}

	auditService := application.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})
	svc := NewWorkflowService(defRepo, stateRepo, transitionRepo, instanceRepo, historyRepo, auditService)

	// Create an ACTIVE definition
	activeDef, _ := svc.CreateWorkflowDefinition(ctx, CreateWorkflowDefinitionParams{
		TenantID:     tenantID,
		Key:          "active_def",
		Name:         "Active Def",
		Description:  "desc",
		Version:      1,
		InitialState: "NEW",
		States:       []domain.WorkflowState{{ID: uuid.New(), TenantID: tenantID, Key: "NEW", Name: "New", Terminal: false, DisplayOrder: 0, CreatedAt: time.Now().UTC()}},
		Transitions:  nil,
		Metadata:     nil,
	})
	_ = svc.ActivateWorkflowDefinition(ctx, tenantID, activeDef.ID, actorID)

	// Create DRAFT definition
	draftDef, _ := svc.CreateWorkflowDefinition(ctx, CreateWorkflowDefinitionParams{
		TenantID:     tenantID,
		Key:          "draft_def",
		Name:         "Draft Def",
		Description:  "desc",
		Version:      1,
		InitialState: "NEW",
		States:       []domain.WorkflowState{{ID: uuid.New(), TenantID: tenantID, Key: "NEW", Name: "New", Terminal: false, DisplayOrder: 0, CreatedAt: time.Now().UTC()}},
		Transitions:  nil,
		Metadata:     nil,
	})

	// Create ARCHIVED definition
	archivedDef, _ := svc.CreateWorkflowDefinition(ctx, CreateWorkflowDefinitionParams{
		TenantID:     tenantID,
		Key:          "archived_def",
		Name:         "Archived Def",
		Description:  "desc",
		Version:      1,
		InitialState: "NEW",
		States:       []domain.WorkflowState{{ID: uuid.New(), TenantID: tenantID, Key: "NEW", Name: "New", Terminal: false, DisplayOrder: 0, CreatedAt: time.Now().UTC()}},
		Transitions:  nil,
		Metadata:     nil,
	})
	_ = svc.ArchiveWorkflowDefinition(ctx, tenantID, archivedDef.ID, actorID)

	// 1. ACTIVE definition should succeed
	_, err := svc.CreateInstanceForCaseByDefID(ctx, tenantID, caseID, activeDef.ID, actorID)
	assert.NoError(t, err)

	// 2. DRAFT definition should fail with ErrWorkflowDefinitionNotActive
	_, err = svc.CreateInstanceForCaseByDefID(ctx, tenantID, caseID, draftDef.ID, actorID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrWorkflowDefinitionNotActive{})

	// 3. ARCHIVED definition should fail with ErrWorkflowDefinitionNotActive
	_, err = svc.CreateInstanceForCaseByDefID(ctx, tenantID, caseID, archivedDef.ID, actorID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrWorkflowDefinitionNotActive{})

	// 4. Non-existent definition should fail with ErrWorkflowDefinitionNotFound
	unknownID := uuid.New()
	_, err = svc.CreateInstanceForCaseByDefID(ctx, tenantID, caseID, unknownID, actorID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrWorkflowDefinitionNotFound{DefID: unknownID})

	// 5. Cross-tenant definition should fail with ErrWorkflowDefinitionNotFound
	_, err = svc.CreateInstanceForCaseByDefID(ctx, tenantID, caseID, otherTenant, actorID)
	assert.Error(t, err)
}
