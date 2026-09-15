package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	intmid "github.com/alrazihi/civora/internal/middleware"
	rulesdomain "github.com/alrazihi/civora/internal/rules/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

func isPostgresUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "SQLSTATE 23505")
}

var (
	ErrRuleSetNotFound       = errors.New("rule set not found")
	ErrRuleSetInvalid        = errors.New("rule set is invalid")
	ErrRuleSetNotDraft       = errors.New("rule set is not a draft")
	ErrRuleSetNotPublished   = errors.New("rule set is not published")
	ErrRuleSetArchived       = errors.New("rule set is archived")
	ErrRuleSetKeyExists      = errors.New("rule set key already exists")
	ErrRuleSetVersionExists  = errors.New("rule set version already exists")
	ErrInvalidOutcome        = errors.New("invalid outcome")
	ErrDuplicatePriority     = errors.New("duplicate rule priority")
	ErrEvaluationNotFound    = errors.New("evaluation not found")
	ErrInvalidTrigger        = errors.New("invalid trigger")
	ErrInvalidFactPath       = errors.New("invalid fact path")
	ErrMaxRulesExceeded      = errors.New("maximum number of rules exceeded")
	ErrInvalidCondition      = errors.New("invalid condition")
	ErrUserNotFound          = errors.New("user not found")
	ErrRuleTemplateNotFound  = errors.New("rule template not found")
	ErrRuleTemplateInvalid   = errors.New("rule template is invalid")
	ErrRuleTemplateKeyExists = errors.New("rule template key already exists")
	ErrMaxTemplatesExceeded  = errors.New("maximum number of rule templates exceeded")
)

type RuleSetService struct {
	repo            rulesdomain.RuleSetRepository
	evalRepo        rulesdomain.EvaluationRepository
	templateRepo    rulesdomain.RuleTemplateRepository
	fieldDiscoverer shared.FieldDiscoverer
	auditor         auditdomain.EventRecorder
}

func NewRuleSetService(
	repo rulesdomain.RuleSetRepository,
	evalRepo rulesdomain.EvaluationRepository,
	templateRepo rulesdomain.RuleTemplateRepository,
	fieldDiscover shared.FieldDiscoverer,
	auditor auditdomain.EventRecorder,
) *RuleSetService {
	return &RuleSetService{
		repo:            repo,
		evalRepo:        evalRepo,
		templateRepo:    templateRepo,
		fieldDiscoverer: fieldDiscover,
		auditor:         auditor,
	}
}

func validDefaultOutcomes() map[rulesdomain.Outcome]bool {
	return map[rulesdomain.Outcome]bool{
		rulesdomain.OutcomeEligible:            true,
		rulesdomain.OutcomeIneligible:          true,
		rulesdomain.OutcomeRequiresReview:      true,
		rulesdomain.OutcomeInformationRequired: true,
		rulesdomain.OutcomeFlag:                true,
		rulesdomain.OutcomeScore:               true,
	}
}

func isValidDefaultOutcome(o rulesdomain.Outcome) bool {
	return validDefaultOutcomes()[o]
}

func validateKey(key string) error {
	if key == "" {
		return errors.New("key is required")
	}
	if len(key) > 100 {
		return errors.New("key exceeds maximum length of 100 characters")
	}
	return nil
}

func validateName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > 200 {
		return errors.New("name exceeds maximum length of 200 characters")
	}
	return nil
}

func validateDescription(desc string) error {
	if len(desc) > 2000 {
		return errors.New("description exceeds maximum length of 2000 characters")
	}
	return nil
}

type CreateRuleSetParams struct {
	OrganizationID uuid.UUID
	CaseID         *uuid.UUID
	Key            string
	Name           string
	Description    string
	DefaultOutcome rulesdomain.Outcome
	Rules          []rulesdomain.Rule
	Triggers       []string
	ActorID        uuid.UUID
}

func (s *RuleSetService) CreateRuleSet(ctx context.Context, params CreateRuleSetParams) (*rulesdomain.RuleSet, error) {
	if params.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrRuleSetInvalid)
	}
	if err := validateKey(params.Key); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetInvalid, err)
	}
	if err := validateName(params.Name); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetInvalid, err)
	}
	if err := validateDescription(params.Description); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetInvalid, err)
	}
	if !isValidDefaultOutcome(params.DefaultOutcome) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidOutcome, params.DefaultOutcome)
	}

	existing, err := s.repo.FindByKey(ctx, params.OrganizationID, params.Key)
	if err != nil && !errors.Is(err, rulesdomain.ErrRuleSetNotFound) {
		return nil, fmt.Errorf("failed to check duplicate key: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: key %s", ErrRuleSetKeyExists, params.Key)
	}

	rs, err := rulesdomain.NewRuleSet(&params.OrganizationID, params.CaseID, params.Key, params.Name, params.Description, params.DefaultOutcome, params.Rules, params.Triggers, params.ActorID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetInvalid, err)
	}

	var saved *rulesdomain.RuleSet
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, rs); err != nil {
			if isPostgresUniqueViolation(err) {
				return fmt.Errorf("%w: key %s", ErrRuleSetKeyExists, rs.Key)
			}
			return fmt.Errorf("failed to save rule set: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: rs.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "ruleset.created",
				Resource:       "ruleset",
				ResourceID:     shared.StrPtr(rs.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":     rs.Key,
					"version": rs.Version,
					"status":  string(rs.Status),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		saved = rs
		return nil
	})
	if err != nil {
		return nil, err
	}

	return saved, nil
}

type UpdateRuleSetParams struct {
	OrganizationID uuid.UUID
	ID             uuid.UUID
	Name           string
	Description    string
	DefaultOutcome rulesdomain.Outcome
	Rules          []rulesdomain.Rule
	Triggers       []string
	ActorID        uuid.UUID
}

func (s *RuleSetService) UpdateRuleSet(ctx context.Context, params UpdateRuleSetParams) (*rulesdomain.RuleSet, error) {
	if params.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrRuleSetInvalid)
	}
	if err := validateName(params.Name); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetInvalid, err)
	}
	if err := validateDescription(params.Description); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetInvalid, err)
	}
	if params.DefaultOutcome != "" && !isValidDefaultOutcome(params.DefaultOutcome) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidOutcome, params.DefaultOutcome)
	}

	var updated *rulesdomain.RuleSet
	err := database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		rs, err := s.repo.FindByIDForUpdateTx(ctx, tx, params.OrganizationID, params.ID)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrRuleSetNotFound, err)
		}

		if rs.Status != rulesdomain.StatusDraft {
			return fmt.Errorf("%w: only draft rule sets can be modified", ErrRuleSetNotDraft)
		}

		rs.Name = params.Name
		rs.Description = params.Description
		if params.DefaultOutcome != "" {
			rs.DefaultOutcome = params.DefaultOutcome
		}
		if params.Rules != nil {
			if len(params.Rules) > rulesdomain.MaxRulesPerRuleSet {
				return fmt.Errorf("%w: maximum %d rules", ErrRuleSetInvalid, rulesdomain.MaxRulesPerRuleSet)
			}
			rs.Rules = params.Rules
		}
		if params.Triggers != nil {
			rs.Triggers = params.Triggers
		}
		rs.UpdatedAt = time.Now().UTC()

		if err := rulesdomain.ValidateRuleSet(rs); err != nil {
			return fmt.Errorf("%w: %v", ErrRuleSetInvalid, err)
		}

		if err := s.repo.UpdateTx(ctx, tx, rs); err != nil {
			return fmt.Errorf("failed to update rule set: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: rs.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "ruleset.updated",
				Resource:       "ruleset",
				ResourceID:     shared.StrPtr(rs.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"version": rs.Version,
					"status":  string(rs.Status),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		updated = rs
		return nil
	})
	if err != nil {
		return nil, err
	}

	return updated, nil
}

type ListRuleSetsParams struct {
	OrganizationID uuid.UUID
	CaseID         *uuid.UUID
	Key            string
	Status         rulesdomain.RuleSetStatus
	Limit          int
	Offset         int
}

func (s *RuleSetService) ListRuleSets(ctx context.Context, params ListRuleSetsParams) ([]*rulesdomain.RuleSet, int, error) {
	if params.OrganizationID == uuid.Nil {
		return nil, 0, fmt.Errorf("%w: organization_id is required", ErrRuleSetInvalid)
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	if params.CaseID != nil && *params.CaseID != uuid.Nil {
		items, _, err := s.repo.ListByCase(ctx, params.OrganizationID, *params.CaseID, params.Limit, params.Offset)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to list rule sets by case: %w", err)
		}
		filtered := filterRuleSets(items, params.Key, params.Status)
		return filtered, len(filtered), nil
	}

	items, _, err := s.repo.List(ctx, params.OrganizationID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list rule sets: %w", err)
	}

	filtered := filterRuleSets(items, params.Key, params.Status)
	return filtered, len(filtered), nil
}

func filterRuleSets(items []*rulesdomain.RuleSet, key string, status rulesdomain.RuleSetStatus) []*rulesdomain.RuleSet {
	var result []*rulesdomain.RuleSet
	for _, rs := range items {
		if key != "" && rs.Key != key {
			continue
		}
		if status != "" && rs.Status != status {
			continue
		}
		result = append(result, rs)
	}
	return result
}

func (s *RuleSetService) GetRuleSet(ctx context.Context, orgID, id uuid.UUID) (*rulesdomain.RuleSet, error) {
	rs, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetNotFound, err)
	}
	return rs, nil
}

func (s *RuleSetService) GetRuleSetByKey(ctx context.Context, orgID uuid.UUID, key string) (*rulesdomain.RuleSet, error) {
	rs, err := s.repo.FindByKey(ctx, orgID, key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetNotFound, err)
	}
	return rs, nil
}

func (s *RuleSetService) ListVersions(ctx context.Context, orgID uuid.UUID, key string, limit, offset int) ([]*rulesdomain.RuleSet, int, error) {
	if orgID == uuid.Nil {
		return nil, 0, fmt.Errorf("%w: organization_id is required", ErrRuleSetInvalid)
	}
	if key == "" {
		return nil, 0, fmt.Errorf("%w: key is required", ErrRuleSetInvalid)
	}
	versions, total, err := s.repo.ListVersions(ctx, orgID, key, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list versions: %w", err)
	}
	return versions, total, nil
}

func (s *RuleSetService) CreateVersion(ctx context.Context, orgID, id uuid.UUID, actorID uuid.UUID) (*rulesdomain.RuleSet, error) {
	current, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetNotFound, err)
	}

	clone := current.Clone()
	clone.Status = rulesdomain.StatusDraft
	clone.UpdatedAt = time.Now().UTC()

	var saved *rulesdomain.RuleSet
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, clone); err != nil {
			return fmt.Errorf("failed to save new version: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: clone.OrganizationID,
				ActorID:        &actorID,
				Action:         "ruleset.version_created",
				Resource:       "ruleset",
				ResourceID:     shared.StrPtr(clone.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":     clone.Key,
					"version": clone.Version,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		saved = clone
		return nil
	})
	if err != nil {
		return nil, err
	}

	return saved, nil
}

func (s *RuleSetService) PublishRuleSet(ctx context.Context, orgID, id uuid.UUID, actorID uuid.UUID) (*rulesdomain.RuleSet, error) {
	rs, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetNotFound, err)
	}

	if rs.Status != rulesdomain.StatusDraft {
		return nil, fmt.Errorf("%w: only draft rule sets can be published", ErrRuleSetNotDraft)
	}

	if err := rulesdomain.ValidateRuleSet(rs); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetInvalid, err)
	}

	rs.Status = rulesdomain.StatusPublished
	rs.UpdatedAt = time.Now().UTC()

	var published *rulesdomain.RuleSet
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatusTx(ctx, tx, orgID, id, rulesdomain.StatusDraft, rulesdomain.StatusPublished); err != nil {
			if errors.Is(err, rulesdomain.ErrRuleSetNotFound) {
				return fmt.Errorf("%w: status changed concurrently", ErrRuleSetNotDraft)
			}
			return fmt.Errorf("failed to publish rule set: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: rs.OrganizationID,
				ActorID:        &actorID,
				Action:         "ruleset.published",
				Resource:       "ruleset",
				ResourceID:     shared.StrPtr(rs.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":     rs.Key,
					"version": rs.Version,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		published = rs
		return nil
	})
	if err != nil {
		return nil, err
	}

	return published, nil
}

func (s *RuleSetService) ArchiveRuleSet(ctx context.Context, orgID, id uuid.UUID, actorID uuid.UUID) (*rulesdomain.RuleSet, error) {
	rs, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetNotFound, err)
	}

	if rs.Status == rulesdomain.StatusArchived {
		return nil, fmt.Errorf("%w: already archived", ErrRuleSetArchived)
	}

	if rs.Status != rulesdomain.StatusPublished {
		return nil, fmt.Errorf("%w: only published rule sets can be archived", ErrRuleSetArchived)
	}

	rs.Status = rulesdomain.StatusArchived
	rs.UpdatedAt = time.Now().UTC()

	var archived *rulesdomain.RuleSet
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatusTx(ctx, tx, orgID, id, rulesdomain.StatusPublished, rulesdomain.StatusArchived); err != nil {
			if errors.Is(err, rulesdomain.ErrRuleSetNotFound) {
				return fmt.Errorf("%w: status changed concurrently", ErrRuleSetArchived)
			}
			return fmt.Errorf("failed to archive rule set: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: rs.OrganizationID,
				ActorID:        &actorID,
				ResourceID:     shared.StrPtr(rs.ID.String()),
				Action:         "ruleset.archived",
				Resource:       "ruleset",
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":     rs.Key,
					"version": rs.Version,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		archived = rs
		return nil
	})
	if err != nil {
		return nil, err
	}

	return archived, nil
}

func (s *RuleSetService) DeleteRuleSet(ctx context.Context, orgID, id uuid.UUID, actorID uuid.UUID) error {
	var rs *rulesdomain.RuleSet
	var err error
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		rs, err = s.repo.FindByIDForUpdateTx(ctx, tx, orgID, id)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrRuleSetNotFound, err)
		}

		if rs.Status != rulesdomain.StatusDraft {
			return fmt.Errorf("%w: only draft rule sets can be deleted", ErrRuleSetNotDraft)
		}

		if err := s.repo.DeleteTx(ctx, tx, orgID, id); err != nil {
			return fmt.Errorf("failed to delete rule set: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: rs.OrganizationID,
				ActorID:        &actorID,
				ResourceID:     shared.StrPtr(rs.ID.String()),
				Action:         "ruleset.deleted",
				Resource:       "ruleset",
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":     rs.Key,
					"version": rs.Version,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

type EvaluateRuleSetParams struct {
	OrganizationID uuid.UUID
	ID             uuid.UUID
	CaseID         *uuid.UUID
	Facts          map[string]interface{}
	Trigger        rulesdomain.Trigger
	ActorID        *uuid.UUID
	EvaluatedAt    *time.Time
}

func (s *RuleSetService) EvaluateRuleSet(ctx context.Context, params EvaluateRuleSetParams) (*rulesdomain.Evaluation, error) {
	rs, err := s.repo.FindByID(ctx, params.OrganizationID, params.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleSetNotFound, err)
	}

	if rs.Status == rulesdomain.StatusArchived {
		return nil, fmt.Errorf("%w: rule set is archived", ErrRuleSetArchived)
	}

	if rs.Status != rulesdomain.StatusPublished {
		return nil, fmt.Errorf("%w: only published rule sets can be evaluated", ErrRuleSetNotPublished)
	}

	trigger := params.Trigger
	if trigger == "" {
		trigger = rulesdomain.TriggerManual
	}
	if trigger != rulesdomain.TriggerManual && trigger != rulesdomain.TriggerAutomatic {
		return nil, ErrInvalidTrigger
	}

	evaluatedAt := time.Now().UTC()
	if params.EvaluatedAt != nil {
		evaluatedAt = params.EvaluatedAt.UTC()
	}

	facts := params.Facts
	if facts == nil {
		facts = map[string]interface{}{}
	}

	if err := rulesdomain.ValidateRuleSet(rs); err != nil {
		eval := &rulesdomain.Evaluation{
			ID:             uuid.New(),
			RuleSetID:      rs.ID,
			RuleSetVersion: rs.Version,
			OrganizationID: rs.OrganizationID,
			CaseID:         rs.CaseID,
			Status:         rulesdomain.StatusError,
			Outcome:        rulesdomain.OutcomeError,
			Reason:         shared.StrPtr(fmt.Sprintf("rule set validation failed: %v", err)),
			Trace:          []rulesdomain.TraceNode{},
			Trigger:        trigger,
			EvaluatedBy:    params.ActorID,
			EvaluatedAt:    evaluatedAt,
			FactsSnapshot:  facts,
		}
		var savedEval *rulesdomain.Evaluation
		err = database.InTransaction(ctx, s.evalRepo.DB(), func(tx *sql.Tx) error {
			if err := s.evalRepo.SaveTx(ctx, tx, eval); err != nil {
				return fmt.Errorf("failed to save evaluation: %w", err)
			}
			savedEval = eval
			return nil
		})
		if err != nil {
			return nil, err
		}
		return savedEval, nil
	}

	eval := rulesdomain.Evaluate(rs, facts, evaluatedAt, params.ActorID, trigger)

	var saved *rulesdomain.Evaluation
	if params.CaseID != nil {
		eval.CaseID = params.CaseID
	}

	err = database.InTransaction(ctx, s.evalRepo.DB(), func(tx *sql.Tx) error {
		if err := s.evalRepo.SaveTx(ctx, tx, eval); err != nil {
			return fmt.Errorf("failed to save evaluation: %w", err)
		}

		if s.auditor != nil {
			auditAction := "evaluation.manual"
			if trigger == rulesdomain.TriggerAutomatic {
				auditAction = "evaluation.automatic"
			}

			metadata := map[string]interface{}{
				"rule_set_id":      rs.ID.String(),
				"rule_set_version": rs.Version,
				"key":              rs.Key,
			}
			if eval.MatchedRuleID != nil {
				metadata["matched_rule_id"] = eval.MatchedRuleID.String()
			}
			metadata["outcome"] = string(eval.Outcome)
			metadata["status"] = string(eval.Status)

			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: eval.OrganizationID,
				ActorID:        params.ActorID,
				Action:         auditAction,
				Resource:       "evaluation",
				ResourceID:     shared.StrPtr(eval.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata:       metadata,
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		saved = eval
		return nil
	})
	if err != nil {
		return nil, err
	}

	return saved, nil
}

func (s *RuleSetService) GetEvaluation(ctx context.Context, orgID, id uuid.UUID) (*rulesdomain.Evaluation, error) {
	ev, err := s.evalRepo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEvaluationNotFound, err)
	}
	return ev, nil
}

func (s *RuleSetService) ListEvaluationsByRuleSet(ctx context.Context, orgID, ruleSetID uuid.UUID, limit, offset int) ([]*rulesdomain.Evaluation, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	items, total, err := s.evalRepo.ListByRuleSet(ctx, orgID, ruleSetID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list evaluations: %w", err)
	}

	return items, total, nil
}

func (s *RuleSetService) ListEvaluationsByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*rulesdomain.Evaluation, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	items, total, err := s.evalRepo.ListByCase(ctx, orgID, caseID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list evaluations by case: %w", err)
	}

	return items, total, nil
}

func (s *RuleSetService) ListDiscoverableFields(ctx context.Context, orgID uuid.UUID) ([]shared.FormFieldView, error) {
	if s.fieldDiscoverer == nil {
		return nil, errors.New("field discovery is not configured")
	}
	return s.fieldDiscoverer.ListDiscoverableFields(ctx, orgID)
}

type CreateRuleTemplateParams struct {
	OrganizationID uuid.UUID
	Scope          rulesdomain.RuleTemplateScope
	Key            string
	Name           string
	Description    string
	Category       string
	Rule           rulesdomain.Rule
	ActorID        uuid.UUID
}

func (s *RuleSetService) CreateRuleTemplate(ctx context.Context, params CreateRuleTemplateParams) (*rulesdomain.RuleTemplate, error) {
	if params.Scope == rulesdomain.RuleTemplateScopeOrg && params.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id required for org scope", ErrRuleTemplateInvalid)
	}
	if params.Scope == rulesdomain.RuleTemplateScopeGlobal && params.OrganizationID != uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id must be nil for global scope", ErrRuleTemplateInvalid)
	}
	if err := rulesdomain.ValidateRuleTemplateKey(params.Key); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleTemplateInvalid, err)
	}
	if params.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrRuleTemplateInvalid)
	}
	if len(params.Name) > 200 {
		return nil, fmt.Errorf("%w: name exceeds maximum length", ErrRuleTemplateInvalid)
	}
	if len(params.Description) > 2000 {
		return nil, fmt.Errorf("%w: description exceeds maximum length", ErrRuleTemplateInvalid)
	}
	if len(params.Category) > 100 {
		return nil, fmt.Errorf("%w: category exceeds maximum length", ErrRuleTemplateInvalid)
	}
	if !rulesdomain.IsValidOutcome(params.Rule.Outcome) {
		return nil, fmt.Errorf("%w: invalid rule outcome: %s", ErrRuleTemplateInvalid, params.Rule.Outcome)
	}
	if err := rulesdomain.ValidateCondition(&params.Rule.Conditions, 0); err != nil {
		return nil, fmt.Errorf("%w: invalid rule condition: %v", ErrRuleTemplateInvalid, err)
	}

	var orgID *uuid.UUID
	if params.Scope == rulesdomain.RuleTemplateScopeOrg {
		orgID = &params.OrganizationID
	}

	count, err := s.templateRepo.CountByKey(ctx, params.OrganizationID, params.Key)
	if err != nil && !errors.Is(err, rulesdomain.ErrRuleSetNotFound) {
		return nil, fmt.Errorf("failed to check duplicate key: %w", err)
	}
	if count > 0 {
		return nil, fmt.Errorf("%w: key %s", ErrRuleTemplateKeyExists, params.Key)
	}

	rt, err := rulesdomain.NewRuleTemplate(params.Scope, orgID, params.Key, params.Name, params.Description, params.Category, params.Rule, params.ActorID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleTemplateInvalid, err)
	}

	var saved *rulesdomain.RuleTemplate
	err = database.InTransaction(ctx, s.templateRepo.DB(), func(tx *sql.Tx) error {
		if err := s.templateRepo.SaveTx(ctx, tx, rt); err != nil {
			return fmt.Errorf("failed to save rule template: %w", err)
		}
		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "ruletemplate.created",
				Resource:       "ruletemplate",
				ResourceID:     shared.StrPtr(rt.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key":      rt.Key,
					"scope":    string(rt.Scope),
					"category": rt.Category,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}
		saved = rt
		return nil
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

type ListRuleTemplatesParams struct {
	OrganizationID uuid.UUID
	Scope          rulesdomain.RuleTemplateScope
	Category       string
	Limit          int
	Offset         int
}

func (s *RuleSetService) ListRuleTemplates(ctx context.Context, params ListRuleTemplatesParams) ([]*rulesdomain.RuleTemplate, int, error) {
	if params.Scope == rulesdomain.RuleTemplateScopeGlobal {
		return s.templateRepo.ListGlobal(ctx, params.Category, params.Limit, params.Offset)
	}
	if params.OrganizationID == uuid.Nil {
		return nil, 0, fmt.Errorf("%w: organization_id required for org scope", ErrRuleTemplateInvalid)
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}
	return s.templateRepo.List(ctx, params.OrganizationID, params.Scope, params.Category, params.Limit, params.Offset)
}

func (s *RuleSetService) GetRuleTemplate(ctx context.Context, orgID, id uuid.UUID) (*rulesdomain.RuleTemplate, error) {
	rt, err := s.templateRepo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleTemplateNotFound, err)
	}
	return rt, nil
}

func (s *RuleSetService) GetRuleTemplateByKey(ctx context.Context, orgID uuid.UUID, key string) (*rulesdomain.RuleTemplate, error) {
	rt, err := s.templateRepo.FindByKey(ctx, orgID, key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleTemplateNotFound, err)
	}
	return rt, nil
}

func (s *RuleSetService) DeleteRuleTemplate(ctx context.Context, orgID, id uuid.UUID, actorID uuid.UUID) error {
	rt, err := s.templateRepo.FindByID(ctx, orgID, id)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRuleTemplateNotFound, err)
	}

	err = database.InTransaction(ctx, s.templateRepo.DB(), func(tx *sql.Tx) error {
		if err := s.templateRepo.DeleteTx(ctx, tx, orgID, id); err != nil {
			return fmt.Errorf("failed to delete rule template: %w", err)
		}
		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: orgID,
				ActorID:        &actorID,
				Action:         "ruletemplate.deleted",
				Resource:       "ruletemplate",
				ResourceID:     shared.StrPtr(rt.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"key": rt.Key,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}
		return nil
	})
	return err
}

func (s *RuleSetService) InstantiateRuleTemplate(ctx context.Context, orgID, templateID uuid.UUID, actorID uuid.UUID) (*rulesdomain.Rule, error) {
	rt, err := s.templateRepo.FindByID(ctx, orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleTemplateNotFound, err)
	}
	cloned := rt.Rule
	cloned.ID = uuid.New()
	cloned.CreatedAt = time.Now().UTC()
	return &cloned, nil
}
