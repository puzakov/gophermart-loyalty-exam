package auth

import (
	"strconv"
)

func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}

func atoi64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
