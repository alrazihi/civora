package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitSQL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "empty",
			in:   "",
			want: nil,
		},
		{
			name: "single statement",
			in:   "CREATE TABLE foo (id UUID PRIMARY KEY);",
			want: []string{"CREATE TABLE foo (id UUID PRIMARY KEY)"},
		},
		{
			name: "multiple statements",
			in:   "CREATE SCHEMA IF NOT EXISTS audit;\n\nCREATE TABLE audit.audit_events (id UUID PRIMARY KEY);\n\nDROP TABLE IF EXISTS audit_events;",
			want: []string{
				"CREATE SCHEMA IF NOT EXISTS audit",
				"CREATE TABLE audit.audit_events (id UUID PRIMARY KEY)",
				"DROP TABLE IF EXISTS audit_events",
			},
		},
		{
			name: "semicolons inside string literals are preserved",
			in:   "ALTER TABLE audit_events ADD CONSTRAINT c CHECK (action IN ('case.created; case.transition'));",
			want: []string{"ALTER TABLE audit_events ADD CONSTRAINT c CHECK (action IN ('case.created; case.transition'))"},
		},
		{
			name: "trailing whitespace is trimmed",
			in:   "SELECT 1;   \n  \n",
			want: []string{"SELECT 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitSQL(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}
