package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCaseRepository struct {
	mu    sync.RWMutex
	cases map[uuid.UUID]*domain.Case
}

func newMockCaseRepo() *mockCaseRepository {
	return &mockCaseRepository{
		cases: make(map[uuid.UUID]*domain.Case),
	}
}

func (m *mockCaseRepository) Save(ctx context.Context, c *domain.Case) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cases[c.ID] = c
	return nil
}

func (m *mockCaseRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Case, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.cases[id]
	if !ok || c.OrganizationID != orgID {
		return nil, errors.New("not found")
	}
	return c, nil
}

func (m *mockCaseRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Case, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Case
	for _, c := range m.cases {
		if c.OrganizationID == orgID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockCaseRepository) UpdateStatus(ctx context.Context, orgID, id uuid.UUID, status domain.CaseStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.cases[id]
	if !ok || c.OrganizationID != orgID {
		return errors.New("not found")
	}
	c.Status = status
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockCaseRepository) Assign(ctx context.Context, orgID, id, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.cases[id]
	if !ok || c.OrganizationID != orgID {
		return errors.New("not found")
	}
	c.AssignedToID = &userID
	return nil
}

func TestCreateCase(t *testing.T) {
	repo := newMockCaseRepo()
	svc := NewCaseService(repo, nil)

	orgID := uuid.New()
	userID := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Emergency Food Request",
		Description:    "Family needs emergency food assistance",
		CreatedByID:    userID,
	})

	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, orgID, c.OrganizationID)
	assert.Equal(t, "Emergency Food Request", c.Title)
	assert.Equal(t, domain.CaseStatusCreated, c.Status)
	assert.NotEmpty(t, c.CaseNumber)
	assert.Equal(t, userID, c.CreatedByID)
}

func TestCaseLifecycle(t *testing.T) {
	repo := newMockCaseRepo()
	svc := NewCaseService(repo, nil)

	orgID := uuid.New()
	userID := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Emergency Assistance",
		Description:    "Need help",
		CreatedByID:    userID,
	})
	require.NoError(t, err)

	transitions := []struct {
		from domain.CaseStatus
		to   domain.CaseStatus
	}{
		{domain.CaseStatusCreated, domain.CaseStatusOpen},
		{domain.CaseStatusOpen, domain.CaseStatusInReview},
		{domain.CaseStatusInReview, domain.CaseStatusResolved},
		{domain.CaseStatusResolved, domain.CaseStatusClosed},
	}

	for _, tc := range transitions {
		_, err := svc.ChangeStatus(context.Background(), ChangeCaseStatusParams{
			OrganizationID: c.OrganizationID,
			CaseID:         c.ID,
			Status:         tc.to,
			ActorID:        userID,
		})
		require.NoError(t, err, "transition %s→%s should succeed", tc.from, tc.to)

		updated, err := svc.GetCase(context.Background(), orgID, c.ID)
		require.NoError(t, err)
		assert.Equal(t, tc.to, updated.Status)
	}
}

func TestInvalidStateTransition(t *testing.T) {
	repo := newMockCaseRepo()
	svc := NewCaseService(repo, nil)

	orgID := uuid.New()
	userID := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Test Case",
		CreatedByID:    userID,
	})
	require.NoError(t, err)

	_, err = svc.ChangeStatus(context.Background(), ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         domain.CaseStatusClosed,
		ActorID:        userID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseTransition)
}

func TestReopenFromReview(t *testing.T) {
	repo := newMockCaseRepo()
	svc := NewCaseService(repo, nil)

	orgID := uuid.New()
	userID := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Test Case",
		CreatedByID:    userID,
	})
	require.NoError(t, err)

	_, err = svc.ChangeStatus(context.Background(), ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         domain.CaseStatusOpen,
		ActorID:        userID,
	})
	require.NoError(t, err)

	_, err = svc.ChangeStatus(context.Background(), ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         domain.CaseStatusInReview,
		ActorID:        userID,
	})
	require.NoError(t, err)

	_, err = svc.ChangeStatus(context.Background(), ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         domain.CaseStatusOpen,
		ActorID:        userID,
	})
	require.NoError(t, err, "reopening from IN_REVIEW to OPEN should be valid")
}

func TestCaseTenantIsolation(t *testing.T) {
	repo := newMockCaseRepo()
	svc := NewCaseService(repo, nil)

	org1 := uuid.New()
	org2 := uuid.New()
	user := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: org1,
		Title:          "Case in Org 1",
		CreatedByID:    user,
	})
	require.NoError(t, err)

	_, err = svc.GetCase(context.Background(), org2, c.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseNotFound)
}

func TestAssignCase(t *testing.T) {
	repo := newMockCaseRepo()
	svc := NewCaseService(repo, nil)

	orgID := uuid.New()
	creator := uuid.New()
	assignee := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Test Case",
		CreatedByID:    creator,
	})
	require.NoError(t, err)

	updated, err := svc.AssignCase(context.Background(), AssignCaseParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		UserID:         assignee,
		ActorID:        creator,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.AssignedToID)
	assert.Equal(t, assignee, *updated.AssignedToID)
}

func TestListCases(t *testing.T) {
	repo := newMockCaseRepo()
	svc := NewCaseService(repo, nil)

	orgID := uuid.New()
	userID := uuid.New()

	for i := 0; i < 5; i++ {
		_, err := svc.CreateCase(context.Background(), CreateCaseParams{
			OrganizationID: orgID,
			Title:          "Case " + string(rune('A'+i)),
			CreatedByID:    userID,
		})
		require.NoError(t, err)
	}

	cases, err := svc.ListCases(context.Background(), orgID, 20, 0)
	require.NoError(t, err)
	assert.Len(t, cases, 5)
}
