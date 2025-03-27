// File: pkg/handlers/base_handler.go
package handlers

import (
	"net/http"

	"github.com/temirov/GAuss/pkg/constants" // Use GAuss constants
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/templates"
	"github.com/temirov/RSVP/pkg/utils"
)

// PageData is the top-level wrapper struct passed to the layout template execution.
type PageData struct {
	IsPublicPage bool
	UserName     string
	UserPicture  string
	CSRFToken    string
	URLForLogout string      // URL for the logout action
	URLForRoot   string      // URL for the application root
	Data         interface{} // View-specific data payload
}

// LoggedUserData holds session information retrieved from the session store.
type LoggedUserData struct {
	UserPicture string
	UserName    string
	UserEmail   string
	UserID      string
}

// GetUserData retrieves user information from the session.
func GetUserData(httpRequest *http.Request) *LoggedUserData {
	sessionInstance, sessionError := session.Store().Get(httpRequest, constants.SessionName)
	if sessionError != nil {
		return &LoggedUserData{}
	}
	sessionUserPicture, _ := sessionInstance.Values[constants.SessionKeyUserPicture].(string)
	sessionUserName, _ := sessionInstance.Values[constants.SessionKeyUserName].(string)
	sessionUserEmail, _ := sessionInstance.Values[constants.SessionKeyUserEmail].(string)

	return &LoggedUserData{
		UserPicture: sessionUserPicture,
		UserName:    sessionUserName,
		UserEmail:   sessionUserEmail,
	}
}

// BaseHttpHandler provides common context and helper methods for resource handlers.
type BaseHttpHandler struct {
	ApplicationContext        *config.ApplicationContext
	ResourceNameForLogging    string
	ResourceBasePathForRoutes string
}

// NewBaseHttpHandler is the constructor for BaseHttpHandler.
func NewBaseHttpHandler(applicationContext *config.ApplicationContext, resourceName string, resourceBasePath string) BaseHttpHandler {
	return BaseHttpHandler{ApplicationContext: applicationContext, ResourceNameForLogging: resourceName, ResourceBasePathForRoutes: resourceBasePath}
}

// GetUserSessionData retrieves user data from the current request's session.
func (handler *BaseHttpHandler) GetUserSessionData(httpRequest *http.Request) *LoggedUserData {
	return GetUserData(httpRequest)
}

// ValidateHttpMethod checks if the request method is one of the allowed methods.
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

// RequireAuthentication checks if a user is logged in based on session data.
func (handler *BaseHttpHandler) RequireAuthentication(responseWriter http.ResponseWriter, request *http.Request) (*LoggedUserData, bool) {
	sessionData := handler.GetUserSessionData(request)
	if sessionData.UserEmail == "" {
		handler.ApplicationContext.Logger.Printf("Unauthorized access attempt to %s", request.URL.Path)
		http.Error(responseWriter, "Unauthorized: Please log in.", http.StatusUnauthorized)
		return sessionData, false
	}
	return sessionData, true
}

// VerifyResourceOwnership checks if the current user owns the specified resource.
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

// GetParam retrieves a single parameter, checking query and form values.
func (handler *BaseHttpHandler) GetParam(request *http.Request, parameterName string) string {
	return utils.GetParam(request, parameterName, utils.BothParams)
}

// GetParams retrieves multiple parameters, checking query and form values.
func (handler *BaseHttpHandler) GetParams(request *http.Request, parameterNames ...string) map[string]string {
	return utils.GetParams(request, parameterNames, utils.BothParams)
}

// RequireParams retrieves required parameters and returns false if any are missing.
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

// RedirectToList redirects the user back to the base path for the resource.
func (handler *BaseHttpHandler) RedirectToList(responseWriter http.ResponseWriter, request *http.Request) {
	http.Redirect(responseWriter, request, handler.ResourceBasePathForRoutes, http.StatusSeeOther)
}

// RedirectWithParams redirects the user back to the base path for the resource, adding specified query parameters.
func (handler *BaseHttpHandler) RedirectWithParams(responseWriter http.ResponseWriter, request *http.Request, parameters map[string]string) {
	utils.RedirectWithParams(responseWriter, request, handler.ResourceBasePathForRoutes, parameters, http.StatusSeeOther)
}

// HandleError provides consistent error logging and HTTP response generation.
func (handler *BaseHttpHandler) HandleError(responseWriter http.ResponseWriter, err error, errorType utils.ErrorType, userMessage string) {
	utils.HandleError(responseWriter, err, errorType, handler.ApplicationContext.Logger, userMessage)
}

// RenderView renders the specified view template using the main layout.
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

	// Initialize PageData and populate common fields needed by the layout/header
	pageData := PageData{
		IsPublicPage: isPublicPage,
		Data:         viewSpecificData,
		URLForLogout: constants.LogoutPath, // Populate with GAuss logout path
		URLForRoot:   config.WebRoot,       // Populate with application root path
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
