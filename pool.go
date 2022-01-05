package sqlf

import (
	"sync"

	"github.com/valyala/bytebufferpool"
)

var (
	stmtPool = sync.Pool{New: newStmt}
)

func newStmt() interface{} {
	return &Stmt{
		chunks: make(stmtChunks, 0, 8),
	}
}

func getStmt(d *Dialect) *Stmt {
	stmt := stmtPool.Get().(*Stmt)
	stmt.dialect = d
	stmt.buf = getBuffer()
	return stmt
}

func reuseStmt(q *Stmt) {
	q.chunks = q.chunks[:0]
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
	putBuffer(q.buf)
	q.buf = nil
	q.sql = ""

	stmtPool.Put(q)
}

func getBuffer() *bytebufferpool.ByteBuffer {
	return bytebufferpool.Get()
}

func putBuffer(buf *bytebufferpool.ByteBuffer) {
	bytebufferpool.Put(buf)
}
