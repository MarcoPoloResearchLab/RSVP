package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
)

// RSVPRouter dispatches requests under the "/rsvps/" base path.
// It supports:
//   - GET "/rsvps?event_id={id}" for listing RSVPs for an event
//   - POST "/rsvps?event_id={id}" for creating a new RSVP
//   - GET "/rsvps?id={id}" for viewing RSVP details
//   - PUT/POST "/rsvps?id={id}" for updating an RSVP
//   - DELETE "/rsvps?id={id}" for deleting an RSVP
func RSVPRouter(applicationContext *config.ApplicationContext) http.HandlerFunc {
	// Define the handlers for RSVP operations
	rsvpHandlers := handlers.ResourceHandlers{
		List:   ListHandler(applicationContext),
		Create: CreateHandler(applicationContext),
		Show:   ShowHandler(applicationContext),
		Update: UpdateHandler(applicationContext),
		Delete: DeleteHandler(applicationContext),
	}

	// Configure the router with RSVP-specific parameters
	routerConfig := handlers.ResourceRouterConfig{
		IDParam:       "id",
		ParentIDParam: "event_id",
		MethodParam:   "_method",
	}

	// Create and return the resource router
	return handlers.ResourceRouter(applicationContext, rsvpHandlers, routerConfig)
}
