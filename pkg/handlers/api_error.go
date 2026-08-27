package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/tyemirov/RSVP/pkg/utils"
)

const (
	// JSONMediaType is the canonical media type for JSON API responses.
	JSONMediaType  = "application/json"
	requestIDBytes = 16
)

type apiErrorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Details   map[string]string `json:"details"`
	RequestID string            `json:"request_id"`
}

// WriteTypedError writes the canonical typed API error response.
func WriteTypedError(responseWriter http.ResponseWriter, statusCode int, errorCode string, errorMessage string) error {
	requestIDValue := make([]byte, requestIDBytes)
	if _, randomError := rand.Read(requestIDValue); randomError != nil {
		http.Error(responseWriter, utils.ErrMsgInternalServer, http.StatusInternalServerError)
		return randomError
	}
	responseBody := apiErrorBody{Error: apiError{
		Code: errorCode, Message: errorMessage, Details: map[string]string{}, RequestID: hex.EncodeToString(requestIDValue),
	}}
	encodedBody, encodeError := json.Marshal(responseBody)
	if encodeError != nil {
		http.Error(responseWriter, utils.ErrMsgInternalServer, http.StatusInternalServerError)
		return encodeError
	}
	responseWriter.Header().Set("Content-Type", JSONMediaType)
	responseWriter.WriteHeader(statusCode)
	encodedBody = append(encodedBody, '\n')
	_, writeError := responseWriter.Write(encodedBody)
	return writeError
}
