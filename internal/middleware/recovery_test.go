package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecover_PanicDoesNotLeakDetails(t *testing.T) {
	handler := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("internal database connection string: postgres://user:pass@host:5432/db")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "postgres://")
	assert.NotContains(t, rec.Body.String(), "connection string")
	assert.NotContains(t, rec.Body.String(), "internal database")
	assert.Contains(t, rec.Body.String(), "Internal server error")
}

func TestRecover_PanicReturnsValidJSON(t *testing.T) {
	handler := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected value with \"quotes\" and \\backslash\\ and <script> tags")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err, "response should be valid JSON even when panic message contains special chars")
	assert.False(t, resp["success"].(bool))
	errObj := resp["error"].(map[string]interface{})
	assert.Equal(t, "Internal server error", errObj["message"])
}
