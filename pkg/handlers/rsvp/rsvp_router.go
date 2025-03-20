// Package rsvp contains HTTP handlers and router logic for RSVP resources.
package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
)

// Router dispatches requests under the "/rsvps/" base path.
func Router(applicationContext *config.ApplicationContext) http.HandlerFunc {
	rsvpHandlers := handlers.ResourceHandlers{
		List:   ListHandler(applicationContext),
		Create: CreateHandler(applicationContext),
		Show:   ShowHandler(applicationContext),
		Update: UpdateHandler(applicationContext),
		Delete: DeleteHandler(applicationContext),
	}

	routerConfiguration := handlers.NewRSVPRouterConfig()
	return handlers.ResourceRouter(applicationContext, rsvpHandlers, routerConfiguration)
}
