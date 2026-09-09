package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/alrazihi/civora/internal/cases/domain"
	eligibilitydomain "github.com/alrazihi/civora/internal/eligibility/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEligibilityRepo struct {
	items map[uuid.UUID]*eligibilitydomain.Eligibility
}

func newMockEligibilityRepo() *mockEligibilityRepo {
	return &mockEligibilityRepo{items: make(map[uuid.UUID]*eligibilitydomain.Eligibility)}
}

func (m *mockEligibilityRepo) DB() *sql.DB { return nil }
func (m *mockEligibilityRepo) Save(ctx context.Context, e *eligibilitydomain.Eligibility) error {
	m.items[e.ID] = e
	return nil
}
func (m *mockEligibilityRepo) SaveTx(ctx context.Context, tx *sql.Tx, e *eligibilitydomain.Eligibility) error {
	return m.Save(ctx, e)
}
func (m *mockEligibilityRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*eligibilitydomain.Eligibility, error) {
	e, ok := m.items[id]
	if !ok || e.OrganizationID != orgID {
		return nil, errors.New("not found")
	}
	return e, nil
}
func (m *mockEligibilityRepo) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*eligibilitydomain.Eligibility, error) {
	for _, e := range m.items {
		if e.ServiceRequestID == serviceRequestID && e.OrganizationID == orgID {
			return e, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockEligibilityRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*eligibilitydomain.Eligibility, error) {
	return nil, nil
}
func (m *mockEligibilityRepo) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	return 0, nil
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

func TestCreateEligibility_CrossTenantCase(t *testing.T) {
	caseFinder := newMockCaseFinder()
	svc := NewEligibilityService(newMockEligibilityRepo(), caseFinder, newMockUserChecker(), nil)

	org1 := uuid.New()
	org2 := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(org1, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	_, err := svc.CreateEligibility(context.Background(), CreateEligibilityParams{
		OrganizationID:   org2,
		ServiceRequestID: c.ID,
		Criteria:         map[string]interface{}{},
		Explanation:      "Test",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseNotFound)
}

func TestCreateEligibility_CrossTenantUser(t *testing.T) {
	caseFinder := newMockCaseFinder()
	userChecker := newMockUserChecker()
	svc := NewEligibilityService(newMockEligibilityRepo(), caseFinder, userChecker, nil)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	_, err := svc.CreateEligibility(context.Background(), CreateEligibilityParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Criteria:         map[string]interface{}{},
		Explanation:      "Test",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestCreateEligibility_ClosedCase(t *testing.T) {
	caseFinder := newMockCaseFinder()
	userChecker := newMockUserChecker()
	svc := NewEligibilityService(newMockEligibilityRepo(), caseFinder, userChecker, nil)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	c.Status = domain.CaseStatusClosed
	caseFinder.addCase(c)
	userChecker.addMember(orgID, actorID)

	_, err := svc.CreateEligibility(context.Background(), CreateEligibilityParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Criteria:         map[string]interface{}{},
		Explanation:      "Test",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseClosed)
}
