package sqlf

import (
	"fmt"
	"strings"

	"github.com/valyala/bytebufferpool"
)

/*
TODO:

1. Postgresql arguments (done)
2. Missing methods:

	.Union(method, query)   UNION, UNION ALL, INTERSECT, EXCEPT

	.SubQuery(query)

	.Wrap(prefix, suffix) - .Wrap("exists (", ")")
	.With(expression, query)

3. Builder optimisation (global cache of chunks, binary heap for chunks)
4. Try channel-based storage for Buffer and reused Stmt instances (instead of sync.Pool) - done
5. Cache built queries (https://github.com/cespare/xxhash)?
6. Use ByteBuffer for chunks
*/

var defaultBuilder = &Builder{Dialect: NoDialect()}

/*
SetDialect selects a Dialect to be used by default Builder

	sqlf.SetDialect(sqlf.PostgreSQL())
*/
func SetDialect(dialect Dialect) {
	defaultBuilder.Dialect = dialect
}

// NewBuilder creates a new SQL builder instance.
func NewBuilder(dialect Dialect) *Builder {
	return &Builder{Dialect: dialect}
}

/*
Builder defines a way SQL statements are built.

In most cases a default builder can be used:

	sqlf.SetDialect(sqlf.PostgreSQL())
	// ...
	q := sqlf.From("user").Select("name").Where("id = ?", 42)
	// Produces
	// SELECT name FROM user WHERE id = $1

Create a Builder instance if an application needs to access multiple
database engines:

	mysqlBuilder := &sqlf.Builder{}
	pgBuilder := sqlf.Builder{dialect: sqlf.PostgreSQL()}

	qMy := mysqlBuilder.From("table").Select("field").Where("id = ?", 42)
	// ...
	qMy.Close()
	// ...
	qPg := pgBuilder.From("table").Select("field").Where("id = ?", 24)
	// ...
	qPg.Close()
*/
type Builder struct {
	Dialect Dialect
}

/*
New initializes a SQL statement builder instance with an arbitrary verb.

Use Select, InsertInto, DeleteFrom methods to create
an instance of an SQL statement builder for common cases.
*/
func (b *Builder) New(verb string, args ...interface{}) *Stmt {
	q := getStmt(b)
	q.clause(posSelect, verb, args...)
	return q
}

/*
From creates a SELECT statement builder.
*/
func (b *Builder) From(expr string, args ...interface{}) *Stmt {
	q := getStmt(b)
	return q.From(expr, args...)
}

/*
Select creates a SELECT statement builder.

Consider using From method to start a SELECT statement - you may find
it easier to read and maintain.
*/
func (b *Builder) Select(expr string, args ...interface{}) *Stmt {
	q := getStmt(b)
	return q.Select(expr, args...)
}

// Update creates an UPDATE statement builder.
func (b *Builder) Update(tableName string) *Stmt {
	q := getStmt(b)
	return q.Update(tableName)
}

// InsertInto creates an INSERT statement builder.
func (b *Builder) InsertInto(tableName string) *Stmt {
	q := getStmt(b)
	return q.InsertInto(tableName)
}

// DeleteFrom creates a DELETE statement builder.
func (b *Builder) DeleteFrom(tableName string) *Stmt {
	q := getStmt(b)
	return q.DeleteFrom(tableName)
}

/*
New initializes a SQL statement builder instance with an arbitrary verb.

Use sqlf.Select(), sqlf.Insert(), sqlf.Delete() to create
an instance of a SQL statement builder for common cases.
*/
func New(verb string, args ...interface{}) *Stmt {
	return defaultBuilder.New(verb, args...)
}

/*
From creates a SELECT statement builder.
*/
func From(expr string, args ...interface{}) *Stmt {
	return defaultBuilder.From(expr, args...)
}

/*
Select creates a SELECT statement builder.

Note that From method can also be used to start a SELECT statement.
*/
func Select(expr string, args ...interface{}) *Stmt {
	return defaultBuilder.Select(expr, args...)
}

/*
Update creates an UPDATE statement builder.
*/
func Update(tableName string) *Stmt {
	return defaultBuilder.Update(tableName)
}

/*
InsertInto creates an INSERT statement builder.
*/
func InsertInto(tableName string) *Stmt {
	return defaultBuilder.InsertInto(tableName)
}

/*
DeleteFrom creates a DELETE statement builder.
*/
func DeleteFrom(tableName string) *Stmt {
	return defaultBuilder.DeleteFrom(tableName)
}

type stmtChunk struct {
	pos    int
	sql    string
	sep    string
	argLen int
}
type stmtChunks []stmtChunk

/*
Stmt provides a set of helper methods for SQL statement building and execution.

Use one of the following methods to create a SQL statement builder instance:

	sqlf.From("table")
	sqlf.Select("field")
    sqlf.InsertInto("table")
	sqlf.Update("table")
    sqlf.DeleteFrom("table")

For an arbitrary SQL statement use New:

    q := sqlf.New("TRUNCATE")
    for _, table := range tablesToBeEmptied {
        q.Expr(table)
    }
	err := q.ExecContext(r.Context(), db)
	q.Close()
*/
type Stmt struct {
	builder *Builder
	pos     int
	chunks  stmtChunks
	sql     string
	args    []interface{}
	dest    []interface{}
}

/*
Select adds a SELECT clause to a statement

	q := sqlf.Select("field1, field2").From("table")

Select can be called at any moment to add an expression to the list:

	q := sqlf.Select("field1").From("table")
	if needField2 {
		q.Select("field2")
	}

Note that a SELECT statement can also be started with call to From:

	q := sqlf.From("table").
		Select("field1")
	// ...
	q.Close()
*/
func (q *Stmt) Select(expr string, args ...interface{}) *Stmt {
	q.clause(posSelect, "SELECT")
	return q.Expr(expr, args...)
}

/*
To sets a scan target for columns to be selected.

Accepts value pointers to be passed to sql.Rows.Scan by
Query and QueryRow methods.

	var (
		field1 int
		field2 string
	)
	q := sqlf.From("table").
		Select("field1").To(&field1).
		Select("field2").To(&field2)
	err := QueryRow(nil, db)
	q.Close()
	if err != nil {
		// ...
	}

To method MUST be called immediately after Select, Returning or other
method that defines data to be returned. This will help to maintain the
proper order of value pointers passed to Scan.
*/
func (q *Stmt) To(dest ...interface{}) *Stmt {
	if len(dest) > 0 {
		// As Scan bindings make sense for a single clause per statement,
		// the order expressions appear in SQL matches the order expressions
		// are added. So dest value pointers can safely be appended
		// to the list on every To call.
		q.dest = insertAt(q.dest, dest, len(q.dest))
	}
	return q
}

/*
Update adds UPDATE clause to a statement.

	q.Update("table")

tableName argument can be a SQL fragment:

	q.Update("ONLY table AS t")
*/
func (q *Stmt) Update(tableName string) *Stmt {
	q.clause(posUpdate, "UPDATE")
	return q.Expr(tableName)
}

/*
InsertInto adds INSERT INTO clause to a statement.

	q.InsertInto("table")

tableName argument can be a SQL fragment:

	q.InsertInto("table AS t")
*/
func (q *Stmt) InsertInto(tableName string) *Stmt {
	q.clause(posInsert, "INSERT INTO")
	q.Expr(tableName)
	q.clause(posInsertFields, "(")
	q.clause(posValues, ") VALUES (")
	q.clause(posValuesEnd, ")")
	q.pos = posInsertFields
	return q
}

/*
DeleteFrom adds DELETE clause to a statement.

	q.DeleteFrom("table").Where("id = ?", id)
*/
func (q *Stmt) DeleteFrom(tableName string) *Stmt {
	q.clause(posDelete, "DELETE FROM")
	return q.Expr(tableName)
}

/*
Set method:
- Adds a column to the list of columns and a value to VALUES clause of INSERT statement,
- Adds an item to SET clause of an UPDATE statement.

	q.Set("field", 32)

For INSERT statements a call to Set method generates
both the list of columns and values to be inserted:

	q := sqlf.InsertInto("table").Set("field", 42)

produces

	INSERT INTO table (field) VALUES (42)
*/
func (q *Stmt) Set(field string, value interface{}) *Stmt {
	return q.SetExpr(field, "?", value)
}

/*
SetExpr is an extended version of a Set method.

	q.SetExpr("field", "field + 1")
	q.SetExpr("field", "? + ?", 31, 11)
*/
func (q *Stmt) SetExpr(field, expr string, args ...interface{}) *Stmt {
	// TODO How to handle both INSERT ... VALUES and SET in ON DUPLICATE KEY UPDATE?
	p := 0
	for _, chunk := range q.chunks {
		if chunk.pos == posInsert || chunk.pos == posUpdate {
			p = chunk.pos
			break
		}
	}

	switch p {
	case posInsert:
		q.addChunk(posInsertFields, field, nil, ", ")
		q.addChunk(posValues, expr, args, ", ")
	case posUpdate:
		q.clause(posSet, "SET")
		q.Expr(field+"="+expr, args...)
	}
	return q
}

// From adds a FROM clause to statement.
func (q *Stmt) From(expr string, args ...interface{}) *Stmt {
	q.clause(posFrom, "FROM")
	return q.Expr(expr, args...)
}

/*
Where adds a filter:

	sqlf.Select("id, name").From("users").Where("email = ?", email).Where("is_active = 1")

*/
func (q *Stmt) Where(expr string, args ...interface{}) *Stmt {
	q.clause(posWhere, "WHERE")
	q.addChunk(q.pos, expr, args, " AND ")
	return q
}

// OrderBy adds the ORDER BY clause to SELECT statement
func (q *Stmt) OrderBy(expr ...string) *Stmt {
	q.clause(posOrderBy, "ORDER BY")
	q.addChunk(q.pos, strings.Join(expr, ", "), nil, ", ")
	return q
}

// GroupBy adds the GROUP BY clause to SELECT statement
func (q *Stmt) GroupBy(expr string) *Stmt {
	q.clause(posGroupBy, "GROUP BY")
	q.addChunk(q.pos, expr, nil, ", ")
	return q
}

// Having adds the HAVING clause to SELECT statement
func (q *Stmt) Having(expr string, args ...interface{}) *Stmt {
	q.clause(posHaving, "HAVING")
	q.addChunk(q.pos, expr, args, " AND ")
	return q
}

// Limit adds a limit on number of returned rows
func (q *Stmt) Limit(limit int) *Stmt {
	q.clause(posLimit, fmt.Sprintf("LIMIT %d", limit))
	return q
}

// Offset adds a limit on number of returned rows
func (q *Stmt) Offset(offset int) *Stmt {
	q.clause(posOffset, fmt.Sprintf("OFFSET %d", offset))
	return q
}

// Paginate provides an easy way to set offset and limit
func (q *Stmt) Paginate(page, pageSize int) *Stmt {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 1
	}
	if page > 1 {
		q.Offset((page - 1) * pageSize)
	}
	q.Limit(pageSize)
	return q
}

// Returning adds a RETURNING clause to a statement
func (q *Stmt) Returning(expr string) *Stmt {
	q.clause(posReturning, "RETURNING")
	q.addChunk(q.pos, expr, nil, " , ")
	return q
}

/*
Expr appends an expression to a current clause of a statement.

Expression is basically a comma-separated part of an SQL statement.

Most common expressions are: lists of fields to be selected or updated,
filtering (WHERE) conditions, ORDER BY clause elements.

There are helper methods like .Select(), .From(), etc for these common cases,
so you likely won't need to call .Expr() directly.

But you may find it useful for special cases, like multiple WINDOW definitions.
*/
func (q *Stmt) Expr(expr string, args ...interface{}) *Stmt {
	q.addChunk(q.pos, expr, args, ", ")
	return q
}

/*
Clause adds a clause to a statement.

    q := sqlf.Select("sum(salary) OVER w").From("empsalary")
    q.Clause("WINDOW w AS (PARTITION BY depname ORDER BY salary DESC)")

*/
func (q *Stmt) Clause(expr string, args ...interface{}) *Stmt {
	p := posEnd
	for _, chunk := range q.chunks {
		if chunk.pos > q.pos {
			p = chunk.pos + 1
		}
	}
	q.clause(p, expr, args...)
	return q
}

// Build returns SQL statement and list of arguments to be passed to database driver for execution.
func (q *Stmt) Build() (sql string, args []interface{}) {
	sql = q.sql
	if sql == "" {
		ctx := q.builder.Dialect.NewCtx()
		if ctx != nil {
			defer ctx.Close()
		}
		// Build a query
		buf := bytebufferpool.Get()

		sep := ""
		pos := 0
		for _, chunk := range q.chunks {
			if chunk.pos > pos && sep != "" {
				sep = " "
				pos = chunk.pos
			}
			// Ignore empty strings
			if l := len(chunk.sql); l > 0 {
				if sep != "" {
					q.builder.Dialect.WriteString(ctx, sep, buf, 0)
				}
				q.builder.Dialect.WriteString(ctx, chunk.sql, buf, chunk.argLen)
				sep = chunk.sep
			}
		}

		sql = buf.String()
		q.sql = sql

		bytebufferpool.Put(buf)
	}
	return sql, q.args
}

// Dest returns a list of value pointers passed via To method calls.
// The order matches the constructed SQL statement.
func (q *Stmt) Dest() []interface{} {
	return q.dest
}

// Invalidate forces a rebuild on next query execution
func (q *Stmt) Invalidate() {
	q.sql = ""
}

/*
Close can be used to reuse memory allocated for SQL statement builder instances:

	var (
		field1 int
		field2 string
	)
	q := sqlf.From("table").
		Select("field1").To(&field1).
		Select("field2").To(&field2)
	err := QueryRow(nil, db)
	q.Close()

Stmt instance should not be used after Close method call.
*/
func (q *Stmt) Close() {
	q.chunks = q.chunks[:0]
	q.sql = ""
	if len(q.args) > 0 {
		for n := range q.args {
			q.args[n] = nil
		}
		q.args = q.args[:0]
	}
	if len(q.dest) > 0 {
		for n := range q.dest {
			q.dest[n] = nil
		}
		q.dest = q.dest[:0]
	}
	reuseStmt(q)
}

// addChunk adds a clause or expression to a statement.
func (q *Stmt) addChunk(pos int, expr string, args []interface{}, sep string) *stmtChunk {
	argLen := len(args)
	newChunk := stmtChunk{
		pos:    pos,
		sep:    sep,
		sql:    expr,
		argLen: argLen,
	}
	index := len(q.chunks)
	argPos := 0
	// Find the position to insert a chunk to
	for n, chunk := range q.chunks {
		if chunk.pos > pos {
			index = n
			break
		}
		argPos += chunk.argLen
	}

	// Insert a chunk
	if cap(q.chunks) == len(q.chunks) {
		chunks := make(stmtChunks, len(q.chunks), cap(q.chunks)*2)
		copy(chunks, q.chunks)
		q.chunks = chunks
	}

	q.chunks = append(q.chunks, newChunk)
	if index < len(q.chunks)-1 {
		copy(q.chunks[index+1:], q.chunks[index:])
		q.chunks[index] = newChunk
	}

	// Insert query arguments
	if argLen > 0 {
		q.args = insertAt(q.args, args, argPos)
	}
	q.Invalidate()

	return &q.chunks[index]
}

// clause adds a clause at given pos unless there is one
func (q *Stmt) clause(pos int, expr string, args ...interface{}) *stmtChunk {
	// Save pos for Expr calls
	q.pos = pos
	// See if clause was already added
	for _, chunk := range q.chunks {
		if chunk.pos == pos {
			return nil
		}
	}
	return q.addChunk(pos, expr, args, " ")
}

const (
	_        = iota
	posStart = 100 * iota
	posWith
	posWithEnd
	posInsert
	posInsertFields
	posValues
	posValuesEnd
	posDelete
	posUpdate
	posSet
	posSelect
	posInto
	posFrom
	posWhere
	posGroupBy
	posHaving
	posOrderBy
	posLimit
	posOffset
	posReturning
	posEnd
)
