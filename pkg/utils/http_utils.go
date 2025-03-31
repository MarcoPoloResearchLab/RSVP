// FILE: pkg/utils/http_utils.go
// Package utils provides miscellaneous utility functions used across the application,
// including HTTP parameter handling, URL building, error handling, and validation logic.
package utils

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// ParamSource defines where to look for HTTP request parameters.
type ParamSource int

// Constants defining the possible sources for parameter extraction.
const (
	QueryParam ParamSource = iota // Look only in URL query parameters.
	FormParam                     // Look only in parsed form data (requires POST/PUT/PATCH and parsing).
	BothParams                    // Look in URL query first, then in parsed form data.
)

// GetParam extracts a single named parameter from an HTTP request based on the specified source preference.
func GetParam(httpRequest *http.Request, paramName string, source ParamSource) string {
	var retrievedValue string

	if source == QueryParam || source == BothParams {
		retrievedValue = httpRequest.URL.Query().Get(paramName)
		if retrievedValue != "" && source == BothParams {
			return retrievedValue
		}
		if source == QueryParam {
			return retrievedValue
		}
	}

	if source == FormParam || source == BothParams {
		if httpRequest.Form == nil {
			httpRequest.Body = http.MaxBytesReader(nil, httpRequest.Body, 10<<20)
			parseFormError := httpRequest.ParseForm()
			if parseFormError != nil && !errors.Is(parseFormError, http.ErrNotMultipart) {
				return ""
			}
		}
		retrievedValue = httpRequest.FormValue(paramName)
	}

	return retrievedValue
}

// GetParams extracts multiple named parameters from an HTTP request using GetParam for each name.
func GetParams(httpRequest *http.Request, paramNames []string, source ParamSource) map[string]string {
	parameters := make(map[string]string)
	for _, singleParamName := range paramNames {
		parameters[singleParamName] = GetParam(httpRequest, singleParamName, source)
	}
	return parameters
}

// BuildRelativeURL constructs a URL string from a relative base path and adds query parameters.
func BuildRelativeURL(basePath string, queryParams map[string]string) string {
	targetURL := url.URL{Path: basePath}
	query := url.Values{}
	for key, value := range queryParams {
		if value != "" {
			query.Set(key, value)
		}
	}
	targetURL.RawQuery = query.Encode()
	return targetURL.String()
}

// BuildPublicURL constructs an absolute URL by resolving a relative path segment against an absolute base URL string,
// and then adding query parameters.
func BuildPublicURL(baseURLString string, pathSegment string, queryParams map[string]string) (string, error) {
	if baseURLString == "" {
		return "", errors.New("base URL string cannot be empty for BuildPublicURL")
	}
	if !strings.HasSuffix(baseURLString, "/") {
		baseURLString += "/"
	}

	baseURL, err := url.Parse(baseURLString)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL '%s': %w", baseURLString, err)
	}

	pathURL, err := url.Parse(pathSegment)
	if err != nil {
		return "", fmt.Errorf("failed to parse path segment '%s': %w", pathSegment, err)
	}

	resolvedURL := baseURL.ResolveReference(pathURL)

	query := resolvedURL.Query()
	for key, value := range queryParams {
		if value != "" {
			query.Set(key, value)
		}
	}
	resolvedURL.RawQuery = query.Encode()

	return resolvedURL.String(), nil
}

// ErrorType enumerates common categories of errors encountered in handlers.
type ErrorType int

// Constants defining specific error types for use with HandleError.
const (
	DatabaseError ErrorType = iota
	ValidationError
	AuthenticationError
	NotFoundError
	ServerError
	ForbiddenError
	MethodNotAllowedError
)

// User-facing error messages (constants).
const (
	ErrMsgInternalServer         = "An internal server error occurred. Please try again later."
	ErrMsgUnauthorized           = "Unauthorized: Please log in."
	ErrMsgInvalidFormData        = "Invalid form data submitted."
	ErrMsgInvalidStartTimeFormat = "Invalid start time format. Please use YYYY-MM-DDTHH:MM."
)

// HandleError provides a consistent way to log errors and send appropriate HTTP error responses.
func HandleError(
	httpResponseWriter http.ResponseWriter,
	err error,
	errorType ErrorType,
	logger *log.Logger,
	userMessage string,
) {
	if logger != nil {
		if err != nil {
			logger.Printf("ERROR Type(%d): %s | Details: %v", errorType, userMessage, err)
		} else {
			logger.Printf("ERROR Type(%d): %s", errorType, userMessage)
		}
	}

	var statusCode int
	switch errorType {
	case ValidationError:
		statusCode = http.StatusBadRequest
	case AuthenticationError:
		statusCode = http.StatusUnauthorized
	case ForbiddenError:
		statusCode = http.StatusForbidden
	case NotFoundError:
		statusCode = http.StatusNotFound
	case MethodNotAllowedError:
		statusCode = http.StatusMethodNotAllowed
	case DatabaseError, ServerError:
		statusCode = http.StatusInternalServerError
		if userMessage == "" {
			userMessage = ErrMsgInternalServer
		}
	default:
		if logger != nil {
			logger.Printf("WARN: Unknown error type (%d) used in HandleError. User Message: %s", errorType, userMessage)
		}
		statusCode = http.StatusInternalServerError
		if userMessage == "" {
			userMessage = ErrMsgInternalServer
		}
	}

	http.Error(httpResponseWriter, userMessage, statusCode)
}
