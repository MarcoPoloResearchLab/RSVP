package event

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
)

// EventRouter dispatches requests under the "/events/" base path.
// It supports:
//   - GET "/events" for listing events
//   - POST "/events" for creating a new event
//   - GET "/events?id={id}" for viewing event details
//   - PUT/POST "/events?id={id}" for updating an event
//   - DELETE "/events?id={id}" for deleting an event
func EventRouter(applicationContext *config.ApplicationContext) http.HandlerFunc {
	// Define the handlers for event operations
	eventHandlers := handlers.ResourceHandlers{
		List:   ListHandler(applicationContext),
		Create: CreateHandler(applicationContext),
		Show:   ShowHandler(applicationContext),
		Update: UpdateHandler(applicationContext),
		Delete: DeleteHandler(applicationContext),
	}

	// Configure the router with event-specific parameters
	routerConfig := handlers.DefaultResourceRouterConfig()

	// Create and return the resource router
	return handlers.ResourceRouter(applicationContext, eventHandlers, routerConfig)
}
