// Package rsvp contains HTTP handlers and router logic for RSVP resources.
package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
)

// Router dispatches requests under "/rsvps/".
//
// - POST /rsvps/?event_id=XYZ => CreateHandler (creates new RSVP)
// - GET  /rsvps/?event_id=XYZ => ListHandler (shows RSVP list for event XYZ, protected)
// - GET  /rsvps/?rsvp_id=ABC  => ShowHandler (printing page / QR code for RSVP ABC)
// - PUT  /rsvps/?rsvp_id=ABC  => (handled by ListHandler) merges with edit form
// - DELETE /rsvps/?rsvp_id=ABC => DeleteHandler (removes RSVP ABC)
//
// Notice: We set “Update” to nil so that “PUT” calls ListHandler instead.
func Router(appCtx *config.ApplicationContext) http.HandlerFunc {
	rsvpHandlers := handlers.ResourceHandlers{
		List:   ListHandler(appCtx),   // GET or PUT => see "list.go"
		Create: CreateHandler(appCtx), // POST => new RSVP
		Update: ListHandler(appCtx),
		Delete: DeleteHandler(appCtx), // DELETE => remove RSVP
		Show:   ShowHandler(appCtx),   // GET => printing page
	}

	routerConfig := handlers.NewRSVPRouterConfig() // uses "rsvp_id" + method override
	return handlers.ResourceRouter(appCtx, rsvpHandlers, routerConfig)
}
