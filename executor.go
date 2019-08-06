package sqlf

import (
	"context"
	"database/sql"
)

// Executor performs SQL queries.
// It's an interface accepted by Query, QueryRow and Exec methods.
// Both sql.DB, sql.Conn and sql.Tx can be passed as executor.
type Executor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// ContextExecutor performs SQL queries with context.
// It's an interface accepted by Query, QueryRow and Exec methods.
// Both sql.DB, sql.Conn and sql.Tx can be passed as context executor.
type ContextExecutor interface {
	Executor

	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Query executes the statement.
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

// QueryAndClose executes the statement and releases all the resources that
// can be reused to a pool. Do not call any Stmt methods after this call.
func (q *Stmt) QueryAndClose(ctx context.Context, db Executor, handler func(rows *sql.Rows)) error {
	err := q.Query(ctx, db, handler)
	q.Close()
	return err
}

// QueryRow executes the statement via Executor methods
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

// QueryRowAndClose executes the statement via Executor methods
// and scans values to variables bound via To method calls.
// All the objects allocated by query builder are moved to a pool
// to be reused.
//
// Do not call any Stmt methods after this call.
func (q *Stmt) QueryRowAndClose(ctx context.Context, db Executor) error {
	err := q.QueryRow(ctx, db)
	q.Close()
	return err
}

// Exec executes the statement.
func (q *Stmt) Exec(ctx context.Context, db Executor) (sql.Result, error) {
	if ctxExecutor, ok := db.(ContextExecutor); ok && ctx != nil {
		return ctxExecutor.ExecContext(ctx, q.String(), q.args...)
	}

	return db.Exec(q.String(), q.args...)
}

// ExecAndClose executes the statement and releases all the objects
// and buffers allocated by statement builder back to a pool.
//
// Do not call any Stmt methods after this call.
func (q *Stmt) ExecAndClose(ctx context.Context, db Executor) (sql.Result, error) {
	res, err := q.Exec(ctx, db)
	q.Close()
	return res, err
}
