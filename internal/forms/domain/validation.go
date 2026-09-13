package domain

import (
	"fmt"
	"regexp"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
var phoneRegex = regexp.MustCompile(`^[+\d\s\-().]{7,25}$`)

func ValidateFieldValidation(fieldType FieldType, validation map[string]any) error {
	if validation == nil {
		return nil
	}

	switch fieldType {
	case FieldTypeText, FieldTypeTextarea:
		if err := validateLengthFieldValidation(validation); err != nil {
			return err
		}
	case FieldTypeNumber, FieldTypeDecimal:
		if err := validateNumericFieldValidation(validation); err != nil {
			return err
		}
	case FieldTypeDate, FieldTypeDatetime:
		if err := validateDateFormatValidation(validation); err != nil {
			return err
		}
	case FieldTypeEmail:
		if err := validateEmailFieldValidation(validation); err != nil {
			return err
		}
	case FieldTypePhone:
		if err := validatePhoneFieldValidation(validation); err != nil {
			return err
		}
	case FieldTypeSelect, FieldTypeMultiSelect, FieldTypeRadio, FieldTypeCheckbox:
		return nil
	case FieldTypeBoolean:
		return nil
	default:
		return fmt.Errorf("%w: unknown field type: %s", ErrFormFieldTypeInvalid, fieldType)
	}

	return validateCommonFieldValidation(validation)
}

func validateCommonFieldValidation(v map[string]any) error {
	for _, key := range []string{"min_length", "max_length", "minLength", "maxLength"} {
		if val, ok := v[key]; ok {
			if _, ok := getFloat(v, key); !ok {
				return fmt.Errorf("%w: %s must be a number, got %T", ErrFormFieldInvalid, key, val)
			}
		}
	}
	for _, key := range []string{"min_value", "max_value", "minValue", "maxValue"} {
		if val, ok := v[key]; ok {
			if _, ok := getFloat(v, key); !ok {
				return fmt.Errorf("%w: %s must be a number, got %T", ErrFormFieldInvalid, key, val)
			}
		}
	}
	return nil
}

func getFloat(v map[string]any, key string) (float64, bool) {
	if val, ok := v[key]; ok {
		switch f := val.(type) {
		case float64:
			return f, true
		case int:
			return float64(f), true
		case int64:
			return float64(f), true
		default:
			return 0, false
		}
	}
	return 0, false
}

func validateLengthFieldValidation(v map[string]any) error {
	for _, minKey := range []string{"min_length", "minLength"} {
		if val, hasMin := v[minKey]; hasMin {
			f, ok := getFloat(v, minKey)
			if !ok {
				return fmt.Errorf("%s must be a number, got %T", minKey, val)
			}
			if f < 0 || f > 1000000 {
				return fmt.Errorf("%s must be between 0 and 1000000", minKey)
			}
		}
	}
	for _, maxKey := range []string{"max_length", "maxLength"} {
		if val, hasMax := v[maxKey]; hasMax {
			f, ok := getFloat(v, maxKey)
			if !ok {
				return fmt.Errorf("%s must be a number, got %T", maxKey, val)
			}
			if f < 0 || f > 1000000 {
				return fmt.Errorf("%s must be between 0 and 1000000", maxKey)
			}
		}
	}
	minVal, hasMin := getFloat(v, "min_length")
	if !hasMin {
		minVal, hasMin = getFloat(v, "minLength")
	}
	maxVal, hasMax := getFloat(v, "max_length")
	if !hasMax {
		maxVal, hasMax = getFloat(v, "maxLength")
	}
	if hasMin && hasMax && minVal > maxVal {
		return fmt.Errorf("min_length cannot exceed max_length")
	}
	return nil
}

func validateNumericFieldValidation(v map[string]any) error {
	minVal, hasMin := getFloat(v, "min_value")
	if !hasMin {
		minVal, hasMin = getFloat(v, "minValue")
	}
	maxVal, hasMax := getFloat(v, "max_value")
	if !hasMax {
		maxVal, hasMax = getFloat(v, "maxValue")
	}
	if hasMin && !isFinite(minVal) {
		return fmt.Errorf("min_value must be a finite number")
	}
	if hasMax && !isFinite(maxVal) {
		return fmt.Errorf("max_value must be a finite number")
	}
	if hasMin && hasMax && minVal > maxVal {
		return fmt.Errorf("min_value cannot exceed max_value")
	}
	return nil
}

func validateDateFormatValidation(v map[string]any) error {
	if format, ok := v["format"].(string); ok {
		switch format {
		case "date", "datetime", "iso8601":
			return nil
		default:
			return fmt.Errorf("unsupported format: %s", format)
		}
	}
	return nil
}

func validateEmailFieldValidation(v map[string]any) error {
	if pattern, ok := v["pattern"].(string); ok {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid pattern: %v", err)
		}
	}
	return nil
}

func validatePhoneFieldValidation(v map[string]any) error {
	if pattern, ok := v["pattern"].(string); ok {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid pattern: %v", err)
		}
	}
	return nil
}

func ValidateFieldOptions(fieldType FieldType, options []FormOption) error {
	if fieldType != FieldTypeSelect && fieldType != FieldTypeMultiSelect &&
		fieldType != FieldTypeRadio && fieldType != FieldTypeCheckbox {
		if len(options) > 0 {
			return fmt.Errorf("%w: options only allowed for SELECT, MULTISELECT, RADIO, CHECKBOX", ErrFormOptionInvalid)
		}
		return nil
	}

	if len(options) == 0 {
		return fmt.Errorf("%w: at least one option is required", ErrFormOptionInvalid)
	}
	if len(options) > maxOptionsCount {
		return fmt.Errorf("%w: maximum %d options allowed", ErrFormOptionInvalid, maxOptionsCount)
	}

	valueSet := make(map[string]bool)
	for i, opt := range options {
		if opt.Label == "" {
			return fmt.Errorf("%w: option at index %d has empty label", ErrFormOptionInvalid, i)
		}
		if opt.Value == "" {
			return fmt.Errorf("%w: option at index %d has empty value", ErrFormOptionInvalid, i)
		}
		if len(opt.Label) > 200 {
			return fmt.Errorf("%w: option label at index %d exceeds maximum length", ErrFormOptionInvalid, i)
		}
		if valueSet[opt.Value] {
			return fmt.Errorf("%w: duplicate option value %q", ErrFormOptionInvalid, opt.Value)
		}
		valueSet[opt.Value] = true
	}
	return nil
}

func ValidateField(fieldType FieldType, label string, required bool, options []FormOption, validation map[string]any) error {
	if err := ValidateFieldLabel(label); err != nil {
		return err
	}
	if err := ValidateFieldType(fieldType); err != nil {
		return err
	}
	if err := ValidateFieldValidation(fieldType, validation); err != nil {
		return err
	}
	if fieldType == FieldTypeSelect || fieldType == FieldTypeMultiSelect || fieldType == FieldTypeRadio {
		if err := ValidateFieldOptions(fieldType, options); err != nil {
			return err
		}
	}
	return nil
}

func isFinite(f float64) bool {
	return f == f && f > -1e308 && f < 1e308
}
