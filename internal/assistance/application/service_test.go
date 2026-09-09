package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	assistancedomain "github.com/alrazihi/civora/internal/assistance/domain"
	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAssistanceRepo struct {
	items map[uuid.UUID]*assistancedomain.Assistance
}

func newMockAssistanceRepo() *mockAssistanceRepo {
	return &mockAssistanceRepo{items: make(map[uuid.UUID]*assistancedomain.Assistance)}
}

func (m *mockAssistanceRepo) DB() *sql.DB { return nil }
func (m *mockAssistanceRepo) Save(ctx context.Context, a *assistancedomain.Assistance) error {
	m.items[a.ID] = a
	return nil
}
func (m *mockAssistanceRepo) SaveTx(ctx context.Context, tx *sql.Tx, a *assistancedomain.Assistance) error {
	return m.Save(ctx, a)
}
func (m *mockAssistanceRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*assistancedomain.Assistance, error) {
	a, ok := m.items[id]
	if !ok || a.OrganizationID != orgID {
		return nil, errors.New("not found")
	}
	return a, nil
}
func (m *mockAssistanceRepo) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*assistancedomain.Assistance, error) {
	return nil, nil
}
func (m *mockAssistanceRepo) CountByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockAssistanceRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*assistancedomain.Assistance, error) {
	return nil, nil
}
func (m *mockAssistanceRepo) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
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

func TestCreateAssistance_CrossTenantCase(t *testing.T) {
	caseFinder := newMockCaseFinder()
	svc := NewAssistanceService(newMockAssistanceRepo(), caseFinder, newMockUserChecker(), nil)

	org1 := uuid.New()
	org2 := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(org1, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	_, err := svc.CreateAssistance(context.Background(), CreateAssistanceParams{
		OrganizationID:   org2,
		ServiceRequestID: c.ID,
		Type:             assistancedomain.AssistanceTypeFood,
		Description:      "Test",
		ResponsibleStaff: actorID,
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseNotFound)
}

func TestCreateAssistance_CrossTenantUser(t *testing.T) {
	caseFinder := newMockCaseFinder()
	userChecker := newMockUserChecker()
	svc := NewAssistanceService(newMockAssistanceRepo(), caseFinder, userChecker, nil)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	_, err := svc.CreateAssistance(context.Background(), CreateAssistanceParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Type:             assistancedomain.AssistanceTypeFood,
		Description:      "Test",
		ResponsibleStaff: actorID,
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestUpdateAssistanceStatus_InvalidAction(t *testing.T) {
	repo := newMockAssistanceRepo()
	caseFinder := newMockCaseFinder()
	svc := NewAssistanceService(repo, caseFinder, newMockUserChecker(), nil)

	orgID := uuid.New()
	actorID := uuid.New()
	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	a, _ := assistancedomain.NewAssistance(orgID, c.ID, actorID, assistancedomain.AssistanceTypeFood, "Test")
	repo.items[a.ID] = a

	_, err := svc.UpdateAssistanceStatus(context.Background(), orgID, a.ID, "invalid", actorID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAssistanceInput)
}
