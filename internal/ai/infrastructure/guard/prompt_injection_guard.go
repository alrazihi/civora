package guard

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PromptInjectionGuard validates LLM responses for prompt injection patterns
// and ensures responses conform to expected schemas.
type PromptInjectionGuard struct{}

func NewPromptInjectionGuard() *PromptInjectionGuard {
	return &PromptInjectionGuard{}
}

// ValidateResponse ensures the LLM response is safe and conforms to expected structure.
// It strips non-JSON wrappers, validates JSON structure, and checks for injection patterns.
func (g *PromptInjectionGuard) ValidateResponse(content json.RawMessage) (json.RawMessage, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}

	raw := string(content)

	// Strip common non-JSON prefixes/suffixers that may contain injected instructions
	raw = stripNonJSONPrefixSuffix(raw)

	// Validate JSON structure
	if !isValidJSON(raw) {
		return nil, fmt.Errorf("LLM response is not valid JSON")
	}

	// Check for prompt injection patterns
	if err := g.checkInjectionPatterns(raw); err != nil {
		return nil, fmt.Errorf("prompt injection detected: %w", err)
	}

	return json.RawMessage(raw), nil
}

// stripNonJSONPrefixSuffix removes non-JSON content that may wrap the actual JSON response.
func stripNonJSONPrefixSuffix(s string) string {
	s = strings.TrimSpace(s)

	// Try to find the first '{' or '[' which typically starts JSON
	start := -1
	for i, c := range s {
		if c == '{' || c == '[' {
			start = i
			break
		}
	}
	if start == -1 {
		return s
	}

	// Find matching closing bracket
	braceCount := 0
	bracketCount := 0
	end := len(s)
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			braceCount++
		case '}':
			braceCount--
		case '[':
			bracketCount++
		case ']':
			bracketCount--
		}

		if braceCount == 0 && bracketCount == 0 {
			end = i + 1
			break
		}
	}

	return s[start:end]
}

// isValidJSON checks if the string is valid JSON.
func isValidJSON(s string) bool {
	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

// checkInjectionPatterns checks for common prompt injection patterns in LLM responses.
func (g *PromptInjectionGuard) checkInjectionPatterns(content string) error {
	lower := strings.ToLower(content)

	injectionPatterns := []string{
		"ignore previous instructions",
		"ignore all previous",
		"disregard all",
		"forget your instructions",
		"you are now",
		"new instructions",
		"override your",
		"system prompt",
		"jailbreak",
		"pretend you are",
		"act as if",
		"roleplay as",
	}

	for _, pattern := range injectionPatterns {
		if strings.Contains(lower, pattern) {
			return fmt.Errorf("detected potential prompt injection pattern: %s", pattern)
		}
	}

	return nil
}

// ValidateObservationResult validates a single observation result for injection.
func (g *PromptInjectionGuard) ValidateObservationResult(obs map[string]any) error {
	if obs == nil {
		return fmt.Errorf("observation is nil")
	}

	// Check for suspicious keys that might indicate injection
	suspiciousKeys := []string{
		"instruction",
		"command",
		"system",
		"prompt",
		"override",
		"jailbreak",
		"role",
		"behavior",
	}

	for key := range obs {
		lowerKey := strings.ToLower(key)
		for _, suspicious := range suspiciousKeys {
			if strings.Contains(lowerKey, suspicious) {
				return fmt.Errorf("suspicious key detected in observation: %s", key)
			}
		}
	}

	return nil
}
