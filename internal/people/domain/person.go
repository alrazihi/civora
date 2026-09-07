package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PersonStatus string

const (
	PersonStatusActive   PersonStatus = "ACTIVE"
	PersonStatusInactive PersonStatus = "INACTIVE"
)

var (
	ErrPersonNotFound     = errors.New("person not found")
	ErrPersonInvalidInput = errors.New("invalid person input")
)

type Person struct {
	ID                uuid.UUID    `json:"id"`
	OrganizationID    uuid.UUID    `json:"organization_id"`
	ExternalReference *string      `json:"external_reference"`
	FirstName         string       `json:"first_name"`
	LastName          string       `json:"last_name"`
	DateOfBirth       *time.Time   `json:"date_of_birth"`
	Email             *string      `json:"email"`
	Phone             *string      `json:"phone"`
	Address           *string      `json:"address"`
	City              *string      `json:"city"`
	PreferredLanguage string       `json:"preferred_language"`
	Status            PersonStatus `json:"status"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

const (
	maxFirstNameLength = 100
	maxLastNameLength  = 100
	maxAddressLength   = 500
	maxCityLength      = 100
	maxLanguageLength  = 10
)

func ValidatePersonInput(firstName, lastName, preferredLanguage string) error {
	if firstName == "" {
		return fmt.Errorf("%w: first name is required", ErrPersonInvalidInput)
	}
	if len(firstName) > maxFirstNameLength {
		return fmt.Errorf("%w: first name exceeds maximum length of %d characters", ErrPersonInvalidInput, maxFirstNameLength)
	}
	if lastName == "" {
		return fmt.Errorf("%w: last name is required", ErrPersonInvalidInput)
	}
	if len(lastName) > maxLastNameLength {
		return fmt.Errorf("%w: last name exceeds maximum length of %d characters", ErrPersonInvalidInput, maxLastNameLength)
	}
	if preferredLanguage == "" {
		return fmt.Errorf("%w: preferred language is required", ErrPersonInvalidInput)
	}
	if len(preferredLanguage) > maxLanguageLength {
		return fmt.Errorf("%w: preferred language exceeds maximum length of %d characters", ErrPersonInvalidInput, maxLanguageLength)
	}
	return nil
}

func NewPerson(orgID uuid.UUID, firstName, lastName, preferredLanguage string) (*Person, error) {
	if err := ValidatePersonInput(firstName, lastName, preferredLanguage); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Person{
		ID:                uuid.New(),
		OrganizationID:    orgID,
		FirstName:         firstName,
		LastName:          lastName,
		PreferredLanguage: preferredLanguage,
		Status:            PersonStatusActive,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func (p *Person) FullName() string {
	return p.FirstName + " " + p.LastName
}

func (p *Person) UpdateStatus(status PersonStatus) {
	p.Status = status
	p.UpdatedAt = time.Now().UTC()
}
