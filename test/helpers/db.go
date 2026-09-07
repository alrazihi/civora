package helpers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/alrazihi/civora/internal/database"
	"github.com/alrazihi/civora/migrations"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestDB(t *testing.T) *sql.DB {
	t.Helper()

	host := getEnv("CIVORA_TEST_DB_HOST", "localhost")
	port := getEnv("CIVORA_TEST_DB_PORT", "5432")
	user := getEnv("CIVORA_TEST_DB_USER", "civora_test")
	password := getEnv("CIVORA_TEST_DB_PASSWORD", "civora_test")
	dbname := getEnv("CIVORA_TEST_DB_NAME", "civora_test")
	sslmode := getEnv("CIVORA_TEST_DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	db.SetMaxOpenConns(1)

	if err := ensureSchema(db); err != nil {
		t.Fatalf("failed to ensure schema: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func ensureSchema(db *sql.DB) error {
	migrator := database.NewMigrator(db, migrations.FS)
	if err := migrator.LoadMigrations(); err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}
	if err := migrator.Migrate(context.Background()); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

func TruncateTables(t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.Exec(`
		TRUNCATE TABLE
			follow_ups,
			assistance,
			decisions,
			assessments,
			evidence,
			eligibilities,
			people,
			audit_events,
			cases,
			users,
			roles,
			organizations
		RESTART IDENTITY CASCADE;
	`)
	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}

func ResetDB(t *testing.T, db *sql.DB) {
	t.Helper()
	TruncateTables(t, db)
}

func SeedOrg(db *sql.DB) uuid.UUID {
	orgID := uuid.New()
	slug := "test-org-" + uuid.NewString()[:8]
	_, err := db.Exec(
		"INSERT INTO organizations (id, name, description, slug, created_at, updated_at) VALUES ($1, $2, $3, $4, NOW(), NOW())",
		orgID, "Test Org", "", slug,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to seed organization: %v", err))
	}
	return orgID
}

func SeedUser(db *sql.DB, orgID uuid.UUID) uuid.UUID {
	userID := uuid.New()
	_, err := db.Exec(
		"INSERT INTO users (id, organization_id, email, name, role_id, password_hash, created_at, updated_at) VALUES ($1, $2, $3, $4, NULL, NULL, NOW(), NOW())",
		userID, orgID, "user-"+uuid.NewString()[:8]+"@example.com", "Test User",
	)
	if err != nil {
		panic(fmt.Sprintf("failed to seed user: %v", err))
	}
	return userID
}

func SeedPerson(db *sql.DB, orgID uuid.UUID) uuid.UUID {
	personID := uuid.New()
	_, err := db.Exec(
		"INSERT INTO people (id, organization_id, first_name, last_name, preferred_language, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())",
		personID, orgID, "Jane", "Doe", "en", "ACTIVE",
	)
	if err != nil {
		panic(fmt.Sprintf("failed to seed person: %v", err))
	}
	return personID
}

func SeedDefaultRoles(db *sql.DB, orgID uuid.UUID) {
	roles := []struct {
		name        string
		description string
		permissions string
	}{
		{"admin", "Full access to all organization resources", `["*"]`},
		{"staff", "Standard user with case and customer access", `["cases:*"]`},
	}
	for _, r := range roles {
		_, err := db.Exec(
			"INSERT INTO roles (id, organization_id, name, description, permissions, created_at) VALUES ($1, $2, $3, $4, $5, NOW())",
			uuid.New(), orgID, r.name, r.description, r.permissions,
		)
		if err != nil {
			panic(fmt.Sprintf("failed to seed role %s: %v", r.name, err))
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
