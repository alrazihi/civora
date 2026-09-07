package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				body := fmt.Sprintf(`{"success":false,"error":{"code":"INTERNAL_ERROR","message":"Internal server error"}}`)
				w.Write([]byte(body))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

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
