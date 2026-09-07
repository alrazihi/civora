package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/cases/domain"
	followupdomain "github.com/alrazihi/civora/internal/followup/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFollowUpRepo struct {
	items map[uuid.UUID]*followupdomain.FollowUp
}

func newMockFollowUpRepo() *mockFollowUpRepo {
	return &mockFollowUpRepo{items: make(map[uuid.UUID]*followupdomain.FollowUp)}
}

func (m *mockFollowUpRepo) DB() *sql.DB { return nil }
func (m *mockFollowUpRepo) Save(ctx context.Context, f *followupdomain.FollowUp) error {
	m.items[f.ID] = f
	return nil
}
func (m *mockFollowUpRepo) SaveTx(ctx context.Context, tx *sql.Tx, f *followupdomain.FollowUp) error {
	return m.Save(ctx, f)
}
func (m *mockFollowUpRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*followupdomain.FollowUp, error) {
	f, ok := m.items[id]
	if !ok || f.OrganizationID != orgID {
		return nil, errors.New("not found")
	}
	return f, nil
}
func (m *mockFollowUpRepo) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*followupdomain.FollowUp, error) {
	return nil, nil
}
func (m *mockFollowUpRepo) CountByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockFollowUpRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*followupdomain.FollowUp, error) {
	return nil, nil
}
func (m *mockFollowUpRepo) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
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

func TestCreateFollowUp_CrossTenantCase(t *testing.T) {
	caseFinder := newMockCaseFinder()
	svc := NewFollowUpService(newMockFollowUpRepo(), caseFinder, newMockUserChecker(), nil)

	org1 := uuid.New()
	org2 := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(org1, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	_, err := svc.CreateFollowUp(context.Background(), CreateFollowUpParams{
		OrganizationID:   org2,
		ServiceRequestID: c.ID,
		ScheduledDate:    time.Now(),
		Outcome:          "Outcome",
		Notes:            "Notes",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseNotFound)
}

func TestCreateFollowUp_CrossTenantUser(t *testing.T) {
	caseFinder := newMockCaseFinder()
	userChecker := newMockUserChecker()
	svc := NewFollowUpService(newMockFollowUpRepo(), caseFinder, userChecker, nil)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	c.Status = domain.CaseStatusApproved
	caseFinder.addCase(c)

	_, err := svc.CreateFollowUp(context.Background(), CreateFollowUpParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		ScheduledDate:    time.Now(),
		Outcome:          "Outcome",
		Notes:            "Notes",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestCreateFollowUp_InvalidCaseStatus(t *testing.T) {
	caseFinder := newMockCaseFinder()
	svc := NewFollowUpService(newMockFollowUpRepo(), caseFinder, newMockUserChecker(), nil)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	c.Status = domain.CaseStatusNew
	caseFinder.addCase(c)

	_, err := svc.CreateFollowUp(context.Background(), CreateFollowUpParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		ScheduledDate:    time.Now(),
		Outcome:          "Outcome",
		Notes:            "Notes",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFollowUpInput)
}
