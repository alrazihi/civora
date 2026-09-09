package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/alrazihi/civora/internal/config"
	"github.com/alrazihi/civora/internal/database"
	"github.com/alrazihi/civora/migrations"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	dsn := database.BuildDSN(cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)
	db, err := database.NewDatabase(dsn, cfg.Database.Driver)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	migrator := database.NewMigrator(db.DB, migrations.FS)
	if err := migrator.LoadMigrations(); err != nil {
		log.Fatalf("failed to load migrations: %v", err)
	}
	if err := migrator.Migrate(context.Background()); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	reset := flag.Bool("reset", false, "truncate all data before seeding")
	flag.Parse()

	if *reset {
		if _, err := db.DB.Exec(`
			TRUNCATE TABLE
				follow_ups, assistance, decisions, assessments,
				evidence, eligibilities, people,
				audit.audit_events, cases, users, roles, organizations
			RESTART IDENTITY CASCADE;
		`); err != nil {
			log.Fatalf("failed to reset database: %v", err)
		}
		fmt.Println("Database reset.")
	}

	ctx := context.Background()
	orgID := uuid.New()
	slug := "emergency-demo"
	var existingSlug string
	err = db.DB.QueryRowContext(ctx, "SELECT slug FROM organizations WHERE slug = $1", slug).Scan(&existingSlug)
	if err == nil {
		fmt.Printf("Demo organization already exists (slug=%s). Skipping seed.\n", slug)
		return
	}

	_, err = db.DB.ExecContext(ctx, `INSERT INTO organizations (id, name, description, slug, created_at, updated_at) VALUES ($1,$2,$3,$4,NOW(),NOW())`,
		orgID, "Emergency Response Demo", "Demo organization for emergency assistance workflow", slug)
	if err != nil {
		log.Fatalf("failed to create organization: %v", err)
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("demopass1234"), 4)

	adminRoleID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO roles (id, organization_id, name, description, permissions, created_at) VALUES ($1,$2,$3,$4,$5,NOW())`,
		adminRoleID, orgID, "admin", "Full access", `["*"]`)
	if err != nil {
		log.Fatalf("failed to create admin role: %v", err)
	}

	staffRoleID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO roles (id, organization_id, name, description, permissions, created_at) VALUES ($1,$2,$3,$4,$5,NOW())`,
		staffRoleID, orgID, "staff", "Standard access", `["cases:*"]`)
	if err != nil {
		log.Fatalf("failed to create staff role: %v", err)
	}

	adminID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO users (id, organization_id, email, name, role_id, password_hash, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())`,
		adminID, orgID, "admin@demo.org", "Demo Admin", adminRoleID, string(hashedPassword))
	if err != nil {
		log.Fatalf("failed to create admin user: %v", err)
	}

	staffID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO users (id, organization_id, email, name, role_id, password_hash, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())`,
		staffID, orgID, "staff@demo.org", "Demo Staff", staffRoleID, string(hashedPassword))
	if err != nil {
		log.Fatalf("failed to create staff user: %v", err)
	}

	personID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO people (id, organization_id, first_name, last_name, preferred_language, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())`,
		personID, orgID, "Amina", "Hassan", "en", "ACTIVE")
	if err != nil {
		log.Fatalf("failed to create person: %v", err)
	}

	caseID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, person_id, created_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())`,
		caseID, orgID, "CAS-20260909-DEMO001", "Emergency Food and Shelter Assistance", "Family of 4 displaced by flooding, needs immediate food and shelter support", "NEW", "EMERGENCY", "URGENT", &personID, adminID)
	if err != nil {
		log.Fatalf("failed to create case: %v", err)
	}

	eligibilityID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO eligibilities (id, organization_id, service_request_id, criteria, result, explanation, assessed_by, assessed_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		eligibilityID, orgID, caseID, `{"displaced":true,"verified":true}`, "REQUIRES_MORE_INFORMATION", "Initial assessment pending full documentation", adminID)
	if err != nil {
		log.Fatalf("failed to create eligibility: %v", err)
	}

	evidenceIDs := []uuid.UUID{uuid.New(), uuid.New()}
	_, err = db.DB.ExecContext(ctx, `INSERT INTO evidence (id, organization_id, service_request_id, type, description, storage_reference, uploaded_by, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`,
		evidenceIDs[0], orgID, caseID, "IDENTITY_DOCUMENT", "Government-issued ID for household head", "s3://civora-evidence/demo-id-001", adminID)
	if err != nil {
		log.Fatalf("failed to create evidence: %v", err)
	}
	_, err = db.DB.ExecContext(ctx, `INSERT INTO evidence (id, organization_id, service_request_id, type, description, storage_reference, uploaded_by, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`,
		evidenceIDs[1], orgID, caseID, "PROOF_OF_RESIDENCE", "Utility bill showing damaged residence", "s3://civora-evidence/demo-res-001", adminID)
	if err != nil {
		log.Fatalf("failed to create evidence: %v", err)
	}

	assessmentID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO assessments (id, organization_id, service_request_id, findings, needs_identified, recommendation, assessor, assessed_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		assessmentID, orgID, caseID, "Household of 4 displaced by flood. Verified identity and residence documents. Income below threshold.", "Emergency food, temporary shelter, clothing", "Approve emergency shelter placement and food package", staffID)
	if err != nil {
		log.Fatalf("failed to create assessment: %v", err)
	}

	decisionID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO decisions (id, organization_id, service_request_id, decision, reason, decision_maker, decided_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())`,
		decisionID, orgID, caseID, "APPROVED", "Meets all eligibility criteria. Assessment supports immediate shelter and food assistance.", adminID)
	if err != nil {
		log.Fatalf("failed to create decision: %v", err)
	}

	assistanceID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO assistance (id, organization_id, service_request_id, type, description, status, responsible_staff, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		assistanceID, orgID, caseID, "SHELTER", "Emergency shelter placement at City Shelter Center for 30 days", "PLANNED", staffID)
	if err != nil {
		log.Fatalf("failed to create assistance: %v", err)
	}

	followUpID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO follow_ups (id, organization_id, service_request_id, scheduled_date, outcome, notes, performed_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		followUpID, orgID, caseID, "2026-10-15", "Family stably housed and receiving ongoing support", "Weekly check-ins scheduled with case manager", staffID)
	if err != nil {
		log.Fatalf("failed to create follow-up: %v", err)
	}

	fmt.Println("Demo seed completed successfully.")
	fmt.Printf("Organization: %s (slug: %s)\n", orgID, slug)
	fmt.Printf("Admin: admin@demo.org / demopass1234\n")
	fmt.Printf("Staff: staff@demo.org / demopass1234\n")
	fmt.Printf("Case ID: %s\n", caseID)
	fmt.Println("Use these credentials to log in at http://localhost:8080")
}
