package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/alrazihi/civora/internal/casecontext/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrContextNotFound      = errors.New("case context not found")
	ErrAuthorizationFailed  = errors.New("authorization failed")
	ErrTenantViolation      = errors.New("tenant violation")
	ErrCaseAccessDenied     = errors.New("case access denied")
	ErrEvidenceAccessDenied = errors.New("evidence access denied")
	ErrDocumentAccessDenied = errors.New("document access denied")
	ErrPersonAccessDenied   = errors.New("person access denied")
	ErrInsufficientContext  = errors.New("insufficient information to build context")
)

type CaseContextService struct {
	ctxRepo      domain.CaseContextRepository
	authChecker  domain.AuthorizationChecker
	caseRepo     domain.CaseRepository
	personRepo   domain.PersonRepository
	evidenceRepo domain.EvidenceRepository
	docProvider  domain.DocumentContentProvider
	formRepo     domain.FormSubmissionRepository
	ruleRepo     domain.RuleEvaluationRepository
	workflowRepo domain.WorkflowRepository
	decisionRepo domain.DecisionRepository
	aiObsRepo    domain.AIObservationRepository
	userChecker  shared.UserChecker
	db           *sql.DB
}

func NewCaseContextService(
	ctxRepo domain.CaseContextRepository,
	authChecker domain.AuthorizationChecker,
	caseRepo domain.CaseRepository,
	personRepo domain.PersonRepository,
	evidenceRepo domain.EvidenceRepository,
	formRepo domain.FormSubmissionRepository,
	ruleRepo domain.RuleEvaluationRepository,
	workflowRepo domain.WorkflowRepository,
	decisionRepo domain.DecisionRepository,
	aiObsRepo domain.AIObservationRepository,
	userChecker shared.UserChecker,
) *CaseContextService {
	return &CaseContextService{
		ctxRepo:      ctxRepo,
		authChecker:  authChecker,
		caseRepo:     caseRepo,
		personRepo:   personRepo,
		evidenceRepo: evidenceRepo,
		formRepo:     formRepo,
		ruleRepo:     ruleRepo,
		workflowRepo: workflowRepo,
		decisionRepo: decisionRepo,
		aiObsRepo:    aiObsRepo,
		userChecker:  userChecker,
	}
}

func (s *CaseContextService) WithTransaction(db *sql.DB) *CaseContextService {
	s.db = db
	return s
}

func (s *CaseContextService) WithDocumentProvider(provider domain.DocumentContentProvider) *CaseContextService {
	s.docProvider = provider
	return s
}

type BuildContextParams struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	ActorID        uuid.UUID
	Options        domain.ContextOptions
}

type BuildContextResult struct {
	Context *domain.CaseContext
}

func (s *CaseContextService) BuildContext(ctx context.Context, params BuildContextParams) (*BuildContextResult, error) {
	if params.ActorID == uuid.Nil {
		return nil, fmt.Errorf("%w: actor ID is required", ErrAuthorizationFailed)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.ActorID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate actor: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("%w: user does not belong to organization", ErrAuthorizationFailed)
	}

	canAccess, err := s.authChecker.CanAccessCase(ctx, params.OrganizationID, params.CaseID, params.ActorID)
	if err != nil {
		return nil, fmt.Errorf("failed to check case access: %w", err)
	}
	if !canAccess {
		return nil, fmt.Errorf("%w: cannot access case %s", ErrCaseAccessDenied, params.CaseID)
	}

	builder := domain.NewContextBuilder(params.OrganizationID, params.CaseID, params.ActorID, params.Options)

	if err := s.buildCaseMetadataFacts(ctx, builder, params); err != nil {
		return nil, err
	}

	if err := s.buildPersonFacts(ctx, builder, params); err != nil {
		return nil, err
	}

	if err := s.buildEvidenceFacts(ctx, builder, params); err != nil {
		return nil, err
	}

	if err := s.buildDocumentFacts(ctx, builder, params); err != nil {
		return nil, err
	}

	if err := s.buildFormSubmissionFacts(ctx, builder, params); err != nil {
		return nil, err
	}

	if err := s.buildRuleEvaluationFacts(ctx, builder, params); err != nil {
		return nil, err
	}

	if err := s.buildWorkflowFacts(ctx, builder, params); err != nil {
		return nil, err
	}

	if err := s.buildDecisionFacts(ctx, builder, params); err != nil {
		return nil, err
	}

	if err := s.buildAIObservationFacts(ctx, builder, params); err != nil {
		return nil, err
	}

	caseContext, err := builder.Build()
	if err != nil {
		return nil, err
	}

	if err := s.saveContext(ctx, caseContext); err != nil {
		return nil, err
	}

	return &BuildContextResult{Context: caseContext}, nil
}

func (s *CaseContextService) saveContext(ctx context.Context, caseCtx *domain.CaseContext) error {
	if s.db != nil {
		return s.withTx(ctx, func(tx *sql.Tx) error {
			return s.ctxRepo.SaveTx(ctx, tx, caseCtx)
		})
	}
	return s.ctxRepo.Save(ctx, caseCtx)
}

func (s *CaseContextService) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	if s.db == nil {
		return fn(nil)
	}
	return s.runInTransaction(ctx, fn)
}

func (s *CaseContextService) runInTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *CaseContextService) buildCaseMetadataFacts(ctx context.Context, builder *domain.ContextBuilder, params BuildContextParams) error {
	if !params.Options.IncludeCaseMetadata {
		builder.RecordAuthorization("case", []uuid.UUID{params.CaseID}, true, "included by option")
		return nil
	}

	c, err := s.caseRepo.FindByID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		builder.RecordAuthorization("case", []uuid.UUID{params.CaseID}, false, err.Error())
		return nil
	}

	builder.RecordAuthorization("case", []uuid.UUID{params.CaseID}, true, "case found")

	fact := domain.NewFact(params.CaseID, domain.FactSourceCaseMetadata, c.ID, domain.ProvenanceSourceFact,
		"case", map[string]any{
			"id":                   c.ID.String(),
			"case_number":          c.CaseNumber,
			"title":                c.Title,
			"description":          c.Description,
			"status":               c.Status,
			"workflow_state":       c.WorkflowState,
			"service_type":         c.ServiceType,
			"priority":             c.Priority,
			"person_id":            uuidOrNil(c.PersonID),
			"assigned_to_id":       uuidOrNil(c.AssignedToID),
			"created_by_id":        c.CreatedByID.String(),
			"created_at":           c.CreatedAt,
			"updated_at":           c.UpdatedAt,
			"closed_at":            c.ClosedAt,
			"workflow_instance_id": uuidOrNil(c.WorkflowInstanceID),
			"workflow_key":         c.WorkflowKey,
			"workflow_id":          uuidOrNil(c.WorkflowID),
			"version":              c.Version,
		})

	return builder.AddFact(fact)
}

func (s *CaseContextService) buildPersonFacts(ctx context.Context, builder *domain.ContextBuilder, params BuildContextParams) error {
	if !params.Options.IncludePerson {
		return nil
	}

	c, err := s.caseRepo.FindByID(ctx, params.OrganizationID, params.CaseID)
	if err != nil || c.PersonID == nil {
		builder.RecordAuthorization("person", nil, false, "no person linked to case")
		return nil
	}

	canAccess, err := s.authChecker.CanAccessPerson(ctx, params.OrganizationID, *c.PersonID, params.ActorID)
	if err != nil {
		builder.RecordAuthorization("person", []uuid.UUID{*c.PersonID}, false, err.Error())
		return nil
	}
	if !canAccess {
		builder.RecordAuthorization("person", []uuid.UUID{*c.PersonID}, false, "access denied")
		return nil
	}

	person, err := s.personRepo.FindByID(ctx, params.OrganizationID, *c.PersonID)
	if err != nil {
		builder.RecordAuthorization("person", []uuid.UUID{*c.PersonID}, false, err.Error())
		return nil
	}

	builder.RecordAuthorization("person", []uuid.UUID{*c.PersonID}, true, "person found")

	personData := map[string]any{
		"id":            person.ID.String(),
		"legal_name":    person.LegalName,
		"date_of_birth": person.DateOfBirth,
		"gender":        person.Gender,
		"nationality":   person.Nationality,
		"address":       person.Address,
		"phone":         person.Phone,
		"email":         person.Email,
		"metadata":      person.Metadata,
		"created_at":    person.CreatedAt,
		"updated_at":    person.UpdatedAt,
	}

	if params.Options.OnlyHumanVerified {
		personData["provenance_note"] = "Person data is SOURCE_FACT - verify if needed"
	}

	fact := domain.NewFact(params.CaseID, domain.FactSourcePerson, person.ID, domain.ProvenanceSourceFact,
		"person", personData)

	return builder.AddFact(fact)
}

func (s *CaseContextService) buildEvidenceFacts(ctx context.Context, builder *domain.ContextBuilder, params BuildContextParams) error {
	if !params.Options.IncludeEvidence && !params.Options.IncludeVerifiedEvidence {
		return nil
	}

	evidenceList, total, err := s.evidenceRepo.FindByCase(ctx, params.OrganizationID, params.CaseID, 100, 0)
	if err != nil {
		builder.RecordAuthorization("evidence", nil, false, err.Error())
		return nil
	}

	var evidenceIDs []uuid.UUID
	for _, e := range evidenceList {
		evidenceIDs = append(evidenceIDs, e.ID)
	}
	builder.RecordAuthorization("evidence", evidenceIDs, len(evidenceIDs) > 0, fmt.Sprintf("found %d evidence items", total))

	for _, e := range evidenceList {
		if params.Options.IncludeVerifiedEvidence && e.VerificationStatus != "VERIFIED" {
			continue
		}
		if params.Options.OnlyVerified && e.VerificationStatus != "VERIFIED" {
			continue
		}

		canAccess, err := s.authChecker.CanAccessEvidence(ctx, params.OrganizationID, e.ID, params.ActorID)
		if err != nil || !canAccess {
			continue
		}

		provenance := domain.ProvenanceSourceFact
		if e.VerificationStatus == "VERIFIED" {
			provenance = domain.ProvenanceHumanVerified
		}

		evidenceData := map[string]any{
			"id":                  e.ID.String(),
			"type":                e.Type,
			"custom_type":         e.CustomType,
			"description":         e.Description,
			"source":              e.Source,
			"metadata":            e.Metadata,
			"verification_status": e.VerificationStatus,
			"verified_by":         uuidOrNil(e.VerifiedBy),
			"verified_at":         e.VerifiedAt,
			"verification_reason": e.VerificationReason,
			"verification_method": e.VerificationMethod,
			"person_id":           uuidOrNil(e.PersonID),
			"uploaded_by":         e.UploadedBy.String(),
			"created_at":          e.CreatedAt,
		}

		fact := domain.NewFact(params.CaseID, domain.FactSourceEvidence, e.ID, provenance,
			"evidence", evidenceData)

		if e.VerifiedBy != nil {
			fact = fact.WithReference(*e.VerifiedBy)
		}

		if err := builder.AddFact(fact); err != nil {
			if strings.Contains(err.Error(), "exceeds maximum") {
				return nil
			}
			return err
		}
	}

	return nil
}

func (s *CaseContextService) buildDocumentFacts(ctx context.Context, builder *domain.ContextBuilder, params BuildContextParams) error {
	if !params.Options.IncludeDocuments || s.docProvider == nil {
		return nil
	}

	evidenceList, _, err := s.evidenceRepo.FindByCase(ctx, params.OrganizationID, params.CaseID, 100, 0)
	if err != nil {
		return nil
	}

	for _, e := range evidenceList {
		if params.Options.OnlyVerified && e.VerificationStatus != "VERIFIED" {
			continue
		}

		docs, _, err := s.evidenceRepo.ListDocuments(ctx, params.OrganizationID, e.ID, 20, 0)
		if err != nil || len(docs) == 0 {
			continue
		}

		for _, doc := range docs {
			canAccess, err := s.authChecker.CanAccessDocument(ctx, params.OrganizationID, doc.ID, params.ActorID)
			if err != nil || !canAccess {
				continue
			}

			content, err := s.docProvider.GetDocumentContent(ctx, params.OrganizationID, e.ID, doc.ID, params.ActorID)
			if err != nil {
				continue
			}

			docData := map[string]any{
				"id":               doc.ID.String(),
				"evidence_id":      e.ID.String(),
				"file_name":        doc.FileName,
				"content_type":     doc.ContentType,
				"size_bytes":       doc.SizeBytes,
				"checksum":         doc.Checksum,
				"storage_provider": doc.StorageProvider,
				"uploaded_by":      doc.UploadedBy.String(),
				"uploaded_at":      doc.UploadedAt,
				"content_preview":  string(content[:min(len(content), 2000)]),
				"truncated":        len(content) > 2000,
			}

			provenance := domain.ProvenanceSourceFact
			if e.VerificationStatus == "VERIFIED" {
				provenance = domain.ProvenanceHumanVerified
			}

			fact := domain.NewFact(params.CaseID, domain.FactSourceDocument, doc.ID, provenance,
				"document", docData)

			if err := builder.AddFact(fact); err != nil {
				if strings.Contains(err.Error(), "exceeds maximum") {
					return nil
				}
				return err
			}
		}
	}

	return nil
}

func (s *CaseContextService) buildFormSubmissionFacts(ctx context.Context, builder *domain.ContextBuilder, params BuildContextParams) error {
	if !params.Options.IncludeFormSubmissions {
		return nil
	}

	submissions, err := s.formRepo.ListByCase(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		builder.RecordAuthorization("form_submission", nil, false, err.Error())
		return nil
	}

	var submissionIDs []uuid.UUID
	for _, sub := range submissions {
		submissionIDs = append(submissionIDs, sub.ID)
	}
	builder.RecordAuthorization("form_submission", submissionIDs, len(submissions) > 0, fmt.Sprintf("found %d submissions", len(submissions)))

	for _, sub := range submissions {
		canAccess, err := s.authChecker.CanAccessFormSubmission(ctx, params.OrganizationID, sub.ID, params.ActorID)
		if err != nil || !canAccess {
			continue
		}

		subData := map[string]any{
			"id":              sub.ID.String(),
			"form_id":         sub.FormID.String(),
			"form_version_id": sub.FormVersionID.String(),
			"submitted_by":    sub.SubmittedBy.String(),
			"status":          sub.Status,
			"data":            sub.Data,
			"submitted_at":    sub.SubmittedAt,
			"updated_at":      sub.UpdatedAt,
		}

		fact := domain.NewFact(params.CaseID, domain.FactSourceFormSubmission, sub.ID, domain.ProvenanceFormSubmission,
			"form_submission", subData)

		if err := builder.AddFact(fact); err != nil {
			if strings.Contains(err.Error(), "exceeds maximum") {
				return nil
			}
			return err
		}
	}

	return nil
}

func (s *CaseContextService) buildRuleEvaluationFacts(ctx context.Context, builder *domain.ContextBuilder, params BuildContextParams) error {
	if !params.Options.IncludeRuleEvaluations {
		return nil
	}

	evaluations, total, err := s.ruleRepo.FindByCase(ctx, params.OrganizationID, params.CaseID, 50, 0)
	if err != nil {
		builder.RecordAuthorization("rule_evaluation", nil, false, err.Error())
		return nil
	}

	var evalIDs []uuid.UUID
	for _, eval := range evaluations {
		evalIDs = append(evalIDs, eval.ID)
	}
	builder.RecordAuthorization("rule_evaluation", evalIDs, len(evaluations) > 0, fmt.Sprintf("found %d evaluations", total))

	for _, eval := range evaluations {
		canAccess, err := s.authChecker.CanAccessRuleEvaluation(ctx, params.OrganizationID, eval.ID, params.ActorID)
		if err != nil || !canAccess {
			continue
		}

		evalData := map[string]any{
			"id":                  eval.ID.String(),
			"rule_set_id":         eval.RuleSetID.String(),
			"rule_set_version":    eval.RuleSetVersion,
			"status":              eval.Status,
			"outcome":             eval.Outcome,
			"reason":              eval.Reason,
			"matched_rule_id":     uuidOrNil(eval.MatchedRuleID),
			"trigger":             eval.Trigger,
			"evaluated_by":        uuidOrNil(eval.EvaluatedBy),
			"evaluated_at":        eval.EvaluatedAt,
			"facts_snapshot_keys": getMapKeys(eval.FactsSnapshot),
		}

		provenance := domain.ProvenanceRuleResult

		fact := domain.NewFact(params.CaseID, domain.FactSourceRuleEvaluation, eval.ID, provenance,
			"rule_evaluation", evalData)

		if eval.MatchedRuleID != nil {
			fact = fact.WithReference(*eval.MatchedRuleID)
		}

		if err := builder.AddFact(fact); err != nil {
			if strings.Contains(err.Error(), "exceeds maximum") {
				return nil
			}
			return err
		}
	}

	return nil
}

func (s *CaseContextService) buildWorkflowFacts(ctx context.Context, builder *domain.ContextBuilder, params BuildContextParams) error {
	if !params.Options.IncludeWorkflowHistory {
		return nil
	}

	instance, err := s.workflowRepo.FindInstanceByCase(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		builder.RecordAuthorization("workflow_instance", nil, false, err.Error())
	} else if instance != nil {
		builder.RecordAuthorization("workflow_instance", []uuid.UUID{instance.ID}, true, "workflow instance found")

		instanceData := map[string]any{
			"id":                   instance.ID.String(),
			"workflow_def_id":      instance.WorkflowDefID.String(),
			"workflow_def_version": instance.WorkflowDefVersion,
			"current_state":        instance.CurrentState,
			"started_at":           instance.StartedAt,
			"completed_at":         instance.CompletedAt,
			"metadata":             instance.Metadata,
			"version":              instance.Version,
		}

		fact := domain.NewFact(params.CaseID, domain.FactSourceWorkflowInstance, instance.ID, domain.ProvenanceSourceFact,
			"workflow_instance", instanceData)

		if err := builder.AddFact(fact); err != nil {
			if strings.Contains(err.Error(), "exceeds maximum") {
				return nil
			}
			return err
		}
	}

	history, _, err := s.workflowRepo.FindHistoryByCase(ctx, params.OrganizationID, params.CaseID, 50, 0)
	if err != nil {
		builder.RecordAuthorization("workflow_history", nil, false, err.Error())
		return nil
	}

	var historyIDs []uuid.UUID
	for _, h := range history {
		historyIDs = append(historyIDs, h.ID)
	}
	builder.RecordAuthorization("workflow_history", historyIDs, len(history) > 0, fmt.Sprintf("found %d history entries", len(history)))

	for _, h := range history {
		historyData := map[string]any{
			"id":             h.ID.String(),
			"from_state":     h.FromState,
			"to_state":       h.ToState,
			"transition_key": h.TransitionKey,
			"actor_id":       uuidOrNil(h.ActorID),
			"occurred_at":    h.OccurredAt,
			"reason":         h.Reason,
			"metadata":       h.Metadata,
			"decision_id":    uuidOrNil(h.DecisionID),
		}

		fact := domain.NewFact(params.CaseID, domain.FactSourceWorkflowHistory, h.ID, domain.ProvenanceWorkflowHistory,
			"workflow_transition", historyData)

		if h.DecisionID != nil {
			fact = fact.WithReference(*h.DecisionID)
		}
		if h.ActorID != nil {
			fact = fact.WithReference(*h.ActorID)
		}

		if err := builder.AddFact(fact); err != nil {
			if strings.Contains(err.Error(), "exceeds maximum") {
				return nil
			}
			return err
		}
	}

	return nil
}

func (s *CaseContextService) buildDecisionFacts(ctx context.Context, builder *domain.ContextBuilder, params BuildContextParams) error {
	if !params.Options.IncludeDecisions {
		return nil
	}

	c, err := s.caseRepo.FindByID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		builder.RecordAuthorization("decision", nil, false, "case not found")
		return nil
	}

	decisions, total, err := s.decisionRepo.FindByServiceRequest(ctx, params.OrganizationID, c.ID, 50, 0)
	if err != nil {
		builder.RecordAuthorization("decision", nil, false, err.Error())
		return nil
	}

	var decisionIDs []uuid.UUID
	for _, d := range decisions {
		decisionIDs = append(decisionIDs, d.ID)
	}
	builder.RecordAuthorization("decision", decisionIDs, len(decisions) > 0, fmt.Sprintf("found %d decisions", total))

	for _, d := range decisions {
		canAccess, err := s.authChecker.CanAccessDecision(ctx, params.OrganizationID, d.ID, params.ActorID)
		if err != nil || !canAccess {
			continue
		}

		decisionData := map[string]any{
			"id":                    d.ID.String(),
			"decision":              d.Decision,
			"reason":                d.Reason,
			"decision_maker":        d.DecisionMaker.String(),
			"decided_at":            d.DecidedAt,
			"workflow_state":        d.WorkflowState,
			"rule_evaluation_ids":   uuidSliceToString(d.RuleEvaluationIDs),
			"evidence_ids":          uuidSliceToString(d.EvidenceIDs),
			"form_submission_id":    uuidOrNil(d.FormSubmissionID),
			"review_queue_entry_id": uuidOrNil(d.ReviewQueueEntryID),
			"version":               d.Version,
			"superseded_by_id":      uuidOrNil(d.SupersededByID),
		}

		fact := domain.NewFact(params.CaseID, domain.FactSourceDecision, d.ID, domain.ProvenanceHumanDecision,
			"decision", decisionData)

		for _, refID := range d.RuleEvaluationIDs {
			fact = fact.WithReference(refID)
		}
		for _, refID := range d.EvidenceIDs {
			fact = fact.WithReference(refID)
		}
		if d.FormSubmissionID != nil {
			fact = fact.WithReference(*d.FormSubmissionID)
		}
		if d.ReviewQueueEntryID != nil {
			fact = fact.WithReference(*d.ReviewQueueEntryID)
		}

		if err := builder.AddFact(fact); err != nil {
			if strings.Contains(err.Error(), "exceeds maximum") {
				return nil
			}
			return err
		}
	}

	return nil
}

func (s *CaseContextService) buildAIObservationFacts(ctx context.Context, builder *domain.ContextBuilder, params BuildContextParams) error {
	if !params.Options.IncludeAIObservations {
		return nil
	}

	evidenceList, _, err := s.evidenceRepo.FindByCase(ctx, params.OrganizationID, params.CaseID, 100, 0)
	if err != nil {
		return nil
	}

	for _, e := range evidenceList {
		observations, _, err := s.aiObsRepo.FindByEvidence(ctx, params.OrganizationID, e.ID, 20, 0)
		if err != nil || len(observations) == 0 {
			continue
		}

		for _, obs := range observations {
			if params.Options.OnlyHumanVerified && obs.Status != "ACCEPTED" {
				continue
			}

			obsData := map[string]any{
				"id":           obs.ID.String(),
				"evidence_id":  obs.EvidenceID.String(),
				"type":         obs.Type,
				"source":       obs.Source,
				"status":       obs.Status,
				"content":      obs.Content,
				"confidence":   obs.Confidence,
				"input_hash":   obs.InputHash,
				"output_hash":  obs.OutputHash,
				"created_at":   obs.CreatedAt,
				"created_by":   uuidOrNil(obs.CreatedBy),
				"reviewed_at":  obs.ReviewedAt,
				"reviewed_by":  uuidOrNil(obs.ReviewedBy),
				"review_notes": obs.ReviewNotes,
			}

			if obs.Model != nil {
				obsData["model"] = map[string]any{
					"name":     obs.Model.Name,
					"version":  obs.Model.Version,
					"provider": obs.Model.Provider,
				}
			}

			provenance := domain.ProvenanceAIObservation
			if obs.Status == "ACCEPTED" {
				provenance = domain.ProvenanceHumanVerified
			}

			fact := domain.NewFact(params.CaseID, domain.FactSourceAIObservation, obs.ID, provenance,
				"ai_observation", obsData)

			if obs.ReviewedBy != nil {
				fact = fact.WithReference(*obs.ReviewedBy)
			}

			if err := builder.AddFact(fact); err != nil {
				if strings.Contains(err.Error(), "exceeds maximum") {
					return nil
				}
				return err
			}
		}
	}

	return nil
}

func (s *CaseContextService) GetContext(ctx context.Context, orgID, contextID uuid.UUID) (*domain.CaseContext, error) {
	ctxObj, err := s.ctxRepo.FindByID(ctx, orgID, contextID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrContextNotFound, err)
	}
	return ctxObj, nil
}

func (s *CaseContextService) GetLatestContext(ctx context.Context, orgID, caseID uuid.UUID) (*domain.CaseContext, error) {
	ctxObj, err := s.ctxRepo.FindLatestByCase(ctx, orgID, caseID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrContextNotFound, err)
	}
	return ctxObj, nil
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func uuidOrNil(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func uuidSliceToString(ids []uuid.UUID) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = id.String()
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
