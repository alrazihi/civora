package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFact(t *testing.T) {
	caseID := uuid.New()
	sourceID := uuid.New()

	fact := NewFact(caseID, FactSourceCaseMetadata, sourceID, ProvenanceSourceFact, "test_key", "test_value")

	require.NotNil(t, fact)
	assert.Equal(t, caseID, fact.CaseID)
	assert.Equal(t, FactSourceCaseMetadata, fact.SourceType)
	assert.Equal(t, sourceID, fact.SourceID)
	assert.Equal(t, ProvenanceSourceFact, fact.Provenance)
	assert.Equal(t, "test_key", fact.Key)
	assert.Equal(t, "test_value", fact.Value)
	assert.Equal(t, "string", fact.ValueType)
	assert.NotNil(t, fact.RetrievedAt)
	assert.Nil(t, fact.ExpiresAt)
}

func TestNewFactWithConfidence(t *testing.T) {
	caseID := uuid.New()
	sourceID := uuid.New()

	fact := NewFactWithConfidence(caseID, FactSourceEvidence, sourceID, ProvenanceHumanVerified, "verified", true, 0.95)

	require.NotNil(t, fact)
	assert.NotNil(t, fact.Confidence)
	assert.Equal(t, 0.95, *fact.Confidence)
}

func TestFactWithExpiration(t *testing.T) {
	caseID := uuid.New()
	sourceID := uuid.New()
	expiresAt := time.Now().UTC().Add(1 * time.Hour)

	fact := NewFact(caseID, FactSourceDocument, sourceID, ProvenanceSourceFact, "doc", "content")
	fact.WithExpiration(expiresAt)

	require.NotNil(t, fact.ExpiresAt)
	assert.Equal(t, expiresAt, *fact.ExpiresAt)
}

func TestFactWithMetadata(t *testing.T) {
	caseID := uuid.New()
	sourceID := uuid.New()

	fact := NewFact(caseID, FactSourcePerson, sourceID, ProvenanceSourceFact, "person", map[string]any{"name": "John"})
	fact.WithMetadata("source", "manual")

	require.NotNil(t, fact.Metadata)
	assert.Equal(t, "manual", fact.Metadata["source"])
}

func TestFactWithReference(t *testing.T) {
	caseID := uuid.New()
	sourceID := uuid.New()
	refID := uuid.New()

	fact := NewFact(caseID, FactSourceDecision, sourceID, ProvenanceHumanDecision, "decision", "approved")
	fact.WithReference(refID)

	require.Len(t, fact.References, 1)
	assert.Equal(t, refID, fact.References[0])
}

func TestValidateFact(t *testing.T) {
	tests := []struct {
		name    string
		fact    *Fact
		wantErr bool
	}{
		{
			name:    "valid fact",
			fact:    NewFact(uuid.New(), FactSourceCaseMetadata, uuid.New(), ProvenanceSourceFact, "key", "value"),
			wantErr: false,
		},
		{
			name:    "nil case ID",
			fact:    &Fact{CaseID: uuid.Nil, SourceID: uuid.New(), SourceType: FactSourceCaseMetadata, Provenance: ProvenanceSourceFact, Key: "key"},
			wantErr: true,
		},
		{
			name:    "nil source ID",
			fact:    &Fact{CaseID: uuid.New(), SourceID: uuid.Nil, SourceType: FactSourceCaseMetadata, Provenance: ProvenanceSourceFact, Key: "key"},
			wantErr: true,
		},
		{
			name:    "invalid source type",
			fact:    &Fact{CaseID: uuid.New(), SourceID: uuid.New(), SourceType: "INVALID", Provenance: ProvenanceSourceFact, Key: "key"},
			wantErr: true,
		},
		{
			name:    "invalid provenance",
			fact:    &Fact{CaseID: uuid.New(), SourceID: uuid.New(), SourceType: FactSourceCaseMetadata, Provenance: "INVALID", Key: "key"},
			wantErr: true,
		},
		{
			name:    "empty key",
			fact:    &Fact{CaseID: uuid.New(), SourceID: uuid.New(), SourceType: FactSourceCaseMetadata, Provenance: ProvenanceSourceFact, Key: ""},
			wantErr: true,
		},
		{
			name:    "key too long",
			fact:    &Fact{CaseID: uuid.New(), SourceID: uuid.New(), SourceType: FactSourceCaseMetadata, Provenance: ProvenanceSourceFact, Key: string(make([]byte, 201))},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFact(tt.fact)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContextBuilder(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()
	options := DefaultContextOptions()

	builder := NewContextBuilder(orgID, caseID, actorID, options)

	assert.Equal(t, orgID, builder.orgID)
	assert.Equal(t, caseID, builder.caseID)
	assert.Equal(t, actorID, builder.actorID)
	assert.Equal(t, options.MaxTokens, builder.options.MaxTokens)
	assert.Equal(t, options.MaxFacts, builder.options.MaxFacts)
}

func TestContextBuilderAddFact(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()
	options := DefaultContextOptions()
	options.MaxFacts = 2
	options.MaxTokens = 100

	builder := NewContextBuilder(orgID, caseID, actorID, options)

	fact1 := NewFact(caseID, FactSourceCaseMetadata, uuid.New(), ProvenanceSourceFact, "key1", "value1")
	err := builder.AddFact(fact1)
	assert.NoError(t, err)
	assert.Equal(t, 1, builder.factCount)

	fact2 := NewFact(caseID, FactSourceEvidence, uuid.New(), ProvenanceSourceFact, "key2", "value2")
	err = builder.AddFact(fact2)
	assert.NoError(t, err)
	assert.Equal(t, 2, builder.factCount)

	fact3 := NewFact(caseID, FactSourceDocument, uuid.New(), ProvenanceSourceFact, "key3", "value3")
	err = builder.AddFact(fact3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum facts")
}

func TestContextBuilderTokenLimit(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()
	options := DefaultContextOptions()
	options.MaxTokens = 50

	builder := NewContextBuilder(orgID, caseID, actorID, options)

	largeValue := string(make([]byte, 300))
	fact := NewFact(caseID, FactSourceCaseMetadata, uuid.New(), ProvenanceSourceFact, "large", largeValue)
	err := builder.AddFact(fact)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token estimate")
}

func TestContextBuilderBuild(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()
	options := DefaultContextOptions()

	builder := NewContextBuilder(orgID, caseID, actorID, options)

	fact := NewFact(caseID, FactSourceCaseMetadata, uuid.New(), ProvenanceSourceFact, "key", "value")
	err := builder.AddFact(fact)
	require.NoError(t, err)

	ctx, err := builder.Build()
	require.NoError(t, err)

	assert.NotNil(t, ctx)
	assert.Equal(t, orgID, ctx.OrganizationID)
	assert.Equal(t, caseID, ctx.CaseID)
	assert.Equal(t, actorID, ctx.ActorID)
	assert.Len(t, ctx.Facts, 1)
	assert.Greater(t, ctx.TokenEstimate, 0)
	assert.False(t, ctx.CreatedAt.IsZero())
	assert.False(t, ctx.ExpiresAt.IsZero())
	assert.NotNil(t, ctx.Authorization)
}

func TestContextBuilderBuildEmpty(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()
	options := DefaultContextOptions()

	builder := NewContextBuilder(orgID, caseID, actorID, options)

	ctx, err := builder.Build()
	assert.Error(t, err)
	assert.Equal(t, ErrInsufficientContext, err)
	assert.Nil(t, ctx)
}

func TestContextBuilderRecordAuthorization(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	actorID := uuid.New()
	options := DefaultContextOptions()

	builder := NewContextBuilder(orgID, caseID, actorID, options)

	resourceIDs := []uuid.UUID{uuid.New(), uuid.New()}
	builder.RecordAuthorization("case", resourceIDs, true, "access granted")

	auth, ok := builder.authMap["case"]
	require.True(t, ok)
	assert.Equal(t, "case", auth.ResourceType)
	assert.Len(t, auth.ResourceIDs, 2)
	assert.True(t, auth.Granted)
	assert.Equal(t, "access granted", auth.Reason)
}

func TestInferValueType(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"nil", nil, "null"},
		{"string", "hello", "string"},
		{"bool", true, "boolean"},
		{"int", 42, "integer"},
		{"int64", int64(42), "integer"},
		{"float64", 3.14, "number"},
		{"array", []interface{}{1, 2, 3}, "array"},
		{"object", map[string]interface{}{"a": 1}, "object"},
		{"time", time.Now(), "datetime"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferValueType(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidProvenance(t *testing.T) {
	valid := []FactProvenance{
		ProvenanceSourceFact,
		ProvenanceAIObservation,
		ProvenanceHumanVerified,
		ProvenanceRuleResult,
		ProvenanceHumanDecision,
		ProvenanceFormSubmission,
		ProvenanceWorkflowHistory,
	}

	for _, p := range valid {
		assert.True(t, isValidProvenance(p), "provenance %s should be valid", p)
	}

	assert.False(t, isValidProvenance("INVALID"))
}

func TestIsValidSourceType(t *testing.T) {
	valid := []FactSourceType{
		FactSourceCaseMetadata,
		FactSourcePerson,
		FactSourceEvidence,
		FactSourceDocument,
		FactSourceFormSubmission,
		FactSourceRuleEvaluation,
		FactSourceWorkflowInstance,
		FactSourceWorkflowHistory,
		FactSourceDecision,
		FactSourceAIObservation,
	}

	for _, s := range valid {
		assert.True(t, isValidSourceType(s), "source type %s should be valid", s)
	}

	assert.False(t, isValidSourceType("INVALID"))
}

func TestDefaultContextOptions(t *testing.T) {
	options := DefaultContextOptions()

	assert.True(t, options.IncludeCaseMetadata)
	assert.True(t, options.IncludePerson)
	assert.True(t, options.IncludeEvidence)
	assert.True(t, options.IncludeVerifiedEvidence)
	assert.True(t, options.IncludeDocuments)
	assert.True(t, options.IncludeFormSubmissions)
	assert.True(t, options.IncludeRuleEvaluations)
	assert.True(t, options.IncludeWorkflowHistory)
	assert.True(t, options.IncludeDecisions)
	assert.True(t, options.IncludeAIObservations)
	assert.Equal(t, 8000, options.MaxTokens)
	assert.Equal(t, 200, options.MaxFacts)
	assert.False(t, options.OnlyVerified)
	assert.False(t, options.OnlyHumanVerified)
}

func TestProvenanceConstants(t *testing.T) {
	assert.Equal(t, "SOURCE_FACT", string(ProvenanceSourceFact))
	assert.Equal(t, "AI_OBSERVATION", string(ProvenanceAIObservation))
	assert.Equal(t, "HUMAN_VERIFIED_FACT", string(ProvenanceHumanVerified))
	assert.Equal(t, "RULE_RESULT", string(ProvenanceRuleResult))
	assert.Equal(t, "HUMAN_DECISION", string(ProvenanceHumanDecision))
	assert.Equal(t, "FORM_SUBMISSION", string(ProvenanceFormSubmission))
	assert.Equal(t, "WORKFLOW_HISTORY", string(ProvenanceWorkflowHistory))
}

func TestFactSourceTypeConstants(t *testing.T) {
	assert.Equal(t, "CASE_METADATA", string(FactSourceCaseMetadata))
	assert.Equal(t, "PERSON", string(FactSourcePerson))
	assert.Equal(t, "EVIDENCE", string(FactSourceEvidence))
	assert.Equal(t, "DOCUMENT", string(FactSourceDocument))
	assert.Equal(t, "FORM_SUBMISSION", string(FactSourceFormSubmission))
	assert.Equal(t, "RULE_EVALUATION", string(FactSourceRuleEvaluation))
	assert.Equal(t, "WORKFLOW_INSTANCE", string(FactSourceWorkflowInstance))
	assert.Equal(t, "WORKFLOW_HISTORY", string(FactSourceWorkflowHistory))
	assert.Equal(t, "DECISION", string(FactSourceDecision))
	assert.Equal(t, "AI_OBSERVATION", string(FactSourceAIObservation))
}
