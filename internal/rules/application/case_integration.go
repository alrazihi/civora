package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	casesdomain "github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database"
	submissiondomain "github.com/alrazihi/civora/internal/form_submission/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	rulesdomain "github.com/alrazihi/civora/internal/rules/domain"
	"github.com/alrazihi/civora/internal/shared"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/google/uuid"
)

var (
	ErrRuleAssignmentNotFound = errors.New("rule assignment not found")
	ErrCaseNotFound           = errors.New("case not found")
)

const TriggerWorkflowTransition = "workflow_transition"

type CaseRuleIntegrationService struct {
	ruleSetRepo          rulesdomain.RuleSetRepository
	evalRepo             rulesdomain.EvaluationRepository
	caseRepo             casesdomain.CaseRepository
	workflowInstanceRepo workflowdomain.WorkflowInstanceRepository
	assignmentRepo       interface {
		FindByWorkflowAndState(ctx context.Context, orgID uuid.UUID, workflowDefID uuid.UUID, stateKey string) ([]*rulesdomain.WorkflowStateRuleAssignment, error)
		FindByWorkflowAndStateTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, workflowDefID uuid.UUID, stateKey string) ([]*rulesdomain.WorkflowStateRuleAssignment, error)
	}
	submissionLister shared.SubmissionLister
	formKeyResolver  shared.FormKeyResolver
	auditor          auditdomain.EventRecorder
}

func NewCaseRuleIntegrationService(
	ruleSetRepo rulesdomain.RuleSetRepository,
	evalRepo rulesdomain.EvaluationRepository,
	caseRepo casesdomain.CaseRepository,
	workflowInstanceRepo workflowdomain.WorkflowInstanceRepository,
	assignmentRepo interface {
		FindByWorkflowAndState(ctx context.Context, orgID uuid.UUID, workflowDefID uuid.UUID, stateKey string) ([]*rulesdomain.WorkflowStateRuleAssignment, error)
		FindByWorkflowAndStateTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, workflowDefID uuid.UUID, stateKey string) ([]*rulesdomain.WorkflowStateRuleAssignment, error)
	},
	submissionLister shared.SubmissionLister,
	formKeyResolver shared.FormKeyResolver,
	auditor auditdomain.EventRecorder,
) *CaseRuleIntegrationService {
	return &CaseRuleIntegrationService{
		ruleSetRepo:          ruleSetRepo,
		evalRepo:             evalRepo,
		caseRepo:             caseRepo,
		workflowInstanceRepo: workflowInstanceRepo,
		assignmentRepo:       assignmentRepo,
		submissionLister:     submissionLister,
		formKeyResolver:      formKeyResolver,
		auditor:              auditor,
	}
}

type EvaluateCaseRulesParams struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	ActorID        uuid.UUID
	Facts          map[string]interface{}
	Trigger        rulesdomain.Trigger
	Event          string
	Tx             *sql.Tx
}

type CaseRuleEvaluationResult struct {
	CaseID           uuid.UUID            `json:"case_id"`
	WorkflowState    string               `json:"workflow_state"`
	AssignedRuleSets []uuid.UUID          `json:"assigned_rule_sets"`
	Evaluations      []CaseRuleEvaluation `json:"evaluations"`
	HasMissingFacts  bool                 `json:"has_missing_facts"`
	EvaluatedAt      time.Time            `json:"evaluated_at"`
}

type CaseRuleEvaluation struct {
	RuleSetID      uuid.UUID                    `json:"rule_set_id"`
	RuleSetKey     string                       `json:"rule_set_key"`
	RuleSetName    string                       `json:"rule_set_name"`
	RuleSetVersion int                          `json:"rule_set_version"`
	Status         rulesdomain.EvaluationStatus `json:"status"`
	Outcome        rulesdomain.Outcome          `json:"outcome"`
	Reason         *string                      `json:"reason,omitempty"`
	MatchedRuleID  *uuid.UUID                   `json:"matched_rule_id,omitempty"`
	Trace          []rulesdomain.TraceNode      `json:"trace"`
	MissingFacts   []string                     `json:"missing_facts,omitempty"`
	EvaluatedAt    time.Time                    `json:"evaluated_at"`
	Trigger        rulesdomain.Trigger          `json:"trigger"`
}

func (s *CaseRuleIntegrationService) EvaluateCaseRules(ctx context.Context, params EvaluateCaseRulesParams) (*CaseRuleEvaluationResult, error) {
	if params.Trigger == "" {
		params.Trigger = rulesdomain.TriggerManual
	}
	if params.Trigger != rulesdomain.TriggerManual && params.Trigger != rulesdomain.TriggerAutomatic {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTrigger, params.Trigger)
	}

	caseEntity, err := s.findCase(ctx, params.Tx, params.OrganizationID, params.CaseID)
	if err != nil {
		if errors.Is(err, casesdomain.ErrCaseNotFound) {
			return nil, fmt.Errorf("case not found: %w", ErrCaseNotFound)
		}
		return nil, fmt.Errorf("case lookup failed: %w", err)
	}

	if caseEntity.WorkflowInstanceID == nil {
		return nil, fmt.Errorf("case has no workflow instance")
	}

	instance, err := s.findWorkflowInstance(ctx, params.Tx, params.OrganizationID, params.CaseID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("workflow instance not found")
		}
		return nil, fmt.Errorf("workflow instance lookup failed: %w", err)
	}

	assignments, err := s.findAssignments(ctx, params.Tx, params.OrganizationID, instance.WorkflowDefID, instance.CurrentState)
	if err != nil {
		return nil, fmt.Errorf("failed to get rule assignments: %w", err)
	}

	facts := params.Facts
	if facts == nil {
		facts, err = s.AssembleFactsFromCase(ctx, params.Tx, params.OrganizationID, params.CaseID)
		if err != nil {
			return nil, fmt.Errorf("failed to assemble facts: %w", err)
		}
	}

	result := &CaseRuleEvaluationResult{
		CaseID:        params.CaseID,
		WorkflowState: instance.CurrentState,
		EvaluatedAt:   time.Now().UTC(),
	}

	var evaluatedBy *uuid.UUID
	if params.ActorID != uuid.Nil {
		evaluatedBy = &params.ActorID
	}

	for _, assignment := range assignments {
		if !assignment.Active {
			continue
		}
		result.AssignedRuleSets = append(result.AssignedRuleSets, assignment.RuleSetID)

		rs, err := s.findRuleSet(ctx, params.Tx, params.OrganizationID, assignment.RuleSetID)
		if err != nil {
			continue
		}

		if rs.Status != rulesdomain.StatusPublished {
			continue
		}

		if !triggersMatch(rs.Triggers, params.Event) {
			continue
		}

		if err := rulesdomain.ValidateRuleSet(rs); err != nil {
			eval := &rulesdomain.Evaluation{
				ID:             uuid.New(),
				RuleSetID:      rs.ID,
				RuleSetVersion: rs.Version,
				OrganizationID: params.OrganizationID,
				CaseID:         &params.CaseID,
				Status:         rulesdomain.StatusError,
				Outcome:        rulesdomain.OutcomeError,
				Reason:         shared.StrPtr(fmt.Sprintf("rule set validation failed: %v", err)),
				Trace:          []rulesdomain.TraceNode{},
				Trigger:        params.Trigger,
				EvaluatedBy:    evaluatedBy,
				EvaluatedAt:    time.Now().UTC(),
				FactsSnapshot:  facts,
			}
			if err := s.saveEvaluation(ctx, params.Tx, eval); err != nil {
				return nil, fmt.Errorf("failed to save evaluation: %w", err)
			}
			result.Evaluations = append(result.Evaluations, CaseRuleEvaluation{
				RuleSetID:      rs.ID,
				RuleSetKey:     rs.Key,
				RuleSetName:    rs.Name,
				RuleSetVersion: rs.Version,
				Status:         eval.Status,
				Outcome:        eval.Outcome,
				Reason:         eval.Reason,
				MatchedRuleID:  nil,
				Trace:          eval.Trace,
				EvaluatedAt:    eval.EvaluatedAt,
				Trigger:        eval.Trigger,
			})
			continue
		}

		evaluatedAt := time.Now().UTC()
		eval := rulesdomain.Evaluate(rs, facts, evaluatedAt, evaluatedBy, params.Trigger)
		eval.CaseID = &params.CaseID

		var missingFacts []string
		if eval.Status == rulesdomain.StatusInformationRequired {
			missingFacts = findMissingFacts(rs, facts)
		}

		crEval := CaseRuleEvaluation{
			RuleSetID:      rs.ID,
			RuleSetKey:     rs.Key,
			RuleSetName:    rs.Name,
			RuleSetVersion: rs.Version,
			Status:         eval.Status,
			Outcome:        eval.Outcome,
			Reason:         eval.Reason,
			MatchedRuleID:  eval.MatchedRuleID,
			Trace:          eval.Trace,
			MissingFacts:   missingFacts,
			EvaluatedAt:    eval.EvaluatedAt,
			Trigger:        eval.Trigger,
		}
		result.Evaluations = append(result.Evaluations, crEval)

		if err := s.saveEvaluation(ctx, params.Tx, eval); err != nil {
			return nil, fmt.Errorf("failed to save evaluation: %w", err)
		}
	}

	result.HasMissingFacts = hasAnyMissing(result.Evaluations)

	return result, nil
}

// triggersMatch reports whether a rule set's triggers match the given event.
// Empty triggers mean the rule set evaluates on any event (default). Otherwise
// the event must appear verbatim in the triggers list, or "workflow_transition"
// must be listed (matching any transition event).
func triggersMatch(triggers []string, event string) bool {
	if len(triggers) == 0 {
		return true
	}
	if event == "" {
		return true
	}
	for _, t := range triggers {
		if t == event || t == TriggerWorkflowTransition {
			return true
		}
	}
	return false
}

func hasAnyMissing(evals []CaseRuleEvaluation) bool {
	for _, e := range evals {
		if e.Status == rulesdomain.StatusInformationRequired || len(e.MissingFacts) > 0 {
			return true
		}
	}
	return false
}

func findMissingFacts(rs *rulesdomain.RuleSet, facts map[string]interface{}) []string {
	var missing []string
	seen := map[string]bool{}
	for _, rule := range rs.Rules {
		findMissingInCondition(rule.Conditions, facts, &missing, seen)
	}
	return missing
}

func findMissingInCondition(c rulesdomain.Condition, facts map[string]interface{}, missing *[]string, seen map[string]bool) {
	if c.IsLeaf() && c.Field != "" {
		if _, ok := rulesdomain.ResolveFact(facts, c.Field); !ok {
			if !seen[c.Field] {
				*missing = append(*missing, c.Field)
				seen[c.Field] = true
			}
		}
		return
	}
	for _, child := range c.All {
		findMissingInCondition(child, facts, missing, seen)
	}
	for _, child := range c.Any {
		findMissingInCondition(child, facts, missing, seen)
	}
	if c.Not != nil {
		findMissingInCondition(*c.Not, facts, missing, seen)
	}
}

func (s *CaseRuleIntegrationService) findCase(ctx context.Context, tx *sql.Tx, orgID, caseID uuid.UUID) (*casesdomain.Case, error) {
	if tx != nil {
		return s.caseRepo.FindByIDTx(ctx, tx, orgID, caseID)
	}
	return s.caseRepo.FindByID(ctx, orgID, caseID)
}

func (s *CaseRuleIntegrationService) findWorkflowInstance(ctx context.Context, tx *sql.Tx, orgID, caseID uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
	if tx != nil {
		return s.workflowInstanceRepo.FindByCaseIDTx(ctx, tx, orgID, caseID)
	}
	return s.workflowInstanceRepo.FindByCaseID(ctx, orgID, caseID)
}

func (s *CaseRuleIntegrationService) findAssignments(ctx context.Context, tx *sql.Tx, orgID, workflowDefID uuid.UUID, stateKey string) ([]*rulesdomain.WorkflowStateRuleAssignment, error) {
	if tx != nil {
		return s.assignmentRepo.FindByWorkflowAndStateTx(ctx, tx, orgID, workflowDefID, stateKey)
	}
	return s.assignmentRepo.FindByWorkflowAndState(ctx, orgID, workflowDefID, stateKey)
}

func (s *CaseRuleIntegrationService) findRuleSet(ctx context.Context, tx *sql.Tx, orgID, ruleSetID uuid.UUID) (*rulesdomain.RuleSet, error) {
	if tx != nil {
		return s.ruleSetRepo.FindByIDTx(ctx, tx, orgID, ruleSetID)
	}
	return s.ruleSetRepo.FindByID(ctx, orgID, ruleSetID)
}

func (s *CaseRuleIntegrationService) saveEvaluation(ctx context.Context, tx *sql.Tx, eval *rulesdomain.Evaluation) error {
	if tx != nil {
		if err := s.evalRepo.SaveTx(ctx, tx, eval); err != nil {
			return fmt.Errorf("failed to save evaluation: %w", err)
		}
		return s.recordEvaluationAudit(ctx, tx, eval)
	}
	return database.InTransaction(ctx, s.evalRepo.DB(), func(innerTx *sql.Tx) error {
		if err := s.evalRepo.SaveTx(ctx, innerTx, eval); err != nil {
			return fmt.Errorf("failed to save evaluation: %w", err)
		}
		return s.recordEvaluationAudit(ctx, innerTx, eval)
	})
}

func (s *CaseRuleIntegrationService) recordEvaluationAudit(ctx context.Context, tx *sql.Tx, eval *rulesdomain.Evaluation) error {
	if s.auditor == nil {
		return nil
	}
	auditAction := "evaluation.manual"
	if eval.Trigger == rulesdomain.TriggerAutomatic {
		auditAction = "evaluation.automatic"
	}
	metadata := map[string]interface{}{
		"rule_set_id":      eval.RuleSetID.String(),
		"rule_set_version": eval.RuleSetVersion,
		"case_id":          eval.CaseID.String(),
		"outcome":          string(eval.Outcome),
		"status":           string(eval.Status),
	}
	if eval.MatchedRuleID != nil {
		metadata["matched_rule_id"] = eval.MatchedRuleID.String()
	}
	return shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
		OrganizationID: eval.OrganizationID,
		ActorID:        eval.EvaluatedBy,
		Action:         auditAction,
		Resource:       "evaluation",
		ResourceID:     shared.StrPtr(eval.ID.String()),
		Outcome:        "success",
		RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
		Metadata:       metadata,
	})
}

func (s *CaseRuleIntegrationService) ListEvaluationsByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*rulesdomain.Evaluation, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return s.evalRepo.ListByCase(ctx, orgID, caseID, limit, offset)
}

func (s *CaseRuleIntegrationService) AssembleFactsFromCase(ctx context.Context, tx *sql.Tx, orgID, caseID uuid.UUID) (map[string]interface{}, error) {
	facts := map[string]interface{}{}

	caseEntity, err := s.findCase(ctx, tx, orgID, caseID)
	if err != nil {
		return nil, fmt.Errorf("case not found: %w", err)
	}

	ageDays := 0.0
	if !caseEntity.CreatedAt.IsZero() {
		ageDays = caseEntity.CreatedAt.UTC().Sub(time.Now().UTC()).Hours() / -24.0
	}
	facts["case"] = map[string]interface{}{
		"id":             caseEntity.ID.String(),
		"status":         string(caseEntity.Status),
		"workflow_state": caseEntity.WorkflowState,
		"case_number":    caseEntity.CaseNumber,
		"title":          caseEntity.Title,
		"service_type":   string(caseEntity.ServiceType),
		"priority":       string(caseEntity.Priority),
		"age_days":       ageDays,
	}

	facts["form"] = map[string]interface{}{}

	var submissions []*submissiondomain.FormSubmission
	if tx != nil {
		submissions, err = s.submissionLister.ListByCaseTx(ctx, tx, orgID, caseID)
	} else {
		submissions, err = s.submissionLister.ListByCase(ctx, orgID, caseID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list submissions: %w", err)
	}

	latestByKey := map[string]*submissiondomain.FormSubmission{}
	for _, sub := range submissions {
		var formView *shared.FormView
		if tx != nil {
			formView, err = s.formKeyResolver.FindByIDTx(ctx, tx, orgID, sub.FormID)
		} else {
			formView, err = s.formKeyResolver.FindByID(ctx, orgID, sub.FormID)
		}
		if err != nil {
			continue
		}
		existing, ok := latestByKey[formView.Key]
		if !ok || sub.SubmittedAt.After(existing.SubmittedAt) {
			latestByKey[formView.Key] = sub
		}
	}

	for formKey, sub := range latestByKey {
		formData := map[string]interface{}{
			"submitted_at": sub.SubmittedAt,
			"form_id":      sub.FormID.String(),
			"status":       string(sub.Status),
		}
		if sub.Data != nil {
			for k, v := range sub.Data {
				formData[k] = v
			}
		}
		facts["form"].(map[string]interface{})[formKey] = formData
	}

	return facts, nil
}
