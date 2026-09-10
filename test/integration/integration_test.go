package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

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
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	workflowpostgres "github.com/alrazihi/civora/internal/workflow/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedWorkflowDefinition(t *testing.T, db *sql.DB) (*workflowapp.WorkflowService, uuid.UUID) {
	t.Helper()
	orgID := helpers.SeedOrg(db)
	defRepo := workflowpostgres.NewPostgresWorkflowDefinitionRepository(db)
	stateRepo := workflowpostgres.NewPostgresWorkflowStateRepository(db)
	transitionRepo := workflowpostgres.NewPostgresWorkflowTransitionRepository(db)
	instanceRepo := workflowpostgres.NewPostgresWorkflowInstanceRepository(db)
	historyRepo := workflowpostgres.NewPostgresWorkflowTransitionHistoryRepository(db)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db)
	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	workflowSvc := workflowapp.NewWorkflowService(defRepo, stateRepo, transitionRepo, instanceRepo, historyRepo, auditService)

	now := time.Now().UTC()
	states := []workflowdomain.WorkflowState{
		{ID: uuid.New(), TenantID: orgID, Key: "NEW", Name: "New Request", Terminal: false, DisplayOrder: 0, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "OPEN", Name: "Open", Terminal: false, DisplayOrder: 1, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "IN_REVIEW", Name: "In Review", Terminal: false, DisplayOrder: 2, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "ASSESSMENT", Name: "Assessment", Terminal: false, DisplayOrder: 3, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "DECISION_PENDING", Name: "Decision Pending", Terminal: false, DisplayOrder: 4, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "APPROVED", Name: "Approved", Terminal: false, DisplayOrder: 5, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "REJECTED", Name: "Rejected", Terminal: false, DisplayOrder: 6, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "IN_PROGRESS", Name: "In Progress", Terminal: false, DisplayOrder: 7, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "FOLLOW_UP", Name: "Follow-up", Terminal: false, DisplayOrder: 8, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "CLOSED", Name: "Closed", Terminal: true, DisplayOrder: 9, CreatedAt: now},
	}

	transitions := []workflowdomain.WorkflowTransition{
		{ID: uuid.New(), TenantID: orgID, Key: "open", Name: "Open", FromState: "NEW", ToState: "OPEN", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "review", Name: "Review", FromState: "NEW", ToState: "IN_REVIEW", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "reopen", Name: "Reopen", FromState: "IN_REVIEW", ToState: "OPEN", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "assess", Name: "Assess", FromState: "OPEN", ToState: "IN_REVIEW", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "assess2", Name: "Assess", FromState: "IN_REVIEW", ToState: "ASSESSMENT", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "decide", Name: "Decide", FromState: "ASSESSMENT", ToState: "DECISION_PENDING", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "approve", Name: "Approve", FromState: "DECISION_PENDING", ToState: "APPROVED", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "reject", Name: "Reject", FromState: "DECISION_PENDING", ToState: "REJECTED", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "start_assistance", Name: "Start Assistance", FromState: "APPROVED", ToState: "IN_PROGRESS", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "close_rejected", Name: "Close Rejected", FromState: "REJECTED", ToState: "CLOSED", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "follow_up", Name: "Follow Up", FromState: "IN_PROGRESS", ToState: "FOLLOW_UP", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "complete", Name: "Complete", FromState: "FOLLOW_UP", ToState: "CLOSED", Active: true, CreatedAt: now},
	}

	def, err := workflowSvc.CreateWorkflowDefinition(context.Background(), workflowapp.CreateWorkflowDefinitionParams{
		TenantID:     orgID,
		ActorID:      uuid.Nil,
		Key:          "emergency_assistance",
		Name:         "Emergency Assistance",
		Description:  "Emergency assistance request workflow",
		Version:      1,
		InitialState: "NEW",
		States:       states,
		Transitions:  transitions,
		Metadata:     map[string]interface{}{},
	})
	require.NoError(t, err)

	err = workflowSvc.ActivateWorkflowDefinition(context.Background(), orgID, def.ID, uuid.Nil)
	require.NoError(t, err)

	// The generic workflow definitions are shared across service types. The
	// general_assistance workflow is the default for cases whose service type
	// does not map to a specialized workflow (e.g. GENERAL), and the
	// medical_assistance workflow is the default for cases whose service type
	// is MEDICAL. Both use the same state machine so the case lifecycle is
	// consistent.
	for _, key := range []string{"general_assistance", "medical_assistance"} {
		statesCopy := make([]workflowdomain.WorkflowState, len(states))
		copy(statesCopy, states)
		for i := range statesCopy {
			statesCopy[i].ID = uuid.New()
		}
		transitionsCopy := make([]workflowdomain.WorkflowTransition, len(transitions))
		copy(transitionsCopy, transitions)
		for i := range transitionsCopy {
			transitionsCopy[i].ID = uuid.New()
		}
		gDef, err := workflowSvc.CreateWorkflowDefinition(context.Background(), workflowapp.CreateWorkflowDefinitionParams{
			TenantID:     orgID,
			ActorID:      uuid.Nil,
			Key:          key,
			Name:         "General Assistance",
			Description:  "General assistance request workflow",
			Version:      1,
			InitialState: "NEW",
			States:       statesCopy,
			Transitions:  transitionsCopy,
			Metadata:     map[string]interface{}{},
		})
		require.NoError(t, err)

		err = workflowSvc.ActivateWorkflowDefinition(context.Background(), orgID, gDef.ID, uuid.Nil)
		require.NoError(t, err)
	}

	return workflowSvc, orgID
}

func setupAppServices(t *testing.T) (
	*orgapp.OrganizationService,
	*caseapp.CaseService,
	*sql.DB,
	uuid.UUID,
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
	orgSvc := orgapp.NewOrganizationService(orgRepo, roleCreator, auditService)

	workflowSvc, workflowOrgID := seedWorkflowDefinition(t, db)

	return orgSvc,
		caseapp.NewCaseService(caseRepo, personRepo, identityDomain.NewOrganizationUserChecker(userRepo), auditService, auditRepo, workflowSvc),
		db,
		workflowOrgID
}

func TestOrganizationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	orgSvc, _, _, _ := setupAppServices(t)
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

	orgSvc, _, _, _ := setupAppServices(t)
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

	_, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()
	helpers.SeedDefaultRoles(db, workflowOrgID)

	actorID := helpers.SeedUser(db, workflowOrgID)
	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Emergency Food Request",
		Description:    "Family needs food assistance",
		ServiceType:    caseDomain.ServiceTypeEmergency,
		Priority:       caseDomain.PriorityHigh,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusNew, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: workflowOrgID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusOpen,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusOpen, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: workflowOrgID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusInReview,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusInReview, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: workflowOrgID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusAssessment,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusAssessment, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: workflowOrgID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusDecisionPending,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusDecisionPending, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: workflowOrgID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusApproved,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusApproved, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: workflowOrgID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusInProgress,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusInProgress, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: workflowOrgID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusFollowUp,
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusFollowUp, c.Status)

	c, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: workflowOrgID,
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

	_, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()
	helpers.SeedDefaultRoles(db, workflowOrgID)

	actorID := helpers.SeedUser(db, workflowOrgID)
	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Test Case",
		Description:    "Description",
		ServiceType:    caseDomain.ServiceTypeGeneral,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	_, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: workflowOrgID,
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

	orgSvc, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()

	org2, err := orgSvc.CreateOrganization(ctx, orgapp.CreateOrganizationParams{
		Name:        "Org Two",
		Description: "",
		Slug:        "iso-org-2-" + uuid.NewString()[:8],
	})
	require.NoError(t, err)

	actorID := helpers.SeedUser(db, workflowOrgID)
	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Case in workflow org",
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

	_, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()
	helpers.SeedDefaultRoles(db, workflowOrgID)

	actorID := helpers.SeedUser(db, workflowOrgID)

	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Test Case for Audit",
		Description:    "Description",
		ServiceType:    caseDomain.ServiceTypeGeneral,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	_, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: workflowOrgID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusOpen,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit.audit_events WHERE organization_id = $1 AND action IN ('case.created', 'case.transition', 'workflow.transition')", workflowOrgID).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 2, "should have at least 2 audit events")

	var hash, prevHash *string
	err = db.QueryRowContext(ctx, "SELECT hash, previous_hash FROM audit.audit_events WHERE organization_id = $1 ORDER BY timestamp DESC, id DESC LIMIT 1", workflowOrgID).Scan(&hash, &prevHash)
	require.NoError(t, err)
	require.NotNil(t, hash, "audit event should have a hash")

	var firstHash *string
	err = db.QueryRowContext(ctx, "SELECT hash FROM audit.audit_events WHERE organization_id = $1 ORDER BY timestamp ASC, id ASC LIMIT 1", workflowOrgID).Scan(&firstHash)
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
