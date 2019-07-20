package sqlf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsertAt(t *testing.T) {
	a := insertAt([]interface{}{1, 2, 3, 4}, []interface{}{5, 6}, 4)
	assert.Equal(t, a, []interface{}{1, 2, 3, 4, 5, 6})

	a = insertAt([]interface{}{}, []interface{}{3, 2}, 0)
	assert.Equal(t, a, []interface{}{3, 2})

	a = insertAt([]interface{}{}, []interface{}{}, 5)
	assert.Equal(t, a, []interface{}{})

	a = insertAt([]interface{}{1, 2}, []interface{}{3}, 1)
	assert.Equal(t, a, []interface{}{1, 3, 2})
}
