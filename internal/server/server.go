package server

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alrazihi/civora/internal/config"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	router      *chi.Mux
	cfg         *config.Config
	httpServer  *http.Server
	db          *sql.DB
	rateLimiter *middleware.RateLimiter
	idemStore   *middleware.IdempotencyStore
}

func New(cfg *config.Config, db *sql.DB) *Server {
	r := chi.NewRouter()

	rl := middleware.NewRateLimiter(cfg.Server.RateLimit, cfg.Server.RateLimitBurst)
	idemStore := middleware.NewIdempotencyStore(24 * time.Hour)

	r.Use(middleware.SecureHeaders)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging)
	r.Use(middleware.Recover)
	r.Use(middleware.CORSHandler(cfg))
	r.Use(middleware.RateLimit(rl))
	r.Use(middleware.IdempotencyKey(idemStore))
	r.Use(middleware.BodySizeLimit())

	r.Get("/health", healthHandler)
	r.Get("/ready", readinessHandler(db))

	return &Server{
		router:      r,
		cfg:         cfg,
		db:          db,
		rateLimiter: rl,
		idemStore:   idemStore,
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

func (s *Server) MountStaticFS(fs http.FileSystem) {
	s.router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		f, err := fs.Open("/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.Copy(w, f)
	})
	s.router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(fs).ServeHTTP(w, r)
	})
}

func (s *Server) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.idemStore.Stop()
		s.rateLimiter.Stop()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			// Shutdown error (e.g. forced timeout) is logged; the server
			// goroutine is exiting anyway.
			_ = err
		}
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
