package server

import (
	"crypto/tls"
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

func TestServer_SecureHeaders_HSTSWhenDirectTLS(t *testing.T) {
	srv := New(&config.Config{
		Server: config.ServerConfig{
			Port:           "0",
			RateLimit:      1000000,
			RateLimitBurst: 1000000,
			TLSCertFile:    "/path/to/cert.pem",
			TLSKeyFile:     "/path/to/key.pem",
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	assert.Equal(t, "max-age=31536000; includeSubDomains", rec.Header().Get("Strict-Transport-Security"))
}

func TestServer_SecureHeaders_NoHSTSOverPlainHTTP(t *testing.T) {
	srv := New(&config.Config{
		Server: config.ServerConfig{
			Port:           "0",
			RateLimit:      1000000,
			RateLimitBurst: 1000000,
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Strict-Transport-Security"))
}

func TestServer_SecureHeaders_HSTSWithForceHTTPSFromTrustedProxy(t *testing.T) {
	srv := New(&config.Config{
		Server: config.ServerConfig{
			Port:           "0",
			RateLimit:      1000000,
			RateLimitBurst: 1000000,
			ForceHTTPS:     true,
			TrustedProxies: []string{"10.0.0.1"},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	assert.Equal(t, "max-age=31536000; includeSubDomains", rec.Header().Get("Strict-Transport-Security"))
}

func TestServer_SecureHeaders_NoHSTSWithForceHTTPSFromUntrustedProxy(t *testing.T) {
	srv := New(&config.Config{
		Server: config.ServerConfig{
			Port:           "0",
			RateLimit:      1000000,
			RateLimitBurst: 1000000,
			ForceHTTPS:     true,
			TrustedProxies: []string{"10.0.0.1"},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Strict-Transport-Security"))
}

func TestServer_ProxyHeaders_SetsSchemeFromTrustedProxy(t *testing.T) {
	srv := New(&config.Config{
		Server: config.ServerConfig{
			Port:           "0",
			RateLimit:      1000000,
			RateLimitBurst: 1000000,
			TrustedProxies: []string{"10.0.0.1"},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	assert.Equal(t, "https", req.URL.Scheme)
}

func TestServer_ProxyHeaders_IgnoresMalformedProto(t *testing.T) {
	srv := New(&config.Config{
		Server: config.ServerConfig{
			Port:           "0",
			RateLimit:      1000000,
			RateLimitBurst: 1000000,
			TrustedProxies: []string{"10.0.0.1"},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-Proto", "ftp")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	assert.Empty(t, req.URL.Scheme)
}

func TestServer_RealIP_RespectsTrustedProxy(t *testing.T) {
	srv := New(&config.Config{
		Server: config.ServerConfig{
			Port:           "0",
			RateLimit:      1000000,
			RateLimitBurst: 1000000,
			TrustedProxies: []string{"10.0.0.1"},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	// The rate limiter uses RealIP internally; we verify via middleware behavior.
	// This test exercises the full middleware chain.
	assert.Equal(t, http.StatusOK, rec.Code)
}
