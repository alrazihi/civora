package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Database struct {
	DB     *sql.DB
	Driver string
}

func NewDatabase(dsn string, driver string) (*Database, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := waitForDatabase(db); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{DB: db, Driver: driver}, nil
}

func waitForDatabase(db *sql.DB) error {
	maxRetries := 30
	delay := 1 * time.Second

	for i := 0; i < maxRetries; i++ {
		if err := db.PingContext(context.Background()); err == nil {
			return nil
		}
		time.Sleep(delay)
		delay = time.Duration(float64(delay) * 1.5)
	}

	return fmt.Errorf("database not reachable after %d retries", maxRetries)
}

func (d *Database) Close() error {
	return d.DB.Close()
}

func BuildDSN(host, port, user, password, dbName, sslMode string) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbName, sslMode)
}

func InTransaction(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("failed to rollback transaction: %w (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func SplitDSN(dsn string) map[string]string {
	result := make(map[string]string)
	parts := strings.Fields(dsn)
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}
