package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	intmid "github.com/alrazihi/civora/internal/middleware"
	peopledomain "github.com/alrazihi/civora/internal/people/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrPersonNotFound        = errors.New("person not found")
	ErrPersonInvalidInput    = errors.New("invalid input")
	ErrPersonTenantViolation = errors.New("person does not belong to organization")
)

type PersonService struct {
	repo    peopledomain.PersonRepository
	auditor auditdomain.EventRecorder
}

func NewPersonService(repo peopledomain.PersonRepository, auditor auditdomain.EventRecorder) *PersonService {
	return &PersonService{
		repo:    repo,
		auditor: auditor,
	}
}

type CreatePersonParams struct {
	OrganizationID    uuid.UUID
	ExternalReference *string
	FirstName         string
	LastName          string
	DateOfBirth       *time.Time
	Email             *string
	Phone             *string
	Address           *string
	City              *string
	PreferredLanguage string
	ActorID           uuid.UUID
}

func (s *PersonService) CreatePerson(ctx context.Context, params CreatePersonParams) (*peopledomain.Person, error) {
	p, err := peopledomain.NewPerson(params.OrganizationID, params.FirstName, params.LastName, params.PreferredLanguage)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPersonInvalidInput, err)
	}

	p.ExternalReference = params.ExternalReference
	p.DateOfBirth = params.DateOfBirth
	p.Email = params.Email
	p.Phone = params.Phone
	p.Address = params.Address
	p.City = params.City

	var result *peopledomain.Person
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, p); err != nil {
			return fmt.Errorf("failed to save person: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: p.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "person.created",
				Resource:       "person",
				ResourceID:     shared.StrPtr(p.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = p
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *PersonService) GetPerson(ctx context.Context, orgID, id uuid.UUID) (*peopledomain.Person, error) {
	p, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrPersonNotFound
	}
	return p, nil
}

func (s *PersonService) ListPersons(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*peopledomain.Person, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.CountByOrganization(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count persons: %w", err)
	}

	persons, err := s.repo.FindByOrganization(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list persons: %w", err)
	}

	return persons, total, nil
}

func (s *PersonService) FindPersonByExternalReference(ctx context.Context, orgID uuid.UUID, ref string) (*peopledomain.Person, error) {
	p, err := s.repo.FindByExternalReference(ctx, orgID, ref)
	if err != nil {
		return nil, ErrPersonNotFound
	}
	return p, nil
}

func (s *PersonService) UpdatePersonStatus(ctx context.Context, orgID, id uuid.UUID, status peopledomain.PersonStatus, actorID uuid.UUID) (*peopledomain.Person, error) {
	p, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrPersonNotFound
	}

	p.UpdateStatus(status)

	var result *peopledomain.Person
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateTx(ctx, tx, p); err != nil {
			return fmt.Errorf("failed to update person: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: p.OrganizationID,
				ActorID:        &actorID,
				Action:         "person.status_updated",
				Resource:       "person",
				ResourceID:     shared.StrPtr(p.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"new_status": string(status),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = p
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}
