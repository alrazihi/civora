package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrContextNotFound      = errors.New("case context not found")
	ErrContextInvalidInput  = errors.New("invalid case context input")
	ErrAuthorizationFailed  = errors.New("authorization failed")
	ErrTenantViolation      = errors.New("tenant violation: resource belongs to different organization")
	ErrCaseAccessDenied     = errors.New("access to case denied")
	ErrEvidenceAccessDenied = errors.New("access to evidence denied")
	ErrDocumentAccessDenied = errors.New("access to document denied")
	ErrPersonAccessDenied   = errors.New("access to person information denied")
	ErrInsufficientContext  = errors.New("insufficient information to build context")
	ErrContextSizeExceeded  = errors.New("context size exceeds maximum allowed")
)

type FactProvenance string

const (
	ProvenanceSourceFact      FactProvenance = "SOURCE_FACT"
	ProvenanceAIObservation   FactProvenance = "AI_OBSERVATION"
	ProvenanceHumanVerified   FactProvenance = "HUMAN_VERIFIED_FACT"
	ProvenanceRuleResult      FactProvenance = "RULE_RESULT"
	ProvenanceHumanDecision   FactProvenance = "HUMAN_DECISION"
	ProvenanceFormSubmission  FactProvenance = "FORM_SUBMISSION"
	ProvenanceWorkflowHistory FactProvenance = "WORKFLOW_HISTORY"
)

type FactSourceType string

const (
	FactSourceCaseMetadata     FactSourceType = "CASE_METADATA"
	FactSourcePerson           FactSourceType = "PERSON"
	FactSourceEvidence         FactSourceType = "EVIDENCE"
	FactSourceDocument         FactSourceType = "DOCUMENT"
	FactSourceFormSubmission   FactSourceType = "FORM_SUBMISSION"
	FactSourceRuleEvaluation   FactSourceType = "RULE_EVALUATION"
	FactSourceWorkflowInstance FactSourceType = "WORKFLOW_INSTANCE"
	FactSourceWorkflowHistory  FactSourceType = "WORKFLOW_HISTORY"
	FactSourceDecision         FactSourceType = "DECISION"
	FactSourceAIObservation    FactSourceType = "AI_OBSERVATION"
)

type Fact struct {
	ID          uuid.UUID      `json:"id"`
	CaseID      uuid.UUID      `json:"case_id"`
	SourceType  FactSourceType `json:"source_type"`
	SourceID    uuid.UUID      `json:"source_id"`
	Provenance  FactProvenance `json:"provenance"`
	Key         string         `json:"key"`
	Value       interface{}    `json:"value"`
	ValueType   string         `json:"value_type"`
	Confidence  *float64       `json:"confidence,omitempty"`
	RetrievedAt time.Time      `json:"retrieved_at"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	References  []uuid.UUID    `json:"references,omitempty"`
}

type CaseContext struct {
	ID             uuid.UUID        `json:"id"`
	OrganizationID uuid.UUID        `json:"organization_id"`
	CaseID         uuid.UUID        `json:"case_id"`
	ActorID        uuid.UUID        `json:"actor_id"`
	Facts          []*Fact          `json:"facts"`
	TokenEstimate  int              `json:"token_estimate"`
	CreatedAt      time.Time        `json:"created_at"`
	ExpiresAt      time.Time        `json:"expires_at"`
	Authorization  AuthorizationMap `json:"authorization"`
}

type AuthorizationMap map[string]AuthorizationEntry

type AuthorizationEntry struct {
	ResourceType string   `json:"resource_type"`
	ResourceIDs  []string `json:"resource_ids"`
	Granted      bool     `json:"granted"`
	Reason       string   `json:"reason,omitempty"`
}

type ContextOptions struct {
	IncludeCaseMetadata     bool
	IncludePerson           bool
	IncludeEvidence         bool
	IncludeVerifiedEvidence bool
	IncludeDocuments        bool
	IncludeFormSubmissions  bool
	IncludeRuleEvaluations  bool
	IncludeWorkflowHistory  bool
	IncludeDecisions        bool
	IncludeAIObservations   bool
	MaxTokens               int
	MaxFacts                int
	OnlyVerified            bool
	OnlyHumanVerified       bool
}

func DefaultContextOptions() ContextOptions {
	return ContextOptions{
		IncludeCaseMetadata:     true,
		IncludePerson:           true,
		IncludeEvidence:         true,
		IncludeVerifiedEvidence: true,
		IncludeDocuments:        true,
		IncludeFormSubmissions:  true,
		IncludeRuleEvaluations:  true,
		IncludeWorkflowHistory:  true,
		IncludeDecisions:        true,
		IncludeAIObservations:   true,
		MaxTokens:               8000,
		MaxFacts:                200,
		OnlyVerified:            false,
		OnlyHumanVerified:       false,
	}
}

func NewFact(caseID uuid.UUID, sourceType FactSourceType, sourceID uuid.UUID, provenance FactProvenance, key string, value interface{}) *Fact {
	valueType := inferValueType(value)
	return &Fact{
		ID:          uuid.New(),
		CaseID:      caseID,
		SourceType:  sourceType,
		SourceID:    sourceID,
		Provenance:  provenance,
		Key:         key,
		Value:       value,
		ValueType:   valueType,
		RetrievedAt: time.Now().UTC(),
	}
}

func NewFactWithConfidence(caseID uuid.UUID, sourceType FactSourceType, sourceID uuid.UUID, provenance FactProvenance, key string, value interface{}, confidence float64) *Fact {
	f := NewFact(caseID, sourceType, sourceID, provenance, key, value)
	f.Confidence = &confidence
	return f
}

func (f *Fact) WithExpiration(expiresAt time.Time) *Fact {
	f.ExpiresAt = &expiresAt
	return f
}

func (f *Fact) WithMetadata(key string, value any) *Fact {
	if f.Metadata == nil {
		f.Metadata = make(map[string]any)
	}
	f.Metadata[key] = value
	return f
}

func (f *Fact) WithReference(refID uuid.UUID) *Fact {
	f.References = append(f.References, refID)
	return f
}

func inferValueType(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case time.Time:
		return "datetime"
	default:
		return "unknown"
	}
}

type ContextBuilder struct {
	orgID      uuid.UUID
	caseID     uuid.UUID
	actorID    uuid.UUID
	options    ContextOptions
	facts      []*Fact
	authMap    AuthorizationMap
	tokenCount int
	factCount  int
}

func NewContextBuilder(orgID, caseID, actorID uuid.UUID, options ContextOptions) *ContextBuilder {
	if options.MaxTokens <= 0 {
		options.MaxTokens = 8000
	}
	if options.MaxFacts <= 0 {
		options.MaxFacts = 200
	}
	return &ContextBuilder{
		orgID:      orgID,
		caseID:     caseID,
		actorID:    actorID,
		options:    options,
		facts:      make([]*Fact, 0),
		authMap:    make(AuthorizationMap),
		tokenCount: 0,
		factCount:  0,
	}
}

func (b *ContextBuilder) AddFact(fact *Fact) error {
	if b.factCount >= b.options.MaxFacts {
		return fmt.Errorf("%w: maximum facts (%d) exceeded", ErrContextSizeExceeded, b.options.MaxFacts)
	}
	estTokens := estimateTokens(fact)
	if b.tokenCount+estTokens > b.options.MaxTokens {
		return fmt.Errorf("%w: token estimate (%d) would exceed maximum (%d)", ErrContextSizeExceeded, b.tokenCount+estTokens, b.options.MaxTokens)
	}
	b.facts = append(b.facts, fact)
	b.tokenCount += estTokens
	b.factCount++
	return nil
}

func (b *ContextBuilder) RecordAuthorization(resourceType string, resourceIDs []uuid.UUID, granted bool, reason string) {
	ids := make([]string, len(resourceIDs))
	for i, id := range resourceIDs {
		ids[i] = id.String()
	}
	b.authMap[resourceType] = AuthorizationEntry{
		ResourceType: resourceType,
		ResourceIDs:  ids,
		Granted:      granted,
		Reason:       reason,
	}
}

func (b *ContextBuilder) Build() (*CaseContext, error) {
	if len(b.facts) == 0 {
		return nil, ErrInsufficientContext
	}

	now := time.Now().UTC()
	ctx := &CaseContext{
		ID:             uuid.New(),
		OrganizationID: b.orgID,
		CaseID:         b.caseID,
		ActorID:        b.actorID,
		Facts:          b.facts,
		TokenEstimate:  b.tokenCount,
		CreatedAt:      now,
		ExpiresAt:      now.Add(1 * time.Hour),
		Authorization:  b.authMap,
	}
	return ctx, nil
}

func estimateTokens(fact *Fact) int {
	tokens := len(fact.Key) / 4
	switch v := fact.Value.(type) {
	case string:
		tokens += len(v) / 4
	case []byte:
		tokens += len(v) / 4
	case map[string]interface{}:
		for k, val := range v {
			tokens += len(k) / 4
			tokens += estimateValueTokens(val)
		}
	case []interface{}:
		for _, val := range v {
			tokens += estimateValueTokens(val)
		}
	default:
		tokens += 10
	}
	return tokens
}

func estimateValueTokens(v interface{}) int {
	if v == nil {
		return 1
	}
	switch val := v.(type) {
	case string:
		return len(val) / 4
	case []byte:
		return len(val) / 4
	case map[string]interface{}:
		tokens := 0
		for k, v := range val {
			tokens += len(k) / 4
			tokens += estimateValueTokens(v)
		}
		return tokens
	case []interface{}:
		tokens := 0
		for _, item := range val {
			tokens += estimateValueTokens(item)
		}
		return tokens
	default:
		return 10
	}
}

func isValidProvenance(p FactProvenance) bool {
	switch p {
	case ProvenanceSourceFact, ProvenanceAIObservation, ProvenanceHumanVerified,
		ProvenanceRuleResult, ProvenanceHumanDecision, ProvenanceFormSubmission,
		ProvenanceWorkflowHistory:
		return true
	default:
		return false
	}
}

func isValidSourceType(s FactSourceType) bool {
	switch s {
	case FactSourceCaseMetadata, FactSourcePerson, FactSourceEvidence, FactSourceDocument,
		FactSourceFormSubmission, FactSourceRuleEvaluation, FactSourceWorkflowInstance,
		FactSourceWorkflowHistory, FactSourceDecision, FactSourceAIObservation:
		return true
	default:
		return false
	}
}

func ValidateFact(f *Fact) error {
	if f.CaseID == uuid.Nil {
		return fmt.Errorf("%w: case ID is required", ErrContextInvalidInput)
	}
	if f.SourceID == uuid.Nil {
		return fmt.Errorf("%w: source ID is required", ErrContextInvalidInput)
	}
	if !isValidSourceType(f.SourceType) {
		return fmt.Errorf("%w: invalid source type %q", ErrContextInvalidInput, f.SourceType)
	}
	if !isValidProvenance(f.Provenance) {
		return fmt.Errorf("%w: invalid provenance %q", ErrContextInvalidInput, f.Provenance)
	}
	if f.Key == "" {
		return fmt.Errorf("%w: fact key is required", ErrContextInvalidInput)
	}
	if len(f.Key) > 200 {
		return fmt.Errorf("%w: fact key exceeds maximum length", ErrContextInvalidInput)
	}
	return nil
}
