package application

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/alrazihi/civora/internal/cases/domain"
	decisionsdomain "github.com/alrazihi/civora/internal/decisions/domain"
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

func (m *mockDecisionRepo) DB() *sql.DB { return nil }
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

type mockCaseUpdater struct {
	mu       sync.RWMutex
	statuses map[uuid.UUID]domain.CaseStatus
}

func newMockCaseUpdater() *mockCaseUpdater {
	return &mockCaseUpdater{statuses: make(map[uuid.UUID]domain.CaseStatus)}
}

func (m *mockCaseUpdater) UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, status domain.CaseStatus, version int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statuses[id] = status
	return nil
}

func (m *mockCaseUpdater) getStatus(id uuid.UUID) domain.CaseStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.statuses[id]
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

func TestMakeDecision_CrossTenantCase(t *testing.T) {
	caseFinder := newMockCaseFinder()
	caseUpdater := newMockCaseUpdater()
	svc := NewDecisionService(newMockDecisionRepo(), caseUpdater, caseFinder, newMockUserChecker(), nil)

	org1 := uuid.New()
	org2 := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(org1, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
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
	caseUpdater := newMockCaseUpdater()
	userChecker := newMockUserChecker()
	svc := NewDecisionService(newMockDecisionRepo(), caseUpdater, caseFinder, userChecker, nil)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
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

func TestMakeDecision_InvalidCaseStatus(t *testing.T) {
	caseFinder := newMockCaseFinder()
	caseUpdater := newMockCaseUpdater()
	userChecker := newMockUserChecker()
	svc := NewDecisionService(newMockDecisionRepo(), caseUpdater, caseFinder, userChecker, nil)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	c.Status = domain.CaseStatusNew
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
	assert.ErrorIs(t, err, ErrInvalidCaseStatus)
}

func TestMakeDecision_AutoTransitionsToApproved(t *testing.T) {
	caseFinder := newMockCaseFinder()
	caseUpdater := newMockCaseUpdater()
	userChecker := newMockUserChecker()
	svc := NewDecisionService(newMockDecisionRepo(), caseUpdater, caseFinder, userChecker, nil)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	c.Status = domain.CaseStatusDecisionPending
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
	assert.Equal(t, domain.CaseStatusApproved, caseUpdater.getStatus(c.ID))
}

func TestMakeDecision_AutoTransitionsToRejected(t *testing.T) {
	caseFinder := newMockCaseFinder()
	caseUpdater := newMockCaseUpdater()
	userChecker := newMockUserChecker()
	svc := NewDecisionService(newMockDecisionRepo(), caseUpdater, caseFinder, userChecker, nil)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	c.Status = domain.CaseStatusDecisionPending
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
	assert.Equal(t, domain.CaseStatusRejected, caseUpdater.getStatus(c.ID))
}
