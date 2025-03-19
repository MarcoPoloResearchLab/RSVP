package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
)

// RSVPRouter dispatches requests under the "/rsvps/" base path.
func RSVPRouter(applicationContext *config.ApplicationContext) http.HandlerFunc {
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
