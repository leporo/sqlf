package sqlf

import (
	"strconv"

	"github.com/valyala/bytebufferpool"
)

var (
	dialectDefault    = new(noDialect)
	dialectPostgreSQL = new(postgreSQL)
)

/*
NoDialect returns a default SQL processor.

	sqlf.SetDialect(sqlf.NoDialect())
	qb := sqlf.NewBuilder(sqlf.NoDialect())
*/
func NoDialect() Dialect {
	return dialectDefault
}

/*
PostgreSQL returns SQL processor to be used to automatically convert ? placeholders
to positional $1, $2... parameters.

	sqlf.SetDialect(sqlf.PostgreSQL())
*/
func PostgreSQL() Dialect {
	return dialectPostgreSQL
}

// Dialect is an interface used by Builder.
type Dialect interface {
	NewCtx() DialectCtx
	// WriteString writes a string to SQL statement build buffer.
	WriteString(ctx DialectCtx, s []byte, buf *bytebufferpool.ByteBuffer, argLen int) error
}

// DialectCtx is a generic interface for statement context
// used by a Dialect interface implementation.
type DialectCtx interface {
	// Close releases the context
	Close()
}

// noDialect is a default SQL dialect handler.
// It doesn't alter SQL passed to statement builder methods.
type noDialect int

// NewCtx returns nothing.
func (d *noDialect) NewCtx() DialectCtx {
	return nil
}

// WriteString method copies source string to builder buffer.
func (d *noDialect) WriteString(ctx DialectCtx, s []byte, buf *bytebufferpool.ByteBuffer, argLen int) error {
	_, err := buf.Write(s)
	return err
}

// postgreSQL supports numbered PostgreSQL query arguments.
// It converts ? placeholders to $1, $2...
type postgreSQL int

type postgresqlCtx struct {
	argNo int
}

func (ctx *postgresqlCtx) Close() {
	putPgCtx(ctx)
}

// NewCtx returns a context to be used by postgreSQL string processor
func (d *postgreSQL) NewCtx() DialectCtx {
	return getPgCtx()
}

// WriteString method replaces ? placeholders with $1, $2...
func (d *postgreSQL) WriteString(ctx DialectCtx, s []byte, buf *bytebufferpool.ByteBuffer, argLen int) error {
	var err error
	if argLen == 0 {
		_, err = buf.Write(s)
	} else {
		pctx := ctx.(*postgresqlCtx)

		start := 0
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
						buf.B = strconv.AppendInt(buf.B, int64(pctx.argNo), 10)
						pctx.argNo++
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
	}
	return err
}
