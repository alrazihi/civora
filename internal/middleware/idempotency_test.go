package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdempotencyStore_GetAndSet(t *testing.T) {
	store := NewIdempotencyStore(50 * time.Millisecond)
	defer store.Stop()

	key := "test-key"
	rec := &IdempotencyRecord{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"success":true}`),
		CreatedAt:  time.Now(),
	}

	store.Set(key, rec)

	got, ok := store.Get(key)
	require.True(t, ok)
	assert.Equal(t, http.StatusOK, got.StatusCode)
	assert.Equal(t, rec.Body, got.Body)
}

func TestIdempotencyStore_TTLExpiration(t *testing.T) {
	store := NewIdempotencyStore(50 * time.Millisecond)
	defer store.Stop()

	key := "expired-key"
	rec := &IdempotencyRecord{
		StatusCode: http.StatusOK,
		Headers:    http.Header{},
		Body:       []byte(`{}`),
		CreatedAt:  time.Now(),
	}

	store.Set(key, rec)

	got, ok := store.Get(key)
	require.True(t, ok)
	assert.Equal(t, http.StatusOK, got.StatusCode)

	time.Sleep(100 * time.Millisecond)

	got, ok = store.Get(key)
	assert.False(t, ok, "expired record should not be found")
	assert.Nil(t, got)
}

func TestIdempotencyStore_CleanupRemovesExpired(t *testing.T) {
	store := NewIdempotencyStore(50 * time.Millisecond)
	defer store.Stop()

	for i := 0; i < 5; i++ {
		store.Set("key-"+string(rune('a'+i)), &IdempotencyRecord{
			StatusCode: http.StatusOK,
			Headers:    http.Header{},
			Body:       []byte(`{}`),
			CreatedAt:  time.Now(),
		})
	}

	time.Sleep(100 * time.Millisecond)

	store.cleanup()

	for i := 0; i < 5; i++ {
		_, ok := store.Get("key-" + string(rune('a'+i)))
		assert.False(t, ok, "expired record should be cleaned up")
	}
}

func TestIdempotencyStore_BackgroundCleanupRuns(t *testing.T) {
	store := NewIdempotencyStore(50 * time.Millisecond)
	defer store.Stop()

	store.Set("bg-key", &IdempotencyRecord{
		StatusCode: http.StatusOK,
		Headers:    http.Header{},
		Body:       []byte(`{}`),
		CreatedAt:  time.Now(),
	})

	time.Sleep(200 * time.Millisecond)

	_, ok := store.Get("bg-key")
	assert.False(t, ok, "background cleanup should have removed expired entry")
}

func TestIdempotencyStore_StopCancelsCleanup(t *testing.T) {
	store := NewIdempotencyStore(10 * time.Millisecond)

	store.Set("key", &IdempotencyRecord{
		StatusCode: http.StatusOK,
		Headers:    http.Header{},
		Body:       []byte(`{}`),
		CreatedAt:  time.Now(),
	})

	store.Stop()

	assert.NotPanics(t, func() {
		store.Stop()
	})
}

func TestIdempotencyKey_ReturnsCachedResponse(t *testing.T) {
	store := NewIdempotencyStore(1 * time.Hour)
	defer store.Stop()

	cachedRec := &IdempotencyRecord{
		StatusCode: http.StatusCreated,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"success":true,"data":123}`),
		CreatedAt:  time.Now(),
	}

	handler := IdempotencyKey(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("should not be called"))
	}))

	r := httptest.NewRequest(http.MethodPost, "/test", nil)
	r.Header.Set("Idempotency-Key", "test-key")

	rec := httptest.NewRecorder()

	store.Set(hashKey(http.MethodPost, "/test", "test-key"), cachedRec)
	handler.ServeHTTP(rec, r)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	body := rec.Body.String()
	assert.Contains(t, body, `"success":true,"data":123`)
}
