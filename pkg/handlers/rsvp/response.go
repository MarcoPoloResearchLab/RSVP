package rsvp

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// ResponseHandler handles GET and POST to /response/?event_id=XXX (unprotected).
// Reuses your "response.html" for a single RSVP perspective.
func ResponseHandler(appCtx *config.ApplicationContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eventID := r.URL.Query().Get(config.EventIDParam)
		if eventID == "" {
			http.Error(w, "Missing event_id", http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodGet {
			// Minimal approach: create a synthetic RSVP object and show your existing response.html
			// The user can do "Yes,0" or "No,0", but it won't tie to an actual RSVP code unless you expand the logic.

			var eventRec models.Event
			if errEvt := eventRec.FindByID(appCtx.Database, eventID); errEvt != nil {
				http.Error(w, "Event not found", http.StatusNotFound)
				return
			}

			// Fake RSVP
			syntheticRSVP := models.RSVP{
				Name:    "Public Guest",
				EventID: eventID,
			}

			data := struct {
				RSVP  models.RSVP
				Event models.Event
			}{
				RSVP:  syntheticRSVP,
				Event: eventRec,
			}
			if errT := appCtx.Templates.ExecuteTemplate(w, config.TemplateResponse, data); errT != nil {
				http.Error(w, "Failed to render response.html", http.StatusInternalServerError)
			}
			return
		}

		if r.Method == http.MethodPost {
			if errForm := r.ParseForm(); errForm != nil {
				http.Error(w, "Invalid form data", http.StatusBadRequest)
				return
			}
			userResponse := r.FormValue("response")
			if userResponse == "" {
				http.Error(w, "Response is required", http.StatusBadRequest)
				return
			}

			// Minimal logic: store a "fake" public RSVP or do nothing. We'll just say "Thank you."
			// Real code might require user picking a real RSVP, etc.
			parts := strings.Split(userResponse, ",")
			var extra int
			if len(parts) == 2 {
				if val, errC := strconv.Atoi(parts[1]); errC == nil {
					extra = val
				}
			}
			finalMsg := fmt.Sprintf("Thank you for responding: %s with +%d. Event = %s, %s",
				userResponse, extra, eventID, time.Now().Format(time.RFC3339))
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(finalMsg))
			return
		}

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}
