package testdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
)

func NewDB() *sql.DB {
	return sql.OpenDB(noopConnector{})
}

type noopConnector struct{}

func (noopConnector) Connect(context.Context) (driver.Conn, error) {
	return noopConn{}, nil
}

func (noopConnector) Driver() driver.Driver {
	return noopDriver{}
}

var _ driver.Connector = noopConnector{}

type noopDriver struct{}

func (noopDriver) Open(string) (driver.Conn, error) {
	return noopConn{}, nil
}

var _ driver.Driver = noopDriver{}

type noopConn struct{}

func (noopConn) Prepare(string) (driver.Stmt, error) {
	return noopStmt{}, nil
}
func (noopConn) Close() error                       { return nil }
func (noopConn) Begin() (driver.Tx, error)          { return noopTx{}, nil }
func (noopConn) ResetSession(context.Context) error { return nil }

var _ driver.Conn = noopConn{}

type noopStmt struct{}

func (noopStmt) Close() error  { return nil }
func (noopStmt) NumInput() int { return -1 }
func (noopStmt) Exec([]driver.Value) (driver.Result, error) {
	return noopResult{}, nil
}
func (noopStmt) Query([]driver.Value) (driver.Rows, error) {
	return noopRows{}, nil
}

var _ driver.Stmt = noopStmt{}

type noopResult struct{}

func (noopResult) LastInsertId() (int64, error) { return 0, nil }
func (noopResult) RowsAffected() (int64, error) { return 0, nil }

type noopTx struct{}

func (noopTx) Commit() error   { return nil }
func (noopTx) Rollback() error { return nil }

type noopRows struct{}

func (noopRows) Columns() []string         { return nil }
func (noopRows) Close() error              { return nil }
func (noopRows) Next([]driver.Value) error { return io.EOF }
