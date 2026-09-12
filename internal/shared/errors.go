package shared

import "fmt"

type ErrorCode string

const (
	CodeInvalidInput            ErrorCode = "INVALID_INPUT"
	CodeUnauthorized            ErrorCode = "UNAUTHORIZED"
	CodeForbidden               ErrorCode = "FORBIDDEN"
	CodeNotFound                ErrorCode = "NOT_FOUND"
	CodeConflict                ErrorCode = "CONFLICT"
	CodeInternalError           ErrorCode = "INTERNAL_ERROR"
	CodeBadRequest              ErrorCode = "BAD_REQUEST"
	CodeStateTransition         ErrorCode = "STATE_TRANSITION"
	CodeTenantViolation         ErrorCode = "TENANT_VIOLATION"
	CodeRequiredFormsIncomplete ErrorCode = "REQUIRED_FORMS_INCOMPLETE"
)

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Err     error     `json:"-"`
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func NewError(code ErrorCode, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     cause,
	}
}

func Errorf(code ErrorCode, format string, args ...any) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}
