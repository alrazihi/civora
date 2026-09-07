package integration

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	caseapp "github.com/alrazihi/civora/internal/cases/application"
	caseDomain "github.com/alrazihi/civora/internal/cases/domain"
	casepostgres "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	orgapp "github.com/alrazihi/civora/internal/organizations/application"
	orgpostgres "github.com/alrazihi/civora/internal/organizations/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAppServices(t *testing.T) (
	*orgapp.OrganizationService,
	*caseapp.CaseService,
	*sql.DB,
) {
	t.Helper()
	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	orgRepo := orgpostgres.NewPostgresOrganizationRepository(db)
	caseRepo := casepostgres.NewPostgresCaseRepository(db)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db)

	auditService := auditapp.NewAuditService(auditRepo)

	return orgapp.NewOrganizationService(orgRepo, auditService),
		caseapp.NewCaseService(caseRepo, auditService),
		db
}

func TestOrganizationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	orgSvc, _, _ := setupAppServices(t)
	ctx := context.Background()

	org, err := orgSvc.CreateOrganization(ctx, orgapp.CreateOrganizationParams{
		Name:        "Integration Org",
		Description: "Test organization for integration",
		Slug:        "integration-org-" + uuid.NewString()[:8],
	})
	require.NoError(t, err)
	require.NotEmpty(t, org.ID)
	assert.Equal(t, "Integration Org", org.Name)
	assert.True(t, strings.HasPrefix(org.Slug, "integration-org-"))

	fetched, err := orgSvc.GetOrganization(ctx, org.ID)
	require.NoError(t, err)
	assert.Equal(t, org.ID, fetched.ID)
	assert.Equal(t, org.Name, fetched.Name)
	assert.Equal(t, org.Slug, fetched.Slug)
}

func TestOrganizationDuplicateSlug(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	orgSvc, _, _ := setupAppServices(t)
	ctx := context.Background()

	slug := "dup-slug-" + uuid.NewString()[:8]
	_, err := orgSvc.CreateOrganization(ctx, orgapp.CreateOrganizationParams{
		Name:        "Org One",
		Description: "",
		Slug:        slug,
	})
	require.NoError(t, err)

	_, err = orgSvc.CreateOrganization(ctx, orgapp.CreateOrganizationParams{
		Name:        "Org Two",
		Description: "",
		Slug:        slug,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, orgapp.ErrOrgSlugTaken)
}

func TestCaseLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	orgSvc, caseSvc, db := setupAppServices(t)
	ctx := context.Background()

	org, err := orgSvc.CreateOrganization(ctx, orgapp.CreateOrganizationParams{
		Name:        "Case Org",
		Description: "",
		Slug:        "case-org-" + uuid.NewString()[:8],
	})
	require.NoError(t, err)

	actorID := helpers.SeedUser(db, org.ID)
	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: org.ID,
		Title:          "Emergency Food Request",
		Description:    "Family needs food assistance",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusCreated, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: org.ID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusOpen,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusOpen, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: org.ID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusInReview,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusInReview, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: org.ID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusResolved,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusResolved, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: org.ID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusClosed,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusClosed, c.Status)
	require.NotNil(t, c.ClosedAt)
}

func TestCaseInvalidTransition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	orgSvc, caseSvc, db := setupAppServices(t)
	ctx := context.Background()

	org, err := orgSvc.CreateOrganization(ctx, orgapp.CreateOrganizationParams{
		Name:        "Transition Org",
		Description: "",
		Slug:        "transition-org-" + uuid.NewString()[:8],
	})
	require.NoError(t, err)

	actorID := helpers.SeedUser(db, org.ID)
	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: org.ID,
		Title:          "Test Case",
		Description:    "Description",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	_, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: org.ID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusClosed,
		ActorID:        actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, caseapp.ErrCaseTransition)
}

func TestCaseTenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	orgSvc, caseSvc, db := setupAppServices(t)
	ctx := context.Background()

	org1, err := orgSvc.CreateOrganization(ctx, orgapp.CreateOrganizationParams{
		Name:        "Org One",
		Description: "",
		Slug:        "iso-org-1-" + uuid.NewString()[:8],
	})
	require.NoError(t, err)

	org2, err := orgSvc.CreateOrganization(ctx, orgapp.CreateOrganizationParams{
		Name:        "Org Two",
		Description: "",
		Slug:        "iso-org-2-" + uuid.NewString()[:8],
	})
	require.NoError(t, err)

	actorID := helpers.SeedUser(db, org1.ID)
	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: org1.ID,
		Title:          "Case in Org 1",
		Description:    "Description",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	_, err = caseSvc.GetCase(ctx, org2.ID, c.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, caseapp.ErrCaseNotFound)
}

func TestCaseGeneratesAuditEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	orgSvc, caseSvc, db := setupAppServices(t)
	ctx := context.Background()

	org, err := orgSvc.CreateOrganization(ctx, orgapp.CreateOrganizationParams{
		Name:        "Audit Org",
		Description: "",
		Slug:        "audit-org-" + uuid.NewString()[:8],
	})
	require.NoError(t, err)

	actorID := helpers.SeedUser(db, org.ID)

	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: org.ID,
		Title:          "Test Case for Audit",
		Description:    "Description",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	_, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: org.ID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusOpen,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE organization_id = $1 AND action IN ('case.created', 'case.transition')", org.ID).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 2, "should have at least 2 audit events")

	var hash, prevHash *string
	err = db.QueryRowContext(ctx, "SELECT hash, previous_hash FROM audit_events WHERE organization_id = $1 ORDER BY timestamp DESC, id DESC LIMIT 1", org.ID).Scan(&hash, &prevHash)
	require.NoError(t, err)
	require.NotNil(t, hash, "audit event should have a hash")

	var firstHash *string
	err = db.QueryRowContext(ctx, "SELECT hash FROM audit_events WHERE organization_id = $1 ORDER BY timestamp ASC, id ASC LIMIT 1", org.ID).Scan(&firstHash)
	require.NoError(t, err)
	require.NotNil(t, firstHash, "first audit event should have a hash")
}
