package sqlf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSQLCache(t *testing.T) {
	buf := getBuffer()
	defer putBuffer(buf)

	buf.WriteString("test")
	_, ok := defaultDialectPointer.Load().getCachedSQL(buf)
	require.False(t, ok)

	defaultDialectPointer.Load().putCachedSQL(buf, "test SQL")
	sql, ok := defaultDialectPointer.Load().getCachedSQL(buf)
	require.True(t, ok)
	require.Equal(t, "test SQL", sql)

	defaultDialectPointer.Load().ClearCache()
}
