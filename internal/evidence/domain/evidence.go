package domain

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

type EvidenceType string

const (
	EvidenceTypeIdentityDocument   EvidenceType = "IDENTITY_DOCUMENT"
	EvidenceTypeProofOfResidence   EvidenceType = "PROOF_OF_RESIDENCE"
	EvidenceTypeReferral           EvidenceType = "REFERRAL"
	EvidenceTypeSupportingDocument EvidenceType = "SUPPORTING_DOCUMENT"
	EvidenceTypePhotograph         EvidenceType = "PHOTOGRAPH"
	EvidenceTypeStaffNote          EvidenceType = "STAFF_NOTE"
)

var (
	ErrEvidenceNotFound     = errors.New("evidence not found")
	ErrEvidenceInvalidInput = errors.New("invalid evidence input")
)

type Evidence struct {
	ID               uuid.UUID    `json:"id"`
	OrganizationID   uuid.UUID    `json:"organization_id"`
	ServiceRequestID uuid.UUID    `json:"service_request_id"`
	Type             EvidenceType `json:"type"`
	Description      string       `json:"description"`
	StorageReference string       `json:"storage_reference"`
	UploadedBy       uuid.UUID    `json:"uploaded_by"`
	CreatedAt        time.Time    `json:"created_at"`
}

func validateStorageReference(reference string) error {
	if strings.IndexFunc(reference, unicode.IsControl) >= 0 {
		return errors.New("storage reference contains control characters")
	}

	parsed, err := url.Parse(reference)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("storage reference must be a valid object URI")
	}
	switch parsed.Scheme {
	case "s3", "gs", "azureblob", "civora":
	default:
		return errors.New("storage reference uses an unsupported scheme")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("storage reference must not contain credentials, query parameters, or fragments")
	}
	for _, segment := range strings.Split(parsed.Path, "/") {
		if segment == ".." {
			return errors.New("storage reference must not contain parent directory segments")
		}
	}
	return nil
}

func NewEvidence(orgID, serviceRequestID, uploadedBy uuid.UUID, evidenceType EvidenceType, description, storageReference string) (*Evidence, error) {
	if len(description) > 5000 {
		return nil, fmt.Errorf("%w: description exceeds maximum length of 5000 characters", ErrEvidenceInvalidInput)
	}
	if description == "" {
		return nil, fmt.Errorf("%w: description is required", ErrEvidenceInvalidInput)
	}
	if len(storageReference) > 500 {
		return nil, fmt.Errorf("%w: storage reference exceeds maximum length of 500 characters", ErrEvidenceInvalidInput)
	}
	if storageReference == "" {
		return nil, fmt.Errorf("%w: storage reference is required", ErrEvidenceInvalidInput)
	}
	if err := validateStorageReference(storageReference); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEvidenceInvalidInput, err)
	}
	now := time.Now().UTC()
	return &Evidence{
		ID:               uuid.New(),
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		Type:             evidenceType,
		Description:      description,
		StorageReference: storageReference,
		UploadedBy:       uploadedBy,
		CreatedAt:        now,
	}, nil
}
