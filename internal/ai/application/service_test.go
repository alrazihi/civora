package application

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/ai/domain"
	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	evidencedomain "github.com/alrazihi/civora/internal/evidence/domain"
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

type mockEvidenceRepo struct {
	evidence  *evidencedomain.Evidence
	documents []*evidencedomain.Document
	findErr   error
	docErr    error
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

func (m *mockEvidenceRepo) ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*evidencedomain.Document, int, error) {
	if m.docErr != nil {
		return nil, 0, m.docErr
	}
	return m.documents, 0, nil
}

func (m *mockEvidenceRepo) SaveTx(ctx context.Context, tx *sql.Tx, e *evidencedomain.Evidence) error {
	return nil
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

func (m *mockObservationRepo) UpdateStatus(ctx context.Context, orgID, observationID uuid.UUID, status domain.ObservationStatus, reviewerID *uuid.UUID, notes string) error {
	return m.updateErr
}

func (m *mockObservationRepo) CountByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID) (int, error) {
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

	evRepo := &mockEvidenceRepo{evidence: evidence, documents: []*evidencedomain.Document{}}
	obsRepo := &mockObservationRepo{}

	svc := NewAIService(obsRepo, evRepo, &mockUserChecker{valid: true}, nil, provider)

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
	if obs.Status != domain.ObservationStatusPendingReview {
		t.Errorf("expected status PENDING_REVIEW, got %s", obs.Status)
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

	evRepo := &mockEvidenceRepo{evidence: evidence, documents: []*evidencedomain.Document{}}
	obsRepo := &mockObservationRepo{}

	svc := NewAIService(obsRepo, evRepo, &mockUserChecker{valid: true}, nil, provider)

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

	evRepo := &mockEvidenceRepo{evidence: evidence, documents: []*evidencedomain.Document{}}
	obsRepo := &mockObservationRepo{}

	svc := NewAIService(obsRepo, evRepo, &mockUserChecker{valid: false}, nil, provider)

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

	evRepo := &mockEvidenceRepo{evidence: nil}
	obsRepo := &mockObservationRepo{}

	svc := NewAIService(obsRepo, evRepo, &mockUserChecker{valid: true}, nil, provider)

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

func TestAIService_ReviewObservation_Accept(t *testing.T) {
	orgID := uuid.New()
	obsID := uuid.New()
	reviewerID := uuid.New()

	evidence := makeTestEvidence()
	evidence.OrganizationID = orgID

	obs := &domain.Observation{
		ID:             obsID,
		OrganizationID: orgID,
		EvidenceID:     evidence.ID,
		Type:           domain.ObservationTypeSummary,
		Source:         domain.ObservationSourceAIModel,
		Status:         domain.ObservationStatusPendingReview,
		Content:        map[string]any{"text": "test"},
		CreatedAt:      time.Now().UTC(),
	}

	obsRepo := &mockObservationRepo{obs: obs}
	evRepo := &mockEvidenceRepo{evidence: evidence}

	svc := NewAIService(obsRepo, evRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

	result, err := svc.ReviewObservation(context.Background(), ReviewObservationParams{
		OrganizationID: orgID,
		ObservationID:  obsID,
		ReviewerID:     reviewerID,
		Action:         domain.ObservationStatusAccepted,
		Notes:          "looks good",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Status != domain.ObservationStatusAccepted {
		t.Errorf("expected status ACCEPTED, got %s", result.Status)
	}
	if *result.ReviewedBy != reviewerID {
		t.Error("expected reviewed_by to be set")
	}
	if result.ReviewNotes != "looks good" {
		t.Errorf("expected notes 'looks good', got %s", result.ReviewNotes)
	}
}

func TestAIService_ReviewObservation_Reject(t *testing.T) {
	orgID := uuid.New()
	obsID := uuid.New()
	reviewerID := uuid.New()

	evidence := makeTestEvidence()
	evidence.OrganizationID = orgID

	obs := &domain.Observation{
		ID:             obsID,
		OrganizationID: orgID,
		EvidenceID:     evidence.ID,
		Type:           domain.ObservationTypeSummary,
		Source:         domain.ObservationSourceAIModel,
		Status:         domain.ObservationStatusPendingReview,
		Content:        map[string]any{"text": "test"},
		CreatedAt:      time.Now().UTC(),
	}

	obsRepo := &mockObservationRepo{obs: obs}
	evRepo := &mockEvidenceRepo{evidence: evidence}

	svc := NewAIService(obsRepo, evRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

	result, err := svc.ReviewObservation(context.Background(), ReviewObservationParams{
		OrganizationID: orgID,
		ObservationID:  obsID,
		ReviewerID:     reviewerID,
		Action:         domain.ObservationStatusRejected,
		Notes:          "not useful",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Status != domain.ObservationStatusRejected {
		t.Errorf("expected status REJECTED, got %s", result.Status)
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
		EvidenceID:     evidence.ID,
		Type:           domain.ObservationTypeSummary,
		Source:         domain.ObservationSourceAIModel,
		Status:         domain.ObservationStatusPendingReview,
		Content:        map[string]any{"text": "test"},
		CreatedAt:      time.Now().UTC(),
	}

	obsRepo := &mockObservationRepo{obs: obs}
	evRepo := &mockEvidenceRepo{evidence: evidence}

	svc := NewAIService(obsRepo, evRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

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
			{ID: uuid.New(), OrganizationID: orgID, EvidenceID: evidenceID, Type: domain.ObservationTypeSummary, Status: domain.ObservationStatusPendingReview},
		},
		listTotal: 1,
	}
	evRepo := &mockEvidenceRepo{evidence: evidence}

	svc := NewAIService(obsRepo, evRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

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

	evRepo := &mockEvidenceRepo{evidence: nil}
	obsRepo := &mockObservationRepo{}

	svc := NewAIService(obsRepo, evRepo, &mockUserChecker{valid: true}, nil, &mockAIProvider{})

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
