package utils

import (
	"log"
	"net/http"
	"net/url"
)

// ParamSource defines where to look for parameters
type ParamSource int

const (
	// QueryParam indicates to look in URL query parameters
	QueryParam ParamSource = iota
	// FormParam indicates to look in form values
	FormParam
	// BothParams indicates to look in both query and form values, with query taking precedence
	BothParams
)

// GetParam extracts a parameter from a request based on the source preference
func GetParam(r *http.Request, paramName string, source ParamSource) string {
	var value string

	// Check query parameters if specified
	if source == QueryParam || source == BothParams {
		value = r.URL.Query().Get(paramName)
		if value != "" {
			return value
		}
	}

	// Check form values if specified
	if source == FormParam || source == BothParams {
		// Parse form if not already parsed
		if r.Form == nil {
			r.ParseForm()
		}
		value = r.FormValue(paramName)
	}

	return value
}

// GetParams extracts multiple parameters from a request
func GetParams(r *http.Request, paramNames []string, source ParamSource) map[string]string {
	params := make(map[string]string)
	for _, name := range paramNames {
		params[name] = GetParam(r, name, source)
	}
	return params
}

// BuildURL creates a URL with query parameters
func BuildURL(basePath string, params map[string]string) string {
	baseURL, _ := url.Parse(basePath)
	queryParams := baseURL.Query()
	
	for key, value := range params {
		if value != "" {
			queryParams.Set(key, value)
		}
	}
	
	baseURL.RawQuery = queryParams.Encode()
	return baseURL.String()
}

// ErrorType defines the type of error for proper handling
type ErrorType int

const (
	// DatabaseError represents database-related errors
	DatabaseError ErrorType = iota
	// ValidationError represents input validation errors
	ValidationError
	// AuthenticationError represents authentication-related errors
	AuthenticationError
	// NotFoundError represents resource not found errors
	NotFoundError
	// ServerError represents internal server errors
	ServerError
)

// HandleError handles common HTTP errors with appropriate status codes and logging
func HandleError(w http.ResponseWriter, err error, errorType ErrorType, logger *log.Logger, message string) {
	// Log the error
	if logger != nil {
		logger.Printf("%s: %v", message, err)
	}

	// Set appropriate status code and message based on error type
	switch errorType {
	case ValidationError:
		http.Error(w, message, http.StatusBadRequest)
	case AuthenticationError:
		http.Error(w, message, http.StatusUnauthorized)
	case NotFoundError:
		http.Error(w, message, http.StatusNotFound)
	case DatabaseError:
		http.Error(w, message, http.StatusInternalServerError)
	case ServerError:
		http.Error(w, message, http.StatusInternalServerError)
	default:
		http.Error(w, message, http.StatusInternalServerError)
	}
}

// RedirectWithParams redirects to a URL with the given parameters
func RedirectWithParams(w http.ResponseWriter, r *http.Request, basePath string, params map[string]string, statusCode int) {
	redirectURL := BuildURL(basePath, params)
	http.Redirect(w, r, redirectURL, statusCode)
}
