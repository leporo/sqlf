package sqlf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSQLCache(t *testing.T) {
	buf := getBuffer()
	defer putBuffer(buf)

	buf.WriteString("test")
	_, ok := defaultDialect.getCachedSQL(buf)
	require.False(t, ok)

	defaultDialect.putCachedSQL(buf, "test SQL")
	sql, ok := defaultDialect.getCachedSQL(buf)
	require.True(t, ok)
	require.Equal(t, "test SQL", sql)

	defaultDialect.ClearCache()
}
