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
//   - GET "/events?event_id={event_id}" for displaying or editing an existing event
//   - PUT/POST "/events?event_id={event_id}" for updating an event
//   - DELETE "/events?event_id={event_id}" for deleting an event
func EventRouter(applicationContext *config.ApplicationContext) http.HandlerFunc {
	eventHandlers := handlers.ResourceHandlers{
		List:   ListHandler(applicationContext),
		Create: CreateHandler(applicationContext),
		Update: UpdateHandler(applicationContext),
		Delete: DeleteHandler(applicationContext),
	}

	routerConfiguration := handlers.NewEventRouterConfig()
	return handlers.ResourceRouter(applicationContext, eventHandlers, routerConfiguration)
}
