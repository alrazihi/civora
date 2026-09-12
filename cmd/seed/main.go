package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

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
	defer func() {
		_ = db.Close()
	}()

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
				workflow_transition_history,
				workflow_instances,
				workflow_transitions,
				workflow_states,
				workflow_definitions,
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

	// CASE A — ACTIVE (at IN_REVIEW) with person, eligibility, evidence, assessment but NO final decision
	caseAID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, person_id, created_by, workflow_state, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),NOW())`,
		caseAID, orgID, "CAS-20260909-DEMO-ACT", "Emergency Food and Shelter Assistance", "Family of 4 displaced by flooding, needs immediate food and shelter support", "IN_REVIEW", "EMERGENCY", "URGENT", &personID, adminID, "IN_REVIEW")
	if err != nil {
		log.Fatalf("failed to create case A: %v", err)
	}

	// Case A: Eligibility (pending - requires more info)
	eligibilityAID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO eligibilities (id, organization_id, service_request_id, criteria, result, explanation, assessed_by, assessed_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		eligibilityAID, orgID, caseAID, `{"displaced":true,"income_verified":false,"household_size":4}`, "REQUIRES_MORE_INFORMATION", "Income verification pending; household size and displacement confirmed", staffID)
	if err != nil {
		log.Fatalf("failed to create eligibility for case A: %v", err)
	}

	// Case A: Evidence
	evidenceAID1 := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO evidence (id, organization_id, service_request_id, type, description, storage_reference, uploaded_by, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`,
		evidenceAID1, orgID, caseAID, "IDENTITY_DOCUMENT", "Government-issued ID for household head", "s3://civora-evidence/demo-act-id-001", adminID)
	if err != nil {
		log.Fatalf("failed to create evidence for case A: %v", err)
	}
	evidenceAID2 := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO evidence (id, organization_id, service_request_id, type, description, storage_reference, uploaded_by, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`,
		evidenceAID2, orgID, caseAID, "PROOF_OF_RESIDENCE", "Utility bill showing damaged residence address", "s3://civora-evidence/demo-act-res-001", adminID)
	if err != nil {
		log.Fatalf("failed to create evidence for case A: %v", err)
	}

	// Case A: Assessment (completed but no decision yet)
	assessmentAID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO assessments (id, organization_id, service_request_id, findings, needs_identified, recommendation, assessor, assessed_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		assessmentAID, orgID, caseAID, "Household of 4 displaced by flood. Identity and residence verified. Income documentation incomplete.", "Emergency food, temporary shelter, clothing, income verification", "Recommend proceeding to decision pending income verification", staffID)
	if err != nil {
		log.Fatalf("failed to create assessment for case A: %v", err)
	}

	// CASE B — COMPLETED (full lifecycle)
	caseBID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, person_id, created_by, workflow_state, created_at, updated_at, closed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),NOW(),NOW())`,
		caseBID, orgID, "CAS-20260909-DEMO-CMP", "Emergency Food and Shelter Assistance", "Family of 4 displaced by flooding, needs immediate food and shelter support", "CLOSED", "EMERGENCY", "URGENT", &personID, adminID, "CLOSED")
	if err != nil {
		log.Fatalf("failed to create case B: %v", err)
	}

	workflowDefID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO workflow_definitions (id, organization_id, key, name, description, version, status, initial_state, metadata, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())`,
		workflowDefID, orgID, "emergency_assistance", "Emergency Assistance", "Emergency assistance request workflow", 1, "ACTIVE", "NEW", `{}`)
	if err != nil {
		log.Fatalf("failed to create workflow definition: %v", err)
	}

	// Medical Assistance workflow definition (separate key, same generic infrastructure)
	medicalWorkflowDefID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO workflow_definitions (id, organization_id, key, name, description, version, status, initial_state, metadata, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())`,
		medicalWorkflowDefID, orgID, "medical_assistance", "Medical Assistance", "Medical assistance request workflow", 1, "ACTIVE", "NEW", `{}`)
	if err != nil {
		log.Fatalf("failed to create medical workflow definition: %v", err)
	}

	now := time.Now().UTC()
	stateIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	stateInsert := `INSERT INTO workflow_states (id, workflow_definition_id, organization_id, key, name, description, category, terminal, display_order, responsible_role, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	for i, stateKey := range []string{"NEW", "OPEN", "IN_REVIEW", "ASSESSMENT", "DECISION_PENDING", "APPROVED", "REJECTED", "IN_PROGRESS", "FOLLOW_UP", "CLOSED"} {
		terminal := "false"
		if stateKey == "REJECTED" || stateKey == "CLOSED" {
			terminal = "true"
		}
		_, err = db.DB.ExecContext(ctx, stateInsert, stateIDs[i], workflowDefID, orgID, stateKey, stateKey, "", "", terminal, i, "", now)
		if err != nil {
			log.Fatalf("failed to create workflow state %s: %v", stateKey, err)
		}
	}

	// Medical workflow states (same state keys, separate definition)
	medicalStateIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	for i, stateKey := range []string{"NEW", "OPEN", "IN_REVIEW", "ASSESSMENT", "DECISION_PENDING", "APPROVED", "REJECTED", "IN_PROGRESS", "FOLLOW_UP", "CLOSED"} {
		terminal := "false"
		if stateKey == "REJECTED" || stateKey == "CLOSED" {
			terminal = "true"
		}
		_, err = db.DB.ExecContext(ctx, stateInsert, medicalStateIDs[i], medicalWorkflowDefID, orgID, stateKey, stateKey, "", "", terminal, i, "", now)
		if err != nil {
			log.Fatalf("failed to create medical workflow state %s: %v", stateKey, err)
		}
	}

	transitionInsert := `INSERT INTO workflow_transitions (id, workflow_definition_id, organization_id, key, name, from_state, to_state, description, conditions, allowed_roles, active, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
	transitions := []struct {
		id            uuid.UUID
		key, from, to string
	}{
		{uuid.New(), "open", "NEW", "OPEN"},
		{uuid.New(), "review", "NEW", "IN_REVIEW"},
		{uuid.New(), "reopen", "IN_REVIEW", "OPEN"},
		{uuid.New(), "assess", "OPEN", "IN_REVIEW"},
		{uuid.New(), "assess2", "IN_REVIEW", "ASSESSMENT"},
		{uuid.New(), "decide", "ASSESSMENT", "DECISION_PENDING"},
		{uuid.New(), "approve", "DECISION_PENDING", "APPROVED"},
		{uuid.New(), "reject", "DECISION_PENDING", "REJECTED"},
		{uuid.New(), "start_assistance", "APPROVED", "IN_PROGRESS"},
		{uuid.New(), "close_rejected", "REJECTED", "CLOSED"},
		{uuid.New(), "follow_up", "IN_PROGRESS", "FOLLOW_UP"},
		{uuid.New(), "complete", "FOLLOW_UP", "CLOSED"},
	}
	for _, t := range transitions {
		_, err = db.DB.ExecContext(ctx, transitionInsert, t.id, workflowDefID, orgID, t.key, t.key, t.from, t.to, "", "[]", "[]", true, now)
		if err != nil {
			log.Fatalf("failed to create workflow transition %s: %v", t.key, err)
		}
	}

	// Medical workflow transitions (same pattern, separate definition)
	medicalTransitions := []struct {
		id            uuid.UUID
		key, from, to string
	}{
		{uuid.New(), "open", "NEW", "OPEN"},
		{uuid.New(), "review", "NEW", "IN_REVIEW"},
		{uuid.New(), "reopen", "IN_REVIEW", "OPEN"},
		{uuid.New(), "assess", "OPEN", "IN_REVIEW"},
		{uuid.New(), "assess2", "IN_REVIEW", "ASSESSMENT"},
		{uuid.New(), "decide", "ASSESSMENT", "DECISION_PENDING"},
		{uuid.New(), "approve", "DECISION_PENDING", "APPROVED"},
		{uuid.New(), "reject", "DECISION_PENDING", "REJECTED"},
		{uuid.New(), "start_assistance", "APPROVED", "IN_PROGRESS"},
		{uuid.New(), "close_rejected", "REJECTED", "CLOSED"},
		{uuid.New(), "follow_up", "IN_PROGRESS", "FOLLOW_UP"},
		{uuid.New(), "complete", "FOLLOW_UP", "CLOSED"},
	}
	for _, t := range medicalTransitions {
		_, err = db.DB.ExecContext(ctx, transitionInsert, t.id, medicalWorkflowDefID, orgID, t.key, t.key, t.from, t.to, "", "[]", "[]", true, now)
		if err != nil {
			log.Fatalf("failed to create medical workflow transition %s: %v", t.key, err)
		}
	}

	// CASE A — ACTIVE (at IN_REVIEW) with available actions
	instanceAID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO workflow_instances (id, organization_id, workflow_definition_id, workflow_definition_version, case_id, current_state, started_at, completed_at, metadata, version) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		instanceAID, orgID, workflowDefID, 1, caseAID, "IN_REVIEW", now, nil, `{}`, 1)
	if err != nil {
		log.Fatalf("failed to create workflow instance A: %v", err)
	}
	_, err = db.DB.ExecContext(ctx, `UPDATE cases SET workflow_instance_id = $1 WHERE id = $2`, instanceAID, caseAID)
	if err != nil {
		log.Fatalf("failed to link workflow instance A to case: %v", err)
	}

	// CASE A — ACTIVE (at IN_REVIEW):
	// Already linked to workflow instance A (IN_REVIEW).
	// Eligibility, evidence, and assessment demonstrate mid-workflow state.
	// No decision recorded yet.

	// CASE B — COMPLETED (full lifecycle)
	instanceBID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO workflow_instances (id, organization_id, workflow_definition_id, workflow_definition_version, case_id, current_state, started_at, completed_at, metadata, version) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		instanceBID, orgID, workflowDefID, 1, caseBID, "CLOSED", now, now, `{}`, 1)
	if err != nil {
		log.Fatalf("failed to create workflow instance B: %v", err)
	}
	_, err = db.DB.ExecContext(ctx, `UPDATE cases SET workflow_instance_id = $1 WHERE id = $2`, instanceBID, caseBID)
	if err != nil {
		log.Fatalf("failed to link workflow instance B to case: %v", err)
	}

	historyBInsert := `INSERT INTO workflow_transition_history (id, organization_id, workflow_instance_id, case_id, from_state, to_state, transition_key, actor_id, occurred_at, reason, metadata) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	historyBEntries := []struct {
		id            uuid.UUID
		from, to, key string
		actor         *uuid.UUID
	}{
		{uuid.New(), "NEW", "OPEN", "open", &adminID},
		{uuid.New(), "OPEN", "IN_REVIEW", "assess", &staffID},
		{uuid.New(), "IN_REVIEW", "ASSESSMENT", "assess2", &staffID},
		{uuid.New(), "ASSESSMENT", "DECISION_PENDING", "decide", &staffID},
		{uuid.New(), "DECISION_PENDING", "APPROVED", "approve", &adminID},
		{uuid.New(), "APPROVED", "IN_PROGRESS", "start_assistance", &staffID},
		{uuid.New(), "IN_PROGRESS", "FOLLOW_UP", "follow_up", &staffID},
		{uuid.New(), "FOLLOW_UP", "CLOSED", "complete", &adminID},
	}
	for _, h := range historyBEntries {
		_, err = db.DB.ExecContext(ctx, historyBInsert, h.id, orgID, instanceBID, caseBID, h.from, h.to, h.key, h.actor, now, "", `{}`)
		if err != nil {
			log.Fatalf("failed to create workflow transition history B: %v", err)
		}
	}

	eligibilityID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO eligibilities (id, organization_id, service_request_id, criteria, result, explanation, assessed_by, assessed_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		eligibilityID, orgID, caseBID, `{"displaced":true,"verified":true}`, "ELIGIBLE", "All criteria verified with documentation", adminID)
	if err != nil {
		log.Fatalf("failed to create eligibility: %v", err)
	}

	evidenceIDs := []uuid.UUID{uuid.New(), uuid.New()}
	_, err = db.DB.ExecContext(ctx, `INSERT INTO evidence (id, organization_id, service_request_id, type, description, storage_reference, uploaded_by, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`,
		evidenceIDs[0], orgID, caseBID, "IDENTITY_DOCUMENT", "Government-issued ID for household head", "s3://civora-evidence/demo-id-001", adminID)
	if err != nil {
		log.Fatalf("failed to create evidence: %v", err)
	}
	_, err = db.DB.ExecContext(ctx, `INSERT INTO evidence (id, organization_id, service_request_id, type, description, storage_reference, uploaded_by, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`,
		evidenceIDs[1], orgID, caseBID, "PROOF_OF_RESIDENCE", "Utility bill showing damaged residence", "s3://civora-evidence/demo-res-001", adminID)
	if err != nil {
		log.Fatalf("failed to create evidence: %v", err)
	}

	assessmentID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO assessments (id, organization_id, service_request_id, findings, needs_identified, recommendation, assessor, assessed_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		assessmentID, orgID, caseBID, "Household of 4 displaced by flood. Verified identity and residence documents. Income below threshold.", "Emergency food, temporary shelter, clothing", "Approve emergency shelter placement and food package", staffID)
	if err != nil {
		log.Fatalf("failed to create assessment: %v", err)
	}

	decisionID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO decisions (id, organization_id, service_request_id, decision, reason, decision_maker, decided_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())`,
		decisionID, orgID, caseBID, "APPROVED", "Meets all eligibility criteria. Assessment supports immediate shelter and food assistance.", adminID)
	if err != nil {
		log.Fatalf("failed to create decision: %v", err)
	}

	assistanceID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO assistance (id, organization_id, service_request_id, type, description, status, responsible_staff, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		assistanceID, orgID, caseBID, "SHELTER", "Emergency shelter placement at City Shelter Center for 30 days", "COMPLETED", staffID)
	if err != nil {
		log.Fatalf("failed to create assistance: %v", err)
	}

	followUpID := uuid.New()
	_, err = db.DB.ExecContext(ctx, `INSERT INTO follow_ups (id, organization_id, service_request_id, scheduled_date, outcome, notes, performed_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		followUpID, orgID, caseBID, "2026-10-15", "Family stably housed and receiving ongoing support", "Weekly check-ins scheduled with case manager", staffID)
	if err != nil {
		log.Fatalf("failed to create follow-up: %v", err)
	}

	fmt.Println("Demo seed completed successfully.")
	fmt.Printf("Organization: %s (slug: %s)\n", orgID, slug)
	fmt.Printf("Admin: admin@demo.org / demopass1234 (id: %s)\n", adminID)
	fmt.Printf("Staff: staff@demo.org / demopass1234 (id: %s)\n", staffID)
	fmt.Printf("Case A (ACTIVE): %s\n", caseAID)
	fmt.Printf("Case B (COMPLETED): %s\n", caseBID)
	fmt.Println("Use these credentials to log in at http://localhost:8080")
}
