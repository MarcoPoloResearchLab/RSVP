package utils

import (
	"crypto/rand"
	"math/big"
	"net/http"
)

// HTTPHandlerWrapper is a pass-through wrapper for http.HandlerFunc.
func HTTPHandlerWrapper(handler http.HandlerFunc) http.HandlerFunc {
	return handler
}

// Base36Encode6 returns a random 6-character base36 string using crypto/rand.
func Base36Encode6() string {
	const codeLength = 6
	return Base36Encode(codeLength)
}

// Base36Encode returns a random string of the specified length from base36 characters.
func Base36Encode(length int) string {
	const allowedCharacters = "0123456789abcdefghijklmnopqrstuvwxyz"
	outputBytes := make([]byte, length)
	maxIndex := big.NewInt(int64(len(allowedCharacters)))
	for index := 0; index < length; index++ {
		randomNumber, randomError := rand.Int(rand.Reader, maxIndex)
		if randomError != nil {
			// Fallback to a default character on error
			outputBytes[index] = allowedCharacters[0]
		} else {
			outputBytes[index] = allowedCharacters[randomNumber.Int64()]
		}
	}
	return string(outputBytes)
}
