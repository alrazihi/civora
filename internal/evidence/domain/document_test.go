package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewDocument_Success(t *testing.T) {
	params := DocumentParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		FileName:       "test.pdf",
		ContentType:    "application/pdf",
		SizeBytes:      1024,
		Checksum:       "abc123",
		StorageKey:     "2024/01/02/uuid.pdf",
		StorageProv:    StorageProviderLocal,
		UploadedBy:     uuid.New(),
	}
	doc, err := NewDocument(params)
	if err != nil {
		t.Fatalf("NewDocument failed: %v", err)
	}
	if doc.ID == uuid.Nil {
		t.Error("ID should not be empty")
	}
	if doc.FileName != "test.pdf" {
		t.Errorf("FileName = %q, want %q", doc.FileName, "test.pdf")
	}
	if doc.SizeBytes != 1024 {
		t.Errorf("SizeBytes = %d, want 1024", doc.SizeBytes)
	}
	if doc.StorageKey != "2024/01/02/uuid.pdf" {
		t.Errorf("StorageKey = %q, want not exposed", doc.StorageKey)
	}
	if doc.UploadedAt.IsZero() {
		t.Error("UploadedAt should be set")
	}
	if doc.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestNewDocument_ZeroOrganizationID(t *testing.T) {
	params := DocumentParams{
		EvidenceID:  uuid.New(),
		FileName:    "test.pdf",
		SizeBytes:   1024,
		Checksum:    "abc123",
		StorageKey:  "key",
		StorageProv: StorageProviderLocal,
		UploadedBy:  uuid.New(),
	}
	_, err := NewDocument(params)
	if err == nil {
		t.Error("expected error for zero organization ID")
	}
}

func TestNewDocument_ZeroEvidenceID(t *testing.T) {
	params := DocumentParams{
		OrganizationID: uuid.New(),
		FileName:       "test.pdf",
		SizeBytes:      1024,
		Checksum:       "abc123",
		StorageKey:     "key",
		StorageProv:    StorageProviderLocal,
		UploadedBy:     uuid.New(),
	}
	_, err := NewDocument(params)
	if err == nil {
		t.Error("expected error for zero evidence ID")
	}
}

func TestNewDocument_ZeroUploader(t *testing.T) {
	params := DocumentParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		FileName:       "test.pdf",
		SizeBytes:      1024,
		Checksum:       "abc123",
		StorageKey:     "key",
		StorageProv:    StorageProviderLocal,
		UploadedBy:     uuid.Nil,
	}
	_, err := NewDocument(params)
	if err == nil {
		t.Error("expected error for zero uploader ID")
	}
}

func TestNewDocument_EmptyFileName(t *testing.T) {
	params := DocumentParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		FileName:       "",
		SizeBytes:      1024,
		Checksum:       "abc123",
		StorageKey:     "key",
		StorageProv:    StorageProviderLocal,
		UploadedBy:     uuid.New(),
	}
	_, err := NewDocument(params)
	if err == nil {
		t.Error("expected error for empty filename")
	}
}

func TestNewDocument_FileNameTooLong(t *testing.T) {
	params := DocumentParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		FileName:       strings.Repeat("a", MaxDocumentFileNameLength+1),
		SizeBytes:      1024,
		Checksum:       "abc123",
		StorageKey:     "key",
		StorageProv:    StorageProviderLocal,
		UploadedBy:     uuid.New(),
	}
	_, err := NewDocument(params)
	if err == nil {
		t.Error("expected error for filename too long")
	}
}

func TestNewDocument_InvalidStorageProvider(t *testing.T) {
	params := DocumentParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		FileName:       "test.pdf",
		SizeBytes:      1024,
		Checksum:       "abc123",
		StorageKey:     "key",
		StorageProv:    StorageProvider("INVALID"),
		UploadedBy:     uuid.New(),
	}
	_, err := NewDocument(params)
	if err == nil {
		t.Error("expected error for invalid storage provider")
	}
}

func TestNewDocument_EmptyChecksum(t *testing.T) {
	params := DocumentParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		FileName:       "test.pdf",
		SizeBytes:      1024,
		Checksum:       "",
		StorageKey:     "key",
		StorageProv:    StorageProviderLocal,
		UploadedBy:     uuid.New(),
	}
	_, err := NewDocument(params)
	if err == nil {
		t.Error("expected error for empty checksum")
	}
}

func TestNewDocument_NegativeSize(t *testing.T) {
	params := DocumentParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		FileName:       "test.pdf",
		SizeBytes:      -1,
		Checksum:       "abc123",
		StorageKey:     "key",
		StorageProv:    StorageProviderLocal,
		UploadedBy:     uuid.New(),
	}
	_, err := NewDocument(params)
	if err == nil {
		t.Error("expected error for negative size")
	}
}

func TestNewDocument_ZeroSize(t *testing.T) {
	params := DocumentParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		FileName:       "test.pdf",
		SizeBytes:      0,
		Checksum:       "abc123",
		StorageKey:     "key",
		StorageProv:    StorageProviderLocal,
		UploadedBy:     uuid.New(),
	}
	_, err := NewDocument(params)
	if err == nil {
		t.Error("expected error for zero size")
	}
}

func TestNewDocument_EmptyStorageKey(t *testing.T) {
	params := DocumentParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		FileName:       "test.pdf",
		SizeBytes:      1024,
		Checksum:       "abc123",
		StorageKey:     "",
		StorageProv:    StorageProviderLocal,
		UploadedBy:     uuid.New(),
	}
	_, err := NewDocument(params)
	if err == nil {
		t.Error("expected error for empty storage key")
	}
}

func TestIsValidStorageProvider(t *testing.T) {
	validProviders := []StorageProvider{StorageProviderLocal, StorageProviderS3, StorageProviderGS, StorageProviderAzure}
	for _, p := range validProviders {
		if !isValidStorageProvider(p) {
			t.Errorf("isValidStorageProvider(%q) = false, want true", p)
		}
	}

	invalidProviders := []StorageProvider{"INVALID", "", "local"}
	for _, p := range invalidProviders {
		if isValidStorageProvider(p) {
			t.Errorf("isValidStorageProvider(%q) = true, want false", p)
		}
	}
}

func TestMaxDocumentFileNameLength(t *testing.T) {
	if MaxDocumentFileNameLength != 255 {
		t.Errorf("MaxDocumentFileNameLength = %d, want 255", MaxDocumentFileNameLength)
	}
}

func TestDocumentUploadedAtBeforeCreatedAt(t *testing.T) {
	params := DocumentParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		FileName:       "test.pdf",
		SizeBytes:      1024,
		Checksum:       "abc123",
		StorageKey:     "key",
		StorageProv:    StorageProviderLocal,
		UploadedBy:     uuid.New(),
	}
	doc, err := NewDocument(params)
	if err != nil {
		t.Fatalf("NewDocument failed: %v", err)
	}
	if !doc.UploadedAt.Before(doc.CreatedAt.Add(1 * time.Millisecond)) {
		t.Error("UploadedAt should be <= CreatedAt")
	}
}

func TestDocumentStorageProviderValues(t *testing.T) {
	expectations := map[StorageProvider]string{
		StorageProviderLocal: "LOCAL",
		StorageProviderS3:    "S3",
		StorageProviderGS:    "GS",
		StorageProviderAzure: "AZURE",
	}
	for provider, expected := range expectations {
		if string(provider) != expected {
			t.Errorf("StorageProvider value = %q, want %q", string(provider), expected)
		}
	}
}
