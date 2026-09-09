package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	caseapp "github.com/alrazihi/civora/internal/cases/application"
	caseDomain "github.com/alrazihi/civora/internal/cases/domain"
	casepostgres "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	identityDomain "github.com/alrazihi/civora/internal/identity/domain"
	identitypostgres "github.com/alrazihi/civora/internal/identity/infrastructure/postgres"
	orgapp "github.com/alrazihi/civora/internal/organizations/application"
	orgpostgres "github.com/alrazihi/civora/internal/organizations/infrastructure/postgres"
	peoplepostgres "github.com/alrazihi/civora/internal/people/infrastructure/postgres"
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
	userRepo := identitypostgres.NewPostgresUserRepository(db)
	roleRepo := identitypostgres.NewPostgresRoleRepository(db)
	caseRepo := casepostgres.NewPostgresCaseRepository(db)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db)
	personRepo := peoplepostgres.NewPostgresPersonRepository(db)

	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	roleCreator := identityDomain.NewDefaultRoleCreator(roleRepo)
	return orgapp.NewOrganizationService(orgRepo, roleCreator, auditService),
		caseapp.NewCaseService(caseRepo, personRepo, identityDomain.NewOrganizationUserChecker(userRepo), auditService, auditRepo),
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
	assert.True(t, len(org.Slug) > 0)

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
		ServiceType:    caseDomain.ServiceTypeEmergency,
		Priority:       caseDomain.PriorityHigh,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusNew, c.Status)

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
		Status:         caseDomain.CaseStatusAssessment,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusAssessment, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: org.ID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusDecisionPending,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusDecisionPending, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: org.ID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusApproved,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusApproved, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: org.ID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusInProgress,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusInProgress, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: org.ID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusFollowUp,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusFollowUp, c.Status)

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
		ServiceType:    caseDomain.ServiceTypeGeneral,
		Priority:       caseDomain.PriorityNormal,
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
		ServiceType:    caseDomain.ServiceTypeGeneral,
		Priority:       caseDomain.PriorityNormal,
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
		ServiceType:    caseDomain.ServiceTypeGeneral,
		Priority:       caseDomain.PriorityNormal,
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
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit.audit_events WHERE organization_id = $1 AND action IN ('case.created', 'case.transition')", org.ID).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 2, "should have at least 2 audit events")

	var hash, prevHash *string
	err = db.QueryRowContext(ctx, "SELECT hash, previous_hash FROM audit.audit_events WHERE organization_id = $1 ORDER BY timestamp DESC, id DESC LIMIT 1", org.ID).Scan(&hash, &prevHash)
	require.NoError(t, err)
	require.NotNil(t, hash, "audit event should have a hash")

	var firstHash *string
	err = db.QueryRowContext(ctx, "SELECT hash FROM audit.audit_events WHERE organization_id = $1 ORDER BY timestamp ASC, id ASC LIMIT 1", org.ID).Scan(&firstHash)
	require.NoError(t, err)
	require.NotNil(t, firstHash, "first audit event should have a hash")
}

type failingAuditRepo struct {
	auditpostgres.PostgresAuditRepository
}

func (f *failingAuditRepo) RecordEventTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, event *auditdomain.AuditEvent) error {
	return errors.New("simulated audit write failure")
}

func (f *failingAuditRepo) RecordEvent(ctx context.Context, orgID uuid.UUID, event *auditdomain.AuditEvent) error {
	return errors.New("simulated audit write failure")
}

func TestAuditAtomicity_CaseCreationRollsBackOnAuditFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	userRepo := identitypostgres.NewPostgresUserRepository(db)
	caseRepo := casepostgres.NewPostgresCaseRepository(db)
	personRepo := peoplepostgres.NewPostgresPersonRepository(db)

	auditRepo := &failingAuditRepo{PostgresAuditRepository: *auditpostgres.NewPostgresAuditRepository(db)}
	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	caseSvc := caseapp.NewCaseService(caseRepo, personRepo, identityDomain.NewOrganizationUserChecker(userRepo), auditService, auditRepo)

	ctx := context.Background()
	orgID := helpers.SeedOrg(db)
	helpers.SeedDefaultRoles(db, orgID)

	actorID := helpers.SeedUser(db, orgID)

	_, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Test Case for Atomicity",
		Description:    "Should not persist if audit fails",
		ServiceType:    caseDomain.ServiceTypeGeneral,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.Error(t, err, "CreateCase should fail when audit recording fails")

	var caseCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cases WHERE organization_id = $1", orgID).Scan(&caseCount)
	require.NoError(t, err)
	assert.Equal(t, 0, caseCount, "case should be rolled back when audit fails")

	var auditCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit.audit_events WHERE organization_id = $1 AND resource = $2", orgID, "case").Scan(&auditCount)
	require.NoError(t, err)
	assert.Equal(t, 0, auditCount, "no audit events should exist for the failed case creation")
}

func TestAuditAtomicity_StatusChangeRollsBackOnAuditFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	userRepo := identitypostgres.NewPostgresUserRepository(db)
	caseRepo := casepostgres.NewPostgresCaseRepository(db)
	personRepo := peoplepostgres.NewPostgresPersonRepository(db)

	auditRepo := &failingAuditRepo{PostgresAuditRepository: *auditpostgres.NewPostgresAuditRepository(db)}
	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	caseSvc := caseapp.NewCaseService(caseRepo, personRepo, identityDomain.NewOrganizationUserChecker(userRepo), auditService, auditRepo)

	ctx := context.Background()
	orgID := helpers.SeedOrg(db)
	helpers.SeedDefaultRoles(db, orgID)

	actorID := helpers.SeedUser(db, orgID)

	caseID := uuid.New()
	_, err := db.ExecContext(ctx,
		"INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, person_id, created_by, assigned_to, created_at, updated_at, closed_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW(), NULL)",
		caseID, orgID, "CASE-001", "Pre-existing Case", "", "NEW", "GENERAL", "NORMAL", nil, actorID, nil,
	)
	require.NoError(t, err)

	_, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		Status:         caseDomain.CaseStatusOpen,
		ActorID:        actorID,
	})
	require.Error(t, err, "ChangeStatus should fail when audit recording fails")

	var statusVal string
	err = db.QueryRowContext(ctx, "SELECT status FROM cases WHERE id = $1", caseID).Scan(&statusVal)
	require.NoError(t, err)
	assert.Equal(t, "NEW", statusVal, "case status should remain unchanged when audit fails")
}

func TestAuditAtomicity_OrgCreationRollsBackOnAuditFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	orgRepo := orgpostgres.NewPostgresOrganizationRepository(db)
	roleRepo := identitypostgres.NewPostgresRoleRepository(db)

	auditRepo := &failingAuditRepo{PostgresAuditRepository: *auditpostgres.NewPostgresAuditRepository(db)}
	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	roleCreator := identityDomain.NewDefaultRoleCreator(roleRepo)
	orgSvc := orgapp.NewOrganizationService(orgRepo, roleCreator, auditService)

	ctx := context.Background()
	slug := "atomic-org-" + uuid.NewString()[:8]
	_, err := orgSvc.CreateOrganization(ctx, orgapp.CreateOrganizationParams{
		Name:        "Atomic Org Creation",
		Description: "",
		Slug:        slug,
	})
	require.Error(t, err, "CreateOrganization should fail when audit recording fails")

	var orgCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM organizations WHERE slug = $1", slug).Scan(&orgCount)
	require.NoError(t, err)
	assert.Equal(t, 0, orgCount, "organization should be rolled back when audit fails")

	var roleCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM roles WHERE organization_id IN (SELECT id FROM organizations WHERE slug = $1)", slug).Scan(&roleCount)
	require.NoError(t, err)
	assert.Equal(t, 0, roleCount, "default roles should be rolled back when audit fails")
}
