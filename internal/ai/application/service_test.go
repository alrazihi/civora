package application

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/ai/domain"
	aisanitizer "github.com/alrazihi/civora/internal/ai/infrastructure/sanitizer"
	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	evidencedomain "github.com/alrazihi/civora/internal/evidence/domain"
	formsubdomain "github.com/alrazihi/civora/internal/form_submission/domain"
	"github.com/google/uuid"
)

type mockAIProvider struct {
	info       domain.ModelInfo
	results    []ObservationResult
	err        error
	calledWith *ProviderRequest
}

func (m *mockAIProvider) GenerateObservations(ctx context.Context, req ProviderRequest) ([]ObservationResult, error) {
	m.calledWith = &req
	return m.results, m.err
}

func (m *mockAIProvider) ProviderInfo() domain.ModelInfo {
	return m.info
}

func (m *mockAIProvider) GenerateChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return nil, errors.New("not implemented in mock")
}

type mockEvidenceRepo struct {
	evidence          *evidencedomain.Evidence
	documents         []*evidencedomain.Document
	findErr           error
	docErr            error
	serviceRequest    []*evidencedomain.Evidence
	serviceRequestErr error
}

func (m *mockEvidenceRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*evidencedomain.Evidence, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.evidence == nil {
		return nil, evidencedomain.ErrEvidenceNotFound
	}
	return m.evidence, nil
}

func (m *mockEvidenceRepo) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*evidencedomain.Evidence, error) {
	if m.serviceRequestErr != nil {
		return nil, m.serviceRequestErr
	}
	return m.serviceRequest, nil
}

func (m *mockEvidenceRepo) ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*evidencedomain.Document, int, error) {
	if m.docErr != nil {
		return nil, 0, m.docErr
	}
	return m.documents, 0, nil
}

func (m *mockEvidenceRepo) SaveTx(ctx context.Context, tx *sql.Tx, e *evidencedomain.Evidence) error {
	return nil
}

type mockFormRepo struct {
	submissions []*formsubdomain.FormSubmission
	listErr     error
}

func (m *mockFormRepo) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID) ([]*formsubdomain.FormSubmission, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.submissions, nil
}

type mockObservationRepo struct {
	saved      []*domain.Observation
	findErr    error
	obs        *domain.Observation
	updateErr  error
	listResult []*domain.Observation
	listTotal  int
	listErr    error
}

func (m *mockObservationRepo) Save(ctx context.Context, o *domain.Observation) error {
	m.saved = append(m.saved, o)
	return nil
}

func (m *mockObservationRepo) SaveTx(ctx context.Context, tx *sql.Tx, o *domain.Observation) error {
	m.saved = append(m.saved, o)
	return nil
}

func (m *mockObservationRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Observation, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.obs == nil {
		return nil, domain.ErrObservationNotFound
	}
	return m.obs, nil
}

func (m *mockObservationRepo) FindByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*domain.Observation, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.listResult, m.listTotal, nil
}

func (m *mockObservationRepo) FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*domain.Observation, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.listResult, m.listTotal, nil
}

func (m *mockObservationRepo) UpdateStatus(ctx context.Context, orgID, observationID uuid.UUID, status domain.ObservationStatus, reviewerID *uuid.UUID, notes string) error {
	return m.updateErr
}

func (m *mockObservationRepo) UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, observationID uuid.UUID, status domain.ObservationStatus, reviewerID *uuid.UUID, notes string) error {
	return m.updateErr
}

func (m *mockObservationRepo) CountByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID) (int, error) {
	return 0, nil
}

func (m *mockObservationRepo) CountByCase(ctx context.Context, orgID, caseID uuid.UUID) (int, error) {
	return 0, nil
}

func makeTestEvidence() *evidencedomain.Evidence {
	return &evidencedomain.Evidence{
		ID:                 uuid.New(),
		OrganizationID:     uuid.New(),
		ServiceRequestID:   uuid.New(),
		Type:               evidencedomain.EvidenceTypeOther,
		Description:        "test",
		StorageReference:   "civora://test/doc",
		UploadedBy:         uuid.New(),
		Source:             evidencedomain.EvidenceSourceManual,
		VerificationStatus: evidencedomain.VerificationStatusUnverified,
	}
}

func TestAIService_GenerateObservations_Success(t *testing.T) {
	orgID := uuid.New()
	evidenceID := uuid.New()
	actorID := uuid.New()

	evidence := &evidencedomain.Evidence{
		ID:                 evidenceID,
		OrganizationID:     orgID,
		ServiceRequestID:   evidenceID,
		Type:               evidencedomain.EvidenceTypeOther,
		Description:        "test",
		StorageReference:   "civora://test/doc",
		UploadedBy:         actorID,
		Source:             evidencedomain.EvidenceSourceManual,
		VerificationStatus: evidencedomain.VerificationStatusUnverified,
	}

	provider := &mockAIProvider{
		info: domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
		results: []ObservationResult{
			{
				Type:       domain.ObservationTypeSummary,
				Content:    map[string]any{"text": "sample observation"},
				Confidence: floatPtr(0.95),
				Model:      domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
			},
		},
	}

	evRepo := &mockEvidenceRepo{evidence: evidence, documents: []*evidencedomain.Document{}, serviceRequest: []*evidencedomain.Evidence{evidence}}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, provider)

	result, err := svc.GenerateObservations(context.Background(), GenerateObservationsParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		ActorID:        actorID,
		Types:          []domain.ObservationType{domain.ObservationTypeSummary},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(result.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(result.Observations))
	}

	obs := result.Observations[0]
	if obs.Type != domain.ObservationTypeSummary {
		t.Errorf("expected type SUMMARY, got %s", obs.Type)
	}
	if *obs.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %v", obs.Confidence)
	}
	if obs.Status != domain.ObservationStatusOpen {
		t.Errorf("expected status OPEN, got %s", obs.Status)
	}
	if obs.InputHash == "" || obs.OutputHash == "" {
		t.Error("expected non-empty hashes")
	}

	if len(obsRepo.saved) != 1 {
		t.Errorf("expected 1 saved observation, got %d", len(obsRepo.saved))
	}
}

func TestAIService_GenerateObservations_ProviderError(t *testing.T) {
	orgID := uuid.New()
	evidenceID := uuid.New()
	actorID := uuid.New()

	evidence := &evidencedomain.Evidence{
		ID:                 evidenceID,
		OrganizationID:     orgID,
		ServiceRequestID:   evidenceID,
		Type:               evidencedomain.EvidenceTypeOther,
		Description:        "test",
		StorageReference:   "civora://test/doc",
		UploadedBy:         actorID,
		Source:             evidencedomain.EvidenceSourceManual,
		VerificationStatus: evidencedomain.VerificationStatusUnverified,
	}

	provider := &mockAIProvider{
		info: domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
		err:  errors.New("provider unavailable"),
	}

	evRepo := &mockEvidenceRepo{evidence: evidence, documents: []*evidencedomain.Document{}, serviceRequest: []*evidencedomain.Evidence{evidence}}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, provider)

	_, err := svc.GenerateObservations(context.Background(), GenerateObservationsParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		ActorID:        actorID,
	})

	if err == nil {
		t.Fatal("expected error from provider failure")
	}

	if !errors.Is(err, ErrAIProviderUnavailable) {
		t.Errorf("expected ErrAIProviderUnavailable, got: %v", err)
	}

	if len(obsRepo.saved) != 0 {
		t.Errorf("expected 0 saved observations on provider failure, got %d", len(obsRepo.saved))
	}
}

func TestAIService_GenerateObservations_UserNotInOrg(t *testing.T) {
	orgID := uuid.New()
	evidenceID := uuid.New()
	actorID := uuid.New()

	evidence := &evidencedomain.Evidence{
		ID:                 evidenceID,
		OrganizationID:     orgID,
		ServiceRequestID:   evidenceID,
		Type:               evidencedomain.EvidenceTypeOther,
		Description:        "test",
		StorageReference:   "civora://test/doc",
		UploadedBy:         actorID,
		Source:             evidencedomain.EvidenceSourceManual,
		VerificationStatus: evidencedomain.VerificationStatusUnverified,
	}

	provider := &mockAIProvider{
		info: domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
	}

	evRepo := &mockEvidenceRepo{evidence: evidence, documents: []*evidencedomain.Document{}, serviceRequest: []*evidencedomain.Evidence{evidence}}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: false}, nil, provider)

	_, err := svc.GenerateObservations(context.Background(), GenerateObservationsParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		ActorID:        actorID,
	})

	if err == nil {
		t.Fatal("expected error for user not in org")
	}
}

func TestAIService_GenerateObservations_EvidenceNotFound(t *testing.T) {
	orgID := uuid.New()
	evidenceID := uuid.New()
	actorID := uuid.New()

	provider := &mockAIProvider{
		info: domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
	}

	evRepo := &mockEvidenceRepo{evidence: nil, serviceRequest: nil}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, provider)

	_, err := svc.GenerateObservations(context.Background(), GenerateObservationsParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		ActorID:        actorID,
	})

	if err == nil {
		t.Fatal("expected error for evidence not found")
	}
	if !errors.Is(err, ErrEvidenceNotFound) {
		t.Errorf("expected ErrEvidenceNotFound, got: %v", err)
	}
}

func TestAIService_GenerateCaseObservations_Success(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	evidence := &evidencedomain.Evidence{
		ID:                 uuid.New(),
		OrganizationID:     orgID,
		ServiceRequestID:   caseID,
		Type:               evidencedomain.EvidenceTypeProofOfResidence,
		Description:        "proof of residence",
		StorageReference:   "civora://test/doc",
		UploadedBy:         actorID,
		Source:             evidencedomain.EvidenceSourceManual,
		VerificationStatus: evidencedomain.VerificationStatusUnverified,
	}

	provider := &mockAIProvider{
		info: domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
		results: []ObservationResult{
			{
				Type:       domain.ObservationTypeMissingInformation,
				Content:    map[string]any{"statement": "Missing proof of income", "source_references": []interface{}{map[string]interface{}{"type": "evidence", "id": evidence.ID.String()}}},
				Confidence: floatPtr(0.9),
				Model:      domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
			},
		},
	}

	evRepo := &mockEvidenceRepo{serviceRequest: []*evidencedomain.Evidence{evidence}}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, provider)

	result, err := svc.GenerateCaseObservations(context.Background(), GenerateCaseObservationsParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Types:          []domain.ObservationType{domain.ObservationTypeMissingInformation},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(result.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(result.Observations))
	}

	obs := result.Observations[0]
	if obs.Type != domain.ObservationTypeMissingInformation {
		t.Errorf("expected type MISSING_INFORMATION, got %s", obs.Type)
	}
	if obs.Statement != "Missing proof of income" {
		t.Errorf("expected statement 'Missing proof of income', got %s", obs.Statement)
	}
	if len(obs.SourceReferences) != 1 || obs.SourceReferences[0].Type != "evidence" {
		t.Errorf("expected source references, got %v", obs.SourceReferences)
	}
	if obs.CaseID == nil || *obs.CaseID != caseID {
		t.Errorf("expected case ID %s, got %v", caseID, obs.CaseID)
	}
	if obs.Status != domain.ObservationStatusOpen {
		t.Errorf("expected status OPEN, got %s", obs.Status)
	}
}

func TestAIService_GenerateCaseObservations_ProviderError(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	provider := &mockAIProvider{
		info: domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
		err:  errors.New("provider unavailable"),
	}

	evRepo := &mockEvidenceRepo{serviceRequest: []*evidencedomain.Evidence{}}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, provider)

	_, err := svc.GenerateCaseObservations(context.Background(), GenerateCaseObservationsParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
	})

	if err == nil {
		t.Fatal("expected error from provider failure")
	}

	if !errors.Is(err, ErrAIProviderUnavailable) {
		t.Errorf("expected ErrAIProviderUnavailable, got: %v", err)
	}

	if len(obsRepo.saved) != 0 {
		t.Errorf("expected 0 saved observations on provider failure, got %d", len(obsRepo.saved))
	}
}

func TestAIService_GenerateCaseObservations_UserNotInOrg(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	provider := &mockAIProvider{
		info: domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
	}

	evRepo := &mockEvidenceRepo{serviceRequest: []*evidencedomain.Evidence{}}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: false}, nil, provider)

	_, err := svc.GenerateCaseObservations(context.Background(), GenerateCaseObservationsParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
	})

	if err == nil {
		t.Fatal("expected error for user not in org")
	}
}

func TestAIService_ReviewObservation_Correct(t *testing.T) {
	orgID := uuid.New()
	obsID := uuid.New()
	reviewerID := uuid.New()

	evidence := makeTestEvidence()
	evidence.OrganizationID = orgID

	obs := &domain.Observation{
		ID:             obsID,
		OrganizationID: orgID,
		CaseID:         &evidence.ServiceRequestID,
		Type:           domain.ObservationTypeSummary,
		Source:         domain.ObservationSourceAIModel,
		Status:         domain.ObservationStatusOpen,
		Content:        map[string]any{"text": "test"},
		CreatedAt:      time.Now().UTC(),
	}

	obsRepo := &mockObservationRepo{obs: obs}
	evRepo := &mockEvidenceRepo{evidence: evidence, serviceRequest: []*evidencedomain.Evidence{evidence}}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

	result, err := svc.ReviewObservation(context.Background(), ReviewObservationParams{
		OrganizationID: orgID,
		ObservationID:  obsID,
		ReviewerID:     reviewerID,
		Action:         domain.ObservationStatusCorrected,
		Notes:          "corrected data",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Status != domain.ObservationStatusCorrected {
		t.Errorf("expected status CORRECTED, got %s", result.Status)
	}
	if result.ReviewNotes != "corrected data" {
		t.Errorf("expected notes 'corrected data', got %s", result.ReviewNotes)
	}
}

func TestAIService_ReviewObservation_Dismiss(t *testing.T) {
	orgID := uuid.New()
	obsID := uuid.New()
	reviewerID := uuid.New()

	evidence := makeTestEvidence()
	evidence.OrganizationID = orgID

	obs := &domain.Observation{
		ID:             obsID,
		OrganizationID: orgID,
		CaseID:         &evidence.ServiceRequestID,
		Type:           domain.ObservationTypeSummary,
		Source:         domain.ObservationSourceAIModel,
		Status:         domain.ObservationStatusOpen,
		Content:        map[string]any{"text": "test"},
		CreatedAt:      time.Now().UTC(),
	}

	obsRepo := &mockObservationRepo{obs: obs}
	evRepo := &mockEvidenceRepo{evidence: evidence, serviceRequest: []*evidencedomain.Evidence{evidence}}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

	result, err := svc.ReviewObservation(context.Background(), ReviewObservationParams{
		OrganizationID: orgID,
		ObservationID:  obsID,
		ReviewerID:     reviewerID,
		Action:         domain.ObservationStatusDismissed,
		Notes:          "not applicable",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Status != domain.ObservationStatusDismissed {
		t.Errorf("expected status DISMISSED, got %s", result.Status)
	}
}

func TestAIService_ReviewObservation_InvalidAction(t *testing.T) {
	orgID := uuid.New()
	obsID := uuid.New()
	reviewerID := uuid.New()

	evidence := makeTestEvidence()
	evidence.OrganizationID = orgID

	obs := &domain.Observation{
		ID:             obsID,
		OrganizationID: orgID,
		CaseID:         &evidence.ServiceRequestID,
		Type:           domain.ObservationTypeSummary,
		Source:         domain.ObservationSourceAIModel,
		Status:         domain.ObservationStatusOpen,
		Content:        map[string]any{"text": "test"},
		CreatedAt:      time.Now().UTC(),
	}

	obsRepo := &mockObservationRepo{obs: obs}
	evRepo := &mockEvidenceRepo{evidence: evidence, serviceRequest: []*evidencedomain.Evidence{evidence}}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

	_, err := svc.ReviewObservation(context.Background(), ReviewObservationParams{
		OrganizationID: orgID,
		ObservationID:  obsID,
		ReviewerID:     reviewerID,
		Action:         domain.ObservationStatusPendingReview,
		Notes:          "invalid action",
	})

	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestAIService_ListObservations(t *testing.T) {
	orgID := uuid.New()
	evidenceID := uuid.New()
	actorID := uuid.New()

	evidence := &evidencedomain.Evidence{
		ID:                 evidenceID,
		OrganizationID:     orgID,
		ServiceRequestID:   evidenceID,
		Type:               evidencedomain.EvidenceTypeOther,
		Description:        "test",
		StorageReference:   "civora://test/doc",
		UploadedBy:         actorID,
		Source:             evidencedomain.EvidenceSourceManual,
		VerificationStatus: evidencedomain.VerificationStatusUnverified,
	}

	obsRepo := &mockObservationRepo{
		listResult: []*domain.Observation{
			{ID: uuid.New(), OrganizationID: orgID, EvidenceID: &evidenceID, Type: domain.ObservationTypeSummary, Status: domain.ObservationStatusOpen},
		},
		listTotal: 1,
	}
	evRepo := &mockEvidenceRepo{evidence: evidence, serviceRequest: []*evidencedomain.Evidence{evidence}}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

	items, total, err := svc.ListObservations(context.Background(), ListObservationsParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		ActorID:        actorID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestAIService_ListObservations_EvidenceNotFound(t *testing.T) {
	orgID := uuid.New()
	evidenceID := uuid.New()
	actorID := uuid.New()

	evRepo := &mockEvidenceRepo{evidence: nil, serviceRequest: nil}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

	_, _, err := svc.ListObservations(context.Background(), ListObservationsParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		ActorID:        actorID,
	})

	if err == nil {
		t.Fatal("expected error for evidence not found")
	}
	if !errors.Is(err, ErrEvidenceNotFound) {
		t.Errorf("expected ErrEvidenceNotFound, got: %v", err)
	}
}

func TestAIService_ListCaseObservations_Success(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	evidence := &evidencedomain.Evidence{
		ID:                 uuid.New(),
		OrganizationID:     orgID,
		ServiceRequestID:   caseID,
		Type:               evidencedomain.EvidenceTypeProofOfResidence,
		Description:        "proof of residence",
		StorageReference:   "civora://test/doc",
		UploadedBy:         actorID,
		Source:             evidencedomain.EvidenceSourceManual,
		VerificationStatus: evidencedomain.VerificationStatusUnverified,
	}

	obsRepo := &mockObservationRepo{
		listResult: []*domain.Observation{
			{ID: uuid.New(), OrganizationID: orgID, CaseID: &caseID, Type: domain.ObservationTypeMissingInformation, Status: domain.ObservationStatusOpen},
		},
		listTotal: 1,
	}
	evRepo := &mockEvidenceRepo{serviceRequest: []*evidencedomain.Evidence{evidence}}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

	items, total, err := svc.ListCaseObservations(context.Background(), ListCaseObservationsParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestAIService_ListCaseObservations_CaseHasNoEvidence(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	evRepo := &mockEvidenceRepo{serviceRequest: []*evidencedomain.Evidence{}}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

	_, _, err := svc.ListCaseObservations(context.Background(), ListCaseObservationsParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
	})

	if err == nil {
		t.Fatal("expected error for case with no evidence")
	}
}

type mockUserChecker struct {
	valid bool
}

func (m *mockUserChecker) BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	return m.valid, nil
}

type mockAuditor struct {
	events []auditdomain.RecordEventParams
}

func (m *mockAuditor) RecordEvent(ctx context.Context, params auditdomain.RecordEventParams) error {
	m.events = append(m.events, params)
	return nil
}

func floatPtr(f float64) *float64 {
	return &f
}

func TestAIService_GenerateCaseObservations_AuditRecorded(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	evidence := &evidencedomain.Evidence{
		ID:                 uuid.New(),
		OrganizationID:     orgID,
		ServiceRequestID:   caseID,
		Type:               evidencedomain.EvidenceTypeProofOfResidence,
		Description:        "proof of residence",
		StorageReference:   "civora://test/doc",
		UploadedBy:         actorID,
		Source:             evidencedomain.EvidenceSourceManual,
		VerificationStatus: evidencedomain.VerificationStatusUnverified,
	}

	provider := &mockAIProvider{
		info: domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
		results: []ObservationResult{
			{
				Type:       domain.ObservationTypeMissingInformation,
				Content:    map[string]any{"statement": "Missing proof of income"},
				Confidence: floatPtr(0.9),
				Model:      domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
			},
		},
	}

	auditor := &mockAuditor{}
	evRepo := &mockEvidenceRepo{serviceRequest: []*evidencedomain.Evidence{evidence}}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, auditor, provider)

	_, err := svc.GenerateCaseObservations(context.Background(), GenerateCaseObservationsParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Types:          []domain.ObservationType{domain.ObservationTypeMissingInformation},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	foundGenerated := false
	foundCreated := false
	for _, event := range auditor.events {
		if event.Action == "ai.observations_generated" {
			foundGenerated = true
		}
		if event.Action == "ai.observation.created" {
			foundCreated = true
		}
	}

	if !foundGenerated {
		t.Error("expected ai.observations_generated audit event")
	}
	if !foundCreated {
		t.Error("expected ai.observation.created audit event")
	}
}

func TestAIService_GenerateCaseObservations_ConflictingSources(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()

	evidence := &evidencedomain.Evidence{
		ID:                 uuid.New(),
		OrganizationID:     orgID,
		ServiceRequestID:   caseID,
		Type:               evidencedomain.EvidenceTypeProofOfResidence,
		Description:        "proof of residence",
		StorageReference:   "civora://test/doc",
		UploadedBy:         actorID,
		Source:             evidencedomain.EvidenceSourceManual,
		VerificationStatus: evidencedomain.VerificationStatusUnverified,
	}

	provider := &mockAIProvider{
		info: domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
		results: []ObservationResult{
			{
				Type:       domain.ObservationTypeInconsistency,
				Content:    map[string]any{"statement": "Income mismatch between form and document", "source_references": []interface{}{map[string]interface{}{"type": "form_submission", "id": "form-1"}, map[string]interface{}{"type": "evidence", "id": evidence.ID.String()}}},
				Confidence: floatPtr(0.85),
				Model:      domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
			},
		},
	}

	evRepo := &mockEvidenceRepo{serviceRequest: []*evidencedomain.Evidence{evidence}}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, provider)

	result, err := svc.GenerateCaseObservations(context.Background(), GenerateCaseObservationsParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Types:          []domain.ObservationType{domain.ObservationTypeInconsistency},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(result.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(result.Observations))
	}

	obs := result.Observations[0]
	if obs.Type != domain.ObservationTypeInconsistency {
		t.Errorf("expected type INCONSISTENCY, got %s", obs.Type)
	}
	if len(obs.SourceReferences) != 2 {
		t.Errorf("expected 2 source references, got %d", len(obs.SourceReferences))
	}
}

func TestAIService_GenerateObservations_MaliciousDocument(t *testing.T) {
	orgID := uuid.New()
	evidenceID := uuid.New()
	actorID := uuid.New()

	evidence := &evidencedomain.Evidence{
		ID:                 evidenceID,
		OrganizationID:     orgID,
		ServiceRequestID:   evidenceID,
		Type:               evidencedomain.EvidenceTypeOther,
		Description:        "test",
		StorageReference:   "civora://test/doc",
		UploadedBy:         actorID,
		Source:             evidencedomain.EvidenceSourceManual,
		VerificationStatus: evidencedomain.VerificationStatusUnverified,
	}

	provider := &mockAIProvider{
		info: domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
	}

	oversizedContent := make([]byte, domain.MaxDocumentBytes+1)
	for i := range oversizedContent {
		oversizedContent[i] = byte('a' + (i % 26))
	}

	evRepo := &mockEvidenceRepo{
		evidence:       evidence,
		documents:      []*evidencedomain.Document{{ID: uuid.New(), FileName: "test.txt", ContentType: "text/plain", SizeBytes: int64(len(oversizedContent)), Checksum: "abc"}},
		serviceRequest: []*evidencedomain.Evidence{evidence},
	}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: true}, nil, provider).WithDocumentContent(&mockDocumentContentProvider{content: oversizedContent}).WithPIISanitizer(aisanitizer.NewRedactingSanitizer())

	_, err := svc.GenerateObservations(context.Background(), GenerateObservationsParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		ActorID:        actorID,
	})
	if err == nil {
		t.Fatal("expected error for oversized document")
	}
}

func TestAIService_GenerateObservations_CrossTenantBlocked(t *testing.T) {
	orgA := uuid.New()
	actorID := uuid.New()

	evidence := &evidencedomain.Evidence{
		ID:                 uuid.New(),
		OrganizationID:     orgA,
		ServiceRequestID:   uuid.New(),
		Type:               evidencedomain.EvidenceTypeOther,
		Description:        "test",
		StorageReference:   "civora://test/doc",
		UploadedBy:         actorID,
		Source:             evidencedomain.EvidenceSourceManual,
		VerificationStatus: evidencedomain.VerificationStatusUnverified,
	}

	provider := &mockAIProvider{
		info: domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
	}

	evRepo := &mockEvidenceRepo{
		evidence:       evidence,
		documents:      []*evidencedomain.Document{{ID: uuid.New(), FileName: "test.txt", ContentType: "text/plain", Checksum: "abc"}},
		serviceRequest: []*evidencedomain.Evidence{evidence},
	}
	obsRepo := &mockObservationRepo{}
	formRepo := &mockFormRepo{}

	// Actor belongs to orgB (mockUserChecker{valid: false}), tries to generate observations for orgA evidence
	svc := NewAIService(obsRepo, evRepo, formRepo, &mockUserChecker{valid: false}, nil, provider).
		WithDocumentContent(&mockDocumentContentProvider{content: []byte("test content")})

	_, err := svc.GenerateObservations(context.Background(), GenerateObservationsParams{
		OrganizationID: orgA,
		EvidenceID:     evidence.ID,
		ActorID:        actorID,
	})
	if err == nil {
		t.Fatal("expected error for cross-tenant observation generation")
	}
}

type mockDocumentContentProvider struct {
	content []byte
	err     error
}

func (m *mockDocumentContentProvider) GetDocumentContent(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(bytes.NewReader(m.content)), nil
}

type mockVerifiedFactRepo struct {
	saved      []*domain.VerifiedFact
	findErr    error
	fact       *domain.VerifiedFact
	updateErr  error
	listResult []*domain.VerifiedFact
	listTotal  int
	listErr    error
}

func (m *mockVerifiedFactRepo) Save(ctx context.Context, f *domain.VerifiedFact) error {
	m.saved = append(m.saved, f)
	return nil
}

func (m *mockVerifiedFactRepo) SaveTx(ctx context.Context, tx *sql.Tx, f *domain.VerifiedFact) error {
	m.saved = append(m.saved, f)
	return nil
}

func (m *mockVerifiedFactRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.VerifiedFact, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.fact == nil {
		return nil, domain.ErrVerifiedFactNotFound
	}
	return m.fact, nil
}

func (m *mockVerifiedFactRepo) FindByObservation(ctx context.Context, orgID, observationID uuid.UUID) (*domain.VerifiedFact, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.fact == nil {
		return nil, domain.ErrVerifiedFactNotFound
	}
	return m.fact, nil
}

func (m *mockVerifiedFactRepo) FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*domain.VerifiedFact, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.listResult, m.listTotal, nil
}

func (m *mockVerifiedFactRepo) CountByCase(ctx context.Context, orgID, caseID uuid.UUID) (int, error) {
	return 0, nil
}

func (m *mockVerifiedFactRepo) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return nil
}

func TestAIService_CreateVerifiedFact_Success(t *testing.T) {
	orgID := uuid.New()
	observationID := uuid.New()
	reviewerID := uuid.New()

	obs := &domain.Observation{
		ID:               observationID,
		OrganizationID:   orgID,
		CaseID:           uuidPtr(uuid.New()),
		Type:             domain.ObservationTypeSummary,
		Source:           domain.ObservationSourceAIModel,
		Status:           domain.ObservationStatusAccepted,
		Model:            &domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
		Content:          map[string]any{"text": "sample observation"},
		Statement:        "Sample statement",
		InputHash:        "abc123",
		OutputHash:       "def456",
		SourceReferences: []domain.SourceReference{{Type: "evidence", ID: uuid.New().String()}},
		ReviewedAt:       timePtr(time.Now()),
		ReviewedBy:       &reviewerID,
		ReviewNotes:      "reviewer notes",
	}

	evRepo := &mockEvidenceRepo{evidence: &evidencedomain.Evidence{OrganizationID: orgID}, serviceRequest: []*evidencedomain.Evidence{{OrganizationID: orgID}}}
	obsRepo := &mockObservationRepo{obs: obs}
	formRepo := &mockFormRepo{}
	vfRepo := &mockVerifiedFactRepo{}
	userChecker := &mockUserChecker{valid: true}

	svc := NewAIService(obsRepo, evRepo, formRepo, userChecker, nil, &mockAIProvider{}).WithVerifiedFactRepo(vfRepo)

	fact, err := svc.CreateVerifiedFact(context.Background(), CreateVerifiedFactParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		ReviewerID:     reviewerID,
		ReviewAction:   domain.ObservationStatusAccepted,
		ReviewNotes:    "accepted",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fact == nil {
		t.Fatal("expected non-nil verified fact")
	}
	if fact.ObservationID != observationID {
		t.Errorf("expected observation ID %s, got %s", observationID, fact.ObservationID)
	}
	if fact.ReviewerID != reviewerID {
		t.Errorf("expected reviewer ID %s, got %s", reviewerID, fact.ReviewerID)
	}
	if fact.ReviewAction != domain.ObservationStatusAccepted {
		t.Errorf("expected review action %s, got %s", domain.ObservationStatusAccepted, fact.ReviewAction)
	}
	if fact.Source != "HUMAN_ACCEPTED" {
		t.Errorf("expected source HUMAN_ACCEPTED, got %s", fact.Source)
	}
	if fact.Provenance == nil {
		t.Fatal("expected non-nil provenance")
	}
	if fact.Provenance.Source != "HUMAN_ACCEPTED" {
		t.Errorf("expected provenance source HUMAN_ACCEPTED, got %s", fact.Provenance.Source)
	}
	if len(vfRepo.saved) != 1 {
		t.Errorf("expected 1 saved verified fact, got %d", len(vfRepo.saved))
	}
}

func TestAIService_CreateVerifiedFact_Corrected(t *testing.T) {
	orgID := uuid.New()
	observationID := uuid.New()
	reviewerID := uuid.New()

	obs := &domain.Observation{
		ID:             observationID,
		OrganizationID: orgID,
		CaseID:         uuidPtr(uuid.New()),
		Type:           domain.ObservationTypeSummary,
		Source:         domain.ObservationSourceAIModel,
		Status:         domain.ObservationStatusCorrected,
		Model:          &domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
		Content:        map[string]any{"text": "sample observation"},
		Statement:      "Sample statement",
		InputHash:      "abc123",
		OutputHash:     "def456",
		ReviewedAt:     timePtr(time.Now()),
		ReviewedBy:     &reviewerID,
		ReviewNotes:    "corrected",
	}

	evRepo := &mockEvidenceRepo{evidence: &evidencedomain.Evidence{OrganizationID: orgID}, serviceRequest: []*evidencedomain.Evidence{{OrganizationID: orgID}}}
	obsRepo := &mockObservationRepo{obs: obs}
	formRepo := &mockFormRepo{}
	vfRepo := &mockVerifiedFactRepo{}
	userChecker := &mockUserChecker{valid: true}

	svc := NewAIService(obsRepo, evRepo, formRepo, userChecker, nil, &mockAIProvider{}).WithVerifiedFactRepo(vfRepo)

	fact, err := svc.CreateVerifiedFact(context.Background(), CreateVerifiedFactParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		ReviewerID:     reviewerID,
		ReviewAction:   domain.ObservationStatusCorrected,
		ReviewNotes:    "corrected value",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fact == nil {
		t.Fatal("expected non-nil verified fact")
	}
	if fact.Source != "HUMAN_CORRECTION" {
		t.Errorf("expected source HUMAN_CORRECTION, got %s", fact.Source)
	}
	if fact.Provenance == nil {
		t.Fatal("expected non-nil provenance")
	}
	if fact.Provenance.Source != "HUMAN_CORRECTION" {
		t.Errorf("expected provenance source HUMAN_CORRECTION, got %s", fact.Provenance.Source)
	}
	if fact.CorrectedValue == nil {
		t.Error("expected non-nil corrected value for corrected action")
	}
	if len(vfRepo.saved) != 1 {
		t.Errorf("expected 1 saved verified fact, got %d", len(vfRepo.saved))
	}
}

func TestAIService_CreateVerifiedFact_UserNotInOrg(t *testing.T) {
	orgID := uuid.New()
	observationID := uuid.New()
	reviewerID := uuid.New()

	obs := &domain.Observation{
		ID:             observationID,
		OrganizationID: orgID,
		CaseID:         uuidPtr(uuid.New()),
		Type:           domain.ObservationTypeSummary,
		Source:         domain.ObservationSourceAIModel,
		Status:         domain.ObservationStatusAccepted,
		Model:          &domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
		Content:        map[string]any{"text": "sample"},
		InputHash:      "abc123",
		OutputHash:     "def456",
		ReviewedAt:     timePtr(time.Now()),
		ReviewedBy:     &reviewerID,
		ReviewNotes:    "reviewer notes",
	}

	evRepo := &mockEvidenceRepo{evidence: &evidencedomain.Evidence{OrganizationID: orgID}, serviceRequest: []*evidencedomain.Evidence{{OrganizationID: orgID}}}
	obsRepo := &mockObservationRepo{obs: obs}
	formRepo := &mockFormRepo{}
	vfRepo := &mockVerifiedFactRepo{}
	userChecker := &mockUserChecker{valid: false}

	svc := NewAIService(obsRepo, evRepo, formRepo, userChecker, nil, &mockAIProvider{}).WithVerifiedFactRepo(vfRepo)

	_, err := svc.CreateVerifiedFact(context.Background(), CreateVerifiedFactParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		ReviewerID:     reviewerID,
		ReviewAction:   domain.ObservationStatusAccepted,
		ReviewNotes:    "accepted",
	})
	if err == nil {
		t.Fatal("expected error for user not in org")
	}
}

func TestAIService_CreateVerifiedFact_ObservationNotFound(t *testing.T) {
	orgID := uuid.New()
	observationID := uuid.New()
	reviewerID := uuid.New()

	evRepo := &mockEvidenceRepo{findErr: evidencedomain.ErrEvidenceNotFound}
	obsRepo := &mockObservationRepo{findErr: domain.ErrObservationNotFound}
	formRepo := &mockFormRepo{}
	vfRepo := &mockVerifiedFactRepo{}
	userChecker := &mockUserChecker{valid: true}

	svc := NewAIService(obsRepo, evRepo, formRepo, userChecker, nil, &mockAIProvider{}).WithVerifiedFactRepo(vfRepo)

	_, err := svc.CreateVerifiedFact(context.Background(), CreateVerifiedFactParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		ReviewerID:     reviewerID,
		ReviewAction:   domain.ObservationStatusAccepted,
		ReviewNotes:    "accepted",
	})
	if err == nil {
		t.Fatal("expected error for observation not found")
	}
}

func TestAIService_CreateVerifiedFact_MissingRepo(t *testing.T) {
	orgID := uuid.New()
	observationID := uuid.New()
	reviewerID := uuid.New()

	obs := &domain.Observation{
		ID:             observationID,
		OrganizationID: orgID,
		CaseID:         uuidPtr(uuid.New()),
		Type:           domain.ObservationTypeSummary,
		Source:         domain.ObservationSourceAIModel,
		Status:         domain.ObservationStatusAccepted,
		Content:        map[string]any{"text": "sample"},
		InputHash:      "abc123",
		OutputHash:     "def456",
		ReviewedAt:     timePtr(time.Now()),
		ReviewedBy:     &reviewerID,
		ReviewNotes:    "reviewer notes",
	}

	evRepo := &mockEvidenceRepo{evidence: &evidencedomain.Evidence{OrganizationID: orgID}, serviceRequest: []*evidencedomain.Evidence{{OrganizationID: orgID}}}
	obsRepo := &mockObservationRepo{obs: obs}
	formRepo := &mockFormRepo{}
	userChecker := &mockUserChecker{valid: true}

	svc := NewAIService(obsRepo, evRepo, formRepo, userChecker, nil, &mockAIProvider{})

	_, err := svc.CreateVerifiedFact(context.Background(), CreateVerifiedFactParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		ReviewerID:     reviewerID,
		ReviewAction:   domain.ObservationStatusAccepted,
		ReviewNotes:    "accepted",
	})
	if err == nil {
		t.Fatal("expected error for missing verified fact repo")
	}
}

func uuidPtr(u uuid.UUID) *uuid.UUID {
	return &u
}

func timePtr(t time.Time) *time.Time {
	return &t
}
