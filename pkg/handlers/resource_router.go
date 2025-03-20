// Package handlers provides shared logic for all HTTP handler components.
package handlers

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/utils"
)

// ResourceHandlers wraps standard CRUD handlers for a resource.
type ResourceHandlers struct {
	List   http.HandlerFunc
	Create http.HandlerFunc
	Show   http.HandlerFunc
	Update http.HandlerFunc
	Delete http.HandlerFunc
}

// ResourceRouterConfig holds parameters for a ResourceRouter.
type ResourceRouterConfig struct {
	IDParam       string
	ParentIDParam string
	MethodParam   string
	ResourceType  string
}

// DefaultResourceRouterConfig returns default router config for events.
func DefaultResourceRouterConfig() ResourceRouterConfig {
	return ResourceRouterConfig{
		IDParam:       config.EventIDParam,
		ParentIDParam: "",
		MethodParam:   config.MethodOverrideParam,
		ResourceType:  "Event",
	}
}

// NewEventRouterConfig returns a router config for event resources.
func NewEventRouterConfig() ResourceRouterConfig {
	return ResourceRouterConfig{
		IDParam:       config.EventIDParam,
		ParentIDParam: "",
		MethodParam:   config.MethodOverrideParam,
		ResourceType:  "Event",
	}
}

// NewRSVPRouterConfig returns a router config for RSVP resources.
func NewRSVPRouterConfig() ResourceRouterConfig {
	return ResourceRouterConfig{
		IDParam:       config.RSVPIDParam,
		ParentIDParam: config.EventIDParam,
		MethodParam:   config.MethodOverrideParam,
		ResourceType:  "RSVP",
	}
}

// NewUserRouterConfig returns a router config for user resources.
func NewUserRouterConfig() ResourceRouterConfig {
	return ResourceRouterConfig{
		IDParam:       config.UserIDParam,
		ParentIDParam: "",
		MethodParam:   config.MethodOverrideParam,
		ResourceType:  "User",
	}
}

// ResourceRouter is a generic router for CRUD operations on a resource (Event, RSVP, etc.).
func ResourceRouter(
	appContext *config.ApplicationContext,
	resourceHandlers ResourceHandlers,
	routerConfig ResourceRouterConfig,
) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		// Parse form if POST
		if httpRequest.Method == http.MethodPost {
			if err := httpRequest.ParseForm(); err != nil {
				http.Error(httpResponseWriter, "Invalid form data", http.StatusBadRequest)
				return
			}
		}

		// Apply _method override if present
		utils.ApplyMethodOverride(httpRequest, routerConfig.MethodParam)

		resourceID := httpRequest.URL.Query().Get(routerConfig.IDParam)
		if resourceID == "" {
			resourceID = httpRequest.FormValue(routerConfig.IDParam)
		}

		var parentResourceID string
		if routerConfig.ParentIDParam != "" {
			parentResourceID = httpRequest.URL.Query().Get(routerConfig.ParentIDParam)
			if parentResourceID == "" {
				parentResourceID = httpRequest.FormValue(routerConfig.ParentIDParam)
			}
		}

		appContext.Logger.Printf(
			"ResourceRouter: Method=%s Resource=%s Parent=%s",
			httpRequest.Method, resourceID, parentResourceID,
		)

		switch httpRequest.Method {
		case http.MethodGet:
			if resourceID != "" {
				// If there's an ID, call ShowHandler
				if resourceHandlers.Show != nil {
					resourceHandlers.Show.ServeHTTP(httpResponseWriter, httpRequest)
				} else {
					http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
				}
			} else {
				// No ID => call ListHandler
				if resourceHandlers.List != nil {
					resourceHandlers.List.ServeHTTP(httpResponseWriter, httpRequest)
				} else {
					http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
				}
			}

		case http.MethodPost:
			if resourceID != "" {
				// Resource ID => update
				if resourceHandlers.Update != nil {
					resourceHandlers.Update.ServeHTTP(httpResponseWriter, httpRequest)
				} else {
					http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
				}
			} else {
				// No ID => create
				if resourceHandlers.Create != nil {
					resourceHandlers.Create.ServeHTTP(httpResponseWriter, httpRequest)
				} else {
					http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
				}
			}

		case http.MethodPut, http.MethodPatch:
			if resourceID == "" {
				http.Error(httpResponseWriter, routerConfig.IDParam+" is required", http.StatusBadRequest)
				return
			}
			if resourceHandlers.Update != nil {
				resourceHandlers.Update.ServeHTTP(httpResponseWriter, httpRequest)
			} else {
				http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
			}

		case http.MethodDelete:
			if resourceID == "" {
				http.Error(httpResponseWriter, routerConfig.IDParam+" is required", http.StatusBadRequest)
				return
			}
			if resourceHandlers.Delete != nil {
				resourceHandlers.Delete.ServeHTTP(httpResponseWriter, httpRequest)
			} else {
				http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
			}

		default:
			http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}
