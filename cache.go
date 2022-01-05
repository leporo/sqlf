package sqlf

import (
	"github.com/valyala/bytebufferpool"
)

type sqlCache map[string]string

/*
ClearCache clears the statement cache.

In most cases you don't need to care about it. It's there to
let caller free memory when a caller executes zillions of unique
SQL statements.
*/
func (d *Dialect) ClearCache() {
	d.cacheLock.Lock()
	d.cache = make(sqlCache)
	d.cacheLock.Unlock()
}

func (d *Dialect) getCache() sqlCache {
	d.cacheOnce.Do(func() {
		d.cache = make(sqlCache)
	})
	return d.cache
}

func (d *Dialect) getCachedSQL(buf *bytebufferpool.ByteBuffer) (string, bool) {
	c := d.getCache()
	s := bufToString(&buf.B)

	d.cacheLock.RLock()
	res, ok := c[s]
	d.cacheLock.RUnlock()
	return res, ok
}

func (d *Dialect) putCachedSQL(buf *bytebufferpool.ByteBuffer, sql string) {
	key := string(buf.B)
	c := d.getCache()
	d.cacheLock.Lock()
	c[key] = sql
	d.cacheLock.Unlock()
}
