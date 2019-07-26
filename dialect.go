package sqlf

import (
	"strconv"
	"sync/atomic"

	"github.com/valyala/bytebufferpool"
)

// Dialect defines the method SQL statement is to be built.
type Dialect uint32

const (
	// NoDialect is a default statement builder mode.
	// No SQL fragments will be altered.
	NoDialect Dialect = iota
	// PostgreSQL mode can be set for a statement:
	//
	//     q := sqlf.PostgreSQL.From("table").Select("field")
	//     ...
	//     q.Close()
	//
	// or as default mode:
	//
	//     sqlf.SetDialect(sqlf.PostgreSQL)
	//	   ...
	//     q := sqlf.From("table").Select("field")
	//     q.Close()
	//
	// Wher PostgreSQL mode is activated, ? placeholders are
	// replaced with numbered positional arguments like $1, $2...
	PostgreSQL Dialect = iota
)

var defaultDialect = NoDialect

/*
SetDialect selects a Dialect to be used by default.

Dialect can be one of sqlf.NoDialect() and sqlf.PostgreSQL()

	sqlf.SetDialect(sqlf.PostgreSQL())
*/
func SetDialect(dialect Dialect) {
	atomic.StoreUint32((*uint32)(&defaultDialect), uint32(dialect))
}

/*
New starts an SQL statement with an arbitrary verb.

Use From, Select, InsertInto or DeleteFrom methods to create
an instance of an SQL statement builder for common cases.
*/
func (b Dialect) New(verb string, args ...interface{}) *Stmt {
	q := getStmt(b)
	q.clause(posSelect, verb, args...)
	return q
}

/*
From starts a SELECT statement.
*/
func (b Dialect) From(expr string, args ...interface{}) *Stmt {
	q := getStmt(b)
	return q.From(expr, args...)
}

/*
Select starts a SELECT statement.

Consider using From method to start a SELECT statement - you may find
it easier to read and maintain.
*/
func (b Dialect) Select(expr string, args ...interface{}) *Stmt {
	q := getStmt(b)
	return q.Select(expr, args...)
}

// Update starts an UPDATE statement.
func (b Dialect) Update(tableName string) *Stmt {
	q := getStmt(b)
	return q.Update(tableName)
}

// InsertInto starts an INSERT statement.
func (b Dialect) InsertInto(tableName string) *Stmt {
	q := getStmt(b)
	return q.InsertInto(tableName)
}

// DeleteFrom starts a DELETE statement.
func (b Dialect) DeleteFrom(tableName string) *Stmt {
	q := getStmt(b)
	return q.DeleteFrom(tableName)
}

// writePg function copies s into buf and replaces ? placeholders with $1, $2...
func writePg(argNo int64, s []byte, buf *bytebufferpool.ByteBuffer) (int64, error) {
	var err error
	start := 0
	// Iterate by runes
	for pos, r := range bufToString(&s) {
		if start > pos {
			continue
		}
		switch r {
		case '\\':
			if pos < len(s)-1 && s[pos+1] == '?' {
				_, err = buf.Write(s[start:pos])
				if err == nil {
					err = buf.WriteByte('?')
				}
				start = pos + 2
			}
		case '?':
			_, err = buf.Write(s[start:pos])
			start = pos + 1
			if err == nil {
				err = buf.WriteByte('$')
				if err == nil {
					buf.B = strconv.AppendInt(buf.B, int64(argNo), 10)
					argNo++
				}
			}
		}
		if err != nil {
			break
		}
	}
	if err == nil && start < len(s) {
		_, err = buf.Write(s[start:])
	}
	return argNo, err
}
