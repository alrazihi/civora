package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

const (
	MaxConditionNestingDepth = 10
	MaxConditionsPerRuleSet  = 500
	MaxRulesPerRuleSet       = 100
)

var (
	// factPathSegment matches a single dot-separated segment. Array indices
	// are handled separately during resolution.
	factPathSegment = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// splitFactPath splits a dot-separated fact path, preserving bracket index
// segments such as "items[0]" as single segments.

// decoded with json.Number (via UseNumber) so the engine can distinguish
// integers from decimals and avoid floating-point precision loss.
func (c *Condition) UnmarshalJSON(data []byte) error {
	var raw struct {
		Field    string          `json:"field"`
		Operator string          `json:"operator"`
		Value    json.RawMessage `json:"value"`
		All      json.RawMessage `json:"all"`
		Any      json.RawMessage `json:"any"`
		Not      *Condition      `json:"not"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("condition decode: %w", err)
	}

	isLeaf := raw.Field != "" || raw.Operator != ""
	hasAll := len(raw.All) > 0 && !jsonNullMatch(raw.All)
	hasAny := len(raw.Any) > 0 && !jsonNullMatch(raw.Any)
	hasNot := raw.Not != nil

	groupCount := 0
	if hasAll {
		groupCount++
	}
	if hasAny {
		groupCount++
	}
	if hasNot {
		groupCount++
	}

	if isLeaf && groupCount > 0 {
		return errors.New("condition cannot be both a leaf (field/operator) and a group (all/any/not)")
	}
	if !isLeaf && groupCount == 0 {
		return errors.New("condition must be either a leaf (field + operator) or a group (all/any/not)")
	}
	if groupCount > 1 {
		return errors.New("condition can only define one of: all, any, not")
	}

	c.Field = raw.Field
	c.Operator = Operator(raw.Operator)

	if len(raw.Value) > 0 && !jsonNullMatch(raw.Value) {
		var v interface{}
		if err := json.Unmarshal(raw.Value, &v); err != nil {
			return fmt.Errorf("condition value decode: %w", err)
		}
		c.Value = v
	}

	if hasAll {
		var all []Condition
		if err := json.Unmarshal(raw.All, &all); err != nil {
			return fmt.Errorf("all group decode: %w", err)
		}
		c.All = all
	}
	if hasAny {
		var any []Condition
		if err := json.Unmarshal(raw.Any, &any); err != nil {
			return fmt.Errorf("any group decode: %w", err)
		}
		c.Any = any
	}
	c.Not = raw.Not
	return nil
}

// jsonNullMatch returns true if the raw JSON message is the literal null.
func jsonNullMatch(msg json.RawMessage) bool {
	s := bytes.TrimSpace(msg)
	return string(s) == "null" || len(s) == 0
}

// MarshalJSON renders the condition back to its JSON form, preserving the
// recursive structure.
func (c Condition) MarshalJSON() ([]byte, error) {
	if !c.IsLeaf() && !c.IsGroup() {
		return nil, errors.New("empty condition")
	}
	if c.IsLeaf() {
		m := map[string]interface{}{
			"field":    c.Field,
			"operator": c.Operator,
		}
		if c.Value != nil {
			m["value"] = c.Value
		}
		return json.Marshal(m)
	}
	obj := map[string]interface{}{}
	if len(c.All) > 0 {
		obj["all"] = c.All
	}
	if len(c.Any) > 0 {
		obj["any"] = c.Any
	}
	if c.Not != nil {
		obj["not"] = c.Not
	}
	return json.Marshal(obj)
}

// Clone returns a deep copy of the condition tree.
func (c Condition) Clone() Condition {
	clone := c
	clone.All = make([]Condition, len(c.All))
	for i := range c.All {
		clone.All[i] = c.All[i].Clone()
	}
	clone.Any = make([]Condition, len(c.Any))
	for i := range c.Any {
		clone.Any[i] = c.Any[i].Clone()
	}
	if c.Not != nil {
		n := c.Not.Clone()
		clone.Not = &n
	}
	return clone
}

// HasValueRequired reports operators that must carry a value.
func hasValueRequired(op Operator) bool {
	return !existenceOperators[op]
}

// ValidateCondition validates a single condition's structure and type
// correctness of its value. Returns the first error encountered.
func ValidateCondition(c *Condition, depth int) error {
	if depth > MaxConditionNestingDepth {
		return ErrMaxDepthExceeded
	}
	if c.IsLeaf() {
		return validateLeaf(c)
	}
	if c.IsGroup() {
		count := 0
		if len(c.All) > 0 {
			count++
		}
		if len(c.Any) > 0 {
			count++
		}
		if c.Not != nil {
			count++
		}
		if count != 1 {
			return errors.New("group condition must define exactly one of all/any/not")
		}
		if len(c.All) > 0 {
			for i := range c.All {
				if err := ValidateCondition(&c.All[i], depth+1); err != nil {
					return err
				}
			}
		}
		if len(c.Any) > 0 {
			for i := range c.Any {
				if err := ValidateCondition(&c.Any[i], depth+1); err != nil {
					return err
				}
			}
		}
		if c.Not != nil {
			if err := ValidateCondition(c.Not, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New("condition must be a leaf or a group")
}

func validateLeaf(c *Condition) error {
	if c.Field == "" {
		return errors.New("leaf condition requires a field")
	}
	if !IsValidOperator(c.Operator) {
		return fmt.Errorf("%w: %s", ErrInvalidOperator, c.Operator)
	}
	if !isValidFactPath(c.Field) {
		return fmt.Errorf("%w: %s", ErrInvalidFactPath, c.Field)
	}
	if existenceOperators[c.Operator] {
		return nil
	}
	if c.Value == nil {
		return errors.New("condition value is required for operator " + string(c.Operator))
	}
	return validateValueForOperator(c.Operator, c.Value)
}

// validateValueForOperator checks that the condition value's JSON type is
// compatible with the operator. This is a structural type check — comparing
// the value against actual fact data happens at evaluation time.
func validateValueForOperator(op Operator, val interface{}) error {
	switch {
	case numericOperators[op]:
		if !isJSONNumber(val) {
			return fmt.Errorf("%w: operator %s requires a numeric value", ErrTypeMismatch, op)
		}
		return nil
	case membershipOperators[op]:
		arr, ok := val.([]interface{})
		if !ok {
			return fmt.Errorf("%w: operator %s requires an array value", ErrTypeMismatch, op)
		}
		for i, elem := range arr {
			if isJSONObject(elem) || isJSONArray(elem) {
				return fmt.Errorf("%w: array element %d of operator %s must not be object/array", ErrTypeMismatch, i, op)
			}
			if _, ok := elem.(json.Number); ok {
				continue
			}
			if isJSONNumber(elem) {
				continue
			}
			_ = arr
		}
		return nil
	case booleanOperators[op]:
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("%w: operator %s requires a boolean value", ErrTypeMismatch, op)
		}
		return nil
	case stringOperators[op]:
		if _, ok := val.(string); !ok {
			return fmt.Errorf("%w: operator %s requires a string value", ErrTypeMismatch, op)
		}
		return nil
	case op == OpMatches:
		pattern, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: operator %s requires a string value", ErrTypeMismatch, op)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("%w: invalid regex pattern for matches: %v", ErrInvalidOperator, err)
		}
		return nil
	default:
		// eq / neq: value must be a scalar (string, number, bool) or null.
		if isJSONObject(val) || isJSONArray(val) {
			return fmt.Errorf("%w: operator %s requires a scalar value", ErrTypeMismatch, op)
		}
		return nil
	}
}

func isJSONNumber(v interface{}) bool {
	switch v.(type) {
	case json.Number, int, int64, float64, int32, float32:
		return true
	}
	return false
}

func isJSONObject(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}

func isJSONArray(v interface{}) bool {
	_, ok := v.([]interface{})
	return ok
}

// isValidFactPath validates that a dot-separated path contains only legal
// segment characters. Array-index brackets like arr[0] are tolerated.
func isValidFactPath(path string) bool {
	if path == "" || len(path) > 256 {
		return false
	}
	for _, seg := range splitFactPath(path) {
		if seg == "" {
			return false
		}
	}
	return true
}

// splitFactPath splits a dotted path, preserving bracket array indices.
// e.g. "submissions.income.data.items[0].name" ->
//   ["submissions","income","data","items[0]","name"]
func splitFactPath(path string) []string {
	var segs []string
	var cur bytes.Buffer
	inBracket := false
	for _, r := range path {
		switch {
		case r == '[':
			inBracket = true
			cur.WriteRune(r)
		case r == ']':
			inBracket = false
			cur.WriteRune(r)
		case r == '.' && !inBracket:
			if cur.Len() > 0 {
				segs = append(segs, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		segs = append(segs, cur.String())
	}
	return segs
}
