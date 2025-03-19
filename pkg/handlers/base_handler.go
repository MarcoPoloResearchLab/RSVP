package handlers

import (
	"log"
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/utils"
)

type BaseHandler struct {
	ApplicationContext *config.ApplicationContext
	ResourceName       string
	ResourcePath       string
}

func NewBaseHandler(
	appContext *config.ApplicationContext,
	resourceName string,
	resourcePath string,
) BaseHandler {
	return BaseHandler{
		ApplicationContext: appContext,
		ResourceName:       resourceName,
		ResourcePath:       resourcePath,
	}
}

// LoggedUserData is defined in common.go, imported here from that file.
func (handler *BaseHandler) GetUserData(httpRequest *http.Request) *LoggedUserData {
	return GetUserData(httpRequest, handler.ApplicationContext)
}

// ValidateMethod checks if the request method is one of the allowed methods.
func (handler *BaseHandler) ValidateMethod(
	httpResponseWriter http.ResponseWriter,
	httpRequest *http.Request,
	allowedMethods ...string,
) bool {
	for _, allowedMethod := range allowedMethods {
		if httpRequest.Method == allowedMethod {
			return true
		}
	}
	http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
	return false
}

// RequireAuthentication ensures the current user is authenticated.
func (handler *BaseHandler) RequireAuthentication(
	httpResponseWriter http.ResponseWriter,
	httpRequest *http.Request,
) (*LoggedUserData, bool) {
	sessionData := handler.GetUserData(httpRequest)
	isAuthenticated := sessionData.UserEmail != ""
	if !isAuthenticated {
		http.Error(httpResponseWriter, "Unauthorized", http.StatusUnauthorized)
	}
	return sessionData, isAuthenticated
}

// VerifyResourceOwnership checks if the current user is the owner of a resource.
func (handler *BaseHandler) VerifyResourceOwnership(
	httpResponseWriter http.ResponseWriter,
	resourceIdentifier string,
	findOwnerIDFunc func(string) (string, error),
	currentUserID string,
) bool {
	if resourceIdentifier == "" {
		return true
	}

	ownerID, findOwnerError := findOwnerIDFunc(resourceIdentifier)
	if findOwnerError != nil {
		handler.HandleError(httpResponseWriter, findOwnerError, utils.NotFoundError, "Resource not found")
		return false
	}

	if ownerID != currentUserID {
		http.Error(httpResponseWriter, "Forbidden", http.StatusForbidden)
		return false
	}

	return true
}

// GetParam extracts a parameter from the request (query or form).
func (handler *BaseHandler) GetParam(
	httpRequest *http.Request,
	parameterName string,
) string {
	return utils.GetParam(httpRequest, parameterName, utils.BothParams)
}

// GetParams extracts multiple parameters from the request (query or form).
func (handler *BaseHandler) GetParams(
	httpRequest *http.Request,
	paramNames ...string,
) map[string]string {
	return utils.GetParams(httpRequest, paramNames, utils.BothParams)
}

// RequireParams ensures all required parameters are present (non-empty).
func (handler *BaseHandler) RequireParams(
	httpResponseWriter http.ResponseWriter,
	httpRequest *http.Request,
	paramNames ...string,
) (map[string]string, bool) {
	parameterMap := handler.GetParams(httpRequest, paramNames...)
	for _, paramName := range paramNames {
		if parameterMap[paramName] == "" {
			http.Error(httpResponseWriter, paramName+" is required", http.StatusBadRequest)
			return parameterMap, false
		}
	}
	return parameterMap, true
}

// RedirectToList redirects to the list view for the resource path.
func (handler *BaseHandler) RedirectToList(
	httpResponseWriter http.ResponseWriter,
	httpRequest *http.Request,
) {
	http.Redirect(httpResponseWriter, httpRequest, handler.ResourcePath, http.StatusSeeOther)
}

// RedirectWithParams redirects to the resource path with added query parameters.
func (handler *BaseHandler) RedirectWithParams(
	httpResponseWriter http.ResponseWriter,
	httpRequest *http.Request,
	parameters map[string]string,
) {
	utils.RedirectWithParams(httpResponseWriter, httpRequest, handler.ResourcePath, parameters, http.StatusSeeOther)
}

// HandleError handles an error with appropriate logging and HTTP response.
func (handler *BaseHandler) HandleError(
	httpResponseWriter http.ResponseWriter,
	errorValue error,
	errorType utils.ErrorType,
	message string,
) {
	utils.HandleError(httpResponseWriter, errorValue, errorType, handler.ApplicationContext.Logger, message)
}

// RenderTemplate renders a specified template name with the provided data.
func (handler *BaseHandler) RenderTemplate(
	httpResponseWriter http.ResponseWriter,
	templateName string,
	data interface{},
) {
	templateError := handler.ApplicationContext.Templates.ExecuteTemplate(httpResponseWriter, templateName, data)
	if templateError != nil {
		handler.ApplicationContext.Logger.Printf("Error rendering template %s: %v", templateName, templateError)
		http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Logger returns the application-wide logger.
func (handler *BaseHandler) Logger() *log.Logger {
	return handler.ApplicationContext.Logger
}
