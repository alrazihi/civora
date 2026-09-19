package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	casepostgres "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	submissionpostgres "github.com/alrazihi/civora/internal/form_submission/infrastructure/postgres"
	formpostgres "github.com/alrazihi/civora/internal/forms/infrastructure/postgres"
	rulesapp "github.com/alrazihi/civora/internal/rules/application"
	rulesdomain "github.com/alrazihi/civora/internal/rules/domain"
	rulesinfra "github.com/alrazihi/civora/internal/rules/infrastructure/postgres"
	workflowpostgres "github.com/alrazihi/civora/internal/workflow/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type caseRuleEnv struct {
	integration    *rulesapp.CaseRuleIntegrationService
	ruleSetSvc     *rulesapp.RuleSetService
	assignmentSvc  *rulesapp.RuleAssignmentService
	db             *sql.DB
	orgID, actorID uuid.UUID
}

func setupCaseRuleEnv(t *testing.T) *caseRuleEnv {
	t.Helper()
	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	orgID := helpers.SeedOrg(db)
	actorID := helpers.SeedUser(db, orgID)
	helpers.SeedDefaultRoles(db, orgID)

	auditRepo := auditpostgres.NewPostgresAuditRepository(db)
	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	ruleSetRepo := rulesinfra.NewPostgresRuleSetRepository(db)
	evalRepo := rulesinfra.NewPostgresEvaluationRepository(db)
	templateRepo := rulesinfra.NewRuleTemplateRepository(db)
	assignmentRepo := rulesinfra.NewPostgresWorkflowStateRuleAssignmentRepository(db)
	caseRepo := casepostgres.NewPostgresCaseRepository(db)
	workflowInstanceRepo := workflowpostgres.NewPostgresWorkflowInstanceRepository(db)
	submissionRepo := submissionpostgres.NewPostgresFormSubmissionRepository(db)
	formRepo := formpostgres.NewPostgresFormRepository(db)
	formKeyResolver := rulesapp.NewFormKeyResolverAdapter(formRepo)

	integration := rulesapp.NewCaseRuleIntegrationService(
		ruleSetRepo, evalRepo, caseRepo, workflowInstanceRepo,
		assignmentRepo, submissionRepo, formKeyResolver, auditService,
	)

	ruleSetSvc := rulesapp.NewRuleSetService(ruleSetRepo, evalRepo, templateRepo, nil, auditService)
	assignmentSvc := rulesapp.NewRuleAssignmentService(assignmentRepo, ruleSetRepo, auditService)

	return &caseRuleEnv{
		integration:   integration,
		ruleSetSvc:    ruleSetSvc,
		assignmentSvc: assignmentSvc,
		db:            db,
		orgID:         orgID,
		actorID:       actorID,
	}
}

func (e *caseRuleEnv) seedCaseWithWorkflow(t *testing.T) (uuid.UUID, uuid.UUID) {
	caseID := uuid.New()
	instanceID := uuid.New()
	workflowDefID := uuid.New()
	stateID := uuid.New()
	ctx := context.Background()

	_, err := e.db.ExecContext(ctx, `
		INSERT INTO workflow_definitions (id, organization_id, key, name, description, version, status, initial_state, metadata, created_at, updated_at)
		VALUES ($1, $2, 'test-workflow', 'Test Workflow', '', 1, 'ACTIVE', 'OPEN', '{}', NOW(), NOW())
	`, workflowDefID, e.orgID)
	require.NoError(t, err, "insert workflow_definition")

	_, err = e.db.ExecContext(ctx, `
		INSERT INTO workflow_states (id, workflow_definition_id, organization_id, key, name, description, category, terminal, display_order, responsible_role, created_at)
		VALUES ($1, $2, $3, 'OPEN', 'Open', '', 'initial', false, 0, NULL, NOW())
	`, stateID, workflowDefID, e.orgID)
	require.NoError(t, err, "insert workflow_state")

	// Match production insertion order: the case is created first with
	// workflow_instance_id = NULL, the workflow instance is created next, and
	// the case is then linked back. This avoids the circular FK between
	// cases.workflow_instance_id -> workflow_instances.id and
	// workflow_instances.case_id -> cases.id.
	_, err = e.db.ExecContext(ctx, `
		INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, person_id, created_by, assigned_to, created_at, updated_at, closed_at, version, workflow_instance_id, workflow_state, workflow_key, workflow_id)
		VALUES ($1, $2, 'CASE-001', 'Test Case', '', 'OPEN', 'GENERAL', 'NORMAL', NULL, $3, NULL, NOW(), NOW(), NULL, 1, NULL, NULL, 'test-workflow', $4)
	`, caseID, e.orgID, e.actorID, workflowDefID)
	require.NoError(t, err, "insert case")

	_, err = e.db.ExecContext(ctx, `
		INSERT INTO workflow_instances (id, organization_id, workflow_definition_id, workflow_definition_version, case_id, current_state, started_at, completed_at, metadata, version)
		VALUES ($1, $2, $3, 1, $4, 'OPEN', NOW(), NULL, '{}', 1)
	`, instanceID, e.orgID, workflowDefID, caseID)
	require.NoError(t, err, "insert workflow_instance")

	_, err = e.db.ExecContext(ctx, `
		UPDATE cases SET workflow_instance_id = $1, workflow_state = 'OPEN', updated_at = NOW() WHERE id = $2
	`, instanceID, caseID)
	require.NoError(t, err, "link case to workflow instance")

	return caseID, workflowDefID
}

func (e *caseRuleEnv) seedFormAndSubmission(t *testing.T, caseID uuid.UUID) {
	formID := uuid.New()
	formVersionID := uuid.New()
	ctx := context.Background()

	_, err := e.db.ExecContext(ctx, `
		INSERT INTO forms (id, organization_id, key, name, description, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'income', 'Income Form', '', 'ACTIVE', $3, NOW(), NOW())
	`, formID, e.orgID, e.actorID)
	require.NoError(t, err, "insert form")

	_, err = e.db.ExecContext(ctx, `
		INSERT INTO form_versions (id, form_id, organization_id, version, status, created_by, created_at, published_at, updated_at)
		VALUES ($1, $2, $3, 1, 'PUBLISHED', $4, NOW(), NOW(), NOW())
	`, formVersionID, formID, e.orgID, e.actorID)
	require.NoError(t, err, "insert form_version")

	submissionData, _ := json.Marshal(map[string]interface{}{
		"amount": float64(300),
	})
	_, err = e.db.ExecContext(ctx, `
		INSERT INTO form_submissions (id, tenant_id, case_id, form_id, form_version_id, submitted_by, status, data, submitted_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, 'submitted', $6, NOW(), NOW())
	`, e.orgID, caseID, formID, formVersionID, e.actorID, string(submissionData))
	require.NoError(t, err, "insert form_submission")
}

func (e *caseRuleEnv) createPublishedRuleSet(t *testing.T, key string, rules []rulesdomain.Rule, triggers []string) *rulesdomain.RuleSet {
	ctx := context.Background()
	rs, err := e.ruleSetSvc.CreateRuleSet(ctx, rulesapp.CreateRuleSetParams{
		OrganizationID: e.orgID,
		Key:            key,
		Name:           key,
		Description:    "Test",
		DefaultOutcome: rulesdomain.OutcomeIneligible,
		Rules:          rules,
		Triggers:       triggers,
		ActorID:        e.actorID,
	})
	require.NoError(t, err)

	published, err := e.ruleSetSvc.PublishRuleSet(ctx, e.orgID, rs.ID, e.actorID)
	require.NoError(t, err)
	return published
}

func (e *caseRuleEnv) assignRuleSet(t *testing.T, workflowDefID, ruleSetID uuid.UUID, stateKey string) {
	ctx := context.Background()
	_, err := e.assignmentSvc.CreateRuleAssignment(ctx, rulesapp.CreateRuleAssignmentParams{
		OrganizationID:   e.orgID,
		WorkflowDefID:    workflowDefID,
		WorkflowStateKey: stateKey,
		RuleSetID:        ruleSetID,
		Required:         true,
		Active:           true,
		DisplayOrder:     0,
		ActorID:          e.actorID,
	})
	require.NoError(t, err)
}

func TestAssembleFactsFromCase_BuildsFormStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := setupCaseRuleEnv(t)
	caseID, _ := env.seedCaseWithWorkflow(t)
	env.seedFormAndSubmission(t, caseID)

	ctx := context.Background()
	facts, err := env.integration.AssembleFactsFromCase(ctx, nil, env.orgID, caseID)
	require.NoError(t, err)

	assert.Contains(t, facts, "case", "facts must contain case.* attributes")
	assert.Contains(t, facts, "form", "facts must contain form.* attributes")

	formFacts := facts["form"].(map[string]interface{})
	assert.Contains(t, formFacts, "income", "form key must be resolved to the form's key")
	incomeForm := formFacts["income"].(map[string]interface{})
	assert.Contains(t, incomeForm, "amount", "form submission data must be included in facts")
	assert.Equal(t, float64(300), incomeForm["amount"])
}

func TestEvaluateCaseRules_PublishedRuleSetMatches(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := setupCaseRuleEnv(t)
	caseID, workflowDefID := env.seedCaseWithWorkflow(t)
	env.seedFormAndSubmission(t, caseID)

	published := env.createPublishedRuleSet(t, "income-eligibility", []rulesdomain.Rule{
		{
			Priority:   0,
			Outcome:    rulesdomain.OutcomeEligible,
			Conditions: rulesdomain.Condition{Field: "form.income.amount", Operator: rulesdomain.OpLessThanEqual, Value: json.Number("500")},
		},
	}, []string{"workflow_transition"})
	env.assignRuleSet(t, workflowDefID, published.ID, "OPEN")

	result, err := env.integration.EvaluateCaseRules(context.Background(), rulesapp.EvaluateCaseRulesParams{
		OrganizationID: env.orgID,
		CaseID:         caseID,
		ActorID:        env.actorID,
		Trigger:        rulesdomain.TriggerManual,
		Event:          "workflow_transition",
	})
	require.NoError(t, err)
	assert.Len(t, result.Evaluations, 1)
	assert.Equal(t, rulesdomain.OutcomeEligible, result.Evaluations[0].Outcome)
	assert.False(t, result.HasMissingFacts)
}

func TestEvaluateCaseRules_SkipsArchivedRuleSets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := setupCaseRuleEnv(t)
	caseID, workflowDefID := env.seedCaseWithWorkflow(t)
	env.seedFormAndSubmission(t, caseID)

	rs := env.createPublishedRuleSet(t, "archived-check", []rulesdomain.Rule{
		{
			Priority:   0,
			Outcome:    rulesdomain.OutcomeEligible,
			Conditions: rulesdomain.Condition{Field: "form.income.amount", Operator: rulesdomain.OpLessThanEqual, Value: json.Number("500")},
		},
	}, []string{"workflow_transition"})

	archived, err := env.ruleSetSvc.ArchiveRuleSet(context.Background(), env.orgID, rs.ID, env.actorID)
	require.NoError(t, err)
	assert.Equal(t, rulesdomain.StatusArchived, archived.Status)

	env.assignRuleSet(t, workflowDefID, rs.ID, "OPEN")

	result, err := env.integration.EvaluateCaseRules(context.Background(), rulesapp.EvaluateCaseRulesParams{
		OrganizationID: env.orgID,
		CaseID:         caseID,
		ActorID:        env.actorID,
		Trigger:        rulesdomain.TriggerManual,
		Event:          "workflow_transition",
	})
	require.NoError(t, err)
	assert.Len(t, result.Evaluations, 0, "ARCHIVED rule sets must not be evaluated")
}

func TestEvaluateCaseRules_SkipsNonMatchingTriggers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := setupCaseRuleEnv(t)
	caseID, workflowDefID := env.seedCaseWithWorkflow(t)
	env.seedFormAndSubmission(t, caseID)

	published := env.createPublishedRuleSet(t, "form-trigger-only", []rulesdomain.Rule{
		{
			Priority:   0,
			Outcome:    rulesdomain.OutcomeEligible,
			Conditions: rulesdomain.Condition{Field: "form.income.amount", Operator: rulesdomain.OpLessThanEqual, Value: json.Number("500")},
		},
	}, []string{"form_submitted"})
	env.assignRuleSet(t, workflowDefID, published.ID, "OPEN")

	result, err := env.integration.EvaluateCaseRules(context.Background(), rulesapp.EvaluateCaseRulesParams{
		OrganizationID: env.orgID,
		CaseID:         caseID,
		ActorID:        env.actorID,
		Trigger:        rulesdomain.TriggerManual,
		Event:          "workflow_transition",
	})
	require.NoError(t, err)
	assert.Len(t, result.Evaluations, 0, "rule sets with non-matching triggers must not be evaluated")
}
