package domain

import (
	"encoding/json"
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
	EvidenceTypeOther              EvidenceType = "OTHER"
)

type EvidenceSource string

const (
	EvidenceSourceUpload            EvidenceSource = "UPLOAD"
	EvidenceSourceExternalReference EvidenceSource = "EXTERNAL_REFERENCE"
	EvidenceSourceExternalAPI       EvidenceSource = "EXTERNAL_API"
	EvidenceSourceManual            EvidenceSource = "MANUAL"
)

type VerificationStatus string

const (
	VerificationStatusUnverified  VerificationStatus = "UNVERIFIED"
	VerificationStatusNeedsReview VerificationStatus = "NEEDS_REVIEW"
	VerificationStatusVerified    VerificationStatus = "VERIFIED"
	VerificationStatusRejected    VerificationStatus = "REJECTED"
)

var (
	ErrEvidenceNotFound     = errors.New("evidence not found")
	ErrEvidenceInvalidInput = errors.New("invalid evidence input")
	ErrVerificationInvalid  = errors.New("invalid verification operation")
	ErrMetadataTooLarge     = errors.New("metadata exceeds maximum size")
)

const MaxMetadataSize = 100_000

type Evidence struct {
	ID                 uuid.UUID          `json:"id"`
	OrganizationID     uuid.UUID          `json:"organization_id"`
	ServiceRequestID   uuid.UUID          `json:"service_request_id"`
	Type               EvidenceType       `json:"type"`
	CustomType         *string            `json:"custom_type,omitempty"`
	Description        string             `json:"description"`
	StorageReference   string             `json:"storage_reference"`
	UploadedBy         uuid.UUID          `json:"uploaded_by"`
	PersonID           *uuid.UUID         `json:"person_id,omitempty"`
	Source             EvidenceSource     `json:"source"`
	Metadata           map[string]any     `json:"metadata"`
	VerificationStatus VerificationStatus `json:"verification_status"`
	VerifiedBy         *uuid.UUID         `json:"verified_by,omitempty"`
	VerifiedAt         *time.Time         `json:"verified_at,omitempty"`
	VerificationReason string             `json:"verification_reason,omitempty"`
	VerificationMethod string             `json:"verification_method,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
}

type EvidenceParams struct {
	OrganizationID   uuid.UUID
	ServiceRequestID uuid.UUID
	Type             EvidenceType
	CustomType       string
	Description      string
	StorageReference string
	UploadedBy       uuid.UUID
	PersonID         *uuid.UUID
	Source           EvidenceSource
	Metadata         map[string]any
}

type VerificationRecord struct {
	ID             uuid.UUID          `json:"id"`
	EvidenceID     uuid.UUID          `json:"evidence_id"`
	OrganizationID uuid.UUID          `json:"organization_id"`
	Status         VerificationStatus `json:"status"`
	VerifierID     *uuid.UUID         `json:"verifier_id,omitempty"`
	VerifiedAt     time.Time          `json:"verified_at"`
	Reason         string             `json:"reason,omitempty"`
	Method         string             `json:"method,omitempty"`
	Notes          string             `json:"notes,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
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

func validateMetadata(metadata map[string]any) error {
	if len(metadata) == 0 {
		return nil
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("metadata is not JSON-serializable: %w", err)
	}
	if len(raw) > MaxMetadataSize {
		return fmt.Errorf("%w: exceeds %d bytes", ErrMetadataTooLarge, MaxMetadataSize)
	}
	for k := range metadata {
		if strings.ContainsAny(k, ".\"") {
			return fmt.Errorf("metadata keys must not contain '.' or '\"' characters")
		}
	}
	return nil
}

func NewEvidence(params EvidenceParams) (*Evidence, error) {
	if params.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization ID is required", ErrEvidenceInvalidInput)
	}
	if params.ServiceRequestID == uuid.Nil {
		return nil, fmt.Errorf("%w: service request ID is required", ErrEvidenceInvalidInput)
	}
	if params.UploadedBy == uuid.Nil {
		return nil, fmt.Errorf("%w: uploaded_by is required", ErrEvidenceInvalidInput)
	}
	if len(params.Description) > 5000 {
		return nil, fmt.Errorf("%w: description exceeds maximum length of 5000 characters", ErrEvidenceInvalidInput)
	}
	if params.Description == "" {
		return nil, fmt.Errorf("%w: description is required", ErrEvidenceInvalidInput)
	}
	if len(params.StorageReference) > 500 {
		return nil, fmt.Errorf("%w: storage reference exceeds maximum length of 500 characters", ErrEvidenceInvalidInput)
	}
	if params.StorageReference == "" {
		return nil, fmt.Errorf("%w: storage reference is required", ErrEvidenceInvalidInput)
	}
	if err := validateStorageReference(params.StorageReference); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEvidenceInvalidInput, err)
	}
	if err := validateMetadata(params.Metadata); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEvidenceInvalidInput, err)
	}
	if params.Type == EvidenceTypeOther && params.CustomType != "" {
		if len(params.CustomType) > 200 {
			return nil, fmt.Errorf("%w: custom type exceeds maximum length of 200 characters", ErrEvidenceInvalidInput)
		}
		for _, c := range params.CustomType {
			if unicode.IsControl(c) {
				return nil, fmt.Errorf("%w: custom type contains control characters", ErrEvidenceInvalidInput)
			}
		}
	}

	if params.Source == "" {
		params.Source = EvidenceSourceUpload
	}

	now := time.Now().UTC()
	e := &Evidence{
		ID:                 uuid.New(),
		OrganizationID:     params.OrganizationID,
		ServiceRequestID:   params.ServiceRequestID,
		Type:               params.Type,
		CustomType:         nil,
		Description:        params.Description,
		StorageReference:   params.StorageReference,
		UploadedBy:         params.UploadedBy,
		Source:             params.Source,
		Metadata:           copyMetadata(params.Metadata),
		VerificationStatus: VerificationStatusUnverified,
		CreatedAt:          now,
	}
	if params.Type == EvidenceTypeOther && params.CustomType != "" {
		e.CustomType = &params.CustomType
	}
	if params.PersonID != nil {
		e.PersonID = params.PersonID
	}

	return e, nil
}

func copyMetadata(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (e *Evidence) Verify(verifierID uuid.UUID, reason, method string) (*VerificationRecord, error) {
	if verifierID == uuid.Nil {
		return nil, fmt.Errorf("%w: verifier ID is required", ErrVerificationInvalid)
	}
	if e.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: evidence is not persisted", ErrVerificationInvalid)
	}
	return e.applyVerification(verifierID, VerificationStatusVerified, reason, method, "")
}

func (e *Evidence) Reject(reviewerID uuid.UUID, reason, method string) (*VerificationRecord, error) {
	if reviewerID == uuid.Nil {
		return nil, fmt.Errorf("%w: reviewer ID is required", ErrVerificationInvalid)
	}
	return e.applyVerification(reviewerID, VerificationStatusRejected, reason, method, "")
}

func (e *Evidence) MarkForReview(reviewerID uuid.UUID, reason, method string) (*VerificationRecord, error) {
	if reviewerID == uuid.Nil {
		return nil, fmt.Errorf("%w: reviewer ID is required", ErrVerificationInvalid)
	}
	return e.applyVerification(reviewerID, VerificationStatusNeedsReview, reason, method, "")
}

func (e *Evidence) applyVerification(actorID uuid.UUID, status VerificationStatus, reason, method, notes string) (*VerificationRecord, error) {
	if e.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: evidence is not persisted", ErrVerificationInvalid)
	}
	if !isValidVerificationStatus(status) {
		return nil, fmt.Errorf("%w: invalid status %q", ErrVerificationInvalid, status)
	}
	if !isValidVerificationTransition(e.VerificationStatus, status) {
		return nil, fmt.Errorf("%w: cannot transition from %s to %s", ErrVerificationInvalid, e.VerificationStatus, status)
	}
	now := time.Now().UTC()
	rec := &VerificationRecord{
		ID:             uuid.New(),
		EvidenceID:     e.ID,
		OrganizationID: e.OrganizationID,
		Status:         status,
		VerifierID:     &actorID,
		VerifiedAt:     now,
		Reason:         reason,
		Method:         method,
		Notes:          notes,
		CreatedAt:      now,
	}
	nowPtr := now
	e.VerificationStatus = status
	e.VerifiedBy = &actorID
	e.VerifiedAt = &nowPtr
	e.VerificationReason = reason
	e.VerificationMethod = method
	return rec, nil
}

func isValidVerificationTransition(from, to VerificationStatus) bool {
	switch from {
	case VerificationStatusUnverified:
		return to == VerificationStatusVerified || to == VerificationStatusRejected || to == VerificationStatusNeedsReview
	case VerificationStatusNeedsReview:
		return to == VerificationStatusVerified || to == VerificationStatusRejected
	case VerificationStatusVerified:
		return to == VerificationStatusRejected || to == VerificationStatusNeedsReview
	case VerificationStatusRejected:
		return to == VerificationStatusVerified || to == VerificationStatusNeedsReview
	default:
		return false
	}
}

func isValidVerificationStatus(s VerificationStatus) bool {
	switch s {
	case VerificationStatusUnverified, VerificationStatusNeedsReview, VerificationStatusVerified, VerificationStatusRejected:
		return true
	default:
		return false
	}
}

func (e *Evidence) UpdateMetadata(actorID uuid.UUID, metadata map[string]any, reason string) error {
	if actorID == uuid.Nil {
		return fmt.Errorf("%w: actor ID is required", ErrEvidenceInvalidInput)
	}
	if err := validateMetadata(metadata); err != nil {
		return fmt.Errorf("%w: %w", ErrEvidenceInvalidInput, err)
	}
	e.Metadata = copyMetadata(metadata)
	return nil
}

func (e *Evidence) SetPerson(personID uuid.UUID) error {
	if personID == uuid.Nil {
		return fmt.Errorf("%w: person ID is required", ErrEvidenceInvalidInput)
	}
	e.PersonID = &personID
	return nil
}
