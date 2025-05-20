// Package handlers contains HTTP handler logic for different application resources (Events, RSVPs, Responses).
// It includes base handler functionality, specific resource handlers, and common utilities.
package handlers

import (
	"net/http"
	"strings"

	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/templates"
	"github.com/temirov/RSVP/pkg/utils"
)

// PageData is the top-level wrapper struct passed to the layout template execution.
// It contains common data needed by the layout (like user info, CSRF token, common URLs)
// and view-specific data (Data field). It now also includes header navigation fields.
type PageData struct {
	IsPublicPage        bool
	UserName            string
	UserPicture         string
	CSRFToken           string
	URLForLogout        string
	URLForRoot          string
	Data                interface{}
	AppTitle            string
	EventsManagerLabel  string
	URLForEventsManager string
	VenueManagerLabel   string
	URLForVenueManager  string
	LabelWelcome        string
	LabelSignOut        string
	LabelNotSignedIn    string
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

// BaseHttpHandler provides common context (database, logger, base URL) and helper methods
// for resource-specific HTTP handlers (like Event or RSVP handlers).
type BaseHttpHandler struct {
	ApplicationContext        *config.ApplicationContext
	ResourceNameForLogging    string
	ResourceBasePathForRoutes string
}

// NewBaseHttpHandler creates a new BaseHttpHandler instance with the necessary context and configuration.
func NewBaseHttpHandler(applicationContext *config.ApplicationContext, resourceName string, resourceBasePath string) BaseHttpHandler {
	return BaseHttpHandler{
		ApplicationContext:        applicationContext,
		ResourceNameForLogging:    resourceName,
		ResourceBasePathForRoutes: resourceBasePath,
	}
}

// GetUserSessionData retrieves user data from the current request's session using the global GetUserData function.
func (handler *BaseHttpHandler) GetUserSessionData(httpRequest *http.Request) *LoggedUserData {
	return GetUserData(httpRequest)
}

// ValidateHttpMethod checks if the request's HTTP method is among the allowed methods.
// If not allowed, it logs the issue and sends a 405 Method Not Allowed response.
// Returns true if the method is allowed, false otherwise (response is sent).
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

// VerifyResourceOwnership checks if the currently authenticated user (currentUserID) owns the resource
// identified by the resourceOwnerID.
// It handles errors related to missing IDs or ownership mismatch by sending appropriate
// HTTP error responses (400, 403) using HandleError.
// Returns true if ownership is verified, false otherwise (response is sent).
func (handler *BaseHttpHandler) VerifyResourceOwnership(responseWriter http.ResponseWriter, request *http.Request, resourceOwnerID string, currentUserID string) bool {
	if resourceOwnerID == "" {
		handler.ApplicationContext.Logger.Printf("Ownership check failed: Resource owner ID is missing for %s on path %s.", handler.ResourceNameForLogging, request.URL.Path)
		handler.HandleError(responseWriter, nil, utils.ServerError, "Internal Server Error: Cannot verify ownership.")
		return false
	}
	if currentUserID == "" {
		handler.ApplicationContext.Logger.Printf("Ownership check failed: Current user ID is missing for %s on path %s.", handler.ResourceNameForLogging, request.URL.Path)
		handler.HandleError(responseWriter, nil, utils.ServerError, "Internal Server Error: Cannot verify ownership.")
		return false
	}

	if resourceOwnerID != currentUserID {
		handler.ApplicationContext.Logger.Printf("Forbidden: User %s attempted action on %s owned by %s at path %s", currentUserID, handler.ResourceNameForLogging, resourceOwnerID, request.URL.Path)
		handler.HandleError(responseWriter, nil, utils.ForbiddenError, "Forbidden: You do not have permission to access this "+handler.ResourceNameForLogging+".")
		return false
	}

	return true
}

// GetParam retrieves a single named parameter from the request, checking both URL query and form values by default.
// It delegates the logic to utils.GetParam.
func (handler *BaseHttpHandler) GetParam(request *http.Request, parameterName string) string {
	return utils.GetParam(request, parameterName, utils.BothParams)
}

// GetParams retrieves multiple named parameters from the request, checking both URL query and form values by default.
// It delegates the logic to utils.GetParams.
func (handler *BaseHttpHandler) GetParams(request *http.Request, parameterNames ...string) map[string]string {
	return utils.GetParams(request, parameterNames, utils.BothParams)
}

// RequireParams retrieves multiple required parameters from the request (checking both query and form).
// If any specified parameter is missing or empty, it sends a 400 Bad Request response using HandleError
// and returns false.
// Returns the map of retrieved parameters (even if incomplete) and true only if all required parameters are present and non-empty.
func (handler *BaseHttpHandler) RequireParams(responseWriter http.ResponseWriter, request *http.Request, parameterNames ...string) (map[string]string, bool) {
	parameters := handler.GetParams(request, parameterNames...)
	missingParameters := []string{}
	for _, parameterName := range parameterNames {
		if value, exists := parameters[parameterName]; !exists || value == "" {
			missingParameters = append(missingParameters, parameterName)
		}
	}

	if len(missingParameters) > 0 {
		errorMessage := "Missing required parameter(s): " + strings.Join(missingParameters, ", ")
		handler.HandleError(responseWriter, nil, utils.ValidationError, errorMessage)
		return parameters, false
	}

	return parameters, true
}

// RedirectToList redirects the user to the base path defined for the handler's resource (e.g., /events/).
// Uses HTTP status 303 See Other, which is appropriate after POST requests.
func (handler *BaseHttpHandler) RedirectToList(responseWriter http.ResponseWriter, request *http.Request) {
	http.Redirect(responseWriter, request, handler.ResourceBasePathForRoutes, http.StatusSeeOther)
}

// RedirectWithParams redirects the user to the handler's resource base path, appending the specified query parameters.
// Uses HTTP status 303 See Other. Delegates URL building to utils.BuildRelativeURL.
func (handler *BaseHttpHandler) RedirectWithParams(responseWriter http.ResponseWriter, request *http.Request, parameters map[string]string) {
	redirectURL := utils.BuildRelativeURL(handler.ResourceBasePathForRoutes, parameters)
	http.Redirect(responseWriter, request, redirectURL, http.StatusSeeOther)
}

// HandleError provides consistent error logging and HTTP response generation based on the error type.
// It delegates the core logic to utils.HandleError, passing the handler's logger.
func (handler *BaseHttpHandler) HandleError(responseWriter http.ResponseWriter, err error, errorType utils.ErrorType, userMessage string) {
	utils.HandleError(responseWriter, err, errorType, handler.ApplicationContext.Logger, userMessage)
}

// RenderView renders the specified view template using the main application layout.
// It prepares the PageData struct, including user information for non-public pages and header navigation data,
// retrieves the precompiled template set from templates.PrecompiledTemplatesMap,
// and executes the "layout" template, passing PageData as the context.
func (handler *BaseHttpHandler) RenderView(
	httpResponseWriter http.ResponseWriter,
	httpRequest *http.Request,
	viewName string,
	viewSpecificData interface{},
) {
	publicViews := map[string]bool{
		config.TemplateResponse: true,
		config.TemplateThankYou: true,
		config.TemplateLanding:  true,
	}
	isPublicPage := publicViews[viewName]

	pageData := PageData{
		IsPublicPage:        isPublicPage,
		Data:                viewSpecificData,
		URLForLogout:        gconstants.LogoutPath,
		URLForRoot:          config.WebRoot,
		AppTitle:            config.AppTitle,
		EventsManagerLabel:  config.ResourceLabelEventManager,
		URLForEventsManager: config.WebEvents,
		VenueManagerLabel:   config.ResourceLabelVenueManager,
		URLForVenueManager:  config.WebVenues,
		LabelWelcome:        config.LabelWelcome,
		LabelSignOut:        config.LabelSignOut,
		LabelNotSignedIn:    config.LabelNotSignedIn,
	}
	if !isPublicPage {
		loggedUserData := handler.GetUserSessionData(httpRequest)
		pageData.UserName = loggedUserData.UserName
		pageData.UserPicture = loggedUserData.UserPicture
		if pageData.UserName == "" && pageData.UserPicture == "" {
			handler.ApplicationContext.Logger.Printf("WARN: Rendering non-public view '%s' but user session data (Name/Picture) seems incomplete for %s.", viewName, httpRequest.URL.Path)
		}
	}
	templateSet, exists := templates.PrecompiledTemplatesMap[viewName]
	if !exists {
		if viewName != config.TemplateLanding {
			handler.ApplicationContext.Logger.Printf("WARN: Template set for view '%s' not found in PrecompiledTemplatesMap. Attempting to render landing page.", viewName)
			handler.HandleError(httpResponseWriter, nil, utils.ServerError, utils.ErrMsgInternalServer)
			return
		}
		handler.ApplicationContext.Logger.Printf("CRITICAL: Template set for view '%s' not found in PrecompiledTemplatesMap.", viewName)
		handler.HandleError(httpResponseWriter, nil, utils.ServerError, utils.ErrMsgInternalServer)
		return

	}
	executionError := templateSet.ExecuteTemplate(httpResponseWriter, config.TemplateLayout, pageData)
	if executionError != nil {
		handler.ApplicationContext.Logger.Printf("ERROR: Failed to execute layout template for view '%s' (Resource: %s, Path: %s): %v", viewName, handler.ResourceNameForLogging, httpRequest.URL.Path, executionError)
	}
}
