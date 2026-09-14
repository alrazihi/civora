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
func valueTypeString(v interface{}) string {
	switch v.(type) {
	case string:
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
