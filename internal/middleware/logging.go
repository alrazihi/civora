package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

type contextKey string

type logEntry struct {
	Time          string `json:"time"`
	Level         string `json:"level"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	Status        int    `json:"status"`
	BytesWritten  int    `json:"bytes_written"`
	LatencyMicros int64  `json:"latency_microseconds"`
	RequestID     string `json:"request_id"`
	RemoteIP      string `json:"remote_ip"`
	UserAgent     string `json:"user_agent"`
}

const requestIDKey contextKey = "request_id"

func GetRequestID(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey).(string)
	return id
}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{ResponseWriter: w, statusCode: 200}
}

func (rw *loggingResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

func Logging(trustedProxies []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newLoggingResponseWriter(w)

			requestID := GetRequestID(r)

			next.ServeHTTP(rw, r)

			latency := time.Since(start)
			entry := logEntry{
				Time:          time.Now().UTC().Format(time.RFC3339Nano),
				Level:         logLevel(rw.statusCode),
				Method:        r.Method,
				Path:          r.URL.Path,
				Status:        rw.statusCode,
				BytesWritten:  rw.bytesWritten,
				LatencyMicros: latency.Microseconds(),
				RequestID:     requestID,
				RemoteIP:      realIP(r, trustedProxies),
				UserAgent:     r.UserAgent(),
			}

			encoded, err := json.Marshal(entry)
			if err != nil {
				// Marshal of a logEntry struct cannot fail in practice, but if it
				// does we must not silently drop the log line.
				if _, werr := os.Stdout.Write(append([]byte(`{"level":"error","message":"failed to marshal log entry"}`), '\n')); werr != nil {
					_ = werr
				}
				return
			}
			if _, err := os.Stdout.Write(append(encoded, '\n')); err != nil {
				// Stdout write failure (pipe closed, disk full) is unrecoverable at
				// request time; nothing can be done except drop the line.
				_ = err
			}
		})
	}
}

func logLevel(statusCode int) string {
	if statusCode >= 500 {
		return "error"
	}
	if statusCode >= 400 {
		return "warn"
	}
	return "info"
}

func realIP(r *http.Request, trustedProxies []string) string {
	remoteIP := remoteHost(r.RemoteAddr)
	if !isTrustedProxy(remoteIP, trustedProxies) {
		return remoteIP
	}

	if ip := firstForwardedIP(r.Header.Get("X-Forwarded-For")); ip != "" {
		return ip
	}
	if ip := parseIP(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	return remoteIP
}

func remoteHost(address string) string {
	address = strings.TrimSpace(address)
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return strings.TrimSpace(host)
	}
	if ip := net.ParseIP(address); ip != nil {
		return ip.String()
	}
	return address
}

func isTrustedProxy(ip string, trustedProxies []string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	for _, trusted := range trustedProxies {
		trusted = strings.TrimSpace(trusted)
		if trusted == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(trusted); err == nil && network.Contains(parsedIP) {
			return true
		}
		if parsedTrusted := net.ParseIP(trusted); parsedTrusted != nil && parsedTrusted.Equal(parsedIP) {
			return true
		}
	}
	return false
}

func firstForwardedIP(header string) string {
	for _, candidate := range strings.Split(header, ",") {
		if ip := parseIP(candidate); ip != "" {
			return ip
		}
	}
	return ""
}

func parseIP(value string) string {
	if ip := net.ParseIP(strings.TrimSpace(value)); ip != nil {
		return ip.String()
	}
	return ""
}
