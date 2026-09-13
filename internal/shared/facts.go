package shared

import (
	"context"

	submissiondomain "github.com/alrazihi/civora/internal/form_submission/domain"
	"github.com/google/uuid"
)

// FormSubmissionFinder resolves form submissions used as fact sources.
// Implementations live in the form_submission module; the rules engine consumes
// them read-only via this interface to preserve module boundaries.
type FormSubmissionFinder interface {
	// FindLatestByFormAndCase returns the most recent submission for the given
	// form key within an organization and case.
	FindLatestByFormAndCase(ctx context.Context, orgID, caseID uuid.UUID, formKey string) (*submissiondomain.FormSubmission, error)
}

// SubmissionLister lists all submissions for a case.
type SubmissionLister interface {
	ListByCase(ctx context.Context, orgID, caseID uuid.UUID) ([]*submissiondomain.FormSubmission, error)
}

// FormKeyResolver maps form keys (the stable identifier rules reference) to
// resolved form metadata.
type FormKeyResolver interface {
	FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*FormView, error)
}

// FormView is a read-only projection of a form used for fact resolution.
type FormView struct {
	ID             uuid.UUID
	Key            string
	Name           string
	OrganizationID uuid.UUID
}
