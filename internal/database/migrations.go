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

	allEntries, _ := fs.ReadDir(m.fs, ".")
	for i := range migrations {
		downFile := fmt.Sprintf("%04d_%s.down.sql", migrations[i].Version, migrations[i].Name)
		for _, entry := range allEntries {
			if entry.Name() == downFile {
				downSQL, _ := fs.ReadFile(m.fs, downFile)
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
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(mg.Up)); err != nil {
		return fmt.Errorf("failed to execute migration up for %d: %w", mg.Version, err)
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, name) VALUES ($1, $2)",
		mg.Version, mg.Name)
	if err != nil {
		return fmt.Errorf("failed to record migration %d: %w", mg.Version, err)
	}

	return tx.Commit()
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
