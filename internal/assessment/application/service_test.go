package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	assessmentdomain "github.com/alrazihi/civora/internal/assessment/domain"
	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAssessmentRepo struct {
	items map[uuid.UUID]*assessmentdomain.Assessment
}

func newMockAssessmentRepo() *mockAssessmentRepo {
	return &mockAssessmentRepo{items: make(map[uuid.UUID]*assessmentdomain.Assessment)}
}

func (m *mockAssessmentRepo) DB() *sql.DB { return nil }
func (m *mockAssessmentRepo) Save(ctx context.Context, a *assessmentdomain.Assessment) error {
	m.items[a.ID] = a
	return nil
}
func (m *mockAssessmentRepo) SaveTx(ctx context.Context, tx *sql.Tx, a *assessmentdomain.Assessment) error {
	return m.Save(ctx, a)
}
func (m *mockAssessmentRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*assessmentdomain.Assessment, error) {
	a, ok := m.items[id]
	if !ok || a.OrganizationID != orgID {
		return nil, errors.New("not found")
	}
	return a, nil
}
func (m *mockAssessmentRepo) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*assessmentdomain.Assessment, error) {
	for _, a := range m.items {
		if a.ServiceRequestID == serviceRequestID && a.OrganizationID == orgID {
			return a, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockAssessmentRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*assessmentdomain.Assessment, error) {
	return nil, nil
}
func (m *mockAssessmentRepo) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
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

func TestCreateAssessment_CrossTenantCase(t *testing.T) {
	caseFinder := newMockCaseFinder()
	svc := NewAssessmentService(newMockAssessmentRepo(), caseFinder, newMockUserChecker(), nil)

	org1 := uuid.New()
	org2 := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(org1, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	_, err := svc.CreateAssessment(context.Background(), CreateAssessmentParams{
		OrganizationID:   org2,
		ServiceRequestID: c.ID,
		Findings:         "Findings",
		NeedsIdentified:  "Needs",
		Recommendation:   "Rec",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseNotFound)
}

func TestCreateAssessment_CrossTenantUser(t *testing.T) {
	caseFinder := newMockCaseFinder()
	userChecker := newMockUserChecker()
	svc := NewAssessmentService(newMockAssessmentRepo(), caseFinder, userChecker, nil)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	_, err := svc.CreateAssessment(context.Background(), CreateAssessmentParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Findings:         "Findings",
		NeedsIdentified:  "Needs",
		Recommendation:   "Rec",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}
