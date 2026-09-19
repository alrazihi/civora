# Stage 9: Reliability & Failure Hostile Review

**Status**: IN REVIEW

## Executive Summary

This audit examines CIVORA's resilience to operational failures, stress conditions, and hostile attack patterns targeting reliability mechanisms. Key areas: transaction boundaries, rollback correctness, idempotency, retry behavior, timeout handling, connection management, and race conditions.

---

## Failure Mode Analysis

### Database Failure Scenarios

#### 1. Database Unavailable (Startup)

**Location**: `internal/database/db.go:17-31`

**Current Behavior**:
```go
func waitForDatabase(db *sql.DB) error {
    maxRetries := 30
    delay := 1 * time.Second
    for i := 0; i < maxRetries; i++ {
        if err := db.PingContext(context.Background()); err == nil {
            return nil
        }
        time.Sleep(delay)
        delay = time.Duration(float64(delay) * 1.5)
    }
    return fmt.Errorf("database not reachable after %d retries", maxRetries)
}
```

**Finding**: FAIL - Excessive startup latency

- **Issue**: 30 retries with exponential backoff = ~8 minutes maximum wait time
- **Impact**: Service takes egregiously long to fail fast during outages
- **Recommendation**: Reduce to 5-10 retries (~30-60 seconds max) for faster failure detection

#### 2. Database Unreachable During Request

**Location**: `internal/server/server.go:144-160` (readiness probe)

**Current Behavior**: Readiness probe checks `db.Ping()`. Uses HTTP 503 when unreachable.

**Finding**: PASS but with limitations

- Database connection pool settings in `db.go:23-24`:
  ```go
  db.SetMaxOpenConns(25)
  db.SetMaxIdleConns(5)
  ```
- Pool is reasonable but `MaxOpenConns` may be too low under high load
- **Recommendation**: Increase `MaxOpenConns` to 100 for production

#### 3. Transaction Rollback

**Location**: `internal/database/db.go:57-79`

**Current Behavior**:
```go
func InTransaction(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
    if db == nil {
        return fmt.Errorf("database connection is nil")
    }
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    if err := fn(tx); err != nil {
        if rbErr := tx.Rollback(); rbErr != nil {
            return fmt.Errorf("failed to rollback transaction: %w (original error: %w)", rbErr, err)
        }
        return err
    }
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    return nil
}
```

**Finding**: PASS - Proper rollback handling

- Rollback errors are wrapped with original error
- Transaction context is passed through properly

---

### Concurrency & Race Conditions

#### 1. Concurrent Case Transitions

**Location**: `test/integration/concurrency_test.go:81-157`

**Finding**: PASS - Tested

- Tests verify exactly one concurrent transition succeeds
- Uses database-level locking via `SELECT ... FOR UPDATE` patterns
- Database constraints enforce invariant integrity

#### 2. Concurrent Duplicate Submissions

**Location**: `test/integration/concurrency_test.go:159-208`

**Finding**: PASS - All succeed as expected (idempotent operations)

- Creating assistance records should be idempotent with unique constraints

#### 3. Concurrent Duplicate Decisions

**Location**: `test/integration/concurrency_test.go:210-305`

**Finding**: PASS - Tested, exactly one decision succeeds

- Duplicate decision detection prevents multiple decisions

#### 4. Concurrent Form Submissions

**Location**: `internal/cases/application/service.go:796-929`

**Finding**: RAISED - No explicit duplicate detection

```go
existingSubmission, err := s.submissionRepo.FindByCaseAndFormVersionForUpdateTx(ctx, tx, orgID, caseID, formVersionID)
if err != nil && err != submissiondomain.ErrSubmissionNotFound {
    return fmt.Errorf("failed to check existing submission: %w", err)
}
if existingSubmission != nil {
    return ErrSubmissionExists
}
```

The duplicate check happens inside the transaction, which prevents race conditions. This is correct.

#### 5. Race Condition in Rate Limiter

**Location**: `internal/middleware/ratelimit.go:65-89`

**Finding**: PASS - Uses mutex for all mutations

```go
func (rl *RateLimiter) checkRate(ip string) (allowed bool, retryAfter time.Duration) {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    // ...
}
```

- All map mutations are protected by mutex
- `defer mutex.Unlock()` ensures release even on panic

#### 6. Race Condition in Idempotency Store

**Location**: `internal/middleware/idempotency.go`

**Finding**: PASS - Uses mutex for all operations

- `Get()` uses `RLock()`, `Set()` uses `Lock()`
- Proper cleanup goroutine with stop channel

---

### Timeouts & Resource Limits

#### 1. HTTP Request Timeouts

**Location**: `internal/server/server.go:54-62`

**Current Behavior**:
```go
httpServer: &http.Server{
    ReadTimeout:       cfg.Server.ReadTimeout,    // 30s default
    WriteTimeout:      cfg.Server.WriteTimeout,   // 30s default
    IdleTimeout:       cfg.Server.IdleTimeout,    // 120s default
    ReadHeaderTimeout: 10 * time.Second,
}
```

**Finding**: PASS - Reasonable timeouts configured

- Read/Write timeouts prevent slow-loris attacks
- Idle timeout releases connections

#### 2. Request Body Size Limit

**Location**: `internal/middleware/headers.go:22-34`

**Current Behavior**:
```go
func BodySizeLimit() func(http.Handler) http.Handler {
    const maxBodyBytes = 1 << 20  // 1 MB
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
```

**Finding**: PASS - Hard limit enforces protection

- `http.MaxBytesReader` prevents memory exhaustion
- Limit is enforced before request processing

**Note**: Storage provider allows 10MB files (`MaxFileSize = 10 << 20`), but API limit is 1MB. This mismatch could be confusing.

#### 3. AI Provider Timeout

**Location**: `internal/ai/infrastructure/provider/openai.go:54`

**Current Behavior**:
```go
if cfg.HTTPClient == nil {
    cfg.HTTPClient = &http.Client{Timeout: defaultOpenAITimeout}  // 60s
}
```

**Finding**: PASS - 60 second timeout

- Sufficient for typical LLM responses
- Context propagation allows cancellation

#### 4. Database Connection Timeout

**Location**: `internal/database/db.go:33-46`

**Finding**: FAIL - Exponential backoff grows too large

- After 30 retries with 1.5x growth, final delay is ~13 minutes
- Total startup wait could exceed 8 minutes in degraded state

---

### File Upload Attack Scenarios

#### 1. Malformed Uploaded Files

**Location**: `internal/evidence/infrastructure/storage/storage.go:63-84`

**Current Behavior**:
```go
func SanitizeFileName(name string) (string, error) {
    if name == "" || name == "." || name == ".." {
        return "", fmt.Errorf("%w: filename is empty or dangerous", ErrInvalidFilename)
    }
    cleaned := filepath.Base(name)
    cleaned = strings.ReplaceAll(cleaned, "..", "")
    // ...
}
```

**Finding**: PASS - Path traversal prevented

- `filepath.Base()` strips directory components
- Recursive `..` removal handles double-encoding attacks
- Null byte filtering (characters < 32) prevents null-byte injection

#### 2. Huge File Uploads

**Location**: `internal/evidence/infrastructure/storage/local_provider.go:28-30`

**Current Behavior**:
```go
func (p *LocalStorageProvider) Store(ctx context.Context, key string, reader io.Reader, size int64) (StorageMetadata, error) {
    if size > MaxFileSize {
        return StorageMetadata{}, fmt.Errorf("%w: %d bytes exceeds limit of %d bytes", ErrFileTooLarge, size, MaxFileSize)
    }
```

**Finding**: PASS - 10MB limit enforced

- Check happens before `io.Copy`
- File is deleted on mismatch (lines 49-50, 54-55)

#### 3. Content-Type Validation

**Location**: `internal/evidence/infrastructure/storage/storage.go:86-100`

**Finding**: PASS - Allowlist enforced

```go
var AllowedContentTypes = map[string]bool{
    "application/pdf": true,
    "image/png": true,
    // ...
}
```

---

### Audit & Maintenance Failure Scenarios

#### 1. Background Maintenance Failure

**Location**: `internal/audit/infrastructure/maintenance.go:141-161`

**Current Behavior**:
```go
go func() {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-stop:
            return
        case <-ticker.C:
            verified, failed, _, _, err := s.RunOnce(ctx)
            if err != nil {
                s.logger.Printf("audit maintenance error: %v", err)
            }
            // ...
        }
    }
}()
```

**Finding**: WARN - Silent failure on maintenance errors

- Errors are logged but not escalated
- Failed retention purges leave data uncleaned
- **Recommendation**: Implement dead-letter queue or alerting on repeated failures

#### 2. Recovery from Panic

**Location**: `internal/middleware/recovery.go:12-38`

**Finding**: PASS - Panic recovery implemented

```go
func Recover(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                log.Printf("PANIC recovered: %v\n%s", rec, debug.Stack())
                // ...
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

- Panics are logged with full stack trace
- Returns 500 with sanitized message to client
- Stack trace is never sent to client

---

### Retry Storm & Duplicate Request Scenarios

#### 1. Idempotency Implementation

**Location**: `internal/middleware/idempotency.go:89-126`

**Finding**: PASS - Proper idempotency support

- `Idempotency-Key` header triggers caching
- Caches response including status code and headers
- 24-hour TTL prevents memory growth
- Cleanup goroutine removes expired entries

#### 2. Duplicate Request Handling

**Finding**: PASS - At idempotency layer

- Middleware returns cached response for duplicate POST/PUT
- No duplicate database writes occur for idempotent requests

---

### Process Restart Scenarios

#### 1. Graceful Shutdown

**Location**: `internal/server/server.go:85-106`

**Current Behavior**:
```go
func (s *Server) Start(ctx context.Context) error {
    go func() {
        <-ctx.Done()
        shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
        defer cancel()
        s.idemStore.Stop()
        s.rateLimiter.Stop()
        if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
            _ = err
        }
    }()
    // ...
}
```

**Finding**: PASS - Graceful shutdown implemented

- 10-second shutdown grace period
- Stops idempotency store and rate limiter before HTTP shutdown
- `http.Server.Shutdown()` completes in-flight requests

#### 2. Background Service Cleanup

**Location**: `main.go:197-198`

```go
maintenanceStop := auditMaintenance.StartBackground(ctx, 24*time.Hour)
defer close(maintenanceStop)
```

**Finding**: WARN - Maintenance goroutine may be interrupted

- If main context is cancelled before maintenance completes current cycle, goroutine exits cleanly
- However, 24-hour ticker means potentially up to 24 hours of missed maintenance on crash

---

### Memory Growth Analysis

#### 1. Rate Limiter Memory

**Location**: `internal/middleware/ratelimit.go:12-26`

**Finding**: MONITOR - Unbounded growth potential

```go
type RateLimiter struct {
    visitors map[string]*visitor  // grows indefinitely until cleanup
    ttl      time.Duration        // 5 minutes
}
```

- Cleanup runs every 5 minutes
- IPs never reconnect remain in map until TTL expires
- With high IP diversity, could grow large
- **Recommendation**: Add max entries or LRU eviction

#### 2. Idempotency Store Memory

**Location**: `internal/middleware/idempotency.go:14-21`

**Finding**: MONITOR - 24-hour TTL is reasonable

- 24-hour cleanup is appropriate for idempotency
- Size bounded by request volume in last 24 hours
- **Recommendation**: Consider max entries limit for protection

---

### AI Provider Failure Scenarios

#### 1. AI Provider Unavailable

**Location**: `internal/ai/application/service.go:257-264`

**Current Behavior**:
```go
results, err := s.provider.GenerateObservations(ctx, req)
if err != nil {
    s.recordAudit(ctx, params.OrganizationID, &params.ActorID, "ai.observation.failed", "evidence", shared.StrPtr(params.EvidenceID.String()), "failure", ...)
    return nil, fmt.Errorf("%w: %v", ErrAIProviderUnavailable, err)
}
```

**Finding**: PASS - Fail-fast with audit

- Failures are recorded in audit log
- Error returned to caller with context
- AI remains optional - service degrades gracefully

#### 2. AI Provider Timeout

**Finding**: PASS - 60 second timeout enforced

- Context with deadline from service layer
- HTTP client timeout is separate safety net
- Timeout errors trigger fail-fast behavior

---

## Stress Test Requirements

The following stress tests should be added to verify resilience:

### 1. Database Connection Storm
- Simulate 1000 concurrent connections
- Verify connections are properly released
- Check for connection leaks under load

### 2. Memory Growth Under Load
- Run 24-hour simulated load test
- Monitor memory usage of rate limiter and idempotency store
- Verify cleanup goroutines prevent unbounded growth

### 3. Full Failure Cycle Test
- Start service with database unavailable
- Verify startup completes without hanging
- Bring database online
- Verify service starts correctly

### 4. AI Provider Chaos
- Randomly return timeouts/errors from mock provider
- Verify observations are rejected cleanly
- Verify audit log captures all failures

---

## Recommendations - IMPLEMENTED

1. **Reduce database startup wait**: Change max retries from 30 to 10 (~60 second max) **DONE** in `internal/database/db.go:34`
2. **Add rate limiter max entries**: Implement LRU eviction at 10,000 entries **DONE**. Added `maxEntries` field with `DefaultMaxRateLimiterEntries = 10000`. Cleanup evicts oldest entries when over limit in `internal/middleware/ratelimit.go`
3. **Add idempotency store max entries**: Implement LRU eviction at 100,000 entries **DONE**. Added `maxEntries` field with `DefaultMaxIdempotencyEntries = 100000`. Cleanup evicts oldest entries when over limit in `internal/middleware/idempotency.go`
4. **Implement maintenance alerting**: Count consecutive failures and log `ALERT:` message after 3 failures **DONE** in `internal/audit/infrastructure/maintenance.go`
5. ~~Increase DB max open conns: Change from 25 to 100 for production workloads~~ Not modified (config-driven, not hardcoded)

---

## Vulnerability Findings - STATUS UPDATED

| ID | Severity | Finding | Status | Location |
|----|----------|---------|--------|----------|
| ST-09-001 | MEDIUM | Database startup wait time too long | **FIXED** | `internal/database/db.go:34` |
| ST-09-002 | INFO | Rate limiter memory unbounded | **FIXED** | `internal/middleware/ratelimit.go` |
| ST-09-003 | INFO | Idempotency store memory unbounded | **FIXED** | `internal/middleware/idempotency.go` |
| ST-09-004 | WARN | Maintenance errors silently logged | **FIXED** | `internal/audit/infrastructure/maintenance.go` |

---

## Conclusion

CIVORA's reliability mechanisms are now enhanced with production-ready safeguards:

- **Database startup**: Reduced from 30 to 10 retries (~60s max wait vs ~8min before)
- **Rate limiter**: LRU eviction at 10,000 entries prevents unbounded memory growth
- **Idempotency store**: LRU eviction at 100,000 entries prevents unbounded memory growth
- **Maintenance alerts**: After 3 consecutive failures, logs `ALERT:` message for detection by monitoring systems
- Transaction boundaries and rollbacks remain correct
- Race conditions are mitigated via database constraints and mutex protection
- Timeouts are properly configured at all layers
- Idempotency is supported at the middleware level
- Graceful shutdown handles process termination correctly
- AI failures are fail-fast with proper audit logging