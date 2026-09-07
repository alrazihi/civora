# Configuration

CIVORA is configured entirely through environment variables. All variables
can be set directly in the environment or in a `.env` file (loaded
automatically from the project root).

## Environment Variables

### Server

| Variable | Default | Description |
|---|---|---|
| `CIVORA_SERVER_PORT` | `8080` | HTTP server port |
| `CIVORA_SERVER_READ_TIMEOUT` | `30s` | Read timeout |
| `CIVORA_SERVER_WRITE_TIMEOUT` | `30s` | Write timeout |
| `CIVORA_SERVER_IDLE_TIMEOUT` | `120s` | Idle timeout |

### Database

| Variable | Default | Description |
|---|---|---|
| `CIVORA_DB_DRIVER` | `pgx` | Database driver |
| `CIVORA_DB_HOST` | `localhost` | Database host |
| `CIVORA_DB_PORT` | `5432` | Database port |
| `CIVORA_DB_USER` | `civora` | Database user |
| `CIVORA_DB_PASSWORD` | `civora` | Database password |
| `CIVORA_DB_NAME` | `civora` | Database name |
| `CIVORA_DB_SSLMODE` | `disable` | SSL mode |

### Authentication

| Variable | Default | Description |
|---|---|---|
| `CIVORA_AUTH_JWT_SECRET` | `dev-secret-change-me` | JWT signing secret (min 32 chars in production) |
| `CIVORA_AUTH_JWT_EXPIRY` | `24h` | JWT token expiry |
| `CIVORA_AUTH_BCRYPT_COST` | `12` | BCrypt hashing cost |
| `CIVORA_AUTH_OIDC_ISSUER` | `""` | OIDC issuer URL (empty = local auth only) |
| `CIVORA_AUTH_OIDC_CLIENT_ID` | `""` | OIDC client ID |
| `CIVORA_AUTH_OIDC_REDIRECT_URL` | `""` | OIDC callback URL |

### Audit

| Variable | Default | Description |
|---|---|---|
| `CIVORA_AUDIT_ENABLED` | `true` | Enable audit logging |
| `CIVORA_AUDIT_HASH_CHAIN` | `true` | Enable hash chain for tamper evidence |
| `CIVORA_AUDIT_RETENTION_DAYS` | `2555` | Audit event retention (7 years) |

### Environment

| Variable | Default | Description |
|---|---|---|
| `CIVORA_ENV` | `""` | Set to `production` for strict validation |

## Security Notes

- **Never commit `.env`** — it is in `.gitignore`
- **Production**: Set `CIVORA_ENV=production` and provide a strong
  `CIVORA_AUTH_JWT_SECRET` (32+ characters)
- **Rate limiting**: 100 requests/second per IP with burst of 20
- **Secrets**: All secrets are read from environment variables at startup

## Local Development

```bash
cp .env.example .env
docker compose up -d
go run ./cmd/civora
```
