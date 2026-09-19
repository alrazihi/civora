package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTriggersMatch(t *testing.T) {
	tests := []struct {
		name      string
		triggers  []string
		event     string
		wantMatch bool
	}{
		{"empty triggers matches any event", []string{}, "workflow_transition", true},
		{"nil triggers matches any event", nil, "workflow_transition", true},
		{"exact event match", []string{"workflow_transition"}, "workflow_transition", true},
		{"state-specific trigger", []string{"state:OPEN"}, "state:OPEN", true},
		{"non-matching trigger", []string{"form_submitted"}, "workflow_transition", false},
		{"event not in triggers", []string{"form_submitted", "case_created"}, "workflow_transition", false},
		{"match in multi-trigger list", []string{"form_submitted", "workflow_transition"}, "workflow_transition", true},
		{"empty event matches any triggers", []string{"form_submitted"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := triggersMatch(tt.triggers, tt.event)
			assert.Equal(t, tt.wantMatch, got)
		})
	}
}
