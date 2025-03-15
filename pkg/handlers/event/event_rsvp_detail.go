package event

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
)

// EventRSVPDetailHandler retrieves an RSVP by its code in the context of an event
// and renders the RSVP detail page (e.g. the QR code view).
func EventRSVPDetailHandler(applicationContext *config.ApplicationContext, eventIdentifier uint, rsvpCode string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionData := handlers.GetUserData(r, applicationContext)
		if sessionData.UserEmail == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Load the event to verify ownership.
		var eventRecord models.Event
		if err := eventRecord.FindByID(applicationContext.Database, eventIdentifier); err != nil {
			http.Redirect(w, r, config.WebEvents, http.StatusSeeOther)
			return
		}

		var currentUser models.User
		if err := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); err != nil || eventRecord.UserID != currentUser.ID {
			http.Redirect(w, r, config.WebEvents, http.StatusSeeOther)
			return
		}

		// Look up the RSVP by its code.
		var rsvpRecord models.RSVP
		if err := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode); err != nil || rsvpRecord.EventID != eventIdentifier {
			http.Redirect(w, r, "/events/", http.StatusSeeOther)
			return
		}

		templateData := struct {
			UserPicture string
			UserName    string
			RSVP        models.RSVP
			Event       models.Event
		}{
			UserPicture: sessionData.UserPicture,
			UserName:    sessionData.UserName,
			RSVP:        rsvpRecord,
			Event:       eventRecord,
		}

		if err := applicationContext.Templates.ExecuteTemplate(w, "rsvp.html", templateData); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	})
}
