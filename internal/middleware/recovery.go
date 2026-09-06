package middleware

import (
	"fmt"
	"net/http"
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
