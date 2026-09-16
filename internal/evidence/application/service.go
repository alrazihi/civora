package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	casesdomain "github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database"
	evidencedomain "github.com/alrazihi/civora/internal/evidence/domain"
	"github.com/alrazihi/civora/internal/evidence/infrastructure/storage"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrEvidenceNotFound = errors.New("evidence not found")
	ErrEvidenceInput    = errors.New("invalid evidence input")
	ErrCaseNotFound     = errors.New("case not found")
	ErrCaseClosed       = errors.New("case is closed")
	ErrUserNotFound     = errors.New("user not found")
	ErrVerifierNotFound = errors.New("verifier not found")
)

type EvidenceService struct {
	repo        evidencedomain.EvidenceRepository
	caseRepo    shared.CaseFinder
	userChecker shared.UserChecker
	auditor     auditdomain.EventRecorder
	storage     storage.StorageProvider
}

func NewEvidenceService(repo evidencedomain.EvidenceRepository, caseRepo shared.CaseFinder, userChecker shared.UserChecker, auditor auditdomain.EventRecorder, storageProvider storage.StorageProvider) *EvidenceService {
	return &EvidenceService{repo: repo, caseRepo: caseRepo, userChecker: userChecker, auditor: auditor, storage: storageProvider}
}

type AddEvidenceParams struct {
	OrganizationID   uuid.UUID
	ServiceRequestID uuid.UUID
	Type             evidencedomain.EvidenceType
	CustomType       string
	Description      string
	StorageReference string
	UploadedBy       uuid.UUID
	PersonID         *uuid.UUID
	Source           evidencedomain.EvidenceSource
	Metadata         map[string]any
}

func (s *EvidenceService) AddEvidence(ctx context.Context, params AddEvidenceParams) (*evidencedomain.Evidence, error) {
	c, err := s.caseRepo.FindByID(ctx, params.OrganizationID, params.ServiceRequestID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaseNotFound, err)
	}
	if c.OrganizationID != params.OrganizationID {
		return nil, ErrCaseNotFound
	}
	if casesdomain.IsClosed(c.Status) {
		return nil, ErrCaseClosed
	}

	if params.PersonID != nil {
		// person existence is validated implicitly via FK; no extra lookup needed
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.UploadedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to validate uploader: %w", err)
	}
	if !valid {
		return nil, ErrUserNotFound
	}

	e, err := evidencedomain.NewEvidence(evidencedomain.EvidenceParams{
		OrganizationID:   params.OrganizationID,
		ServiceRequestID: params.ServiceRequestID,
		Type:             params.Type,
		CustomType:       params.CustomType,
		Description:      params.Description,
		StorageReference: params.StorageReference,
		UploadedBy:       params.UploadedBy,
		PersonID:         params.PersonID,
		Source:           params.Source,
		Metadata:         params.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEvidenceInput, err)
	}

	var result *evidencedomain.Evidence
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, e); err != nil {
			return fmt.Errorf("failed to save evidence: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: e.OrganizationID,
				ActorID:        &params.UploadedBy,
				Action:         "evidence.added",
				Resource:       "evidence",
				ResourceID:     shared.StrPtr(e.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": e.ServiceRequestID.String(),
					"evidence_type":      string(e.Type),
					"source":             string(e.Source),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = e
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *EvidenceService) GetEvidence(ctx context.Context, orgID, id uuid.UUID) (*evidencedomain.Evidence, error) {
	e, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrEvidenceNotFound
	}
	return e, nil
}

func (s *EvidenceService) ListEvidence(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*evidencedomain.Evidence, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.CountByServiceRequest(ctx, orgID, serviceRequestID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count evidence: %w", err)
	}

	items, err := s.repo.FindByServiceRequest(ctx, orgID, serviceRequestID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list evidence: %w", err)
	}

	return items, total, nil
}

type VerifyEvidenceParams struct {
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	VerifierID     uuid.UUID
	Reason         string
	Method         string
}

func (s *EvidenceService) VerifyEvidence(ctx context.Context, params VerifyEvidenceParams) (*evidencedomain.Evidence, *evidencedomain.VerificationRecord, error) {
	return s.applyVerification(ctx, params.EvidenceID, params.OrganizationID, params.VerifierID,
		func(e *evidencedomain.Evidence) (*evidencedomain.VerificationRecord, error) {
			return e.Verify(params.VerifierID, params.Reason, params.Method)
		}, "evidence.verified", params)
}

func (s *EvidenceService) RejectEvidence(ctx context.Context, params VerifyEvidenceParams) (*evidencedomain.Evidence, *evidencedomain.VerificationRecord, error) {
	return s.applyVerification(ctx, params.EvidenceID, params.OrganizationID, params.VerifierID,
		func(e *evidencedomain.Evidence) (*evidencedomain.VerificationRecord, error) {
			return e.Reject(params.VerifierID, params.Reason, params.Method)
		}, "evidence.rejected", params)
}

func (s *EvidenceService) MarkEvidenceForReview(ctx context.Context, params VerifyEvidenceParams) (*evidencedomain.Evidence, *evidencedomain.VerificationRecord, error) {
	return s.applyVerification(ctx, params.EvidenceID, params.OrganizationID, params.VerifierID,
		func(e *evidencedomain.Evidence) (*evidencedomain.VerificationRecord, error) {
			return e.MarkForReview(params.VerifierID, params.Reason, params.Method)
		}, "evidence.needs_review", params)
}

func (s *EvidenceService) applyVerification(
	ctx context.Context,
	evidenceID, orgID, actorID uuid.UUID,
	verifyFn func(e *evidencedomain.Evidence) (*evidencedomain.VerificationRecord, error),
	auditAction string,
	params VerifyEvidenceParams,
) (*evidencedomain.Evidence, *evidencedomain.VerificationRecord, error) {
	if actorID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: verifier ID is required", ErrEvidenceInput)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, orgID, actorID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to validate verifier: %w", err)
	}
	if !valid {
		return nil, nil, ErrVerifierNotFound
	}

	e, err := s.repo.FindByID(ctx, orgID, evidenceID)
	if err != nil {
		return nil, nil, ErrEvidenceNotFound
	}

	rec, err := verifyFn(e)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrEvidenceInput, err)
	}

	var resultEvidence *evidencedomain.Evidence
	var resultRec *evidencedomain.VerificationRecord
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateVerificationTx(ctx, tx, e); err != nil {
			return fmt.Errorf("failed to update verification: %w", err)
		}
		if err := s.repo.SaveVerificationHistoryTx(ctx, tx, rec); err != nil {
			return fmt.Errorf("failed to save verification history: %w", err)
		}

		if s.auditor != nil {
			actorPtr := &actorID
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: orgID,
				ActorID:        actorPtr,
				Action:         auditAction,
				Resource:       "evidence",
				ResourceID:     shared.StrPtr(e.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id":  e.ServiceRequestID.String(),
					"verification_status": string(e.VerificationStatus),
					"reason":              params.Reason,
					"method":              params.Method,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		resultEvidence = e
		resultRec = rec
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return resultEvidence, resultRec, nil
}

type UpdateEvidenceMetadataParams struct {
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	Metadata       map[string]any
	ActorID        uuid.UUID
}

func (s *EvidenceService) UpdateEvidenceMetadata(ctx context.Context, params UpdateEvidenceMetadataParams) (*evidencedomain.Evidence, error) {
	if params.ActorID == uuid.Nil {
		return nil, fmt.Errorf("%w: actor ID is required", ErrEvidenceInput)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.ActorID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate actor: %w", err)
	}
	if !valid {
		return nil, ErrUserNotFound
	}

	e, err := s.repo.FindByID(ctx, params.OrganizationID, params.EvidenceID)
	if err != nil {
		return nil, ErrEvidenceNotFound
	}

	if err := e.UpdateMetadata(params.ActorID, params.Metadata, ""); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEvidenceInput, err)
	}

	var result *evidencedomain.Evidence
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, e); err != nil {
			return fmt.Errorf("failed to update evidence: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: e.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "evidence.metadata_updated",
				Resource:       "evidence",
				ResourceID:     shared.StrPtr(e.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": e.ServiceRequestID.String(),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = e
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

type ListVerificationHistoryParams struct {
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	Limit          int
	Offset         int
}

type ListDocumentsParams struct {
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	DownloaderID   uuid.UUID
	Limit          int
	Offset         int
}

func (s *EvidenceService) ListVerificationHistory(ctx context.Context, params ListVerificationHistoryParams) ([]*evidencedomain.VerificationRecord, int, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	total, err := s.repo.CountVerificationHistory(ctx, params.OrganizationID, params.EvidenceID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count verification history: %w", err)
	}

	items, err := s.repo.FindVerificationHistory(ctx, params.OrganizationID, params.EvidenceID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list verification history: %w", err)
	}

	return items, total, nil
}

type UploadDocumentParams struct {
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	FileName       string
	ContentType    string
	Size           int64
	Reader         io.Reader
	UploadedBy     uuid.UUID
}

type UploadDocumentResult struct {
	Document   *evidencedomain.Document
	StorageKey string
	Checksum   string
}

func (s *EvidenceService) UploadDocument(ctx context.Context, params UploadDocumentParams) (*UploadDocumentResult, error) {
	if params.UploadedBy == uuid.Nil {
		return nil, fmt.Errorf("%w: uploaded_by is required", ErrEvidenceInput)
	}
	if params.EvidenceID == uuid.Nil {
		return nil, fmt.Errorf("%w: evidence ID is required", ErrEvidenceInput)
	}
	if params.FileName == "" {
		return nil, fmt.Errorf("%w: file name is required", ErrEvidenceInput)
	}
	if params.Size <= 0 {
		return nil, fmt.Errorf("%w: file size must be positive", ErrEvidenceInput)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.UploadedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to validate uploader: %w", err)
	}
	if !valid {
		return nil, ErrUserNotFound
	}

	_, err = s.repo.FindByID(ctx, params.OrganizationID, params.EvidenceID)
	if err != nil {
		return nil, ErrEvidenceNotFound
	}

	storageKey := storage.GenerateStorageKey(params.EvidenceID, params.FileName)

	metadata, err := s.storage.Store(ctx, storageKey, params.Reader, params.Size)
	if err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	if metadata.SizeBytes != params.Size {
		_ = s.storage.Delete(ctx, storageKey)
		return nil, fmt.Errorf("%w: size mismatch: declared %d, actual %d", ErrEvidenceInput, params.Size, metadata.SizeBytes)
	}

	contentType, err := storage.ValidateContentType(params.ContentType)
	if err != nil {
		_ = s.storage.Delete(ctx, storageKey)
		return nil, fmt.Errorf("%w: %w", ErrEvidenceInput, err)
	}

	sanitizedName, err := storage.SanitizeFileName(params.FileName)
	if err != nil {
		_ = s.storage.Delete(ctx, storageKey)
		return nil, fmt.Errorf("%w: %w", ErrEvidenceInput, err)
	}

	doc, err := evidencedomain.NewDocument(evidencedomain.DocumentParams{
		OrganizationID: params.OrganizationID,
		EvidenceID:     params.EvidenceID,
		FileName:       sanitizedName,
		ContentType:    contentType,
		SizeBytes:      metadata.SizeBytes,
		Checksum:       metadata.Checksum,
		StorageKey:     storageKey,
		StorageProv:    evidencedomain.StorageProviderLocal,
		UploadedBy:     params.UploadedBy,
	})
	if err != nil {
		_ = s.storage.Delete(ctx, storageKey)
		return nil, fmt.Errorf("%w: %v", ErrEvidenceInput, err)
	}

	var result *UploadDocumentResult
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveDocumentTx(ctx, tx, doc); err != nil {
			return fmt.Errorf("failed to save document metadata: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.UploadedBy,
				Action:         "evidence.document_uploaded",
				Resource:       "evidence_document",
				ResourceID:     shared.StrPtr(doc.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"evidence_id":  params.EvidenceID.String(),
					"file_name":    sanitizedName,
					"content_type": contentType,
					"size_bytes":   metadata.SizeBytes,
					"checksum":     metadata.Checksum,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = &UploadDocumentResult{
			Document:   doc,
			StorageKey: storageKey,
			Checksum:   metadata.Checksum,
		}
		return nil
	})
	if err != nil {
		_ = s.storage.Delete(ctx, storageKey)
		return nil, err
	}

	return result, nil
}

type DownloadDocumentResult struct {
	Document    *evidencedomain.Document
	Content     io.ReadCloser
	ContentType string
	FileName    string
}

func (s *EvidenceService) GetDocumentStream(ctx context.Context, orgID, evidenceID uuid.UUID, downloaderID uuid.UUID) (*DownloadDocumentResult, error) {
	if downloaderID == uuid.Nil {
		return nil, fmt.Errorf("%w: downloader ID is required", ErrEvidenceInput)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, orgID, downloaderID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate downloader: %w", err)
	}
	if !valid {
		return nil, ErrUserNotFound
	}

	_, err = s.repo.FindByID(ctx, orgID, evidenceID)
	if err != nil {
		return nil, ErrEvidenceNotFound
	}

	doc, err := s.repo.FindDocumentByEvidence(ctx, orgID, evidenceID)
	if err != nil {
		return nil, evidencedomain.ErrDocumentNotFound
	}

	reader, err := s.storage.Retrieve(ctx, doc.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve file: %w", err)
	}

	if s.auditor != nil {
		_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: orgID,
			ActorID:        &downloaderID,
			Action:         "evidence.document_downloaded",
			Resource:       "evidence_document",
			ResourceID:     shared.StrPtr(doc.ID.String()),
			Outcome:        "success",
			RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
			Metadata: map[string]interface{}{
				"evidence_id":  evidenceID.String(),
				"file_name":    doc.FileName,
				"content_type": doc.ContentType,
				"size_bytes":   doc.SizeBytes,
				"checksum":     doc.Checksum,
			},
		})
	}

	return &DownloadDocumentResult{
		Document:    doc,
		Content:     reader,
		ContentType: doc.ContentType,
		FileName:    doc.FileName,
	}, nil
}

func (s *EvidenceService) ListDocuments(ctx context.Context, params ListDocumentsParams) ([]*evidencedomain.Document, int, error) {
	if params.DownloaderID == uuid.Nil {
		return nil, 0, fmt.Errorf("%w: downloader ID is required", ErrEvidenceInput)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.DownloaderID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to validate downloader: %w", err)
	}
	if !valid {
		return nil, 0, ErrUserNotFound
	}

	if _, err := s.repo.FindByID(ctx, params.OrganizationID, params.EvidenceID); err != nil {
		return nil, 0, ErrEvidenceNotFound
	}

	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	docs, total, err := s.repo.ListDocuments(ctx, params.OrganizationID, params.EvidenceID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list documents: %w", err)
	}

	return docs, total, nil
}

type DeleteDocumentParams struct {
	OrganizationID uuid.UUID
	DocumentID     uuid.UUID
	DeletedBy      uuid.UUID
}

func (s *EvidenceService) DeleteDocument(ctx context.Context, params DeleteDocumentParams) error {
	if params.DeletedBy == uuid.Nil {
		return fmt.Errorf("%w: deleted_by is required", ErrEvidenceInput)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.DeletedBy)
	if err != nil {
		return fmt.Errorf("failed to validate deleter: %w", err)
	}
	if !valid {
		return ErrUserNotFound
	}

	doc, err := s.repo.FindDocumentByID(ctx, params.OrganizationID, params.DocumentID)
	if err != nil {
		return evidencedomain.ErrDocumentNotFound
	}

	if err := s.storage.Delete(ctx, doc.StorageKey); err != nil {
		return fmt.Errorf("failed to delete file from storage: %w", err)
	}

	if err := s.repo.DeleteDocument(ctx, params.OrganizationID, params.DocumentID); err != nil {
		return fmt.Errorf("failed to delete document metadata: %w", err)
	}

	if s.auditor != nil {
		_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: params.OrganizationID,
			ActorID:        &params.DeletedBy,
			Action:         "evidence.document_deleted",
			Resource:       "evidence_document",
			ResourceID:     shared.StrPtr(doc.ID.String()),
			Outcome:        "success",
			RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
			Metadata: map[string]interface{}{
				"evidence_id":  doc.EvidenceID.String(),
				"file_name":    doc.FileName,
				"content_type": doc.ContentType,
				"size_bytes":   doc.SizeBytes,
				"checksum":     doc.Checksum,
			},
		})
	}

	return nil
}
