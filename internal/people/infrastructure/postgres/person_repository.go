package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alrazihi/civora/internal/people/domain"
	"github.com/google/uuid"
)

type PostgresPersonRepository struct {
	db *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewPostgresPersonRepository(db *sql.DB) *PostgresPersonRepository {
	return &PostgresPersonRepository{db: db}
}

func (r *PostgresPersonRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresPersonRepository) Save(ctx context.Context, p *domain.Person) error {
	return r.savePerson(ctx, r.db, p)
}

func (r *PostgresPersonRepository) SaveTx(ctx context.Context, tx *sql.Tx, p *domain.Person) error {
	return r.savePerson(ctx, tx, p)
}

func (r *PostgresPersonRepository) savePerson(ctx context.Context, e sqlExecer, p *domain.Person) error {
	query := `
		INSERT INTO people (
			id, organization_id, external_reference, first_name, last_name,
			date_of_birth, email, phone, address, city, preferred_language,
			status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := e.ExecContext(ctx, query,
		p.ID, p.OrganizationID, p.ExternalReference, p.FirstName, p.LastName,
		p.DateOfBirth, p.Email, p.Phone, p.Address, p.City, p.PreferredLanguage,
		p.Status, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert person: %w", err)
	}
	return nil
}

func (r *PostgresPersonRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Person, error) {
	query := `
		SELECT id, organization_id, external_reference, first_name, last_name,
			   date_of_birth, email, phone, address, city, preferred_language,
			   status, created_at, updated_at
		FROM people
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanPerson(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresPersonRepository) FindByExternalReference(ctx context.Context, orgID uuid.UUID, ref string) (*domain.Person, error) {
	query := `
		SELECT id, organization_id, external_reference, first_name, last_name,
			   date_of_birth, email, phone, address, city, preferred_language,
			   status, created_at, updated_at
		FROM people
		WHERE organization_id = $1 AND external_reference = $2
	`
	return r.scanPerson(r.db.QueryRowContext(ctx, query, orgID, ref))
}

func (r *PostgresPersonRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Person, error) {
	query := `
		SELECT id, organization_id, external_reference, first_name, last_name,
			   date_of_birth, email, phone, address, city, preferred_language,
			   status, created_at, updated_at
		FROM people
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query persons: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var persons []*domain.Person
	for rows.Next() {
		p, err := r.scanPersonFromRows(rows)
		if err != nil {
			return nil, err
		}
		persons = append(persons, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return persons, nil
}

func (r *PostgresPersonRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM people WHERE organization_id = $1`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count persons: %w", err)
	}
	return total, nil
}

func (r *PostgresPersonRepository) Update(ctx context.Context, p *domain.Person) error {
	return r.updatePerson(ctx, r.db, p)
}

func (r *PostgresPersonRepository) UpdateTx(ctx context.Context, tx *sql.Tx, p *domain.Person) error {
	return r.updatePerson(ctx, tx, p)
}

func (r *PostgresPersonRepository) updatePerson(ctx context.Context, e sqlExecer, p *domain.Person) error {
	query := `
		UPDATE people
		SET status = $1, updated_at = $2
		WHERE organization_id = $3 AND id = $4
	`
	result, err := e.ExecContext(ctx, query, p.Status, p.UpdatedAt, p.OrganizationID, p.ID)
	if err != nil {
		return fmt.Errorf("failed to update person: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("no rows affected")
	}
	return nil
}

func (r *PostgresPersonRepository) scanPerson(row interface {
	Scan(dest ...any) error
}) (*domain.Person, error) {
	var p domain.Person
	if err := row.Scan(
		&p.ID, &p.OrganizationID, &p.ExternalReference, &p.FirstName, &p.LastName,
		&p.DateOfBirth, &p.Email, &p.Phone, &p.Address, &p.City, &p.PreferredLanguage,
		&p.Status, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan person: %w", err)
	}
	return &p, nil
}

func (r *PostgresPersonRepository) scanPersonFromRows(rows *sql.Rows) (*domain.Person, error) {
	var p domain.Person
	if err := rows.Scan(
		&p.ID, &p.OrganizationID, &p.ExternalReference, &p.FirstName, &p.LastName,
		&p.DateOfBirth, &p.Email, &p.Phone, &p.Address, &p.City, &p.PreferredLanguage,
		&p.Status, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan person: %w", err)
	}
	return &p, nil
}
