package sqlf

import (
	"strings"

	"github.com/valyala/bytebufferpool"
)

/*
New initializes a SQL statement builder instance with an arbitrary verb.

Use sqlf.Select(), sqlf.InsertInto(), sqlf.DeleteFrom() to start
common SQL statements.

Use New for special cases like this:

	q := sqlf.New("TRANCATE")
	for _, table := range tableNames {
		q.Expr(table)
	}
	q.Clause("RESTART IDENTITY")
	err := q.ExecAndClose(ctx, db)
	if err != nil {
		panic(err)
	}
*/
func New(verb string, args ...interface{}) *Stmt {
	return defaultDialect.New(verb, args...)
}

/*
From starts a SELECT statement.

	var cnt int64

	err := sqlf.From("table").
		Select("COUNT(*)").To(&cnt)
		Where("value >= ?", 42).
		QueryRowAndClose(ctx, db)
	if err != nil {
		panic(err)
	}
*/
func From(expr string, args ...interface{}) *Stmt {
	return defaultDialect.From(expr, args...)
}

/*
Select starts a SELECT statement.

	var cnt int64

	err := sqlf.Select("COUNT(*)").To(&cnt).
		From("table").
		Where("value >= ?", 42).
		QueryRowAndClose(ctx, db)
	if err != nil {
		panic(err)
	}

Note that From method can also be used to start a SELECT statement.
*/
func Select(expr string, args ...interface{}) *Stmt {
	return defaultDialect.Select(expr, args...)
}

/*
Update starts an UPDATE statement.

	err := sqlf.Update("table").
		Set("field1", "newvalue").
		Where("id = ?", 42).
		ExecAndClose(ctx, db)
	if err != nil {
		panic(err)
	}
*/
func Update(tableName string) *Stmt {
	return defaultDialect.Update(tableName)
}

/*
InsertInto starts an INSERT statement.

	var newId int64
	err := sqlf.InsertInto("table").
		Set("field", value).
		Returning("id").To(&newId).
		ExecAndClose(ctx, db)
	if err != nil {
		panic(err)
	}
*/
func InsertInto(tableName string) *Stmt {
	return defaultDialect.InsertInto(tableName)
}

/*
DeleteFrom starts a DELETE statement.

	err := sqlf.DeleteFrom("table").Where("id = ?", id).ExecAndClose(ctx, db)
*/
func DeleteFrom(tableName string) *Stmt {
	return defaultDialect.DeleteFrom(tableName)
}

type stmtChunk struct {
	pos     int
	bufLow  int
	bufHigh int
	hasExpr bool
	argLen  int
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

For other SQL statements use New:

	q := sqlf.New("TRUNCATE")
	for _, table := range tablesToBeEmptied {
		q.Expr(table)
	}
	err := q.ExecAndClose(ctx, db)
	if err != nil {
		panic(err)
	}
*/
type Stmt struct {
	dialect Dialect
	pos     int
	chunks  stmtChunks
	buf     *bytebufferpool.ByteBuffer
	sql     *bytebufferpool.ByteBuffer
	args    []interface{}
	dest    []interface{}
}

/*
Select adds a SELECT clause to a statement and/or appends
an expression that defines columns of a resulting data set.

	q := sqlf.Select("DISTINCT field1, field2").From("table")

Select can be called multiple times to add more columns:

	q := sqlf.From("table").Select("field1")
	if needField2 {
		q.Select("field2")
	}
	// ...
	q.Close()

Use To method to bind variables to selected columns:

	var (
		num  int
		name string
	)

	res := sqlf.From("table").
		Select("num, name").To(&num, &name).
		Where("id = ?", 42).
		QueryRowAndClose(ctx, db)
	if err != nil {
		panic(err)
	}

Note that a SELECT statement can also be started by a From method call.
*/
func (q *Stmt) Select(expr string, args ...interface{}) *Stmt {
	q.addChunk(posSelect, "SELECT", expr, args, ", ")
	return q
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
method that defines data to be returned.
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
	q.addChunk(posUpdate, "UPDATE", tableName, nil, ", ")
	return q
}

/*
InsertInto adds INSERT INTO clause to a statement.

	q.InsertInto("table")

tableName argument can be a SQL fragment:

	q.InsertInto("table AS t")
*/
func (q *Stmt) InsertInto(tableName string) *Stmt {
	q.addChunk(posInsert, "INSERT INTO", tableName, nil, ", ")
	q.addChunk(posInsertFields-1, "(", "", nil, "")
	q.addChunk(posValues-1, ") VALUES (", "", nil, "")
	q.addChunk(posValues+1, ")", "", nil, "")
	q.pos = posInsertFields
	return q
}

/*
DeleteFrom adds DELETE clause to a statement.

	q.DeleteFrom("table").Where("id = ?", id)
*/
func (q *Stmt) DeleteFrom(tableName string) *Stmt {
	q.addChunk(posDelete, "DELETE FROM", tableName, nil, ", ")
	return q
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
		q.addChunk(posInsertFields, "", field, nil, ", ")
		q.addChunk(posValues, "", expr, args, ", ")
	case posUpdate:
		q.addChunk(posSet, "SET", field+"="+expr, args, ", ")
	}
	return q
}

// From adds a FROM clause to statement.
func (q *Stmt) From(expr string, args ...interface{}) *Stmt {
	q.addChunk(posFrom, "FROM", expr, args, ", ")
	return q
}

/*
Where adds a filter:

	sqlf.From("users").
		Select("id, name").
		Where("email = ?", email).
		Where("is_active = 1")

*/
func (q *Stmt) Where(expr string, args ...interface{}) *Stmt {
	q.addChunk(posWhere, "WHERE", expr, args, " AND ")
	return q
}

/*
In adds IN expression to the current filter.

In method must be called after a Where method call.
*/
func (q *Stmt) In(args ...interface{}) *Stmt {
	buf := bytebufferpool.Get()
	buf.WriteString("IN (")
	l := len(args) - 1
	for i := range args {
		if i < l {
			buf.Write(placeholderComma)
		} else {
			buf.Write(placeholder)
		}
	}
	buf.WriteString(")")

	q.addChunk(posWhere, "", bufToString(&buf.B), args, " ")

	bytebufferpool.Put(buf)
	return q
}

/*
Join adds an INNERT JOIN clause to SELECT statement
*/
func (q *Stmt) Join(table, on string) *Stmt {
	q.join("JOIN ", table, on)
	return q
}

/*
LeftJoin adds a LEFT OUTER JOIN clause to SELECT statement
*/
func (q *Stmt) LeftJoin(table, on string) *Stmt {
	q.join("LEFT JOIN ", table, on)
	return q
}

/*
RightJoin adds a RIGHT OUTER JOIN clause to SELECT statement
*/
func (q *Stmt) RightJoin(table, on string) *Stmt {
	q.join("RIGHT JOIN ", table, on)
	return q
}

/*
FullJoin adds a FULL OUTER JOIN clause to SELECT statement
*/
func (q *Stmt) FullJoin(table, on string) *Stmt {
	q.join("FULL JOIN ", table, on)
	return q
}

// OrderBy adds the ORDER BY clause to SELECT statement
func (q *Stmt) OrderBy(expr ...string) *Stmt {
	q.addChunk(posOrderBy, "ORDER BY", strings.Join(expr, ", "), nil, ", ")
	return q
}

// GroupBy adds the GROUP BY clause to SELECT statement
func (q *Stmt) GroupBy(expr string) *Stmt {
	q.addChunk(posGroupBy, "GROUP BY", expr, nil, ", ")
	return q
}

// Having adds the HAVING clause to SELECT statement
func (q *Stmt) Having(expr string, args ...interface{}) *Stmt {
	q.addChunk(posHaving, "HAVING", expr, args, " AND ")
	return q
}

// Limit adds a limit on number of returned rows
func (q *Stmt) Limit(limit interface{}) *Stmt {
	q.addChunk(posLimit, "LIMIT ?", "", []interface{}{limit}, "")
	return q
}

// Offset adds a limit on number of returned rows
func (q *Stmt) Offset(offset interface{}) *Stmt {
	q.addChunk(posOffset, "OFFSET ?", "", []interface{}{offset}, "")
	return q
}

// Paginate provides an easy way to set both offset and limit
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
	q.addChunk(posReturning, "RETURNING", expr, nil, ", ")
	return q
}

// With prepends a statement with an WITH clause.
// With method calls a Close method of a given query, so
// make sure not to reuse it afterwards.
func (q *Stmt) With(queryName string, query *Stmt) *Stmt {
	q.addChunk(posWith, "WITH", "", nil, "")
	return q.SubQuery(queryName+" AS (", ")", query)
}

/*
Expr appends an expression to the most recently added clause.

Expressions are separated with commas.
*/
func (q *Stmt) Expr(expr string, args ...interface{}) *Stmt {
	q.addChunk(q.pos, "", expr, args, ", ")
	return q
}

/*
SubQuery appends a sub query expression to a current clause.

SubQuery method call closes the Stmt passed as query parameter.
Do not reuse it afterwards.
*/
func (q *Stmt) SubQuery(prefix, suffix string, query *Stmt) *Stmt {
	delimiter := ", "
	if q.pos == posWhere {
		delimiter = " AND "
	}
	index := q.addChunk(q.pos, "", prefix, query.args, delimiter)
	chunk := &q.chunks[index]
	// Make sure subquery is not dialect-specific.
	if query.dialect != NoDialect {
		query.dialect = NoDialect
		query.Invalidate()
	}
	q.buf.WriteString(query.String())
	q.buf.WriteString(suffix)
	chunk.bufHigh = q.buf.Len()
	// Close the subquery
	query.Close()

	return q
}

/*
Union adds a UNION clause to the statement.

all argument controls if UNION ALL or UNION clause
is to be constructed. Use UNION ALL if possible to
get faster queries.
*/
func (q *Stmt) Union(all bool, query *Stmt) *Stmt {
	p := posUnion
	if len(q.chunks) > 0 {
		last := (&q.chunks[len(q.chunks)-1]).pos
		if last >= p {
			p = last + 1
		}
	}
	var index int
	if all {
		index = q.addChunk(p, "UNION ALL ", "", query.args, "")
	} else {
		index = q.addChunk(p, "UNION ", "", query.args, "")
	}
	chunk := &q.chunks[index]
	// Make sure subquery is not dialect-specific.
	if query.dialect != NoDialect {
		query.dialect = NoDialect
		query.Invalidate()
	}
	q.buf.WriteString(query.String())
	chunk.bufHigh = q.buf.Len()
	// Close the subquery
	query.Close()

	return q
}

/*
Clause appends a raw SQL fragment to the statement.

Use it to add a raw SQL fragment like ON CONFLICT, ON DUPLICATE KEY, WINDOW, etc.

An SQL fragment added via Clause method appears after the last clause previously
added. If called first, Clause method prepends a statement with a raw SQL.
*/
func (q *Stmt) Clause(expr string, args ...interface{}) *Stmt {
	p := posStart
	if len(q.chunks) > 0 {
		p = (&q.chunks[len(q.chunks)-1]).pos + 10
	}
	q.addChunk(p, expr, "", args, ", ")
	return q
}

// String method builds and returns an SQL statement.
func (q *Stmt) String() string {
	if q.sql == nil {
		var argNo int64 = 1
		// Build a query
		buf := getBuffer()
		q.sql = buf

		pos := 0
		for n, chunk := range q.chunks {
			// Separate clauses with spaces
			if n > 0 && chunk.pos > pos {
				buf.Write(space)
			}
			s := q.buf.B[chunk.bufLow:chunk.bufHigh]
			if chunk.argLen > 0 && q.dialect == PostgreSQL {
				argNo, _ = writePg(argNo, s, buf)
			} else {
				buf.Write(s)
			}
			pos = chunk.pos
		}
	}
	return bufToString(&q.sql.B)
}

/*
Args returns the list of arguments to be passed to
database driver for statement execution.

Do not access a slice returned by this method after Stmt is closed.

An array, a returned slice points to, can be altered by any method that
adds a clause or an expression with arguments.

Make sure to make a copy of the returned slice if you need to preserve it.
*/
func (q *Stmt) Args() []interface{} {
	return q.args
}

/*
Dest returns a list of value pointers passed via To method calls.
The order matches the constructed SQL statement.

Do not access a slice returned by this method after Stmt is closed.

Note that an array, a returned slice points to, can be altered by To method
calls.

Make sure to make a copy if you need to preserve a slice returned by this method.
*/
func (q *Stmt) Dest() []interface{} {
	return q.dest
}

/*
Invalidate forces a rebuild on next query execution.

Most likely you don't need to call this method directly.
*/
func (q *Stmt) Invalidate() {
	if q.sql != nil {
		putBuffer(q.sql)
		q.sql = nil
	}
}

/*
Close puts buffers and other objects allocated to build an SQL statement
back to pool for reuse by other Stmt instances.

Stmt instance should not be used after Close method call.
*/
func (q *Stmt) Close() {
	reuseStmt(q)
}

// Clone creates a copy of the statement.
func (q *Stmt) Clone() *Stmt {
	stmt := getStmt(q.dialect)
	if cap(stmt.chunks) < len(q.chunks) {
		stmt.chunks = make(stmtChunks, len(q.chunks), len(q.chunks)+2)
		copy(stmt.chunks, q.chunks)
	} else {
		stmt.chunks = append(stmt.chunks, q.chunks...)
	}
	stmt.args = insertAt(stmt.args, q.args, 0)
	stmt.dest = insertAt(stmt.dest, q.dest, 0)
	stmt.buf.Write(q.buf.B)
	if q.sql != nil {
		stmt.sql = getBuffer()
		stmt.sql.Write(q.sql.B)
	}

	return stmt
}

// join adds a join clause to a SELECT statement
func (q *Stmt) join(joinType, table, on string) (index int) {
	buf := bytebufferpool.Get()
	buf.WriteString(joinType)
	buf.WriteString(table)
	buf.Write(joinOn)
	buf.WriteString(on)
	buf.WriteByte(')')

	index = q.addChunk(posFrom, "", bufToString(&buf.B), nil, " ")

	bytebufferpool.Put(buf)

	return index
}

// addChunk adds a clause or expression to a statement.
func (q *Stmt) addChunk(pos int, clause, expr string, args []interface{}, sep string) (index int) {
	// Remember the position
	q.pos = pos

	argLen := len(args)
	bufLow := len(q.buf.B)
	index = len(q.chunks)
	argTail := 0

	addNew := true
	addClause := clause != ""

	// Find the position to insert a chunk to
loop:
	for i := index - 1; i >= 0; i-- {
		chunk := &q.chunks[i]
		index = i
		switch {
		// See if an existing chunk can be extended
		case chunk.pos == pos:
			// Do nothing if a clause is already there and no expressions are to be added
			if expr == "" {
				return i
			}
			// Write a separator
			if chunk.hasExpr {
				q.buf.WriteString(sep)
			} else {
				q.buf.WriteString(" ")
			}
			if chunk.bufHigh == bufLow {
				// Do not add a chunk
				addNew = false
				// Update the existing one
				q.buf.WriteString(expr)
				chunk.argLen += argLen
				chunk.bufHigh = len(q.buf.B)
				chunk.hasExpr = true
			} else {
				// Do not add a clause
				addClause = false
				index = i + 1
			}
			break loop
		// No existing chunks of this type
		case chunk.pos < pos:
			index = i + 1
			break loop
		default:
			argTail += chunk.argLen
		}
	}

	if addNew {
		// Insert a new chunk
		if addClause {
			q.buf.WriteString(clause)
			if expr != "" {
				q.buf.WriteString(" ")
			}
		}
		q.buf.WriteString(expr)

		if cap(q.chunks) == len(q.chunks) {
			chunks := make(stmtChunks, len(q.chunks), cap(q.chunks)*2)
			copy(chunks, q.chunks)
			q.chunks = chunks
		}

		chunk := stmtChunk{
			pos:     pos,
			bufLow:  bufLow,
			bufHigh: len(q.buf.B),
			argLen:  argLen,
			hasExpr: expr != "",
		}

		q.chunks = append(q.chunks, chunk)
		if index < len(q.chunks)-1 {
			copy(q.chunks[index+1:], q.chunks[index:])
			q.chunks[index] = chunk
		}
	}

	// Insert query arguments
	if argLen > 0 {
		q.args = insertAt(q.args, args, len(q.args)-argTail)
	}
	q.Invalidate()

	return index
}

var (
	space            = []byte{' '}
	placeholder      = []byte{'?'}
	placeholderComma = []byte{'?', ','}
	joinOn           = []byte{' ', 'O', 'N', ' ', '('}
)

const (
	_        = iota
	posStart = 100 * iota
	posWith
	posInsert
	posInsertFields
	posValues
	posDelete
	posUpdate
	posSet
	posSelect
	posInto
	posFrom
	posWhere
	posGroupBy
	posHaving
	posUnion
	posOrderBy
	posLimit
	posOffset
	posReturning
	posEnd
)
