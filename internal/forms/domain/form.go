package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

type FormStatus string

const (
	FormStatusDraft    FormStatus = "DRAFT"
	FormStatusActive   FormStatus = "ACTIVE"
	FormStatusArchived FormStatus = "ARCHIVED"
)

type FormVersionStatus string

const (
	FormVersionStatusDraft     FormVersionStatus = "DRAFT"
	FormVersionStatusPublished FormVersionStatus = "PUBLISHED"
	FormVersionStatusArchived  FormVersionStatus = "ARCHIVED"
)

type Form struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	Key            string         `json:"key"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Status         FormStatus     `json:"status"`
	CreatedByID    uuid.UUID      `json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Versions       []*FormVersion `json:"versions,omitempty"`
}

type FormVersion struct {
	ID             uuid.UUID         `json:"id"`
	FormID         uuid.UUID         `json:"form_id"`
	OrganizationID uuid.UUID         `json:"organization_id"`
	Version        int               `json:"version"`
	Status         FormVersionStatus `json:"status"`
	CreatedByID    uuid.UUID         `json:"created_by"`
	CreatedAt      time.Time         `json:"created_at"`
	PublishedAt    *time.Time        `json:"published_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Fields         []*FormField      `json:"fields,omitempty"`
}

type FormField struct {
	ID             uuid.UUID      `json:"id"`
	FormID         uuid.UUID      `json:"form_id"`
	FormVersionID  uuid.UUID      `json:"form_version_id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	Key            string         `json:"key"`
	Label          string         `json:"label"`
	Type           FieldType      `json:"type"`
	Required       bool           `json:"required"`
	Description    string         `json:"description"`
	Placeholder    string         `json:"placeholder"`
	DefaultValue   *string        `json:"default_value"`
	Validation     map[string]any `json:"validation"`
	Options        []FormOption   `json:"options"`
	Order          int            `json:"order"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type FormOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type FieldType string

const (
	FieldTypeText        FieldType = "TEXT"
	FieldTypeTextarea    FieldType = "TEXTAREA"
	FieldTypeNumber      FieldType = "NUMBER"
	FieldTypeDecimal     FieldType = "DECIMAL"
	FieldTypeDate        FieldType = "DATE"
	FieldTypeDatetime    FieldType = "DATETIME"
	FieldTypeBoolean     FieldType = "BOOLEAN"
	FieldTypeSelect      FieldType = "SELECT"
	FieldTypeMultiSelect FieldType = "MULTISELECT"
	FieldTypeRadio       FieldType = "RADIO"
	FieldTypeCheckbox    FieldType = "CHECKBOX"
	FieldTypeEmail       FieldType = "EMAIL"
	FieldTypePhone       FieldType = "PHONE"
)

var validFieldTypes = map[FieldType]bool{
	FieldTypeText:        true,
	FieldTypeTextarea:    true,
	FieldTypeNumber:      true,
	FieldTypeDecimal:     true,
	FieldTypeDate:        true,
	FieldTypeDatetime:    true,
	FieldTypeBoolean:     true,
	FieldTypeSelect:      true,
	FieldTypeMultiSelect: true,
	FieldTypeRadio:       true,
	FieldTypeCheckbox:    true,
	FieldTypeEmail:       true,
	FieldTypePhone:       true,
}

func IsValidFieldType(t FieldType) bool {
	return validFieldTypes[t]
}

var (
	maxFormKeyName     = 100
	maxFormName        = 200
	maxFormDescription = 2000
	maxFieldKey        = 100
	maxFieldLabel      = 200
	maxDescription     = 2000
	maxPlaceholder     = 500
	maxOptionsCount    = 100
)

func NewForm(orgID uuid.UUID, key, name, description string, createdByID uuid.UUID) (*Form, error) {
	if err := ValidateFormKey(key); err != nil {
		return nil, err
	}
	if err := ValidateFormName(name); err != nil {
		return nil, err
	}
	if err := ValidateFormDescription(description); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Form{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Key:            key,
		Name:           name,
		Description:    description,
		Status:         FormStatusDraft,
		CreatedByID:    createdByID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func NewFormVersion(formID, orgID uuid.UUID, createdByID uuid.UUID) *FormVersion {
	now := time.Now().UTC()
	return &FormVersion{
		ID:             uuid.New(),
		FormID:         formID,
		OrganizationID: orgID,
		Version:        1,
		Status:         FormVersionStatusDraft,
		CreatedByID:    createdByID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func NewFormField(formID, versionID, orgID uuid.UUID, key, label string, fieldType FieldType, required bool, description, placeholder string, defaultValue *string, validation map[string]any, options []FormOption, order int) *FormField {
	now := time.Now().UTC()
	if validation == nil {
		validation = map[string]any{}
	}
	if options == nil {
		options = []FormOption{}
	}
	return &FormField{
		ID:             uuid.New(),
		FormID:         formID,
		FormVersionID:  versionID,
		OrganizationID: orgID,
		Key:            key,
		Label:          label,
		Type:           fieldType,
		Required:       required,
		Description:    description,
		Placeholder:    placeholder,
		DefaultValue:   defaultValue,
		Validation:     validation,
		Options:        options,
		Order:          order,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func ValidateFormKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: key is required", ErrFormInvalidInput)
	}
	if len(key) > maxFormKeyName {
		return fmt.Errorf("%w: key exceeds maximum length of %d characters", ErrFormInvalidInput, maxFormKeyName)
	}
	if !strings.EqualFold(key, strings.ToUpper(key)) {
		// allow mixed but strip control chars
	}
	if strings.ContainsFunc(key, unicode.IsControl) {
		return fmt.Errorf("%w: key must not contain control characters", ErrFormInvalidInput)
	}
	return nil
}

func ValidateFormName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is required", ErrFormInvalidInput)
	}
	if len(name) > maxFormName {
		return fmt.Errorf("%w: name exceeds maximum length of %d characters", ErrFormInvalidInput, maxFormName)
	}
	return nil
}

func ValidateFormDescription(desc string) error {
	if desc != "" && len(desc) > maxFormDescription {
		return fmt.Errorf("%w: description exceeds maximum length of %d characters", ErrFormInvalidInput, maxFormDescription)
	}
	return nil
}

func ValidateFieldKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: key is required", ErrFormFieldInvalid)
	}
	if len(key) > maxFieldKey {
		return fmt.Errorf("%w: key exceeds maximum length of %d characters", ErrFormFieldInvalid, maxFieldKey)
	}
	if strings.ContainsFunc(key, unicode.IsControl) {
		return fmt.Errorf("%w: key must not contain control characters", ErrFormFieldInvalid)
	}
	return nil
}

func ValidateFieldLabel(label string) error {
	if label == "" {
		return fmt.Errorf("%w: label is required", ErrFormFieldInvalid)
	}
	if len(label) > maxFieldLabel {
		return fmt.Errorf("%w: label exceeds maximum length of %d characters", ErrFormFieldInvalid, maxFieldLabel)
	}
	return nil
}

func ValidateFieldType(t FieldType) error {
	if !IsValidFieldType(t) {
		return fmt.Errorf("%w: %s", ErrFormFieldTypeInvalid, t)
	}
	return nil
}

func ValidateFormStatusTransition(from, to FormStatus) error {
	switch from {
	case FormStatusDraft:
		if to == FormStatusActive || to == FormStatusArchived {
			return nil
		}
	case FormStatusActive:
		if to == FormStatusArchived {
			return nil
		}
	case FormStatusArchived:
		// no transitions from archived
	}
	return fmt.Errorf("%w: cannot transition from %s to %s", ErrFormInvalidStatus, from, to)
}

func ValidateVersionStatusTransition(from, to FormVersionStatus) error {
	switch from {
	case FormVersionStatusDraft:
		if to == FormVersionStatusPublished || to == FormVersionStatusArchived {
			return nil
		}
	case FormVersionStatusPublished:
		if to == FormVersionStatusArchived {
			return nil
		}
	case FormVersionStatusArchived:
		// no transitions from archived
	}
	return fmt.Errorf("%w: cannot transition from %s to %s", ErrFormVersionStatus, from, to)
}
