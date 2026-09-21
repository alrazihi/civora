package main

import (
	"context"
	"io"
	"time"

	aiinfrapostgres "github.com/alrazihi/civora/internal/ai/infrastructure/postgres"
	casecontextdomain "github.com/alrazihi/civora/internal/casecontext/domain"
	caseinfra "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	decisioninfra "github.com/alrazihi/civora/internal/decisions/infrastructure/postgres"
	evidenceinfra "github.com/alrazihi/civora/internal/evidence/infrastructure/postgres"
	formsubinfra "github.com/alrazihi/civora/internal/form_submission/infrastructure/postgres"
	peopleinfra "github.com/alrazihi/civora/internal/people/infrastructure/postgres"
	rulesinfra "github.com/alrazihi/civora/internal/rules/infrastructure/postgres"
	workflowinfra "github.com/alrazihi/civora/internal/workflow/infrastructure/postgres"
	"github.com/google/uuid"
)

type caseContextCaseRepoAdapter struct {
	inner *caseinfra.PostgresCaseRepository
}

func (a *caseContextCaseRepoAdapter) FindByID(ctx context.Context, orgID, id uuid.UUID) (*casecontextdomain.Case, error) {
	c, err := a.inner.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return &casecontextdomain.Case{
		ID:                 c.ID,
		OrganizationID:     c.OrganizationID,
		CaseNumber:         c.CaseNumber,
		Title:              c.Title,
		Description:        c.Description,
		Status:             string(c.Status),
		WorkflowState:      c.WorkflowState,
		ServiceType:        string(c.ServiceType),
		Priority:           string(c.Priority),
		PersonID:           c.PersonID,
		AssignedToID:       c.AssignedToID,
		CreatedByID:        c.CreatedByID,
		CreatedAt:          c.CreatedAt,
		UpdatedAt:          c.UpdatedAt,
		ClosedAt:           c.ClosedAt,
		WorkflowInstanceID: c.WorkflowInstanceID,
		WorkflowKey:        c.WorkflowKey,
		WorkflowID:         c.WorkflowID,
		Version:            c.Version,
	}, nil
}

type caseContextPersonRepoAdapter struct {
	inner *peopleinfra.PostgresPersonRepository
}

func (a *caseContextPersonRepoAdapter) FindByID(ctx context.Context, orgID, id uuid.UUID) (*casecontextdomain.Person, error) {
	p, err := a.inner.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	var dob *time.Time
	if p.DateOfBirth != nil {
		dob = p.DateOfBirth
	}
	var address, phone, email string
	if p.Address != nil {
		address = *p.Address
	}
	if p.Phone != nil {
		phone = *p.Phone
	}
	if p.Email != nil {
		email = *p.Email
	}
	return &casecontextdomain.Person{
		ID:             p.ID,
		OrganizationID: p.OrganizationID,
		LegalName:      p.FirstName + " " + p.LastName,
		DateOfBirth:    dob,
		Gender:         "",
		Nationality:    "",
		Address:        address,
		Phone:          phone,
		Email:          email,
		Metadata:       nil,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}, nil
}

type caseContextEvidenceRepoAdapter struct {
	inner *evidenceinfra.PostgresEvidenceRepository
}

func (a *caseContextEvidenceRepoAdapter) FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*casecontextdomain.Evidence, int, error) {
	items, err := a.inner.FindByServiceRequest(ctx, orgID, caseID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*casecontextdomain.Evidence, len(items))
	for i, e := range items {
		result[i] = &casecontextdomain.Evidence{
			ID:                 e.ID,
			OrganizationID:     e.OrganizationID,
			ServiceRequestID:   e.ServiceRequestID,
			Type:               string(e.Type),
			CustomType:         e.CustomType,
			Description:        e.Description,
			StorageReference:   e.StorageReference,
			UploadedBy:         e.UploadedBy,
			PersonID:           e.PersonID,
			Source:             string(e.Source),
			Metadata:           e.Metadata,
			VerificationStatus: string(e.VerificationStatus),
			VerifiedBy:         e.VerifiedBy,
			VerifiedAt:         e.VerifiedAt,
			VerificationReason: e.VerificationReason,
			VerificationMethod: e.VerificationMethod,
			CreatedAt:          e.CreatedAt,
		}
	}
	return result, len(result), nil
}

func (a *caseContextEvidenceRepoAdapter) FindByID(ctx context.Context, orgID, id uuid.UUID) (*casecontextdomain.Evidence, error) {
	e, err := a.inner.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return &casecontextdomain.Evidence{
		ID:                 e.ID,
		OrganizationID:     e.OrganizationID,
		ServiceRequestID:   e.ServiceRequestID,
		Type:               string(e.Type),
		CustomType:         e.CustomType,
		Description:        e.Description,
		StorageReference:   e.StorageReference,
		UploadedBy:         e.UploadedBy,
		PersonID:           e.PersonID,
		Source:             string(e.Source),
		Metadata:           e.Metadata,
		VerificationStatus: string(e.VerificationStatus),
		VerifiedBy:         e.VerifiedBy,
		VerifiedAt:         e.VerifiedAt,
		VerificationReason: e.VerificationReason,
		VerificationMethod: e.VerificationMethod,
		CreatedAt:          e.CreatedAt,
	}, nil
}

func (a *caseContextEvidenceRepoAdapter) ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*casecontextdomain.Document, int, error) {
	docs, total, err := a.inner.ListDocuments(ctx, orgID, evidenceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*casecontextdomain.Document, len(docs))
	for i, d := range docs {
		result[i] = &casecontextdomain.Document{
			ID:              d.ID,
			EvidenceID:      d.EvidenceID,
			OrganizationID:  d.OrganizationID,
			FileName:        d.FileName,
			ContentType:     d.ContentType,
			SizeBytes:       d.SizeBytes,
			Checksum:        d.Checksum,
			StorageKey:      d.StorageKey,
			StorageProvider: string(d.StorageProvider),
			UploadedBy:      d.UploadedBy,
			UploadedAt:      d.UploadedAt,
			CreatedAt:       d.CreatedAt,
		}
	}
	return result, total, nil
}

type caseContextFormSubmissionRepoAdapter struct {
	inner *formsubinfra.PostgresFormSubmissionRepository
}

func (a *caseContextFormSubmissionRepoAdapter) ListByCase(ctx context.Context, orgID, caseID uuid.UUID) ([]*casecontextdomain.FormSubmission, error) {
	items, err := a.inner.ListByCase(ctx, orgID, caseID)
	if err != nil {
		return nil, err
	}
	result := make([]*casecontextdomain.FormSubmission, len(items))
	for i, s := range items {
		result[i] = &casecontextdomain.FormSubmission{
			ID:            s.ID,
			TenantID:      s.TenantID,
			CaseID:        s.CaseID,
			FormID:        s.FormID,
			FormVersionID: s.FormVersionID,
			SubmittedBy:   s.SubmittedBy,
			Status:        string(s.Status),
			Data:          s.Data,
			SubmittedAt:   s.SubmittedAt,
			UpdatedAt:     s.UpdatedAt,
		}
	}
	return result, nil
}

func (a *caseContextFormSubmissionRepoAdapter) FindByID(ctx context.Context, orgID, id uuid.UUID) (*casecontextdomain.FormSubmission, error) {
	return nil, nil
}

type caseContextRuleRepoAdapter struct {
	inner *rulesinfra.PostgresEvaluationRepository
}

func (a *caseContextRuleRepoAdapter) FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*casecontextdomain.RuleEvaluation, int, error) {
	items, total, err := a.inner.ListByCase(ctx, orgID, caseID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*casecontextdomain.RuleEvaluation, len(items))
	for i, e := range items {
		var caseIDVal uuid.UUID
		if e.CaseID != nil {
			caseIDVal = *e.CaseID
		}
		result[i] = &casecontextdomain.RuleEvaluation{
			ID:             e.ID,
			RuleSetID:      e.RuleSetID,
			RuleSetVersion: e.RuleSetVersion,
			OrganizationID: e.OrganizationID,
			CaseID:         caseIDVal,
			Status:         string(e.Status),
			Outcome:        string(e.Outcome),
			Reason:         e.Reason,
			MatchedRuleID:  e.MatchedRuleID,
			Trigger:        string(e.Trigger),
			EvaluatedBy:    e.EvaluatedBy,
			EvaluatedAt:    e.EvaluatedAt,
			FactsSnapshot:  e.FactsSnapshot,
		}
	}
	return result, total, nil
}

func (a *caseContextRuleRepoAdapter) FindByID(ctx context.Context, orgID, id uuid.UUID) (*casecontextdomain.RuleEvaluation, error) {
	return nil, nil
}

type caseContextWorkflowRepoAdapter struct {
	innerInstance *workflowinfra.PostgresWorkflowInstanceRepository
	innerHistory  *workflowinfra.PostgresWorkflowTransitionHistoryRepository
}

func (a *caseContextWorkflowRepoAdapter) FindInstanceByCase(ctx context.Context, orgID, caseID uuid.UUID) (*casecontextdomain.WorkflowInstance, error) {
	inst, err := a.innerInstance.FindByCaseID(ctx, orgID, caseID)
	if err != nil {
		return nil, err
	}
	return &casecontextdomain.WorkflowInstance{
		ID:                 inst.ID,
		TenantID:           inst.TenantID,
		WorkflowDefID:      inst.WorkflowDefID,
		WorkflowDefVersion: inst.WorkflowDefVersion,
		CaseID:             inst.CaseID,
		CurrentState:       inst.CurrentState,
		StartedAt:          inst.StartedAt,
		CompletedAt:        inst.CompletedAt,
		Metadata:           inst.Metadata,
		Version:            inst.Version,
	}, nil
}

func (a *caseContextWorkflowRepoAdapter) FindHistoryByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*casecontextdomain.WorkflowTransitionHistory, int, error) {
	items, err := a.innerHistory.FindByCaseID(ctx, orgID, caseID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*casecontextdomain.WorkflowTransitionHistory, len(items))
	for i, h := range items {
		result[i] = &casecontextdomain.WorkflowTransitionHistory{
			ID:                 h.ID,
			TenantID:           h.TenantID,
			WorkflowInstanceID: h.WorkflowInstanceID,
			CaseID:             h.CaseID,
			FromState:          h.FromState,
			ToState:            h.ToState,
			TransitionKey:      h.TransitionKey,
			ActorID:            h.ActorID,
			OccurredAt:         h.OccurredAt,
			Reason:             h.Reason,
			Metadata:           h.Metadata,
			DecisionID:         h.DecisionID,
		}
	}
	return result, len(result), nil
}

type caseContextDecisionRepoAdapter struct {
	inner *decisioninfra.PostgresDecisionRepository
}

func (a *caseContextDecisionRepoAdapter) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*casecontextdomain.Decision, int, error) {
	d, err := a.inner.FindByServiceRequest(ctx, orgID, serviceRequestID)
	if err != nil {
		return nil, 0, err
	}
	if d == nil {
		return nil, 0, nil
	}
	decision := &casecontextdomain.Decision{
		ID:                 d.ID,
		OrganizationID:     d.OrganizationID,
		ServiceRequestID:   d.ServiceRequestID,
		Decision:           string(d.Decision),
		Reason:             d.Reason,
		DecisionMaker:      d.DecisionMaker,
		DecidedAt:          d.DecidedAt,
		CreatedAt:          d.CreatedAt,
		SupersededByID:     d.SupersededByID,
		WorkflowState:      d.WorkflowState,
		RuleEvaluationIDs:  d.RuleEvaluationIDs,
		EvidenceIDs:        d.EvidenceIDs,
		FormSubmissionID:   d.FormSubmissionID,
		ReviewQueueEntryID: d.ReviewQueueEntryID,
		Version:            d.Version,
	}
	return []*casecontextdomain.Decision{decision}, 1, nil
}

func (a *caseContextDecisionRepoAdapter) FindByID(ctx context.Context, orgID, id uuid.UUID) (*casecontextdomain.Decision, error) {
	return nil, nil
}

type caseContextAIObsRepoAdapter struct {
	inner *aiinfrapostgres.PostgresObservationRepository
}

func (a *caseContextAIObsRepoAdapter) FindByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*casecontextdomain.AIObservation, int, error) {
	items, total, err := a.inner.FindByEvidence(ctx, orgID, evidenceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*casecontextdomain.AIObservation, len(items))
	for i, o := range items {
		var model *casecontextdomain.ModelInfo
		if o.Model != nil {
			model = &casecontextdomain.ModelInfo{
				Name:     o.Model.Name,
				Version:  o.Model.Version,
				Provider: o.Model.Provider,
			}
		}
		var evidenceIDVal uuid.UUID
		if o.EvidenceID != nil {
			evidenceIDVal = *o.EvidenceID
		}
		result[i] = &casecontextdomain.AIObservation{
			ID:             o.ID,
			OrganizationID: o.OrganizationID,
			EvidenceID:     evidenceIDVal,
			Type:           string(o.Type),
			Source:         string(o.Source),
			Status:         string(o.Status),
			Model:          model,
			Content:        o.Content,
			Confidence:     o.Confidence,
			InputHash:      o.InputHash,
			OutputHash:     o.OutputHash,
			CreatedAt:      o.CreatedAt,
			CreatedBy:      o.CreatedBy,
			ReviewedAt:     o.ReviewedAt,
			ReviewedBy:     o.ReviewedBy,
			ReviewNotes:    o.ReviewNotes,
		}
	}
	return result, total, nil
}

type caseContextDocumentContentProviderAdapter struct {
	inner func(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) (io.ReadCloser, error)
}

func (a *caseContextDocumentContentProviderAdapter) GetDocumentContent(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) ([]byte, error) {
	rc, err := a.inner(ctx, orgID, evidenceID, documentID, actorID)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

type documentIntelligenceDocumentContentProviderAdapter struct {
	inner func(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) (io.ReadCloser, error)
}

func (a *documentIntelligenceDocumentContentProviderAdapter) GetDocumentContent(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) ([]byte, error) {
	rc, err := a.inner(ctx, orgID, evidenceID, documentID, actorID)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
