package sqlf

import (
	"strconv"
	"strings"
	"sync"
)

// Dialect defines the method SQL statement is to be built.
//
// NoDialect is a default statement builder mode.
// No SQL fragments will be altered.
// PostgreSQL mode can be set for a statement:
//
//	q := sqlf.PostgreSQL.From("table").Select("field")
//		...
//	q.Close()
//
// or as default mode:
//
//	    sqlf.SetDialect(sqlf.PostgreSQL)
//		   ...
//	    q := sqlf.From("table").Select("field")
//	    q.Close()
//
// When PostgreSQL mode is activated, ? placeholders are
// replaced with numbered positional arguments like $1, $2...
type Dialect struct {
	cacheOnce sync.Once
	cacheLock sync.RWMutex
	cache     sqlCache
}

var (
	// NoDialect is a default statement builder mode.
	NoDialect *Dialect = &Dialect{}
	// PostgreSQL mode is to be used to automatically replace ? placeholders with $1, $2...
	PostgreSQL *Dialect = &Dialect{}
)

var defaultDialectPointer = newAtomicPointer(NoDialect)

/*
SetDialect selects a Dialect to be used by default.

Dialect can be one of sqlf.NoDialect or sqlf.PostgreSQL

	sqlf.SetDialect(sqlf.PostgreSQL)
*/
func SetDialect(newDefaultDialect *Dialect) {
	defaultDialectPointer.Store(newDefaultDialect)
}

/*
New starts an SQL statement with an arbitrary verb.

Use From, Select, InsertInto or DeleteFrom methods to create
an instance of an SQL statement builder for common statements.
*/
func (b *Dialect) New(verb string, args ...interface{}) *Stmt {
	q := getStmt(b)
	q.addChunk(posSelect, verb, "", args, ", ")
	return q
}

/*
With starts a statement prepended by WITH clause
and closes a subquery passed as an argument.
*/
func (b *Dialect) With(queryName string, query *Stmt) *Stmt {
	q := getStmt(b)
	return q.With(queryName, query)
}

/*
From starts a SELECT statement.
*/
func (b *Dialect) From(expr string, args ...interface{}) *Stmt {
	q := getStmt(b)
	return q.From(expr, args...)
}

/*
Select starts a SELECT statement.

Consider using From method to start a SELECT statement - you may find
it easier to read and maintain.
*/
func (b *Dialect) Select(expr string, args ...interface{}) *Stmt {
	q := getStmt(b)
	return q.Select(expr, args...)
}

// Update starts an UPDATE statement.
func (b *Dialect) Update(tableName string) *Stmt {
	q := getStmt(b)
	return q.Update(tableName)
}

// InsertInto starts an INSERT statement.
func (b *Dialect) InsertInto(tableName string) *Stmt {
	q := getStmt(b)
	return q.InsertInto(tableName)
}

// DeleteFrom starts a DELETE statement.
func (b *Dialect) DeleteFrom(tableName string) *Stmt {
	q := getStmt(b)
	return q.DeleteFrom(tableName)
}

// writePg function copies s into buf and replaces ? placeholders with $1, $2...
func writePg(argNo int, s []byte, buf *strings.Builder) (int, error) {
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
					buf.WriteString(strconv.Itoa(argNo))
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
