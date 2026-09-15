package integration

import (
	"context"
	"database/sql"
	"testing"

	casedomain "github.com/alrazihi/civora/internal/cases/domain"
	reviewqueueapp "github.com/alrazihi/civora/internal/review_queue/application"
	reviewdomain "github.com/alrazihi/civora/internal/review_queue/domain"
	reviewqueuepostgres "github.com/alrazihi/civora/internal/review_queue/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reviewQueueEnv struct {
	svc    *reviewqueueapp.ReviewQueueService
	db     *sql.DB
	orgID  uuid.UUID
	userID uuid.UUID
}

func setupReviewQueueEnv(t *testing.T) *reviewQueueEnv {
	t.Helper()
	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	orgID := helpers.SeedOrg(db)
	userID := helpers.SeedUser(db, orgID)
	helpers.SeedDefaultRoles(db, orgID)

	repo := reviewqueuepostgres.NewPostgresReviewQueueRepository(db)
	svc := reviewqueueapp.NewReviewQueueService(
		repo,
		&mockCaseFinder{db: db},
		&mockUserChecker{orgID: orgID, userID: userID},
		nil,
		nil,
	)

	return &reviewQueueEnv{
		svc:    svc,
		db:     db,
		orgID:  orgID,
		userID: userID,
	}
}

type mockCaseFinder struct {
	db *sql.DB
}

func (m *mockCaseFinder) FindByID(ctx context.Context, orgID, caseID uuid.UUID) (*casedomain.Case, error) {
	var c casedomain.Case
	var workflowState sql.NullString
	var workflowKey sql.NullString
	var workflowIDBytes []byte
	err := m.db.QueryRowContext(ctx, `
		SELECT id, organization_id, case_number, title, description, status, service_type, priority, person_id, created_by, assigned_to, created_at, updated_at, closed_at, version, workflow_instance_id, workflow_state, workflow_key, workflow_id
		FROM cases WHERE id = $1 AND organization_id = $2
	`, caseID, orgID).Scan(
		&c.ID, &c.OrganizationID, &c.CaseNumber, &c.Title, &c.Description, &c.Status, &c.ServiceType, &c.Priority, &c.PersonID, &c.CreatedByID, &c.AssignedToID, &c.CreatedAt, &c.UpdatedAt, &c.ClosedAt, &c.Version, &c.WorkflowInstanceID, &workflowState, &workflowKey, &workflowIDBytes,
	)
	if err != nil {
		return nil, err
	}
	c.WorkflowState = workflowState.String
	c.WorkflowKey = workflowKey.String
	if workflowIDBytes != nil {
		id := uuid.UUID(workflowIDBytes)
		c.WorkflowID = &id
	}
	return &c, nil
}

type mockUserChecker struct {
	orgID  uuid.UUID
	userID uuid.UUID
}

func (m *mockUserChecker) BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	return orgID == m.orgID && userID == m.userID, nil
}

func seedCase(t *testing.T, db *sql.DB, orgID, actorID uuid.UUID) uuid.UUID {
	t.Helper()
	caseID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, created_by, created_at, updated_at, version)
		VALUES ($1, $2, 'CASE-001', 'Test Case', '', 'OPEN', 'GENERAL', 'NORMAL', $3, NOW(), NOW(), 1)
	`, caseID, orgID, actorID)
	require.NoError(t, err)
	return caseID
}

func seedWorkflowInstance(t *testing.T, db *sql.DB, orgID, caseID uuid.UUID) uuid.UUID {
	t.Helper()
	instanceID := uuid.New()
	wfDefID := uuid.New()
	wfDefVer := 1
	// Create workflow definition
	_, err := db.Exec(`
		INSERT INTO workflow_definitions (id, organization_id, key, name, description, version, status, initial_state, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
	`, wfDefID, orgID, "wf-default", "Default Workflow", "Default workflow for testing", 1, "ACTIVE", "NEW", "{}")
	if err != nil {
		return uuid.Nil
	}
	// Create workflow instance
	_, err = db.Exec(`
		INSERT INTO workflow_instances (id, organization_id, workflow_definition_id, workflow_definition_version, case_id, current_state, started_at, completed_at, metadata, version)
		VALUES ($1, $2, $3, $4, $5, 'DECISION_PENDING', NOW(), NULL, '{}', 1)
	`, instanceID, orgID, wfDefID, wfDefVer, caseID)
	if err != nil {
		return uuid.Nil
	}
	return instanceID
}

func newReviewEntry(orgID, caseID, instanceID uuid.UUID, workflowState string, priority string, ruleEvalIDs []uuid.UUID) *reviewdomain.ReviewQueueEntry {
	entry := reviewdomain.NewReviewQueueEntry(orgID, caseID, instanceID, workflowState, casedomain.Priority(priority), ruleEvalIDs)
	if priority != "" {
		entry.Priority = casedomain.Priority(priority)
	}
	return entry
}

func TestReviewQueueService_CreateAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := setupReviewQueueEnv(t)
	ctx := context.Background()

	caseID := seedCase(t, env.db, env.orgID, env.userID)
	instanceID := seedWorkflowInstance(t, env.db, env.orgID, caseID)

	entry := newReviewEntry(env.orgID, caseID, instanceID, "DECISION_PENDING", "NORMAL", []uuid.UUID{})
	err := env.svc.Save(ctx, entry)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, env.orgID, entry.OrganizationID)
	assert.Equal(t, caseID, entry.CaseID)
	assert.Equal(t, instanceID, entry.WorkflowInstanceID)
	assert.Equal(t, "PENDING", string(entry.Status))
	assert.Equal(t, "NORMAL", string(entry.Priority))
	assert.Equal(t, "DECISION_PENDING", entry.WorkflowState)

	fetched, err := env.svc.GetReview(ctx, env.orgID, entry.ID)
	require.NoError(t, err)
	assert.Equal(t, entry.ID, fetched.ID)
}

func TestReviewQueueService_ClaimAndStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := setupReviewQueueEnv(t)
	ctx := context.Background()

	caseID := seedCase(t, env.db, env.orgID, env.userID)
	instanceID := seedWorkflowInstance(t, env.db, env.orgID, caseID)

	entry := newReviewEntry(env.orgID, caseID, instanceID, "DECISION_PENDING", "HIGH", []uuid.UUID{uuid.New()})
	err := env.svc.Save(ctx, entry)
	require.NoError(t, err)

	claimed, err := env.svc.ClaimReview(ctx, reviewqueueapp.ClaimReviewParams{
		OrganizationID: env.orgID,
		ReviewID:       entry.ID,
		ReviewerID:     env.userID,
	})
	require.NoError(t, err)
	assert.Equal(t, "ASSIGNED", string(claimed.Status))
	assert.Equal(t, &env.userID, claimed.AssignedToID)

	started, err := env.svc.StartReview(ctx, reviewqueueapp.StartReviewParams{
		OrganizationID: env.orgID,
		ReviewID:       entry.ID,
		ReviewerID:     env.userID,
	})
	require.NoError(t, err)
	assert.Equal(t, "IN_REVIEW", string(started.Status))
}

func TestReviewQueueService_CompleteReview(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := setupReviewQueueEnv(t)
	ctx := context.Background()

	caseID := seedCase(t, env.db, env.orgID, env.userID)
	instanceID := seedWorkflowInstance(t, env.db, env.orgID, caseID)

	entry := newReviewEntry(env.orgID, caseID, instanceID, "DECISION_PENDING", "NORMAL", []uuid.UUID{})
	err := env.svc.Save(ctx, entry)
	require.NoError(t, err)

	_, err = env.svc.ClaimReview(ctx, reviewqueueapp.ClaimReviewParams{
		OrganizationID: env.orgID,
		ReviewID:       entry.ID,
		ReviewerID:     env.userID,
	})
	require.NoError(t, err)

	_, err = env.svc.StartReview(ctx, reviewqueueapp.StartReviewParams{
		OrganizationID: env.orgID,
		ReviewID:       entry.ID,
		ReviewerID:     env.userID,
	})
	require.NoError(t, err)

	completed, err := env.svc.CompleteReview(ctx, reviewqueueapp.CompleteReviewParams{
		OrganizationID: env.orgID,
		ReviewID:       entry.ID,
		ReviewerID:     env.userID,
		Decision:       "APPROVED",
		Reason:         "All criteria met",
	})
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", string(completed.Status))
	assert.NotNil(t, completed.CompletedAt)
}

func TestReviewQueueService_EscalateReview(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := setupReviewQueueEnv(t)
	ctx := context.Background()

	caseID := seedCase(t, env.db, env.orgID, env.userID)
	instanceID := seedWorkflowInstance(t, env.db, env.orgID, caseID)

	entry := newReviewEntry(env.orgID, caseID, instanceID, "DECISION_PENDING", "URGENT", []uuid.UUID{})
	err := env.svc.Save(ctx, entry)
	require.NoError(t, err)

	_, err = env.svc.ClaimReview(ctx, reviewqueueapp.ClaimReviewParams{
		OrganizationID: env.orgID,
		ReviewID:       entry.ID,
		ReviewerID:     env.userID,
	})
	require.NoError(t, err)

	_, err = env.svc.StartReview(ctx, reviewqueueapp.StartReviewParams{
		OrganizationID: env.orgID,
		ReviewID:       entry.ID,
		ReviewerID:     env.userID,
	})
	require.NoError(t, err)

	escalated, err := env.svc.EscalateReview(ctx, reviewqueueapp.EscalateReviewParams{
		OrganizationID: env.orgID,
		ReviewID:       entry.ID,
		ReviewerID:     env.userID,
		Reason:         "Requires senior review",
	})
	require.NoError(t, err)
	assert.Equal(t, "ESCALATED", string(escalated.Status))
	assert.NotNil(t, escalated.CompletedAt)
}

func TestReviewQueueService_RequestInformation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := setupReviewQueueEnv(t)
	ctx := context.Background()

	caseID := seedCase(t, env.db, env.orgID, env.userID)
	instanceID := seedWorkflowInstance(t, env.db, env.orgID, caseID)

	entry := newReviewEntry(env.orgID, caseID, instanceID, "DECISION_PENDING", "NORMAL", []uuid.UUID{})
	err := env.svc.Save(ctx, entry)
	require.NoError(t, err)

	_, err = env.svc.ClaimReview(ctx, reviewqueueapp.ClaimReviewParams{
		OrganizationID: env.orgID,
		ReviewID:       entry.ID,
		ReviewerID:     env.userID,
	})
	require.NoError(t, err)

	_, err = env.svc.StartReview(ctx, reviewqueueapp.StartReviewParams{
		OrganizationID: env.orgID,
		ReviewID:       entry.ID,
		ReviewerID:     env.userID,
	})
	require.NoError(t, err)

	waiting, err := env.svc.RequestInformation(ctx, reviewqueueapp.RequestInformationParams{
		OrganizationID: env.orgID,
		ReviewID:       entry.ID,
		ReviewerID:     env.userID,
		MissingFields:  []string{"income_proof"},
		Reason:         "Missing document",
	})
	require.NoError(t, err)
	assert.Equal(t, "WAITING_INFORMATION", string(waiting.Status))
	assert.Equal(t, []string{"income_proof"}, waiting.MissingInformation)
}

func TestReviewQueueService_ListByOrganization(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := setupReviewQueueEnv(t)
	ctx := context.Background()

	caseID := seedCase(t, env.db, env.orgID, env.userID)
	instanceID := seedWorkflowInstance(t, env.db, env.orgID, caseID)

	entry := newReviewEntry(env.orgID, caseID, instanceID, "DECISION_PENDING", "HIGH", []uuid.UUID{})
	err := env.svc.Save(ctx, entry)
	require.NoError(t, err)

	items, total, err := env.svc.GetQueue(ctx, reviewqueueapp.GetQueueParams{
		OrganizationID: env.orgID,
		Limit:          20,
		Offset:         0,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, items, 1)
	assert.Equal(t, "PENDING", string(items[0].Status))
}
