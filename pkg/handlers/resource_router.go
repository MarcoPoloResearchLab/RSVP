package handlers

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
)

// ResourceHandlers holds handler functions for different operations on a resource
type ResourceHandlers struct {
	List   http.HandlerFunc
	Create http.HandlerFunc
	Show   http.HandlerFunc
	Update http.HandlerFunc
	Delete http.HandlerFunc
}

// ResourceRouterConfig holds configuration for a resource router
type ResourceRouterConfig struct {
	IDParam       string // The parameter name for resource ID
	ParentIDParam string // Optional parameter name for parent resource ID
	MethodParam   string // Parameter name for method override
	ResourceType  string // Type of resource (Event, RSVP, User)
}

// DefaultResourceRouterConfig returns a default configuration for resource routers
func DefaultResourceRouterConfig() ResourceRouterConfig {
	return ResourceRouterConfig{
		IDParam:       config.EventIDParam, // Default to event_id
		ParentIDParam: "",
		MethodParam:   config.MethodOverrideParam,
		ResourceType:  "Event",
	}
}

// NewEventRouterConfig returns a configuration for Event resource routers
func NewEventRouterConfig() ResourceRouterConfig {
	return ResourceRouterConfig{
		IDParam:       config.EventIDParam,
		ParentIDParam: "",
		MethodParam:   config.MethodOverrideParam,
		ResourceType:  "Event",
	}
}

// NewRSVPRouterConfig returns a configuration for RSVP resource routers
func NewRSVPRouterConfig() ResourceRouterConfig {
	return ResourceRouterConfig{
		IDParam:       config.RSVPIDParam,
		ParentIDParam: config.EventIDParam, // RSVPs have events as parents
		MethodParam:   config.MethodOverrideParam,
		ResourceType:  "RSVP",
	}
}

// NewUserRouterConfig returns a configuration for User resource routers
func NewUserRouterConfig() ResourceRouterConfig {
	return ResourceRouterConfig{
		IDParam:       config.UserIDParam,
		ParentIDParam: "",
		MethodParam:   config.MethodOverrideParam,
		ResourceType:  "User",
	}
}

// ResourceRouter creates a router for a resource type (event, rsvp, etc.)
func ResourceRouter(
	appContext *config.ApplicationContext,
	handlers ResourceHandlers,
	config ResourceRouterConfig,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get resource ID and parent ID from query parameters
		resourceID := r.URL.Query().Get(config.IDParam)
		var parentID string
		if config.ParentIDParam != "" {
			parentID = r.URL.Query().Get(config.ParentIDParam)
		}

		// Check for method override in form values
		var methodOverride string
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err == nil {
				methodOverride = r.FormValue(config.MethodParam)

				// If ID is not in query params, check form values
				if resourceID == "" {
					resourceID = r.FormValue(config.IDParam)
				}

				// If parent ID is not in query params, check form values
				if config.ParentIDParam != "" && parentID == "" {
					parentID = r.FormValue(config.ParentIDParam)
				}
			}
		}

		// Route based on HTTP method, method override, and presence of IDs
		switch {
		// Handle DELETE method override
		case methodOverride == "DELETE" && resourceID != "":
			if handlers.Delete != nil {
				handlers.Delete.ServeHTTP(w, r)
			} else {
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}

		// Handle GET requests
		case r.Method == http.MethodGet:
			if resourceID != "" {
				// Show a specific resource
				if handlers.Show != nil {
					handlers.Show.ServeHTTP(w, r)
				} else {
					http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				}
			} else {
				// List resources
				if handlers.List != nil {
					handlers.List.ServeHTTP(w, r)
				} else {
					http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				}
			}

		// Handle POST requests
		case r.Method == http.MethodPost:
			if resourceID != "" {
				// Update an existing resource
				if handlers.Update != nil {
					handlers.Update.ServeHTTP(w, r)
				} else {
					http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				}
			} else {
				// Create a new resource
				if handlers.Create != nil {
					handlers.Create.ServeHTTP(w, r)
				} else {
					http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				}
			}

		// Handle PUT/PATCH requests
		case r.Method == http.MethodPut || r.Method == http.MethodPatch:
			if resourceID == "" {
				http.Error(w, config.IDParam+" is required", http.StatusBadRequest)
				return
			}
			// Update an existing resource
			if handlers.Update != nil {
				handlers.Update.ServeHTTP(w, r)
			} else {
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}

		// Handle DELETE requests
		case r.Method == http.MethodDelete:
			if resourceID == "" {
				http.Error(w, config.IDParam+" is required", http.StatusBadRequest)
				return
			}
			// Delete a resource
			if handlers.Delete != nil {
				handlers.Delete.ServeHTTP(w, r)
			} else {
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}

		// Handle unsupported methods
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}
