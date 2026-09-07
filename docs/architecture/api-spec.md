# API Specification

This document describes the CIVORA API architecture and conventions.

The canonical API specification is maintained in `api/openapi/openapi.yaml`.

## Conventions

### Base URL

- Local development: `http://localhost:8080/api/v1`
- Production: configured via reverse proxy

### Authentication

All endpoints (except `/organizations POST`, `/auth/register`, `/auth/login`)
require a Bearer JWT token:

```
Authorization: Bearer <jwt-token>
```

Tokens are valid for 24 hours by default.

### Security

- **Tenant isolation**: Every authenticated request includes a JWT with an
  `organization_id` claim. The `RequireSameTenant` middleware verifies that
  the token's organization matches the `{orgId}` path parameter. Mismatches
  result in `403 Forbidden`.

- **Role-based access**: After authentication, the `RequireRole` (or
  `RequireAnyRole`) middleware checks the JWT's `role` claim against the
  requested resource. Insufficient privileges result in `403 Forbidden`.

- **Idempotency**: POST and PUT operations accept an optional
  `Idempotency-Key: <uuid>` header. When provided, duplicate requests with
  the same key return the cached response, guaranteeing at-most-once
  execution.

- **Correlation IDs**: Every request receives an `X-Request-ID` header
  (generated if not provided by the client). This ID is included in all
  audit events and structured log entries.

### Response Format

All responses follow a standardized envelope:

```json
{
  "success": true,
  "data": { ... },
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

Errors:

```json
{
  "success": false,
  "error": {
    "code": "INVALID_INPUT",
    "message": "title is required"
  }
}
```

### Pagination

- Query parameters: `page` (default: 1), `per_page` (default: 20, max: 200)
- Response header: `X-Request-ID` (correlation ID)
- Total count and total pages included in `meta`.

### Filtering

Supported query parameters:
- `status` on `/cases` (enum: CREATED, OPEN, IN_REVIEW, RESOLVED, CLOSED)

## Endpoints

### System

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | No | Health check |
| GET | `/ready` | No | Readiness check |

### Organizations

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/organizations` | No | Create organization |
| GET | `/organizations/{orgId}` | Yes | Get organization |

### Authentication

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/organizations/{orgId}/auth/register` | No | Register user |
| POST | `/organizations/{orgId}/auth/login` | No | Authenticate, get JWT |

### Users

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/organizations/{orgId}/users` | Yes | List users |
| GET | `/organizations/{orgId}/users/{userId}` | Yes | Get user |

### Cases

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/organizations/{orgId}/cases` | Yes | Create case |
| GET | `/organizations/{orgId}/cases` | Yes | List cases (filter: status) |
| GET | `/organizations/{orgId}/cases/{caseId}` | Yes | Get case |
| POST | `/organizations/{orgId}/cases/{caseId}/transitions` | Yes | Change status |
| POST | `/organizations/{orgId}/cases/{caseId}/assign` | Yes | Assign case |

### Audit

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/organizations/{orgId}/audit` | Yes | List audit events |
