package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/alrazihi/civora/internal/cases/domain"
	evidencedomain "github.com/alrazihi/civora/internal/evidence/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEvidenceRepo struct {
	items map[uuid.UUID]*evidencedomain.Evidence
}

func newMockEvidenceRepo() *mockEvidenceRepo {
	return &mockEvidenceRepo{items: make(map[uuid.UUID]*evidencedomain.Evidence)}
}

func (m *mockEvidenceRepo) DB() *sql.DB { return nil }
func (m *mockEvidenceRepo) Save(ctx context.Context, e *evidencedomain.Evidence) error {
	m.items[e.ID] = e
	return nil
}
func (m *mockEvidenceRepo) SaveTx(ctx context.Context, tx *sql.Tx, e *evidencedomain.Evidence) error {
	return m.Save(ctx, e)
}
func (m *mockEvidenceRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*evidencedomain.Evidence, error) {
	e, ok := m.items[id]
	if !ok || e.OrganizationID != orgID {
		return nil, errors.New("not found")
	}
	return e, nil
}
func (m *mockEvidenceRepo) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*evidencedomain.Evidence, error) {
	return nil, nil
}
func (m *mockEvidenceRepo) CountByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockEvidenceRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*evidencedomain.Evidence, error) {
	return nil, nil
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

func TestAddEvidence_CrossTenantCase(t *testing.T) {
	caseFinder := newMockCaseFinder()
	svc := NewEvidenceService(newMockEvidenceRepo(), caseFinder, newMockUserChecker(), nil)

	org1 := uuid.New()
	org2 := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(org1, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	_, err := svc.AddEvidence(context.Background(), AddEvidenceParams{
		OrganizationID:   org2,
		ServiceRequestID: c.ID,
		Type:             evidencedomain.EvidenceTypeStaffNote,
		Description:      "Test",
		StorageReference: "s3://bucket/test",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseNotFound)
}

func TestAddEvidence_CrossTenantUser(t *testing.T) {
	caseFinder := newMockCaseFinder()
	userChecker := newMockUserChecker()
	svc := NewEvidenceService(newMockEvidenceRepo(), caseFinder, userChecker, nil)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	_, err := svc.AddEvidence(context.Background(), AddEvidenceParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Type:             evidencedomain.EvidenceTypeStaffNote,
		Description:      "Test",
		StorageReference: "s3://bucket/test",
		ActorID:          actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}
