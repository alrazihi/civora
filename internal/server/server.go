package server

import (
	"context"
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
}

func New(cfg *config.Config) *Server {
	r := chi.NewRouter()

	idemStore := middleware.NewIdempotencyStore(24 * time.Hour)

	r.Use(middleware.SecureHeaders)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging)
	r.Use(middleware.Recover)
	r.Use(middleware.CORSHandler())
	r.Use(middleware.IdempotencyKey(idemStore))

	r.Get("/health", healthHandler)
	r.Get("/ready", readinessHandler)
	r.Get("/metrics", metricsHandler)

	return &Server{
		router: r,
		cfg:    cfg,
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
			Handler:      r,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
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

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	shared.WriteJSON(w, http.StatusOK, shared.APIResponse{
		Success: true,
		Data:    map[string]interface{}{"status": "ready"},
	})
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# CIVORA metrics\n# endpoint: /metrics\n"))
}
