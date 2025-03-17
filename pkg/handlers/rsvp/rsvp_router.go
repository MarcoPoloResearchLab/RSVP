package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
)

// RSVPRouter dispatches requests under the "/rsvps/" base path.
// It supports:
//   - GET "/rsvps?event_id={event_id}" for listing RSVPs for an event
//   - POST "/rsvps?event_id={event_id}" for creating a new RSVP
//   - GET "/rsvps?rsvp_id={rsvp_id}" for viewing RSVP details
//   - PUT/POST "/rsvps?rsvp_id={rsvp_id}" for updating an RSVP
//   - DELETE "/rsvps?rsvp_id={rsvp_id}" for deleting an RSVP
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
	routerConfig := handlers.NewRSVPRouterConfig()

	// Create and return the resource router
	return handlers.ResourceRouter(applicationContext, rsvpHandlers, routerConfig)
}
