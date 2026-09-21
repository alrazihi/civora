package integration

import (
	"context"
	"database/sql"
	"testing"

	assistanceapp "github.com/alrazihi/civora/internal/assistance/application"
	assistancedomain "github.com/alrazihi/civora/internal/assistance/domain"
	assistancepostgres "github.com/alrazihi/civora/internal/assistance/infrastructure/postgres"
	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	caseapp "github.com/alrazihi/civora/internal/cases/application"
	casepostgres "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	decisionsapp "github.com/alrazihi/civora/internal/decisions/application"
	decisionspostgres "github.com/alrazihi/civora/internal/decisions/infrastructure/postgres"
	identityDomain "github.com/alrazihi/civora/internal/identity/domain"
	identitypostgres "github.com/alrazihi/civora/internal/identity/infrastructure/postgres"
	orgapp "github.com/alrazihi/civora/internal/organizations/application"
	orgpostgres "github.com/alrazihi/civora/internal/organizations/infrastructure/postgres"
	peoplepostgres "github.com/alrazihi/civora/internal/people/infrastructure/postgres"
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowpostgres "github.com/alrazihi/civora/internal/workflow/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tenantIsolationServices struct {
	caseSvc       *caseapp.CaseService
	decisionSvc   *decisionsapp.DecisionService
	assistanceSvc *assistanceapp.AssistanceService
}

func newTenantIsolationServices(db *sql.DB) *tenantIsolationServices {
	orgRepo := orgpostgres.NewPostgresOrganizationRepository(db)
	userRepo := identitypostgres.NewPostgresUserRepository(db)
	roleRepo := identitypostgres.NewPostgresRoleRepository(db)
	caseRepo := casepostgres.NewPostgresCaseRepository(db)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db)
	decisionRepo := decisionspostgres.NewPostgresDecisionRepository(db)
	assistanceRepo := assistancepostgres.NewPostgresAssistanceRepository(db)

	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})
	roleCreator := identityDomain.NewDefaultRoleCreator(roleRepo)
	_ = orgapp.NewOrganizationService(orgRepo, roleCreator, auditService)

	defRepo := workflowpostgres.NewPostgresWorkflowDefinitionRepository(db)
	stateRepo := workflowpostgres.NewPostgresWorkflowStateRepository(db)
	transitionRepo := workflowpostgres.NewPostgresWorkflowTransitionRepository(db)
	instanceRepo := workflowpostgres.NewPostgresWorkflowInstanceRepository(db)
	historyRepo := workflowpostgres.NewPostgresWorkflowTransitionHistoryRepository(db)
	workflowSvc := workflowapp.NewWorkflowService(defRepo, stateRepo, transitionRepo, instanceRepo, historyRepo, auditService)

	caseSvc := caseapp.NewCaseService(caseRepo, peoplepostgres.NewPostgresPersonRepository(db), identityDomain.NewOrganizationUserChecker(userRepo), auditService, auditRepo, workflowSvc)
	decisionSvc := decisionsapp.NewDecisionService(decisionRepo, caseRepo, identityDomain.NewOrganizationUserChecker(userRepo), auditService, workflowSvc)
	assistanceSvc := assistanceapp.NewAssistanceService(assistanceRepo, caseRepo, identityDomain.NewOrganizationUserChecker(userRepo), auditService)

	return &tenantIsolationServices{
		caseSvc:       caseSvc,
		decisionSvc:   decisionSvc,
		assistanceSvc: assistanceSvc,
	}
}

func TestDecision_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)
	ctx := context.Background()

	svc := newTenantIsolationServices(db)

	org1, err := setupConcurrencyOrg(ctx, db, t)
	require.NoError(t, err)
	org2, err := setupConcurrencyOrg(ctx, db, t)
	require.NoError(t, err)

	actor1 := helpers.SeedUser(db, org1.ID)
	_ = helpers.SeedUser(db, org2.ID)

	caseID := uuid.New()
	_, err = db.ExecContext(ctx,
		`INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, created_by, assigned_to, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())`,
		caseID, org1.ID, "DEC-TI-001", "Decision Tenant Case", "", "NEW", "GENERAL", "NORMAL", actor1, nil,
	)
	require.NoError(t, err)

	decID := uuid.New()
	_, err = db.ExecContext(ctx,
		`INSERT INTO decisions (id, organization_id, service_request_id, decision, reason, decision_maker, decided_at, created_at) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
		decID, org1.ID, caseID, "APPROVED", "Approved for testing", actor1,
	)
	require.NoError(t, err)

	_, err = svc.decisionSvc.GetDecision(ctx, org2.ID, decID)
	require.Error(t, err)
	assert.ErrorIs(t, err, decisionsapp.ErrDecisionNotFound)
}

func TestAssistance_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)
	ctx := context.Background()

	svc := newTenantIsolationServices(db)

	org1, err := setupConcurrencyOrg(ctx, db, t)
	require.NoError(t, err)
	org2, err := setupConcurrencyOrg(ctx, db, t)
	require.NoError(t, err)

	actor1 := helpers.SeedUser(db, org1.ID)
	_ = helpers.SeedUser(db, org2.ID)

	_, concurrentDefID, _ := seedWorkflowDefinitionForConcurrency(ctx, db, t, org1.ID, uuid.Nil)

	c1, err := svc.caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: org1.ID,
		Title:          "Org1 Assistance Case",
		Description:    "Test",
		ServiceType:    "General",
		Priority:       "Normal",
		CreatedByID:    actor1,
		WorkflowID:     &concurrentDefID,
	})
	require.NoError(t, err)

	a, err := svc.assistanceSvc.CreateAssistance(ctx, assistanceapp.CreateAssistanceParams{
		OrganizationID:   org1.ID,
		ServiceRequestID: c1.ID,
		Type:             assistancedomain.AssistanceTypeShelter,
		Description:      "Emergency shelter",
		ResponsibleStaff: actor1,
		ActorID:          actor1,
	})
	require.NoError(t, err)

	_, err = svc.assistanceSvc.GetAssistance(ctx, org2.ID, a.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, assistanceapp.ErrAssistanceNotFound)
}
