package utils

import (
	"math/rand"
	"net/http"
)

func HTTPHandlerWrapper(handler http.HandlerFunc) http.HandlerFunc {
	return handler
}

// Base36Encode6 returns a random 6-digit base36 string.
// This does NOT rely on incremental IDs; it just generates 6 random base36 chars.
func Base36Encode6() string {
	const length = 6
	return Base36Encode(length)
}

func Base36Encode(length int) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"

	out := make([]byte, length)
	for i := 0; i < length; i++ {
		out[i] = chars[rand.Intn(len(chars))]
	}
	return string(out)
}
