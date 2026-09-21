package integration

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	aiapp "github.com/alrazihi/civora/internal/ai/application"
	ai_domain "github.com/alrazihi/civora/internal/ai/domain"
	aiadapter "github.com/alrazihi/civora/internal/ai/infrastructure/adapter"
	aiinfra "github.com/alrazihi/civora/internal/ai/infrastructure/postgres"
	aiprovider "github.com/alrazihi/civora/internal/ai/infrastructure/provider"
	aisanitizer "github.com/alrazihi/civora/internal/ai/infrastructure/sanitizer"
	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	caseapp "github.com/alrazihi/civora/internal/cases/application"
	casedomain "github.com/alrazihi/civora/internal/cases/domain"
	caseinfra "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	decapp "github.com/alrazihi/civora/internal/decisions/application"
	decdomain "github.com/alrazihi/civora/internal/decisions/domain"
	decpostgres "github.com/alrazihi/civora/internal/decisions/infrastructure/postgres"
	evidenceapp "github.com/alrazihi/civora/internal/evidence/application"
	evidencedomain "github.com/alrazihi/civora/internal/evidence/domain"
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

// mockAIProvider returns deterministic observations for testing.
type mockAIProvider struct {
	evidenceObservations []aiapp.ObservationResult
	caseObservations     []aiapp.ObservationResult
}

func (m *mockAIProvider) GenerateObservations(ctx context.Context, req aiapp.ProviderRequest) ([]aiapp.ObservationResult, error) {
	if len(req.Documents) > 0 {
		return m.evidenceObservations, nil
	}
	return m.caseObservations, nil
}

func (m *mockAIProvider) ProviderInfo() ai_domain.ModelInfo {
	return ai_domain.ModelInfo{Name: "mock-ai", Version: "1.0.0", Provider: "test"}
}

func (m *mockAIProvider) GenerateChatCompletion(ctx context.Context, req aiapp.ChatRequest) (*aiapp.ChatResponse, error) {
	return nil, errors.New("not implemented in mock")
}

// newAIService creates a real AI service wired with a mock provider and real postgres repos.
func newAIService(db *sql.DB, auditor auditdomain.EventRecorder, provider aiapp.AIProvider, evidenceSvc *evidenceapp.EvidenceService) *aiapp.AIService {
	aiObsRepo := aiinfra.NewPostgresObservationRepository(db)
	evidenceRepo := evidencepostgres.NewPostgresEvidenceRepository(db)
	formSubmissionRepo := submissioninfra.NewPostgresFormSubmissionRepository(db)

	svc := aiapp.NewAIService(
		aiObsRepo,
		evidenceRepo,
		formSubmissionRepo,
		&fixedUserChecker{},
		auditor,
		provider,
	).
		WithTransaction(db).
		WithDocumentContent(aiadapter.NewDocumentContentProvider(evidenceSvc)).
		WithPIISanitizer(aisanitizer.NewRedactingSanitizer()).
		WithVerifiedFactRepo(aiinfra.NewPostgresVerifiedFactRepository(db))

	return svc
}

type fixedUserChecker struct{}

func (f *fixedUserChecker) BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	return true, nil
}

func TestAIPlatformProof_FullPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

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
	aiObsRepo := aiinfra.NewPostgresObservationRepository(db)
	aiFactRepo := aiinfra.NewPostgresVerifiedFactRepository(db)

	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	workflowSvc := workflowapp.NewWorkflowService(workflowDefRepo, workflowStateRepo, workflowTransRepo, workflowInstRepo, workflowHistRepo, auditService)
	caseSvc := caseapp.NewCaseService(caseRepo, nil, nil, auditService, auditRepo, workflowSvc)
	formSvc := formapp.NewFormService(formRepo, formVerRepo, formFieldRepo, auditService)
	assignmentSvc := assignmentapp.NewWorkflowStateFormAssignmentService(workflowDefRepo, workflowStateRepo, formRepo, formVerRepo, assignmentRepo, auditService)
	caseSvc.SetFormRepos(nil, workflowInstRepo, formRepo, formVerRepo, formFieldRepo, assignmentRepo, submissionRepo)

	caseFinder := &simpleCaseFinder{caseRepo: caseRepo}

	ruleSvc := rulesapp.NewRuleSetService(ruleSetRepo, evalRepo, nil, nil, auditService)
	evidenceSvc := evidenceapp.NewEvidenceService(evidenceRepo, caseFinder, &simpleUserChecker{}, auditService, evidenceStorage)
	decisionSvc := decapp.NewDecisionService(decisionRepo, caseFinder, nil, auditService, workflowSvc)
	reviewSvc := revapp.NewReviewQueueService(reviewRepo, caseFinder, &simpleUserChecker{}, auditService, workflowSvc, decisionSvc)

	// ==================================================================
	// PROCESS A: Emergency Assistance (Organization A)
	// ==================================================================
	t.Run("Process A Emergency Assistance - Full AI Pipeline", func(t *testing.T) {
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
		require.NoError(t, workflowDefRepo.Save(ctx, wfA))

		statesA := []workflowdomain.WorkflowState{
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "INTAKE", Name: "Intake", Terminal: false, DisplayOrder: 0, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "ASSESSMENT", Name: "Assessment", Terminal: false, DisplayOrder: 1, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "REVIEW", Name: "Review", Terminal: false, DisplayOrder: 2, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "APPROVED", Name: "Approved", Terminal: false, DisplayOrder: 3, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "REJECTED", Name: "Rejected", Terminal: true, DisplayOrder: 4, CreatedAt: now},
		}
		require.NoError(t, workflowStateRepo.SaveBatch(ctx, statesA))

		transitionsA := []workflowdomain.WorkflowTransition{
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "assess", Name: "Assess", FromState: "INTAKE", ToState: "ASSESSMENT", Active: true, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "review", Name: "Review", FromState: "ASSESSMENT", ToState: "REVIEW", Active: true, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "approve", Name: "Approve", FromState: "REVIEW", ToState: "APPROVED", Active: true, DecisionType: string(decdomain.DecisionTypeApproved), CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfA.ID, TenantID: orgA, Key: "reject", Name: "Reject", FromState: "REVIEW", ToState: "REJECTED", Active: true, DecisionType: string(decdomain.DecisionTypeRejected), CreatedAt: now},
		}
		require.NoError(t, workflowTransRepo.SaveBatch(ctx, transitionsA))
		require.NoError(t, workflowDefRepo.UpdateStatus(ctx, orgA, wfA.ID, workflowdomain.WorkflowStatusActive, wfA.Version))

		// Form: Emergency Assessment
		formA, err := formSvc.CreateForm(ctx, formapp.CreateFormParams{OrganizationID: orgA, Key: "emergency_assessment", Name: "Emergency Assessment", Description: "Emergency needs assessment", CreatedByID: adminA})
		require.NoError(t, err)

		verA, err := formSvc.CreateVersion(ctx, formapp.CreateVersionParams{OrganizationID: orgA, FormID: formA.ID, ActorID: adminA})
		require.NoError(t, err)

		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgA, FormID: formA.ID, VersionID: verA.ID, Key: "household_size", Label: "Household Size", Type: formdomain.FieldTypeNumber, Required: true, Validation: map[string]any{"min_value": 1}, Order: 1, ActorID: adminA})
		require.NoError(t, err)
		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgA, FormID: formA.ID, VersionID: verA.ID, Key: "monthly_income", Label: "Monthly Income", Type: formdomain.FieldTypeDecimal, Required: true, Validation: map[string]any{"min_value": 0}, Order: 2, ActorID: adminA})
		require.NoError(t, err)
		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgA, FormID: formA.ID, VersionID: verA.ID, Key: "housing_situation", Label: "Housing Situation", Type: formdomain.FieldTypeSelect, Required: true, Options: []formdomain.FormOption{{Label: "Rent", Value: "rent"}, {Label: "Own", Value: "own"}, {Label: "Shelter", Value: "shelter"}, {Label: "None", Value: "none"}}, Order: 3, ActorID: adminA})
		require.NoError(t, err)
		_, err = formSvc.AddField(ctx, formapp.AddFieldParams{OrganizationID: orgA, FormID: formA.ID, VersionID: verA.ID, Key: "urgency", Label: "Urgency", Type: formdomain.FieldTypeSelect, Required: true, Options: []formdomain.FormOption{{Label: "Low", Value: "low"}, {Label: "Medium", Value: "medium"}, {Label: "High", Value: "high"}, {Label: "Critical", Value: "critical"}}, Order: 4, ActorID: adminA})
		require.NoError(t, err)
		_, err = formSvc.PublishVersion(ctx, formapp.PublishVersionParams{OrganizationID: orgA, VersionID: verA.ID, ActorID: adminA})
		require.NoError(t, err)

		_, err = assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
			TenantID: orgA, WorkflowDefinitionID: wfA.ID, WorkflowStateKey: "INTAKE",
			FormID: formA.ID, FormVersionID: verA.ID, Required: true, DisplayOrder: 0, Active: true, CreatedBy: adminA,
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
		_, err = ruleSvc.PublishRuleSet(ctx, orgA, ruleSetA.ID, adminA)
		require.NoError(t, err)

		// Create case
		caseA, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
			OrganizationID: orgA, CreatedByID: adminA,
			Title: "Emergency Case A", Description: "Family needs assistance",
			ServiceType: "Emergency", Priority: "High",
			WorkflowID: &wfA.ID,
		})
		require.NoError(t, err)

		// Submit form
		_, err = caseSvc.SubmitForm(ctx, orgA, caseA.ID, adminA, verA.ID, map[string]interface{}{
			"household_size": 4, "monthly_income": 2500.50, "housing_situation": "rent", "urgency": "high",
		})
		require.NoError(t, err)

		// Add evidence with documents for AI processing
		evidenceA1, err := evidenceSvc.AddEvidence(ctx, evidenceapp.AddEvidenceParams{
			OrganizationID: orgA, ServiceRequestID: caseA.ID,
			Type: evidencedomain.EvidenceTypeIdentityDocument, Description: "Government ID",
			StorageReference: "civora://test/emergency/id_a", UploadedBy: adminA,
			Source: evidencedomain.EvidenceSourceManual,
		})
		require.NoError(t, err)

		_, err = evidenceSvc.UploadDocument(ctx, evidenceapp.UploadDocumentParams{
			OrganizationID: orgA, EvidenceID: evidenceA1.ID,
			FileName: "id_document.txt", ContentType: "text/plain",
			Size: int64(len("Emergency ID: verified identity document for household of 4")), Reader: strings.NewReader("Emergency ID: verified identity document for household of 4"),
			UploadedBy: adminA,
		})
		require.NoError(t, err)

		evidenceA2, err := evidenceSvc.AddEvidence(ctx, evidenceapp.AddEvidenceParams{
			OrganizationID: orgA, ServiceRequestID: caseA.ID,
			Type: evidencedomain.EvidenceTypeProofOfResidence, Description: "Proof of residence",
			StorageReference: "civora://test/emergency/residence_a", UploadedBy: adminA,
			Source: evidencedomain.EvidenceSourceManual,
		})
		require.NoError(t, err)

		_, err = evidenceSvc.UploadDocument(ctx, evidenceapp.UploadDocumentParams{
			OrganizationID: orgA, EvidenceID: evidenceA2.ID,
			FileName: "residence_proof.txt", ContentType: "text/plain",
			Size: int64(len("Proof of residence: family currently renting at 123 Main St, urgent situation")), Reader: strings.NewReader("Proof of residence: family currently renting at 123 Main St, urgent situation"),
			UploadedBy: adminA,
		})
		require.NoError(t, err)

		// AI: Generate evidence-level observations
		aiProviderA := &mockAIProvider{
			evidenceObservations: []aiapp.ObservationResult{
				{Type: ai_domain.ObservationTypeSummary, Content: map[string]any{"statement": "ID document confirms household of 4, verified identity"}, Confidence: floatPtr(0.95), Model: ai_domain.ModelInfo{Name: "mock-ai", Version: "1.0.0", Provider: "test"}},
				{Type: ai_domain.ObservationTypeEntityExtraction, Content: map[string]any{"entities": []string{"household_size:4"}}, Confidence: floatPtr(0.9), Model: ai_domain.ModelInfo{Name: "mock-ai", Version: "1.0.0", Provider: "test"}},
			},
			caseObservations: []aiapp.ObservationResult{
				{Type: ai_domain.ObservationTypeSummary, Content: map[string]any{"statement": "Emergency case: family of 4, high urgency, renting"}, Confidence: floatPtr(0.92), Model: ai_domain.ModelInfo{Name: "mock-ai", Version: "1.0.0", Provider: "test"}},
			},
		}

		aiSvcA := newAIService(db, auditService, aiProviderA, evidenceSvc)

		// Generate evidence-level observations for first evidence
		evObsResult, err := aiSvcA.GenerateObservations(ctx, aiapp.GenerateObservationsParams{
			OrganizationID: orgA, EvidenceID: evidenceA1.ID, ActorID: adminA,
			Types: []ai_domain.ObservationType{ai_domain.ObservationTypeSummary, ai_domain.ObservationTypeEntityExtraction},
		})
		require.NoError(t, err)
		require.Len(t, evObsResult.Observations, 2)
		assert.Equal(t, ai_domain.ObservationStatusOpen, evObsResult.Observations[0].Status)
		assert.Equal(t, ai_domain.ObservationSourceAIModel, evObsResult.Observations[0].Source)

		// Generate case-level observations
		caseObsResult, err := aiSvcA.GenerateCaseObservations(ctx, aiapp.GenerateCaseObservationsParams{
			OrganizationID: orgA, CaseID: caseA.ID, ActorID: adminA,
			Types: []ai_domain.ObservationType{ai_domain.ObservationTypeSummary},
		})
		require.NoError(t, err)
		require.Len(t, caseObsResult.Observations, 1)
		assert.Equal(t, ai_domain.ObservationStatusOpen, caseObsResult.Observations[0].Status)

		// Human verification: accept evidence observation
		acceptedObs, err := aiSvcA.ReviewObservation(ctx, aiapp.ReviewObservationParams{
			OrganizationID: orgA, ObservationID: evObsResult.Observations[0].ID,
			ReviewerID: adminA, Action: ai_domain.ObservationStatusAccepted, Notes: "Verified summary",
		})
		require.NoError(t, err)
		assert.Equal(t, ai_domain.ObservationStatusAccepted, acceptedObs.Status)

		// Create verified fact from accepted observation
		fact, err := aiSvcA.CreateVerifiedFact(ctx, aiapp.CreateVerifiedFactParams{
			OrganizationID: orgA, ObservationID: evObsResult.Observations[0].ID,
			ReviewerID: adminA, ReviewAction: ai_domain.ObservationStatusAccepted, ReviewNotes: "Verified",
		})
		require.NoError(t, err)
		assert.NotNil(t, fact)
		assert.Equal(t, "HUMAN_ACCEPTED", fact.Source)

		// Evaluate rules
		evalA, err := ruleSvc.EvaluateRuleSet(ctx, rulesapp.EvaluateRuleSetParams{
			OrganizationID: orgA, ID: ruleSetA.ID, Trigger: rulesdomain.TriggerManual, ActorID: &adminA, EvaluatedAt: &now,
		})
		require.NoError(t, err)
		assert.NotNil(t, evalA)

		// Advance workflow
		instA, _ := workflowSvc.GetInstanceByCaseID(ctx, orgA, caseA.ID)
		_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{TenantID: orgA, InstanceID: instA.ID, TransitionKey: "assess", ActorID: adminA, ActorRole: "admin", Reason: "Move to assessment"})
		require.NoError(t, err)
		instA, _ = workflowSvc.GetInstanceByCaseID(ctx, orgA, caseA.ID)
		_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{TenantID: orgA, InstanceID: instA.ID, TransitionKey: "review", ActorID: adminA, ActorRole: "admin", Reason: "Move to review"})
		require.NoError(t, err)

		// Human review
		instA, _ = workflowSvc.GetInstanceByCaseID(ctx, orgA, caseA.ID)
		reviewEntryA := revdomain.NewReviewQueueEntry(orgA, caseA.ID, instA.ID, "REVIEW", casedomain.Priority("Normal"), []uuid.UUID{evalA.ID}, nil, nil)
		require.NoError(t, reviewRepo.Save(ctx, reviewEntryA))

		reviewEntryA, err = reviewSvc.ClaimReview(ctx, revapp.ClaimReviewParams{OrganizationID: orgA, ReviewID: reviewEntryA.ID, ReviewerID: adminA})
		require.NoError(t, err)
		reviewEntryA, err = reviewSvc.StartReview(ctx, revapp.StartReviewParams{OrganizationID: orgA, ReviewID: reviewEntryA.ID, ReviewerID: adminA})
		require.NoError(t, err)
		reviewEntryA, err = reviewSvc.CompleteReview(ctx, revapp.CompleteReviewParams{
			OrganizationID: orgA, ReviewID: reviewEntryA.ID, ReviewerID: adminA,
			Decision: string(decdomain.DecisionTypeApproved), Reason: "Meets emergency criteria",
		})
		require.NoError(t, err)
		assert.Equal(t, revdomain.ReviewStatusCompleted, reviewEntryA.Status)

		// Verify decision
		decisionA, err := decisionSvc.GetDecisionByServiceRequest(ctx, orgA, caseA.ID)
		require.NoError(t, err)
		assert.NotNil(t, decisionA)
		assert.Equal(t, decdomain.DecisionTypeApproved, decisionA.Decision)

		// Verify workflow state
		instA, err = workflowSvc.GetInstanceByCaseID(ctx, orgA, caseA.ID)
		require.NoError(t, err)
		assert.Equal(t, "APPROVED", instA.CurrentState)

		// Verify AI observations persisted
		aiObsA, _, err := aiObsRepo.FindByCase(ctx, orgA, caseA.ID, 100, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(aiObsA), 1)

		// Verify verified fact persisted
		factsA, _, err := aiFactRepo.FindByCase(ctx, orgA, caseA.ID, 100, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(factsA), 1)

		// Verify audit
		auditEventsA, err := auditRepo.FindByOrganization(ctx, orgA, 100, 0)
		require.NoError(t, err)
		assert.True(t, len(auditEventsA) > 0)
	})

	// ==================================================================
	// PROCESS B: Education Assistance (Organization B)
	// ==================================================================
	t.Run("Process B Education Assistance - Full AI Pipeline", func(t *testing.T) {
		now := time.Now().UTC()

		// Workflow: Applied -> DocumentReview -> Interview -> Awarded/Rejected -> Enrolled
		wfB := &workflowdomain.WorkflowDefinition{
			ID: uuid.New(), TenantID: orgB, Key: "education_assistance", Name: "Education Assistance",
			Description: "Education grant and scholarship workflow", Version: 1, Status: workflowdomain.WorkflowStatusDraft,
			InitialState: "APPLIED", CreatedAt: now, UpdatedAt: now,
		}
		require.NoError(t, workflowDefRepo.Save(ctx, wfB))

		statesB := []workflowdomain.WorkflowState{
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "APPLIED", Name: "Applied", Terminal: false, DisplayOrder: 0, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "DOCUMENT_REVIEW", Name: "Document Review", Terminal: false, DisplayOrder: 1, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "INTERVIEW", Name: "Interview", Terminal: false, DisplayOrder: 2, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "AWARDED", Name: "Awarded", Terminal: false, DisplayOrder: 3, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "REJECTED", Name: "Rejected", Terminal: true, DisplayOrder: 4, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "ENROLLED", Name: "Enrolled", Terminal: true, DisplayOrder: 5, CreatedAt: now},
		}
		require.NoError(t, workflowStateRepo.SaveBatch(ctx, statesB))

		transitionsB := []workflowdomain.WorkflowTransition{
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "start_review", Name: "Start Review", FromState: "APPLIED", ToState: "DOCUMENT_REVIEW", Active: true, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "schedule_interview", Name: "Schedule Interview", FromState: "DOCUMENT_REVIEW", ToState: "INTERVIEW", Active: true, CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "award", Name: "Award", FromState: "INTERVIEW", ToState: "AWARDED", Active: true, DecisionType: string(decdomain.DecisionTypeApproved), CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "reject", Name: "Reject", FromState: "INTERVIEW", ToState: "REJECTED", Active: true, DecisionType: string(decdomain.DecisionTypeRejected), CreatedAt: now},
			{ID: uuid.New(), WorkflowDefID: wfB.ID, TenantID: orgB, Key: "enroll", Name: "Enroll", FromState: "AWARDED", ToState: "ENROLLED", Active: true, CreatedAt: now},
		}
		require.NoError(t, workflowTransRepo.SaveBatch(ctx, transitionsB))
		require.NoError(t, workflowDefRepo.UpdateStatus(ctx, orgB, wfB.ID, workflowdomain.WorkflowStatusActive, wfB.Version))

		// Form: Education Application (different fields from Emergency)
		formB, err := formSvc.CreateForm(ctx, formapp.CreateFormParams{OrganizationID: orgB, Key: "education_application", Name: "Education Application", Description: "Education grant application", CreatedByID: adminB})
		require.NoError(t, err)

		verB, err := formSvc.CreateVersion(ctx, formapp.CreateVersionParams{OrganizationID: orgB, FormID: formB.ID, ActorID: adminB})
		require.NoError(t, err)

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
		_, err = formSvc.PublishVersion(ctx, formapp.PublishVersionParams{OrganizationID: orgB, VersionID: verB.ID, ActorID: adminB})
		require.NoError(t, err)

		_, err = assignmentSvc.CreateAssignment(ctx, assignmentapp.CreateAssignmentParams{
			TenantID: orgB, WorkflowDefinitionID: wfB.ID, WorkflowStateKey: "APPLIED",
			FormID: formB.ID, FormVersionID: verB.ID, Required: true, DisplayOrder: 0, Active: true, CreatedBy: adminB,
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
		_, err = ruleSvc.PublishRuleSet(ctx, orgB, ruleSetB.ID, adminB)
		require.NoError(t, err)

		// Create case
		caseB, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
			OrganizationID: orgB, CreatedByID: adminB,
			Title: "Education Case B", Description: "Scholarship application",
			ServiceType: "Education", Priority: "Normal",
			WorkflowID: &wfB.ID,
		})
		require.NoError(t, err)

		// Submit form
		_, err = caseSvc.SubmitForm(ctx, orgB, caseB.ID, adminB, verB.ID, map[string]interface{}{
			"student_name": "Alice Student", "gpa": 3.8, "major": "cs", "previous_degree": "hs",
			"essay": "I am a dedicated student with a passion for computer science and a commitment to academic excellence. This essay demonstrates my qualifications for the scholarship program.",
		})
		require.NoError(t, err)

		// Add evidence with documents for AI processing
		evidenceB1, err := evidenceSvc.AddEvidence(ctx, evidenceapp.AddEvidenceParams{
			OrganizationID: orgB, ServiceRequestID: caseB.ID,
			Type: evidencedomain.EvidenceTypeSupportingDocument, Description: "Academic transcript",
			StorageReference: "civora://test/education/transcript_b", UploadedBy: adminB,
			Source: evidencedomain.EvidenceSourceManual,
		})
		require.NoError(t, err)

		_, err = evidenceSvc.UploadDocument(ctx, evidenceapp.UploadDocumentParams{
			OrganizationID: orgB, EvidenceID: evidenceB1.ID,
			FileName: "transcript.txt", ContentType: "text/plain",
			Size: int64(len("Transcript: Alice Student, GPA 3.8, Computer Science, previous degree: High School")), Reader: strings.NewReader("Transcript: Alice Student, GPA 3.8, Computer Science, previous degree: High School"),
			UploadedBy: adminB,
		})
		require.NoError(t, err)

		evidenceB2, err := evidenceSvc.AddEvidence(ctx, evidenceapp.AddEvidenceParams{
			OrganizationID: orgB, ServiceRequestID: caseB.ID,
			Type: evidencedomain.EvidenceTypeOther, Description: "Personal statement essay",
			StorageReference: "civora://test/education/essay_b", UploadedBy: adminB,
			Source: evidencedomain.EvidenceSourceManual,
		})
		require.NoError(t, err)

		_, err = evidenceSvc.UploadDocument(ctx, evidenceapp.UploadDocumentParams{
			OrganizationID: orgB, EvidenceID: evidenceB2.ID,
			FileName: "essay.txt", ContentType: "text/plain",
			Size: int64(len("Personal essay demonstrating commitment to computer science and academic excellence")), Reader: strings.NewReader("Personal essay demonstrating commitment to computer science and academic excellence"),
			UploadedBy: adminB,
		})
		require.NoError(t, err)

		// AI: Generate evidence-level and case-level observations
		aiProviderB := &mockAIProvider{
			evidenceObservations: []aiapp.ObservationResult{
				{Type: ai_domain.ObservationTypeSummary, Content: map[string]any{"statement": "Transcript confirms GPA 3.8 in Computer Science"}, Confidence: floatPtr(0.96), Model: ai_domain.ModelInfo{Name: "mock-ai", Version: "1.0.0", Provider: "test"}},
				{Type: ai_domain.ObservationTypeEntityExtraction, Content: map[string]any{"entities": []string{"GPA:3.8", "Major:Computer Science"}}, Confidence: floatPtr(0.94), Model: ai_domain.ModelInfo{Name: "mock-ai", Version: "1.0.0", Provider: "test"}},
			},
			caseObservations: []aiapp.ObservationResult{
				{Type: ai_domain.ObservationTypeSummary, Content: map[string]any{"statement": "Education case: strong candidate with GPA 3.8, no prior degree, eligible for scholarship"}, Confidence: floatPtr(0.93), Model: ai_domain.ModelInfo{Name: "mock-ai", Version: "1.0.0", Provider: "test"}},
			},
		}

		aiSvcB := newAIService(db, auditService, aiProviderB, evidenceSvc)

		// Evidence-level observations
		evObsResultB, err := aiSvcB.GenerateObservations(ctx, aiapp.GenerateObservationsParams{
			OrganizationID: orgB, EvidenceID: evidenceB1.ID, ActorID: adminB,
			Types: []ai_domain.ObservationType{ai_domain.ObservationTypeSummary, ai_domain.ObservationTypeEntityExtraction},
		})
		require.NoError(t, err)
		require.Len(t, evObsResultB.Observations, 2)

		// Case-level observations
		caseObsResultB, err := aiSvcB.GenerateCaseObservations(ctx, aiapp.GenerateCaseObservationsParams{
			OrganizationID: orgB, CaseID: caseB.ID, ActorID: adminB,
			Types: []ai_domain.ObservationType{ai_domain.ObservationTypeSummary},
		})
		require.NoError(t, err)
		require.Len(t, caseObsResultB.Observations, 1)

		// Human verification: reject one observation, accept another
		_, err = aiSvcB.ReviewObservation(ctx, aiapp.ReviewObservationParams{
			OrganizationID: orgB, ObservationID: evObsResultB.Observations[0].ID,
			ReviewerID: adminB, Action: ai_domain.ObservationStatusAccepted, Notes: "Accurate transcript summary",
		})
		require.NoError(t, err)

		_, err = aiSvcB.ReviewObservation(ctx, aiapp.ReviewObservationParams{
			OrganizationID: orgB, ObservationID: evObsResultB.Observations[1].ID,
			ReviewerID: adminB, Action: ai_domain.ObservationStatusCorrected, Notes: "Entity extraction needs refinement",
		})
		require.NoError(t, err)

		// Create verified facts
		factB1, err := aiSvcB.CreateVerifiedFact(ctx, aiapp.CreateVerifiedFactParams{
			OrganizationID: orgB, ObservationID: evObsResultB.Observations[0].ID,
			ReviewerID: adminB, ReviewAction: ai_domain.ObservationStatusAccepted, ReviewNotes: "Verified",
		})
		require.NoError(t, err)
		assert.Equal(t, "HUMAN_ACCEPTED", factB1.Source)

		factB2, err := aiSvcB.CreateVerifiedFact(ctx, aiapp.CreateVerifiedFactParams{
			OrganizationID: orgB, ObservationID: evObsResultB.Observations[1].ID,
			ReviewerID: adminB, ReviewAction: ai_domain.ObservationStatusCorrected, ReviewNotes: "Corrected extraction",
		})
		require.NoError(t, err)
		assert.Equal(t, "HUMAN_CORRECTION", factB2.Source)

		// Evaluate rules
		evalB, err := ruleSvc.EvaluateRuleSet(ctx, rulesapp.EvaluateRuleSetParams{
			OrganizationID: orgB, ID: ruleSetB.ID, Trigger: rulesdomain.TriggerManual, ActorID: &adminB, EvaluatedAt: &now,
		})
		require.NoError(t, err)
		assert.NotNil(t, evalB)

		// Advance workflow
		instB, _ := workflowSvc.GetInstanceByCaseID(ctx, orgB, caseB.ID)
		_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{TenantID: orgB, InstanceID: instB.ID, TransitionKey: "start_review", ActorID: adminB, ActorRole: "admin", Reason: "Move to document review"})
		require.NoError(t, err)
		instB, _ = workflowSvc.GetInstanceByCaseID(ctx, orgB, caseB.ID)
		_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{TenantID: orgB, InstanceID: instB.ID, TransitionKey: "schedule_interview", ActorID: adminB, ActorRole: "admin", Reason: "Move to interview"})
		require.NoError(t, err)

		// Human review
		instB, _ = workflowSvc.GetInstanceByCaseID(ctx, orgB, caseB.ID)
		reviewEntryB := revdomain.NewReviewQueueEntry(orgB, caseB.ID, instB.ID, "INTERVIEW", casedomain.Priority("Normal"), []uuid.UUID{evalB.ID}, nil, nil)
		require.NoError(t, reviewRepo.Save(ctx, reviewEntryB))

		reviewEntryB, err = reviewSvc.ClaimReview(ctx, revapp.ClaimReviewParams{OrganizationID: orgB, ReviewID: reviewEntryB.ID, ReviewerID: adminB})
		require.NoError(t, err)
		reviewEntryB, err = reviewSvc.StartReview(ctx, revapp.StartReviewParams{OrganizationID: orgB, ReviewID: reviewEntryB.ID, ReviewerID: adminB})
		require.NoError(t, err)
		reviewEntryB, err = reviewSvc.CompleteReview(ctx, revapp.CompleteReviewParams{
			OrganizationID: orgB, ReviewID: reviewEntryB.ID, ReviewerID: adminB,
			Decision: string(decdomain.DecisionTypeApproved), Reason: "Strong candidate, proceed to enrollment",
		})
		require.NoError(t, err)
		assert.Equal(t, revdomain.ReviewStatusCompleted, reviewEntryB.Status)

		// Verify decision
		decisionB, err := decisionSvc.GetDecisionByServiceRequest(ctx, orgB, caseB.ID)
		require.NoError(t, err)
		assert.NotNil(t, decisionB)
		assert.Equal(t, decdomain.DecisionTypeApproved, decisionB.Decision)

		// Verify workflow state
		instB, err = workflowSvc.GetInstanceByCaseID(ctx, orgB, caseB.ID)
		require.NoError(t, err)
		assert.Equal(t, "AWARDED", instB.CurrentState)

		// Verify AI data for org B
		aiObsB, _, err := aiObsRepo.FindByCase(ctx, orgB, caseB.ID, 100, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(aiObsB), 1)

		factsB, _, err := aiFactRepo.FindByCase(ctx, orgB, caseB.ID, 100, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(factsB), 2)
	})

	// ==================================================================
}

func TestAIPlatformProof_AIDisabled_CoreFunctionality(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.ResetDB(t, db)
	ctx := context.Background()

	org := helpers.SeedOrg(db)
	admin := helpers.SeedUser(db, org)

	caseRepo := caseinfra.NewPostgresCaseRepository(db)
	workflowDefRepo := workflowinfra.NewPostgresWorkflowDefinitionRepository(db)
	workflowStateRepo := workflowinfra.NewPostgresWorkflowStateRepository(db)
	workflowTransRepo := workflowinfra.NewPostgresWorkflowTransitionRepository(db)
	workflowInstRepo := workflowinfra.NewPostgresWorkflowInstanceRepository(db)
	workflowHistRepo := workflowinfra.NewPostgresWorkflowTransitionHistoryRepository(db)
	formRepo := forminfra.NewPostgresFormRepository(db)
	formVerRepo := forminfra.NewPostgresFormVersionRepository(db)
	formFieldRepo := forminfra.NewPostgresFormFieldRepository(db)
	evidenceRepo := evidencepostgres.NewPostgresEvidenceRepository(db)
	evidenceStorage, err := evidencestorage.NewLocalStorageProvider(t.TempDir())
	require.NoError(t, err)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db)

	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	workflowSvc := workflowapp.NewWorkflowService(workflowDefRepo, workflowStateRepo, workflowTransRepo, workflowInstRepo, workflowHistRepo, auditService)
	caseSvc := caseapp.NewCaseService(caseRepo, nil, nil, auditService, auditRepo, workflowSvc)
	caseSvc.SetFormRepos(nil, workflowInstRepo, formRepo, formVerRepo, formFieldRepo, nil, nil)

	caseFinder := &simpleCaseFinder{caseRepo: caseRepo}
	evidenceSvc := evidenceapp.NewEvidenceService(evidenceRepo, caseFinder, &simpleUserChecker{}, auditService, evidenceStorage)

	aiSvc := newAIService(db, auditService, aiprovider.NewNoopProvider(), evidenceSvc)

	t.Run("Core CIVORA functions work with AI disabled", func(t *testing.T) {
		wf := &workflowdomain.WorkflowDefinition{
			ID: uuid.New(), TenantID: org, Key: "assistance", Name: "Assistance",
			Description: "General assistance workflow", Version: 1, Status: workflowdomain.WorkflowStatusDraft,
			InitialState: "INTAKE", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}
		require.NoError(t, workflowDefRepo.Save(ctx, wf))

		states := []workflowdomain.WorkflowState{
			{ID: uuid.New(), WorkflowDefID: wf.ID, TenantID: org, Key: "INTAKE", Name: "Intake", Terminal: false, DisplayOrder: 0, CreatedAt: time.Now().UTC()},
			{ID: uuid.New(), WorkflowDefID: wf.ID, TenantID: org, Key: "REVIEW", Name: "Review", Terminal: false, DisplayOrder: 1, CreatedAt: time.Now().UTC()},
			{ID: uuid.New(), WorkflowDefID: wf.ID, TenantID: org, Key: "CLOSED", Name: "Closed", Terminal: true, DisplayOrder: 2, CreatedAt: time.Now().UTC()},
		}
		require.NoError(t, workflowStateRepo.SaveBatch(ctx, states))

		transitions := []workflowdomain.WorkflowTransition{
			{ID: uuid.New(), WorkflowDefID: wf.ID, TenantID: org, Key: "submit", Name: "Submit", FromState: "INTAKE", ToState: "REVIEW", Active: true, CreatedAt: time.Now().UTC()},
			{ID: uuid.New(), WorkflowDefID: wf.ID, TenantID: org, Key: "close", Name: "Close", FromState: "REVIEW", ToState: "CLOSED", Active: true, CreatedAt: time.Now().UTC()},
		}
		require.NoError(t, workflowTransRepo.SaveBatch(ctx, transitions))
		require.NoError(t, workflowDefRepo.UpdateStatus(ctx, org, wf.ID, workflowdomain.WorkflowStatusActive, wf.Version))

		case1, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
			OrganizationID: org, CreatedByID: admin,
			Title: "AI Disabled Case", Description: "Test case with AI disabled",
			ServiceType: "Emergency", Priority: "Normal",
			WorkflowID: &wf.ID,
		})
		require.NoError(t, err)

		evidence1, err := evidenceSvc.AddEvidence(ctx, evidenceapp.AddEvidenceParams{
			OrganizationID: org, ServiceRequestID: case1.ID,
			Type: evidencedomain.EvidenceTypeIdentityDocument, Description: "ID",
			StorageReference: "civora://test/ai_disabled/id", UploadedBy: admin,
			Source: evidencedomain.EvidenceSourceManual,
		})
		require.NoError(t, err)
		assert.NotNil(t, evidence1)

		_, err = aiSvc.GenerateObservations(ctx, aiapp.GenerateObservationsParams{
			OrganizationID: org, EvidenceID: evidence1.ID, ActorID: admin,
			Types: []ai_domain.ObservationType{ai_domain.ObservationTypeSummary},
		})
		assert.ErrorIs(t, err, aiapp.ErrAIProviderUnavailable)

		inst, _ := workflowSvc.GetInstanceByCaseID(ctx, org, case1.ID)
		require.NotNil(t, inst)
		_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{TenantID: org, InstanceID: inst.ID, TransitionKey: "submit", ActorID: admin, ActorRole: "admin", Reason: "Submit"})
		require.NoError(t, err)
		inst, _ = workflowSvc.GetInstanceByCaseID(ctx, org, case1.ID)
		assert.Equal(t, "REVIEW", inst.CurrentState)
	})
}

func floatPtr(f float64) *float64 {
	return &f
}
