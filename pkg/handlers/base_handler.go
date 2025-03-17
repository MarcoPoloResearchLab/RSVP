package handlers

import (
	"log"
	"net/http"
	"net/url"

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
func (h *BaseHandler) GetUserData(r *http.Request) *LoggedUserData {
	return GetUserData(r, h.ApplicationContext)
}

// ValidateMethod checks if the request method is one of the allowed methods
func (h *BaseHandler) ValidateMethod(w http.ResponseWriter, r *http.Request, allowedMethods ...string) bool {
	for _, method := range allowedMethods {
		if r.Method == method {
			return true
		}
	}

	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	return false
}

// RequireAuthentication gets the user data and checks if the user is authenticated.
// This method assumes the request has already passed through the gauss.AuthMiddleware,
// which guarantees that only authenticated users reach this point.
// It returns the user data and a boolean indicating if the user is authenticated.
func (h *BaseHandler) RequireAuthentication(w http.ResponseWriter, r *http.Request) (*LoggedUserData, bool) {
	sessionData := h.GetUserData(r)
	isAuthenticated := sessionData.UserEmail != ""
	
	if !isAuthenticated {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
	
	return sessionData, isAuthenticated
}

// VerifyResourceOwnership checks if the current user is the owner of a resource.
// It takes a resource ID, a function to find the resource owner ID, and the current user ID.
// It returns true if the user owns the resource, false otherwise.
// If the resource is not found or the user is not the owner, it returns an appropriate HTTP error.
func (h *BaseHandler) VerifyResourceOwnership(w http.ResponseWriter, resourceID string, 
	findOwnerIDFunc func(string) (string, error), currentUserID string) bool {
	
	// If no resource ID is provided, return true (creating a new resource)
	if resourceID == "" {
		return true
	}
	
	// Find the resource owner ID
	ownerID, findError := findOwnerIDFunc(resourceID)
	if findError != nil {
		h.HandleError(w, findError, utils.NotFoundError, "Resource not found")
		return false
	}
	
	// Check if the current user is the owner
	if ownerID != currentUserID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	
	return true
}

// GetParam extracts a parameter from the request
func (h *BaseHandler) GetParam(r *http.Request, paramName string) string {
	return utils.GetParam(r, paramName, utils.BothParams)
}

// GetParams extracts multiple parameters from the request
func (h *BaseHandler) GetParams(r *http.Request, paramNames ...string) map[string]string {
	return utils.GetParams(r, paramNames, utils.BothParams)
}

// RequireParams ensures all required parameters are present
func (h *BaseHandler) RequireParams(w http.ResponseWriter, r *http.Request, paramNames ...string) (map[string]string, bool) {
	params := h.GetParams(r, paramNames...)
	
	for _, name := range paramNames {
		if params[name] == "" {
			http.Error(w, name+" is required", http.StatusBadRequest)
			return params, false
		}
	}
	
	return params, true
}

// RedirectToList redirects to the list view for the resource
func (h *BaseHandler) RedirectToList(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.ResourcePath, http.StatusSeeOther)
}

// RedirectToResource redirects to a specific resource
func (h *BaseHandler) RedirectToResource(w http.ResponseWriter, r *http.Request, resourceID string) {
	redirectURL, _ := url.Parse(h.ResourcePath)
	queryParams := redirectURL.Query()
	queryParams.Set("id", resourceID)
	redirectURL.RawQuery = queryParams.Encode()
	http.Redirect(w, r, redirectURL.String(), http.StatusSeeOther)
}

// RedirectWithParams redirects to a URL with the given parameters
func (h *BaseHandler) RedirectWithParams(w http.ResponseWriter, r *http.Request, params map[string]string) {
	utils.RedirectWithParams(w, r, h.ResourcePath, params, http.StatusSeeOther)
}

// HandleError handles an error with appropriate logging and response
func (h *BaseHandler) HandleError(w http.ResponseWriter, err error, errorType utils.ErrorType, message string) {
	utils.HandleError(w, err, errorType, h.ApplicationContext.Logger, message)
}

// RenderTemplate renders a template with the given data
func (h *BaseHandler) RenderTemplate(w http.ResponseWriter, templateName string, data interface{}) {
	if err := h.ApplicationContext.Templates.ExecuteTemplate(w, templateName, data); err != nil {
		h.ApplicationContext.Logger.Printf("Error rendering template %s: %v", templateName, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Logger returns the application logger
func (h *BaseHandler) Logger() *log.Logger {
	return h.ApplicationContext.Logger
}
