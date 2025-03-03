package handlers

import (
	"fmt"
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// RsvpHandler fetches an RSVP by code and renders the RSVP page.
func RsvpHandler(applicationContext *config.App) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		code := httpRequest.URL.Query().Get("code")
		if code == "" {
			http.Redirect(httpResponseWriter, httpRequest, config.WebRoot, http.StatusSeeOther)
			return
		}

		var rsvp models.RSVP
		if err := rsvp.FindByCode(applicationContext.Database, code); err != nil {
			applicationContext.Logger.Println("Could not find invite for code:", code, err)
			http.Error(httpResponseWriter, "Invite not found", http.StatusNotFound)
			return
		}

		displayName := rsvp.Name
		if displayName == "" {
			displayName = "Friend"
		}

		// Build current answer using response and extra guests.
		currentAnswer := fmt.Sprintf("%s,%d", rsvp.Response, rsvp.ExtraGuests)
		data := struct {
			Name          string
			Code          string
			CurrentAnswer string
		}{
			Name:          displayName,
			Code:          code,
			CurrentAnswer: currentAnswer,
		}

		if err := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, "rsvp.html", data); err != nil {
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			applicationContext.Logger.Printf("failed to render template rsvp.html: %v", err)
			return
		}
	}
}
