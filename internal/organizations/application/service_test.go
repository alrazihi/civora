package application

import (
	"context"
	"database/sql"
	"testing"

	"github.com/alrazihi/civora/internal/organizations/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOrgRepo struct {
	orgs map[uuid.UUID]*domain.Organization
}

func newMockOrgRepo() *mockOrgRepo {
	return &mockOrgRepo{orgs: make(map[uuid.UUID]*domain.Organization)}
}

func (m *mockOrgRepo) Save(ctx context.Context, org *domain.Organization) error {
	return m.SaveTx(ctx, nil, org)
}

func (m *mockOrgRepo) SaveTx(ctx context.Context, tx *sql.Tx, org *domain.Organization) error {
	m.orgs[org.ID] = org
	return nil
}

func (m *mockOrgRepo) DB() *sql.DB {
	return nil
}

func (m *mockOrgRepo) FindBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	for _, org := range m.orgs {
		if org.Slug == slug {
			return org, nil
		}
	}
	return nil, nil
}

func (m *mockOrgRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	org, ok := m.orgs[id]
	if !ok {
		return nil, nil
	}
	return org, nil
}

type mockRoleCreator struct {
	createdRoles []string
	called       bool
}

func (m *mockRoleCreator) CreateDefaultRoles(ctx context.Context, orgID uuid.UUID) error {
	return m.CreateDefaultRolesTx(ctx, nil, orgID)
}

func (m *mockRoleCreator) CreateDefaultRolesTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) error {
	m.called = true
	m.createdRoles = append(m.createdRoles, "admin", "staff")
	return nil
}

func TestCreateOrganization_CreatesDefaultRoles(t *testing.T) {
	repo := newMockOrgRepo()
	roleCreator := &mockRoleCreator{}
	svc := NewOrganizationService(repo, roleCreator, nil)

	org, err := svc.CreateOrganization(context.Background(), CreateOrganizationParams{
		Name:        "Test Org",
		Description: "A test organization",
		Slug:        "test-org",
	})
	require.NoError(t, err)
	require.NotNil(t, org)
	assert.True(t, roleCreator.called, "default roles should have been created")
	assert.Len(t, roleCreator.createdRoles, 2)
}

func TestCreateOrganization_DuplicateSlug(t *testing.T) {
	repo := newMockOrgRepo()
	svc := NewOrganizationService(repo, &mockRoleCreator{}, nil)

	_, err := svc.CreateOrganization(context.Background(), CreateOrganizationParams{
		Name: "First Org",
		Slug: "same-slug",
	})
	require.NoError(t, err)

	_, err = svc.CreateOrganization(context.Background(), CreateOrganizationParams{
		Name: "Second Org",
		Slug: "same-slug",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOrgSlugTaken)
}

func TestCreateOrganization_InvalidInput(t *testing.T) {
	repo := newMockOrgRepo()
	svc := NewOrganizationService(repo, &mockRoleCreator{}, nil)

	_, err := svc.CreateOrganization(context.Background(), CreateOrganizationParams{
		Name: "",
		Slug: "",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOrgInvalidInput)
}
