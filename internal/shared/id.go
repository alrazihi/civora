package shared

import "github.com/google/uuid"

func NewID() uuid.UUID {
	return uuid.New()
}

func ParseID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

func IsValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
