package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
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
		if id == "" {
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

func Logging(next http.Handler) http.Handler {
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
			RemoteIP:      realIP(r),
			UserAgent:     r.UserAgent(),
		}

		encoded, _ := json.Marshal(entry)
		os.Stdout.Write(append(encoded, '\n'))
	})
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

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
