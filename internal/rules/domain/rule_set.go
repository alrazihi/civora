package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ValidateRuleSet validates the structural integrity of a RuleSet and all its
// conditions. Returns an aggregate error describing every problem found, so the
// caller can surface a precise reason to the rule author.
func ValidateRuleSet(rs *RuleSet) error {
	var errs []error

	if rs.OrganizationID == uuid.Nil {
		errs = append(errs, fmt.Errorf("organization_id is required"))
	}
	if rs.Key == "" {
		errs = append(errs, fmt.Errorf("key is required"))
	}
	if rs.Name == "" {
		errs = append(errs, fmt.Errorf("name is required"))
	}
	if rs.Version < 1 {
		errs = append(errs, fmt.Errorf("version must be >= 1"))
	}
	if !IsValidOutcome(rs.DefaultOutcome) {
		errs = append(errs, fmt.Errorf("invalid default_outcome: %s", rs.DefaultOutcome))
	}
	if len(rs.Rules) > MaxRulesPerRuleSet {
		errs = append(errs, fmt.Errorf("too many rules: %d (max %d)", len(rs.Rules), MaxRulesPerRuleSet))
	}

	seenPriorities := map[int]bool{}
	condCount := 0
	for i, rule := range rs.Rules {
		if rule.Priority < 0 {
			errs = append(errs, fmt.Errorf("rule %d: priority must be >= 0", i))
		}
		if !IsValidOutcome(rule.Outcome) {
			errs = append(errs, fmt.Errorf("rule %d: invalid outcome: %s", i, rule.Outcome))
		}
		if !IsValidOperator(rule.Conditions.Operator) && !rule.Conditions.IsGroup() && !rule.Conditions.IsLeaf() {
			errs = append(errs, fmt.Errorf("rule %d: condition must be leaf or group", i))
		}
		if err := ValidateCondition(&rule.Conditions, 0); err != nil {
			errs = append(errs, fmt.Errorf("rule %d: %w", i, err))
		}
		condCount += countConditions(rule.Conditions)
		if seenPriorities[rule.Priority] {
			errs = append(errs, ErrDuplicateRulePriority)
		}
		seenPriorities[rule.Priority] = true
	}
	if condCount > MaxConditionsPerRuleSet {
		errs = append(errs, fmt.Errorf("too many total conditions: %d (max %d)", condCount, MaxConditionsPerRuleSet))
	}

	if len(errs) > 0 {
		return fmt.Errorf("rule set validation failed: %v", errs)
	}
	return nil
}

func countConditions(c Condition) int {
	if c.IsLeaf() {
		return 1
	}
	n := 0
	for i := range c.All {
		n += countConditions(c.All[i])
	}
	for i := range c.Any {
		n += countConditions(c.Any[i])
	}
	if c.Not != nil {
		n += countConditions(*c.Not)
	}
	return n
}

// MarshalRulesJSON serialises the rules slice to JSONB-compatible JSON. This is
// what gets persisted in the rules.rule_sets.rules column.
func MarshalRulesJSON(rules []Rule) (json.RawMessage, error) {
	return json.Marshal(rules)
}

// UnmarshalRulesJSON deserialises the JSONB rules column into typed rules.
func UnmarshalRulesJSON(data []byte) ([]Rule, error) {
	var rules []Rule
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&rules); err != nil {
		return nil, fmt.Errorf("failed to decode rules: %w", err)
	}
	return rules, nil
}

// NewRuleSet creates a new DRAFT rule-set with version 1.
func NewRuleSet(orgID, caseID *uuid.UUID, key, name, description string, defaultOutcome Outcome, rules []Rule, triggers []string, createdBy uuid.UUID) (*RuleSet, error) {
	if orgID == nil || *orgID == uuid.Nil {
		return nil, fmt.Errorf("organization_id is required")
	}
	if err := validateKey(key); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := validateName(name); err != nil {
		return nil, err
	}
	if err := validateDescription(description); err != nil {
		return nil, err
	}
	if err := ValidateOutcome(defaultOutcome); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	rs := &RuleSet{
		ID:             uuid.New(),
		OrganizationID: *orgID,
		Key:            key,
		Name:           name,
		Description:    description,
		Version:        1,
		Status:         StatusDraft,
		DefaultOutcome: defaultOutcome,
		Rules:          rules,
		Triggers:       triggers,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if caseID != nil && *caseID != uuid.Nil {
		rs.CaseID = caseID
	}
	if err := ValidateRuleSet(rs); err != nil {
		return nil, err
	}
	return rs, nil
}

// Clone creates a deep copy suitable for versioning.
func (rs *RuleSet) Clone() *RuleSet {
	clone := *rs
	clone.Rules = make([]Rule, len(rs.Rules))
	for i := range rs.Rules {
		clone.Rules[i] = Rule{
			ID:         uuid.New(),
			Priority:   rs.Rules[i].Priority,
			Outcome:    rs.Rules[i].Outcome,
			Conditions: rs.Rules[i].Conditions.Clone(),
			Active:     rs.Rules[i].Active,
			CreatedAt:  rs.Rules[i].CreatedAt,
		}
	}
	clone.ID = uuid.New()
	clone.Version = rs.Version + 1
	return &clone
}

// ValidateOutcome checks that default outcome is a valid value.
func ValidateOutcome(o Outcome) error {
	if !IsValidOutcome(o) {
		return fmt.Errorf("invalid default_outcome: %s", o)
	}
	return nil
}

func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if len(key) > 100 {
		return fmt.Errorf("key exceeds maximum length of 100 characters")
	}
	return nil
}

func validateName(name string) error {
	if len(name) > 200 {
		return fmt.Errorf("name exceeds maximum length of 200 characters")
	}
	return nil
}

func validateDescription(desc string) error {
	if len(desc) > 2000 {
		return fmt.Errorf("description exceeds maximum length of 2000 characters")
	}
	return nil
}
