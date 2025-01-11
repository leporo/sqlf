package sqlf

import (
	"github.com/savsgio/gotils/strconv"
)

func insertAt(dest, src []interface{}, index int) []interface{} {
	srcLen := len(src)
	if srcLen > 0 {
		oldLen := len(dest)
		dest = append(dest, src...)
		if index < oldLen {
			copy(dest[index+srcLen:], dest[index:])
			copy(dest[index:], src)
		}
	}

	return dest
}

// bufToString returns a string pointing to a ByteBuffer contents
// It helps to avoid memory copyng.
// Use the returned string with care, make sure to never use it after
// the ByteBuffer is deallocated or returned to a pool.
func bufToString(buf *[]byte) string {
	return strconv.B2S(*buf)
}
