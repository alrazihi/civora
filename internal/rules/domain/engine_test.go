package domain

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeJSON[T any](t *testing.T, raw string, v *T) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.UseNumber()
	require.NoError(t, dec.Decode(v))
}

func decodeCondition(t *testing.T, raw string) Condition {
	t.Helper()
	var c Condition
	decodeJSON(t, raw, &c)
	return c
}

func parseRuleSetJSON(raw string) (*RuleSet, error) {
	var rs struct {
		Key            string   `json:"key"`
		Name           string   `json:"name"`
		Description    string   `json:"description"`
		Version        int      `json:"version"`
		Status         string   `json:"status"`
		DefaultOutcome string   `json:"default_outcome"`
		Rules          []Rule   `json:"rules"`
		Triggers       []string `json:"triggers"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.UseNumber()
	if err := dec.Decode(&rs); err != nil {
		return nil, err
	}
	outcome := Outcome(rs.DefaultOutcome)
	status := RuleSetStatus(rs.Status)
	if rs.Version < 1 {
		rs.Version = 1
	}
	rules := make([]Rule, len(rs.Rules))
	for i, r := range rs.Rules {
		rules[i] = Rule{
			ID:         uuid.New(),
			Priority:   r.Priority,
			Outcome:    r.Outcome,
			Conditions: r.Conditions,
			Active:     true,
		}
	}
	return &RuleSet{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Key:            rs.Key,
		Name:           rs.Name,
		Description:    rs.Description,
		Version:        rs.Version,
		Status:         status,
		DefaultOutcome: outcome,
		Rules:          rules,
		Triggers:       rs.Triggers,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}, nil
}

func factsFrom(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var f map[string]interface{}
	decodeJSON(t, raw, &f)
	return f
}

func evalRules(t *testing.T, jsonStr string, facts map[string]interface{}) *Evaluation {
	t.Helper()
	rs, err := parseRuleSetJSON(jsonStr)
	require.NoError(t, err)
	return Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
}

func TestEvaluate_LeafEqualString(t *testing.T) {
	facts := factsFrom(t, `{"housing_status":"HOMELESS"}`)
	ev := evalRules(t, `{"key":"r","name":"r","version":1,"status":"PUBLISHED","default_outcome":"INELIGIBLE","rules":[{"priority":0,"outcome":"ELIGIBLE","conditions":{"field":"housing_status","operator":"eq","value":"HOMELESS"}}]}`, facts)
	assert.Equal(t, StatusEligible, ev.Status)
}

func TestEvaluate_LeafEqualNumber(t *testing.T) {
	facts := factsFrom(t, `{"income":500}`)
	ev := evalRules(t, `{"key":"r","name":"r","version":1,"status":"PUBLISHED","default_outcome":"INELIGIBLE","rules":[{"priority":0,"outcome":"ELIGIBLE","conditions":{"field":"income","operator":"eq","value":500}}]}`, facts)
	assert.Equal(t, StatusEligible, ev.Status)
}

func TestEvaluate_LeafNumericOperators(t *testing.T) {
	facts := factsFrom(t, `{"income":500}`)
	tests := []struct {
		op    Operator
		value json.Number
		want  EvaluationStatus
	}{
		{OpLessThanEqual, json.Number("500"), StatusEligible},
		{OpLessThan, json.Number("501"), StatusEligible},
		{OpGreaterThan, json.Number("499"), StatusEligible},
		{OpGreaterThanEqual, json.Number("500"), StatusEligible},
		{OpLessThan, json.Number("500"), StatusIneligible},
		{OpGreaterThan, json.Number("500"), StatusIneligible},
	}
	for _, tt := range tests {
		cond := Condition{Field: "income", Operator: tt.op, Value: tt.value}
		rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: cond}}}
		ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
		assert.Equal(t, tt.want, ev.Status, "op=%s value=%s", tt.op, tt.value)
	}
}

func TestEvaluate_DecimalPrecision(t *testing.T) {
	facts := factsFrom(t, `{"total":0.1}`)
	cond := Condition{Field: "total", Operator: OpEqual, Value: json.Number("0.1")}
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: cond}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusEligible, ev.Status)
}

func TestEvaluate_AndGroup(t *testing.T) {
	facts := factsFrom(t, `{"income":300,"housing_status":"HOMELESS"}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{
		All: []Condition{
			{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")},
			{Field: "housing_status", Operator: OpEqual, Value: "HOMELESS"},
		},
	}}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusEligible, ev.Status)
}

func TestEvaluate_AndGroup_FalseShortCircuits(t *testing.T) {
	facts := factsFrom(t, `{"income":1000,"housing_status":"HOMELESS"}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{
		All: []Condition{
			{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")},
			{Field: "housing_status", Operator: OpEqual, Value: "HOMELESS"},
		},
	}}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusIneligible, ev.Status)
}

func TestEvaluate_OrGroup(t *testing.T) {
	facts := factsFrom(t, `{"program_enrolled":false,"income":2000}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{
		Any: []Condition{
			{Field: "program_enrolled", Operator: OpIs, Value: true},
			{Field: "income", Operator: OpLessThanEqual, Value: json.Number("1000")},
		},
	}}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusIneligible, ev.Status)
}

func TestEvaluate_NotGroup(t *testing.T) {
	facts := factsFrom(t, `{"flagged":false}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{
		Not: &Condition{Field: "flagged", Operator: OpIs, Value: true},
	}}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusEligible, ev.Status)
}

func TestEvaluate_NestedGroups(t *testing.T) {
	facts := factsFrom(t, `{"income":300,"housing_status":"HOMELESS","age":25}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{
		All: []Condition{
			{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")},
			{Not: &Condition{Field: "housing_status", Operator: OpEqual, Value: "EMPLOYED"}},
		},
	}}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusEligible, ev.Status)
}

func TestEvaluate_MissingFactUnknown(t *testing.T) {
	facts := map[string]interface{}{"other": "value"}
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")}}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusInformationRequired, ev.Status)
}

func TestEvaluate_TypeMismatchError(t *testing.T) {
	facts := factsFrom(t, `{"income":"500"}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")}}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusError, ev.Status)
}

func TestEvaluate_StringNumberNotEqual(t *testing.T) {
	facts := factsFrom(t, `{"income":"500"}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{Field: "income", Operator: OpEqual, Value: json.Number("500")}}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusError, ev.Status)
}

func TestEvaluate_InOperator(t *testing.T) {
	facts := factsFrom(t, `{"status":"OPEN"}`)
	cond := Condition{Field: "status", Operator: OpIn, Value: []interface{}{"OPEN", "CLOSED"}}
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: cond}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusEligible, ev.Status)
}

func TestEvaluate_NotInOperator(t *testing.T) {
	facts := factsFrom(t, `{"status":"OPEN"}`)
	cond := Condition{Field: "status", Operator: OpNotIn, Value: []interface{}{"CLOSED", "PENDING"}}
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: cond}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusEligible, ev.Status)
}

func TestEvaluate_ExistsAndNotExists(t *testing.T) {
	facts := factsFrom(t, `{"name":"Jane"}`)
	for _, tt := range []struct {
		op   Operator
		want EvaluationStatus
	}{
		{OpExists, StatusEligible},
		{OpNotExists, StatusIneligible},
	} {
		cond := Condition{Field: "name", Operator: tt.op}
		rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: cond}}}
		ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
		assert.Equal(t, tt.want, ev.Status, "op=%s", tt.op)
	}
}

func TestEvaluate_Contains(t *testing.T) {
	facts := factsFrom(t, `{"name":"Jane Doe"}`)
	cond := Condition{Field: "name", Operator: OpContains, Value: "Doe"}
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: cond}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusEligible, ev.Status)
}

func TestEvaluate_Matches(t *testing.T) {
	facts := factsFrom(t, `{"email":"user@example.com"}`)
	cond := Condition{Field: "email", Operator: OpMatches, Value: `^[^@]+@example\.com$`}
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: cond}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusEligible, ev.Status)
}

func TestEvaluate_BooleanIs(t *testing.T) {
	facts := factsFrom(t, `{"active":true}`)
	cond := Condition{Field: "active", Operator: OpIs, Value: true}
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: cond}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusEligible, ev.Status)
}

func TestEvaluate_PriorityOrdering(t *testing.T) {
	facts := factsFrom(t, `{"income":300}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{
		{ID: uuid.New(), Priority: 1, Outcome: OutcomeIneligible, Conditions: Condition{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")}},
		{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")}},
	}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusEligible, ev.Status)
	assert.True(t, ev.MatchedRuleID != nil)
}

func TestEvaluate_DefaultOutcome(t *testing.T) {
	facts := factsFrom(t, `{"income":99999}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeRequiresReview, Rules: []Rule{
		{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")}},
	}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusRequiresReview, ev.Status)
}

func TestEvaluate_Determinism(t *testing.T) {
	facts := factsFrom(t, `{"income":300,"housing_status":"HOMELESS"}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{
		All: []Condition{
			{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")},
			{Field: "housing_status", Operator: OpEqual, Value: "HOMELESS"},
		},
	}}}}
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ev1 := Evaluate(rs, facts, at, nil, TriggerManual)
	ev2 := Evaluate(rs, facts, at, nil, TriggerManual)

	b1, _ := json.Marshal(ev1.Trace)
	b2, _ := json.Marshal(ev2.Trace)
	assert.Equal(t, string(b1), string(b2), "trace must be byte-identical for identical inputs")
	assert.Equal(t, ev1.Status, ev2.Status)
}

func TestEvaluate_TraceNodesPopulated(t *testing.T) {
	facts := factsFrom(t, `{"income":300,"housing_status":"HOMELESS"}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{
		All: []Condition{
			{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")},
			{Field: "housing_status", Operator: OpEqual, Value: "HOMELESS"},
		},
	}}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	require.Len(t, ev.Trace, 2)
	assert.Equal(t, TraceRoot, ev.Trace[0].NodeType)
	assert.Equal(t, TraceRule, ev.Trace[1].NodeType)
	assert.Len(t, ev.Trace[1].Children, 1)
}

func TestEvaluate_FactMissingVsNull(t *testing.T) {
	facts := map[string]interface{}{"name": nil}
	cond := Condition{Field: "name", Operator: OpExists}
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: cond}}}
	ev := Evaluate(rs, facts, time.Now().UTC(), nil, TriggerManual)
	assert.Equal(t, StatusIneligible, ev.Status)
}

func TestValidateCondition_RejectsUnknownOperator(t *testing.T) {
	c := Condition{Field: "x", Operator: "contains_all_the_things", Value: "y"}
	err := ValidateCondition(&c, 0)
	assert.ErrorIs(t, err, ErrInvalidOperator)
}

func TestValidateCondition_RejectsDeepNesting(t *testing.T) {
	c := &Condition{}
	cur := c
	for i := 0; i < MaxConditionNestingDepth+2; i++ {
		inner := &Condition{
			Not: &Condition{},
		}
		cur.Not = inner
		cur = inner.Not
	}
	err := ValidateCondition(c, 0)
	assert.ErrorIs(t, err, ErrMaxDepthExceeded)
}

func TestValidateCondition_RejectsLeafAndGroup(t *testing.T) {
	c := Condition{
		Field:    "x",
		Operator: OpEqual,
		All:      []Condition{{Field: "y", Operator: OpEqual, Value: 1}},
	}
	err := ValidateCondition(&c, 0)
	assert.Error(t, err)
}

func TestNewRuleSet_RejectsEmptyKey(t *testing.T) {
	orgID := uuid.New()
	_, err := NewRuleSet(&orgID, nil, "", "test", "", OutcomeEligible, nil, nil, uuid.New())
	assert.Error(t, err)
}

func TestNewRuleSet_Success(t *testing.T) {
	orgID := uuid.New()
	rs, err := NewRuleSet(&orgID, nil, "income-check", "Income Check", "desc", OutcomeEligible, nil, nil, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, StatusDraft, rs.Status)
	assert.Equal(t, 1, rs.Version)
}

func TestRuleSet_Clone(t *testing.T) {
	orgID := uuid.New()
	rs, err := NewRuleSet(&orgID, nil, "income-check", "Income Check", "desc", OutcomeEligible, []Rule{
		{Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")}},
	}, nil, uuid.New())
	require.NoError(t, err)

	clone := rs.Clone()
	assert.NotEqual(t, rs.ID, clone.ID)
	assert.Equal(t, rs.Version+1, clone.Version)
	assert.Equal(t, rs.Key, clone.Key)
}

func TestEvaluate_HistoricalReproducibility(t *testing.T) {
	facts := factsFrom(t, `{"income":300,"housing_status":"HOMELESS"}`)
	rs := &RuleSet{ID: uuid.New(), OrganizationID: uuid.New(), Key: "t", Version: 1, Status: StatusPublished, DefaultOutcome: OutcomeIneligible, Rules: []Rule{{ID: uuid.New(), Priority: 0, Outcome: OutcomeEligible, Conditions: Condition{
		All: []Condition{
			{Field: "income", Operator: OpLessThanEqual, Value: json.Number("500")},
			{Field: "housing_status", Operator: OpEqual, Value: "HOMELESS"},
		},
	}}}}
	at := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	actor := uuid.New()
	ev := Evaluate(rs, facts, at, &actor, TriggerManual)

	// Re-evaluate from the stored snapshot and verify identical trace.
	ev2 := Evaluate(rs, ev.FactsSnapshot, at, &actor, TriggerManual)
	b1, _ := json.Marshal(ev.Trace)
	b2, _ := json.Marshal(ev2.Trace)
	assert.Equal(t, string(b1), string(b2))
}

func TestToRat_Float64(t *testing.T) {
	r, ok := toRat(float64(42))
	assert.True(t, ok)
	assert.NotNil(t, r)
	assert.Equal(t, "42", r.RatString())

	r, ok = toRat(float64(0))
	assert.True(t, ok)
	assert.NotNil(t, r)
}

func TestToRat_NonNumeric(t *testing.T) {
	_, ok := toRat("not-a-number")
	assert.False(t, ok)

	r, ok := toRat("3.14")
	assert.True(t, ok)
	assert.NotNil(t, r)

	r, ok = toRat(json.Number("99"))
	assert.True(t, ok)
	assert.Equal(t, "99", r.RatString())

	r, ok = toRat(int(7))
	assert.True(t, ok)
	assert.Equal(t, "7", r.RatString())

	_, ok = toRat(true)
	assert.False(t, ok)
}

var _ = strings.TrimSpace
