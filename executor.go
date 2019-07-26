package sqlf

import (
	"context"
	"database/sql"
)

// Executor can perform SQL queries.
type Executor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// ContextExecutor can perform SQL queries with context
type ContextExecutor interface {
	Executor

	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Query builds and executes the statement.
// For every row of a returned dataset it calls a handler function.
// If scan targets were set via To method calls, Query method automatically
// executes rows.Scan right before calling a handler function.
func (q *Stmt) Query(ctx context.Context, db Executor, handler func(rows *sql.Rows)) error {
	var (
		rows *sql.Rows
		err  error
	)
	// Fetch rows
	if ctxExecutor, ok := db.(ContextExecutor); ok && ctx != nil {
		rows, err = ctxExecutor.QueryContext(ctx, q.String(), q.args...)
	} else {
		rows, err = db.Query(q.String(), q.args...)
	}
	if err != nil {
		return err
	}

	// Iterate through rows of returned dataset
	for rows.Next() {
		if len(q.dest) > 0 {
			err = rows.Scan(q.dest...)
			if err != nil {
				break
			}
		}
		// Call a callback function
		handler(rows)
	}
	// Check for errors during rows "Close".
	// This may be more important if multiple statements are executed
	// in a single batch and rows were written as well as read.
	if closeErr := rows.Close(); closeErr != nil {
		return closeErr
	}

	// Check for row scan error.
	if err != nil {
		return err
	}

	// Check for errors during row iteration.
	return rows.Err()
}

// QueryRow builds, executes the statement via Executor methods
// and scans values to variables bound via To method calls.
func (q *Stmt) QueryRow(ctx context.Context, db Executor) error {
	var row *sql.Row
	if ctxExecutor, ok := db.(ContextExecutor); ok && ctx != nil {
		row = ctxExecutor.QueryRowContext(ctx, q.String(), q.args...)
	} else {
		row = db.QueryRow(q.String(), q.args...)
	}

	return row.Scan(q.dest...)
}

// Exec builds and executes the statement
func (q *Stmt) Exec(ctx context.Context, db Executor) (sql.Result, error) {
	if ctxExecutor, ok := db.(ContextExecutor); ok && ctx != nil {
		return ctxExecutor.ExecContext(ctx, q.String(), q.args...)
	}

	return db.Exec(q.String(), q.args...)
}
