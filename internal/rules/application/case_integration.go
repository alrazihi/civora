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
	intmid "github.com/alrazihi/civora/internal/middleware"
	rulesdomain "github.com/alrazihi/civora/internal/rules/domain"
	"github.com/alrazihi/civora/internal/shared"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/google/uuid"
)

var (
	ErrRuleAssignmentNotFound = errors.New("rule assignment not found")
	ErrNoRuleAssignments      = errors.New("no rule assignments for this state")
	ErrCaseNotFound           = errors.New("case not found")
)

type CaseRuleIntegrationService struct {
	ruleSetRepo          rulesdomain.RuleSetRepository
	evalRepo             rulesdomain.EvaluationRepository
	caseRepo             casesdomain.CaseRepository
	workflowInstanceRepo workflowdomain.WorkflowInstanceRepository
	assignmentRepo       interface {
		FindByWorkflowAndState(ctx context.Context, orgID uuid.UUID, workflowDefID uuid.UUID, stateKey string) ([]*rulesdomain.WorkflowStateRuleAssignment, error)
	}
	submissionLister shared.SubmissionLister
	auditor          auditdomain.EventRecorder
}

func NewCaseRuleIntegrationService(
	ruleSetRepo rulesdomain.RuleSetRepository,
	evalRepo rulesdomain.EvaluationRepository,
	caseRepo casesdomain.CaseRepository,
	workflowInstanceRepo workflowdomain.WorkflowInstanceRepository,
	assignmentRepo interface {
		FindByWorkflowAndState(ctx context.Context, orgID uuid.UUID, workflowDefID uuid.UUID, stateKey string) ([]*rulesdomain.WorkflowStateRuleAssignment, error)
	},
	submissionLister shared.SubmissionLister,
	auditor auditdomain.EventRecorder,
) *CaseRuleIntegrationService {
	return &CaseRuleIntegrationService{
		ruleSetRepo:          ruleSetRepo,
		evalRepo:             evalRepo,
		caseRepo:             caseRepo,
		workflowInstanceRepo: workflowInstanceRepo,
		assignmentRepo:       assignmentRepo,
		submissionLister:     submissionLister,
		auditor:              auditor,
	}
}

type EvaluateCaseRulesParams struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	ActorID        uuid.UUID
	Facts          map[string]interface{}
	Trigger        rulesdomain.Trigger
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
		return nil, fmt.Errorf("invalid trigger: %w", ErrInvalidTrigger)
	}

	caseEntity, err := s.caseRepo.FindByID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		if errors.Is(err, casesdomain.ErrCaseNotFound) {
			return nil, fmt.Errorf("case not found: %w", ErrCaseNotFound)
		}
		return nil, fmt.Errorf("case lookup failed: %w", err)
	}

	if caseEntity.WorkflowInstanceID == nil {
		return nil, fmt.Errorf("case has no workflow instance")
	}

	instance, err := s.workflowInstanceRepo.FindByCaseID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("workflow instance not found")
		}
		return nil, fmt.Errorf("workflow instance lookup failed: %w", err)
	}

	assignments, err := s.assignmentRepo.FindByWorkflowAndState(ctx, params.OrganizationID, instance.WorkflowDefID, instance.CurrentState)
	if err != nil {
		return nil, fmt.Errorf("failed to get rule assignments: %w", err)
	}

	result := &CaseRuleEvaluationResult{
		CaseID:        params.CaseID,
		WorkflowState: instance.CurrentState,
		EvaluatedAt:   time.Now().UTC(),
	}

	for _, assignment := range assignments {
		if !assignment.Active {
			continue
		}
		result.AssignedRuleSets = append(result.AssignedRuleSets, assignment.RuleSetID)

		rs, err := s.ruleSetRepo.FindByID(ctx, params.OrganizationID, assignment.RuleSetID)
		if err != nil {
			continue
		}

		facts := params.Facts
		if facts == nil {
			facts = map[string]interface{}{}
		}

		evaluatedAt := time.Now().UTC()
		eval := rulesdomain.Evaluate(rs, facts, evaluatedAt, &params.ActorID, params.Trigger)
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

		if err := s.saveEvaluation(ctx, eval); err != nil {
			return nil, fmt.Errorf("failed to save evaluation: %w", err)
		}
	}

	result.HasMissingFacts = hasAnyMissing(result.Evaluations)

	return result, nil
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

func (s *CaseRuleIntegrationService) saveEvaluation(ctx context.Context, eval *rulesdomain.Evaluation) error {
	return database.InTransaction(ctx, s.evalRepo.DB(), func(tx *sql.Tx) error {
		if err := s.evalRepo.SaveTx(ctx, tx, eval); err != nil {
			return fmt.Errorf("failed to save evaluation: %w", err)
		}
		if s.auditor != nil {
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
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: eval.OrganizationID,
				ActorID:        eval.EvaluatedBy,
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
		return nil
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

func (s *CaseRuleIntegrationService) AssembleFactsFromCase(ctx context.Context, orgID, caseID uuid.UUID) (map[string]interface{}, error) {
	facts := map[string]interface{}{}

	caseEntity, err := s.caseRepo.FindByID(ctx, orgID, caseID)
	if err != nil {
		return nil, fmt.Errorf("case not found: %w", err)
	}

	facts["case"] = map[string]interface{}{
		"id":             caseEntity.ID.String(),
		"status":         string(caseEntity.Status),
		"workflow_state": caseEntity.WorkflowState,
		"case_number":    caseEntity.CaseNumber,
		"title":          caseEntity.Title,
		"service_type":   string(caseEntity.ServiceType),
		"priority":       string(caseEntity.Priority),
	}

	submissions, err := s.submissionLister.ListByCase(ctx, orgID, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed to list submissions: %w", err)
	}

	submissionList := make([]map[string]interface{}, 0, len(submissions))
	for _, sub := range submissions {
		submissionList = append(submissionList, map[string]interface{}{
			"form_id":         sub.FormID.String(),
			"form_version_id": sub.FormVersionID.String(),
			"status":          string(sub.Status),
			"submitted_at":    sub.SubmittedAt,
			"data":            sub.Data,
		})
	}
	facts["form_submissions"] = submissionList

	return facts, nil
}
