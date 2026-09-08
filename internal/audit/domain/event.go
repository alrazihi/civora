package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrMetadataUnmarshal is returned when audit metadata cannot be serialized
// for hashing. It is a hard error: a broken hash chain is a data-integrity
// failure and must never be silently ignored.
var ErrMetadataUnmarshal = errors.New("audit metadata is not JSON-serializable")

type AuditEvent struct {
	ID             uuid.UUID              `json:"id"`
	OrganizationID uuid.UUID              `json:"organization_id"`
	ActorID        *uuid.UUID             `json:"actor_id"`
	Action         string                 `json:"action"`
	Resource       string                 `json:"resource"`
	ResourceID     *string                `json:"resource_id"`
	Outcome        string                 `json:"outcome"`
	RequestID      *string                `json:"request_id"`
	Metadata       map[string]interface{} `json:"metadata"`
	Timestamp      time.Time              `json:"timestamp"`
	PreviousHash   *string                `json:"previous_hash"`
	Hash           string                 `json:"hash"`
}

func NewAuditEvent(
	orgID uuid.UUID,
	actorID uuid.UUID,
	action, resource string,
	resourceID *string,
	outcome string,
	requestID *string,
	metadata map[string]interface{},
	previousHash *string,
) (*AuditEvent, error) {
	ev := &AuditEvent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		ActorID:        &actorID,
		Action:         action,
		Resource:       resource,
		ResourceID:     resourceID,
		Outcome:        outcome,
		RequestID:      requestID,
		Metadata:       metadata,
		Timestamp:      time.Now().UTC(),
		PreviousHash:   previousHash,
	}
	hash, err := ev.ComputeHash()
	if err != nil {
		return nil, err
	}
	ev.Hash = hash
	return ev, nil
}

func (e *AuditEvent) ComputeHash() (string, error) {
	h := sha256.New()

	h.Write([]byte(e.OrganizationID.String()))
	h.Write([]byte("|"))

	if e.ActorID != nil {
		h.Write([]byte(e.ActorID.String()))
	}
	h.Write([]byte("|"))

	h.Write([]byte(e.Action))
	h.Write([]byte("|"))
	h.Write([]byte(e.Resource))
	h.Write([]byte("|"))

	if e.ResourceID != nil {
		h.Write([]byte(*e.ResourceID))
	}
	h.Write([]byte("|"))

	h.Write([]byte(e.Outcome))
	h.Write([]byte("|"))

	if e.RequestID != nil {
		h.Write([]byte(*e.RequestID))
	}
	h.Write([]byte("|"))

	metaBytes, err := json.Marshal(e.Metadata)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrMetadataUnmarshal, err)
	}
	h.Write(metaBytes)
	h.Write([]byte("|"))

	h.Write([]byte(e.Timestamp.UTC().Format("2006-01-02T15:04:05.000000Z07:00")))
	h.Write([]byte("|"))

	if e.PreviousHash != nil {
		h.Write([]byte(*e.PreviousHash))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (e *AuditEvent) VerifyIntegrity() bool {
	recomputed, err := e.ComputeHash()
	if err != nil {
		return false
	}
	return e.Hash == recomputed
}

// VerifyChain walks the audit events for a single organization in
// timestamp/id order and checks that each event's hash matches its content
// and that each event links to its predecessor. It returns the number of
// events verified, the number of events that failed verification, and any
// error encountered while reading events from the repository.
//
// This is a read-only operation: it does not modify any audit data.
func VerifyChain(events []*AuditEvent) (verified, failed int) {
	var prevHash *string
	for _, ev := range events {
		if ev == nil {
			failed++
			continue
		}
		if !ev.VerifyIntegrity() {
			failed++
			prevHash = nil
			continue
		}
		if ev.PreviousHash != nil && prevHash != nil && *ev.PreviousHash != *prevHash {
			failed++
			prevHash = nil
			continue
		}
		verified++
		prevHash = &ev.Hash
	}
	return verified, failed
}

func IsValidOutcome(s string) bool {
	switch s {
	case "success", "failure":
		return true
	default:
		return false
	}
}

type RecordEventParams struct {
	OrganizationID uuid.UUID
	ActorID        *uuid.UUID
	Action         string
	Resource       string
	ResourceID     *string
	Outcome        string
	RequestID      *string
	Metadata       map[string]interface{}
}

type EventRecorder interface {
	RecordEvent(ctx context.Context, params RecordEventParams) error
}
