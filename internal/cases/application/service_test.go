package application

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/cases/domain"
	peopleDomain "github.com/alrazihi/civora/internal/people/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPersonFinder struct {
	persons map[uuid.UUID]*peopleDomain.Person
}

func newMockPersonFinder() *mockPersonFinder {
	return &mockPersonFinder{persons: make(map[uuid.UUID]*peopleDomain.Person)}
}

func (m *mockPersonFinder) FindByID(ctx context.Context, orgID, id uuid.UUID) (*peopleDomain.Person, error) {
	p, ok := m.persons[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return p, nil
}

func (m *mockPersonFinder) addPerson(p *peopleDomain.Person) {
	m.persons[p.ID] = p
}

type mockCaseRepository struct {
	mu    sync.RWMutex
	cases map[uuid.UUID]*domain.Case
}

func newMockCaseRepo() *mockCaseRepository {
	return &mockCaseRepository{
		cases: make(map[uuid.UUID]*domain.Case),
	}
}

func (m *mockCaseRepository) DB() *sql.DB {
	return nil
}

func (m *mockCaseRepository) Save(ctx context.Context, c *domain.Case) error {
	return m.SaveTx(ctx, nil, c)
}

func (m *mockCaseRepository) SaveTx(ctx context.Context, tx *sql.Tx, c *domain.Case) error {
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
	return m.FindByOrganizationWithFilter(ctx, orgID, limit, offset, domain.CaseFilter{})
}

func (m *mockCaseRepository) FindByOrganizationWithFilter(ctx context.Context, orgID uuid.UUID, limit, offset int, filter domain.CaseFilter) ([]*domain.Case, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Case
	for _, c := range m.cases {
		if c.OrganizationID == orgID {
			if filter.Status != "" && c.Status != filter.Status {
				continue
			}
			if filter.PersonID != nil && c.PersonID != filter.PersonID {
				continue
			}
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockCaseRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID, filter domain.CaseFilter) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, c := range m.cases {
		if c.OrganizationID == orgID {
			if filter.Status != "" && c.Status != filter.Status {
				continue
			}
			count++
		}
	}
	return count, nil
}

func (m *mockCaseRepository) UpdateStatus(ctx context.Context, orgID, id uuid.UUID, status domain.CaseStatus, version int) error {
	return m.UpdateStatusTx(ctx, nil, orgID, id, status, version)
}

func (m *mockCaseRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, status domain.CaseStatus, version int) error {
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
	return m.AssignTx(ctx, nil, orgID, id, userID)
}

func (m *mockCaseRepository) AssignTx(ctx context.Context, tx *sql.Tx, orgID, id, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.cases[id]
	if !ok || c.OrganizationID != orgID {
		return errors.New("not found")
	}
	c.AssignedToID = &userID
	return nil
}

func (m *mockCaseRepository) UpdateWorkflowInstanceID(ctx context.Context, orgID, caseID, instanceID uuid.UUID) error {
	return m.UpdateWorkflowInstanceIDTx(ctx, nil, orgID, caseID, instanceID)
}

func (m *mockCaseRepository) UpdateWorkflowInstanceIDTx(ctx context.Context, tx *sql.Tx, orgID, caseID, instanceID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.cases[caseID]
	if !ok || c.OrganizationID != orgID {
		return errors.New("not found")
	}
	c.WorkflowInstanceID = &instanceID
	return nil
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

func TestCreateCase(t *testing.T) {
	repo := newMockCaseRepo()
	svc := NewCaseService(repo, newMockPersonFinder(), newMockUserChecker(), nil, nil)

	orgID := uuid.New()
	userID := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Emergency Food Request",
		Description:    "Family needs emergency food assistance",
		ServiceType:    domain.ServiceTypeEmergency,
		Priority:       domain.PriorityHigh,
		CreatedByID:    userID,
	})

	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, orgID, c.OrganizationID)
	assert.Equal(t, "Emergency Food Request", c.Title)
	assert.Equal(t, domain.CaseStatusNew, c.Status)
	assert.NotEmpty(t, c.CaseNumber)
	assert.Equal(t, userID, c.CreatedByID)
	assert.Equal(t, domain.ServiceTypeEmergency, c.ServiceType)
	assert.Equal(t, domain.PriorityHigh, c.Priority)
}

func TestCaseLifecycle(t *testing.T) {
	repo := newMockCaseRepo()
	svc := NewCaseService(repo, newMockPersonFinder(), newMockUserChecker(), nil, nil)

	orgID := uuid.New()
	userID := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Emergency Assistance",
		Description:    "Need help",
		ServiceType:    domain.ServiceTypeGeneral,
		Priority:       domain.PriorityHigh,
		CreatedByID:    userID,
	})
	require.NoError(t, err)

	transitions := []struct {
		from domain.CaseStatus
		to   domain.CaseStatus
	}{
		{domain.CaseStatusNew, domain.CaseStatusOpen},
		{domain.CaseStatusOpen, domain.CaseStatusInReview},
		{domain.CaseStatusInReview, domain.CaseStatusAssessment},
		{domain.CaseStatusAssessment, domain.CaseStatusDecisionPending},
		{domain.CaseStatusDecisionPending, domain.CaseStatusApproved},
		{domain.CaseStatusApproved, domain.CaseStatusInProgress},
		{domain.CaseStatusInProgress, domain.CaseStatusFollowUp},
		{domain.CaseStatusFollowUp, domain.CaseStatusClosed},
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
	svc := NewCaseService(repo, newMockPersonFinder(), newMockUserChecker(), nil, nil)

	orgID := uuid.New()
	userID := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Test Case",
		ServiceType:    domain.ServiceTypeGeneral,
		Priority:       domain.PriorityNormal,
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
	svc := NewCaseService(repo, newMockPersonFinder(), newMockUserChecker(), nil, nil)

	orgID := uuid.New()
	userID := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Test Case",
		ServiceType:    domain.ServiceTypeGeneral,
		Priority:       domain.PriorityNormal,
		CreatedByID:    userID,
	})
	require.NoError(t, err)

	_, err = svc.ChangeStatus(context.Background(), ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         domain.CaseStatusOpen,
	})
	require.NoError(t, err)

	_, err = svc.ChangeStatus(context.Background(), ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         domain.CaseStatusInReview,
	})
	require.NoError(t, err)

	_, err = svc.ChangeStatus(context.Background(), ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         domain.CaseStatusOpen,
	})
	require.NoError(t, err, "reopening from IN_REVIEW to OPEN should be valid")
}

func TestCasePersonTenantIsolation(t *testing.T) {
	repo := newMockCaseRepo()
	personFinder := newMockPersonFinder()
	svc := NewCaseService(repo, personFinder, newMockUserChecker(), nil, nil)

	org1 := uuid.New()
	org2 := uuid.New()
	userID := uuid.New()

	person, _ := peopleDomain.NewPerson(org1, "John", "Doe", "en")
	personFinder.addPerson(person)

	_, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: org2,
		Title:          "Case with cross-tenant person",
		Description:    "Desc",
		ServiceType:    domain.ServiceTypeGeneral,
		Priority:       domain.PriorityNormal,
		PersonID:       &person.ID,
		CreatedByID:    userID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPersonTenantViolation)
}

func TestCaseTenantIsolation(t *testing.T) {
	repo := newMockCaseRepo()
	svc := NewCaseService(repo, newMockPersonFinder(), newMockUserChecker(), nil, nil)

	org1 := uuid.New()
	org2 := uuid.New()
	user := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: org1,
		Title:          "Case in Org 1",
		ServiceType:    domain.ServiceTypeGeneral,
		Priority:       domain.PriorityNormal,
		CreatedByID:    user,
	})
	require.NoError(t, err)

	_, err = svc.GetCase(context.Background(), org2, c.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseNotFound)
}

func TestAssignCase(t *testing.T) {
	repo := newMockCaseRepo()
	checker := newMockUserChecker()
	svc := NewCaseService(repo, newMockPersonFinder(), checker, nil, nil)

	orgID := uuid.New()
	creator := uuid.New()
	assignee := uuid.New()
	checker.addMember(orgID, creator)
	checker.addMember(orgID, assignee)

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Test Case",
		ServiceType:    domain.ServiceTypeGeneral,
		Priority:       domain.PriorityNormal,
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

func TestAssignCase_CrossTenantRejected(t *testing.T) {
	repo := newMockCaseRepo()
	checker := newMockUserChecker()
	svc := NewCaseService(repo, newMockPersonFinder(), checker, nil, nil)

	org1 := uuid.New()
	org2 := uuid.New()
	creator := uuid.New()
	assignee := uuid.New()
	checker.addMember(org1, creator)
	checker.addMember(org2, assignee)

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: org1,
		Title:          "Test Case",
		ServiceType:    domain.ServiceTypeGeneral,
		Priority:       domain.PriorityNormal,
		CreatedByID:    creator,
	})
	require.NoError(t, err)

	_, err = svc.AssignCase(context.Background(), AssignCaseParams{
		OrganizationID: org1,
		CaseID:         c.ID,
		UserID:         assignee,
		ActorID:        creator,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseTenantViolation)
}

func TestListCases(t *testing.T) {
	repo := newMockCaseRepo()
	svc := NewCaseService(repo, newMockPersonFinder(), newMockUserChecker(), nil, nil)

	orgID := uuid.New()
	userID := uuid.New()

	for i := 0; i < 5; i++ {
		_, err := svc.CreateCase(context.Background(), CreateCaseParams{
			OrganizationID: orgID,
			Title:          "Case " + string(rune('A'+i)),
			ServiceType:    domain.ServiceTypeGeneral,
			Priority:       domain.PriorityNormal,
			CreatedByID:    userID,
		})
		require.NoError(t, err)
	}

	cases, _, err := svc.ListCases(context.Background(), orgID, 20, 0, domain.CaseFilter{})
	require.NoError(t, err)
	assert.Len(t, cases, 5)
}

type collisionMockCaseRepo struct {
	*mockCaseRepository
	collisions    int
	maxCollisions int
}

func (m *collisionMockCaseRepo) SaveTx(ctx context.Context, tx *sql.Tx, c *domain.Case) error {
	if m.collisions < m.maxCollisions {
		m.collisions++
		return domain.ErrCaseNumberConflict
	}
	return m.mockCaseRepository.SaveTx(ctx, tx, c)
}

func NewCollisionMockCaseRepo(maxCollisions int) *collisionMockCaseRepo {
	return &collisionMockCaseRepo{
		mockCaseRepository: newMockCaseRepo(),
		maxCollisions:      maxCollisions,
	}
}

func TestCaseNumberCollision_RetryOnConflict(t *testing.T) {
	repo := NewCollisionMockCaseRepo(1)
	svc := NewCaseService(repo, newMockPersonFinder(), newMockUserChecker(), nil, nil)

	orgID := uuid.New()
	userID := uuid.New()

	c, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Case with Collision",
		ServiceType:    domain.ServiceTypeGeneral,
		Priority:       domain.PriorityNormal,
		CreatedByID:    userID,
	})
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, orgID, c.OrganizationID)
	assert.Equal(t, 1, repo.collisions, "service should have retried after case number conflict")
}

func TestCaseNumberCollision_ExhaustsRetries(t *testing.T) {
	repo := NewCollisionMockCaseRepo(10)
	svc := NewCaseService(repo, newMockPersonFinder(), newMockUserChecker(), nil, nil)

	orgID := uuid.New()
	userID := uuid.New()

	_, err := svc.CreateCase(context.Background(), CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Case That Keeps Colliding",
		ServiceType:    domain.ServiceTypeGeneral,
		Priority:       domain.PriorityNormal,
		CreatedByID:    userID,
	})
	require.Error(t, err, "should fail after exhausting retries")
}
