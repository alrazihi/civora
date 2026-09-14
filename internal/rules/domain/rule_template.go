package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RuleTemplateScope string

const (
	RuleTemplateScopeOrg    RuleTemplateScope = "ORG"
	RuleTemplateScopeGlobal RuleTemplateScope = "GLOBAL"
)

type RuleTemplate struct {
	ID          uuid.UUID
	Scope       RuleTemplateScope
	OrganizationID *uuid.UUID
	Key         string
	Name        string
	Description string
	Category    string
	Rule        Rule
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const (
	MaxRuleTemplatesPerOrg = 200
	MaxRuleTemplateKeyLen  = 100
)

func NewRuleTemplate(scope RuleTemplateScope, orgID *uuid.UUID, key, name, description, category string, rule Rule, createdBy uuid.UUID) (*RuleTemplate, error) {
	if scope != RuleTemplateScopeOrg && scope != RuleTemplateScopeGlobal {
		return nil, fmt.Errorf("invalid scope: %s", scope)
	}
	if scope == RuleTemplateScopeOrg && (orgID == nil || *orgID == uuid.Nil) {
		return nil, fmt.Errorf("organization_id required for org scope")
	}
	if scope == RuleTemplateScopeGlobal && orgID != nil && *orgID != uuid.Nil {
		return nil, fmt.Errorf("organization_id must be nil for global scope")
	}
	if err := ValidateRuleTemplateKey(key); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(name) > 200 {
		return nil, fmt.Errorf("name exceeds maximum length of 200 characters")
	}
	if len(description) > 2000 {
		return nil, fmt.Errorf("description exceeds maximum length of 2000 characters")
	}
	if len(category) > 100 {
		return nil, fmt.Errorf("category exceeds maximum length of 100 characters")
	}
	if !IsValidOutcome(rule.Outcome) {
		return nil, fmt.Errorf("invalid rule outcome: %s", rule.Outcome)
	}
	if err := ValidateCondition(&rule.Conditions, 0); err != nil {
		return nil, fmt.Errorf("invalid rule condition: %w", err)
	}

	now := time.Now().UTC()
	rt := &RuleTemplate{
		ID:             uuid.New(),
		Scope:          scope,
		OrganizationID: orgID,
		Key:            key,
		Name:           name,
		Description:    description,
		Category:       category,
		Rule:           rule,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return rt, nil
}

func ValidateRuleTemplateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if len(key) > MaxRuleTemplateKeyLen {
		return fmt.Errorf("key exceeds maximum length of %d characters", MaxRuleTemplateKeyLen)
	}
	return nil
}

func (rt *RuleTemplate) Clone(orgID *uuid.UUID, createdBy uuid.UUID) (*RuleTemplate, error) {
	clone := *rt
	clone.ID = uuid.New()
	clone.OrganizationID = orgID
	clone.CreatedBy = createdBy
	clone.CreatedAt = time.Now().UTC()
	clone.UpdatedAt = time.Now().UTC()
	clone.Rule = Rule{
		ID:         uuid.New(),
		Priority:   rt.Rule.Priority,
		Outcome:    rt.Rule.Outcome,
		Conditions: rt.Rule.Conditions.Clone(),
		Active:     rt.Rule.Active,
		CreatedAt:  time.Now().UTC(),
	}
	return &clone, nil
}

func MarshalRuleTemplateRule(rule Rule) (json.RawMessage, error) {
	return json.Marshal(rule)
}

func UnmarshalRuleTemplateRule(data []byte) (Rule, error) {
	var rule Rule
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&rule); err != nil {
		return Rule{}, fmt.Errorf("failed to decode rule template rule: %w", err)
	}
	return rule, nil
}

