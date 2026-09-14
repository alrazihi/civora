package domain

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ResolveFact retrieves a value from the facts document by following a
// dot-separated path. Array indices may be expressed as [n] segments.
// Returns ok=false when the path cannot be resolved (fact missing).
func ResolveFact(facts map[string]interface{}, path string) (interface{}, bool) {
	segs := splitFactPath(path)
	if len(segs) == 0 {
		return nil, false
	}
	cur := interface{}(facts)
	for _, seg := range segs {
		arrIdx, bracket := parseBracketIndex(seg)
		key := seg
		if bracket {
			key = strings.TrimSpace(strings.TrimSuffix(seg, "]"))
			key = strings.TrimPrefix(key, "[")
		}
		var found bool
		cur, found = descend(cur, key, arrIdx, bracket)
		if !found {
			return nil, false
		}
	}
	return cur, true
}

// parseBracketIndex reports whether seg is an array index, e.g. "items[0]".
// Returns (index, true) when seg ends with [n]; the caller still receives the
// raw seg as key so it can look up the object field first.
func parseBracketIndex(seg string) (int, bool) {
	idx := strings.LastIndex(seg, "[")
	if idx < 0 || !strings.HasSuffix(seg, "]") {
		return 0, false
	}
	inner := seg[idx+1 : len(seg)-1]
	var n int
	if _, err := fmt.Sscanf(inner, "%d", &n); err != nil {
		return 0, true
	}
	return n, true
}

// descend navigates one step into the facts.
func descend(cur interface{}, key string, idx int, bracket bool) (interface{}, bool) {
	if cur == nil {
		return nil, false
	}
	switch v := cur.(type) {
	case map[string]interface{}:
		if val, ok := v[key]; ok {
			return val, true
		}
		return nil, false
	case []interface{}:
		if bracket && idx >= 0 && idx < len(v) {
			return v[idx], true
		}
		return nil, false
	default:
		return nil, false
	}
}

// evalContext carries deterministic, per-evaluation state so that repeated
// calls with identical inputs produce byte-identical traces.
type evalContext struct {
	evaluatedAt time.Time
	counter     int64
}

// newTraceNode creates a trace node with a deterministic ID derived from the
// evaluation counter and a timestamp supplied by the caller. No wall-clock or
// random state is read, keeping evaluations reproducible.
func (ec *evalContext) newTraceNode(nodeType TraceNodeType, description string) TraceNode {
	ec.counter++
	return TraceNode{
		ID:          strconv.FormatInt(ec.counter, 10),
		NodeType:    nodeType,
		Description: description,
		Timestamp:   ec.evaluatedAt,
	}
}

// Evaluate runs a rule-set against a frozen facts document and returns an
// Evaluation. The function is pure: it performs no I/O and reads no wall-clock
// state. The caller supplies all time-dependent inputs (evaluatedAt,
// evaluatedBy) so repeated calls with identical inputs produce byte-identical
// traces.
func Evaluate(rs *RuleSet, facts map[string]interface{}, evaluatedAt time.Time, evaluatedBy *uuid.UUID, trigger Trigger) *Evaluation {
	ec := &evalContext{evaluatedAt: evaluatedAt}
	trace := []TraceNode{
		ec.newTraceNode(TraceRoot, "evaluate rule set "+rs.Key),
	}

	// Copy the rules slice before sorting so the caller's RuleSet is never
	// mutated — Evaluate is a pure function over its inputs.
	rules := make([]Rule, len(rs.Rules))
	copy(rules, rs.Rules)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})

	var matchedRuleID *uuid.UUID
	var matchedOutcome Outcome
	var overallStatus EvaluationStatus
	statusOrder := []conditionResult{}
	anyError := false
	anyUnknown := false
	matched := false

	for _, rule := range rules {
		ruleTrace := ec.newTraceNode(TraceRule, "rule priority "+strconv.Itoa(rule.Priority))
		ruleTrace.Field = ""
		result := evaluateCondition(rule.Conditions, facts, &ruleTrace, ec)
		ruleTrace.Result = string(result)
		trace = append(trace, ruleTrace)
		if result == resultTrue {
			matchedOutcome = rule.Outcome
			matchedRuleID = &rule.ID
			matched = true
			break
		}
		if result == resultError {
			anyError = true
		}
		if result == resultUnknown {
			anyUnknown = true
		}
		statusOrder = append(statusOrder, result)
	}

	if !matched {
		if anyError {
			overallStatus = StatusError
			matchedOutcome = OutcomeError
		} else if anyUnknown {
			overallStatus = StatusInformationRequired
			matchedOutcome = OutcomeInformationRequired
		} else {
			overallStatus = outcomeToStatus(rs.DefaultOutcome)
			matchedOutcome = rs.DefaultOutcome
		}
	} else {
		overallStatus = outcomeToStatus(matchedOutcome)
	}

	var reason *string
	if !matched {
		if anyError {
			r := "evaluation encountered errors"
			reason = &r
		} else if anyUnknown {
			r := "information required to evaluate"
			reason = &r
		} else {
			r := "no matching rule; applied default outcome"
			reason = &r
		}
	} else {
		r := "rule matched"
		reason = &r
	}

	return &Evaluation{
		ID:             uuid.New(),
		RuleSetID:      rs.ID,
		RuleSetVersion: rs.Version,
		OrganizationID: rs.OrganizationID,
		CaseID:         rs.CaseID,
		Status:         overallStatus,
		Outcome:        matchedOutcome,
		Reason:         reason,
		MatchedRuleID:  matchedRuleID,
		Trace:          trace,
		Trigger:        trigger,
		EvaluatedBy:    evaluatedBy,
		EvaluatedAt:    evaluatedAt,
		FactsSnapshot:  facts,
	}
}

// evaluateCondition returns the tri-state result of a condition and appends
// trace nodes.
func evaluateCondition(c Condition, facts map[string]interface{}, parent *TraceNode, ec *evalContext) conditionResult {
	node := ec.newTraceNode(TraceLeaf, "")
	if c.IsLeaf() {
		node.Field = c.Field
		node.Operator = string(c.Operator)
		node.Expected = marshalValue(c.Value)
		val, found := ResolveFact(facts, c.Field)
		if !found {
			node.Actual = []byte("null")
			node.ActualType = "missing"
			node.Result = string(resultUnknown)
			node.Reason = "fact path not found"
			*parent = appendChild(*parent, node)
			return resultUnknown
		}
		node.Actual = marshalValue(val)
		node.ActualType = valueTypeString(val)
		res, reason := evalLeaf(c.Operator, val, c.Value)
		node.Result = string(res)
		node.Reason = reason
		*parent = appendChild(*parent, node)
		return res
	}

	// Group
	groupNode := ec.newTraceNode(TraceGroup, "")
	switch {
	case len(c.All) > 0:
		groupNode.Description = "AND"
		r := evalGroup(c.All, facts, &groupNode, true, ec)
		*parent = appendChild(*parent, groupNode)
		return r
	case len(c.Any) > 0:
		groupNode.Description = "OR"
		r := evalGroup(c.Any, facts, &groupNode, false, ec)
		*parent = appendChild(*parent, groupNode)
		return r
	case c.Not != nil:
		groupNode.Description = "NOT"
		r := evaluateCondition(*c.Not, facts, &groupNode, ec)
		inverted := invertResult(r)
		groupNode.Result = string(inverted)
		*parent = appendChild(*parent, groupNode)
		return inverted
	}
	groupNode.Result = string(resultError)
	*parent = appendChild(*parent, groupNode)
	return resultError
}

func appendChild(parent TraceNode, child TraceNode) TraceNode {
	if parent.Children == nil {
		parent.Children = []TraceNode{}
	}
	parent.Children = append(parent.Children, child)
	return parent
}

func evalGroup(children []Condition, facts map[string]interface{}, parent *TraceNode, isAll bool, ec *evalContext) conditionResult {
	hasError := false
	hasUnknown := false
	for i := range children {
		childNode := ec.newTraceNode(TraceLeaf, "")
		r := evaluateCondition(children[i], facts, &childNode, ec)
		childNode.Result = string(r)
		parent.Children = append(parent.Children, childNode)
		switch r {
		case resultTrue:
			if !isAll {
				return resultTrue
			}
		case resultFalse:
			if isAll {
				return resultFalse
			}
		case resultError:
			hasError = true
		case resultUnknown:
			hasUnknown = true
		}
	}
	if hasError {
		return resultError
	}
	if hasUnknown {
		return resultUnknown
	}
	if isAll {
		return resultTrue
	}
	return resultFalse
}

func invertResult(r conditionResult) conditionResult {
	switch r {
	case resultTrue:
		return resultFalse
	case resultFalse:
		return resultTrue
	case resultUnknown:
		return resultUnknown
	case resultError:
		return resultError
	default:
		return resultError
	}
}

// evalLeaf performs the type-safe comparison of a single leaf condition.
func evalLeaf(op Operator, actual, expected interface{}) (conditionResult, string) {
	switch op {
	case OpExists:
		if actual == nil {
			return resultFalse, "value is null"
		}
		return resultTrue, "value present"
	case OpNotExists:
		if actual == nil {
			return resultTrue, "value is null"
		}
		return resultFalse, "value present"
	}

	return compareTyped(op, actual, expected)
}

// compareTyped dispatches to the type-safe comparison routine.
func compareTyped(op Operator, actual, expected interface{}) (conditionResult, string) {
	switch {
	case numericOperators[op]:
		return compareNumeric(op, actual, expected)
	case membershipOperators[op]:
		return compareMembership(op, actual, expected)
	case booleanOperators[op]:
		return compareBoolean(op, actual, expected)
	case stringOperators[op] || op == OpMatches:
		return compareString(op, actual, expected)
	default:
		return compareEquality(op, actual, expected)
	}
}

func toRat(v interface{}) (*big.Rat, bool) {
	switch n := v.(type) {
	case json.Number:
		r, ok := new(big.Rat).SetString(n.String())
		return r, ok
	case int:
		return new(big.Rat).SetInt64(int64(n)), true
	case int64:
		return new(big.Rat).SetInt64(n), true
	case float64:
		r := new(big.Rat).SetFloat64(n)
		return r, r != nil
	case bool:
		return nil, false
	case string:
		r, ok := new(big.Rat).SetString(n)
		return r, ok
	default:
		return nil, false
	}
}

func isNumber(v interface{}) bool {
	switch v.(type) {
	case json.Number, int, int64, float64, int32, float32:
		return true
	}
	return false
}

func compareNumeric(op Operator, actual, expected interface{}) (conditionResult, string) {
	if !isNumber(actual) {
		return resultError, "fact value is not numeric"
	}
	if !isNumber(expected) {
		return resultError, "condition value is not numeric"
	}
	a, aok := toRat(actual)
	e, eok := toRat(expected)
	if !aok || !eok {
		return resultError, "cannot parse numeric value"
	}
	cmp := a.Cmp(e)
	switch op {
	case OpGreaterThan:
		if cmp > 0 {
			return resultTrue, "actual greater than expected"
		}
		return resultFalse, "actual not greater than expected"
	case OpGreaterThanEqual:
		if cmp >= 0 {
			return resultTrue, "actual >= expected"
		}
		return resultFalse, "actual < expected"
	case OpLessThan:
		if cmp < 0 {
			return resultTrue, "actual less than expected"
		}
		return resultFalse, "actual not less than expected"
	case OpLessThanEqual:
		if cmp <= 0 {
			return resultTrue, "actual <= expected"
		}
		return resultFalse, "actual > expected"
	}
	return resultError, "unhandled numeric operator"
}

func compareEquality(op Operator, actual, expected interface{}) (conditionResult, string) {
	switch {
	case isNumber(actual) && isNumber(expected):
		a, aok := toRat(actual)
		e, eok := toRat(expected)
		if !aok || !eok {
			return resultError, "cannot parse numeric value"
		}
		cmp := a.Cmp(e)
		switch op {
		case OpEqual:
			if cmp == 0 {
				return resultTrue, "numeric values equal"
			}
			return resultFalse, "numeric values not equal"
		case OpNotEqual:
			if cmp != 0 {
				return resultTrue, "numeric values not equal"
			}
			return resultFalse, "numeric values equal"
		}
		return resultError, "unhandled equality operator"
	case isStringType(actual) && isStringType(expected):
		as, _ := actual.(string)
		es, _ := expected.(string)
		switch op {
		case OpEqual:
			if as == es {
				return resultTrue, "strings equal"
			}
			return resultFalse, "strings not equal"
		case OpNotEqual:
			if as != es {
				return resultTrue, "strings not equal"
			}
			return resultFalse, "strings equal"
		}
		return resultError, "unhandled string operator"
	case isBoolean(actual) && isBoolean(expected):
		ab, _ := actual.(bool)
		eb, _ := expected.(bool)
		switch op {
		case OpEqual:
			if ab == eb {
				return resultTrue, "booleans equal"
			}
			return resultFalse, "booleans not equal"
		case OpNotEqual:
			if ab != eb {
				return resultTrue, "booleans not equal"
			}
			return resultFalse, "booleans equal"
		}
		return resultError, "unhandled boolean operator"
	case isNull(actual) && isNull(expected):
		switch op {
		case OpEqual:
			return resultTrue, "both null"
		case OpNotEqual:
			return resultFalse, "both null"
		}
	}

	// Different type categories: a string "500" vs number 500 is an error,
	// never silently coerced.
	return resultError, fmt.Sprintf("type mismatch: %s vs %s", valueTypeString(actual), valueTypeString(expected))
}

func compareMembership(op Operator, actual, expected interface{}) (conditionResult, string) {
	arr, ok := expected.([]interface{})
	if !ok {
		return resultError, "membership value is not an array"
	}
	for _, elem := range arr {
		c, _ := compareEquality(OpEqual, actual, elem)
		if c == resultTrue {
			if op == OpIn {
				return resultTrue, "value found in array"
			}
			return resultFalse, "value found in array"
		}
		if c == resultError {
			return resultError, "type mismatch in array element"
		}
	}
	if op == OpIn {
		return resultFalse, "value not found in array"
	}
	return resultTrue, "value not found in array"
}

func compareBoolean(op Operator, actual, expected interface{}) (conditionResult, string) {
	ab, aok := actual.(bool)
	eb, ok := expected.(bool)
	if !aok || !ok {
		return resultError, "boolean operator requires boolean values"
	}
	switch op {
	case OpIs:
		if ab == eb && eb {
			return resultTrue, "value is true"
		}
		return resultFalse, "value is not true"
	case OpIsNot:
		if ab != eb {
			return resultTrue, "value differs from expected"
		}
		return resultFalse, "value equals expected"
	}
	return resultError, "unhandled boolean operator"
}

func compareString(op Operator, actual, expected interface{}) (conditionResult, string) {
	as, aok := actual.(string)
	es, ok := expected.(string)
	if !aok || !ok {
		return resultError, "string operator requires string values"
	}
	switch op {
	case OpContains:
		if strings.Contains(as, es) {
			return resultTrue, "substring found"
		}
		return resultFalse, "substring not found"
	case OpMatches:
		re, err := regexp.Compile(es)
		if err != nil {
			return resultError, "invalid regex pattern"
		}
		if re.MatchString(as) {
			return resultTrue, "regex matched"
		}
		return resultFalse, "regex did not match"
	}
	return resultError, "unhandled string operator"
}

func isStringType(v interface{}) bool {
	_, ok := v.(string)
	return ok
}

func isBoolean(v interface{}) bool {
	_, ok := v.(bool)
	return ok
}

func isNull(v interface{}) bool {
	return v == nil
}

func outcomeToStatus(o Outcome) EvaluationStatus {
	switch o {
	case OutcomeEligible:
		return StatusEligible
	case OutcomeIneligible:
		return StatusIneligible
	case OutcomeRequiresReview:
		return StatusRequiresReview
	case OutcomeInformationRequired:
		return StatusInformationRequired
	case OutcomeFlag:
		return StatusFlag
	case OutcomeScore:
		return StatusScore
	case OutcomeError:
		return StatusError
	default:
		return StatusError
	}
}
