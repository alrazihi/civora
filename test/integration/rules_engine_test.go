package integration

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	rulesapp "github.com/alrazihi/civora/internal/rules/application"
	rulesdomain "github.com/alrazihi/civora/internal/rules/domain"
	rulesinfra "github.com/alrazihi/civora/internal/rules/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRulesServices(t *testing.T) (*rulesapp.RuleSetService, uuid.UUID, uuid.UUID) {
	t.Helper()
	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	orgID := helpers.SeedOrg(db)
	actorID := helpers.SeedUser(db, orgID)

	auditRepo := auditpostgres.NewPostgresAuditRepository(db)
	auditService := auditapp.NewAuditService(auditRepo, config.AuditConfig{Enabled: true})

	ruleSetRepo := rulesinfra.NewPostgresRuleSetRepository(db)
	evalRepo := rulesinfra.NewPostgresEvaluationRepository(db)
	templateRepo := rulesinfra.NewRuleTemplateRepository(db)

	svc := rulesapp.NewRuleSetService(ruleSetRepo, evalRepo, templateRepo, nil, auditService)
	return svc, orgID, actorID
}

func TestRuleSetCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc, orgID, actorID := setupRulesServices(t)
	ctx := context.Background()

	rs, err := svc.CreateRuleSet(ctx, rulesapp.CreateRuleSetParams{
		OrganizationID: orgID,
		Key:            "income-eligibility",
		Name:           "Income Eligibility",
		Description:    "Test rule set",
		DefaultOutcome: rulesdomain.OutcomeRequiresReview,
		Rules: []rulesdomain.Rule{
			{
				Priority:   0,
				Outcome:    rulesdomain.OutcomeEligible,
				Conditions: rulesdomain.Condition{Field: "form.income_verification.amount", Operator: rulesdomain.OpLessThanEqual, Value: json.Number("500")},
			},
		},
		Triggers: []string{"form_submitted"},
		ActorID:  actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, rulesdomain.StatusDraft, rs.Status)
	assert.Equal(t, 1, rs.Version)
	assert.Len(t, rs.Triggers, 1)
	assert.Equal(t, "form_submitted", rs.Triggers[0])

	fetched, err := svc.GetRuleSet(ctx, orgID, rs.ID)
	require.NoError(t, err)
	assert.Equal(t, rs.ID, fetched.ID)
	assert.Len(t, fetched.Triggers, 1)
	assert.Equal(t, rs.Key, fetched.Key)

	updated, err := svc.UpdateRuleSet(ctx, rulesapp.UpdateRuleSetParams{
		OrganizationID: orgID,
		ID:             rs.ID,
		Name:           "Income Eligibility Updated",
		Description:    "Updated description",
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated description", updated.Description)

	published, err := svc.PublishRuleSet(ctx, orgID, rs.ID, actorID)
	require.NoError(t, err)
	assert.Equal(t, rulesdomain.StatusPublished, published.Status)

	newVer, err := svc.CreateVersion(ctx, orgID, rs.ID, actorID)
	require.NoError(t, err)
	assert.Equal(t, 2, newVer.Version)
	assert.Equal(t, rulesdomain.StatusDraft, newVer.Status)

	archived, err := svc.ArchiveRuleSet(ctx, orgID, published.ID, actorID)
	require.NoError(t, err)
	assert.Equal(t, rulesdomain.StatusArchived, archived.Status)

	err = svc.DeleteRuleSet(ctx, orgID, newVer.ID, actorID)
	require.NoError(t, err)

	archivedSet, err := svc.GetRuleSet(ctx, orgID, rs.ID)
	require.NoError(t, err)
	assert.Equal(t, rulesdomain.StatusArchived, archivedSet.Status)

	_, err = svc.GetRuleSet(ctx, orgID, newVer.ID)
	assert.Error(t, err)
}

func TestRuleSetTenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc, org1, actorID := setupRulesServices(t)
	ctx := context.Background()

	org2 := helpers.SeedOrg(helpers.TestDB(t))

	_, err := svc.CreateRuleSet(ctx, rulesapp.CreateRuleSetParams{
		OrganizationID: org1,
		Key:            "tenant-isolated",
		Name:           "Tenant Isolated",
		Description:    "Test",
		DefaultOutcome: rulesdomain.OutcomeRequiresReview,
		ActorID:        actorID,
	})
	require.NoError(t, err)

	_, err = svc.GetRuleSet(ctx, org2, uuid.New())
	assert.Error(t, err)

	results, _, err := svc.ListRuleSets(ctx, rulesapp.ListRuleSetsParams{
		OrganizationID: org2,
		Limit:          10,
		Offset:         0,
	})
	require.NoError(t, err)
	assert.Empty(t, results, "tenant B must not see tenant A's rule sets")
}

func TestEvaluateRuleSetEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc, orgID, actorID := setupRulesServices(t)
	ctx := context.Background()

	rs, err := svc.CreateRuleSet(ctx, rulesapp.CreateRuleSetParams{
		OrganizationID: orgID,
		Key:            "income-check",
		Name:           "Income Check",
		Description:    "Test",
		DefaultOutcome: rulesdomain.OutcomeIneligible,
		Rules: []rulesdomain.Rule{
			{
				Priority:   0,
				Outcome:    rulesdomain.OutcomeEligible,
				Conditions: rulesdomain.Condition{Field: "form.income.amount", Operator: rulesdomain.OpLessThanEqual, Value: json.Number("500")},
			},
		},
		ActorID: actorID,
	})
	require.NoError(t, err)

	published, err := svc.PublishRuleSet(ctx, orgID, rs.ID, actorID)
	require.NoError(t, err)

	ev, err := svc.EvaluateRuleSet(ctx, rulesapp.EvaluateRuleSetParams{
		OrganizationID: orgID,
		ID:             published.ID,
		Facts: map[string]interface{}{
			"form": map[string]interface{}{
				"income": map[string]interface{}{
					"amount": json.Number("300"),
				},
			},
		},
		Trigger: rulesdomain.TriggerManual,
		ActorID: &actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, rulesdomain.OutcomeEligible, ev.Outcome)

	evs, _, err := svc.ListEvaluationsByRuleSet(ctx, orgID, published.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, evs, 1)
	assert.Equal(t, ev.ID, evs[0].ID)
}

func TestEvaluateRuleSetInvalidRuleSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc, orgID, actorID := setupRulesServices(t)
	ctx := context.Background()

	rs, err := svc.CreateRuleSet(ctx, rulesapp.CreateRuleSetParams{
		OrganizationID: orgID,
		Key:            "bad-rules",
		Name:           "Bad Rules",
		Description:    "Test",
		DefaultOutcome: rulesdomain.OutcomeIneligible,
		Rules: []rulesdomain.Rule{
			{
				Priority:   0,
				Outcome:    "INVALID_OUTCOME",
				Conditions: rulesdomain.Condition{Field: "form.income.amount", Operator: rulesdomain.OpLessThanEqual, Value: json.Number("500")},
			},
		},
		ActorID: actorID,
	})
	require.Error(t, err, "rule set with invalid outcome should be rejected at creation")
	assert.Nil(t, rs)
}

func TestConcurrentRuleSetCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc, orgID, actorID := setupRulesServices(t)
	ctx := context.Background()

	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = svc.CreateRuleSet(ctx, rulesapp.CreateRuleSetParams{
				OrganizationID: orgID,
				Key:            "concurrent-key",
				Name:           "Concurrent",
				Description:    "Test",
				DefaultOutcome: rulesdomain.OutcomeRequiresReview,
				Rules:          []rulesdomain.Rule{},
				ActorID:        actorID,
			})
		}(i)
	}
	wg.Wait()

	success := 0
	dupes := 0
	for _, err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, rulesapp.ErrRuleSetKeyExists) {
			dupes++
		}
	}
	assert.Equal(t, 1, success, "exactly one concurrent create should succeed")
	assert.GreaterOrEqual(t, dupes, 1, "remaining concurrent creates must report duplicate key")
}

func TestRuleSetTemporalOperators(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	svc, orgID, actorID := setupRulesServices(t)
	ctx := context.Background()

	rs, err := svc.CreateRuleSet(ctx, rulesapp.CreateRuleSetParams{
		OrganizationID: orgID,
		Key:            "temporal-eligibility",
		Name:           "Temporal Eligibility",
		Description:    "Test",
		DefaultOutcome: rulesdomain.OutcomeIneligible,
		Rules: []rulesdomain.Rule{
			{
				Priority:   0,
				Outcome:    rulesdomain.OutcomeEligible,
				Conditions: rulesdomain.Condition{Field: "form.due_date", Operator: rulesdomain.OpBefore, Value: "2025-12-31"},
			},
		},
		Triggers: []string{"form_submitted"},
		ActorID:  actorID,
	})
	require.NoError(t, err)

	published, err := svc.PublishRuleSet(ctx, orgID, rs.ID, actorID)
	require.NoError(t, err)

	// Malformed temporal value must be rejected at validation time, not stored.
	_, err = svc.CreateRuleSet(ctx, rulesapp.CreateRuleSetParams{
		OrganizationID: orgID,
		Key:            "temporal-bad",
		Name:           "Temporal Bad",
		Description:    "Test",
		DefaultOutcome: rulesdomain.OutcomeIneligible,
		Rules: []rulesdomain.Rule{
			{
				Priority:   0,
				Outcome:    rulesdomain.OutcomeEligible,
				Conditions: rulesdomain.Condition{Field: "form.due_date", Operator: rulesdomain.OpBefore, Value: "not-a-date"},
			},
		},
		ActorID: actorID,
	})
	require.Error(t, err, "malformed temporal value must be rejected at creation")

	// Evaluate with a matching fact.
	ev, err := svc.EvaluateRuleSet(ctx, rulesapp.EvaluateRuleSetParams{
		OrganizationID: orgID,
		ID:             published.ID,
		Facts: map[string]interface{}{
			"form": map[string]interface{}{
				"due_date": "2025-06-15",
			},
		},
		Trigger: rulesdomain.TriggerManual,
	})
	require.NoError(t, err)
	assert.Equal(t, rulesdomain.StatusEligible, ev.Status)
	assert.Equal(t, rulesdomain.OutcomeEligible, ev.Outcome)
}
