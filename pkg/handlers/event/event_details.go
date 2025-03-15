package event

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
)

// EventDetailHandler loads the details of an event (with RSVPs) for a given eventID.
func EventDetailHandler(applicationContext *config.ApplicationContext, eventIdentifier uint) http.Handler {
	return http.HandlerFunc(func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		sessionData := handlers.GetUserData(httpRequest, applicationContext)
		if sessionData.UserEmail == "" {
			http.Redirect(httpResponseWriter, httpRequest, "/login", http.StatusSeeOther)
			return
		}

		var currentUser models.User
		if userError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); userError != nil {
			applicationContext.Logger.Println("User not found in DB:", userError)
			http.Redirect(httpResponseWriter, httpRequest, "/login", http.StatusSeeOther)
			return
		}

		var eventRecord models.Event
		if loadError := eventRecord.LoadWithRSVPs(applicationContext.Database, eventIdentifier); loadError != nil {
			applicationContext.Logger.Println("Event not found:", loadError)
			http.Redirect(httpResponseWriter, httpRequest, "/", http.StatusSeeOther)
			return
		}

		// Business logic: if an RSVP's Response is empty, set it to "Pending"
		for index, currentRSVP := range eventRecord.RSVPs {
			if currentRSVP.Response == "" {
				eventRecord.RSVPs[index].Response = "Pending"
			}
		}

		// Ensure the event belongs to the logged‑in user.
		if eventRecord.UserID != currentUser.ID {
			http.Redirect(httpResponseWriter, httpRequest, "/", http.StatusSeeOther)
			return
		}

		templateData := struct {
			UserPicture string
			UserName    string
			Event       models.Event
		}{
			UserPicture: sessionData.UserPicture,
			UserName:    sessionData.UserName,
			Event:       eventRecord,
		}

		if renderError := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, "event_detail.html", templateData); renderError != nil {
			applicationContext.Logger.Printf("Error rendering event_detail.html: %v", renderError)
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
		}
	})
}
