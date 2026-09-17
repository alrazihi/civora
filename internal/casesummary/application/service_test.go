package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/casecontext/application"
	casecontextdomain "github.com/alrazihi/civora/internal/casecontext/domain"
	casesdomain "github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/casesummary/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

type mockSummaryRepository struct {
	summaries map[string]*domain.CaseSummary
	saveFunc  func(ctx context.Context, s *domain.CaseSummary) error
}

func (m *mockSummaryRepository) Save(ctx context.Context, s *domain.CaseSummary) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, s)
	}
	m.summaries[s.ID.String()] = s
	return nil
}

func (m *mockSummaryRepository) SaveTx(ctx context.Context, tx *sql.Tx, s *domain.CaseSummary) error {
	return m.Save(ctx, s)
}

func (m *mockSummaryRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.CaseSummary, error) {
	if s, ok := m.summaries[id.String()]; ok {
		return s, nil
	}
	return nil, domain.ErrSummaryNotFound
}

func (m *mockSummaryRepository) FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*domain.CaseSummary, int, error) {
	d := make(map[string]*domain.CaseSummary)
	for _, s := range m.summaries {
		if s.CaseID == caseID {
			d[s.ID.String()] = s
		}
	}
	result := make([]*domain.CaseSummary, 0, len(d))
	for _, s := range d {
		result = append(result, s)
	}
	return result, len(result), nil
}

func (m *mockSummaryRepository) UpdateStatus(ctx context.Context, orgID, summaryID uuid.UUID, status domain.SummaryStatus, reviewerID *uuid.UUID, notes string) error {
	if s, err := m.FindByID(ctx, orgID, summaryID); err == nil {
		s.Status = status
		s.ReviewedAt = ptrTime(time.Now())
		s.ReviewedBy = reviewerID
		s.ReviewNotes = notes
		return nil
	}
	return domain.ErrSummaryNotFound
}

func (m *mockSummaryRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, summaryID uuid.UUID, status domain.SummaryStatus, reviewerID *uuid.UUID, notes string) error {
	return m.UpdateStatus(ctx, orgID, summaryID, status, reviewerID, notes)
}

func (m *mockSummaryRepository) CountByCase(ctx context.Context, orgID, caseID uuid.UUID) (int, error) {
	count := 0
	for _, s := range m.summaries {
		if s.CaseID == caseID {
			count++
		}
	}
	return count, nil
}

type mockCaseFinder struct {
	cases map[uuid.UUID]*casesdomain.Case
}

func (m *mockCaseFinder) FindByID(ctx context.Context, orgID, id uuid.UUID) (*casesdomain.Case, error) {
	if c, ok := m.cases[id]; ok {
		return c, nil
	}
	return nil, casesdomain.ErrCaseNotFound
}

var _ shared.CaseFinder = (*mockCaseFinder)(nil)

type mockUserChecker struct {
	orgs map[uuid.UUID]map[uuid.UUID]bool
}

func (m *mockUserChecker) BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	if orgs, ok := m.orgs[orgID]; ok {
		return orgs[userID], nil
	}
	return false, nil
}

type mockContextService struct {
	context *casecontextdomain.CaseContext
	error   error
}

func (m *mockContextService) BuildContext(ctx context.Context, params application.BuildContextParams) (*application.BuildContextResult, error) {
	if m.error != nil {
		return nil, m.error
	}
	return &application.BuildContextResult{Context: m.context}, nil
}

type mockProvider struct {
	result *CaseSummaryResult
	err    error
	info   domain.ModelInfo
}

func (m *mockProvider) GenerateCaseSummary(ctx context.Context, req CaseSummaryRequest) (*CaseSummaryResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockProvider) ProviderInfo() domain.ModelInfo {
	return m.info
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

var _ shared.CaseFinder = (*mockCaseFinder)(nil)
var _ shared.UserChecker = (*mockUserChecker)(nil)
var _ CaseSummaryProvider = (*mockProvider)(nil)

func TestGenerateCaseSummary_Success(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	repo := &mockSummaryRepository{summaries: make(map[string]*domain.CaseSummary)}
	caseFinder := &mockCaseFinder{cases: map[uuid.UUID]*casesdomain.Case{caseID: {ID: caseID, OrganizationID: orgID}}}
	userChecker := &mockUserChecker{orgs: map[uuid.UUID]map[uuid.UUID]bool{orgID: {actorID: true}}}
	auditor := &mockAuditRecorder{}
	contextSvc := &mockContextService{
		context: &casecontextdomain.CaseContext{
			ID:             uuid.New(),
			OrganizationID: orgID,
			CaseID:         caseID,
			ActorID:        actorID,
			Facts:          []*casecontextdomain.Fact{},
		},
	}

	svc := NewCaseSummaryService(repo, caseFinder, contextSvc, userChecker, auditor).
		WithProvider(&mockProvider{
			result: &CaseSummaryResult{
				Content: domain.SummaryContent{},
				Model: domain.ModelInfo{
					Name:     "gpt-4o-mini",
					Version:  "1.0",
					Provider: "openai",
				},
			},
		}).
		WithTransaction(nil)

	_, err := svc.GenerateCaseSummary(context.Background(), GenerateCaseSummaryParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestGenerateCaseSummary_ActorNotInOrg(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	repo := &mockSummaryRepository{summaries: make(map[string]*domain.CaseSummary)}
	caseFinder := &mockCaseFinder{cases: map[uuid.UUID]*casesdomain.Case{caseID: {ID: caseID, OrganizationID: orgID}}}
	userChecker := &mockUserChecker{orgs: map[uuid.UUID]map[uuid.UUID]bool{orgID: {}}} // actor not in org
	auditor := &mockAuditRecorder{}

	svc := NewCaseSummaryService(repo, caseFinder, &mockContextService{}, userChecker, auditor).
		WithProvider(&mockProvider{}).
		WithTransaction(nil)

	_, err := svc.GenerateCaseSummary(context.Background(), GenerateCaseSummaryParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
	})
	if err == nil {
		t.Fatal("expected error for actor not in organization")
	}
}

type testCaseSummaryProvider interface{}

func TestGenerateCaseSummary_ProviderError(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	repo := &mockSummaryRepository{summaries: make(map[string]*domain.CaseSummary)}
	caseFinder := &mockCaseFinder{cases: map[uuid.UUID]*casesdomain.Case{caseID: {ID: caseID, OrganizationID: orgID}}}
	userChecker := &mockUserChecker{orgs: map[uuid.UUID]map[uuid.UUID]bool{orgID: {actorID: true}}}
	auditor := &mockAuditRecorder{}
	contextSvc := &mockContextService{
		context: &casecontextdomain.CaseContext{
			ID:             uuid.New(),
			OrganizationID: orgID,
			CaseID:         caseID,
		},
	}

	svc := NewCaseSummaryService(repo, caseFinder, contextSvc, userChecker, auditor).
		WithProvider(&mockProvider{
			err: errors.New("provider error"),
		}).
		WithTransaction(nil)

	_, err := svc.GenerateCaseSummary(context.Background(), GenerateCaseSummaryParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
	})
	if err == nil {
		t.Fatal("expected error from provider failure")
	}
}

type mockAuditRecorder struct{}

func (m *mockAuditRecorder) RecordEvent(ctx context.Context, params auditdomain.RecordEventParams) error {
	return nil
}

func (m *mockAuditRecorder) RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error {
	return nil
}
