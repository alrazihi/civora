package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrDocumentNotFound     = errors.New("document not found")
	ErrDocumentInvalidInput = errors.New("invalid document input")
)

type StorageProvider string

const (
	StorageProviderLocal StorageProvider = "LOCAL"
	StorageProviderS3    StorageProvider = "S3"
	StorageProviderGS    StorageProvider = "GS"
	StorageProviderAzure StorageProvider = "AZURE"
)

type Document struct {
	ID              uuid.UUID       `json:"id"`
	EvidenceID      uuid.UUID       `json:"evidence_id"`
	OrganizationID  uuid.UUID       `json:"organization_id"`
	FileName        string          `json:"file_name"`
	ContentType     string          `json:"content_type"`
	SizeBytes       int64           `json:"size_bytes"`
	Checksum        string          `json:"checksum"`
	StorageKey      string          `json:"-"` // internal, never serialized
	StorageProvider StorageProvider `json:"storage_provider"`
	UploadedBy      uuid.UUID       `json:"uploaded_by"`
	UploadedAt      time.Time       `json:"uploaded_at"`
	CreatedAt       time.Time       `json:"created_at"`
}

type DocumentParams struct {
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	FileName       string
	ContentType    string
	SizeBytes      int64
	Checksum       string
	StorageKey     string
	StorageProv    StorageProvider
	UploadedBy     uuid.UUID
}

const MaxDocumentFileNameLength = 255

func NewDocument(params DocumentParams) (*Document, error) {
	if params.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization ID is required", ErrDocumentInvalidInput)
	}
	if params.EvidenceID == uuid.Nil {
		return nil, fmt.Errorf("%w: evidence ID is required", ErrDocumentInvalidInput)
	}
	if params.UploadedBy == uuid.Nil {
		return nil, fmt.Errorf("%w: uploaded_by is required", ErrDocumentInvalidInput)
	}
	if params.FileName == "" || len(params.FileName) > MaxDocumentFileNameLength {
		return nil, fmt.Errorf("%w: file name is required and must not exceed %d characters", ErrDocumentInvalidInput, MaxDocumentFileNameLength)
	}
	if !isValidStorageProvider(params.StorageProv) {
		return nil, fmt.Errorf("%w: invalid storage provider %q", ErrDocumentInvalidInput, params.StorageProv)
	}
	if params.Checksum == "" {
		return nil, fmt.Errorf("%w: checksum is required", ErrDocumentInvalidInput)
	}
	if params.SizeBytes <= 0 {
		return nil, fmt.Errorf("%w: size_bytes must be positive", ErrDocumentInvalidInput)
	}
	if params.StorageKey == "" {
		return nil, fmt.Errorf("%w: storage key is required", ErrDocumentInvalidInput)
	}

	now := time.Now().UTC()
	return &Document{
		ID:              uuid.New(),
		EvidenceID:      params.EvidenceID,
		OrganizationID:  params.OrganizationID,
		FileName:        params.FileName,
		ContentType:     params.ContentType,
		SizeBytes:       params.SizeBytes,
		Checksum:        params.Checksum,
		StorageKey:      params.StorageKey,
		StorageProvider: params.StorageProv,
		UploadedBy:      params.UploadedBy,
		UploadedAt:      now,
		CreatedAt:       now,
	}, nil
}

func isValidStorageProvider(p StorageProvider) bool {
	switch p {
	case StorageProviderLocal, StorageProviderS3, StorageProviderGS, StorageProviderAzure:
		return true
	default:
		return false
	}
}
