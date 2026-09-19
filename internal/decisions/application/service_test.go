package application

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"testing"

	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database/testdb"
	decisionsdomain "github.com/alrazihi/civora/internal/decisions/domain"
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDecisionRepo struct {
	items map[uuid.UUID]*decisionsdomain.Decision
}

func newMockDecisionRepo() *mockDecisionRepo {
	return &mockDecisionRepo{items: make(map[uuid.UUID]*decisionsdomain.Decision)}
}

func (m *mockDecisionRepo) DB() *sql.DB { return testdb.NewDB() }
func (m *mockDecisionRepo) Save(ctx context.Context, d *decisionsdomain.Decision) error {
	m.items[d.ID] = d
	return nil
}
func (m *mockDecisionRepo) SaveTx(ctx context.Context, tx *sql.Tx, d *decisionsdomain.Decision) error {
	return m.Save(ctx, d)
}
func (m *mockDecisionRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*decisionsdomain.Decision, error) {
	d, ok := m.items[id]
	if !ok || d.OrganizationID != orgID {
		return nil, errors.New("not found")
	}
	return d, nil
}
func (m *mockDecisionRepo) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*decisionsdomain.Decision, error) {
	for _, d := range m.items {
		if d.ServiceRequestID == serviceRequestID && d.OrganizationID == orgID {
			return d, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockDecisionRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*decisionsdomain.Decision, error) {
	return nil, nil
}
func (m *mockDecisionRepo) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockDecisionRepo) ListByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) ([]*decisionsdomain.Decision, error) {
	var result []*decisionsdomain.Decision
	for _, d := range m.items {
		if d.ServiceRequestID == serviceRequestID && d.OrganizationID == orgID {
			result = append(result, d)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("not found")
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result, nil
}

type mockCaseFinder struct {
	cases map[uuid.UUID]*domain.Case
}

func newMockCaseFinder() *mockCaseFinder {
	return &mockCaseFinder{cases: make(map[uuid.UUID]*domain.Case)}
}

func (m *mockCaseFinder) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Case, error) {
	c, ok := m.cases[id]
	if !ok || c.OrganizationID != orgID {
		return nil, errors.New("not found")
	}
	return c, nil
}

func (m *mockCaseFinder) addCase(c *domain.Case) {
	m.cases[c.ID] = c
}

type mockUserChecker struct {
	members map[uuid.UUID]map[uuid.UUID]bool
}

func newMockUserChecker() *mockUserChecker {
	return &mockUserChecker{members: make(map[uuid.UUID]map[uuid.UUID]bool)}
}

func (m *mockUserChecker) BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	if users, ok := m.members[orgID]; ok {
		return users[userID], nil
	}
	return false, nil
}

func (m *mockUserChecker) addMember(orgID, userID uuid.UUID) {
	if m.members[orgID] == nil {
		m.members[orgID] = make(map[uuid.UUID]bool)
	}
	m.members[orgID][userID] = true
}

type mockWorkflowService struct {
	instanceID  uuid.UUID
	state       string
	transitions map[string]workflowdomain.WorkflowTransition
}

func newMockWorkflowService(initialState string) *mockWorkflowService {
	return &mockWorkflowService{
		instanceID:  uuid.New(),
		state:       initialState,
		transitions: make(map[string]workflowdomain.WorkflowTransition),
	}
}

func (m *mockWorkflowService) GetInstanceByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
	return &workflowdomain.WorkflowInstance{ID: m.instanceID, TenantID: tenantID, CaseID: caseID, CurrentState: m.state}, nil
}

func (m *mockWorkflowService) GetValidTransitions(ctx context.Context, tenantID, instanceID uuid.UUID) ([]workflowdomain.WorkflowTransition, error) {
	var result []workflowdomain.WorkflowTransition
	for _, tr := range m.transitions {
		result = append(result, tr)
	}
	return result, nil
}

func (m *mockWorkflowService) ExecuteTransition(ctx context.Context, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error) {
	if tr, ok := m.transitions[params.TransitionKey]; ok {
		m.state = tr.ToState
		return &workflowdomain.WorkflowInstance{ID: m.instanceID, TenantID: params.TenantID, CurrentState: m.state}, nil
	}
	return nil, errors.New("transition not found")
}

func (m *mockWorkflowService) ExecuteTransitionInTx(ctx context.Context, tx *sql.Tx, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error) {
	return m.ExecuteTransition(ctx, params)
}

func (m *mockWorkflowService) addTransition(key, fromState, toState string) {
	m.transitions[key] = workflowdomain.WorkflowTransition{
		Key:       key,
		FromState: fromState,
		ToState:   toState,
		Active:    true,
	}
}

func TestMakeDecision_CrossTenantCase(t *testing.T) {
	caseFinder := newMockCaseFinder()
	wf := newMockWorkflowService("NEW")
	svc := NewDecisionService(newMockDecisionRepo(), caseFinder, newMockUserChecker(), nil, wf)

	org1 := uuid.New()
	org2 := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(org1, actorID, "Test", "Desc", "General", "Normal", nil)
	caseFinder.addCase(c)

	_, err := svc.MakeDecision(context.Background(), MakeDecisionParams{
		OrganizationID:   org2,
		ServiceRequestID: c.ID,
		Decision:         decisionsdomain.DecisionTypeApproved,
		Reason:           "Approved",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseNotFound)
}

func TestMakeDecision_CrossTenantUser(t *testing.T) {
	caseFinder := newMockCaseFinder()
	wf := newMockWorkflowService("NEW")
	userChecker := newMockUserChecker()
	svc := NewDecisionService(newMockDecisionRepo(), caseFinder, userChecker, nil, wf)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", "General", "Normal", nil)
	caseFinder.addCase(c)

	_, err := svc.MakeDecision(context.Background(), MakeDecisionParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Decision:         decisionsdomain.DecisionTypeApproved,
		Reason:           "Approved",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestMakeDecision_NoValidTransition(t *testing.T) {
	caseFinder := newMockCaseFinder()
	wf := newMockWorkflowService("NEW")
	userChecker := newMockUserChecker()
	svc := NewDecisionService(newMockDecisionRepo(), caseFinder, userChecker, nil, wf)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", "General", "Normal", nil)
	caseFinder.addCase(c)
	userChecker.addMember(orgID, actorID)

	_, err := svc.MakeDecision(context.Background(), MakeDecisionParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Decision:         decisionsdomain.DecisionTypeApproved,
		Reason:           "Approved",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseTransition)
}

func TestMakeDecision_TransitionsToApproved(t *testing.T) {
	caseFinder := newMockCaseFinder()
	wf := newMockWorkflowService("DECISION_PENDING")
	wf.addTransition("approve", "DECISION_PENDING", "APPROVED")
	userChecker := newMockUserChecker()
	svc := NewDecisionService(newMockDecisionRepo(), caseFinder, userChecker, nil, wf)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", "General", "Normal", nil)
	caseFinder.addCase(c)
	userChecker.addMember(orgID, actorID)

	_, err := svc.MakeDecision(context.Background(), MakeDecisionParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Decision:         decisionsdomain.DecisionTypeApproved,
		Reason:           "Approved",
		ActorID:          actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "APPROVED", wf.state)
}

func TestMakeDecision_TransitionsToRejected(t *testing.T) {
	caseFinder := newMockCaseFinder()
	wf := newMockWorkflowService("DECISION_PENDING")
	wf.addTransition("reject", "DECISION_PENDING", "REJECTED")
	userChecker := newMockUserChecker()
	svc := NewDecisionService(newMockDecisionRepo(), caseFinder, userChecker, nil, wf)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", "General", "Normal", nil)
	caseFinder.addCase(c)
	userChecker.addMember(orgID, actorID)

	_, err := svc.MakeDecision(context.Background(), MakeDecisionParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Decision:         decisionsdomain.DecisionTypeRejected,
		Reason:           "Rejected",
		ActorID:          actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "REJECTED", wf.state)
}

func TestMakeDecision_DuplicateDecision(t *testing.T) {
	caseFinder := newMockCaseFinder()
	wf := newMockWorkflowService("DECISION_PENDING")
	wf.addTransition("approve", "DECISION_PENDING", "APPROVED")
	userChecker := newMockUserChecker()
	repo := newMockDecisionRepo()
	svc := NewDecisionService(repo, caseFinder, userChecker, nil, wf)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", "General", "Normal", nil)
	caseFinder.addCase(c)
	userChecker.addMember(orgID, actorID)

	repo.items[uuid.New()] = &decisionsdomain.Decision{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Decision:         decisionsdomain.DecisionTypeApproved,
		DecisionMaker:    actorID,
	}

	_, err := svc.MakeDecision(context.Background(), MakeDecisionParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Decision:         decisionsdomain.DecisionTypeApproved,
		Reason:           "Approved",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDecisionInput)
}

func TestMakeDecision_TransitionsToEscalate(t *testing.T) {
	caseFinder := newMockCaseFinder()
	wf := newMockWorkflowService("DECISION_PENDING")
	wf.addTransition("escalate", "DECISION_PENDING", "ESCALATED")
	userChecker := newMockUserChecker()
	svc := NewDecisionService(newMockDecisionRepo(), caseFinder, userChecker, nil, wf)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", "General", "Normal", nil)
	caseFinder.addCase(c)
	userChecker.addMember(orgID, actorID)

	_, err := svc.MakeDecision(context.Background(), MakeDecisionParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Decision:         decisionsdomain.DecisionTypeEscalate,
		Reason:           "Escalated to senior review",
		ActorID:          actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "ESCALATED", wf.state)
}

func TestSupersedeDecision_CreatesNewVersion(t *testing.T) {
	caseFinder := newMockCaseFinder()
	wf := newMockWorkflowService("APPROVED")
	wf.addTransition("approve", "DECISION_PENDING", "APPROVED")
	userChecker := newMockUserChecker()
	repo := newMockDecisionRepo()
	svc := NewDecisionService(repo, caseFinder, userChecker, nil, wf)

	orgID := uuid.New()
	actorID := uuid.New()
	prevDecisionID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", "General", "Normal", nil)
	caseFinder.addCase(c)
	userChecker.addMember(orgID, actorID)

	prev := &decisionsdomain.Decision{
		ID:               prevDecisionID,
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Decision:         decisionsdomain.DecisionTypeApproved,
		DecisionMaker:    actorID,
		Version:          1,
	}
	repo.items[prevDecisionID] = prev

	_, err := svc.SupersedeDecision(context.Background(), SupersedeDecisionParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Decision:         decisionsdomain.DecisionTypeRejected,
		Reason:           "Reversed after review",
		ActorID:          actorID,
	})
	require.NoError(t, err)

	assert.Len(t, repo.items, 2)
	for _, d := range repo.items {
		if d.ID != prevDecisionID {
			assert.Equal(t, 2, d.Version)
			assert.Equal(t, decisionsdomain.DecisionTypeRejected, d.Decision)
			assert.NotNil(t, d.SupersededByID)
			assert.Equal(t, d.ID, *d.SupersededByID)
		}
	}
}

func TestSupersedeDecision_NoExistingDecision(t *testing.T) {
	caseFinder := newMockCaseFinder()
	wf := newMockWorkflowService("DECISION_PENDING")
	userChecker := newMockUserChecker()
	svc := NewDecisionService(newMockDecisionRepo(), caseFinder, userChecker, nil, wf)

	orgID := uuid.New()
	actorID := uuid.New()
	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", "General", "Normal", nil)
	caseFinder.addCase(c)
	userChecker.addMember(orgID, actorID)

	_, err := svc.SupersedeDecision(context.Background(), SupersedeDecisionParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Decision:         decisionsdomain.DecisionTypeRejected,
		Reason:           "Reversed",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDecisionInput)
}

func TestGetDecisionHistory(t *testing.T) {
	caseFinder := newMockCaseFinder()
	wf := newMockWorkflowService("DECISION_PENDING")
	userChecker := newMockUserChecker()
	repo := newMockDecisionRepo()
	svc := NewDecisionService(repo, caseFinder, userChecker, nil, wf)

	orgID := uuid.New()
	actorID := uuid.New()
	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", "General", "Normal", nil)
	caseFinder.addCase(c)

	first := &decisionsdomain.Decision{ID: uuid.New(), OrganizationID: orgID, ServiceRequestID: c.ID, Decision: decisionsdomain.DecisionTypeApproved, DecisionMaker: actorID, Version: 1}
	second := &decisionsdomain.Decision{ID: uuid.New(), OrganizationID: orgID, ServiceRequestID: c.ID, Decision: decisionsdomain.DecisionTypeRejected, DecisionMaker: actorID, Version: 2}
	repo.items[first.ID] = first
	repo.items[second.ID] = second

	history, err := svc.GetDecisionHistory(context.Background(), orgID, c.ID)
	require.NoError(t, err)
	assert.Len(t, history, 2)
	assert.Equal(t, first.ID, history[0].ID)
	assert.Equal(t, second.ID, history[1].ID)
}
