package middleware

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func alwaysHTTPS(r *http.Request) bool { return true }
func neverHTTPS(r *http.Request) bool  { return false }

func TestSecureHeaders_HSTSOverHTTPS(t *testing.T) {
	handler := SecureHeaders(alwaysHTTPS)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "max-age=31536000; includeSubDomains", rec.Header().Get("Strict-Transport-Security"))
}

func TestSecureHeaders_NoHSTSOverHTTP(t *testing.T) {
	handler := SecureHeaders(neverHTTPS)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Strict-Transport-Security"))
}

func TestSecureHeaders_SetsOtherHeaders(t *testing.T) {
	handler := SecureHeaders(alwaysHTTPS)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "no-store, no-cache, must-revalidate", rec.Header().Get("Cache-Control"))
}

func TestSecureHeaders_NoHSTSWhenNilFunction(t *testing.T) {
	handler := SecureHeaders(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Strict-Transport-Security"))
}

func TestBodySizeLimit_AllowsNormalSize(t *testing.T) {
	handler := BodySizeLimit()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("small body"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodySizeLimit_RejectsOversized(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := BodySizeLimit()(handler)
	ts := httptest.NewServer(wrapped)
	defer ts.Close()

	largeBody := strings.Repeat("a", 2<<20)
	resp, err := http.Post(ts.URL, "application/json", strings.NewReader(largeBody))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
}

func TestBodySizeLimit_RejectsOversizedWithoutContentLength(t *testing.T) {
	handler := BodySizeLimit()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL, &chunkedReader{data: make([]byte, 2<<20)})
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
}

type chunkedReader struct {
	data []byte
	pos  int
}

func (c *chunkedReader) Read(p []byte) (int, error) {
	if c.pos >= len(c.data) {
		return 0, io.EOF
	}
	n := copy(p, c.data[c.pos:])
	c.pos += n
	return n, nil
}

func TestCORS_SetsHeaders(t *testing.T) {
	handler := CORSHandler(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_DoesNotAllowAllOrigins(t *testing.T) {
	handler := CORSHandler(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	badOrigins := []string{
		"http://evil.com",
		"https://malicious-site.com",
	}

	for _, origin := range badOrigins {
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Content-Type")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
		assert.NotEqual(t, "*", allowOrigin, "CORS should not allow wildcard origin %s", origin)
	}
}

func TestProxyHeaders_SetsSchemeFromTrustedProxy(t *testing.T) {
	handler := ProxyHeaders([]string{"10.0.0.1"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "https", req.URL.Scheme)
}

func TestProxyHeaders_IgnoresUntrustedProxy(t *testing.T) {
	handler := ProxyHeaders([]string{"10.0.0.1"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Empty(t, req.URL.Scheme)
}

func TestProxyHeaders_IgnoresMalformedProto(t *testing.T) {
	handler := ProxyHeaders([]string{"10.0.0.1"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-Proto", "ftp")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Empty(t, req.URL.Scheme)
}

func TestProxyHeaders_NoopWhenEmpty(t *testing.T) {
	handler := ProxyHeaders(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Empty(t, req.URL.Scheme)
}

func TestRealIP_ReturnsDirectIPForUntrusted(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	ip := RealIP(req, []string{"10.0.0.1"})
	assert.Equal(t, "192.168.1.100", ip)
}

func TestRealIP_ReturnsForwardedForTrusted(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 198.51.100.25")
	ip := RealIP(req, []string{"10.0.0.1"})
	assert.Equal(t, "203.0.113.50", ip)
}

func TestRealIP_IgnoresMalformedForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "not-an-ip, also-bad")
	ip := RealIP(req, []string{"10.0.0.1"})
	assert.Equal(t, "10.0.0.1", ip)
}

func TestIsTrustedProxy_MatchesCIDR(t *testing.T) {
	assert.True(t, IsTrustedProxy("10.0.0.5", []string{"10.0.0.0/24"}))
	assert.False(t, IsTrustedProxy("10.0.1.5", []string{"10.0.0.0/24"}))
}

func TestIsTrustedProxy_MatchesExact(t *testing.T) {
	assert.True(t, IsTrustedProxy("10.0.0.1", []string{"10.0.0.1"}))
	assert.False(t, IsTrustedProxy("10.0.0.2", []string{"10.0.0.1"}))
}

func TestIsTrustedProxy_RejectsMalformed(t *testing.T) {
	assert.False(t, IsTrustedProxy("not-an-ip", []string{"not-an-ip"}))
}

func TestRemoteHost_ParsesHostPort(t *testing.T) {
	assert.Equal(t, "10.0.0.1", RemoteHost("10.0.0.1:8080"))
	assert.Equal(t, "10.0.0.1", RemoteHost("10.0.0.1"))
}

func TestRemoteHost_HandlesBareAddress(t *testing.T) {
	assert.Equal(t, "10.0.0.1", RemoteHost("10.0.0.1"))
}
