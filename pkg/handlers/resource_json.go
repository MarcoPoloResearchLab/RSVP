package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
)

var (
	// ErrUnsupportedJSONMediaType indicates that a request does not contain canonical JSON.
	ErrUnsupportedJSONMediaType = errors.New("content type must be application/json")
	// ErrMalformedJSON indicates that a request body does not contain one canonical JSON object.
	ErrMalformedJSON  = errors.New("request body is malformed")
	resourceIDPattern = regexp.MustCompile(`^[0-9A-Za-z]{1,8}$`)
)

// DecodeJSON decodes one bounded JSON object and rejects unknown fields or trailing content.
func DecodeJSON(responseWriter http.ResponseWriter, request *http.Request, maximumBytes int64, destination any) error {
	mediaType, _, contentTypeError := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if contentTypeError != nil || mediaType != JSONMediaType {
		return ErrUnsupportedJSONMediaType
	}
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maximumBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if decodeError := decoder.Decode(destination); decodeError != nil {
		return fmt.Errorf("%w: %v", ErrMalformedJSON, decodeError)
	}
	if trailingError := decoder.Decode(&struct{}{}); !errors.Is(trailingError, io.EOF) {
		if trailingError == nil {
			return fmt.Errorf("%w: trailing JSON value", ErrMalformedJSON)
		}
		return fmt.Errorf("%w: %v", ErrMalformedJSON, trailingError)
	}
	return nil
}

// ResourceIDFromPath returns one valid opaque identifier after a resource base path.
func ResourceIDFromPath(basePath string, requestPath string) (string, bool) {
	if !strings.HasPrefix(requestPath, basePath) {
		return "", false
	}
	identifier := strings.TrimPrefix(requestPath, basePath)
	return identifier, resourceIDPattern.MatchString(identifier)
}

// WriteJSON writes one JSON representation.
func WriteJSON(responseWriter http.ResponseWriter, statusCode int, value any) error {
	responseWriter.Header().Set("Content-Type", JSONMediaType)
	responseWriter.WriteHeader(statusCode)
	return json.NewEncoder(responseWriter).Encode(value)
}
