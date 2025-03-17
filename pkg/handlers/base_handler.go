package handlers

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/utils"
)

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
	ApplicationContext *config.ApplicationContext
	ResourceName       string
	ResourcePath       string
}

// NewBaseHandler creates a new BaseHandler with the given context and resource information
func NewBaseHandler(appContext *config.ApplicationContext, resourceName, resourcePath string) BaseHandler {
	return BaseHandler{
		ApplicationContext: appContext,
		ResourceName:       resourceName,
		ResourcePath:       resourcePath,
	}
}

// GetUserData retrieves user data from the session
func (handler *BaseHandler) GetUserData(request *http.Request) *LoggedUserData {
	return GetUserData(request, handler.ApplicationContext)
}

// ValidateMethod checks if the request method is one of the allowed methods
func (handler *BaseHandler) ValidateMethod(responseWriter http.ResponseWriter, request *http.Request, allowedMethods ...string) bool {
	for _, methodName := range allowedMethods {
		if request.Method == methodName {
			return true
		}
	}

	http.Error(responseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
	return false
}

// RequireAuthentication gets the user data and checks if the user is authenticated.
// This method assumes the request has already passed through the gauss.AuthMiddleware,
// which guarantees that only authenticated users reach this point.
// It returns the user data and a boolean indicating if the user is authenticated.
func (handler *BaseHandler) RequireAuthentication(responseWriter http.ResponseWriter, request *http.Request) (*LoggedUserData, bool) {
	sessionData := handler.GetUserData(request)
	isAuthenticated := sessionData.UserEmail != ""

	if !isAuthenticated {
		http.Error(responseWriter, "Unauthorized", http.StatusUnauthorized)
	}

	return sessionData, isAuthenticated
}

// VerifyResourceOwnership checks if the current user is the owner of a resource.
// It takes a resource ID, a function to find the resource owner ID, and the current user ID.
// It returns true if the user owns the resource, false otherwise.
// If the resource is not found or the user is not the owner, it returns an appropriate HTTP error.
func (handler *BaseHandler) VerifyResourceOwnership(responseWriter http.ResponseWriter, resourceID string,
	findOwnerIDFunc func(string) (string, error), currentUserID string) bool {

	// If no resource ID is provided, return true (creating a new resource)
	if resourceID == "" {
		return true
	}

	// Find the resource owner ID
	ownerID, findError := findOwnerIDFunc(resourceID)
	if findError != nil {
		handler.HandleError(responseWriter, findError, utils.NotFoundError, "Resource not found")
		return false
	}

	// Check if the current user is the owner
	if ownerID != currentUserID {
		http.Error(responseWriter, "Forbidden", http.StatusForbidden)
		return false
	}

	return true
}

// GetParam extracts a parameter from the request
func (handler *BaseHandler) GetParam(request *http.Request, paramName string) string {
	return utils.GetParam(request, paramName, utils.BothParams)
}

// GetParams extracts multiple parameters from the request
func (handler *BaseHandler) GetParams(request *http.Request, paramNames ...string) map[string]string {
	return utils.GetParams(request, paramNames, utils.BothParams)
}

// RequireParams ensures all required parameters are present
func (handler *BaseHandler) RequireParams(responseWriter http.ResponseWriter, request *http.Request, paramNames ...string) (map[string]string, bool) {
	params := handler.GetParams(request, paramNames...)

	for _, paramName := range paramNames {
		if params[paramName] == "" {
			http.Error(responseWriter, paramName+" is required", http.StatusBadRequest)
			return params, false
		}
	}

	return params, true
}

// RedirectToList redirects to the list view for the resource
func (handler *BaseHandler) RedirectToList(responseWriter http.ResponseWriter, request *http.Request) {
	http.Redirect(responseWriter, request, handler.ResourcePath, http.StatusSeeOther)
}

// RedirectToResource redirects to a specific resource
func (handler *BaseHandler) RedirectToResource(responseWriter http.ResponseWriter, request *http.Request, resourceID string) {
	redirectURL, _ := url.Parse(handler.ResourcePath)
	queryParams := redirectURL.Query()

	// Use the appropriate ID parameter name based on resource type
	var idParamName string
	if handler.ResourceName == "Event" {
		idParamName = config.EventIDParam
	} else if handler.ResourceName == "RSVP" {
		idParamName = config.RSVPIDParam
	} else if handler.ResourceName == "User" {
		idParamName = config.UserIDParam
	} else {
		// Fallback to a specific name derived from resource
		// This should rarely happen since we define constants for core resources
		idParamName = strings.ToLower(handler.ResourceName) + "_id"
	}

	queryParams.Set(idParamName, resourceID)
	redirectURL.RawQuery = queryParams.Encode()
	http.Redirect(responseWriter, request, redirectURL.String(), http.StatusSeeOther)
}

// RedirectWithParams redirects to a URL with the given parameters
func (handler *BaseHandler) RedirectWithParams(responseWriter http.ResponseWriter, request *http.Request, params map[string]string) {
	utils.RedirectWithParams(responseWriter, request, handler.ResourcePath, params, http.StatusSeeOther)
}

// HandleError handles an error with appropriate logging and response
func (handler *BaseHandler) HandleError(responseWriter http.ResponseWriter, errorValue error, errorType utils.ErrorType, message string) {
	utils.HandleError(responseWriter, errorValue, errorType, handler.ApplicationContext.Logger, message)
}

// RenderTemplate renders a template with the given data
func (handler *BaseHandler) RenderTemplate(responseWriter http.ResponseWriter, templateName string, data interface{}) {
	if templateError := handler.ApplicationContext.Templates.ExecuteTemplate(responseWriter, templateName, data); templateError != nil {
		handler.ApplicationContext.Logger.Printf("Error rendering template %s: %v", templateName, templateError)
		http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Logger returns the application logger
func (handler *BaseHandler) Logger() *log.Logger {
	return handler.ApplicationContext.Logger
}
