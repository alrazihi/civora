package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/alrazihi/civora/internal/forms/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

// formKeyResolverAdapter adapts the forms module's read-only repository
// into the shared.FormKeyResolver contract consumed by the rules engine.
// This preserves module boundaries: the rules module never imports the
// forms infrastructure package directly.
type formKeyResolverAdapter struct {
	formRepo domain.FormRepository
}

func NewFormKeyResolverAdapter(formRepo domain.FormRepository) shared.FormKeyResolver {
	return &formKeyResolverAdapter{formRepo: formRepo}
}

func (a *formKeyResolverAdapter) FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*shared.FormView, error) {
	form, err := a.formRepo.FindByKey(ctx, orgID, key)
	if err != nil {
		if errors.Is(err, domain.ErrFormNotFound) {
			return nil, shared.NewError(shared.CodeNotFound, "form key not found", err)
		}
		return nil, fmt.Errorf("failed to resolve form key %s: %w", key, err)
	}
	return a.toFormView(form), nil
}

func (a *formKeyResolverAdapter) FindByID(ctx context.Context, orgID, formID uuid.UUID) (*shared.FormView, error) {
	return a.findByID(ctx, nil, orgID, formID)
}

func (a *formKeyResolverAdapter) FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, formID uuid.UUID) (*shared.FormView, error) {
	return a.findByID(ctx, tx, orgID, formID)
}

func (a *formKeyResolverAdapter) findByID(ctx context.Context, tx *sql.Tx, orgID, formID uuid.UUID) (*shared.FormView, error) {
	form, err := a.formRepo.FindByIDTx(ctx, tx, orgID, formID)
	if err != nil {
		if errors.Is(err, domain.ErrFormNotFound) {
			return nil, shared.NewError(shared.CodeNotFound, "form not found", err)
		}
		return nil, fmt.Errorf("failed to resolve form: %w", err)
	}
	return a.toFormView(form), nil
}

func (a *formKeyResolverAdapter) toFormView(form *domain.Form) *shared.FormView {
	return &shared.FormView{
		ID:             form.ID,
		Key:            form.Key,
		Name:           form.Name,
		OrganizationID: form.OrganizationID,
	}
}
