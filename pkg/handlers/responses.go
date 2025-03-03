package handlers

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// ResponsesHandler lists all RSVPs.
func ResponsesHandler(applicationContext *config.App) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		var allRSVPs []models.RSVP
		if err := applicationContext.Database.Find(&allRSVPs).Error; err != nil {
			applicationContext.Logger.Println("Error fetching RSVPs:", err)
			http.Error(httpResponseWriter, "Could not retrieve RSVPs", http.StatusInternalServerError)
			return
		}

		// Mark empty responses as "Pending"
		for index := range allRSVPs {
			if allRSVPs[index].Response == "" {
				allRSVPs[index].Response = "Pending"
			}
		}

		loggedUserData := getUserData(httpRequest, applicationContext)
		data := struct {
			RSVPs          []models.RSVP
			LoggedUserData LoggedUserData
		}{
			RSVPs:          allRSVPs,
			LoggedUserData: *loggedUserData,
		}

		if err := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, "responses.html", data); err != nil {
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			applicationContext.Logger.Printf("failed to render template responses.html: %v", err)
			return
		}
	}
}
