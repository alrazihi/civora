package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	aiapi "github.com/alrazihi/civora/internal/ai/api"
	aiapplication "github.com/alrazihi/civora/internal/ai/application"
	aiadapter "github.com/alrazihi/civora/internal/ai/infrastructure/adapter"
	aiinfrapostgres "github.com/alrazihi/civora/internal/ai/infrastructure/postgres"
	aiprovider "github.com/alrazihi/civora/internal/ai/infrastructure/provider"
	aisanitizer "github.com/alrazihi/civora/internal/ai/infrastructure/sanitizer"
	assessmentapi "github.com/alrazihi/civora/internal/assessment/api"
	assessmentapp "github.com/alrazihi/civora/internal/assessment/application"
	assessmentpostgres "github.com/alrazihi/civora/internal/assessment/infrastructure/postgres"
	assistanceapi "github.com/alrazihi/civora/internal/assistance/api"
	assistancapp "github.com/alrazihi/civora/internal/assistance/application"
	assistancepostgres "github.com/alrazihi/civora/internal/assistance/infrastructure/postgres"
	auditapi "github.com/alrazihi/civora/internal/audit/api"
	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditinfra "github.com/alrazihi/civora/internal/audit/infrastructure"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	casecontextapi "github.com/alrazihi/civora/internal/casecontext/api"
	casecontextapp "github.com/alrazihi/civora/internal/casecontext/application"
	casecontextauth "github.com/alrazihi/civora/internal/casecontext/infrastructure/auth"
	casecontextpostgres "github.com/alrazihi/civora/internal/casecontext/infrastructure/postgres"
	caseapi "github.com/alrazihi/civora/internal/cases/api"
	caseapp "github.com/alrazihi/civora/internal/cases/application"
	casepostgres "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	casesummaryapi "github.com/alrazihi/civora/internal/casesummary/api"
	casesummaryapp "github.com/alrazihi/civora/internal/casesummary/application"
	casesummarypostgres "github.com/alrazihi/civora/internal/casesummary/infrastructure/postgres"
	casesummaryprovider "github.com/alrazihi/civora/internal/casesummary/infrastructure/provider"
	"github.com/alrazihi/civora/internal/config"
	"github.com/alrazihi/civora/internal/database"
	decisionsapi "github.com/alrazihi/civora/internal/decisions/api"
	decisionsapp "github.com/alrazihi/civora/internal/decisions/application"
	decisionspostgres "github.com/alrazihi/civora/internal/decisions/infrastructure/postgres"
	documentintelligenceapi "github.com/alrazihi/civora/internal/documentintelligence/api"
	documentintelligenceapp "github.com/alrazihi/civora/internal/documentintelligence/application"
	documentintelligencepostgres "github.com/alrazihi/civora/internal/documentintelligence/infrastructure/postgres"
	documentintelligenceprovider "github.com/alrazihi/civora/internal/documentintelligence/infrastructure/provider"
	eligibilityapi "github.com/alrazihi/civora/internal/eligibility/api"
	eligibilityapp "github.com/alrazihi/civora/internal/eligibility/application"
	eligibilitypostgres "github.com/alrazihi/civora/internal/eligibility/infrastructure/postgres"
	evidenceapi "github.com/alrazihi/civora/internal/evidence/api"
	evidenceapp "github.com/alrazihi/civora/internal/evidence/application"
	evidencepostgres "github.com/alrazihi/civora/internal/evidence/infrastructure/postgres"
	evidencestorage "github.com/alrazihi/civora/internal/evidence/infrastructure/storage"
	followupapi "github.com/alrazihi/civora/internal/followup/api"
	followupapp "github.com/alrazihi/civora/internal/followup/application"
	followuppostgres "github.com/alrazihi/civora/internal/followup/infrastructure/postgres"
	submissionpostgres "github.com/alrazihi/civora/internal/form_submission/infrastructure/postgres"
	formapi "github.com/alrazihi/civora/internal/forms/api"
	formapp "github.com/alrazihi/civora/internal/forms/application"
	formpostgres "github.com/alrazihi/civora/internal/forms/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/identity"
	identityapi "github.com/alrazihi/civora/internal/identity/api"
	identityapp "github.com/alrazihi/civora/internal/identity/application"
	"github.com/alrazihi/civora/internal/identity/domain"
	identitypostgres "github.com/alrazihi/civora/internal/identity/infrastructure/postgres"
	intmid "github.com/alrazihi/civora/internal/middleware"
	orgapi "github.com/alrazihi/civora/internal/organizations/api"
	orgapp "github.com/alrazihi/civora/internal/organizations/application"
	orgdomain "github.com/alrazihi/civora/internal/organizations/domain"
	orgpostgres "github.com/alrazihi/civora/internal/organizations/infrastructure/postgres"
	peopleapi "github.com/alrazihi/civora/internal/people/api"
	peoplapp "github.com/alrazihi/civora/internal/people/application"
	peoplepostgres "github.com/alrazihi/civora/internal/people/infrastructure/postgres"
	reviewqueueapi "github.com/alrazihi/civora/internal/review_queue/api"
	reviewqueueapp "github.com/alrazihi/civora/internal/review_queue/application"
	reviewqueuepostgres "github.com/alrazihi/civora/internal/review_queue/infrastructure/postgres"
	rulesapi "github.com/alrazihi/civora/internal/rules/api"
	rulesapp "github.com/alrazihi/civora/internal/rules/application"
	rulespostgres "github.com/alrazihi/civora/internal/rules/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/server"
	workflowapi "github.com/alrazihi/civora/internal/workflow/api"
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowpostgres "github.com/alrazihi/civora/internal/workflow/infrastructure/postgres"
	assignmentapi "github.com/alrazihi/civora/internal/workflow_form_assignment/api"
	assignmentapp "github.com/alrazihi/civora/internal/workflow_form_assignment/application"
	assignmentpostgres "github.com/alrazihi/civora/internal/workflow_form_assignment/infrastructure/postgres"
	"github.com/alrazihi/civora/migrations"
)

func main() {
	identity.MustVerifyIntegrity()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	dsn := database.BuildDSN(cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)
	db, err := database.NewDatabase(dsn, cfg.Database.Driver)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	migrator := database.NewMigrator(db.DB, migrations.FS)
	if err := migrator.LoadMigrations(); err != nil {
		log.Fatalf("failed to load migrations: %v", err)
	}
	ctx := context.Background()
	if err := migrator.Migrate(ctx); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	userRepo := identitypostgres.NewPostgresUserRepository(db.DB)
	roleRepo := identitypostgres.NewPostgresRoleRepository(db.DB)
	hasher := domain.NewBCryptHasher(cfg.Auth.BCryptCost)

	storageProvider, err := evidencestorage.NewLocalStorageProvider(cfg.Storage.LocalPath)
	if err != nil {
		log.Fatalf("failed to initialize storage provider: %v", err)
	}

	orgRepo := orgpostgres.NewPostgresOrganizationRepository(db.DB)
	caseRepo := casepostgres.NewPostgresCaseRepository(db.DB)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db.DB)
	personRepo := peoplepostgres.NewPostgresPersonRepository(db.DB)
	eligibilityRepo := eligibilitypostgres.NewPostgresEligibilityRepository(db.DB)
	evidenceRepo := evidencepostgres.NewPostgresEvidenceRepository(db.DB)
	assessmentRepo := assessmentpostgres.NewPostgresAssessmentRepository(db.DB)
	decisionRepo := decisionspostgres.NewPostgresDecisionRepository(db.DB)
	assistanceRepo := assistancepostgres.NewPostgresAssistanceRepository(db.DB)
	followUpRepo := followuppostgres.NewPostgresFollowUpRepository(db.DB)
	formRepo := formpostgres.NewPostgresFormRepository(db.DB)
	formVersionRepo := formpostgres.NewPostgresFormVersionRepository(db.DB)
	formFieldRepo := formpostgres.NewPostgresFormFieldRepository(db.DB)
	formSubmissionRepo := submissionpostgres.NewPostgresFormSubmissionRepository(db.DB)

	ruleSetRepo := rulespostgres.NewPostgresRuleSetRepository(db.DB)
	evalRepo := rulespostgres.NewPostgresEvaluationRepository(db.DB)
	ruleTemplateRepo := rulespostgres.NewRuleTemplateRepository(db.DB)
	ruleAssignmentRepo := rulespostgres.NewPostgresWorkflowStateRuleAssignmentRepository(db.DB)

	summaryRepo := casesummarypostgres.NewPostgresCaseSummaryRepository(db.DB)

	ctxRepo := casecontextpostgres.NewCaseContextRepository(db.DB)

	documentAnalysisRepo := documentintelligencepostgres.NewPostgresAnalysisRepository(db.DB)

	workflowDefRepo := workflowpostgres.NewPostgresWorkflowDefinitionRepository(db.DB)
	workflowStateRepo := workflowpostgres.NewPostgresWorkflowStateRepository(db.DB)
	workflowTransitionRepo := workflowpostgres.NewPostgresWorkflowTransitionRepository(db.DB)
	workflowInstanceRepo := workflowpostgres.NewPostgresWorkflowInstanceRepository(db.DB)
	workflowHistoryRepo := workflowpostgres.NewPostgresWorkflowTransitionHistoryRepository(db.DB)
	workflowAssignmentRepo := assignmentpostgres.NewPostgresWorkflowStateFormAssignmentRepository(db.DB)

	auditService := auditapp.NewAuditService(auditRepo, cfg.Audit)

	workflowService := workflowapp.NewWorkflowService(
		workflowDefRepo,
		workflowStateRepo,
		workflowTransitionRepo,
		workflowInstanceRepo,
		workflowHistoryRepo,
		auditService,
	)

	assignmentService := assignmentapp.NewWorkflowStateFormAssignmentService(
		workflowDefRepo,
		workflowStateRepo,
		formRepo,
		formVersionRepo,
		workflowAssignmentRepo,
		auditService,
	)

	if cfg.Audit.RetentionDays > 0 {
		if _, err := auditService.PurgeOld(context.Background()); err != nil {
			log.Printf("warning: failed to purge old audit events: %v", err)
		}
	}

	// Audit maintenance service: periodic integrity verification and
	// retention purging. Runs in the background and exits when the
	// context is cancelled (SIGINT/SIGTERM).
	auditMaintenance := auditinfra.NewAuditMaintenanceService(auditRepo, cfg.Audit)
	maintenanceStop := auditMaintenance.StartBackground(ctx, 24*time.Hour)
	defer close(maintenanceStop)

	// Run an initial verification pass on startup so that any pre-existing
	// integrity failures are surfaced in the logs immediately.
	if verified, failed, _, err := auditMaintenance.VerifyAllOrganizations(ctx); err != nil {
		log.Printf("warning: audit verification failed: %v", err)
	} else if failed > 0 {
		log.Printf("AUDIT INTEGRITY FAILURE: %d of %d events failed verification", failed, verified+failed)
	}

	jwtSvc := intmid.NewJWTService(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry, "civora")
	authMiddleware := intmid.AuthRequired(jwtSvc)

	identityService := identityapp.NewIdentityService(userRepo, roleRepo, hasher, jwtSvc, auditService)

	userRateLimiter := intmid.NewUserRateLimiter(20, 5*time.Minute, 5*time.Minute)
	identityHandler := identityapi.NewHandlerWithOrgLookup(identityService, userRateLimiter, func(ctx context.Context, slug string) (*orgdomain.Organization, error) {
		return orgRepo.FindBySlug(ctx, slug)
	})

	roleCreator := domain.NewDefaultRoleCreator(roleRepo)
	orgService := orgapp.NewOrganizationService(orgRepo, roleCreator, auditService)
	orgHandler := orgapi.NewHandler(orgService)

	caseService := caseapp.NewCaseService(caseRepo, personRepo, domain.NewOrganizationUserChecker(userRepo), auditService, auditRepo, workflowService)
	caseService.SetFormRepos(
		workflowDefRepo,
		workflowInstanceRepo,
		formRepo,
		formVersionRepo,
		formFieldRepo,
		workflowAssignmentRepo,
		formSubmissionRepo,
	)
	caseHandler := caseapi.NewHandler(caseService)

	personService := peoplapp.NewPersonService(personRepo, auditService)
	personHandler := peopleapi.NewHandler(personService)

	eligibilityService := eligibilityapp.NewEligibilityService(eligibilityRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	eligibilityHandler := eligibilityapi.NewHandler(eligibilityService)

	evidenceService := evidenceapp.NewEvidenceService(evidenceRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService, storageProvider)
	evidenceHandler := evidenceapi.NewHandler(evidenceService)

	assessmentService := assessmentapp.NewAssessmentService(assessmentRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	assessmentHandler := assessmentapi.NewHandler(assessmentService)

	decisionService := decisionsapp.NewDecisionService(decisionRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService, workflowService)
	decisionHandler := decisionsapi.NewHandler(decisionService)

	assistanceService := assistancapp.NewAssistanceService(assistanceRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	assistanceHandler := assistanceapi.NewHandler(assistanceService)

	followUpService := followupapp.NewFollowUpService(followUpRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	followUpHandler := followupapi.NewHandler(followUpService)

	formService := formapp.NewFormService(formRepo, formVersionRepo, formFieldRepo, auditService)
	formHandler := formapi.NewHandler(formService)

	rulesService := rulesapp.NewRuleSetService(ruleSetRepo, evalRepo, ruleTemplateRepo, formService, auditService)
	rulesHandler := rulesapi.NewHandler(rulesService)

	rulesAssignmentService := rulesapp.NewRuleAssignmentService(ruleAssignmentRepo, ruleSetRepo, auditService)
	rulesAssignmentHandler := rulesapi.NewAssignmentHandler(rulesAssignmentService)

	ruleIntegration := rulesapp.NewCaseRuleIntegrationService(ruleSetRepo, evalRepo, caseRepo, workflowInstanceRepo, ruleAssignmentRepo, submissionpostgres.NewPostgresFormSubmissionRepository(db.DB), rulesapp.NewFormKeyResolverAdapter(formRepo), auditService)
	caseService.SetRuleIntegration(ruleIntegration)

	reviewQueueRepo := reviewqueuepostgres.NewPostgresReviewQueueRepository(db.DB)
	reviewQueueService := reviewqueueapp.NewReviewQueueService(
		reviewQueueRepo,
		caseRepo,
		domain.NewOrganizationUserChecker(userRepo),
		auditService,
		workflowService,
		decisionService,
	)
	reviewQueueHandler := reviewqueueapi.NewHandler(reviewQueueService)

	auditHandler := auditapi.NewHandler(auditService)

	aiObsRepo := aiinfrapostgres.NewPostgresObservationRepository(db.DB)
	var aiProvider aiapplication.AIProvider
	switch {
	case cfg.AI.Enabled && cfg.AI.Provider == "openai" && cfg.AI.OpenAIAPIKey != "":
		aiProvider = aiprovider.NewOpenAIProvider(aiprovider.OpenAIProviderConfig{
			APIKey:  cfg.AI.OpenAIAPIKey,
			BaseURL: cfg.AI.OpenAIBaseURL,
			Model:   cfg.AI.OpenAIModel,
		})
	case cfg.AI.Enabled && cfg.AI.Provider == "local" && cfg.AI.LocalEnabled:
		aiProvider = aiprovider.NewLocalProvider(cfg.AI.LocalBaseURL, cfg.AI.LocalModel)
	default:
		aiProvider = aiprovider.NewNoopProvider()
	}
	aiService := aiapplication.NewAIService(aiObsRepo, evidenceRepo, formSubmissionRepo, domain.NewOrganizationUserChecker(userRepo), auditService, aiProvider).
		WithTransaction(db.DB).
		WithDocumentContent(aiadapter.NewDocumentContentProvider(evidenceService)).
		WithPIISanitizer(piiSanitizer(cfg))
	aiHandler := aiapi.NewHandler(aiService)

	summaryProvider := casesummaryprovider.NewOpenAICaseSummaryProvider(casesummaryprovider.OpenAIProviderConfig{
		APIKey:  cfg.AI.OpenAIAPIKey,
		BaseURL: cfg.AI.OpenAIBaseURL,
		Model:   cfg.AI.OpenAIModel,
	})
	ctxService := casecontextapp.NewCaseContextService(
		ctxRepo,
		casecontextauth.NewAuthorizationChecker(domain.NewOrganizationUserChecker(userRepo)),
		&caseContextCaseRepoAdapter{inner: caseRepo},
		&caseContextPersonRepoAdapter{inner: personRepo},
		&caseContextEvidenceRepoAdapter{inner: evidenceRepo},
		&caseContextFormSubmissionRepoAdapter{inner: formSubmissionRepo},
		&caseContextRuleRepoAdapter{inner: evalRepo},
		&caseContextWorkflowRepoAdapter{innerInstance: workflowInstanceRepo, innerHistory: workflowHistoryRepo},
		&caseContextDecisionRepoAdapter{inner: decisionRepo},
		&caseContextAIObsRepoAdapter{inner: aiObsRepo},
		domain.NewOrganizationUserChecker(userRepo),
	).
		WithTransaction(db.DB).
		WithDocumentProvider(&caseContextDocumentContentProviderAdapter{inner: func(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) (io.ReadCloser, error) {
			result, err := evidenceService.GetDocumentStream(ctx, orgID, evidenceID, documentID, actorID)
			if err != nil {
				return nil, err
			}
			return result.Content, nil
		}})
	summaryService := casesummaryapp.NewCaseSummaryService(summaryRepo, caseRepo, ctxService, domain.NewOrganizationUserChecker(userRepo), auditService).
		WithTransaction(db.DB).
		WithProvider(summaryProvider)
	summaryHandler := casesummaryapi.NewHandler(summaryService)

	documentIntelligenceProvider := documentintelligenceprovider.NewOpenAIProvider(documentintelligenceprovider.OpenAIProviderConfig{
		APIKey:  cfg.AI.OpenAIAPIKey,
		BaseURL: cfg.AI.OpenAIBaseURL,
		Model:   cfg.AI.OpenAIModel,
	})
	documentIntelligenceService := documentintelligenceapp.NewDocumentAnalysisService(documentAnalysisRepo, evidenceRepo, domain.NewOrganizationUserChecker(userRepo), auditService, documentIntelligenceProvider).
		WithTransaction(db.DB).
		WithDocumentContent(&documentIntelligenceDocumentContentProviderAdapter{inner: func(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) (io.ReadCloser, error) {
			result, err := evidenceService.GetDocumentStream(ctx, orgID, evidenceID, documentID, actorID)
			if err != nil {
				return nil, err
			}
			return result.Content, nil
		}}).
		WithPIISanitizer(piiSanitizer(cfg))
	documentIntelligenceHandler := documentintelligenceapi.NewHandler(documentIntelligenceService)

	caseContextHandler := casecontextapi.NewHandler(ctxService)

	workflowHandler := workflowapi.NewHandler(workflowService)
	assignmentHandler := assignmentapi.NewHandler(assignmentService)

	// The /cases/{caseId}/workflow/forms|requirements|form-submissions endpoints
	// are implemented by the cases module but must be served from the workflow
	// subrouter, whose longer mount path otherwise shadows them (bare 404).
	workflowHandler.SetCaseFormRoutes(workflowapi.CaseFormRoutes{
		Forms:           caseHandler.GetCaseForms,
		Requirements:    caseHandler.GetWorkflowRequirements,
		FormSubmissions: caseHandler.GetCaseFormSubmissions,
	})

	srv := server.New(cfg, db.DB)
	identityHandler.RegisterRoutes(srv.Router(), authMiddleware)
	orgHandler.RegisterRoutes(srv.Router(), authMiddleware)
	caseHandler.RegisterRoutes(srv.Router(), authMiddleware)
	personHandler.RegisterRoutes(srv.Router(), authMiddleware)
	eligibilityHandler.RegisterRoutes(srv.Router(), authMiddleware)
	evidenceHandler.RegisterRoutes(srv.Router(), authMiddleware)
	assessmentHandler.RegisterRoutes(srv.Router(), authMiddleware)
	decisionHandler.RegisterRoutes(srv.Router(), authMiddleware)
	assistanceHandler.RegisterRoutes(srv.Router(), authMiddleware)
	followUpHandler.RegisterRoutes(srv.Router(), authMiddleware)
	formHandler.RegisterRoutes(srv.Router(), authMiddleware)
	rulesHandler.RegisterRoutes(srv.Router(), authMiddleware)
	rulesAssignmentHandler.RegisterRoutes(srv.Router(), authMiddleware)
	auditHandler.RegisterRoutes(srv.Router(), authMiddleware)
	workflowHandler.RegisterRoutes(srv.Router(), authMiddleware)
	assignmentHandler.RegisterRoutes(srv.Router(), authMiddleware)
	reviewQueueHandler.RegisterRoutes(srv.Router(), authMiddleware)
	aiHandler.RegisterRoutes(srv.Router(), authMiddleware)
	summaryHandler.RegisterRoutes(srv.Router(), authMiddleware)
	documentIntelligenceHandler.RegisterRoutes(srv.Router(), authMiddleware)
	caseContextHandler.RegisterRoutes(srv.Router(), authMiddleware)
	srv.MountStaticFS(http.Dir("web"))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := srv.Start(ctx); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	userRateLimiter.Stop()
}

// piiSanitizer returns a PII redactor. PII sanitization is mandatory and
// cannot be disabled.
func piiSanitizer(cfg *config.Config) aiapplication.PIISanitizer {
	return aisanitizer.NewRedactingSanitizer()
}
