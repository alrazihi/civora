package shared

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func ValidateRequired(s, field string) error {
	val := strings.TrimSpace(s)
	if val == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

func ValidateEmail(s string) error {
	if err := ValidateRequired(s, "email"); err != nil {
		return err
	}
	if !strings.Contains(s, "@") || !strings.Contains(s, ".") {
		return fmt.Errorf("email format is invalid")
	}
	return nil
}

func ValidateUsername(s string) error {
	if err := ValidateRequired(s, "username"); err != nil {
		return err
	}
	if len(s) > 64 {
		return fmt.Errorf("username must be 64 characters or fewer")
	}
	return nil
}

func ValidateUUID(s, field string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, fmt.Errorf("%s is required", field)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s is not a valid UUID", field)
	}
	return id, nil
}

func ValidatePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func ValidatePerPage(perPage int) int {
	if perPage < 1 {
		return 20
	}
	if perPage > 200 {
		return 200
	}
	return perPage
}
