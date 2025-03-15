package event

import (
	"fmt"
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// CreateRSVPForEventHandler creates a new RSVP for the given event and redirects
// to the RSVP detail route: /events/{event_id}/rsvps/{rsvp_code}
func CreateRSVPForEventHandler(applicationContext *config.ApplicationContext, eventIdentifier uint) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		sessionData := handlers.GetUserData(r, applicationContext)
		if sessionData.UserEmail == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var currentUser models.User
		if err := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var eventRecord models.Event
		if err := eventRecord.FindByID(applicationContext.Database, eventIdentifier); err != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Ensure the event belongs to the current user.
		if eventRecord.UserID != currentUser.ID {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		rsvpName := r.FormValue("name")
		if rsvpName == "" {
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}

		newRSVP := models.RSVP{
			Name:    rsvpName,
			Code:    utils.Base36Encode6(), // Code is the string identifier
			EventID: eventIdentifier,
		}
		if err := newRSVP.Create(applicationContext.Database); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Redirect to the RSVP detail route using the RSVP code.
		redirectURL := fmt.Sprint(config.WebRSVP, eventIdentifier, newRSVP.Code)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	})
}
