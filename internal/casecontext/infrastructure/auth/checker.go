package auth

import (
	"context"

	casecontextdomain "github.com/alrazihi/civora/internal/casecontext/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

type AuthorizationChecker struct {
	userChecker shared.UserChecker
}

func NewAuthorizationChecker(userChecker shared.UserChecker) *AuthorizationChecker {
	return &AuthorizationChecker{userChecker: userChecker}
}

func (c *AuthorizationChecker) CanAccessCase(ctx context.Context, orgID, caseID, actorID uuid.UUID) (bool, error) {
	return c.userChecker.BelongsToOrganization(ctx, orgID, actorID)
}

func (c *AuthorizationChecker) CanAccessPerson(ctx context.Context, orgID, personID, actorID uuid.UUID) (bool, error) {
	return c.userChecker.BelongsToOrganization(ctx, orgID, actorID)
}

func (c *AuthorizationChecker) CanAccessEvidence(ctx context.Context, orgID, evidenceID, actorID uuid.UUID) (bool, error) {
	return c.userChecker.BelongsToOrganization(ctx, orgID, actorID)
}

func (c *AuthorizationChecker) CanAccessDocument(ctx context.Context, orgID, documentID, actorID uuid.UUID) (bool, error) {
	return c.userChecker.BelongsToOrganization(ctx, orgID, actorID)
}

func (c *AuthorizationChecker) CanAccessFormSubmission(ctx context.Context, orgID, submissionID, actorID uuid.UUID) (bool, error) {
	return c.userChecker.BelongsToOrganization(ctx, orgID, actorID)
}

func (c *AuthorizationChecker) CanAccessRuleEvaluation(ctx context.Context, orgID, evaluationID, actorID uuid.UUID) (bool, error) {
	return c.userChecker.BelongsToOrganization(ctx, orgID, actorID)
}

func (c *AuthorizationChecker) CanAccessDecision(ctx context.Context, orgID, decisionID, actorID uuid.UUID) (bool, error) {
	return c.userChecker.BelongsToOrganization(ctx, orgID, actorID)
}

var _ casecontextdomain.AuthorizationChecker = (*AuthorizationChecker)(nil)
