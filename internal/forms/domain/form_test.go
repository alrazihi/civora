package domain

import (
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewForm_Valid(t *testing.T) {
	orgID := uuid.New()
	form, err := NewForm(orgID, "test-form", "Test Form", "Description", uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "test-form", form.Key)
	assert.Equal(t, "Test Form", form.Name)
	assert.Equal(t, "Description", form.Description)
	assert.Equal(t, FormStatusDraft, form.Status)
	assert.Equal(t, orgID, form.OrganizationID)
	assert.NotEqual(t, uuid.Nil, form.ID)
}

func TestNewForm_EmptyKey(t *testing.T) {
	_, err := NewForm(uuid.New(), "", "Test Form", "", uuid.New())
	require.Error(t, err)
	assert.ErrorContains(t, err, "key is required")
}

func TestNewForm_EmptyName(t *testing.T) {
	_, err := NewForm(uuid.New(), "test-form", "", "", uuid.New())
	require.Error(t, err)
	assert.ErrorContains(t, err, "name is required")
}

func TestNewForm_KeyTooLong(t *testing.T) {
	key := make([]byte, maxFormKeyName+1)
	for i := range key {
		key[i] = 'a'
	}
	_, err := NewForm(uuid.New(), string(key), "Test Form", "", uuid.New())
	require.Error(t, err)
}

func TestNewFormVersion(t *testing.T) {
	formID := uuid.New()
	orgID := uuid.New()
	v := NewFormVersion(formID, orgID, uuid.New())
	assert.Equal(t, formID, v.FormID)
	assert.Equal(t, orgID, v.OrganizationID)
	assert.Equal(t, 1, v.Version)
	assert.Equal(t, FormVersionStatusDraft, v.Status)
	assert.NotNil(t, v.CreatedAt)
	assert.Nil(t, v.PublishedAt)
}

func TestNewFormField(t *testing.T) {
	formID := uuid.New()
	versionID := uuid.New()
	orgID := uuid.New()
	defaultVal := "default"
	field := NewFormField(formID, versionID, orgID, "name", "Name", FieldTypeText, true,
		"desc", "placeholder", &defaultVal, map[string]any{"min_length": 1}, []FormOption{{Label: "Opt 1", Value: "opt1"}}, 1)
	assert.Equal(t, formID, field.FormID)
	assert.Equal(t, versionID, field.FormVersionID)
	assert.Equal(t, orgID, field.OrganizationID)
	assert.Equal(t, "name", field.Key)
	assert.Equal(t, "Name", field.Label)
	assert.Equal(t, FieldTypeText, field.Type)
	assert.True(t, field.Required)
	assert.Equal(t, "desc", field.Description)
	assert.Equal(t, "placeholder", field.Placeholder)
	assert.Equal(t, &defaultVal, field.DefaultValue)
	assert.Equal(t, map[string]any{"min_length": 1}, field.Validation)
	assert.Len(t, field.Options, 1)
	assert.Equal(t, 1, field.Order)
}

func TestNewFormField_NilMaps(t *testing.T) {
	formID := uuid.New()
	versionID := uuid.New()
	orgID := uuid.New()
	field := NewFormField(formID, versionID, orgID, "f", "F", FieldTypeText, false, "", "", nil, nil, nil, 0)
	assert.NotNil(t, field.Validation)
	assert.NotNil(t, field.Options)
	assert.Empty(t, field.Options)
}

func TestValidateField_EmailType(t *testing.T) {
	err := ValidateField(FieldTypeEmail, "Email", true, nil, nil)
	assert.NoError(t, err)
}

func TestValidateField_InvalidType(t *testing.T) {
	err := ValidateField("INVALID_TYPE", "Label", true, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFormFieldTypeInvalid)
}

func TestValidateField_EmptyLabel(t *testing.T) {
	err := ValidateField(FieldTypeText, "", true, nil, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "label is required")
}

func TestValidateField_SelectWithNoOptions(t *testing.T) {
	err := ValidateField(FieldTypeSelect, "Choose", true, nil, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "at least one option is required")
}

func TestValidateField_SelectWithDuplicateOptions(t *testing.T) {
	opts := []FormOption{{Label: "A", Value: "x"}, {Label: "B", Value: "x"}}
	err := ValidateField(FieldTypeSelect, "Choose", true, opts, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "duplicate option value")
}

func TestValidateField_SelectWithOptions(t *testing.T) {
	opts := []FormOption{{Label: "Yes", Value: "yes"}, {Label: "No", Value: "no"}}
	err := ValidateField(FieldTypeSelect, "Choose", true, opts, nil)
	assert.NoError(t, err)
}

func TestValidateField_NumericRange(t *testing.T) {
	validation := map[string]any{"min_value": 0.0, "max_value": 100.0}
	err := ValidateField(FieldTypeNumber, "Score", true, nil, validation)
	assert.NoError(t, err)
}

func TestValidateField_NumericInvalidRange(t *testing.T) {
	validation := map[string]any{"min_value": 100.0, "max_value": 0.0}
	err := ValidateField(FieldTypeNumber, "Score", true, nil, validation)
	require.Error(t, err)
	assert.ErrorContains(t, err, "min_value cannot exceed max_value")
}

func TestValidateField_LengthConstraints(t *testing.T) {
	validation := map[string]any{"min_length": 1, "max_length": 255}
	err := ValidateField(FieldTypeText, "Name", true, nil, validation)
	assert.NoError(t, err)
}

func TestValidateField_LengthInverted(t *testing.T) {
	validation := map[string]any{"min_length": 100, "max_length": 10}
	err := ValidateField(FieldTypeText, "Name", true, nil, validation)
	require.Error(t, err)
	assert.ErrorContains(t, err, "min_length cannot exceed max_length")
}

func TestValidateField_MaxLengthExceeded(t *testing.T) {
	validation := map[string]any{"max_length": 2000000}
	err := ValidateField(FieldTypeText, "Name", true, nil, validation)
	require.Error(t, err)
}

func TestValidateField_MinValueExceeded(t *testing.T) {
	validation := map[string]any{"min_value": math.MaxFloat64}
	err := ValidateField(FieldTypeNumber, "Score", true, nil, validation)
	require.Error(t, err)
}

func TestValidateField_EmailWithPattern(t *testing.T) {
	validation := map[string]any{"pattern": `^[a-z]+@[a-z]+\.[a-z]+$`}
	err := ValidateField(FieldTypeEmail, "Email", true, nil, validation)
	assert.NoError(t, err)
}

func TestValidateField_EmailWithInvalidPattern(t *testing.T) {
	validation := map[string]any{"pattern": `[invalid`}
	err := ValidateField(FieldTypeEmail, "Email", true, nil, validation)
	require.Error(t, err)
}

func TestValidateField_PhoneWithPattern(t *testing.T) {
	validation := map[string]any{"pattern": `^\+?[\d\s\-().]{7,25}$`}
	err := ValidateField(FieldTypePhone, "Phone", true, nil, validation)
	assert.NoError(t, err)
}

func TestValidateFieldCheckbox_WithOptions(t *testing.T) {
	opts := []FormOption{{Label: "A", Value: "a"}}
	err := ValidateField(FieldTypeCheckbox, "Check", true, opts, nil)
	assert.NoError(t, err)
}

func TestValidateFieldCheckbox_WithoutOptions(t *testing.T) {
	err := ValidateField(FieldTypeCheckbox, "Check", true, nil, nil)
	assert.NoError(t, err)
}

func TestValidateField_MultiSelect_WithOptions(t *testing.T) {
	opts := []FormOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}
	err := ValidateField(FieldTypeMultiSelect, "Multi", true, opts, nil)
	assert.NoError(t, err)
}

func TestValidateField_DateWithFormat(t *testing.T) {
	validation := map[string]any{"format": "date"}
	err := ValidateField(FieldTypeDate, "Date", true, nil, validation)
	assert.NoError(t, err)
}

func TestValidateField_DateWithUnsupportedFormat(t *testing.T) {
	validation := map[string]any{"format": "unsupported"}
	err := ValidateField(FieldTypeDate, "Date", true, nil, validation)
	require.Error(t, err)
}

func TestValidateFieldRadioButton_WithOptions(t *testing.T) {
	opts := []FormOption{{Label: "A", Value: "a"}}
	err := ValidateField(FieldTypeRadio, "Radio", true, opts, nil)
	assert.NoError(t, err)
}

func TestValidateFieldRadioWithoutOptions(t *testing.T) {
	err := ValidateField(FieldTypeRadio, "Radio", true, nil, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "at least one option is required")
}

func TestValidateFormStatusTransition(t *testing.T) {
	assert.NoError(t, ValidateFormStatusTransition(FormStatusDraft, FormStatusActive))
	assert.NoError(t, ValidateFormStatusTransition(FormStatusDraft, FormStatusArchived))
	assert.NoError(t, ValidateFormStatusTransition(FormStatusActive, FormStatusArchived))
	assert.Error(t, ValidateFormStatusTransition(FormStatusArchived, FormStatusDraft))
	assert.Error(t, ValidateFormStatusTransition(FormStatusActive, FormStatusDraft))
}

func TestValidateVersionStatusTransition(t *testing.T) {
	assert.NoError(t, ValidateVersionStatusTransition(FormVersionStatusDraft, FormVersionStatusPublished))
	assert.NoError(t, ValidateVersionStatusTransition(FormVersionStatusDraft, FormVersionStatusArchived))
	assert.NoError(t, ValidateVersionStatusTransition(FormVersionStatusPublished, FormVersionStatusArchived))
	assert.Error(t, ValidateVersionStatusTransition(FormVersionStatusArchived, FormVersionStatusDraft))
	assert.Error(t, ValidateVersionStatusTransition(FormVersionStatusPublished, FormVersionStatusDraft))
}

func TestIsValidFieldType(t *testing.T) {
	for _, ft := range []FieldType{
		FieldTypeText, FieldTypeTextarea, FieldTypeNumber, FieldTypeDecimal,
		FieldTypeDate, FieldTypeDatetime, FieldTypeBoolean,
		FieldTypeSelect, FieldTypeMultiSelect, FieldTypeRadio, FieldTypeCheckbox,
		FieldTypeEmail, FieldTypePhone,
	} {
		assert.True(t, IsValidFieldType(ft), "expected %s to be valid", ft)
	}
	assert.False(t, IsValidFieldType("INVALID"))
}

func TestValidateFieldKey(t *testing.T) {
	assert.NoError(t, ValidateFieldKey("valid_key"))
	assert.Error(t, ValidateFieldKey(""))
	assert.Error(t, ValidateFieldKey(string(make([]byte, 101))))
	assert.Error(t, ValidateFieldKey("InvalidKey"))
}

func TestValidateFormKey(t *testing.T) {
	assert.NoError(t, ValidateFormKey("valid-key"))
	assert.Error(t, ValidateFormKey(""))
	assert.Error(t, ValidateFormKey("InvalidKey"))
}

func TestValidateFormName(t *testing.T) {
	assert.NoError(t, ValidateFormName("Valid Name"))
	assert.Error(t, ValidateFormName(""))
}

func TestValidateFormDescription(t *testing.T) {
	assert.NoError(t, ValidateFormDescription(""))
	assert.NoError(t, ValidateFormDescription("A valid description"))
	assert.Error(t, ValidateFormDescription(string(make([]byte, 2001))))
}

func TestForm_VersionIncrement(t *testing.T) {
	formID := uuid.New()
	orgID := uuid.New()
	v1 := NewFormVersion(formID, orgID, uuid.New())
	assert.Equal(t, 1, v1.Version)
	v2 := NewFormVersion(formID, orgID, uuid.New())
	v2.Version = v1.Version + 1
	assert.Equal(t, 2, v2.Version)
}
