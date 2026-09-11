package integration

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	assistanceapp "github.com/alrazihi/civora/internal/assistance/application"
	assistancedomain "github.com/alrazihi/civora/internal/assistance/domain"
	assistancepostgres "github.com/alrazihi/civora/internal/assistance/infrastructure/postgres"
	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	caseapp "github.com/alrazihi/civora/internal/cases/application"
	caseDomain "github.com/alrazihi/civora/internal/cases/domain"
	casepostgres "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	decisionsapp "github.com/alrazihi/civora/internal/decisions/application"
	decisionspostgres "github.com/alrazihi/civora/internal/decisions/infrastructure/postgres"
	eligibilityapp "github.com/alrazihi/civora/internal/eligibility/application"
	eligibilitypostgres "github.com/alrazihi/civora/internal/eligibility/infrastructure/postgres"
	identityDomain "github.com/alrazihi/civora/internal/identity/domain"
	identitypostgres "github.com/alrazihi/civora/internal/identity/infrastructure/postgres"
	orgapp "github.com/alrazihi/civora/internal/organizations/application"
	orgdomain "github.com/alrazihi/civora/internal/organizations/domain"
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

func setupConcurrencyServices(t *testing.T) (
	*caseapp.CaseService,
	*decisionsapp.DecisionService,
	*assistanceapp.AssistanceService,
	*workflowapp.WorkflowService,
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
	decisionRepo := decisionspostgres.NewPostgresDecisionRepository(db)
	assistanceRepo := assistancepostgres.NewPostgresAssistanceRepository(db)
	eligibilityRepo := eligibilitypostgres.NewPostgresEligibilityRepository(db)
	defRepo := workflowpostgres.NewPostgresWorkflowDefinitionRepository(db)
	stateRepo := workflowpostgres.NewPostgresWorkflowStateRepository(db)
	transitionRepo := workflowpostgres.NewPostgresWorkflowTransitionRepository(db)
	instanceRepo := workflowpostgres.NewPostgresWorkflowInstanceRepository(db)
	historyRepo := workflowpostgres.NewPostgresWorkflowTransitionHistoryRepository(db)

	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})
	roleCreator := identityDomain.NewDefaultRoleCreator(roleRepo)
	orgSvc := orgapp.NewOrganizationService(orgRepo, roleCreator, auditService)
	_ = orgSvc

	workflowSvc := workflowapp.NewWorkflowService(defRepo, stateRepo, transitionRepo, instanceRepo, historyRepo, auditService)
	caseSvc := caseapp.NewCaseService(caseRepo, peoplepostgres.NewPostgresPersonRepository(db), identityDomain.NewOrganizationUserChecker(userRepo), auditService, auditRepo, workflowSvc)
	decisionSvc := decisionsapp.NewDecisionService(decisionRepo, caseRepo, identityDomain.NewOrganizationUserChecker(userRepo), auditService, workflowSvc)
	assistanceSvc := assistanceapp.NewAssistanceService(assistanceRepo, caseRepo, identityDomain.NewOrganizationUserChecker(userRepo), auditService)
	_ = eligibilityRepo
	_ = eligibilityapp.NewEligibilityService

	return caseSvc, decisionSvc, assistanceSvc, workflowSvc, db
}

func TestConcurrentCaseTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency test")
	}

	caseSvc, _, _, workflowSvc, db := setupConcurrencyServices(t)
	ctx := context.Background()

	org, err := setupConcurrencyOrg(ctx, db, t)
	require.NoError(t, err)
	actorID := helpers.SeedUser(db, org.ID)

	_, _, _ = seedWorkflowDefinitionForConcurrency(ctx, db, t, org.ID, uuid.Nil)

	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: org.ID,
		Title:          "Concurrency Test Case",
		Description:    "Test",
		ServiceType:    caseDomain.ServiceTypeGeneral,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	instance, err := workflowSvc.GetInstanceByCaseID(ctx, org.ID, c.ID)
	require.NoError(t, err)

	transitions, err := workflowSvc.GetValidTransitions(ctx, org.ID, instance.ID)
	require.NoError(t, err)
	require.NotEmpty(t, transitions)

	var openTransition *workflowdomain.WorkflowTransition
	for i := range transitions {
		if transitions[i].ToState == "OPEN" {
			openTransition = &transitions[i]
			break
		}
	}
	require.NotNil(t, openTransition)

	var successCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{
				TenantID:      org.ID,
				InstanceID:    instance.ID,
				TransitionKey: openTransition.Key,
				ActorID:       actorID,
				ActorRole:     "",
				Reason:        "",
			})
			if err != nil {
				t.Logf("ChangeStatus error: %v", err)
			}
			mu.Lock()
			if err == nil {
				successCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	assert.Equal(t, 1, successCount,
		"exactly one concurrent transition should succeed; got %d", successCount)

	updated, err := caseSvc.GetCase(ctx, org.ID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, caseDomain.CaseStatusOpen, updated.Status)
}

func TestConcurrentDuplicateAssistance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency test")
	}

	_, _, assistanceSvc, _, db := setupConcurrencyServices(t)
	ctx := context.Background()

	org, err := setupConcurrencyOrg(ctx, db, t)
	require.NoError(t, err)
	actorID := helpers.SeedUser(db, org.ID)

	caseID := uuid.New()
	_, err = db.ExecContext(ctx,
		`INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, created_by, assigned_to, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())`,
		caseID, org.ID, "CONC-001", "Concurrency Case", "", "NEW", "GENERAL", "NORMAL", actorID, nil,
	)
	require.NoError(t, err)

	var successCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := assistanceSvc.CreateAssistance(ctx, assistanceapp.CreateAssistanceParams{
				OrganizationID:   org.ID,
				ServiceRequestID: caseID,
				Type:             assistancedomain.AssistanceTypeShelter,
				Description:      "Emergency shelter",
				ResponsibleStaff: actorID,
				ActorID:          actorID,
			})
			if err != nil {
				t.Logf("CreateAssistance error: %v", err)
			}
			mu.Lock()
			if err == nil {
				successCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	assert.Equal(t, 5, successCount, "all concurrent assistance creations should succeed; got %d", successCount)
}

func TestConcurrentDuplicateDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency test")
	}

	caseSvc, _, _, workflowSvc, db := setupConcurrencyServices(t)
	ctx := context.Background()

	org, err := setupConcurrencyOrg(ctx, db, t)
	require.NoError(t, err)
	actorID := helpers.SeedUser(db, org.ID)

	_, _, _ = seedWorkflowDefinitionForConcurrency(ctx, db, t, org.ID, uuid.Nil)

	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: org.ID,
		Title:          "Concurrency Decision Case",
		Description:    "Test",
		ServiceType:    caseDomain.ServiceTypeGeneral,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	instance, err := workflowSvc.GetInstanceByCaseID(ctx, org.ID, c.ID)
	require.NoError(t, err)

	openTransitions, err := workflowSvc.GetValidTransitions(ctx, org.ID, instance.ID)
	require.NoError(t, err)

	var openTransition *workflowdomain.WorkflowTransition
	for i := range openTransitions {
		if openTransitions[i].ToState == "OPEN" {
			openTransition = &openTransitions[i]
			break
		}
	}
	require.NotNil(t, openTransition)

	_, err = workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{
		TenantID:      org.ID,
		InstanceID:    instance.ID,
		TransitionKey: openTransition.Key,
		ActorID:       actorID,
		ActorRole:     "",
		Reason:        "",
	})
	require.NoError(t, err)

	instance, err = workflowSvc.GetInstanceByCaseID(ctx, org.ID, c.ID)
	require.NoError(t, err)

	transitions, err := workflowSvc.GetValidTransitions(ctx, org.ID, instance.ID)
	require.NoError(t, err)

	var approveTransition *workflowdomain.WorkflowTransition
	for i := range transitions {
		if transitions[i].ToState == "APPROVED" {
			approveTransition = &transitions[i]
			break
		}
	}
	require.NotNil(t, approveTransition)

	var successCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{
				TenantID:      org.ID,
				InstanceID:    instance.ID,
				TransitionKey: approveTransition.Key,
				ActorID:       actorID,
				ActorRole:     "",
				Reason:        "",
			})
			mu.Lock()
			if err == nil {
				successCount++
			} else {
				t.Logf("MakeDecision error: %v", err)
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	assert.Equal(t, 1, successCount,
		"exactly one concurrent decision should succeed; got %d", successCount)
}

func setupConcurrencyOrg(ctx context.Context, db *sql.DB, t *testing.T) (*orgdomain.Organization, error) {
	orgRepo := orgpostgres.NewPostgresOrganizationRepository(db)
	roleRepo := identitypostgres.NewPostgresRoleRepository(db)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db)
	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})
	roleCreator := identityDomain.NewDefaultRoleCreator(roleRepo)
	orgSvc := orgapp.NewOrganizationService(orgRepo, roleCreator, auditService)

	return orgSvc.CreateOrganization(ctx, orgapp.CreateOrganizationParams{
		Name:        "Concurrency Org",
		Description: "",
		Slug:        "conc-org-" + uuid.NewString()[:8],
	})
}

func seedWorkflowDefinitionForConcurrency(ctx context.Context, db *sql.DB, t *testing.T, orgID uuid.UUID, caseID uuid.UUID) (*workflowapp.WorkflowService, uuid.UUID, *workflowdomain.WorkflowInstance) {
	t.Helper()
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
		{ID: uuid.New(), TenantID: orgID, Key: "APPROVED", Name: "Approved", Terminal: false, DisplayOrder: 2, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "CLOSED", Name: "Closed", Terminal: true, DisplayOrder: 3, CreatedAt: now},
	}

	transitions := []workflowdomain.WorkflowTransition{
		{ID: uuid.New(), TenantID: orgID, Key: "open", Name: "Open", FromState: "NEW", ToState: "OPEN", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "approve", Name: "Approve", FromState: "OPEN", ToState: "APPROVED", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "close", Name: "Close", FromState: "APPROVED", ToState: "CLOSED", Active: true, CreatedAt: now},
	}

	allKeys := []string{
		"emergency_assistance",
		"medical_assistance",
		"financial_assistance",
		"food_assistance",
		"shelter_assistance",
		"education_assistance",
		"transport_assistance",
		"general_assistance",
		"concurrent_test",
	}

	var concurrentDefID uuid.UUID
	for _, key := range allKeys {
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

		def, err := workflowSvc.CreateWorkflowDefinition(ctx, workflowapp.CreateWorkflowDefinitionParams{
			TenantID:     orgID,
			ActorID:      uuid.Nil,
			Key:          key,
			Name:         "Concurrent Test",
			Description:  "Workflow for concurrency testing",
			Version:      1,
			InitialState: "NEW",
			States:       statesCopy,
			Transitions:  transitionsCopy,
			Metadata:     map[string]interface{}{},
		})
		require.NoError(t, err)

		err = workflowSvc.ActivateWorkflowDefinition(ctx, orgID, def.ID, uuid.Nil)
		require.NoError(t, err)

		if key == "concurrent_test" {
			concurrentDefID = def.ID
		}
	}

	if caseID == uuid.Nil {
		return workflowSvc, concurrentDefID, nil
	}

	instance, err := workflowSvc.CreateInstanceForCase(ctx, orgID, caseID, "concurrent_test", uuid.Nil)
	require.NoError(t, err)

	return workflowSvc, concurrentDefID, instance
}

func TestConcurrentWorkflowInstanceTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)
	ctx := context.Background()

	org, err := setupConcurrencyOrg(ctx, db, t)
	require.NoError(t, err)

	actorID := helpers.SeedUser(db, org.ID)

	caseID := uuid.New()
	_, err = db.ExecContext(ctx,
		`INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, created_by, assigned_to, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())`,
		caseID, org.ID, "CONC-WF-001", "Concurrency Workflow Case", "", "NEW", "GENERAL", "NORMAL", actorID, nil,
	)
	require.NoError(t, err)

	workflowSvc, _, instance := seedWorkflowDefinitionForConcurrency(ctx, db, t, org.ID, caseID)

	var successCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := workflowSvc.ExecuteTransition(ctx, workflowapp.ExecuteTransitionParams{
				TenantID:      org.ID,
				InstanceID:    instance.ID,
				TransitionKey: "open",
				ActorID:       actorID,
				ActorRole:     "",
				Reason:        "",
			})
			if err != nil {
				t.Logf("ExecuteTransition error: %v", err)
			}
			mu.Lock()
			if err == nil {
				successCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	assert.Equal(t, 1, successCount,
		"exactly one concurrent workflow transition should succeed; got %d", successCount)

	updated, err := workflowSvc.GetInstanceByCaseID(ctx, org.ID, instance.CaseID)
	require.NoError(t, err)
	assert.Equal(t, "OPEN", updated.CurrentState)
}
