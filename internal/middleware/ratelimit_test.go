package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRateLimiter_IsLockedReturnsFalseForNewEmail(t *testing.T) {
	rl := NewUserRateLimiter(3, 15*time.Minute, 15*time.Minute)
	email := "test@example.com"

	locked, remaining := rl.IsLocked(email)
	t.Logf("IsLocked result: locked=%v, remaining=%v", locked, remaining)
	assert.False(t, locked, "new email should not be locked")
	assert.Equal(t, time.Duration(0), remaining, "remaining should be 0")
}

func TestUserRateLimiter_LocksAfterMaxFailures(t *testing.T) {
	rl := NewUserRateLimiter(3, 15*time.Minute, 15*time.Minute)
	email := "test@example.com"

	lockedFromIsLocked, remainingFromIsLocked := rl.IsLocked(email)
	t.Logf("Direct IsLocked call: locked=%v, remaining=%v", lockedFromIsLocked, remainingFromIsLocked)

	allowed, _, retryAfter := rl.CheckRateLimit(email)
	t.Logf("CheckRateLimit call: allowed=%v, retryAfter=%v", allowed, retryAfter)

	require.True(t, allowed, "should be allowed before any failures")

	rl.RecordFailedAttempt(email)
	allowed, _, _ = rl.CheckRateLimit(email)
	assert.True(t, allowed, "should be allowed with 1 failure")

	rl.RecordFailedAttempt(email)
	allowed, _, _ = rl.CheckRateLimit(email)
	assert.True(t, allowed, "should be allowed with 2 failures")

	rl.RecordFailedAttempt(email)
	allowed, _, retryAfter = rl.CheckRateLimit(email)
	require.False(t, allowed, "should be blocked after 3 failures, got allowed=%v", allowed)
	assert.Greater(t, retryAfter, time.Duration(0), "should have retry-after duration")

	allowed2, _, retryAfter2 := rl.CheckRateLimit(email)
	assert.False(t, allowed2, "should remain blocked")
	assert.Greater(t, retryAfter2, time.Duration(0), "should have retry-after duration")
}

func TestUserRateLimiter_ResetsOnSuccess(t *testing.T) {
	rl := NewUserRateLimiter(3, 15*time.Minute, 15*time.Minute)
	email := "test@example.com"

	rl.RecordFailedAttempt(email)
	rl.RecordFailedAttempt(email)

	rl.RecordSuccessfulAttempt(email)

	allowed, _, _ := rl.CheckRateLimit(email)
	assert.True(t, allowed, "should be allowed after successful attempt")
}

func TestUserRateLimiter_TracksSeparateEmails(t *testing.T) {
	rl := NewUserRateLimiter(3, 15*time.Minute, 15*time.Minute)
	email1 := "user1@example.com"
	email2 := "user2@example.com"

	rl.RecordFailedAttempt(email1)
	rl.RecordFailedAttempt(email1)
	rl.RecordFailedAttempt(email1)

	allowed1, _, _ := rl.CheckRateLimit(email1)
	allowed2, _, _ := rl.CheckRateLimit(email2)

	assert.False(t, allowed1, "email1 should be blocked")
	assert.True(t, allowed2, "email2 should be allowed")
}

func TestUserRateLimiter_CaseInsensitive(t *testing.T) {
	rl := NewUserRateLimiter(2, 15*time.Minute, 15*time.Minute)

	rl.RecordFailedAttempt("User@Example.com")
	rl.RecordFailedAttempt("user@example.com")

	allowed, _, _ := rl.CheckRateLimit("USER@EXAMPLE.COM")
	assert.False(t, allowed, "should be blocked regardless of case")
}

func TestUserRateLimiter_RetryAfterDuration(t *testing.T) {
	rl := NewUserRateLimiter(3, 5*time.Minute, 5*time.Minute)
	email := "test@example.com"

	rl.RecordFailedAttempt(email)
	rl.RecordFailedAttempt(email)
	rl.RecordFailedAttempt(email)

	_, _, retryAfter := rl.CheckRateLimit(email)
	assert.GreaterOrEqual(t, retryAfter, 4*time.Minute, "retry-after should be near lockout duration")
}
