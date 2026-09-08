package shared

import (
	"database/sql"
	"fmt"
)

// ScanRows closes rows and returns any error encountered while iterating.
// It must be deferred immediately after a successful Query/QueryContext call so
// that database I/O failures during iteration are surfaced instead of being
// silently swallowed.
//
// Usage:
//
//	rows, err := db.QueryContext(ctx, query, args...)
//	if err != nil { return ..., err }
//	defer shared.CloseRows(rows)
//	for rows.Next() { ... }
//	return ..., shared.CloseRows(rows)
func CloseRows(rows *sql.Rows) error {
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("rows iteration error: %w", err)
	}
	return rows.Close()
}
