package handlers

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/utils"
)

type ResourceHandlers struct {
	List   http.HandlerFunc
	Create http.HandlerFunc
	Show   http.HandlerFunc
	Update http.HandlerFunc
	Delete http.HandlerFunc
}

type ResourceRouterConfig struct {
	IDParam       string
	ParentIDParam string
	MethodParam   string
	ResourceType  string
}

func DefaultResourceRouterConfig() ResourceRouterConfig {
	return ResourceRouterConfig{
		IDParam:       config.EventIDParam,
		ParentIDParam: "",
		MethodParam:   config.MethodOverrideParam,
		ResourceType:  "Event",
	}
}

func NewEventRouterConfig() ResourceRouterConfig {
	return ResourceRouterConfig{
		IDParam:       config.EventIDParam,
		ParentIDParam: "",
		MethodParam:   config.MethodOverrideParam,
		ResourceType:  "Event",
	}
}

func NewRSVPRouterConfig() ResourceRouterConfig {
	return ResourceRouterConfig{
		IDParam:       config.RSVPIDParam,
		ParentIDParam: config.EventIDParam,
		MethodParam:   config.MethodOverrideParam,
		ResourceType:  "RSVP",
	}
}

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
	resourceHandlers ResourceHandlers,
	routerConfiguration ResourceRouterConfig,
) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.Method == http.MethodPost {
			formParseError := httpRequest.ParseForm()
			if formParseError != nil {
				http.Error(httpResponseWriter, "Invalid form data", http.StatusBadRequest)
				return
			}
		}

		utils.ApplyMethodOverride(httpRequest, routerConfiguration.MethodParam)

		var resourceID string
		var parentResourceID string

		resourceID = httpRequest.URL.Query().Get(routerConfiguration.IDParam)
		if routerConfiguration.ParentIDParam != "" {
			parentResourceID = httpRequest.URL.Query().Get(routerConfiguration.ParentIDParam)
		}

		if resourceID == "" {
			resourceID = httpRequest.FormValue(routerConfiguration.IDParam)
		}
		if routerConfiguration.ParentIDParam != "" && parentResourceID == "" {
			parentResourceID = httpRequest.FormValue(routerConfiguration.ParentIDParam)
		}

		appContext.Logger.Printf(
			"Request: Method=%s, ResourceID=%s, ParentID=%s",
			httpRequest.Method,
			resourceID,
			parentResourceID,
		)

		switch {
		case httpRequest.Method == http.MethodDelete && resourceID != "":
			if resourceHandlers.Delete != nil {
				resourceHandlers.Delete.ServeHTTP(httpResponseWriter, httpRequest)
			} else {
				http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
			}

		case httpRequest.Method == http.MethodGet:
			if resourceID != "" {
				if routerConfiguration.ResourceType == "RSVP" &&
					httpRequest.URL.Query().Get("print") == "true" {
					if resourceHandlers.Show != nil {
						resourceHandlers.Show.ServeHTTP(httpResponseWriter, httpRequest)
					} else {
						http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
					}
				} else {
					if resourceHandlers.List != nil {
						resourceHandlers.List.ServeHTTP(httpResponseWriter, httpRequest)
					} else {
						http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
					}
				}
			} else {
				if resourceHandlers.List != nil {
					resourceHandlers.List.ServeHTTP(httpResponseWriter, httpRequest)
				} else {
					http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
				}
			}

		case httpRequest.Method == http.MethodPost:
			if resourceID != "" {
				if resourceHandlers.Update != nil {
					resourceHandlers.Update.ServeHTTP(httpResponseWriter, httpRequest)
				} else {
					http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
				}
			} else {
				if resourceHandlers.Create != nil {
					resourceHandlers.Create.ServeHTTP(httpResponseWriter, httpRequest)
				} else {
					http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
				}
			}

		case httpRequest.Method == http.MethodPut || httpRequest.Method == http.MethodPatch:
			if resourceID == "" {
				http.Error(httpResponseWriter, routerConfiguration.IDParam+" is required", http.StatusBadRequest)
				return
			}
			if resourceHandlers.Update != nil {
				resourceHandlers.Update.ServeHTTP(httpResponseWriter, httpRequest)
			} else {
				http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
			}

		case httpRequest.Method == http.MethodDelete:
			if resourceID == "" {
				http.Error(httpResponseWriter, routerConfiguration.IDParam+" is required", http.StatusBadRequest)
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
