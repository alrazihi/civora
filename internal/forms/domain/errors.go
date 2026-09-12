package domain

import "errors"

var (
	ErrFormNotFound         = errors.New("form not found")
	ErrFormInvalidInput     = errors.New("invalid form input")
	ErrFormKeyExists        = errors.New("form key already exists")
	ErrFormVersionNotFound  = errors.New("form version not found")
	ErrFormFieldNotFound    = errors.New("form field not found")
	ErrFormInvalidStatus    = errors.New("invalid form status transition")
	ErrFormVersionStatus    = errors.New("invalid form version status")
	ErrFormFieldTypeInvalid = errors.New("invalid field type")
	ErrFormFieldDuplicate   = errors.New("duplicate field key")
	ErrFormFieldInvalid     = errors.New("invalid field input")
	ErrFormOptionInvalid    = errors.New("invalid option")
)
