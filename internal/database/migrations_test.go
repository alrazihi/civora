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
		{
			name: "semicolon inside line comment is ignored",
			in:   "-- comment with a semicolon;\nCREATE TABLE foo (id UUID);",
			want: []string{"CREATE TABLE foo (id UUID)"},
		},
		{
			name: "block comments are skipped",
			in:   "/* multi\nline comment; */ CREATE TABLE bar (id UUID);",
			want: []string{"CREATE TABLE bar (id UUID)"},
		},
		{
			name: "escaped quote inside string literal is preserved",
			in:   "SELECT 'it''s a test' AS foo; SELECT 2;",
			want: []string{"SELECT 'it''s a test' AS foo", "SELECT 2"},
		},
		{
			name: "dollar-quoted string with semicolons is atomic",
			in:   "CREATE FUNCTION example() RETURNS void AS $$ BEGIN SELECT 1; SELECT 2; END; $$ LANGUAGE plpgsql;",
			want: []string{"CREATE FUNCTION example() RETURNS void AS $$ BEGIN SELECT 1; SELECT 2; END; $$ LANGUAGE plpgsql"},
		},
		{
			name: "tagged dollar-quoted string with semicolons is atomic",
			in:   "CREATE FUNCTION example() RETURNS void AS $body$ BEGIN SELECT 1; END; $body$ LANGUAGE plpgsql;",
			want: []string{"CREATE FUNCTION example() RETURNS void AS $body$ BEGIN SELECT 1; END; $body$ LANGUAGE plpgsql"},
		},
		{
			name: "dollar-quoted strings with single quotes are unaffected",
			in:   "SELECT $$it's a test$$ AS foo; SELECT 2;",
			want: []string{"SELECT $$it's a test$$ AS foo", "SELECT 2"},
		},
		{
			name: "nested dollar-quoted strings with different tags",
			in:   "CREATE FUNCTION example() RETURNS void AS $outer$ BEGIN $inner$ SELECT 1; SELECT 2; $inner$ END; $outer$ LANGUAGE plpgsql;",
			want: []string{"CREATE FUNCTION example() RETURNS void AS $outer$ BEGIN $inner$ SELECT 1; SELECT 2; $inner$ END; $outer$ LANGUAGE plpgsql"},
		},
		{
			name: "dollar sign not part of a tag is treated as literal",
			in:   "SELECT price = $100; SELECT 2;",
			want: []string{"SELECT price = $100", "SELECT 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitSQL(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}
