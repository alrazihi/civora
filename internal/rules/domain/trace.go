package domain

import (
	"encoding/json"
	"time"
)

// TraceNode is a single annotated step in an evaluation trace.
type TraceNode struct {
	ID          string          `json:"id"`
	NodeType    TraceNodeType   `json:"node_type"`
	Description string          `json:"description"`
	Field       string          `json:"field,omitempty"`
	Operator    string          `json:"operator,omitempty"`
	Expected    json.RawMessage `json:"expected,omitempty"`
	Actual      json.RawMessage `json:"actual,omitempty"`
	ActualType  string          `json:"actual_type,omitempty"`
	Result      string          `json:"result"`
	Reason      string          `json:"reason,omitempty"`
	Children    []TraceNode     `json:"children,omitempty"`
	Timestamp   time.Time       `json:"timestamp"`
}

// TraceNodeType classifies a trace node.
type TraceNodeType string

const (
	TraceLeaf  TraceNodeType = "leaf"
	TraceGroup TraceNodeType = "group"
	TraceRule  TraceNodeType = "rule"
	TraceRoot  TraceNodeType = "root"
)

// conditionResult is the internal tri-state/quad result of a condition.
type conditionResult string

const (
	resultTrue    conditionResult = "TRUE"
	resultFalse   conditionResult = "FALSE"
	resultUnknown conditionResult = "UNKNOWN"
	resultError   conditionResult = "ERROR"
)

// marshalValue renders a Go value as JSON. nil becomes null. Numbers are
// preserved as their original representation.
func marshalValue(v interface{}) json.RawMessage {
	if v == nil {
		return []byte("null")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return b
}

// valueTypeString returns a human-readable type label for a fact value, for
// trace/explanation output. It does NOT participate in comparison logic.
// RFC3339 date (YYYY-MM-DD) and date-time (RFC3339) strings are labelled as
// "date" and "datetime" respectively so the trace matches the ValueType enum.
func valueTypeString(v interface{}) string {
	switch x := v.(type) {
	case string:
		if isRFC3339Date(x) {
			return "date"
		}
		if isRFC3339DateTime(x) {
			return "datetime"
		}
		return "string"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}

// isRFC3339Date reports whether s is a valid ISO 8601 calendar date
// (YYYY-MM-DD). It does not accept full RFC3339 date-times.
func isRFC3339Date(s string) bool {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return false
	}
	return t.Format("2006-01-02") == s
}

// isRFC3339DateTime reports whether s is a valid RFC3339 date-time. The
// trailing 'Z' or explicit numeric offset forms are both accepted.
func isRFC3339DateTime(s string) bool {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return false
	}
	return t.Format(time.RFC3339) == s
}

// parseTemporal parses an RFC3339 date or date-time string into a time.Time
// in UTC. It returns ok=false when the value is not a recognised temporal
// string. A bare date (YYYY-MM-DD) is normalised to midnight UTC.
func parseTemporal(s string) (time.Time, bool) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		if t.Format("2006-01-02") == s {
			return t.UTC(), true
		}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		if t.Format(time.RFC3339) == s {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
