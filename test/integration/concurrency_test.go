package integration

import (
	"context"
	"database/sql"
	"sync"
	"testing"

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
	decisionsdomain "github.com/alrazihi/civora/internal/decisions/domain"
	decisionspostgres "github.com/alrazihi/civora/internal/decisions/infrastructure/postgres"
	eligibilityapp "github.com/alrazihi/civora/internal/eligibility/application"
	eligibilitypostgres "github.com/alrazihi/civora/internal/eligibility/infrastructure/postgres"
	identityDomain "github.com/alrazihi/civora/internal/identity/domain"
	identitypostgres "github.com/alrazihi/civora/internal/identity/infrastructure/postgres"
	orgapp "github.com/alrazihi/civora/internal/organizations/application"
	orgdomain "github.com/alrazihi/civora/internal/organizations/domain"
	orgpostgres "github.com/alrazihi/civora/internal/organizations/infrastructure/postgres"
	peoplepostgres "github.com/alrazihi/civora/internal/people/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupConcurrencyServices(t *testing.T) (
	*caseapp.CaseService,
	*decisionsapp.DecisionService,
	*assistanceapp.AssistanceService,
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

	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})
	roleCreator := identityDomain.NewDefaultRoleCreator(roleRepo)
	orgSvc := orgapp.NewOrganizationService(orgRepo, roleCreator, auditService)
	_ = orgSvc

	caseSvc := caseapp.NewCaseService(caseRepo, peoplepostgres.NewPostgresPersonRepository(db), identityDomain.NewOrganizationUserChecker(userRepo), auditService)
	decisionSvc := decisionsapp.NewDecisionService(decisionRepo, caseRepo, identityDomain.NewOrganizationUserChecker(userRepo), auditService)
	assistanceSvc := assistanceapp.NewAssistanceService(assistanceRepo, caseRepo, identityDomain.NewOrganizationUserChecker(userRepo), auditService)
	_ = eligibilityRepo
	_ = eligibilityapp.NewEligibilityService

	return caseSvc, decisionSvc, assistanceSvc, db
}

func TestConcurrentCaseTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency test")
	}

	caseSvc, _, _, db := setupConcurrencyServices(t)
	ctx := context.Background()

	org, err := setupConcurrencyOrg(ctx, db, t)
	require.NoError(t, err)
	actorID := helpers.SeedUser(db, org.ID)

	c, err := caseSvc.CreateCase(ctx, caseapp.CreateCaseParams{
		OrganizationID: org.ID,
		Title:          "Concurrency Test Case",
		Description:    "Test",
		ServiceType:    caseDomain.ServiceTypeGeneral,
		Priority:       caseDomain.PriorityNormal,
		CreatedByID:    actorID,
	})
	require.NoError(t, err)

	var successCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := caseSvc.ChangeStatus(ctx, caseapp.ChangeCaseStatusParams{
				OrganizationID: org.ID,
				CaseID:         c.ID,
				Status:         caseDomain.CaseStatusOpen,
				ActorID:        actorID,
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

	_, _, assistanceSvc, db := setupConcurrencyServices(t)
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
			mu.Lock()
			if err == nil {
				successCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	assert.Greater(t, successCount, 0, "at least one assistance creation should succeed")
}

func TestConcurrentDuplicateDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency test")
	}

	_, decisionSvc, _, db := setupConcurrencyServices(t)
	ctx := context.Background()

	org, err := setupConcurrencyOrg(ctx, db, t)
	require.NoError(t, err)
	actorID := helpers.SeedUser(db, org.ID)

	caseID := uuid.New()
	_, err = db.ExecContext(ctx,
		`INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, created_by, assigned_to, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())`,
		caseID, org.ID, "CONC-DEC", "Concurrency Decision Case", "", "DECISION_PENDING", "GENERAL", "NORMAL", actorID, nil,
	)
	require.NoError(t, err)

	var successCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := decisionSvc.MakeDecision(ctx, decisionsapp.MakeDecisionParams{
				OrganizationID:   org.ID,
				ServiceRequestID: caseID,
				Decision:         decisionsdomain.DecisionTypeApproved,
				Reason:           "Concurrency test",
				ActorID:          actorID,
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
