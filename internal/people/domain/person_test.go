package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPerson(t *testing.T) {
	t.Run("valid person", func(t *testing.T) {
		p, err := NewPerson(uuid.New(), "Jane", "Doe", "en")
		require.NoError(t, err)
		assert.NotEmpty(t, p.ID)
		assert.Equal(t, "Jane", p.FirstName)
		assert.Equal(t, "Doe", p.LastName)
		assert.Equal(t, "en", p.PreferredLanguage)
		assert.Equal(t, PersonStatusActive, p.Status)
	})

	t.Run("empty first name", func(t *testing.T) {
		_, err := NewPerson(uuid.New(), "", "Doe", "en")
		assert.ErrorIs(t, err, ErrPersonInvalidInput)
	})

	t.Run("empty last name", func(t *testing.T) {
		_, err := NewPerson(uuid.New(), "Jane", "", "en")
		assert.ErrorIs(t, err, ErrPersonInvalidInput)
	})

	t.Run("empty preferred language", func(t *testing.T) {
		_, err := NewPerson(uuid.New(), "Jane", "Doe", "")
		assert.ErrorIs(t, err, ErrPersonInvalidInput)
	})

	t.Run("first name too long", func(t *testing.T) {
		longName := strings.Repeat("x", maxFirstNameLength+1)
		_, err := NewPerson(uuid.New(), longName, "Doe", "en")
		assert.ErrorIs(t, err, ErrPersonInvalidInput)
	})

	t.Run("last name too long", func(t *testing.T) {
		longName := strings.Repeat("x", maxLastNameLength+1)
		_, err := NewPerson(uuid.New(), "Jane", longName, "en")
		assert.ErrorIs(t, err, ErrPersonInvalidInput)
	})
}

func TestPersonFullName(t *testing.T) {
	p, err := NewPerson(uuid.New(), "Jane", "Doe", "en")
	require.NoError(t, err)
	assert.Equal(t, "Jane Doe", p.FullName())
}

func TestPersonUpdateStatus(t *testing.T) {
	p, err := NewPerson(uuid.New(), "Jane", "Doe", "en")
	require.NoError(t, err)
	assert.Equal(t, PersonStatusActive, p.Status)

	p.UpdateStatus(PersonStatusInactive)
	assert.Equal(t, PersonStatusInactive, p.Status)
}

func TestPersonWithOptionalFields(t *testing.T) {
	p, err := NewPerson(uuid.New(), "John", "Smith", "en")
	require.NoError(t, err)

	extRef := "EXT-001"
	email := "john@example.com"
	phone := "+1234567890"
	address := "123 Main St"
	city := "Anytown"
	dob := time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC)

	p.ExternalReference = &extRef
	p.Email = &email
	p.Phone = &phone
	p.Address = &address
	p.City = &city
	p.DateOfBirth = &dob

	require.NotNil(t, p.ExternalReference)
	assert.Equal(t, "EXT-001", *p.ExternalReference)
	assert.Equal(t, "john@example.com", *p.Email)
	assert.Equal(t, "+1234567890", *p.Phone)
	assert.Equal(t, "123 Main St", *p.Address)
	assert.Equal(t, "Anytown", *p.City)
	assert.Equal(t, dob, *p.DateOfBirth)
}
