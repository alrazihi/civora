package testdb

import (
	"context"
	"database/sql"
	"testing"

	"github.com/alrazihi/civora/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDB(t *testing.T) {
	db := NewDB()
	if db == nil {
		t.Fatal("expected non-nil db")
	}

	err := database.InTransaction(context.Background(), db, func(tx *sql.Tx) error {
		if tx == nil {
			t.Fatal("expected non-nil tx")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InTransaction returned error: %v", err)
	}
}

func TestNewDB_RollbackOnCallbackError(t *testing.T) {
	db := NewDB()
	callbackErr := assert.AnError

	err := database.InTransaction(context.Background(), db, func(tx *sql.Tx) error {
		return callbackErr
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, callbackErr)
}

func TestInTransaction_NilDBReturnsError(t *testing.T) {
	err := database.InTransaction(context.Background(), nil, func(tx *sql.Tx) error {
		t.Fatal("callback should not be called with nil db")
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database connection is nil")
}
