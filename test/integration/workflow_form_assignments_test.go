package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	formapp "github.com/alrazihi/civora/internal/forms/application"
	formdomain "github.com/alrazihi/civora/internal/forms/domain"
	formpostgres "github.com/alrazihi/civora/internal/forms/infrastructure/postgres"
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	workflowpostgres "github.com/alrazihi/civora/internal/workflow/infrastructure/postgres"
	assignmentapp "github.com/alrazihi/civora/internal/workflow_form_assignment/application"
	assignmentpostgres "github.com/alrazihi/civora/internal/workflow_form_assignment/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAssignmentServices(t *testing.T, db *sql.DB) (
	*assignmentapp.WorkflowStateFormAssignmentService,
	uuid.UUID,
	*workflowapp.WorkflowService,
	*formapp.FormService,
) {
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

	formRepo := formpostgres.NewPostgresFormRepository(db)
	formVersionRepo := formpostgres.NewPostgresFormVersionRepository(db)
	formFieldRepo := formpostgres.NewPostgresFormFieldRepository(db)
	formSvc := formapp.NewFormService(formRepo, formVersionRepo, formFieldRepo, auditService)

	assignmentRepo := assignmentpostgres.NewPostgresWorkflowStateFormAssignmentRepository(db)
	assignmentSvc := assignmentapp.NewWorkflowStateFormAssignmentService(defRepo, stateRepo, formRepo, formVersionRepo, assignmentRepo, auditService)

	return assignmentSvc, orgID, workflowSvc, formSvc
}

func seedWorkflowWithState(t *testing.T, db *sql.DB, svc *workflowapp.WorkflowService, orgID, actorID uuid.UUID) *workflowdomain.WorkflowDefinition {
	t.Helper()
	states := []workflowdomain.WorkflowState{
		{ID: uuid.New(), TenantID: orgID, Key: "OPEN", Name: "Open", Terminal: false, DisplayOrder: 0, CreatedAt: time.Now().UTC()},
		{ID: uuid.New(), TenantID: orgID, Key: "ASSESSMENT", Name: "Assessment", Terminal: false, DisplayOrder: 1, CreatedAt: time.Now().UTC()},
		{ID: uuid.New(), TenantID: orgID, Key: "DECISION", Name: "Decision", Terminal: true, DisplayOrder: 2, CreatedAt: time.Now().UTC()},
	}
	transitions := []workflowdomain.WorkflowTransition{
		{ID: uuid.New(), TenantID: orgID, Key: "open", Name: "Open", FromState: "OPEN", ToState: "ASSESSMENT", Active: true, CreatedAt: time.Now().UTC()},
		{ID: uuid.New(), TenantID: orgID, Key: "decide", Name: "Decide", FromState: "ASSESSMENT", ToState: "DECISION", Active: true, CreatedAt: time.Now().UTC()},
	}

	def, err := svc.CreateWorkflowDefinition(context.Background(), workflowapp.CreateWorkflowDefinitionParams{
		TenantID:     orgID,
		ActorID:      actorID,
		Key:          "test-workflow-" + uuid.New().String()[:8],
		Name:         "Test Workflow",
		Description:  "Test",
		Version:      1,
		InitialState: "OPEN",
		States:       states,
		Transitions:  transitions,
		Metadata:     map[string]interface{}{},
	})
	require.NoError(t, err)

	err = svc.ActivateWorkflowDefinition(context.Background(), orgID, def.ID, actorID)
	require.NoError(t, err)

	return def
}

func seedPublishedForm(t *testing.T, db *sql.DB, formSvc *formapp.FormService, orgID, actorID uuid.UUID) (*formdomain.Form, *formdomain.FormVersion) {
	t.Helper()
	form, err := formSvc.CreateForm(context.Background(), formapp.CreateFormParams{
		OrganizationID: orgID,
		Key:            "test-form-" + uuid.New().String()[:8],
		Name:           "Test Form",
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	version, err := formSvc.CreateVersion(context.Background(), formapp.CreateVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
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

func TestAssignment_CreateSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	assignmentSvc, orgID, workflowSvc, formSvc := setupAssignmentServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	workflowDef := seedWorkflowWithState(t, db, workflowSvc, orgID, actorID)
	_, version := seedPublishedForm(t, db, formSvc, orgID, actorID)

	assignment, err := assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDef.ID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               version.FormID,
		FormVersionID:        version.ID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "ASSESSMENT", assignment.WorkflowStateKey)
	assert.Equal(t, version.ID, assignment.FormVersionID)
	assert.True(t, assignment.Required)
	assert.True(t, assignment.Active)
}

func TestAssignment_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	ctx := context.Background()

	assignmentSvc1, org1, workflowSvc1, formSvc1 := setupAssignmentServices(t, db)
	assignmentSvc2, org2, _, _ := setupAssignmentServices(t, db)
	actor1 := helpers.SeedUser(db, org1)
	_ = helpers.SeedUser(db, org2)

	workflowDef1 := seedWorkflowWithState(t, db, workflowSvc1, org1, actor1)
	_, version1 := seedPublishedForm(t, db, formSvc1, org1, actor1)

	_, err := assignmentSvc1.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
		TenantID:             org1,
		WorkflowDefinitionID: workflowDef1.ID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               version1.FormID,
		FormVersionID:        version1.ID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actor1,
	})
	require.NoError(t, err)

	_, err = assignmentSvc2.GetAssignment(ctx, org2, uuid.New())
	require.Error(t, err)
	assert.ErrorContains(t, err, "assignment not found")
}

func TestAssignment_DuplicatePrevention(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	assignmentSvc, orgID, workflowSvc, formSvc := setupAssignmentServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	workflowDef := seedWorkflowWithState(t, db, workflowSvc, orgID, actorID)
	_, version := seedPublishedForm(t, db, formSvc, orgID, actorID)

	_, err := assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDef.ID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               version.FormID,
		FormVersionID:        version.ID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.NoError(t, err)

	_, err = assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDef.ID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               version.FormID,
		FormVersionID:        version.ID,
		Required:             false,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "assignment already exists")
}

func TestAssignment_InvalidWorkflowState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	assignmentSvc, orgID, workflowSvc, formSvc := setupAssignmentServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	workflowDef := seedWorkflowWithState(t, db, workflowSvc, orgID, actorID)
	_, version := seedPublishedForm(t, db, formSvc, orgID, actorID)

	_, err := assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDef.ID,
		WorkflowStateKey:     "NONEXISTENT",
		FormID:               version.FormID,
		FormVersionID:        version.ID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid workflow state key")
}

func TestAssignment_DraftFormVersionRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	assignmentSvc, orgID, workflowSvc, formSvc := setupAssignmentServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	workflowDef := seedWorkflowWithState(t, db, workflowSvc, orgID, actorID)
	form, _ := seedPublishedForm(t, db, formSvc, orgID, actorID)

	draftVersion, err := formSvc.CreateVersion(ctx, formapp.CreateVersionParams{
		OrganizationID: orgID,
		FormID:         form.ID,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDef.ID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               form.ID,
		FormVersionID:        draftVersion.ID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid form version")
}

func TestAssignment_UpdateSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	assignmentSvc, orgID, workflowSvc, formSvc := setupAssignmentServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	workflowDef := seedWorkflowWithState(t, db, workflowSvc, orgID, actorID)
	_, version := seedPublishedForm(t, db, formSvc, orgID, actorID)

	created, err := assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDef.ID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               version.FormID,
		FormVersionID:        version.ID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.NoError(t, err)

	updated, err := assignmentSvc.UpdateAssignment(ctx, assignmentapp.UpdateAssignmentParams{
		TenantID:     orgID,
		AssignmentID: created.ID,
		Required:     boolPtr(false),
		Active:       boolPtr(false),
	})
	require.NoError(t, err)
	assert.False(t, updated.Required)
	assert.False(t, updated.Active)
}

func TestAssignment_DeleteSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	assignmentSvc, orgID, workflowSvc, formSvc := setupAssignmentServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	workflowDef := seedWorkflowWithState(t, db, workflowSvc, orgID, actorID)
	_, version := seedPublishedForm(t, db, formSvc, orgID, actorID)

	created, err := assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDef.ID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               version.FormID,
		FormVersionID:        version.ID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.NoError(t, err)

	err = assignmentSvc.DeleteAssignment(ctx, orgID, created.ID, actorID)
	require.NoError(t, err)

	_, err = assignmentSvc.GetAssignment(ctx, orgID, created.ID)
	require.Error(t, err)
	assert.ErrorContains(t, err, "assignment not found")
}

func TestAssignment_ListByWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	assignmentSvc, orgID, workflowSvc, formSvc := setupAssignmentServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	workflowDef := seedWorkflowWithState(t, db, workflowSvc, orgID, actorID)
	_, version := seedPublishedForm(t, db, formSvc, orgID, actorID)

	_, err := assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDef.ID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               version.FormID,
		FormVersionID:        version.ID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.NoError(t, err)

	assignments, err := assignmentSvc.ListAssignmentsByWorkflow(ctx, orgID, workflowDef.ID)
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	assert.Equal(t, "ASSESSMENT", assignments[0].WorkflowStateKey)
}

func TestAssignment_GetForState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	assignmentSvc, orgID, workflowSvc, formSvc := setupAssignmentServices(t, db)
	ctx := context.Background()
	actorID := helpers.SeedUser(db, orgID)

	workflowDef := seedWorkflowWithState(t, db, workflowSvc, orgID, actorID)
	_, version := seedPublishedForm(t, db, formSvc, orgID, actorID)

	_, err := assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowDef.ID,
		WorkflowStateKey:     "ASSESSMENT",
		FormID:               version.FormID,
		FormVersionID:        version.ID,
		Required:             true,
		DisplayOrder:         0,
		Active:               true,
		CreatedBy:            actorID,
	})
	require.NoError(t, err)

	assignments, err := assignmentSvc.GetAssignmentsForState(ctx, orgID, workflowDef.ID, "ASSESSMENT")
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	assert.Equal(t, version.ID, assignments[0].FormVersionID)
}

func boolPtr(b bool) *bool {
	return &b
}
