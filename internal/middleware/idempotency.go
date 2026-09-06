package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

type IdempotencyStore struct {
	mu   sync.RWMutex
	data map[string]*IdempotencyRecord
	ttl  time.Duration
}

type IdempotencyRecord struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	CreatedAt  time.Time
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	return &IdempotencyStore{
		data: make(map[string]*IdempotencyRecord),
		ttl:  ttl,
	}
}

func (s *IdempotencyStore) Get(key string) (*IdempotencyRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.data[key]
	if !ok || time.Since(rec.CreatedAt) > s.ttl {
		return nil, false
	}
	return rec, ok
}

func (s *IdempotencyStore) Set(key string, rec *IdempotencyRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = rec
}

func (s *IdempotencyStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.data {
		if now.Sub(v.CreatedAt) > s.ttl {
			delete(s.data, k)
		}
	}
}

func IdempotencyKey(store *IdempotencyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost && r.Method != http.MethodPut {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			cacheKey := hashKey(r.Method, r.URL.Path, key)
			if rec, ok := store.Get(cacheKey); ok {
				for k, vs := range rec.Headers {
					for _, v := range vs {
						w.Header().Set(k, v)
					}
				}
				w.WriteHeader(rec.StatusCode)
				w.Write(rec.Body)
				return
			}

			rww := newIdempotencyResponseWriter(w)
			next.ServeHTTP(rww, r)

			store.Set(cacheKey, &IdempotencyRecord{
				StatusCode: rww.statusCode,
				Headers:    rww.Header().Clone(),
				Body:       rww.body.Bytes(),
				CreatedAt:  time.Now(),
			})
		})
	}
}

func hashKey(method, path, key string) string {
	h := sha256.New()
	h.Write([]byte(method + ":" + path + ":" + key))
	return hex.EncodeToString(h.Sum(nil))
}

type idempotencyResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func newIdempotencyResponseWriter(w http.ResponseWriter) *idempotencyResponseWriter {
	return &idempotencyResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (w *idempotencyResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *idempotencyResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
