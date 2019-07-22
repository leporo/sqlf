package sqlf

import (
	"sync"

	"github.com/valyala/bytebufferpool"
)

var (
	stmtPool  = sync.Pool{New: newStmt}
	pgCtxPool = sync.Pool{New: newPgCtx}
	bufPool   = sync.Pool{New: newBuf}
)

func newStmt() interface{} {
	return &Stmt{
		chunks: make(stmtChunks, 0, 8),
	}
}

func getStmt(b *Builder) *Stmt {
	stmt := stmtPool.Get().(*Stmt)
	stmt.builder = b
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
	if q.sql != nil {
		putBuffer(q.sql)
	}
	q.sql = nil

	stmtPool.Put(q)
}

func newPgCtx() interface{} {
	return &postgresqlCtx{argNo: 1}
}

func getPgCtx() *postgresqlCtx {
	ctx := pgCtxPool.Get().(*postgresqlCtx)
	ctx.argNo = 1
	return ctx
}

func putPgCtx(ctx *postgresqlCtx) {
	pgCtxPool.Put(ctx)
}

func newBuf() interface{} {
	return &bytebufferpool.ByteBuffer{
		B: make([]byte, 0, 256),
	}
}

func getBuffer() *bytebufferpool.ByteBuffer {
	// return bytebufferpool.Get()
	return bufPool.Get().(*bytebufferpool.ByteBuffer)
}

func putBuffer(buf *bytebufferpool.ByteBuffer) {
	// bytebufferpool.Put(buf)
	if len(buf.B) > 0 {
		buf.Reset()
	}
	bufPool.Put(buf)
}
