package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	caseapp "github.com/alrazihi/civora/internal/cases/application"
	casedomain "github.com/alrazihi/civora/internal/cases/domain"
	caseinfra "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	submissioninfra "github.com/alrazihi/civora/internal/form_submission/infrastructure/postgres"
	formapp "github.com/alrazihi/civora/internal/forms/application"
	formdomain "github.com/alrazihi/civora/internal/forms/domain"
	forminfra "github.com/alrazihi/civora/internal/forms/infrastructure/postgres"
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	workflowinfra "github.com/alrazihi/civora/internal/workflow/infrastructure/postgres"
	assignmentdomain "github.com/alrazihi/civora/internal/workflow_form_assignment/domain"
	assignmentinfra "github.com/alrazihi/civora/internal/workflow_form_assignment/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupFormSubmissionServices(t *testing.T, db *sql.DB) (
	*caseapp.CaseService,
	*formapp.FormService,
	*workflowapp.WorkflowService,
	uuid.UUID,
) {
	t.Helper()
	orgID := helpers.SeedOrg(db)

	defRepo := workflowinfra.NewPostgresWorkflowDefinitionRepository(db)
	stateRepo := workflowinfra.NewPostgresWorkflowStateRepository(db)
	transitionRepo := workflowinfra.NewPostgresWorkflowTransitionRepository(db)
	instanceRepo := workflowinfra.NewPostgresWorkflowInstanceRepository(db)
	historyRepo := workflowinfra.NewPostgresWorkflowTransitionHistoryRepository(db)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db)
	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	workflowSvc := workflowapp.NewWorkflowService(defRepo, stateRepo, transitionRepo, instanceRepo, historyRepo, auditService)

	caseRepo := caseinfra.NewPostgresCaseRepository(db)
	caseSvc := caseapp.NewCaseService(caseRepo, nil, nil, auditService, auditRepo, workflowSvc)

	formRepo := forminfra.NewPostgresFormRepository(db)
	versionRepo := forminfra.NewPostgresFormVersionRepository(db)
	fieldRepo := forminfra.NewPostgresFormFieldRepository(db)
	formSvc := formapp.NewFormService(formRepo, versionRepo, fieldRepo, auditService)

	assignmentRepo := assignmentinfra.NewPostgresWorkflowStateFormAssignmentRepository(db)
	submissionRepo := submissioninfra.NewPostgresFormSubmissionRepository(db)

	caseSvc.SetFormRepos(defRepo, instanceRepo, formRepo, versionRepo, fieldRepo, assignmentRepo, submissionRepo)

	return caseSvc, formSvc, workflowSvc, orgID
}

func seedWorkflowWithAssessmentState(t *testing.T, db *sql.DB, workflowSvc *workflowapp.WorkflowService, orgID, actorID uuid.UUID) *workflowdomain.WorkflowDefinition {
	t.Helper()
	now := time.Now().UTC()
	states := []workflowdomain.WorkflowState{
		{ID: uuid.New(), TenantID: orgID, Key: "NEW", Name: "New", Terminal: false, DisplayOrder: 0, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "OPEN", Name: "Open", Terminal: false, DisplayOrder: 1, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "ASSESSMENT", Name: "Assessment", Terminal: false, DisplayOrder: 2, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "DECISION", Name: "Decision", Terminal: true, DisplayOrder: 3, CreatedAt: now},
	}
	transitions := []workflowdomain.WorkflowTransition{
		{ID: uuid.New(), TenantID: orgID, Key: "open", Name: "Open", FromState: "NEW", ToState: "OPEN", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "assess", Name: "Assess", FromState: "OPEN", ToState: "ASSESSMENT", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "decide", Name: "Decide", FromState: "ASSESSMENT", ToState: "DECISION", Active: true, CreatedAt: now},
	}

	def, err := workflowSvc.CreateWorkflowDefinition(context.Background(), workflowapp.CreateWorkflowDefinitionParams{
		TenantID:     orgID,
		ActorID:      actorID,
		Key:          "general_assistance",
		Name:         "General Assistance Workflow",
		Description:  "Test",
		Version:      1,
		InitialState: "NEW",
		States:       states,
		Transitions:  transitions,
		Metadata:     map[string]interface{}{},
	})
	require.NoError(t, err)

	err = workflowSvc.ActivateWorkflowDefinition(context.Background(), orgID, def.ID, actorID)
	require.NoError(t, err)

	return def
}

func seedPublishedFormWithFields(t *testing.T, formSvc *formapp.FormService, orgID, actorID uuid.UUID) (*formdomain.Form, *formdomain.FormVersion) {
	t.Helper()
	form, err := formSvc.CreateForm(context.Background(), formapp.CreateFormParams{
		OrganizationID: orgID,
		Key:            "batch-form-" + uuid.New().String()[:8],
		Name:           "Batch Form",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	version, err := formSvc.CreateVersion(context.Background(), formapp.CreateVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = formSvc.AddField(context.Background(), formapp.AddFieldParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		VersionID:      version.ID,
		Key:            "name",
		Label:          "Name",
		Type:           formdomain.FieldTypeText,
		Required:       true,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = formSvc.AddField(context.Background(), formapp.AddFieldParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		VersionID:      version.ID,
		Key:            "notes",
		Label:          "Notes",
		Type:           formdomain.FieldTypeTextarea,
		Required:       false,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = formSvc.PublishVersion(context.Background(), formapp.PublishVersionParams{
		OrganizationID: orgID,
		VersionID:      version.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	return form, version
}

func TestFormSubmission_GetCaseFormsBatchQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	caseSvc, formSvc, workflowSvc, orgID := setupFormSubmissionServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)
	helpers.SeedDefaultRoles(db, orgID)

	workflowDef := seedWorkflowWithAssessmentState(t, db, workflowSvc, orgID, actorID)
	_, version := seedPublishedFormWithFields(t, formSvc, orgID, actorID)

	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Batch Query Case",
		ServiceType:    casedomain.ServiceTypeGeneral,
		Priority:       casedomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	_, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         casedomain.CaseStatusOpen,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         casedomain.CaseStatusAssessment,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	assignmentRepo := assignmentinfra.NewPostgresWorkflowStateFormAssignmentRepository(db)
	assignment, err := assignmentdomain.NewWorkflowStateFormAssignment(
		orgID, workflowDef.ID, version.FormID, version.ID, actorID,
		"ASSESSMENT", true, 0,
	)
	require.NoError(t, err)
	require.NoError(t, assignmentRepo.Save(ctx, assignment))

	forms, err := caseSvc.GetCaseForms(ctx, orgID, c.ID)
	require.NoError(t, err)
	require.Len(t, forms, 1)
	assert.Equal(t, version.ID, forms[0].FormVersionID)
	assert.Equal(t, "Batch Form", forms[0].FormName)
	assert.Len(t, forms[0].Fields, 2)

	reqs, err := caseSvc.GetWorkflowRequirements(ctx, orgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, reqs.TotalRequired)
	assert.Equal(t, 0, reqs.SubmittedCount)
	assert.Len(t, reqs.MissingRequired, 1)
	assert.False(t, reqs.CanProceed)
}

func TestFormSubmission_GetWorkflowRequirementsWithSubmission(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	caseSvc, formSvc, workflowSvc, orgID := setupFormSubmissionServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)
	helpers.SeedDefaultRoles(db, orgID)

	workflowDef := seedWorkflowWithAssessmentState(t, db, workflowSvc, orgID, actorID)
	_, version := seedPublishedFormWithFields(t, formSvc, orgID, actorID)

	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Requirements With Submission",
		ServiceType:    casedomain.ServiceTypeGeneral,
		Priority:       casedomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	_, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         casedomain.CaseStatusOpen,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         casedomain.CaseStatusAssessment,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	assignmentRepo := assignmentinfra.NewPostgresWorkflowStateFormAssignmentRepository(db)
	assignment, err := assignmentdomain.NewWorkflowStateFormAssignment(
		orgID, workflowDef.ID, version.FormID, version.ID, actorID,
		"ASSESSMENT", true, 0,
	)
	require.NoError(t, err)
	require.NoError(t, assignmentRepo.Save(ctx, assignment))

	_, err = caseSvc.SubmitForm(ctx, orgID, c.ID, actorID, version.ID, map[string]interface{}{"name": "John", "notes": "test"})
	require.NoError(t, err)

	reqs, err := caseSvc.GetWorkflowRequirements(ctx, orgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, reqs.TotalRequired)
	assert.Equal(t, 1, reqs.SubmittedCount)
	assert.Len(t, reqs.MissingRequired, 0)
	assert.True(t, reqs.CanProceed)
	assert.Len(t, reqs.SubmittedForms, 1)
}

func TestFormSubmission_MultipleFormsBatchQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	caseSvc, formSvc, workflowSvc, orgID := setupFormSubmissionServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)
	helpers.SeedDefaultRoles(db, orgID)

	workflowDef := seedWorkflowWithAssessmentState(t, db, workflowSvc, orgID, actorID)
	_, version1 := seedPublishedFormWithFields(t, formSvc, orgID, actorID)
	_, version2 := seedPublishedFormWithFields(t, formSvc, orgID, actorID)

	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: orgID,
		Title:          "Multi Form Case",
		ServiceType:    casedomain.ServiceTypeGeneral,
		Priority:       casedomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	_, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         casedomain.CaseStatusOpen,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         c.ID,
		Status:         casedomain.CaseStatusAssessment,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	assignmentRepo := assignmentinfra.NewPostgresWorkflowStateFormAssignmentRepository(db)
	assignment1, err := assignmentdomain.NewWorkflowStateFormAssignment(
		orgID, workflowDef.ID, version1.FormID, version1.ID, actorID,
		"ASSESSMENT", true, 0,
	)
	require.NoError(t, err)
	require.NoError(t, assignmentRepo.SaveTx(ctx, nil, assignment1))

	assignment2, err := assignmentdomain.NewWorkflowStateFormAssignment(
		orgID, workflowDef.ID, version2.FormID, version2.ID, actorID,
		"ASSESSMENT", true, 1,
	)
	require.NoError(t, err)
	require.NoError(t, assignmentRepo.SaveTx(ctx, nil, assignment2))

	forms, err := caseSvc.GetCaseForms(ctx, orgID, c.ID)
	require.NoError(t, err)
	require.Len(t, forms, 2)

	reqs, err := caseSvc.GetWorkflowRequirements(ctx, orgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, reqs.TotalRequired)
	assert.Equal(t, 0, reqs.SubmittedCount)
	assert.Len(t, reqs.MissingRequired, 2)
	assert.False(t, reqs.CanProceed)
}
