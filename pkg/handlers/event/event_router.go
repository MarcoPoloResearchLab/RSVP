// Package event contains HTTP handlers and router logic for event resources.
package event

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
)

// Router dispatches requests under the "/events/" base path.
func Router(applicationContext *config.ApplicationContext) http.HandlerFunc {
	eventHandlers := handlers.ResourceHandlers{
		List:   ListHandler(applicationContext),
		Create: CreateHandler(applicationContext),
		Update: UpdateHandler(applicationContext),
		Delete: DeleteHandler(applicationContext),
		Show:   ListHandler(applicationContext),
	}

	routerConfiguration := handlers.NewEventRouterConfig()
	return handlers.ResourceRouter(applicationContext, eventHandlers, routerConfiguration)
}
