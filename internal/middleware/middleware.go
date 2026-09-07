package middleware

import (
	"net/http"

	"github.com/alrazihi/civora/internal/config"
	"github.com/go-chi/cors"
)

func CORSHandler(cfg *config.Config) func(http.Handler) http.Handler {
	origins := []string{}
	if cfg != nil && len(cfg.Server.CORSOrigins) > 0 {
		origins = cfg.Server.CORSOrigins
	}
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000"}
	}
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
		ExposedHeaders:   []string{"X-Request-ID"},
		MaxAge:           86400,
	})
}
