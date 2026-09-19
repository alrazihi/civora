package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/casecontext/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockCaseContextRepo struct {
	mock.Mock
}

func (m *mockCaseContextRepo) DB() *sql.DB { return nil }
func (m *mockCaseContextRepo) Save(ctx context.Context, c *domain.CaseContext) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}
func (m *mockCaseContextRepo) SaveTx(ctx context.Context, tx *sql.Tx, c *domain.CaseContext) error {
	args := m.Called(ctx, tx, c)
	return args.Error(0)
}
func (m *mockCaseContextRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.CaseContext, error) {
	args := m.Called(ctx, orgID, id)
	return args.Get(0).(*domain.CaseContext), args.Error(1)
}
func (m *mockCaseContextRepo) FindLatestByCase(ctx context.Context, orgID, caseID uuid.UUID) (*domain.CaseContext, error) {
	args := m.Called(ctx, orgID, caseID)
	return args.Get(0).(*domain.CaseContext), args.Error(1)
}
func (m *mockCaseContextRepo) DeleteExpired(ctx context.Context, orgID uuid.UUID) (int, error) {
	args := m.Called(ctx, orgID)
	return args.Int(0), args.Error(1)
}

type mockAuthChecker struct {
	mock.Mock
}

func (m *mockAuthChecker) CanAccessCase(ctx context.Context, orgID, caseID, actorID uuid.UUID) (bool, error) {
	args := m.Called(ctx, orgID, caseID, actorID)
	return args.Bool(0), args.Error(1)
}
func (m *mockAuthChecker) CanAccessPerson(ctx context.Context, orgID, personID, actorID uuid.UUID) (bool, error) {
	args := m.Called(ctx, orgID, personID, actorID)
	return args.Bool(0), args.Error(1)
}
func (m *mockAuthChecker) CanAccessEvidence(ctx context.Context, orgID, evidenceID, actorID uuid.UUID) (bool, error) {
	args := m.Called(ctx, orgID, evidenceID, actorID)
	return args.Bool(0), args.Error(1)
}
func (m *mockAuthChecker) CanAccessDocument(ctx context.Context, orgID, documentID, actorID uuid.UUID) (bool, error) {
	args := m.Called(ctx, orgID, documentID, actorID)
	return args.Bool(0), args.Error(1)
}
func (m *mockAuthChecker) CanAccessFormSubmission(ctx context.Context, orgID, submissionID, actorID uuid.UUID) (bool, error) {
	args := m.Called(ctx, orgID, submissionID, actorID)
	return args.Bool(0), args.Error(1)
}
func (m *mockAuthChecker) CanAccessRuleEvaluation(ctx context.Context, orgID, evaluationID, actorID uuid.UUID) (bool, error) {
	args := m.Called(ctx, orgID, evaluationID, actorID)
	return args.Bool(0), args.Error(1)
}
func (m *mockAuthChecker) CanAccessDecision(ctx context.Context, orgID, decisionID, actorID uuid.UUID) (bool, error) {
	args := m.Called(ctx, orgID, decisionID, actorID)
	return args.Bool(0), args.Error(1)
}

type mockCaseRepo struct {
	mock.Mock
}

func (m *mockCaseRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Case, error) {
	args := m.Called(ctx, orgID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Case), args.Error(1)
}

type mockPersonRepo struct {
	mock.Mock
}

func (m *mockPersonRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Person, error) {
	args := m.Called(ctx, orgID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Person), args.Error(1)
}

type mockEvidenceRepo struct {
	mock.Mock
}

func (m *mockEvidenceRepo) FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*domain.Evidence, int, error) {
	args := m.Called(ctx, orgID, caseID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Evidence), args.Int(1), args.Error(2)
}
func (m *mockEvidenceRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Evidence, error) {
	args := m.Called(ctx, orgID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Evidence), args.Error(1)
}
func (m *mockEvidenceRepo) ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*domain.Document, int, error) {
	args := m.Called(ctx, orgID, evidenceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Document), args.Int(1), args.Error(2)
}

type mockFormRepo struct {
	mock.Mock
}

func (m *mockFormRepo) ListByCase(ctx context.Context, orgID, caseID uuid.UUID) ([]*domain.FormSubmission, error) {
	args := m.Called(ctx, orgID, caseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.FormSubmission), args.Error(1)
}
func (m *mockFormRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.FormSubmission, error) {
	args := m.Called(ctx, orgID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FormSubmission), args.Error(1)
}

type mockRuleRepo struct {
	mock.Mock
}

func (m *mockRuleRepo) FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*domain.RuleEvaluation, int, error) {
	args := m.Called(ctx, orgID, caseID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.RuleEvaluation), args.Int(1), args.Error(2)
}
func (m *mockRuleRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.RuleEvaluation, error) {
	args := m.Called(ctx, orgID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RuleEvaluation), args.Error(1)
}

type mockWorkflowRepo struct {
	mock.Mock
}

func (m *mockWorkflowRepo) FindInstanceByCase(ctx context.Context, orgID, caseID uuid.UUID) (*domain.WorkflowInstance, error) {
	args := m.Called(ctx, orgID, caseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.WorkflowInstance), args.Error(1)
}
func (m *mockWorkflowRepo) FindHistoryByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*domain.WorkflowTransitionHistory, int, error) {
	args := m.Called(ctx, orgID, caseID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.WorkflowTransitionHistory), args.Int(1), args.Error(2)
}

type mockDecisionRepo struct {
	mock.Mock
}

func (m *mockDecisionRepo) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*domain.Decision, int, error) {
	args := m.Called(ctx, orgID, serviceRequestID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Decision), args.Int(1), args.Error(2)
}
func (m *mockDecisionRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Decision, error) {
	args := m.Called(ctx, orgID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Decision), args.Error(1)
}

type mockAIObservationRepo struct {
	mock.Mock
}

func (m *mockAIObservationRepo) FindByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*domain.AIObservation, int, error) {
	args := m.Called(ctx, orgID, evidenceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.AIObservation), args.Int(1), args.Error(2)
}

type mockUserChecker struct {
	mock.Mock
}

func (m *mockUserChecker) BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, orgID, userID)
	return args.Bool(0), args.Error(1)
}

func TestBuildContext_Success(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	ctxRepo := new(mockCaseContextRepo)
	authChecker := new(mockAuthChecker)
	caseRepo := new(mockCaseRepo)
	personRepo := new(mockPersonRepo)
	evidenceRepo := new(mockEvidenceRepo)
	formRepo := new(mockFormRepo)
	ruleRepo := new(mockRuleRepo)
	workflowRepo := new(mockWorkflowRepo)
	decisionRepo := new(mockDecisionRepo)
	aiObsRepo := new(mockAIObservationRepo)
	userChecker := new(mockUserChecker)

	svc := NewCaseContextService(
		ctxRepo, authChecker, caseRepo, personRepo,
		evidenceRepo, formRepo, ruleRepo, workflowRepo,
		decisionRepo, aiObsRepo, userChecker,
	)

	validCase := &domain.Case{
		ID:             caseID,
		OrganizationID: orgID,
		CaseNumber:     "CAS-20240101-00000001",
		Title:          "Test Case",
		Description:    "Test Description",
		Status:         "OPEN",
		WorkflowState:  "OPEN",
		ServiceType:    "GENERAL",
		Priority:       "NORMAL",
		PersonID:       nil,
		AssignedToID:   nil,
		CreatedByID:    actorID,
		Version:        1,
	}

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, actorID).Return(true, nil)
	authChecker.On("CanAccessCase", mock.Anything, orgID, caseID, actorID).Return(true, nil)
	caseRepo.On("FindByID", mock.Anything, orgID, caseID).Return(validCase, nil)
	evidenceRepo.On("FindByCase", mock.Anything, orgID, caseID, 100, 0).Return([]*domain.Evidence{}, 0, nil)
	formRepo.On("ListByCase", mock.Anything, orgID, caseID).Return([]*domain.FormSubmission{}, nil)
	ruleRepo.On("FindByCase", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.RuleEvaluation{}, 0, nil)
	workflowRepo.On("FindInstanceByCase", mock.Anything, orgID, caseID).Return(nil, errors.New("not found"))
	workflowRepo.On("FindHistoryByCase", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.WorkflowTransitionHistory{}, 0, nil)
	decisionRepo.On("FindByServiceRequest", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.Decision{}, 0, nil)
	ctxRepo.On("Save", mock.Anything, mock.AnythingOfType("*domain.CaseContext")).Return(nil)

	options := domain.DefaultContextOptions()
	options.IncludePerson = false
	options.IncludeEvidence = false
	options.IncludeDocuments = false
	options.IncludeFormSubmissions = false
	options.IncludeRuleEvaluations = false
	options.IncludeWorkflowHistory = false
	options.IncludeDecisions = false
	options.IncludeAIObservations = false

	result, err := svc.BuildContext(context.Background(), BuildContextParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Options:        options,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Context)
	assert.Equal(t, orgID, result.Context.OrganizationID)
	assert.Equal(t, caseID, result.Context.CaseID)
	assert.Equal(t, actorID, result.Context.ActorID)
	assert.Greater(t, len(result.Context.Facts), 0)
	assert.Contains(t, result.Context.Authorization, "case")

	userChecker.AssertExpectations(t)
	authChecker.AssertExpectations(t)
	caseRepo.AssertExpectations(t)
	ctxRepo.AssertExpectations(t)
}

func TestBuildContext_ActorNotInOrg(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	ctxRepo := new(mockCaseContextRepo)
	authChecker := new(mockAuthChecker)
	caseRepo := new(mockCaseRepo)
	personRepo := new(mockPersonRepo)
	evidenceRepo := new(mockEvidenceRepo)
	formRepo := new(mockFormRepo)
	ruleRepo := new(mockRuleRepo)
	workflowRepo := new(mockWorkflowRepo)
	decisionRepo := new(mockDecisionRepo)
	aiObsRepo := new(mockAIObservationRepo)
	userChecker := new(mockUserChecker)

	svc := NewCaseContextService(
		ctxRepo, authChecker, caseRepo, personRepo,
		evidenceRepo, formRepo, ruleRepo, workflowRepo,
		decisionRepo, aiObsRepo, userChecker,
	)

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, actorID).Return(false, nil)

	_, err := svc.BuildContext(context.Background(), BuildContextParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Options:        domain.DefaultContextOptions(),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user does not belong to organization")
}

func TestBuildContext_CaseAccessDenied(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	ctxRepo := new(mockCaseContextRepo)
	authChecker := new(mockAuthChecker)
	caseRepo := new(mockCaseRepo)
	personRepo := new(mockPersonRepo)
	evidenceRepo := new(mockEvidenceRepo)
	formRepo := new(mockFormRepo)
	ruleRepo := new(mockRuleRepo)
	workflowRepo := new(mockWorkflowRepo)
	decisionRepo := new(mockDecisionRepo)
	aiObsRepo := new(mockAIObservationRepo)
	userChecker := new(mockUserChecker)

	svc := NewCaseContextService(
		ctxRepo, authChecker, caseRepo, personRepo,
		evidenceRepo, formRepo, ruleRepo, workflowRepo,
		decisionRepo, aiObsRepo, userChecker,
	)

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, actorID).Return(true, nil)
	authChecker.On("CanAccessCase", mock.Anything, orgID, caseID, actorID).Return(false, nil)

	_, err := svc.BuildContext(context.Background(), BuildContextParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Options:        domain.DefaultContextOptions(),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot access case")
}

func TestBuildContext_NilActorID(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()

	ctxRepo := new(mockCaseContextRepo)
	authChecker := new(mockAuthChecker)
	caseRepo := new(mockCaseRepo)
	personRepo := new(mockPersonRepo)
	evidenceRepo := new(mockEvidenceRepo)
	formRepo := new(mockFormRepo)
	ruleRepo := new(mockRuleRepo)
	workflowRepo := new(mockWorkflowRepo)
	decisionRepo := new(mockDecisionRepo)
	aiObsRepo := new(mockAIObservationRepo)
	userChecker := new(mockUserChecker)

	svc := NewCaseContextService(
		ctxRepo, authChecker, caseRepo, personRepo,
		evidenceRepo, formRepo, ruleRepo, workflowRepo,
		decisionRepo, aiObsRepo, userChecker,
	)

	_, err := svc.BuildContext(context.Background(), BuildContextParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        uuid.Nil,
		Options:        domain.DefaultContextOptions(),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "actor ID is required")
}

func TestBuildContext_WithPerson(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()
	personID := uuid.New()

	ctxRepo := new(mockCaseContextRepo)
	authChecker := new(mockAuthChecker)
	caseRepo := new(mockCaseRepo)
	personRepo := new(mockPersonRepo)
	evidenceRepo := new(mockEvidenceRepo)
	formRepo := new(mockFormRepo)
	ruleRepo := new(mockRuleRepo)
	workflowRepo := new(mockWorkflowRepo)
	decisionRepo := new(mockDecisionRepo)
	aiObsRepo := new(mockAIObservationRepo)
	userChecker := new(mockUserChecker)

	svc := NewCaseContextService(
		ctxRepo, authChecker, caseRepo, personRepo,
		evidenceRepo, formRepo, ruleRepo, workflowRepo,
		decisionRepo, aiObsRepo, userChecker,
	)

	validCase := &domain.Case{
		ID:             caseID,
		OrganizationID: orgID,
		CaseNumber:     "CAS-20240101-00000001",
		Title:          "Test Case",
		Description:    "Test Description",
		Status:         "OPEN",
		WorkflowState:  "OPEN",
		ServiceType:    "GENERAL",
		Priority:       "NORMAL",
		PersonID:       &personID,
		AssignedToID:   nil,
		CreatedByID:    actorID,
		Version:        1,
	}

	validPerson := &domain.Person{
		ID:             personID,
		OrganizationID: orgID,
		LegalName:      "John Doe",
		Gender:         "M",
		Nationality:    "US",
		Address:        "123 Main St",
		Phone:          "555-1234",
		Email:          "john@example.com",
	}

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, actorID).Return(true, nil)
	authChecker.On("CanAccessCase", mock.Anything, orgID, caseID, actorID).Return(true, nil)
	authChecker.On("CanAccessPerson", mock.Anything, orgID, personID, actorID).Return(true, nil)
	caseRepo.On("FindByID", mock.Anything, orgID, caseID).Return(validCase, nil)
	personRepo.On("FindByID", mock.Anything, orgID, personID).Return(validPerson, nil)
	evidenceRepo.On("FindByCase", mock.Anything, orgID, caseID, 100, 0).Return([]*domain.Evidence{}, 0, nil)
	formRepo.On("ListByCase", mock.Anything, orgID, caseID).Return([]*domain.FormSubmission{}, nil)
	ruleRepo.On("FindByCase", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.RuleEvaluation{}, 0, nil)
	workflowRepo.On("FindInstanceByCase", mock.Anything, orgID, caseID).Return(nil, errors.New("not found"))
	workflowRepo.On("FindHistoryByCase", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.WorkflowTransitionHistory{}, 0, nil)
	decisionRepo.On("FindByServiceRequest", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.Decision{}, 0, nil)
	ctxRepo.On("Save", mock.Anything, mock.AnythingOfType("*domain.CaseContext")).Return(nil)

	options := domain.DefaultContextOptions()
	options.IncludePerson = true
	options.IncludeEvidence = false
	options.IncludeDocuments = false
	options.IncludeFormSubmissions = false
	options.IncludeRuleEvaluations = false
	options.IncludeWorkflowHistory = false
	options.IncludeDecisions = false
	options.IncludeAIObservations = false

	result, err := svc.BuildContext(context.Background(), BuildContextParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Options:        options,
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	foundPersonFact := false
	for _, fact := range result.Context.Facts {
		if fact.SourceType == domain.FactSourcePerson {
			foundPersonFact = true
			assert.Equal(t, domain.ProvenanceSourceFact, fact.Provenance)
			assert.Equal(t, "person", fact.Key)
			break
		}
	}
	assert.True(t, foundPersonFact, "person fact should be included")
}

func TestBuildContext_WithEvidence(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()
	evidenceID := uuid.New()

	ctxRepo := new(mockCaseContextRepo)
	authChecker := new(mockAuthChecker)
	caseRepo := new(mockCaseRepo)
	personRepo := new(mockPersonRepo)
	evidenceRepo := new(mockEvidenceRepo)
	formRepo := new(mockFormRepo)
	ruleRepo := new(mockRuleRepo)
	workflowRepo := new(mockWorkflowRepo)
	decisionRepo := new(mockDecisionRepo)
	aiObsRepo := new(mockAIObservationRepo)
	userChecker := new(mockUserChecker)

	svc := NewCaseContextService(
		ctxRepo, authChecker, caseRepo, personRepo,
		evidenceRepo, formRepo, ruleRepo, workflowRepo,
		decisionRepo, aiObsRepo, userChecker,
	)

	validCase := &domain.Case{
		ID:             caseID,
		OrganizationID: orgID,
		CaseNumber:     "CAS-20240101-00000001",
		Title:          "Test Case",
		Description:    "Test Description",
		Status:         "OPEN",
		WorkflowState:  "OPEN",
		ServiceType:    "GENERAL",
		Priority:       "NORMAL",
		PersonID:       nil,
		AssignedToID:   nil,
		CreatedByID:    actorID,
		Version:        1,
	}

	validEvidence := &domain.Evidence{
		ID:                 evidenceID,
		OrganizationID:     orgID,
		ServiceRequestID:   caseID,
		Type:               "IDENTITY_DOCUMENT",
		Description:        "Passport",
		VerificationStatus: "VERIFIED",
		VerifiedBy:         &actorID,
		CreatedAt:          time.Now().UTC(),
	}

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, actorID).Return(true, nil)
	authChecker.On("CanAccessCase", mock.Anything, orgID, caseID, actorID).Return(true, nil)
	authChecker.On("CanAccessEvidence", mock.Anything, orgID, evidenceID, actorID).Return(true, nil)
	caseRepo.On("FindByID", mock.Anything, orgID, caseID).Return(validCase, nil)
	evidenceRepo.On("FindByCase", mock.Anything, orgID, caseID, 100, 0).Return([]*domain.Evidence{validEvidence}, 1, nil)
	evidenceRepo.On("ListDocuments", mock.Anything, orgID, evidenceID, 20, 0).Return([]*domain.Document{}, 0, nil)
	formRepo.On("ListByCase", mock.Anything, orgID, caseID).Return([]*domain.FormSubmission{}, nil)
	ruleRepo.On("FindByCase", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.RuleEvaluation{}, 0, nil)
	workflowRepo.On("FindInstanceByCase", mock.Anything, orgID, caseID).Return(nil, errors.New("not found"))
	workflowRepo.On("FindHistoryByCase", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.WorkflowTransitionHistory{}, 0, nil)
	decisionRepo.On("FindByServiceRequest", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.Decision{}, 0, nil)
	ctxRepo.On("Save", mock.Anything, mock.AnythingOfType("*domain.CaseContext")).Return(nil)

	options := domain.DefaultContextOptions()
	options.IncludePerson = false
	options.IncludeEvidence = true
	options.IncludeDocuments = false
	options.IncludeFormSubmissions = false
	options.IncludeRuleEvaluations = false
	options.IncludeWorkflowHistory = false
	options.IncludeDecisions = false
	options.IncludeAIObservations = false

	result, err := svc.BuildContext(context.Background(), BuildContextParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Options:        options,
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	foundEvidenceFact := false
	for _, fact := range result.Context.Facts {
		if fact.SourceType == domain.FactSourceEvidence && fact.SourceID == evidenceID {
			foundEvidenceFact = true
			assert.Equal(t, domain.ProvenanceHumanVerified, fact.Provenance)
			break
		}
	}
	assert.True(t, foundEvidenceFact, "verified evidence fact should be included with HUMAN_VERIFIED provenance")
}

func TestBuildContext_OnlyVerifiedEvidence(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()
	verifiedEvidenceID := uuid.New()
	unverifiedEvidenceID := uuid.New()

	ctxRepo := new(mockCaseContextRepo)
	authChecker := new(mockAuthChecker)
	caseRepo := new(mockCaseRepo)
	personRepo := new(mockPersonRepo)
	evidenceRepo := new(mockEvidenceRepo)
	formRepo := new(mockFormRepo)
	ruleRepo := new(mockRuleRepo)
	workflowRepo := new(mockWorkflowRepo)
	decisionRepo := new(mockDecisionRepo)
	aiObsRepo := new(mockAIObservationRepo)
	userChecker := new(mockUserChecker)

	svc := NewCaseContextService(
		ctxRepo, authChecker, caseRepo, personRepo,
		evidenceRepo, formRepo, ruleRepo, workflowRepo,
		decisionRepo, aiObsRepo, userChecker,
	)

	validCase := &domain.Case{
		ID:             caseID,
		OrganizationID: orgID,
		CaseNumber:     "CAS-20240101-00000001",
		Title:          "Test Case",
		Description:    "Test Description",
		Status:         "OPEN",
		WorkflowState:  "OPEN",
		ServiceType:    "GENERAL",
		Priority:       "NORMAL",
		PersonID:       nil,
		AssignedToID:   nil,
		CreatedByID:    actorID,
		Version:        1,
	}

	verifiedEvidence := &domain.Evidence{
		ID:                 verifiedEvidenceID,
		OrganizationID:     orgID,
		ServiceRequestID:   caseID,
		Type:               "IDENTITY_DOCUMENT",
		Description:        "Verified Passport",
		VerificationStatus: "VERIFIED",
	}

	unverifiedEvidence := &domain.Evidence{
		ID:                 unverifiedEvidenceID,
		OrganizationID:     orgID,
		ServiceRequestID:   caseID,
		Type:               "SUPPORTING_DOCUMENT",
		Description:        "Unverified Document",
		VerificationStatus: "UNVERIFIED",
	}

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, actorID).Return(true, nil)
	authChecker.On("CanAccessCase", mock.Anything, orgID, caseID, actorID).Return(true, nil)
	authChecker.On("CanAccessEvidence", mock.Anything, orgID, verifiedEvidenceID, actorID).Return(true, nil)
	authChecker.On("CanAccessEvidence", mock.Anything, orgID, unverifiedEvidenceID, actorID).Return(true, nil)
	caseRepo.On("FindByID", mock.Anything, orgID, caseID).Return(validCase, nil)
	evidenceRepo.On("FindByCase", mock.Anything, orgID, caseID, 100, 0).Return(
		[]*domain.Evidence{verifiedEvidence, unverifiedEvidence}, 2, nil,
	)
	evidenceRepo.On("ListDocuments", mock.Anything, orgID, mock.Anything, 20, 0).Return([]*domain.Document{}, 0, nil)
	formRepo.On("ListByCase", mock.Anything, orgID, caseID).Return([]*domain.FormSubmission{}, nil)
	ruleRepo.On("FindByCase", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.RuleEvaluation{}, 0, nil)
	workflowRepo.On("FindInstanceByCase", mock.Anything, orgID, caseID).Return(nil, errors.New("not found"))
	workflowRepo.On("FindHistoryByCase", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.WorkflowTransitionHistory{}, 0, nil)
	decisionRepo.On("FindByServiceRequest", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.Decision{}, 0, nil)
	ctxRepo.On("Save", mock.Anything, mock.AnythingOfType("*domain.CaseContext")).Return(nil)

	options := domain.DefaultContextOptions()
	options.IncludePerson = false
	options.IncludeEvidence = true
	options.IncludeVerifiedEvidence = true
	options.OnlyVerified = true
	options.IncludeDocuments = false
	options.IncludeFormSubmissions = false
	options.IncludeRuleEvaluations = false
	options.IncludeWorkflowHistory = false
	options.IncludeDecisions = false
	options.IncludeAIObservations = false

	result, err := svc.BuildContext(context.Background(), BuildContextParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Options:        options,
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	evidenceCount := 0
	for _, fact := range result.Context.Facts {
		if fact.SourceType == domain.FactSourceEvidence {
			evidenceCount++
			assert.Equal(t, verifiedEvidenceID, fact.SourceID)
		}
	}
	assert.Equal(t, 1, evidenceCount, "only verified evidence should be included")
}

func TestBuildContext_PersonAccessDenied(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()
	personID := uuid.New()

	ctxRepo := new(mockCaseContextRepo)
	authChecker := new(mockAuthChecker)
	caseRepo := new(mockCaseRepo)
	personRepo := new(mockPersonRepo)
	evidenceRepo := new(mockEvidenceRepo)
	formRepo := new(mockFormRepo)
	ruleRepo := new(mockRuleRepo)
	workflowRepo := new(mockWorkflowRepo)
	decisionRepo := new(mockDecisionRepo)
	aiObsRepo := new(mockAIObservationRepo)
	userChecker := new(mockUserChecker)

	svc := NewCaseContextService(
		ctxRepo, authChecker, caseRepo, personRepo,
		evidenceRepo, formRepo, ruleRepo, workflowRepo,
		decisionRepo, aiObsRepo, userChecker,
	)

	validCase := &domain.Case{
		ID:             caseID,
		OrganizationID: orgID,
		CaseNumber:     "CAS-20240101-00000001",
		Title:          "Test Case",
		Description:    "Test Description",
		Status:         "OPEN",
		WorkflowState:  "OPEN",
		ServiceType:    "GENERAL",
		Priority:       "NORMAL",
		PersonID:       &personID,
		AssignedToID:   nil,
		CreatedByID:    actorID,
		Version:        1,
	}

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, actorID).Return(true, nil)
	authChecker.On("CanAccessCase", mock.Anything, orgID, caseID, actorID).Return(true, nil)
	authChecker.On("CanAccessPerson", mock.Anything, orgID, personID, actorID).Return(false, nil)
	caseRepo.On("FindByID", mock.Anything, orgID, caseID).Return(validCase, nil)
	evidenceRepo.On("FindByCase", mock.Anything, orgID, caseID, 100, 0).Return([]*domain.Evidence{}, 0, nil)
	formRepo.On("ListByCase", mock.Anything, orgID, caseID).Return([]*domain.FormSubmission{}, nil)
	ruleRepo.On("FindByCase", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.RuleEvaluation{}, 0, nil)
	workflowRepo.On("FindInstanceByCase", mock.Anything, orgID, caseID).Return(nil, errors.New("not found"))
	workflowRepo.On("FindHistoryByCase", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.WorkflowTransitionHistory{}, 0, nil)
	decisionRepo.On("FindByServiceRequest", mock.Anything, orgID, caseID, 50, 0).Return([]*domain.Decision{}, 0, nil)
	ctxRepo.On("Save", mock.Anything, mock.AnythingOfType("*domain.CaseContext")).Return(nil)

	options := domain.DefaultContextOptions()
	options.IncludePerson = true
	options.IncludeEvidence = false
	options.IncludeDocuments = false
	options.IncludeFormSubmissions = false
	options.IncludeRuleEvaluations = false
	options.IncludeWorkflowHistory = false
	options.IncludeDecisions = false
	options.IncludeAIObservations = false

	result, err := svc.BuildContext(context.Background(), BuildContextParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Options:        options,
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	foundPersonFact := false
	for _, fact := range result.Context.Facts {
		if fact.SourceType == domain.FactSourcePerson {
			foundPersonFact = true
		}
	}
	assert.False(t, foundPersonFact, "person fact should not be included when access denied")
}
