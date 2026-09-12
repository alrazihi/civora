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
		return validateLengthFieldValidation(validation)
	case FieldTypeNumber, FieldTypeDecimal:
		return validateNumericFieldValidation(validation)
	case FieldTypeDate, FieldTypeDatetime:
		return validateDateFormatValidation(validation)
	case FieldTypeEmail:
		return validateEmailFieldValidation(validation)
	case FieldTypePhone:
		return validatePhoneFieldValidation(validation)
	case FieldTypeSelect, FieldTypeMultiSelect, FieldTypeRadio, FieldTypeCheckbox:
		return nil
	case FieldTypeBoolean:
		return nil
	default:
		return fmt.Errorf("unknown field type: %s", fieldType)
	}
}

func getFloat(v map[string]any, key string) (float64, bool) {
	if val, ok := v[key]; ok {
		switch f := val.(type) {
		case float64:
			return f, true
		case int:
			return float64(f), true
		}
	}
	return 0, false
}

func validateLengthFieldValidation(v map[string]any) error {
	if min, ok := getFloat(v, "min_length"); ok {
		if min < 0 || min > 1000000 {
			return fmt.Errorf("min_length must be between 0 and 1000000")
		}
	}
	if max, ok := getFloat(v, "max_length"); ok {
		if max < 0 || max > 1000000 {
			return fmt.Errorf("max_length must be between 0 and 1000000")
		}
	}
	if min, ok := getFloat(v, "min_length"); ok {
		if max, ok := getFloat(v, "max_length"); ok {
			if min > max {
				return fmt.Errorf("min_length cannot exceed max_length")
			}
		}
	}
	return nil
}

func validateNumericFieldValidation(v map[string]any) error {
	if min, ok := getFloat(v, "min_value"); ok {
		if !isFinite(min) {
			return fmt.Errorf("min_value must be a finite number")
		}
	}
	if max, ok := getFloat(v, "max_value"); ok {
		if !isFinite(max) {
			return fmt.Errorf("max_value must be a finite number")
		}
	}
	if min, ok := getFloat(v, "min_value"); ok {
		if max, ok := getFloat(v, "max_value"); ok {
			if min > max {
				return fmt.Errorf("min_value cannot exceed max_value")
			}
		}
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
