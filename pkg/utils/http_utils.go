package utils

import (
	"log"
	"net/http"
	"net/url"
)

type ParamSource int

const (
	QueryParam ParamSource = iota
	FormParam
	BothParams
)

// GetParam extracts a parameter from a request based on the source preference.
func GetParam(httpRequest *http.Request, paramName string, source ParamSource) string {
	var retrievedValue string

	if source == QueryParam || source == BothParams {
		retrievedValue = httpRequest.URL.Query().Get(paramName)
		if retrievedValue != "" {
			return retrievedValue
		}
	}

	if source == FormParam || source == BothParams {
		if httpRequest.Form == nil {
			parseFormErr := httpRequest.ParseForm()
			if parseFormErr != nil {
				return ""
			}
		}
		retrievedValue = httpRequest.FormValue(paramName)
	}

	return retrievedValue
}

// GetParams extracts multiple parameters from a request.
func GetParams(httpRequest *http.Request, paramNames []string, source ParamSource) map[string]string {
	parameters := make(map[string]string)
	for _, singleParamName := range paramNames {
		parameters[singleParamName] = GetParam(httpRequest, singleParamName, source)
	}
	return parameters
}

// BuildURL creates a URL with query parameters.
func BuildURL(basePath string, params map[string]string) string {
	parsedBaseURL, _ := url.Parse(basePath)
	queryParams := parsedBaseURL.Query()
	for keyName, keyValue := range params {
		if keyValue != "" {
			queryParams.Set(keyName, keyValue)
		}
	}
	parsedBaseURL.RawQuery = queryParams.Encode()
	return parsedBaseURL.String()
}

type ErrorType int

const (
	DatabaseError ErrorType = iota
	ValidationError
	AuthenticationError
	NotFoundError
	ServerError
)

// HandleError handles common HTTP errors with appropriate status codes and logging.
func HandleError(
	httpResponseWriter http.ResponseWriter,
	err error,
	errorType ErrorType,
	logger *log.Logger,
	message string,
) {
	if logger != nil {
		logger.Printf("%s: %v", message, err)
	}

	switch errorType {
	case ValidationError:
		http.Error(httpResponseWriter, message, http.StatusBadRequest)
	case AuthenticationError:
		http.Error(httpResponseWriter, message, http.StatusUnauthorized)
	case NotFoundError:
		http.Error(httpResponseWriter, message, http.StatusNotFound)
	case DatabaseError, ServerError:
		http.Error(httpResponseWriter, message, http.StatusInternalServerError)
	default:
		http.Error(httpResponseWriter, message, http.StatusInternalServerError)
	}
}

// RedirectWithParams redirects to a URL with the given parameters.
func RedirectWithParams(
	httpResponseWriter http.ResponseWriter,
	httpRequest *http.Request,
	basePath string,
	params map[string]string,
	statusCode int,
) {
	redirectURL := BuildURL(basePath, params)
	http.Redirect(httpResponseWriter, httpRequest, redirectURL, statusCode)
}

// ApplyMethodOverride checks for method override in form data.
func ApplyMethodOverride(httpRequest *http.Request, methodParamName string) {
	if httpRequest.Method != http.MethodPost {
		return
	}
	if httpRequest.Form == nil {
		parseFormError := httpRequest.ParseForm()
		if parseFormError != nil {
			return
		}
	}
	methodOverride := httpRequest.FormValue(methodParamName)
	if methodOverride == "" {
		return
	}
	switch methodOverride {
	case http.MethodDelete:
		httpRequest.Method = http.MethodDelete
	case http.MethodPut:
		httpRequest.Method = http.MethodPut
	case http.MethodPatch:
		httpRequest.Method = http.MethodPatch
	}
}
