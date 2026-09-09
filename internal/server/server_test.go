package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alrazihi/civora/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestMountStaticFS_ServesIndexHTML(t *testing.T) {
	srv := New(&config.Config{
		Server: config.ServerConfig{
			Port:           "0",
			RateLimit:      1000000,
			RateLimitBurst: 1000000,
		},
	}, nil)

	fs := http.Dir("../../web")
	srv.MountStaticFS(fs)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
}

func TestMountStaticFS_ServesStaticFile(t *testing.T) {
	srv := New(&config.Config{
		Server: config.ServerConfig{
			Port:           "0",
			RateLimit:      1000000,
			RateLimitBurst: 1000000,
		},
	}, nil)

	fs := http.Dir("../../web")
	srv.MountStaticFS(fs)

	req := httptest.NewRequest(http.MethodGet, "/css/style.css", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/css")
}

func TestMountStaticFS_ReturnsNotFoundForMissingFile(t *testing.T) {
	srv := New(&config.Config{
		Server: config.ServerConfig{
			Port:           "0",
			RateLimit:      1000000,
			RateLimitBurst: 1000000,
		},
	}, nil)

	fs := http.Dir("../../web")
	srv.MountStaticFS(fs)

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist.txt", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
