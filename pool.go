package sqlf

import (
	"sync"
)

var (
	stmtPool  = sync.Pool{New: newStmt}
	pgCtxPool = sync.Pool{New: newPgCtx}
)

func newStmt() interface{} {
	return &Stmt{
		chunks: make(stmtChunks, 0, 10),
	}
}

func getStmt(b *Builder) *Stmt {
	stmt := stmtPool.Get().(*Stmt)
	stmt.builder = b
	return stmt
}

func reuseStmt(q *Stmt) {
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
