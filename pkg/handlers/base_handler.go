// Package handlers provides base functionality and shared utilities for HTTP request handlers.
package handlers

import (
	"net/http"

	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/templates"
	"github.com/temirov/RSVP/pkg/utils"
)

// PageData is the top-level wrapper struct passed to the layout template execution.
// It contains common data needed by the layout (like user info) and view-specific data.
type PageData struct {
	IsPublicPage bool
	UserName     string
	UserPicture  string
	CSRFToken    string
	URLForLogout string
	URLForRoot   string
	Data         interface{}
}

// LoggedUserData holds essential user information retrieved from the session.
type LoggedUserData struct {
	UserPicture string
	UserName    string
	UserEmail   string
	UserID      string
}

// GetUserData retrieves user information (name, email, picture) from the current session.
// Returns an empty LoggedUserData struct if the session is invalid or data is missing.
func GetUserData(httpRequest *http.Request) *LoggedUserData {
	sessionInstance, sessionError := session.Store().Get(httpRequest, gconstants.SessionName)
	if sessionError != nil {
		return &LoggedUserData{}
	}
	sessionUserPicture, _ := sessionInstance.Values[gconstants.SessionKeyUserPicture].(string)
	sessionUserName, _ := sessionInstance.Values[gconstants.SessionKeyUserName].(string)
	sessionUserEmail, _ := sessionInstance.Values[gconstants.SessionKeyUserEmail].(string)

	return &LoggedUserData{
		UserPicture: sessionUserPicture,
		UserName:    sessionUserName,
		UserEmail:   sessionUserEmail,
	}
}

// BaseHttpHandler provides common context (database, logger) and helper methods
// for resource-specific HTTP handlers.
type BaseHttpHandler struct {
	ApplicationContext        *config.ApplicationContext
	ResourceNameForLogging    string
	ResourceBasePathForRoutes string
}

// NewBaseHttpHandler creates a new BaseHttpHandler instance.
func NewBaseHttpHandler(applicationContext *config.ApplicationContext, resourceName string, resourceBasePath string) BaseHttpHandler {
	return BaseHttpHandler{ApplicationContext: applicationContext, ResourceNameForLogging: resourceName, ResourceBasePathForRoutes: resourceBasePath}
}

// GetUserSessionData retrieves user data from the current request's session using GetUserData.
func (handler *BaseHttpHandler) GetUserSessionData(httpRequest *http.Request) *LoggedUserData {
	return GetUserData(httpRequest)
}

// ValidateHttpMethod checks if the request's HTTP method is among the allowed methods.
// If not allowed, it logs the issue and sends a 405 Method Not Allowed response.
// Returns true if the method is allowed, false otherwise.
func (handler *BaseHttpHandler) ValidateHttpMethod(responseWriter http.ResponseWriter, request *http.Request, allowedMethods ...string) bool {
	currentMethod := request.Method
	for _, allowedMethod := range allowedMethods {
		if currentMethod == allowedMethod {
			return true
		}
	}
	handler.ApplicationContext.Logger.Printf("Method Not Allowed: Received %s, Expected %v for %s", currentMethod, allowedMethods, request.URL.Path)
	http.Error(responseWriter, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	return false
}

// RequireAuthentication checks if a user is logged in by verifying session data.
// If the user is not authenticated, it logs the attempt and sends a 401 Unauthorized response.
// Returns the user data and true if authenticated, or user data and false otherwise.
func (handler *BaseHttpHandler) RequireAuthentication(responseWriter http.ResponseWriter, request *http.Request) (*LoggedUserData, bool) {
	sessionData := handler.GetUserSessionData(request)
	if sessionData.UserEmail == "" {
		handler.ApplicationContext.Logger.Printf("Unauthorized access attempt to %s", request.URL.Path)
		http.Error(responseWriter, "Unauthorized: Please log in.", http.StatusUnauthorized)
		return sessionData, false
	}
	return sessionData, true
}

// VerifyResourceOwnership checks if the currently authenticated user owns the specified resource.
// It uses the provided findOwnerIDFunction to retrieve the resource owner's ID based on the resourceIdentifier.
// It handles errors related to missing IDs, resource not found, or ownership mismatch by sending appropriate HTTP error responses (400, 500, 404, 403).
// Returns true if ownership is verified, false otherwise.
func (handler *BaseHttpHandler) VerifyResourceOwnership(responseWriter http.ResponseWriter, resourceIdentifier string, findOwnerIDFunction func(string) (string, error), currentUserID string) bool {
	if resourceIdentifier == "" {
		handler.ApplicationContext.Logger.Printf("Ownership check skipped: No resource identifier provided for %s.", handler.ResourceNameForLogging)
		handler.HandleError(responseWriter, nil, utils.ValidationError, "Resource identifier missing for ownership check.")
		return false
	}
	if currentUserID == "" {
		handler.ApplicationContext.Logger.Printf("Ownership check failed: Current user ID is missing.")
		http.Error(responseWriter, "Internal Server Error: Cannot verify ownership.", http.StatusInternalServerError)
		return false
	}

	ownerID, findError := findOwnerIDFunction(resourceIdentifier)
	if findError != nil {
		handler.HandleError(responseWriter, findError, utils.NotFoundError, "Resource not found or ownership check failed for "+handler.ResourceNameForLogging)
		return false
	}

	if ownerID != currentUserID {
		handler.ApplicationContext.Logger.Printf("Forbidden: User %s attempted action on %s %s owned by %s", currentUserID, handler.ResourceNameForLogging, resourceIdentifier, ownerID)
		http.Error(responseWriter, "Forbidden: You do not have permission to access this "+handler.ResourceNameForLogging+".", http.StatusForbidden)
		return false
	}
	return true
}

// GetParam retrieves a single named parameter from the request, checking both URL query and form values.
func (handler *BaseHttpHandler) GetParam(request *http.Request, parameterName string) string {
	return utils.GetParam(request, parameterName, utils.BothParams)
}

// GetParams retrieves multiple named parameters from the request, checking both URL query and form values.
func (handler *BaseHttpHandler) GetParams(request *http.Request, parameterNames ...string) map[string]string {
	return utils.GetParams(request, parameterNames, utils.BothParams)
}

// RequireParams retrieves multiple required parameters and returns false, sending a 400 Bad Request response if any are missing or empty.
// Returns the map of retrieved parameters and true if all required parameters are present.
func (handler *BaseHttpHandler) RequireParams(responseWriter http.ResponseWriter, request *http.Request, parameterNames ...string) (map[string]string, bool) {
	parameters := handler.GetParams(request, parameterNames...)
	missingParameters := []string{}
	for _, parameterName := range parameterNames {
		if value, exists := parameters[parameterName]; !exists || value == "" {
			missingParameters = append(missingParameters, parameterName)
		}
	}
	if len(missingParameters) > 0 {
		errorMessage := "Missing required parameter(s): " + utils.JoinStrings(missingParameters, ", ")
		handler.ApplicationContext.Logger.Printf("Bad Request: %s for %s", errorMessage, request.URL.Path)
		http.Error(responseWriter, errorMessage, http.StatusBadRequest)
		return parameters, false
	}
	return parameters, true
}

// RedirectToList redirects the user to the base path defined for the handler's resource (e.g., /events/).
func (handler *BaseHttpHandler) RedirectToList(responseWriter http.ResponseWriter, request *http.Request) {
	http.Redirect(responseWriter, request, handler.ResourceBasePathForRoutes, http.StatusSeeOther)
}

// RedirectWithParams redirects the user to the handler's resource base path, adding the specified query parameters.
func (handler *BaseHttpHandler) RedirectWithParams(responseWriter http.ResponseWriter, request *http.Request, parameters map[string]string) {
	utils.RedirectWithParams(responseWriter, request, handler.ResourceBasePathForRoutes, parameters, http.StatusSeeOther)
}

// HandleError provides consistent error logging and HTTP response generation based on the error type.
func (handler *BaseHttpHandler) HandleError(responseWriter http.ResponseWriter, err error, errorType utils.ErrorType, userMessage string) {
	utils.HandleError(responseWriter, err, errorType, handler.ApplicationContext.Logger, userMessage)
}

// RenderView renders the specified view template using the main application layout.
// It determines if the page is public, populates PageData with necessary common data
// (like user info for non-public pages), retrieves the precompiled template set,
// and executes it with the "layout" template as the entry point.
func (handler *BaseHttpHandler) RenderView(
	httpResponseWriter http.ResponseWriter,
	httpRequest *http.Request,
	viewName string,
	viewSpecificData interface{},
) {
	publicViews := map[string]bool{
		config.TemplateResponse: true,
		config.TemplateThankYou: true,
	}
	isPublicPage := publicViews[viewName]

	pageData := PageData{
		IsPublicPage: isPublicPage,
		Data:         viewSpecificData,
		URLForLogout: gconstants.LogoutPath,
		URLForRoot:   config.WebRoot,
	}

	if !isPublicPage {
		loggedUserData := handler.GetUserSessionData(httpRequest)
		pageData.UserName = loggedUserData.UserName
		pageData.UserPicture = loggedUserData.UserPicture
		if pageData.UserName == "" && pageData.UserPicture == "" {
			handler.ApplicationContext.Logger.Printf("WARN: Rendering non-public view '%s' but user session data (Name/Picture) seems incomplete.", viewName)
		}
	}

	templateSet, exists := templates.PrecompiledTemplatesMap[viewName]
	if !exists {
		handler.ApplicationContext.Logger.Printf("CRITICAL: Template set for view '%s' not found in PrecompiledTemplatesMap.", viewName)
		http.Error(httpResponseWriter, "Internal Server Error: Application configuration issue.", http.StatusInternalServerError)
		return
	}

	executionError := templateSet.ExecuteTemplate(httpResponseWriter, "layout", pageData)
	if executionError != nil {
		handler.ApplicationContext.Logger.Printf("ERROR: Failed to execute layout template for view '%s' (Resource: %s): %v", viewName, handler.ResourceNameForLogging, executionError)
		http.Error(httpResponseWriter, "Internal Server Error: Could not render the page.", http.StatusInternalServerError)
	}
}
