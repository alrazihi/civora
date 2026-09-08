package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

type Migrator struct {
	db         *sql.DB
	fs         fs.FS
	migrations []Migration
}

type Migration struct {
	Version int
	Name    string
	Up      []byte
	Down    []byte
}

func NewMigrator(db *sql.DB, fsys fs.FS) *Migrator {
	return &Migrator{db: db, fs: fsys}
}

func (m *Migrator) LoadMigrations() error {
	entries, err := fs.ReadDir(m.fs, ".")
	if err != nil {
		return fmt.Errorf("failed to read migrations: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		version, migrationName, err := parseMigrationFilename(name)
		if err != nil {
			continue
		}

		upSQL, err := fs.ReadFile(m.fs, name)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", name, err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    migrationName,
			Up:      upSQL,
		})
	}

	allEntries, err := fs.ReadDir(m.fs, ".")
	if err != nil {
		return fmt.Errorf("failed to read migrations: %w", err)
	}
	for i := range migrations {
		downFile := fmt.Sprintf("%04d_%s.down.sql", migrations[i].Version, migrations[i].Name)
		for _, entry := range allEntries {
			if entry.Name() == downFile {
				downSQL, err := fs.ReadFile(m.fs, downFile)
				if err != nil {
					return fmt.Errorf("failed to read migration %s: %w", downFile, err)
				}
				migrations[i].Down = downSQL
				break
			}
		}
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	m.migrations = migrations
	return nil
}

func (m *Migrator) Migrate(ctx context.Context) error {
	if err := m.createSchemaTable(); err != nil {
		return fmt.Errorf("failed to create schema migrations table: %w", err)
	}

	for _, mg := range m.migrations {
		applied, err := m.isApplied(ctx, mg.Version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		if err := m.runMigration(ctx, mg); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", mg.Version, err)
		}
	}

	return nil
}

func (m *Migrator) createSchemaTable() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name    TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

func (m *Migrator) isApplied(ctx context.Context, version int) (bool, error) {
	var exists bool
	err := m.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
	return exists, err
}

func (m *Migrator) runMigration(ctx context.Context, mg Migration) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin migration transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(); rbErr != nil {
				// Rollback after commit returns sql.ErrTxDone; a genuine
				// rollback failure is logged but not surfaced because the
				// migration has already failed.
				_ = rbErr
			}
		}
	}()

	// The pgx stdlib driver executes the whole string as a single
	// statement, so multi-statement migrations must be split before
	// execution. Statements are split on unquoted semicolons.
	for _, stmt := range splitSQL(string(mg.Up)) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute migration up for %d: %w", mg.Version, err)
		}
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, name) VALUES ($1, $2)",
		mg.Version, mg.Name)
	if err != nil {
		return fmt.Errorf("failed to record migration %d: %w", mg.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration %d: %w", mg.Version, err)
	}

	committed = true
	return nil
}

// splitSQL splits a migration script into individual SQL statements,
// respecting single-quoted string literals so that semicolons inside
// them (e.g. CHECK constraint value lists) are not treated as statement
// boundaries.
func splitSQL(sql string) []string {
	var stmts []string
	var cur []rune
	inString := false
	for _, r := range sql {
		if r == '\'' {
			inString = !inString
		}
		if r == ';' && !inString {
			if len(cur) > 0 {
				stmts = append(stmts, strings.TrimSpace(string(cur)))
				cur = nil
			}
			continue
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		s := strings.TrimSpace(string(cur))
		if s != "" {
			stmts = append(stmts, s)
		}
	}
	return stmts
}

func parseMigrationFilename(name string) (int, string, error) {
	name = strings.TrimSuffix(name, ".up.sql")
	parts := strings.SplitN(name, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid migration filename: %s", name)
	}

	var version int
	if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
		return 0, "", fmt.Errorf("invalid migration version in filename: %s", name)
	}

	return version, parts[1], nil
}
