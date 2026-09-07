package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/alrazihi/civora/internal/config"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	router     *chi.Mux
	cfg        *config.Config
	httpServer *http.Server
	db         *sql.DB
}

func New(cfg *config.Config, db *sql.DB) *Server {
	r := chi.NewRouter()

	rl := middleware.NewRateLimiter(100, 20)

	r.Use(middleware.SecureHeaders)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging)
	r.Use(middleware.Recover)
	r.Use(middleware.CORSHandler(cfg))
	r.Use(middleware.RateLimit(rl))
	r.Use(middleware.IdempotencyKey(middleware.NewIdempotencyStore(24 * time.Hour)))
	r.Use(middleware.BodySizeLimit())

	r.Get("/health", healthHandler)
	r.Get("/ready", readinessHandler(db))
	r.Get("/metrics", metricsHandler)

	return &Server{
		router: r,
		cfg:    cfg,
		db:     db,
		httpServer: &http.Server{
			Addr:              fmt.Sprintf(":%s", cfg.Server.Port),
			Handler:           r,
			ReadTimeout:       cfg.Server.ReadTimeout,
			WriteTimeout:      cfg.Server.WriteTimeout,
			IdleTimeout:       cfg.Server.IdleTimeout,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

func (s *Server) Router() *chi.Mux {
	return s.router
}

func (s *Server) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.httpServer.Shutdown(shutdownCtx)
	}()

	return s.httpServer.ListenAndServe()
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	shared.WriteJSON(w, http.StatusOK, shared.APIResponse{
		Success: true,
		Data:    map[string]interface{}{"status": "ok"},
	})
}

func readinessHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			shared.WriteJSON(w, http.StatusServiceUnavailable, shared.APIResponse{
				Success: false,
				Error: &shared.ErrorResponse{
					Code:    "SERVICE_UNAVAILABLE",
					Message: "database unavailable",
				},
			})
			return
		}
		shared.WriteJSON(w, http.StatusOK, shared.APIResponse{
			Success: true,
			Data:    map[string]interface{}{"status": "ready"},
		})
	}
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# CIVORA metrics\n# endpoint: /metrics\n"))
}
