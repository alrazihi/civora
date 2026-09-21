package integration

import (
	"context"
	"testing"
	"time"

	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	caseapp "github.com/alrazihi/civora/internal/cases/application"
	casedomain "github.com/alrazihi/civora/internal/cases/domain"
	caseinfra "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	decapp "github.com/alrazihi/civora/internal/decisions/application"
	decdomain "github.com/alrazihi/civora/internal/decisions/domain"
	decpostgres "github.com/alrazihi/civora/internal/decisions/infrastructure/postgres"
	evidenceapp "github.com/alrazihi/civora/internal/evidence/application"
	evidencepostgres "github.com/alrazihi/civora/internal/evidence/infrastructure/postgres"
	evidencestorage "github.com/alrazihi/civora/internal/evidence/infrastructure/storage"
	submissioninfra "github.com/alrazihi/civora/internal/form_submission/infrastructure/postgres"
	formapp "github.com/alrazihi/civora/internal/forms/application"
	formdomain "github.com/alrazihi/civora/internal/forms/domain"
	forminfra "github.com/alrazihi/civora/internal/forms/infrastructure/postgres"
	revapp "github.com/alrazihi/civora/internal/review_queue/application"
	revdomain "github.com/alrazihi/civora/internal/review_queue/domain"
	revpostgres "github.com/alrazihi/civora/internal/review_queue/infrastructure/postgres"
	rulesapp "github.com/alrazihi/civora/internal/rules/application"
	rulesdomain "github.com/alrazihi/civora/internal/rules/domain"
	rulespostgres "github.com/alrazihi/civora/internal/rules/infrastructure/postgres"
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	workflowinfra "github.com/alrazihi/civora/internal/workflow/infrastructure/postgres"
	assignmentapp "github.com/alrazihi/civora/internal/workflow_form_assignment/application"
	assignmentinfra "github.com/alrazihi/civora/internal/workflow_form_assignment/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type simpleCaseFinder struct {
	caseRepo *caseinfra.PostgresCaseRepository
}

func (f *simpleCaseFinder) FindByID(ctx context.Context, orgID, caseID uuid.UUID) (*casedomain.Case, error) {
	return f.caseRepo.FindByID(ctx, orgID, caseID)
}

type simpleUserChecker struct{}

func (s *simpleUserChecker) BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	return true, nil
}

func TestPlatformConfigurability_TwoDistinctProcesses(t *testing.T) {
	db := helpers.TestDB(t)
	helpers.ResetDB(t, db)
	ctx := context.Background()

	// ------------------------------------------------------------------
	// Bootstrap: two orgs, users
	// ------------------------------------------------------------------
	orgA := helpers.SeedOrg(db)
	orgB := helpers.SeedOrg(db)

	adminA := helpers.SeedUser(db, orgA)
	adminB := helpers.SeedUser(db, orgB)

	// Repos
	caseRepo := caseinfra.NewPostgresCaseRepository(db)
	workflowDefRepo := workflowinfra.NewPostgresWorkflowDefinitionRepository(db)
	workflowStateRepo := workflowinfra.NewPostgresWorkflowStateRepository(db)
	workflowTransRepo := workflowinfra.NewPostgresWorkflowTransitionRepository(db)
	workflowInstRepo := workflowinfra.NewPostgresWorkflowInstanceRepository(db)
	workflowHistRepo := workflowinfra.NewPostgresWorkflowTransitionHistoryRepository(db)
	formRepo := forminfra.NewPostgresFormRepository(db)
	formVerRepo := forminfra.NewPostgresFormVersionRepository(db)
	formFieldRepo := forminfra.NewPostgresFormFieldRepository(db)
	ruleSetRepo := rulespostgres.NewPostgresRuleSetRepository(db)
	evalRepo := rulespostgres.NewPostgresEvaluationRepository(db)
	evidenceRepo := evidencepostgres.NewPostgresEvidenceRepository(db)
	evidenceStorage, err := evidencestorage.NewLocalStorageProvider(t.TempDir())
	require.NoError(t, err)
	reviewRepo := revpostgres.NewPostgresReviewQueueRepository(db)
	decisionRepo := decpostgres.NewPostgresDecisionRepository(db)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db)
	assignmentRepo := assignmentinfra.NewPostgresWorkflowStateFormAssignmentRepository(db)
	submissionRepo := submissioninfra.NewPostgresFormSubmissionRepository(db)

	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	workflowSvc := workflowapp.NewWorkflowService(workflowDefRepo, workflowStateRepo, workflowTransRepo, workflowInstRepo, workflowHistRepo, auditService)
	caseSvc := caseapp.NewCaseService(caseRepo, nil, nil, auditService, auditRepo, workflowSvc)
	formSvc := formapp.NewFormService(formRepo, formVerRepo, formFieldRepo, auditService)
	assignmentSvc := assignmentapp.NewWorkflowStateFormAssignmentService(workflowDefRepo, workflowStateRepo, formRepo, formVerRepo, assignmentRepo, auditService)
	caseSvc.SetFormRepos(nil, workflowInstRepo, formRepo, formVerRepo, formFieldRepo, assignmentRepo, submissionRepo)

	caseFinder := &simpleCaseFinder{caseRepo: caseRepo}

	ruleSvc := rulesapp.NewRuleSetService(ruleSetRepo, evalRepo, nil, nil, auditService)
	evidenceSvc := evidenceapp.NewEvidenceService(evidenceRepo, caseFinder, nil, auditService, evidenceStorage)
	decisionSvc := decapp.NewDecisionService(decisionRepo, caseFinder, nil, auditService, workflowSvc)
	reviewSvc := revapp.NewReviewQueueService(reviewRepo, caseFinder, &simpleUserChecker{}, auditService, workflowSvc, decisionSvc)

	_ = evidenceSvc

	// ==================================================================
	// PROCESS A: Emergency Assistance
	// ==================================================================
	t.Run("Process A Emergency Assistance", func(t *testing.T) {
		now := time.Now().UTC()

		// Workflow: Intake -> Assessment -> Review -> Approved/Rejected
		wfA := &workflowdomain.WorkflowDefinition{
			ID:           uuid.New(),
			TenantID:     orgA,
			Key:          "emergency_assistance",
			Name:         "Emergency Assistance",
			Description:  "Emergency assistance request workflow",
			Version:      1,
			Status:       workflowdomain.WorkflowStatusDraft,
			InitialState: "INTAKE",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		err := workflowDefRepo.Save(ctx, wfA)
		require.NoError(t, err)

		statesA := []workflowdomain.WorkflowState{
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "INTAKE", Name: "Intake", Terminal: false, DisplayOrder: 0, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "ASSESSMENT", Name: "Assessment", Terminal: false, DisplayOrder: 1, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "REVIEW", Name: "Review", Terminal: false, DisplayOrder: 2, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "APPROVED", Name: "Approved", Terminal: false, DisplayOrder: 3, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "REJECTED", Name: "Rejected", Terminal: true, DisplayOrder: 4, CreatedAt: now},
		}
		err = workflowStateRepo.SaveBatch(ctx, statesA)
		require.NoError(t, err)

		transitionsA := []workflowdomain.WorkflowTransition{
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "assess", Name: "Assess", FromState: "INTAKE", ToState: "ASSESSMENT", Active: true, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "review", Name: "Review", FromState: "ASSESSMENT", ToState: "REVIEW", Active: true, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "approve", Name: "Approve", FromState: "REVIEW", ToState: "APPROVED", Active: true, DecisionType: string(decdomain.DecisionTypeApproved), CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "reject", Name: "Reject", FromState: "REVIEW", ToState: "REJECTED", Active: true, DecisionType: string(decdomain.DecisionTypeRejected), CreatedAt: now},
		}
		err = workflowTransRepo.SaveBatch(ctx, transitionsA)
		require.NoError(t, err)

		// Activate workflow
		require.NoError(t, workflowDefRepo.UpdateStatus(ctx, orgA, wfA.ID, workflowdomain.WorkflowStatusActive, wfA.Version))

		// Form: Emergency Assessment
		formA, err := formSvc.CreateForm(ctx, formapp.CreateFormParams{OrganizationID: orgA, Key: "emergency_assessment", Name: "Emergency Assessment", Description: "Emergency needs assessment", CreatedByID: adminA})
		require.NoError(t, err)

		verA, err := formSvc.CreateVersion(ctx, formapp.CreateVersionParams{OrganizationID: orgA, FormID: formA.ID, ActorID: adminA})
		require.NoError(t, err)

		// Add fields
		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgA, FormID: formA.ID, VersionID: verA.ID, Key: "household_size", Label: "Household Size", Type: formdomain.FieldTypeNumber, Required: true, Validation: map[string]any{"min_value": 1}, Order: 1, ActorID: adminA})
		require.NoError(t, err)

		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgA, FormID: formA.ID, VersionID: verA.ID, Key: "monthly_income", Label: "Monthly Income", Type: formdomain.FieldTypeDecimal, Required: true, Validation: map[string]any{"min_value": 0}, Order: 2, ActorID: adminA})
		require.NoError(t, err)

		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgA, FormID: formA.ID, VersionID: verA.ID, Key: "housing_situation", Label: "Housing Situation", Type: formdomain.FieldTypeSelect, Required: true, Options: []formdomain.FormOption{{Label: "Rent", Value: "rent"}, {Label: "Own", Value: "own"}, {Label: "Shelter", Value: "shelter"}, {Label: "None", Value: "none"}}, Order: 3, ActorID: adminA})
		require.NoError(t, err)

		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgA, FormID: formA.ID, VersionID: verA.ID, Key: "urgency", Label: "Urgency", Type: formdomain.FieldTypeSelect, Required: true, Options: []formdomain.FormOption{{Label: "Low", Value: "low"}, {Label: "Medium", Value: "medium"}, {Label: "High", Value: "high"}, {Label: "Critical", Value: "critical"}}, Order: 4, ActorID: adminA})
		require.NoError(t, err)

		// Publish form
		_, err = formSvc.PublishVersion(ctx, formapp.PublishVersionParams{OrganizationID: orgA, VersionID: verA.ID, ActorID: adminA})
		require.NoError(t, err)

		// Assign form to workflow state
		_, err = assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
			TenantID:             orgA,
			WorkflowDefinitionID: wfA.ID,
			WorkflowStateKey:     "INTAKE",
			FormID:               formA.ID,
			FormVersionID:        verA.ID,
			Required:             true,
			DisplayOrder:         0,
			Active:               true,
			CreatedBy:            adminA,
		})
		require.NoError(t, err)

		// Rule Set: Emergency rules
		ruleSetA, err := ruleSvc.CreateRuleSet(ctx, rulesapp.CreateRuleSetParams{
			OrganizationID: orgA,
			Key:            "emergency_rules",
			Name:           "Emergency Rules",
			Description:    "Rules for emergency assistance",
			DefaultOutcome: rulesdomain.OutcomeEligible,
			Rules: []rulesdomain.Rule{
				{ID: uuid.New(), Priority: 1, Outcome: rulesdomain.OutcomeEligible, Conditions: rulesdomain.Condition{Field: "form.household_size", Operator: rulesdomain.OpGreaterThan, Value: 0}, Active: true, CreatedAt: now},
				{ID: uuid.New(), Priority: 2, Outcome: rulesdomain.OutcomeRequiresReview, Conditions: rulesdomain.Condition{Field: "form.urgency", Operator: rulesdomain.OpEqual, Value: "critical"}, Active: true, CreatedAt: now},
			},
			Triggers: []string{"case.created"},
			ActorID:  adminA,
		})
		require.NoError(t, err)

		// Publish rule set
		_, err = ruleSvc.PublishRuleSet(ctx, orgA, ruleSetA.ID, adminA)
		require.NoError(t, err)

		// Create case with workflow
		caseA, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
			OrganizationID: orgA,
			CreatedByID:    adminA,
			Title:          "Emergency Case A",
			Description:    "Family needs assistance",
			ServiceType:    "Emergency",
			Priority:       "High",
			WorkflowID:     &wfA.ID,
		})
		require.NoError(t, err)

		// Submit form
		_, err = caseSvc.SubmitForm(ctx, orgA, caseA.ID, adminA, verA.ID, map[string]interface{}{
			"household_size":    4,
			"monthly_income":    2500.50,
			"housing_situation": "rent",
			"urgency":           "high",
		})
		require.NoError(t, err)

		// Evaluate rules
		evalA, err := ruleSvc.EvaluateRuleSet(ctx, rulesapp.EvaluateRuleSetParams{
			OrganizationID: orgA,
			ID:             ruleSetA.ID,
			Trigger:        rulesdomain.TriggerManual,
			ActorID:        &adminA,
			EvaluatedAt:    &now,
		})
		require.NoError(t, err)
		assert.NotNil(t, evalA)
		assert.Equal(t, ruleSetA.ID, evalA.RuleSetID)
		assert.NotEmpty(t, evalA.Trace)

		// Advance workflow through states
		instA, _ := workflowSvc.GetInstanceByCaseID(ctx, orgA, caseA.ID)
		_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{TenantID: orgA, InstanceID: instA.ID, TransitionKey: "assess", ActorID: adminA, ActorRole: "admin", Reason: "Move to assessment"})
		require.NoError(t, err)
		instA, _ = workflowSvc.GetInstanceByCaseID(ctx, orgA, caseA.ID)
		_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{TenantID: orgA, InstanceID: instA.ID, TransitionKey: "review", ActorID: adminA, ActorRole: "admin", Reason: "Move to review"})
		require.NoError(t, err)

		// Enter human review
		instA, _ = workflowSvc.GetInstanceByCaseID(ctx, orgA, caseA.ID)
		reviewEntryA := revdomain.NewReviewQueueEntry(orgA, caseA.ID, instA.ID, "REVIEW", "Normal", []uuid.UUID{evalA.ID}, nil, nil)
		err = reviewRepo.Save(ctx, reviewEntryA)
		require.NoError(t, err)

		// Claim, start, complete review
		reviewEntryA, err = reviewSvc.ClaimReview(ctx, revapp.ClaimReviewParams{OrganizationID: orgA, ReviewID: reviewEntryA.ID, ReviewerID: adminA})
		require.NoError(t, err)

		reviewEntryA, err = reviewSvc.StartReview(ctx, revapp.StartReviewParams{OrganizationID: orgA, ReviewID: reviewEntryA.ID, ReviewerID: adminA})
		require.NoError(t, err)

		reviewEntryA, err = reviewSvc.CompleteReview(ctx, revapp.CompleteReviewParams{
			OrganizationID: orgA,
			ReviewID:       reviewEntryA.ID,
			ReviewerID:     adminA,
			Decision:       string(decdomain.DecisionTypeApproved),
			Reason:         "Meets emergency criteria",
		})
		require.NoError(t, err)
		assert.Equal(t, revdomain.ReviewStatusCompleted, reviewEntryA.Status)

		// Verify decision
		decisionA, err := decisionSvc.GetDecisionByServiceRequest(ctx, orgA, caseA.ID)
		require.NoError(t, err)
		assert.NotNil(t, decisionA)
		assert.Equal(t, decdomain.DecisionTypeApproved, decisionA.Decision)
		assert.Equal(t, "Meets emergency criteria", decisionA.Reason)
		assert.Equal(t, adminA, decisionA.DecisionMaker)
		assert.Equal(t, reviewEntryA.ID, *decisionA.ReviewQueueEntryID)

		// Verify workflow state
		instA, err = workflowSvc.GetInstanceByCaseID(ctx, orgA, caseA.ID)
		require.NoError(t, err)
		assert.Equal(t, "APPROVED", instA.CurrentState)

		// Verify audit
		auditEvents, err := auditRepo.FindByOrganization(ctx, orgA, 100, 0)
		require.NoError(t, err)
		assert.True(t, len(auditEvents) > 0)
	})

	// ==================================================================
	// PROCESS B: Education Assistance (completely different)
	// ==================================================================
	t.Run("Process B Education Assistance", func(t *testing.T) {
		now := time.Now().UTC()

		// Workflow: Applied -> DocumentReview -> Interview -> Awarded/Rejected -> Enrolled
		wfB := &workflowdomain.WorkflowDefinition{
			ID:           uuid.New(),
			TenantID:     orgB,
			Key:          "education_assistance",
			Name:         "Education Assistance",
			Description:  "Education grant and scholarship workflow",
			Version:      1,
			Status:       workflowdomain.WorkflowStatusDraft,
			InitialState: "APPLIED",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		err := workflowDefRepo.Save(ctx, wfB)
		require.NoError(t, err)

		statesB := []workflowdomain.WorkflowState{
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "APPLIED", Name: "Applied", Terminal: false, DisplayOrder: 0, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "DOCUMENT_REVIEW", Name: "Document Review", Terminal: false, DisplayOrder: 1, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "INTERVIEW", Name: "Interview", Terminal: false, DisplayOrder: 2, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "AWARDED", Name: "Awarded", Terminal: false, DisplayOrder: 3, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "REJECTED", Name: "Rejected", Terminal: true, DisplayOrder: 4, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "ENROLLED", Name: "Enrolled", Terminal: true, DisplayOrder: 5, CreatedAt: now},
		}
		err = workflowStateRepo.SaveBatch(ctx, statesB)
		require.NoError(t, err)

		transitionsB := []workflowdomain.WorkflowTransition{
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "start_review", Name: "Start Review", FromState: "APPLIED", ToState: "DOCUMENT_REVIEW", Active: true, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "schedule_interview", Name: "Schedule Interview", FromState: "DOCUMENT_REVIEW", ToState: "INTERVIEW", Active: true, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "award", Name: "Award", FromState: "INTERVIEW", ToState: "AWARDED", Active: true, DecisionType: string(decdomain.DecisionTypeApproved), CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "reject", Name: "Reject", FromState: "INTERVIEW", ToState: "REJECTED", Active: true, DecisionType: string(decdomain.DecisionTypeRejected), CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "enroll", Name: "Enroll", FromState: "AWARDED", ToState: "ENROLLED", Active: true, CreatedAt: now},
		}
		err = workflowTransRepo.SaveBatch(ctx, transitionsB)
		require.NoError(t, err)

		// Activate workflow
		require.NoError(t, workflowDefRepo.UpdateStatus(ctx, orgB, wfB.ID, workflowdomain.WorkflowStatusActive, wfB.Version))

		// Form: Education Application (different fields from Emergency)
		formB, err := formSvc.CreateForm(ctx, formapp.CreateFormParams{OrganizationID: orgB, Key: "education_application", Name: "Education Application", Description: "Education grant application", CreatedByID: adminB})
		require.NoError(t, err)

		verB, err := formSvc.CreateVersion(ctx, formapp.CreateVersionParams{OrganizationID: orgB, FormID: formB.ID, ActorID: adminB})
		require.NoError(t, err)

		// Add fields: student_name, gpa, major, previous_degree, essay
		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgB, FormID: formB.ID, VersionID: verB.ID, Key: "student_name", Label: "Student Name", Type: formdomain.FieldTypeText, Required: true, Order: 1, ActorID: adminB})
		require.NoError(t, err)

		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgB, FormID: formB.ID, VersionID: verB.ID, Key: "gpa", Label: "GPA", Type: formdomain.FieldTypeDecimal, Required: true, Validation: map[string]any{"min_value": 0, "max_value": 4.0}, Order: 2, ActorID: adminB})
		require.NoError(t, err)

		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgB, FormID: formB.ID, VersionID: verB.ID, Key: "major", Label: "Major", Type: formdomain.FieldTypeSelect, Required: true, Options: []formdomain.FormOption{{Label: "Computer Science", Value: "cs"}, {Label: "Mathematics", Value: "math"}, {Label: "Physics", Value: "physics"}, {Label: "Literature", Value: "lit"}}, Order: 3, ActorID: adminB})
		require.NoError(t, err)

		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgB, FormID: formB.ID, VersionID: verB.ID, Key: "previous_degree", Label: "Previous Degree", Type: formdomain.FieldTypeSelect, Options: []formdomain.FormOption{{Label: "None", Value: "none"}, {Label: "High School", Value: "hs"}, {Label: "Bachelor", Value: "bs"}, {Label: "Master", Value: "ms"}}, Order: 4, ActorID: adminB})
		require.NoError(t, err)

		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgB, FormID: formB.ID, VersionID: verB.ID, Key: "essay", Label: "Personal Essay", Type: formdomain.FieldTypeTextarea, Required: true, Validation: map[string]any{"min_length": 100}, Order: 5, ActorID: adminB})
		require.NoError(t, err)

		// Publish form
		_, err = formSvc.PublishVersion(ctx, formapp.PublishVersionParams{OrganizationID: orgB, VersionID: verB.ID, ActorID: adminB})
		require.NoError(t, err)

		// Assign form to workflow state
		_, err = assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
			TenantID:             orgB,
			WorkflowDefinitionID: wfB.ID,
			WorkflowStateKey:     "APPLIED",
			FormID:               formB.ID,
			FormVersionID:        verB.ID,
			Required:             true,
			DisplayOrder:         0,
			Active:               true,
			CreatedBy:            adminB,
		})
		require.NoError(t, err)

		// Rule Set: Education rules (different logic from Emergency)
		ruleSetB, err := ruleSvc.CreateRuleSet(ctx, rulesapp.CreateRuleSetParams{
			OrganizationID: orgB,
			Key:            "education_rules",
			Name:           "Education Rules",
			Description:    "Rules for education assistance",
			DefaultOutcome: rulesdomain.OutcomeEligible,
			Rules: []rulesdomain.Rule{
				{ID: uuid.New(), Priority: 1, Outcome: rulesdomain.OutcomeEligible, Conditions: rulesdomain.Condition{Field: "form.gpa", Operator: rulesdomain.OpGreaterThanEqual, Value: 3.0}, Active: true, CreatedAt: now},
				{ID: uuid.New(), Priority: 2, Outcome: rulesdomain.OutcomeRequiresReview, Conditions: rulesdomain.Condition{Field: "form.previous_degree", Operator: rulesdomain.OpEqual, Value: "none"}, Active: true, CreatedAt: now},
			},
			Triggers: []string{"case.created"},
			ActorID:  adminB,
		})
		require.NoError(t, err)

		// Publish rule set
		_, err = ruleSvc.PublishRuleSet(ctx, orgB, ruleSetB.ID, adminB)
		require.NoError(t, err)

		// Create case
		caseB, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
			OrganizationID: orgB,
			CreatedByID:    adminB,
			Title:          "Education Case B",
			Description:    "Scholarship application",
			ServiceType:    "Education",
			Priority:       "Normal",
			WorkflowID:     &wfB.ID,
		})
		require.NoError(t, err)

		// Submit form
		_, err = caseSvc.SubmitForm(ctx, orgB, caseB.ID, adminB, verB.ID, map[string]interface{}{
			"student_name":    "Alice Student",
			"gpa":             3.8,
			"major":           "cs",
			"previous_degree": "hs",
			"essay":           "I am a dedicated student with a passion for computer science and a commitment to academic excellence. This essay demonstrates my qualifications for the scholarship program.",
		})
		require.NoError(t, err)

		// Evaluate rules
		evalB, err := ruleSvc.EvaluateRuleSet(ctx, rulesapp.EvaluateRuleSetParams{
			OrganizationID: orgB,
			ID:             ruleSetB.ID,
			Trigger:        rulesdomain.TriggerManual,
			ActorID:        &adminB,
			EvaluatedAt:    &now,
		})
		require.NoError(t, err)
		assert.NotNil(t, evalB)
		assert.Equal(t, ruleSetB.ID, evalB.RuleSetID)
		assert.NotEmpty(t, evalB.Trace)

		// Advance workflow through states
		instB, _ := workflowSvc.GetInstanceByCaseID(ctx, orgB, caseB.ID)
		_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{TenantID: orgB, InstanceID: instB.ID, TransitionKey: "start_review", ActorID: adminB, ActorRole: "admin", Reason: "Move to document review"})
		require.NoError(t, err)
		instB, _ = workflowSvc.GetInstanceByCaseID(ctx, orgB, caseB.ID)
		_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{TenantID: orgB, InstanceID: instB.ID, TransitionKey: "schedule_interview", ActorID: adminB, ActorRole: "admin", Reason: "Move to interview"})
		require.NoError(t, err)

		// Enter human review
		instB, _ = workflowSvc.GetInstanceByCaseID(ctx, orgB, caseB.ID)
		reviewEntryB := revdomain.NewReviewQueueEntry(orgB, caseB.ID, instB.ID, "INTERVIEW", "Normal", []uuid.UUID{evalB.ID}, nil, nil)
		err = reviewRepo.Save(ctx, reviewEntryB)
		require.NoError(t, err)

		// Claim, start, complete review
		reviewEntryB, err = reviewSvc.ClaimReview(ctx, revapp.ClaimReviewParams{OrganizationID: orgB, ReviewID: reviewEntryB.ID, ReviewerID: adminB})
		require.NoError(t, err)

		reviewEntryB, err = reviewSvc.StartReview(ctx, revapp.StartReviewParams{OrganizationID: orgB, ReviewID: reviewEntryB.ID, ReviewerID: adminB})
		require.NoError(t, err)

		reviewEntryB, err = reviewSvc.CompleteReview(ctx, revapp.CompleteReviewParams{
			OrganizationID: orgB,
			ReviewID:       reviewEntryB.ID,
			ReviewerID:     adminB,
			Decision:       string(decdomain.DecisionTypeApproved),
			Reason:         "Strong candidate, proceed to enrollment",
		})
		require.NoError(t, err)
		assert.Equal(t, revdomain.ReviewStatusCompleted, reviewEntryB.Status)

		// Verify decision
		decisionB, err := decisionSvc.GetDecisionByServiceRequest(ctx, orgB, caseB.ID)
		require.NoError(t, err)
		assert.NotNil(t, decisionB)
		assert.Equal(t, decdomain.DecisionTypeApproved, decisionB.Decision)
		assert.Equal(t, "Strong candidate, proceed to enrollment", decisionB.Reason)
		assert.Equal(t, adminB, decisionB.DecisionMaker)
		assert.Equal(t, reviewEntryB.ID, *decisionB.ReviewQueueEntryID)

		// Verify workflow state
		instB, err = workflowSvc.GetInstanceByCaseID(ctx, orgB, caseB.ID)
		require.NoError(t, err)
		assert.Equal(t, "AWARDED", instB.CurrentState)

		// Verify audit
		auditEventsB, err := auditRepo.FindByOrganization(ctx, orgB, 100, 0)
		require.NoError(t, err)
		assert.True(t, len(auditEventsB) > 0)
	})

	// ==================================================================
	// TENANT ISOLATION
	// ==================================================================
	t.Run("Tenant Isolation", func(t *testing.T) {
		casesA, _ := caseRepo.FindByOrganization(ctx, orgA, 100, 0)
		for _, c := range casesA {
			assert.Equal(t, orgA, c.OrganizationID)
		}

		casesB, _ := caseRepo.FindByOrganization(ctx, orgB, 100, 0)
		for _, c := range casesB {
			assert.Equal(t, orgB, c.OrganizationID)
		}

		formsA, _, _ := formRepo.List(ctx, orgA, 100, 0)
		for _, f := range formsA {
			assert.Equal(t, orgA, f.OrganizationID)
		}

		formsB, _, _ := formRepo.List(ctx, orgB, 100, 0)
		for _, f := range formsB {
			assert.Equal(t, orgB, f.OrganizationID)
		}

		auditA, _ := auditRepo.FindByOrganization(ctx, orgA, 100, 0)
		for _, e := range auditA {
			assert.Equal(t, orgA, e.OrganizationID)
		}

		auditB, _ := auditRepo.FindByOrganization(ctx, orgB, 100, 0)
		for _, e := range auditB {
			assert.Equal(t, orgB, e.OrganizationID)
		}
	})
}
