package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/alrazihi/civora/internal/shared"
)

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.WriteHeader(http.StatusInternalServerError)
				body, _ := json.Marshal(shared.APIResponse{
					Success: false,
					Error: &shared.ErrorResponse{
						Code:    string(shared.CodeInternalError),
						Message: "Internal server error",
					},
				})
				w.Write(body)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
