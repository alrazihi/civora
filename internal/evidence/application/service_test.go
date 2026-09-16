package application

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"sync"
	"testing"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database/testdb"
	evidencedomain "github.com/alrazihi/civora/internal/evidence/domain"
	"github.com/alrazihi/civora/internal/evidence/infrastructure/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEvidenceRepo struct {
	mu        sync.Mutex
	items     map[uuid.UUID]*evidencedomain.Evidence
	history   map[uuid.UUID][]*evidencedomain.VerificationRecord
	documents map[uuid.UUID][]*evidencedomain.Document
}

func newMockEvidenceRepo() *mockEvidenceRepo {
	return &mockEvidenceRepo{
		items:     make(map[uuid.UUID]*evidencedomain.Evidence),
		history:   make(map[uuid.UUID][]*evidencedomain.VerificationRecord),
		documents: make(map[uuid.UUID][]*evidencedomain.Document),
	}
}

func (m *mockEvidenceRepo) DB() *sql.DB { return testdb.NewDB() }

func (m *mockEvidenceRepo) Save(ctx context.Context, e *evidencedomain.Evidence) error {
	return m.SaveTx(ctx, nil, e)
}

func (m *mockEvidenceRepo) SaveTx(ctx context.Context, tx *sql.Tx, e *evidencedomain.Evidence) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[e.ID] = e
	return nil
}

func (m *mockEvidenceRepo) UpdateVerification(ctx context.Context, e *evidencedomain.Evidence) error {
	return m.UpdateVerificationTx(ctx, nil, e)
}

func (m *mockEvidenceRepo) UpdateVerificationTx(ctx context.Context, tx *sql.Tx, e *evidencedomain.Evidence) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.items[e.ID]; ok && existing.OrganizationID == e.OrganizationID {
		m.items[e.ID] = e
		return nil
	}
	return evidencedomain.ErrEvidenceNotFound
}

func (m *mockEvidenceRepo) FindByID(ctx context.Context, orgID, id uuid.UUID) (*evidencedomain.Evidence, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.items[id]
	if !ok || e.OrganizationID != orgID {
		return nil, errors.New("not found")
	}
	return e, nil
}

func (m *mockEvidenceRepo) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*evidencedomain.Evidence, error) {
	return nil, nil
}

func (m *mockEvidenceRepo) CountByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (int, error) {
	return 0, nil
}

func (m *mockEvidenceRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*evidencedomain.Evidence, error) {
	return nil, nil
}

func (m *mockEvidenceRepo) SaveVerificationHistory(ctx context.Context, rec *evidencedomain.VerificationRecord) error {
	return m.SaveVerificationHistoryTx(ctx, nil, rec)
}

func (m *mockEvidenceRepo) SaveVerificationHistoryTx(ctx context.Context, tx *sql.Tx, rec *evidencedomain.VerificationRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history[rec.EvidenceID] = append(m.history[rec.EvidenceID], rec)
	return nil
}

func (m *mockEvidenceRepo) FindVerificationHistory(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*evidencedomain.VerificationRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.history[evidenceID], nil
}

func (m *mockEvidenceRepo) CountVerificationHistory(ctx context.Context, orgID, evidenceID uuid.UUID) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.history[evidenceID]), nil
}

func (m *mockEvidenceRepo) SaveDocument(ctx context.Context, d *evidencedomain.Document) error {
	return m.SaveDocumentTx(ctx, nil, d)
}

func (m *mockEvidenceRepo) SaveDocumentTx(ctx context.Context, tx *sql.Tx, d *evidencedomain.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d == nil {
		return errors.New("document is nil")
	}
	m.documents[d.EvidenceID] = append(m.documents[d.EvidenceID], d)
	return nil
}

func (m *mockEvidenceRepo) FindDocumentByID(ctx context.Context, orgID, id uuid.UUID) (*evidencedomain.Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, docs := range m.documents {
		for _, d := range docs {
			if d.ID == id {
				return d, nil
			}
		}
	}
	return nil, errors.New("not found")
}

func (m *mockEvidenceRepo) FindDocumentByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID) (*evidencedomain.Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	docs := m.documents[evidenceID]
	if len(docs) == 0 {
		return nil, errors.New("not found")
	}
	return docs[len(docs)-1], nil
}

func (m *mockEvidenceRepo) ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*evidencedomain.Document, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	docs := m.documents[evidenceID]
	if limit <= 0 {
		limit = 50
	}
	end := offset + limit
	if end > len(docs) {
		end = len(docs)
	}
	if offset > len(docs) {
		offset = len(docs)
	}
	return docs[offset:end], len(docs), nil
}

func (m *mockEvidenceRepo) DeleteDocument(ctx context.Context, orgID, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for evidenceID, docs := range m.documents {
		for i, d := range docs {
			if d.ID == id {
				m.documents[evidenceID] = append(docs[:i], docs[i+1:]...)
				return nil
			}
		}
	}
	return evidencedomain.ErrDocumentNotFound
}

func (m *mockEvidenceRepo) CountDocuments(ctx context.Context, orgID uuid.UUID) (int, error) {
	return 0, nil
}

type mockCaseFinder struct {
	cases map[uuid.UUID]*domain.Case
}

func newMockCaseFinder() *mockCaseFinder {
	return &mockCaseFinder{cases: make(map[uuid.UUID]*domain.Case)}
}

func (m *mockCaseFinder) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Case, error) {
	c, ok := m.cases[id]
	if !ok || c.OrganizationID != orgID {
		return nil, errors.New("not found")
	}
	return c, nil
}

func (m *mockCaseFinder) addCase(c *domain.Case) {
	m.cases[c.ID] = c
}

type mockUserChecker struct {
	members map[uuid.UUID]map[uuid.UUID]bool
}

func newMockUserChecker() *mockUserChecker {
	return &mockUserChecker{members: make(map[uuid.UUID]map[uuid.UUID]bool)}
}

func (m *mockUserChecker) BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	if users, ok := m.members[orgID]; ok {
		return users[userID], nil
	}
	return false, nil
}

func (m *mockUserChecker) addUser(orgID, userID uuid.UUID) {
	if m.members[orgID] == nil {
		m.members[orgID] = make(map[uuid.UUID]bool)
	}
	m.members[orgID][userID] = true
}

type mockAuditor struct {
	mu     sync.Mutex
	events []auditdomain.RecordEventParams
	txChan chan struct{}
}

func newMockAuditor() *mockAuditor {
	return &mockAuditor{
		events: make([]auditdomain.RecordEventParams, 0),
	}
}

func (m *mockAuditor) RecordEvent(ctx context.Context, params auditdomain.RecordEventParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, params)
	return nil
}

func (m *mockAuditor) RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error {
	return m.RecordEvent(ctx, params)
}

func (m *mockAuditor) eventsForAction(action string) []auditdomain.RecordEventParams {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []auditdomain.RecordEventParams
	for _, e := range m.events {
		if e.Action == action {
			result = append(result, e)
		}
	}
	return result
}

func (m *mockAuditor) eventCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

type mockStorageProvider struct {
	mu    sync.Mutex
	files map[string][]byte
}

func newMockStorageProvider() *mockStorageProvider {
	return &mockStorageProvider{files: make(map[string][]byte)}
}

func (m *mockStorageProvider) Store(ctx context.Context, key string, reader io.Reader, size int64) (storage.StorageMetadata, error) {
	data, err := io.ReadAll(io.LimitReader(reader, size+1))
	if err != nil {
		return storage.StorageMetadata{}, err
	}
	if int64(len(data)) != size {
		return storage.StorageMetadata{}, errors.New("size mismatch")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[key] = data
	return storage.StorageMetadata{
		Key:       key,
		Checksum:  "test-checksum",
		SizeBytes: int64(len(data)),
	}, nil
}

func (m *mockStorageProvider) Retrieve(ctx context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[key]
	if !ok {
		return nil, errors.New("file not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockStorageProvider) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, key)
	return nil
}

func setupTestService(t *testing.T) (*EvidenceService, *mockEvidenceRepo, *mockCaseFinder, *mockUserChecker, *mockAuditor) {
	t.Helper()
	repo := newMockEvidenceRepo()
	caseFinder := newMockCaseFinder()
	userChecker := newMockUserChecker()
	auditor := newMockAuditor()
	svc := NewEvidenceService(repo, caseFinder, userChecker, auditor, newMockStorageProvider())
	return svc, repo, caseFinder, userChecker, auditor
}

func createTestEvidence(t *testing.T, repo *mockEvidenceRepo, orgID, srID, uploadedBy uuid.UUID) *evidencedomain.Evidence {
	t.Helper()
	e, err := evidencedomain.NewEvidence(evidencedomain.EvidenceParams{
		OrganizationID:   orgID,
		ServiceRequestID: srID,
		Type:             evidencedomain.EvidenceTypeIdentityDocument,
		Description:      "Passport",
		StorageReference: "s3://bucket/passport.pdf",
		UploadedBy:       uploadedBy,
	})
	require.NoError(t, err)
	err = repo.Save(context.Background(), e)
	require.NoError(t, err)
	return e
}

func TestAddEvidence_CrossTenantCase(t *testing.T) {
	svc, _, caseFinder, userChecker, _ := setupTestService(t)

	org1 := uuid.New()
	org2 := uuid.New()
	actorID := uuid.New()
	userChecker.addUser(org1, actorID)

	c, _ := domain.NewCase(org1, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	_, err := svc.AddEvidence(context.Background(), AddEvidenceParams{
		OrganizationID:   org2,
		ServiceRequestID: c.ID,
		Type:             evidencedomain.EvidenceTypeStaffNote,
		Description:      "Test",
		StorageReference: "s3://bucket/test",
		UploadedBy:       actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaseNotFound)
}

func TestAddEvidence_CrossTenantUser(t *testing.T) {
	svc, _, caseFinder, _, _ := setupTestService(t)

	orgID := uuid.New()
	actorID := uuid.New()

	c, _ := domain.NewCase(orgID, actorID, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	_, err := svc.AddEvidence(context.Background(), AddEvidenceParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Type:             evidencedomain.EvidenceTypeStaffNote,
		Description:      "Test",
		StorageReference: "s3://bucket/test",
		UploadedBy:       actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestAddEvidence_SuccessWithMetadata(t *testing.T) {
	svc, repo, caseFinder, userChecker, auditor := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)

	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	e, err := svc.AddEvidence(context.Background(), AddEvidenceParams{
		OrganizationID:   orgID,
		ServiceRequestID: c.ID,
		Type:             evidencedomain.EvidenceTypeOther,
		CustomType:       "UTILITY_BILL",
		Description:      "Utility bill",
		StorageReference: "s3://bucket/bill.pdf",
		UploadedBy:       uploadedBy,
		Source:           evidencedomain.EvidenceSourceExternalReference,
		Metadata:         map[string]any{"pages": 3},
	})
	require.NoError(t, err)
	assert.Equal(t, evidencedomain.EvidenceTypeOther, e.Type)
	require.NotNil(t, e.CustomType)
	assert.Equal(t, "UTILITY_BILL", *e.CustomType)
	assert.Equal(t, evidencedomain.EvidenceSourceExternalReference, e.Source)
	assert.Equal(t, 3, e.Metadata["pages"])
	assert.Equal(t, evidencedomain.VerificationStatusUnverified, e.VerificationStatus)

	stored, err := repo.FindByID(context.Background(), orgID, e.ID)
	require.NoError(t, err)
	assert.Equal(t, e.ID, stored.ID)
	assert.Equal(t, e.VerificationStatus, stored.VerificationStatus)

	assert.Len(t, auditor.eventsForAction("evidence.added"), 1)
	assert.Equal(t, "success", auditor.eventsForAction("evidence.added")[0].Outcome)
}

func TestVerifyEvidence_Success(t *testing.T) {
	svc, repo, caseFinder, userChecker, auditor := setupTestService(t)

	orgID := uuid.New()
	srID := uuid.New()
	uploadedBy := uuid.New()
	verifierID := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	userChecker.addUser(orgID, verifierID)

	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)

	e := createTestEvidence(t, repo, orgID, srID, uploadedBy)

	updatedE, rec, err := svc.VerifyEvidence(context.Background(), VerifyEvidenceParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		VerifierID:     verifierID,
		Reason:         "Document looks authentic",
		Method:         "manual",
	})
	require.NoError(t, err)
	assert.Equal(t, evidencedomain.VerificationStatusVerified, updatedE.VerificationStatus)
	require.NotNil(t, updatedE.VerifiedBy)
	assert.Equal(t, verifierID, *updatedE.VerifiedBy)
	require.NotNil(t, rec)
	assert.Equal(t, evidencedomain.VerificationStatusVerified, rec.Status)

	assert.Len(t, auditor.eventsForAction("evidence.verified"), 1)
	assert.Equal(t, "success", auditor.eventsForAction("evidence.verified")[0].Outcome)
}

func TestVerifyEvidence_NotFound(t *testing.T) {
	svc, _, _, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	verifierID := uuid.New()
	userChecker.addUser(orgID, verifierID)

	_, _, err := svc.VerifyEvidence(context.Background(), VerifyEvidenceParams{
		OrganizationID: orgID,
		EvidenceID:     uuid.New(),
		VerifierID:     verifierID,
		Reason:         "test",
		Method:         "manual",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEvidenceNotFound)
}

func TestVerifyEvidence_VerifierNotInOrg(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	verifierID := uuid.New()
	userChecker.addUser(orgID, uploadedBy)

	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	_, _, err := svc.VerifyEvidence(context.Background(), VerifyEvidenceParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		VerifierID:     verifierID,
		Reason:         "test",
		Method:         "manual",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrVerifierNotFound)
}

func TestVerifyEvidence_NilVerifierID(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)

	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	_, _, err := svc.VerifyEvidence(context.Background(), VerifyEvidenceParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		VerifierID:     uuid.Nil,
		Reason:         "test",
		Method:         "manual",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEvidenceInput)
}

func TestRejectEvidence_Success(t *testing.T) {
	svc, repo, caseFinder, userChecker, auditor := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	reviewerID := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	userChecker.addUser(orgID, reviewerID)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	updatedE, rec, err := svc.RejectEvidence(context.Background(), VerifyEvidenceParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		VerifierID:     reviewerID,
		Reason:         "Document is altered",
		Method:         "manual",
	})
	require.NoError(t, err)
	assert.Equal(t, evidencedomain.VerificationStatusRejected, updatedE.VerificationStatus)
	require.NotNil(t, rec)
	assert.Equal(t, "Document is altered", updatedE.VerificationReason)

	assert.Len(t, auditor.eventsForAction("evidence.rejected"), 1)
}

func TestMarkEvidenceForReview_Success(t *testing.T) {
	svc, repo, caseFinder, userChecker, auditor := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	reviewerID := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	userChecker.addUser(orgID, reviewerID)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	updatedE, rec, err := svc.MarkEvidenceForReview(context.Background(), VerifyEvidenceParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		VerifierID:     reviewerID,
		Reason:         "Need more documents",
		Method:         "manual",
	})
	require.NoError(t, err)
	assert.Equal(t, evidencedomain.VerificationStatusNeedsReview, updatedE.VerificationStatus)
	require.NotNil(t, rec)
	assert.Equal(t, "Need more documents", updatedE.VerificationReason)

	assert.Len(t, auditor.eventsForAction("evidence.needs_review"), 1)
}

func TestVerificationHistoryRecorded(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	verifierID := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	userChecker.addUser(orgID, verifierID)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	_, _, err := svc.VerifyEvidence(context.Background(), VerifyEvidenceParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		VerifierID:     verifierID,
		Reason:         "looks good",
		Method:         "manual",
	})
	require.NoError(t, err)

	_, _, err = svc.RejectEvidence(context.Background(), VerifyEvidenceParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		VerifierID:     verifierID,
		Reason:         "actually bad",
		Method:         "manual",
	})
	require.NoError(t, err)

	history, total, err := svc.ListVerificationHistory(context.Background(), ListVerificationHistoryParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		Limit:          20,
		Offset:         0,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, history, 2)
	assert.Equal(t, evidencedomain.VerificationStatusVerified, history[0].Status)
	assert.Equal(t, evidencedomain.VerificationStatusRejected, history[1].Status)
}

func TestUpdateEvidenceMetadata_Success(t *testing.T) {
	svc, repo, caseFinder, userChecker, auditor := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	actorID := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	userChecker.addUser(orgID, actorID)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	updatedE, err := svc.UpdateEvidenceMetadata(context.Background(), UpdateEvidenceMetadataParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		Metadata:       map[string]any{"key": "value"},
		ActorID:        actorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "value", updatedE.Metadata["key"])
	assert.Len(t, auditor.eventsForAction("evidence.metadata_updated"), 1)
}

func TestUpdateEvidenceMetadata_NotFound(t *testing.T) {
	svc, _, _, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	actorID := uuid.New()
	userChecker.addUser(orgID, actorID)

	_, err := svc.UpdateEvidenceMetadata(context.Background(), UpdateEvidenceMetadataParams{
		OrganizationID: orgID,
		EvidenceID:     uuid.New(),
		Metadata:       map[string]any{"key": "value"},
		ActorID:        actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEvidenceNotFound)
}

func TestUpdateEvidenceMetadata_NilActor(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	_, err := svc.UpdateEvidenceMetadata(context.Background(), UpdateEvidenceMetadataParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		Metadata:       map[string]any{"key": "value"},
		ActorID:        uuid.Nil,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEvidenceInput)
}

func TestUpdateEvidenceMetadata_InvalidMetadataKey(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	actorID := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	userChecker.addUser(orgID, actorID)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	_, err := svc.UpdateEvidenceMetadata(context.Background(), UpdateEvidenceMetadataParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		Metadata:       map[string]any{"key.sub": "value"},
		ActorID:        actorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEvidenceInput)
}

func TestUploadDocument_Success(t *testing.T) {
	svc, repo, caseFinder, userChecker, auditor := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	reader := bytes.NewReader([]byte("test document content"))
	result, err := svc.UploadDocument(context.Background(), UploadDocumentParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		FileName:       "test.pdf",
		ContentType:    "application/pdf",
		Size:           reader.Size(),
		Reader:         reader,
		UploadedBy:     uploadedBy,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, result.Document.ID)
	assert.Equal(t, e.ID, result.Document.EvidenceID)
	assert.Equal(t, "test.pdf", result.Document.FileName)
	assert.Equal(t, "application/pdf", result.Document.ContentType)
	assert.NotEmpty(t, result.Document.Checksum)
	assert.NotEmpty(t, result.StorageKey)

	assert.Len(t, auditor.eventsForAction("evidence.document_uploaded"), 1)
	assert.Equal(t, "success", auditor.eventsForAction("evidence.document_uploaded")[0].Outcome)
}

func TestUploadDocument_EvidenceNotFound(t *testing.T) {
	svc, _, _, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)

	reader := bytes.NewReader([]byte("content"))
	_, err := svc.UploadDocument(context.Background(), UploadDocumentParams{
		OrganizationID: orgID,
		EvidenceID:     uuid.New(),
		FileName:       "test.pdf",
		ContentType:    "application/pdf",
		Size:           reader.Size(),
		Reader:         reader,
		UploadedBy:     uploadedBy,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEvidenceNotFound)
}

func TestUploadDocument_UnsupportedContentType(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	reader := bytes.NewReader([]byte("content"))
	_, err := svc.UploadDocument(context.Background(), UploadDocumentParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		FileName:       "test.exe",
		ContentType:    "application/x-msdownload",
		Size:           reader.Size(),
		Reader:         reader,
		UploadedBy:     uploadedBy,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEvidenceInput)
}

func TestUploadDocument_NilUploader(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	reader := bytes.NewReader([]byte("content"))
	_, err := svc.UploadDocument(context.Background(), UploadDocumentParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		FileName:       "test.pdf",
		ContentType:    "application/pdf",
		Size:           reader.Size(),
		Reader:         reader,
		UploadedBy:     uuid.Nil,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEvidenceInput)
}

func TestUploadDocument_DangerousFilename(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	reader := bytes.NewReader([]byte("content"))
	result, err := svc.UploadDocument(context.Background(), UploadDocumentParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		FileName:       "../../etc/passwd",
		ContentType:    "application/pdf",
		Size:           reader.Size(),
		Reader:         reader,
		UploadedBy:     uploadedBy,
	})
	require.NoError(t, err)
	assert.NotEqual(t, "../../etc/passwd", result.Document.FileName)
	assert.NotContains(t, result.Document.FileName, "..")
	assert.NotContains(t, result.Document.FileName, "/")
}

func TestGetDocumentStream_DocumentNotFound(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	_, err := svc.GetDocumentStream(context.Background(), orgID, e.ID, uploadedBy)
	require.Error(t, err)
}

func TestListDocuments_Success(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	reader := bytes.NewReader([]byte("document content"))
	result, err := svc.UploadDocument(context.Background(), UploadDocumentParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		FileName:       "doc1.pdf",
		ContentType:    "application/pdf",
		Size:           reader.Size(),
		Reader:         reader,
		UploadedBy:     uploadedBy,
	})
	require.NoError(t, err)

	docs, total, err := svc.ListDocuments(context.Background(), ListDocumentsParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		DownloaderID:   uploadedBy,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, docs, 1)
	assert.Equal(t, result.Document.ID, docs[0].ID)
	assert.Equal(t, "doc1.pdf", docs[0].FileName)
}

func TestListDocuments_EvidenceNotFound(t *testing.T) {
	svc, _, _, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)

	_, _, err := svc.ListDocuments(context.Background(), ListDocumentsParams{
		OrganizationID: orgID,
		EvidenceID:     uuid.New(),
		DownloaderID:   uploadedBy,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEvidenceNotFound)
}

func TestListDocuments_NilDownloader(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	_, _, err := svc.ListDocuments(context.Background(), ListDocumentsParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		DownloaderID:   uuid.Nil,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEvidenceInput)
}

func TestListDocuments_UserNotInOrg(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	otherOrg := uuid.New()
	otherUser := uuid.New()
	userChecker.addUser(otherOrg, otherUser)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	_, _, err := svc.ListDocuments(context.Background(), ListDocumentsParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		DownloaderID:   otherUser,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestListDocuments_EmptyResults(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	docs, total, err := svc.ListDocuments(context.Background(), ListDocumentsParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		DownloaderID:   uploadedBy,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, docs)
}

func TestGetDocumentStream_DownloadAudit(t *testing.T) {
	svc, repo, caseFinder, userChecker, auditor := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	reader := bytes.NewReader([]byte("audit test content"))
	result, err := svc.UploadDocument(context.Background(), UploadDocumentParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		FileName:       "audit_test.pdf",
		ContentType:    "application/pdf",
		Size:           reader.Size(),
		Reader:         reader,
		UploadedBy:     uploadedBy,
	})
	require.NoError(t, err)

	dlResult, err := svc.GetDocumentStream(context.Background(), orgID, e.ID, uploadedBy)
	require.NoError(t, err)
	dlResult.Content.Close()

	uploadEvents := auditor.eventsForAction("evidence.document_uploaded")
	assert.Len(t, uploadEvents, 1)
	assert.Equal(t, "success", uploadEvents[0].Outcome)

	downloadEvents := auditor.eventsForAction("evidence.document_downloaded")
	assert.Len(t, downloadEvents, 1)
	assert.Equal(t, "success", downloadEvents[0].Outcome)
	assert.Equal(t, "evidence_document", downloadEvents[0].Resource)
	if downloadEvents[0].ResourceID == nil || *downloadEvents[0].ResourceID != result.Document.ID.String() {
		t.Errorf("expected resource ID %s, got %v", result.Document.ID.String(), downloadEvents[0].ResourceID)
	}
}

func TestDeleteDocument_Success(t *testing.T) {
	svc, repo, caseFinder, userChecker, auditor := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	reader := bytes.NewReader([]byte("delete me content"))
	result, err := svc.UploadDocument(context.Background(), UploadDocumentParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		FileName:       "to_delete.pdf",
		ContentType:    "application/pdf",
		Size:           reader.Size(),
		Reader:         reader,
		UploadedBy:     uploadedBy,
	})
	require.NoError(t, err)

	err = svc.DeleteDocument(context.Background(), DeleteDocumentParams{
		OrganizationID: orgID,
		DocumentID:     result.Document.ID,
		DeletedBy:      uploadedBy,
	})
	require.NoError(t, err)

	_, err = svc.GetDocumentStream(context.Background(), orgID, e.ID, uploadedBy)
	require.Error(t, err)

	docs, total, err := svc.ListDocuments(context.Background(), ListDocumentsParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		DownloaderID:   uploadedBy,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, docs)

	deleteEvents := auditor.eventsForAction("evidence.document_deleted")
	assert.Len(t, deleteEvents, 1)
	assert.Equal(t, "success", deleteEvents[0].Outcome)
	assert.Equal(t, "evidence_document", deleteEvents[0].Resource)
}

func TestDeleteDocument_NotFound(t *testing.T) {
	svc, _, _, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	deletedBy := uuid.New()
	userChecker.addUser(orgID, deletedBy)

	err := svc.DeleteDocument(context.Background(), DeleteDocumentParams{
		OrganizationID: orgID,
		DocumentID:     uuid.New(),
		DeletedBy:      deletedBy,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, evidencedomain.ErrDocumentNotFound)
}

func TestDeleteDocument_NilDeleter(t *testing.T) {
	svc, _, _, _, _ := setupTestService(t)

	orgID := uuid.New()
	err := svc.DeleteDocument(context.Background(), DeleteDocumentParams{
		OrganizationID: orgID,
		DocumentID:     uuid.New(),
		DeletedBy:      uuid.Nil,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEvidenceInput)
}

func TestDeleteDocument_UserNotInOrg(t *testing.T) {
	svc, repo, caseFinder, userChecker, _ := setupTestService(t)

	orgID := uuid.New()
	uploadedBy := uuid.New()
	userChecker.addUser(orgID, uploadedBy)
	otherOrg := uuid.New()
	otherUser := uuid.New()
	userChecker.addUser(otherOrg, otherUser)
	c, _ := domain.NewCase(orgID, uploadedBy, "Test", "Desc", domain.ServiceTypeGeneral, domain.PriorityNormal, nil)
	caseFinder.addCase(c)
	e := createTestEvidence(t, repo, orgID, c.ID, uploadedBy)

	reader := bytes.NewReader([]byte("content"))
	result, err := svc.UploadDocument(context.Background(), UploadDocumentParams{
		OrganizationID: orgID,
		EvidenceID:     e.ID,
		FileName:       "test.pdf",
		ContentType:    "application/pdf",
		Size:           reader.Size(),
		Reader:         reader,
		UploadedBy:     uploadedBy,
	})
	require.NoError(t, err)

	err = svc.DeleteDocument(context.Background(), DeleteDocumentParams{
		OrganizationID: orgID,
		DocumentID:     result.Document.ID,
		DeletedBy:      otherUser,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}
