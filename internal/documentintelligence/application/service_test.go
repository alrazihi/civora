package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/documentintelligence/domain"
	evidencedomain "github.com/alrazihi/civora/internal/evidence/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations

type mockEvidenceRepo struct {
	mock.Mock
}

func (m *mockEvidenceRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*evidencedomain.Evidence, error) {
	args := m.Called(ctx, orgID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*evidencedomain.Evidence), args.Error(1)
}

func (m *mockEvidenceRepo) FindDocumentByID(ctx context.Context, orgID, documentID uuid.UUID) (*evidencedomain.Document, error) {
	args := m.Called(ctx, orgID, documentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*evidencedomain.Document), args.Error(1)
}

func (m *mockEvidenceRepo) ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*evidencedomain.Document, int, error) {
	args := m.Called(ctx, orgID, evidenceID, limit, offset)
	return args.Get(0).([]*evidencedomain.Document), args.Int(1), args.Error(2)
}

func (m *mockEvidenceRepo) SaveTx(ctx context.Context, tx *sql.Tx, e *evidencedomain.Evidence) error {
	args := m.Called(ctx, tx, e)
	return args.Error(0)
}

type mockAnalysisRepo struct {
	mock.Mock
}

func (m *mockAnalysisRepo) Save(ctx context.Context, a *domain.DocumentAnalysis) error {
	args := m.Called(ctx, a)
	return args.Error(0)
}

func (m *mockAnalysisRepo) SaveTx(ctx context.Context, tx *sql.Tx, a *domain.DocumentAnalysis) error {
	args := m.Called(ctx, tx, a)
	return args.Error(0)
}

func (m *mockAnalysisRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.DocumentAnalysis, error) {
	args := m.Called(ctx, orgID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DocumentAnalysis), args.Error(1)
}

func (m *mockAnalysisRepo) FindByDocument(ctx context.Context, orgID, documentID uuid.UUID, limit, offset int) ([]*domain.DocumentAnalysis, int, error) {
	args := m.Called(ctx, orgID, documentID, limit, offset)
	return args.Get(0).([]*domain.DocumentAnalysis), args.Int(1), args.Error(2)
}

func (m *mockAnalysisRepo) FindByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*domain.DocumentAnalysis, int, error) {
	args := m.Called(ctx, orgID, evidenceID, limit, offset)
	return args.Get(0).([]*domain.DocumentAnalysis), args.Int(1), args.Error(2)
}

func (m *mockAnalysisRepo) UpdateStatus(ctx context.Context, orgID, analysisID uuid.UUID, status domain.AnalysisStatus, reviewerID *uuid.UUID, notes string) error {
	args := m.Called(ctx, orgID, analysisID, status, reviewerID, notes)
	return args.Error(0)
}

func (m *mockAnalysisRepo) UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, analysisID uuid.UUID, status domain.AnalysisStatus, reviewerID *uuid.UUID, notes string) error {
	args := m.Called(ctx, tx, orgID, analysisID, status, reviewerID, notes)
	return args.Error(0)
}

func (m *mockAnalysisRepo) CountByDocument(ctx context.Context, orgID, documentID uuid.UUID) (int, error) {
	args := m.Called(ctx, orgID, documentID)
	return args.Int(0), args.Error(1)
}

type mockUserChecker struct {
	mock.Mock
}

func (m *mockUserChecker) BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, orgID, userID)
	return args.Bool(0), args.Error(1)
}

type mockAuditor struct {
	mock.Mock
}

func (m *mockAuditor) RecordEvent(ctx context.Context, params auditdomain.RecordEventParams) error {
	args := m.Called(ctx, params)
	return args.Error(0)
}

func (m *mockAuditor) RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error {
	args := m.Called(ctx, tx, params)
	return args.Error(0)
}

type mockProvider struct {
	mock.Mock
}

func (m *mockProvider) GenerateDocumentAnalyses(ctx context.Context, req ProviderRequest) ([]AnalysisResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]AnalysisResult), args.Error(1)
}

func (m *mockProvider) ProviderInfo() domain.ModelInfo {
	args := m.Called()
	return args.Get(0).(domain.ModelInfo)
}

type mockDocumentContentProvider struct {
	mock.Mock
}

func (m *mockDocumentContentProvider) GetDocumentContent(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) ([]byte, error) {
	args := m.Called(ctx, orgID, evidenceID, documentID, actorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

type mockSanitizer struct {
	mock.Mock
}

func (m *mockSanitizer) SanitizeDocumentContent(content []byte, contentType, fileName string) ([]byte, error) {
	args := m.Called(content, contentType, fileName)
	return args.Get(0).([]byte), args.Error(1)
}

// Test helpers

func newTestService() (*DocumentAnalysisService, *mockAnalysisRepo, *mockEvidenceRepo, *mockUserChecker, *mockAuditor, *mockProvider, *mockDocumentContentProvider, *mockSanitizer) {
	repo := new(mockAnalysisRepo)
	evidenceRepo := new(mockEvidenceRepo)
	userChecker := new(mockUserChecker)
	auditor := new(mockAuditor)
	provider := new(mockProvider)
	docProvider := new(mockDocumentContentProvider)
	sanitizer := new(mockSanitizer)

	service := NewDocumentAnalysisService(repo, evidenceRepo, userChecker, auditor, provider).
		WithDocumentContent(docProvider).
		WithPIISanitizer(sanitizer)

	return service, repo, evidenceRepo, userChecker, auditor, provider, docProvider, sanitizer
}

func createTestDocument() *evidencedomain.Document {
	doc, _ := evidencedomain.NewDocument(evidencedomain.DocumentParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		FileName:       "test.pdf",
		ContentType:    "application/pdf",
		SizeBytes:      1024,
		Checksum:       "8d430eb73472bd0177cf3cd165c9541c775a59f6871cf2a9e736e40584d24b78",
		StorageKey:     "2024/01/01/test.pdf",
		StorageProv:    evidencedomain.StorageProviderLocal,
		UploadedBy:     uuid.New(),
	})
	return doc
}

func createTestEvidence(doc *evidencedomain.Document) *evidencedomain.Evidence {
	e, _ := evidencedomain.NewEvidence(evidencedomain.EvidenceParams{
		OrganizationID:   doc.OrganizationID,
		ServiceRequestID: uuid.New(),
		Type:             evidencedomain.EvidenceTypeSupportingDocument,
		Description:      "Test evidence",
		StorageReference: "civora://test",
		UploadedBy:       doc.UploadedBy,
		Source:           evidencedomain.EvidenceSourceUpload,
	})
	return e
}

func TestGenerateAnalyses_Success(t *testing.T) {
	service, repo, evidenceRepo, userChecker, auditor, provider, docProvider, sanitizer := newTestService()

	orgID := uuid.New()
	documentID := uuid.New()
	evidenceID := uuid.New()
	actorID := uuid.New()

	doc := createTestDocument()
	doc.ID = documentID
	doc.OrganizationID = orgID
	doc.EvidenceID = evidenceID

	e := createTestEvidence(doc)

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, actorID).Return(true, nil)
	evidenceRepo.On("FindDocumentByID", mock.Anything, orgID, documentID).Return(doc, nil)
	evidenceRepo.On("FindByID", mock.Anything, orgID, evidenceID).Return(e, nil)

	expectedContent := []byte("test document content")
	docProvider.On("GetDocumentContent", mock.Anything, orgID, evidenceID, documentID, actorID).Return(expectedContent, nil)
	sanitizer.On("SanitizeDocumentContent", expectedContent, "application/pdf", "test.pdf").Return(expectedContent, nil)

	result := []AnalysisResult{
		{
			Type:       domain.AnalysisTypeClassification,
			Content:    map[string]any{"classification": "employment_contract"},
			Confidence: float64Ptr(0.9),
			Model:      domain.ModelInfo{Name: "gpt-4o-mini", Version: "1.0", Provider: "openai"},
		},
	}
	provider.On("GenerateDocumentAnalyses", mock.Anything, mock.Anything).Return(result, nil)
	provider.On("ProviderInfo").Return(domain.ModelInfo{Name: "gpt-4o-mini", Version: "1.0", Provider: "openai"})

	repo.On("SaveTx", mock.Anything, mock.Anything, mock.MatchedBy(func(a *domain.DocumentAnalysis) bool {
		return a.OrganizationID == orgID && a.DocumentID == documentID
	})).Return(nil)

	auditor.On("RecordEventInTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	auditor.On("RecordEvent", mock.Anything, mock.Anything).Return(nil)

	params := GenerateAnalysesParams{
		OrganizationID: orgID,
		DocumentID:     documentID,
		ActorID:        actorID,
		Types:          []domain.AnalysisType{domain.AnalysisTypeClassification},
		MaxTokens:      4096,
	}

	res, err := service.GenerateAnalyses(context.Background(), params)

	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, res.Analyses, 1)
	assert.Equal(t, domain.AnalysisTypeClassification, res.Analyses[0].Type)
}

func TestGenerateAnalyses_ActorNotInOrg(t *testing.T) {
	service, _, _, userChecker, _, _, _, _ := newTestService()

	userChecker.On("BelongsToOrganization", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

	params := GenerateAnalysesParams{
		OrganizationID: uuid.New(),
		DocumentID:     uuid.New(),
		ActorID:        uuid.New(),
	}

	_, err := service.GenerateAnalyses(context.Background(), params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user does not belong to organization")
}

func TestGenerateAnalyses_DocumentNotFound(t *testing.T) {
	service, _, evidenceRepo, userChecker, _, _, _, _ := newTestService()

	userChecker.On("BelongsToOrganization", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	evidenceRepo.On("FindDocumentByID", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("not found"))

	params := GenerateAnalysesParams{
		OrganizationID: uuid.New(),
		DocumentID:     uuid.New(),
		ActorID:        uuid.New(),
	}

	_, err := service.GenerateAnalyses(context.Background(), params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document not found")
}

func TestGenerateAnalyses_ProviderError(t *testing.T) {
	service, _, evidenceRepo, userChecker, auditor, provider, docProvider, sanitizer := newTestService()

	orgID := uuid.New()
	documentID := uuid.New()
	evidenceID := uuid.New()
	actorID := uuid.New()

	doc := createTestDocument()
	doc.ID = documentID
	doc.OrganizationID = orgID
	doc.EvidenceID = evidenceID
	doc.Checksum = "ed7002b439e9ac845f22357d822bac1444730fbdb6016d3ec9432297b9ec9f73"

	e := createTestEvidence(doc)

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, actorID).Return(true, nil)
	evidenceRepo.On("FindDocumentByID", mock.Anything, orgID, documentID).Return(doc, nil)
	evidenceRepo.On("FindByID", mock.Anything, orgID, evidenceID).Return(e, nil)

	docProvider.On("GetDocumentContent", mock.Anything, orgID, evidenceID, documentID, actorID).Return([]byte("content"), nil)
	sanitizer.On("SanitizeDocumentContent", mock.Anything, mock.Anything, mock.Anything).Return([]byte("content"), nil)

	provider.On("GenerateDocumentAnalyses", mock.Anything, mock.Anything).Return(nil, errors.New("provider failed"))
	provider.On("ProviderInfo").Return(domain.ModelInfo{Name: "test", Version: "1.0", Provider: "test"})
	auditor.On("RecordEvent", mock.Anything, mock.Anything).Return(nil)

	params := GenerateAnalysesParams{
		OrganizationID: orgID,
		DocumentID:     documentID,
		ActorID:        actorID,
	}

	_, err := service.GenerateAnalyses(context.Background(), params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AI provider unavailable")
}

func TestListAnalyses_Success(t *testing.T) {
	service, repo, evidenceRepo, _, _, _, _, _ := newTestService()

	orgID := uuid.New()
	documentID := uuid.New()
	evidenceID := uuid.New()
	actorID := uuid.New()

	doc := createTestDocument()
	doc.ID = documentID
	doc.OrganizationID = orgID
	doc.EvidenceID = evidenceID

	evidenceRepo.On("FindDocumentByID", mock.Anything, orgID, documentID).Return(doc, nil)
	evidenceRepo.On("FindByID", mock.Anything, orgID, evidenceID).Return(createTestEvidence(doc), nil)

	analysis := &domain.DocumentAnalysis{
		ID:             uuid.New(),
		OrganizationID: orgID,
		DocumentID:     documentID,
		EvidenceID:     evidenceID,
		Type:           domain.AnalysisTypeSummarization,
		Source:         domain.AnalysisSourceAIModel,
		Status:         domain.AnalysisStatusPendingReview,
		Content:        map[string]any{"summary": "test"},
		CreatedAt:      time.Now().UTC(),
	}
	repo.On("FindByDocument", mock.Anything, orgID, documentID, 20, 0).Return([]*domain.DocumentAnalysis{analysis}, 1, nil)

	params := ListAnalysesParams{
		OrganizationID: orgID,
		DocumentID:     documentID,
		ActorID:        actorID,
		Limit:          20,
		Offset:         0,
	}

	items, total, err := service.ListAnalyses(context.Background(), params)

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, items, 1)
}

func TestReviewAnalysis_Accept(t *testing.T) {
	service, repo, _, userChecker, auditor, _, _, _ := newTestService()

	orgID := uuid.New()
	analysisID := uuid.New()
	reviewerID := uuid.New()

	analysis := &domain.DocumentAnalysis{
		ID:             analysisID,
		OrganizationID: orgID,
		DocumentID:     uuid.New(),
		EvidenceID:     uuid.New(),
		Type:           domain.AnalysisTypeClassification,
		Source:         domain.AnalysisSourceAIModel,
		Status:         domain.AnalysisStatusPendingReview,
		Content:        map[string]any{"classification": "test"},
		CreatedAt:      time.Now().UTC(),
	}

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, reviewerID).Return(true, nil)
	repo.On("FindByID", mock.Anything, orgID, analysisID).Return(analysis, nil)
	repo.On("UpdateStatusTx", mock.Anything, mock.Anything, orgID, analysisID, domain.AnalysisStatusAccepted, &reviewerID, "approved").Return(nil)
	auditor.On("RecordEventInTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	auditor.On("RecordEvent", mock.Anything, mock.Anything).Return(nil)

	params := ReviewAnalysisParams{
		OrganizationID: orgID,
		AnalysisID:     analysisID,
		ReviewerID:     reviewerID,
		Action:         domain.AnalysisStatusAccepted,
		Notes:          "approved",
	}

	result, err := service.ReviewAnalysis(context.Background(), params)

	require.NoError(t, err)
	assert.Equal(t, domain.AnalysisStatusAccepted, result.Status)
	assert.Equal(t, reviewerID, *result.ReviewedBy)
	assert.Equal(t, "approved", result.ReviewNotes)
}

func TestReviewAnalysis_Reject(t *testing.T) {
	service, repo, _, userChecker, auditor, _, _, _ := newTestService()

	orgID := uuid.New()
	analysisID := uuid.New()
	reviewerID := uuid.New()

	analysis := &domain.DocumentAnalysis{
		ID:             analysisID,
		OrganizationID: orgID,
		DocumentID:     uuid.New(),
		EvidenceID:     uuid.New(),
		Type:           domain.AnalysisTypeClassification,
		Source:         domain.AnalysisSourceAIModel,
		Status:         domain.AnalysisStatusPendingReview,
		Content:        map[string]any{"classification": "test"},
		CreatedAt:      time.Now().UTC(),
	}

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, reviewerID).Return(true, nil)
	repo.On("FindByID", mock.Anything, orgID, analysisID).Return(analysis, nil)
	repo.On("UpdateStatusTx", mock.Anything, mock.Anything, orgID, analysisID, domain.AnalysisStatusRejected, &reviewerID, "rejected").Return(nil)
	auditor.On("RecordEventInTx", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	auditor.On("RecordEvent", mock.Anything, mock.Anything).Return(nil)

	params := ReviewAnalysisParams{
		OrganizationID: orgID,
		AnalysisID:     analysisID,
		ReviewerID:     reviewerID,
		Action:         domain.AnalysisStatusRejected,
		Notes:          "rejected",
	}

	result, err := service.ReviewAnalysis(context.Background(), params)

	require.NoError(t, err)
	assert.Equal(t, domain.AnalysisStatusRejected, result.Status)
}

func TestReviewAnalysis_InvalidAction(t *testing.T) {
	service, repo, _, userChecker, _, _, _, _ := newTestService()

	orgID := uuid.New()
	analysisID := uuid.New()
	reviewerID := uuid.New()

	analysis := &domain.DocumentAnalysis{
		ID:             analysisID,
		OrganizationID: orgID,
		DocumentID:     uuid.New(),
		EvidenceID:     uuid.New(),
		Type:           domain.AnalysisTypeClassification,
		Source:         domain.AnalysisSourceAIModel,
		Status:         domain.AnalysisStatusPendingReview,
		Content:        map[string]any{"classification": "test"},
		CreatedAt:      time.Now().UTC(),
	}

	userChecker.On("BelongsToOrganization", mock.Anything, orgID, reviewerID).Return(true, nil)
	repo.On("FindByID", mock.Anything, orgID, analysisID).Return(analysis, nil)

	params := ReviewAnalysisParams{
		OrganizationID: orgID,
		AnalysisID:     analysisID,
		ReviewerID:     reviewerID,
		Action:         domain.AnalysisStatus("INVALID"),
		Notes:          "notes",
	}

	_, err := service.ReviewAnalysis(context.Background(), params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid action")
}

func float64Ptr(f float64) *float64 {
	return &f
}
