package middleware

import (
	"net/http"
)

func SecureHeaders(isHTTPSFn func(*http.Request) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			if isHTTPSFn != nil && isHTTPSFn(r) {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ProxyHeaders(trustedProxies []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(trustedProxies) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			remoteIP := RemoteHost(r.RemoteAddr)
			if IsTrustedProxy(remoteIP, trustedProxies) {
				if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" || proto == "http" {
					r.URL.Scheme = proto
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func BodySizeLimit() func(http.Handler) http.Handler {
	const maxBodyBytes = 1 << 20
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBodyBytes {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			next.ServeHTTP(w, r)
		})
	}
}
