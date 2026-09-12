package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/audit/domain"
	"github.com/google/uuid"
)

type PostgresAuditRepository struct {
	db *sql.DB
}

func NewPostgresAuditRepository(db *sql.DB) *PostgresAuditRepository {
	return &PostgresAuditRepository{db: db}
}

func (r *PostgresAuditRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresAuditRepository) RecordEvent(ctx context.Context, orgID uuid.UUID, event *domain.AuditEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(); rbErr != nil {
				// Rollback after Commit returns sql.ErrTxDone; a genuine
				// rollback failure is logged but not surfaced because the
				// audit write has already failed.
				_ = rbErr
			}
		}
	}()

	if err := r.writeEvent(ctx, tx, orgID, event); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit audit transaction: %w", err)
	}
	committed = true
	return nil
}

func (r *PostgresAuditRepository) RecordEventTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, event *domain.AuditEvent) error {
	return r.writeEvent(ctx, tx, orgID, event)
}

func (r *PostgresAuditRepository) writeEvent(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, event *domain.AuditEvent) error {
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, orgID.String()); err != nil {
		return fmt.Errorf("failed to lock audit chain: %w", err)
	}

	var lastHash *string
	err := tx.QueryRowContext(ctx, `
		SELECT hash FROM audit.audit_events
		WHERE organization_id = $1
		ORDER BY timestamp DESC, id DESC
		LIMIT 1
	`, orgID).Scan(&lastHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to query last hash: %w", err)
	}

	event.PreviousHash = lastHash
	hash, err := event.ComputeHash()
	if err != nil {
		return fmt.Errorf("failed to compute audit hash: %w", err)
	}
	event.Hash = hash

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO audit.audit_events (
			id, organization_id, actor_id, action, resource,
			resource_id, outcome, request_id, metadata,
			timestamp, previous_hash, hash
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err = tx.ExecContext(
		ctx, query,
		event.ID,
		event.OrganizationID,
		event.ActorID,
		event.Action,
		event.Resource,
		event.ResourceID,
		event.Outcome,
		event.RequestID,
		metadataJSON,
		event.Timestamp,
		event.PreviousHash,
		event.Hash,
	)
	if err != nil {
		return fmt.Errorf("failed to insert audit event: %w", err)
	}
	return nil
}

func (r *PostgresAuditRepository) Save(ctx context.Context, event *domain.AuditEvent) error {
	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO audit.audit_events (
			id, organization_id, actor_id, action, resource,
			resource_id, outcome, request_id, metadata,
			timestamp, previous_hash, hash
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err = r.db.ExecContext(
		ctx, query,
		event.ID,
		event.OrganizationID,
		event.ActorID,
		event.Action,
		event.Resource,
		event.ResourceID,
		event.Outcome,
		event.RequestID,
		metadataJSON,
		event.Timestamp,
		event.PreviousHash,
		event.Hash,
	)
	if err != nil {
		return fmt.Errorf("failed to insert audit event: %w", err)
	}
	return nil
}

func (r *PostgresAuditRepository) GetLastHash(ctx context.Context, orgID uuid.UUID) (*string, error) {
	var lastHash *string
	query := `
		SELECT hash FROM audit.audit_events
		WHERE organization_id = $1
		ORDER BY timestamp DESC, id DESC
		LIMIT 1
	`
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&lastHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query last hash: %w", err)
	}
	return lastHash, nil
}

func (r *PostgresAuditRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.AuditEvent, error) {
	query := `
SELECT id, organization_id, actor_id, action, resource,
		   resource_id, outcome, request_id, metadata,
		   timestamp, previous_hash, hash
		FROM audit.audit_events
		WHERE organization_id = $1
		ORDER BY timestamp DESC, id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit events: %w", err)
	}
	defer rows.Close()

	var events []*domain.AuditEvent
	for rows.Next() {
		var ev domain.AuditEvent
		var metadataJSON []byte

		if err := rows.Scan(
			&ev.ID,
			&ev.OrganizationID,
			&ev.ActorID,
			&ev.Action,
			&ev.Resource,
			&ev.ResourceID,
			&ev.Outcome,
			&ev.RequestID,
			&metadataJSON,
			&ev.Timestamp,
			&ev.PreviousHash,
			&ev.Hash,
		); err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &ev.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal audit metadata: %w", err)
			}
		}
		events = append(events, &ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return events, nil
}

func (r *PostgresAuditRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	var total int
	query := `SELECT COUNT(*) FROM audit.audit_events WHERE organization_id = $1`
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit events: %w", err)
	}
	return total, nil
}

func (r *PostgresAuditRepository) FindByResource(ctx context.Context, orgID uuid.UUID, resourceID string) ([]*domain.AuditEvent, error) {
	query := `
SELECT id, organization_id, actor_id, action, resource,
		   resource_id, outcome, request_id, metadata,
		   timestamp, previous_hash, hash
		FROM audit.audit_events
		WHERE organization_id = $1 AND resource_id = $2
		ORDER BY timestamp ASC, id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, resourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit events by resource: %w", err)
	}
	defer rows.Close()

	var events []*domain.AuditEvent
	for rows.Next() {
		var ev domain.AuditEvent
		var metadataJSON []byte

		if err := rows.Scan(
			&ev.ID,
			&ev.OrganizationID,
			&ev.ActorID,
			&ev.Action,
			&ev.Resource,
			&ev.ResourceID,
			&ev.Outcome,
			&ev.RequestID,
			&metadataJSON,
			&ev.Timestamp,
			&ev.PreviousHash,
			&ev.Hash,
		); err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &ev.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal audit metadata: %w", err)
			}
		}
		events = append(events, &ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return events, nil
}

// AllOrganizationIDs returns every distinct organization_id that has at
// least one audit event. This is used by the audit maintenance service to
// run periodic integrity verification over all tenants.
func (r *PostgresAuditRepository) AllOrganizationIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT organization_id FROM audit.audit_events ORDER BY organization_id`)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit organization ids: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan audit organization id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return ids, nil
}

func (r *PostgresAuditRepository) PurgeOld(ctx context.Context, olderThan time.Time) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin audit purge transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT organization_id
		FROM audit.audit_events
		WHERE timestamp < $1
		ORDER BY organization_id
	`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to list audit organizations for purge: %w", err)
	}
	var orgIDs []uuid.UUID
	for rows.Next() {
		var orgID uuid.UUID
		if err := rows.Scan(&orgID); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("failed to scan audit organization: %w", err)
		}
		orgIDs = append(orgIDs, orgID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("failed to iterate audit organizations: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("failed to close audit organization query: %w", err)
	}

	var total int64
	for _, orgID := range orgIDs {
		if _, err := tx.ExecContext(ctx, `
			SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))
		`, orgID.String()); err != nil {
			return 0, fmt.Errorf("failed to lock audit chain: %w", err)
		}

		result, err := tx.ExecContext(ctx, `
			DELETE FROM audit.audit_events
			WHERE organization_id = $1 AND timestamp < $2
		`, orgID, olderThan)
		if err != nil {
			return 0, fmt.Errorf("failed to purge old audit events: %w", err)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("failed to get rows affected: %w", err)
		}
		total += n

		if err := r.rebuildAuditChain(ctx, tx, orgID); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit audit purge transaction: %w", err)
	}
	committed = true
	return int(total), nil
}

func (r *PostgresAuditRepository) rebuildAuditChain(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, organization_id, actor_id, action, resource, resource_id,
		       outcome, request_id, metadata, timestamp, previous_hash, hash
		FROM audit.audit_events
		WHERE organization_id = $1
		ORDER BY timestamp ASC, id ASC
		FOR UPDATE
	`, orgID)
	if err != nil {
		return fmt.Errorf("failed to read audit chain: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var previousHash *string
	for rows.Next() {
		var event domain.AuditEvent
		var metadataJSON []byte
		if err := rows.Scan(
			&event.ID,
			&event.OrganizationID,
			&event.ActorID,
			&event.Action,
			&event.Resource,
			&event.ResourceID,
			&event.Outcome,
			&event.RequestID,
			&metadataJSON,
			&event.Timestamp,
			&event.PreviousHash,
			&event.Hash,
		); err != nil {
			return fmt.Errorf("failed to scan audit chain: %w", err)
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
				return fmt.Errorf("failed to unmarshal audit metadata: %w", err)
			}
		}

		event.PreviousHash = previousHash
		hash, err := event.ComputeHash()
		if err != nil {
			return fmt.Errorf("failed to recompute audit hash: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE audit.audit_events
			SET previous_hash = $1, hash = $2
			WHERE id = $3
		`, event.PreviousHash, hash, event.ID); err != nil {
			return fmt.Errorf("failed to update audit chain: %w", err)
		}
		hashValue := hash
		previousHash = &hashValue
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate audit chain: %w", err)
	}
	return nil
}
