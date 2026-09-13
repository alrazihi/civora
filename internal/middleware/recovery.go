package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/alrazihi/civora/internal/shared"
)

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC recovered: %v\n%s", rec, debug.Stack())
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.WriteHeader(http.StatusInternalServerError)
				body, err := json.Marshal(shared.APIResponse{
					Success: false,
					Error: &shared.ErrorResponse{
						Code:    string(shared.CodeInternalError),
						Message: "Internal server error",
					},
				})
				if err != nil {
					log.Printf("marshal recovery response: %v", err)
					return
				}
				if _, err := w.Write(body); err != nil {
					log.Printf("write recovery response: %v", err)
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}
