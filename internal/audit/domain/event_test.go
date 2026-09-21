package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuditEvent(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	action := "case.created"
	resource := "case"
	resourceID := "case-123"

	ev, err := NewAuditEvent(orgID, actorID, action, resource, &resourceID, "success", nil, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, orgID, ev.OrganizationID)
	assert.Equal(t, &actorID, ev.ActorID)
	assert.Equal(t, action, ev.Action)
	assert.Equal(t, resource, ev.Resource)
	assert.Equal(t, &resourceID, ev.ResourceID)
	assert.Equal(t, "success", ev.Outcome)
	assert.NotEmpty(t, ev.Hash)
	assert.Nil(t, ev.PreviousHash)
	assert.False(t, ev.Timestamp.IsZero())
}

func TestAuditEvent_ComputeHash(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()

	ev1, err := NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil,
		map[string]interface{}{"seq": 1}, nil)
	require.NoError(t, err)
	ev2, err := NewAuditEvent(orgID, actorID, "case.closed", "case", nil, "success", nil,
		map[string]interface{}{"seq": 2}, nil)
	require.NoError(t, err)

	assert.NotEmpty(t, ev1.Hash)
	assert.NotEqual(t, ev1.Hash, ev2.Hash, "events with different content should have different hashes")
}

func TestAuditEvent_VerifyIntegrity(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()

	ev, err := NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil, nil, nil)
	require.NoError(t, err)
	assert.True(t, ev.VerifyIntegrity())
}

func TestAuditEvent_TamperDetection(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()

	ev, err := NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil, nil, nil)
	require.NoError(t, err)
	assert.True(t, ev.VerifyIntegrity())

	ev.Outcome = "failure"
	assert.False(t, ev.VerifyIntegrity(), "tampered event should fail integrity check")
}

func TestAuditEvent_HashChain(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()

	ev1, err := NewAuditEvent(orgID, actorID, "case.created", "case", strPtr("case-1"), "success", nil, nil, nil)
	require.NoError(t, err)
	ev2, err := NewAuditEvent(orgID, actorID, "case.status_changed", "case", strPtr("case-1"), "success", nil, nil, &ev1.Hash)
	require.NoError(t, err)

	assert.True(t, ev1.VerifyIntegrity())
	assert.True(t, ev2.VerifyIntegrity())

	require.NotNil(t, ev2.PreviousHash)
	assert.Equal(t, ev1.Hash, *ev2.PreviousHash)
}

func TestIsValidOutcome(t *testing.T) {
	assert.True(t, IsValidOutcome("success"))
	assert.True(t, IsValidOutcome("failure"))
	assert.False(t, IsValidOutcome("unknown"))
	assert.False(t, IsValidOutcome(""))
}

func TestComputeHashManual(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()

	ev := &AuditEvent{
		OrganizationID: orgID,
		ActorID:        &actorID,
		Action:         "test.action",
		Resource:       "test",
		Outcome:        "success",
		Timestamp:      time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	hash, err := ev.ComputeHash()
	require.NoError(t, err)
	ev.Hash = hash

	metaBytes, _ := json.Marshal(ev.Metadata)

	expected := sha256.New()
	expected.Write([]byte(ev.OrganizationID.String()))
	expected.Write([]byte("|"))
	expected.Write([]byte(ev.ActorID.String()))
	expected.Write([]byte("|"))
	expected.Write([]byte("test.action"))
	expected.Write([]byte("|"))
	expected.Write([]byte("test"))
	expected.Write([]byte("|"))
	expected.Write([]byte("|"))
	expected.Write([]byte("success"))
	expected.Write([]byte("|"))
	expected.Write([]byte("|"))
	expected.Write(metaBytes)
	expected.Write([]byte("|"))
	expected.Write([]byte(ev.Timestamp.UTC().Format("2006-01-02T15:04:05.000000Z07:00")))
	expected.Write([]byte("|"))

	expectedHash := hex.EncodeToString(expected.Sum(nil))
	assert.Equal(t, expectedHash, ev.Hash)
}

func strPtr(s string) *string {
	return &s
}

func TestVerifyChain(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()

	ev1, err := NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil, nil, nil)
	require.NoError(t, err)
	ev2, err := NewAuditEvent(orgID, actorID, "case.status_changed", "case", nil, "success", nil, nil, &ev1.Hash)
	require.NoError(t, err)
	ev3, err := NewAuditEvent(orgID, actorID, "case.closed", "case", nil, "success", nil, nil, &ev2.Hash)
	require.NoError(t, err)

	verified, failed, err := VerifyChain([]*AuditEvent{ev1, ev2, ev3})
	require.NoError(t, err)
	assert.Equal(t, 3, verified, "all events in a valid chain should pass")
	assert.Equal(t, 0, failed, "no events should fail in a valid chain")

	ev2.PreviousHash = strPtr("broken")
	verified, failed, err = VerifyChain([]*AuditEvent{ev1, ev2, ev3})
	assert.Equal(t, 2, verified, "ev1 and ev3 should still pass")
	assert.Equal(t, 1, failed, "ev2 should fail because its previous hash does not match ev1's hash")
	assert.Error(t, err, "verification failures should return an error")
}

func TestVerifyChain_Empty(t *testing.T) {
	verified, failed, err := VerifyChain(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, verified)
	assert.Equal(t, 0, failed)
}
