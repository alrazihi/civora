package integration

import (
	"context"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/cases/application"
	caseDomain "github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/config"
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	workflowpostgres "github.com/alrazihi/civora/internal/workflow/infrastructure/postgres"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	auditapp "github.com/alrazihi/civora/internal/audit/application"
	assistancepostgres "github.com/alrazihi/civora/internal/assistance/infrastructure/postgres"
	assessmentpostgres "github.com/alrazihi/civora/internal/assessment/infrastructure/postgres"
	decisionspostgres "github.com/alrazihi/civora/internal/decisions/infrastructure/postgres"
	eligibilitypostgres "github.com/alrazihi/civora/internal/eligibility/infrastructure/postgres"
	evidencepostgres "github.com/alrazihi/civora/internal/evidence/infrastructure/postgres"
	followuppostgres "github.com/alrazihi/civora/internal/followup/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegression_NewEmergencyAssistanceCaseStartsInInitialState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping regression test")
	}

	_, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()
	helpers.SeedDefaultRoles(db, workflowOrgID)
	actorID := helpers.SeedUser(db, workflowOrgID)

	c, err := caseSvc.CreateCase(ctx, application.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Regression Test Case",
		Description:    "Test",
		ServiceType:    caseDomain.ServiceTypeEmergency,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	assert.Equal(t, caseDomain.CaseStatusNew, c.Status)

	instance, err := caseSvc.GetWorkflowInstance(ctx, workflowOrgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "NEW", instance.CurrentState)
}

func TestRegression_NewCaseIsNotClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping regression test")
	}

	_, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()
	helpers.SeedDefaultRoles(db, workflowOrgID)
	actorID := helpers.SeedUser(db, workflowOrgID)

	c, err := caseSvc.CreateCase(ctx, application.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Not Closed Test",
		Description:    "Test",
		ServiceType:    caseDomain.ServiceTypeEmergency,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)
	assert.NotEqual(t, caseDomain.CaseStatusClosed, c.Status)
}

func TestRegression_ValidTransitionExposed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping regression test")
	}

	_, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()
	helpers.SeedDefaultRoles(db, workflowOrgID)
	actorID := helpers.SeedUser(db, workflowOrgID)

	c, err := caseSvc.CreateCase(ctx, application.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Transition Test",
		Description:    "Test",
		ServiceType:    caseDomain.ServiceTypeEmergency,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	workflowSvc := caseSvc.GetWorkflowService()
	require.NotNil(t, workflowSvc)

	instance, err := workflowSvc.GetInstanceByCaseID(ctx, workflowOrgID, c.ID)
	require.NoError(t, err)

	transitions, err := workflowSvc.GetValidTransitions(ctx, workflowOrgID, instance.ID)
	require.NoError(t, err)
	require.NotEmpty(t, transitions, "new case should have valid transitions available")

	keys := make([]string, 0, len(transitions))
	for _, tr := range transitions {
		keys = append(keys, tr.Key)
	}
	assert.Contains(t, keys, "open")
	assert.Contains(t, keys, "review")
}

func TestRegression_ExecutingTransitionUpdatesWorkflowState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping regression test")
	}

	_, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()
	helpers.SeedDefaultRoles(db, workflowOrgID)
	actorID := helpers.SeedUser(db, workflowOrgID)

	c, err := caseSvc.CreateCase(ctx, application.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Execute Transition Test",
		Description:    "Test",
		ServiceType:    caseDomain.ServiceTypeEmergency,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	workflowSvc := caseSvc.GetWorkflowService()
	require.NotNil(t, workflowSvc)

	instance, err := workflowSvc.GetInstanceByCaseID(ctx, workflowOrgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "NEW", instance.CurrentState)

	_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{
		TenantID:      workflowOrgID,
		InstanceID:    instance.ID,
		TransitionKey: "open",
		ActorID:       actorID,
		ActorRole:     "",
		Reason:        "",
	})
	require.NoError(t, err)

	instance, err = workflowSvc.GetInstanceByCaseID(ctx, workflowOrgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "OPEN", instance.CurrentState)
}

func TestRegression_CaseStatusAndWorkflowStateCannotContradict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping regression test")
	}

	_, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()
	helpers.SeedDefaultRoles(db, workflowOrgID)
	actorID := helpers.SeedUser(db, workflowOrgID)

	c, err := caseSvc.CreateCase(ctx, application.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Consistency Test",
		Description:    "Test",
		ServiceType:    caseDomain.ServiceTypeEmergency,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	workflowSvc := caseSvc.GetWorkflowService()
	require.NotNil(t, workflowSvc)

	instance, err := workflowSvc.GetInstanceByCaseID(ctx, workflowOrgID, c.ID)
	require.NoError(t, err)

	transitions := []string{"open", "assess", "assess2", "decide", "approve", "start_assistance", "follow_up", "complete"}
	for _, key := range transitions {
		_, err := workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{
			TenantID:      workflowOrgID,
			InstanceID:    instance.ID,
			TransitionKey: key,
			ActorID:       actorID,
			ActorRole:     "",
			Reason:        "",
		})
		require.NoError(t, err, "transition %s should succeed", key)

		instance, err = workflowSvc.GetInstanceByCaseID(ctx, workflowOrgID, c.ID)
		require.NoError(t, err)

		updatedCase, err := caseSvc.GetCase(ctx, workflowOrgID, c.ID)
		require.NoError(t, err)

		assert.Equal(t, caseDomain.CaseStatus(instance.CurrentState), updatedCase.Status,
			"case status (%s) must match workflow state (%s)", updatedCase.Status, instance.CurrentState)
	}
}

func TestRegression_CompletedDemoCaseShowsClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping regression test")
	}

	_, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()
	helpers.SeedDefaultRoles(db, workflowOrgID)
	actorID := helpers.SeedUser(db, workflowOrgID)

	c, err := caseSvc.CreateCase(ctx, application.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Completed Demo",
		Description:    "Test",
		ServiceType:    caseDomain.ServiceTypeEmergency,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	workflowSvc := caseSvc.GetWorkflowService()
	require.NotNil(t, workflowSvc)

	instance, err := workflowSvc.GetInstanceByCaseID(ctx, workflowOrgID, c.ID)
	require.NoError(t, err)

	transitions := []string{"open", "assess", "assess2", "decide", "approve", "start_assistance", "follow_up", "complete"}
	for _, key := range transitions {
		_, err := workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{
			TenantID:      workflowOrgID,
			InstanceID:    instance.ID,
			TransitionKey: key,
			ActorID:       actorID,
			ActorRole:     "",
			Reason:        "",
		})
		require.NoError(t, err, "transition %s should succeed", key)

		instance, err = workflowSvc.GetInstanceByCaseID(ctx, workflowOrgID, c.ID)
		require.NoError(t, err)
	}

	assert.Equal(t, "CLOSED", instance.CurrentState)

	updatedCase, err := caseSvc.GetCase(ctx, workflowOrgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusClosed, updatedCase.Status)
	assert.NotNil(t, updatedCase.ClosedAt)
}

func TestRegression_CompletedCaseRetainsDomainData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping regression test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)
	ctx := context.Background()

	orgID := helpers.SeedOrg(db)
	helpers.SeedDefaultRoles(db, orgID)
	adminID := helpers.SeedUser(db, orgID)
	staffID := helpers.SeedUser(db, orgID)
	personID := helpers.SeedPerson(db, orgID)

	eligibilityRepo := eligibilitypostgres.NewPostgresEligibilityRepository(db)
	evidenceRepo := evidencepostgres.NewPostgresEvidenceRepository(db)
	assessmentRepo := assessmentpostgres.NewPostgresAssessmentRepository(db)
	decisionRepo := decisionspostgres.NewPostgresDecisionRepository(db)
	assistanceRepo := assistancepostgres.NewPostgresAssistanceRepository(db)
	followUpRepo := followuppostgres.NewPostgresFollowUpRepository(db)

	caseID := uuid.New()
	_, err := db.ExecContext(ctx, `INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, person_id, created_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())`,
		caseID, orgID, "REG-CMP-001", "Regression Completed Case", "Test", "CLOSED", "EMERGENCY", "URGENT", &personID, adminID)
	require.NoError(t, err)

	eligibilityID := uuid.New()
	_, err = db.ExecContext(ctx, `INSERT INTO eligibilities (id, organization_id, service_request_id, criteria, result, explanation, assessed_by, assessed_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		eligibilityID, orgID, caseID, `{}`, "ELIGIBLE", "Test", adminID)
	require.NoError(t, err)

	evidenceID := uuid.New()
	_, err = db.ExecContext(ctx, `INSERT INTO evidence (id, organization_id, service_request_id, type, description, storage_reference, uploaded_by, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`,
		evidenceID, orgID, caseID, "IDENTITY_DOCUMENT", "Test evidence", "s3://test", adminID)
	require.NoError(t, err)

	assessmentID := uuid.New()
	_, err = db.ExecContext(ctx, `INSERT INTO assessments (id, organization_id, service_request_id, findings, needs_identified, recommendation, assessor, assessed_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		assessmentID, orgID, caseID, "Findings", "Needs", "Recommendation", staffID)
	require.NoError(t, err)

	decisionID := uuid.New()
	_, err = db.ExecContext(ctx, `INSERT INTO decisions (id, organization_id, service_request_id, decision, reason, decision_maker, decided_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())`,
		decisionID, orgID, caseID, "APPROVED", "Reason", adminID)
	require.NoError(t, err)

	assistanceID := uuid.New()
	_, err = db.ExecContext(ctx, `INSERT INTO assistance (id, organization_id, service_request_id, type, description, status, responsible_staff, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		assistanceID, orgID, caseID, "SHELTER", "Assistance", "COMPLETED", staffID)
	require.NoError(t, err)

	followUpID := uuid.New()
	_, err = db.ExecContext(ctx, `INSERT INTO follow_ups (id, organization_id, service_request_id, scheduled_date, outcome, notes, performed_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		followUpID, orgID, caseID, "2026-10-15", "Outcome", "Notes", staffID)
	require.NoError(t, err)

	eligibility, err := eligibilityRepo.FindByServiceRequest(ctx, orgID, caseID)
	require.NoError(t, err)
	require.NotNil(t, eligibility)

	evidence, err := evidenceRepo.FindByServiceRequest(ctx, orgID, caseID, 100, 0)
	require.NoError(t, err)
	require.Len(t, evidence, 1)

	assessment, err := assessmentRepo.FindByServiceRequest(ctx, orgID, caseID)
	require.NoError(t, err)
	require.NotNil(t, assessment)

	decision, err := decisionRepo.FindByServiceRequest(ctx, orgID, caseID)
	require.NoError(t, err)
	require.NotNil(t, decision)

	assistance, err := assistanceRepo.FindByServiceRequest(ctx, orgID, caseID, 100, 0)
	require.NoError(t, err)
	require.Len(t, assistance, 1)

	followUp, err := followUpRepo.FindByServiceRequest(ctx, orgID, caseID, 100, 0)
	require.NoError(t, err)
	require.Len(t, followUp, 1)
}

func TestRegression_GenericWorkflowStillWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping regression test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)
	ctx := context.Background()

	orgID := helpers.SeedOrg(db)
	now := time.Now().UTC()

	defRepo := workflowpostgres.NewPostgresWorkflowDefinitionRepository(db)
	stateRepo := workflowpostgres.NewPostgresWorkflowStateRepository(db)
	transitionRepo := workflowpostgres.NewPostgresWorkflowTransitionRepository(db)
	instanceRepo := workflowpostgres.NewPostgresWorkflowInstanceRepository(db)
	historyRepo := workflowpostgres.NewPostgresWorkflowTransitionHistoryRepository(db)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db)
	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	workflowSvc := workflowapp.NewWorkflowService(defRepo, stateRepo, transitionRepo, instanceRepo, historyRepo, auditService)

	states := []workflowdomain.WorkflowState{
		{ID: uuid.New(), TenantID: orgID, Key: "START", Name: "Start", Terminal: false, DisplayOrder: 0, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "MIDDLE", Name: "Middle", Terminal: false, DisplayOrder: 1, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "END", Name: "End", Terminal: true, DisplayOrder: 2, CreatedAt: now},
	}
	transitions := []workflowdomain.WorkflowTransition{
		{ID: uuid.New(), TenantID: orgID, Key: "advance", Name: "Advance", FromState: "START", ToState: "MIDDLE", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "finish", Name: "Finish", FromState: "MIDDLE", ToState: "END", Active: true, CreatedAt: now},
	}

	def, err := workflowSvc.CreateWorkflowDefinition(ctx, workflowapp.CreateWorkflowDefinitionParams{
		TenantID:     orgID,
		ActorID:      uuid.Nil,
		Key:          "generic_test",
		Name:         "Generic Test",
		Description:  "Test",
		Version:      1,
		InitialState: "START",
		States:       states,
		Transitions:  transitions,
		Metadata:     map[string]interface{}{},
	})
	require.NoError(t, err)

	err = workflowSvc.ActivateWorkflowDefinition(ctx, orgID, def.ID, uuid.Nil)
	require.NoError(t, err)

	caseID := uuid.New()
	actorID := helpers.SeedUser(db, orgID)
	_, err = db.ExecContext(ctx, `INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, created_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())`,
		caseID, orgID, "GEN-001", "Generic Test", "", "NEW", "GENERAL", "NORMAL", actorID)
	require.NoError(t, err)

	instance, err := workflowSvc.CreateInstanceForCase(ctx, orgID, caseID, def.Key, uuid.Nil)
	require.NoError(t, err)
	assert.Equal(t, "START", instance.CurrentState)

	_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{
		TenantID:      orgID,
		InstanceID:    instance.ID,
		TransitionKey: "advance",
		ActorID:       actorID,
		ActorRole:     "",
		Reason:        "",
	})
	require.NoError(t, err)

	instance, err = workflowSvc.GetInstanceByCaseID(ctx, orgID, caseID)
	require.NoError(t, err)
	assert.Equal(t, "MIDDLE", instance.CurrentState)

	_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{
		TenantID:      orgID,
		InstanceID:    instance.ID,
		TransitionKey: "finish",
		ActorID:       actorID,
		ActorRole:     "",
		Reason:        "",
	})
	require.NoError(t, err)

	instance, err = workflowSvc.GetInstanceByCaseID(ctx, orgID, caseID)
	require.NoError(t, err)
	assert.Equal(t, "END", instance.CurrentState)
}

func TestRegression_MedicalAssistanceUsesGenericWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping regression test")
	}

	_, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()
	helpers.SeedDefaultRoles(db, workflowOrgID)
	actorID := helpers.SeedUser(db, workflowOrgID)

	c, err := caseSvc.CreateCase(ctx, application.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Medical Assistance Request",
		Description:    "Test",
		ServiceType:    caseDomain.ServiceTypeMedical,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusNew, c.Status)

	_, err = caseSvc.ChangeStatus(ctx, application.ChangeCaseStatusParams{
		OrganizationID: workflowOrgID,
		CaseID:         c.ID,
		Status:         caseDomain.CaseStatusOpen,
		ActorID:        actorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)

	updated, err := caseSvc.GetCase(ctx, workflowOrgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusOpen, updated.Status)
	assert.Equal(t, caseDomain.ServiceTypeMedical, updated.ServiceType)
}

func TestRegression_OpenCaseCannotDisplayClosedWorkflowState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping regression test")
	}

	_, caseSvc, db, workflowOrgID := setupAppServices(t)
	ctx := context.Background()
	helpers.SeedDefaultRoles(db, workflowOrgID)
	actorID := helpers.SeedUser(db, workflowOrgID)

	c, err := caseSvc.CreateCase(ctx, application.CreateCaseParams{
		OrganizationID: workflowOrgID,
		Title:          "Contradiction Test",
		Description:    "Test",
		ServiceType:    caseDomain.ServiceTypeEmergency,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	workflowSvc := caseSvc.GetWorkflowService()
	require.NotNil(t, workflowSvc)

	instance, err := workflowSvc.GetInstanceByCaseID(ctx, workflowOrgID, c.ID)
	require.NoError(t, err)

	_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{
		TenantID:      workflowOrgID,
		InstanceID:    instance.ID,
		TransitionKey: "open",
		ActorID:       actorID,
		ActorRole:     "",
		Reason:        "",
	})
	require.NoError(t, err)

	instance, err = workflowSvc.GetInstanceByCaseID(ctx, workflowOrgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "OPEN", instance.CurrentState)

	updatedCase, err := caseSvc.GetCase(ctx, workflowOrgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusOpen, updatedCase.Status)

	err = updatedCase.ValidateConsistency(instance.CurrentState)
	assert.NoError(t, err, "case status and workflow state must not contradict")
}
