package domain

import (
	"time"

	"github.com/google/uuid"
)

// RuleSetStatus is the lifecycle state of a rule-set version.
type RuleSetStatus string

const (
	StatusDraft     RuleSetStatus = "DRAFT"
	StatusPublished RuleSetStatus = "PUBLISHED"
	StatusArchived  RuleSetStatus = "ARCHIVED"
)

// Outcome is the result a rule (or default) can produce.
type Outcome string

const (
	OutcomeEligible            Outcome = "ELIGIBLE"
	OutcomeIneligible          Outcome = "INELIGIBLE"
	OutcomeRequiresReview      Outcome = "REQUIRES_REVIEW"
	OutcomeInformationRequired Outcome = "INFORMATION_REQUIRED"
	OutcomeFlag                Outcome = "FLAG"
	OutcomeScore               Outcome = "SCORE"
	OutcomeError               Outcome = "ERROR"
)

// IsValidOutcome reports whether o is a recognised rule outcome.
func IsValidOutcome(o Outcome) bool {
	switch o {
	case OutcomeEligible, OutcomeIneligible, OutcomeRequiresReview,
		OutcomeInformationRequired, OutcomeFlag, OutcomeScore, OutcomeError:
		return true
	default:
		return false
	}
}

// Operator is a whitelisted comparison operator. No arbitrary code is executed;
// conditions are pure data interpreted by the engine.
type Operator string

const (
	OpEqual            Operator = "eq"
	OpNotEqual         Operator = "neq"
	OpGreaterThan      Operator = "gt"
	OpGreaterThanEqual Operator = "gte"
	OpLessThan         Operator = "lt"
	OpLessThanEqual    Operator = "lte"
	OpIn               Operator = "in"
	OpNotIn            Operator = "not_in"
	OpIs               Operator = "is"
	OpIsNot            Operator = "is_not"
	OpExists           Operator = "exists"
	OpNotExists        Operator = "not_exists"
	OpContains         Operator = "contains"
	OpMatches          Operator = "matches"
	OpBefore           Operator = "before"
	OpAfter            Operator = "after"
	OpOnOrBefore       Operator = "on_or_before"
	OpOnOrAfter        Operator = "on_or_after"
)

// validOperators is the authoritative whitelist. Any operator not present here
// is rejected during parsing/validation.
var validOperators = map[Operator]bool{
	OpEqual:            true,
	OpNotEqual:         true,
	OpGreaterThan:      true,
	OpGreaterThanEqual: true,
	OpLessThan:         true,
	OpLessThanEqual:    true,
	OpIn:               true,
	OpNotIn:            true,
	OpIs:               true,
	OpIsNot:            true,
	OpExists:           true,
	OpNotExists:        true,
	OpContains:         true,
	OpMatches:          true,
	OpBefore:           true,
	OpAfter:            true,
	OpOnOrBefore:       true,
	OpOnOrAfter:        true,
}

// IsValidOperator reports whether op is a recognised operator.
func IsValidOperator(op Operator) bool {
	return validOperators[op]
}

// numericOperators require both operands to be JSON numbers.
var numericOperators = map[Operator]bool{
	OpGreaterThan:      true,
	OpGreaterThanEqual: true,
	OpLessThan:         true,
	OpLessThanEqual:    true,
}

// membershipOperators require the condition value to be a JSON array.
var membershipOperators = map[Operator]bool{
	OpIn:    true,
	OpNotIn: true,
}

// booleanOperators expect a boolean condition value and a boolean fact value.
var booleanOperators = map[Operator]bool{
	OpIs:    true,
	OpIsNot: true,
}

// existenceOperators do not consume a value.
var existenceOperators = map[Operator]bool{
	OpExists:    true,
	OpNotExists: true,
}

// stringOperators require both the fact and the condition value to be strings.
var stringOperators = map[Operator]bool{
	OpContains: true,
	OpMatches:  true,
}

// temporalOperators compare date/datetime values. Both operands must be
// RFC3339 strings; comparisons are performed in UTC to remain deterministic
// regardless of the caller's local timezone.
var temporalOperators = map[Operator]bool{
	OpBefore:     true,
	OpAfter:      true,
	OpOnOrBefore: true,
	OpOnOrAfter:  true,
}

// OperatorCategory classifies an operator for value-type validation.
func OperatorCategory(op Operator) string {
	switch {
	case numericOperators[op]:
		return "numeric"
	case membershipOperators[op]:
		return "membership"
	case booleanOperators[op]:
		return "boolean"
	case existenceOperators[op]:
		return "existence"
	case stringOperators[op]:
		return "string"
	case temporalOperators[op]:
		return "temporal"
	case op == OpEqual || op == OpNotEqual:
		return "equality"
	default:
		return "unknown"
	}
}

// ValueType enumerates the JSON value types a fact can hold.
type ValueType string

const (
	TypeString   ValueType = "string"
	TypeInteger  ValueType = "integer"
	TypeDecimal  ValueType = "decimal"
	TypeBoolean  ValueType = "boolean"
	TypeNull     ValueType = "null"
	TypeArray    ValueType = "array"
	TypeObject   ValueType = "object"
	TypeDate     ValueType = "date"
	TypeDateTime ValueType = "datetime"
)

// Trigger identifies what caused an evaluation to run.
type Trigger string

const (
	TriggerManual    Trigger = "MANUAL"
	TriggerAutomatic Trigger = "AUTOMATIC"
)

// EvalScope is the scope of evaluation: a single rule-set or the full set.
type EvalScope string

// RuleSet describes a versioned, tenant-scoped collection of rules.
type RuleSet struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	CaseID         *uuid.UUID
	Key            string
	Name           string
	Description    string
	Version        int
	Status         RuleSetStatus
	DefaultOutcome Outcome
	Rules          []Rule
	Triggers       []string
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Rule is a single conditional expression with an outcome and priority.
type Rule struct {
	ID         uuid.UUID
	Priority   int
	Outcome    Outcome
	Conditions Condition
	Active     bool
	CreatedAt  time.Time
}

// Condition is a recursive condition node.
type Condition struct {
	// Leaf fields — set when this is a comparison (leaf) condition.
	Field    string
	Operator Operator
	Value    interface{}

	// Group fields — at most one may be non-empty.
	All []Condition
	Any []Condition
	Not *Condition
}

// IsLeaf reports whether the condition is a leaf comparison.
func (c Condition) IsLeaf() bool {
	return c.Field != ""
}

// IsGroup reports whether the condition is a group (and/or/not).
func (c Condition) IsGroup() bool {
	return len(c.All) > 0 || len(c.Any) > 0 || c.Not != nil
}

// Facts is the frozen input document consumed by the engine.
type Facts = map[string]interface{}

// Evaluation captures the result of evaluating a rule-set against facts.
type Evaluation struct {
	ID             uuid.UUID
	RuleSetID      uuid.UUID
	RuleSetVersion int
	OrganizationID uuid.UUID
	CaseID         *uuid.UUID
	Status         EvaluationStatus
	Outcome        Outcome
	Reason         *string
	MatchedRuleID  *uuid.UUID
	Trace          []TraceNode
	Trigger        Trigger
	EvaluatedBy    *uuid.UUID
	EvaluatedAt    time.Time
	FactsSnapshot  Facts
}

// EvaluationStatus is the status of an evaluation.
type EvaluationStatus string

const (
	StatusEligible            EvaluationStatus = "ELIGIBLE"
	StatusIneligible          EvaluationStatus = "INELIGIBLE"
	StatusRequiresReview      EvaluationStatus = "REQUIRES_REVIEW"
	StatusInformationRequired EvaluationStatus = "INFORMATION_REQUIRED"
	StatusFlag                EvaluationStatus = "FLAG"
	StatusScore               EvaluationStatus = "SCORE"
	StatusError               EvaluationStatus = "ERROR"
)
