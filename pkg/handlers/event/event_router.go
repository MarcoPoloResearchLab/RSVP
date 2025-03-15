package event

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/temirov/RSVP/pkg/config"
)

// EventRouter dispatches requests under the "/events/" base path.
// It supports:
//   - GET "/events/{event_id}" for event details,
//   - POST "/events/{event_id}/rsvps" for adding an RSVP,
//   - GET "/events/{event_id}/rsvps/{rsvp_code}" for viewing an RSVP detail.
func EventRouter(applicationContext *config.ApplicationContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Remove the base prefix "/events/"
		path := strings.TrimPrefix(r.URL.Path, config.WebEvents)
		segments := strings.Split(path, "/")
		if len(segments) < 1 || segments[0] == "" {
			http.NotFound(w, r)
			return
		}

		eventID, err := strconv.ParseUint(segments[0], 10, 32)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		eventIdentifier := uint(eventID)

		// If only the event id is provided, show event details.
		if len(segments) == 1 {
			EventDetailHandler(applicationContext, eventIdentifier).ServeHTTP(w, r)
			return
		}

		// If the next segment is "rsvps"
		if segments[1] == "rsvps" {
			// If exactly two segments and method POST, create an RSVP.
			if len(segments) == 2 && r.Method == http.MethodPost {
				CreateRSVPForEventHandler(applicationContext, eventIdentifier).ServeHTTP(w, r)
				return
			}
			// If three segments and method GET, show the RSVP detail.
			if len(segments) == 3 && r.Method == http.MethodGet {
				rsvpCode := segments[2] // RSVP code is a string
				EventRSVPDetailHandler(applicationContext, eventIdentifier, rsvpCode).ServeHTTP(w, r)
				return
			}
		}

		http.NotFound(w, r)
	}
}
