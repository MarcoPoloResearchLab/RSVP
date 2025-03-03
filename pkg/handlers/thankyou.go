package handlers

import (
	"fmt"
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// ThankYouHandler fetches the RSVP by code and displays a thank-you message.
func ThankYouHandler(applicationContext *config.App) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		code := httpRequest.URL.Query().Get("code")
		if code == "" {
			http.Error(httpResponseWriter, "Code is required", http.StatusBadRequest)
			return
		}

		var rsvp models.RSVP
		if err := rsvp.FindByCode(applicationContext.Database, code); err != nil {
			applicationContext.Logger.Println("Could not find invite for code:", code, err)
			http.Error(httpResponseWriter, "Invite not found", http.StatusNotFound)
			return
		}

		var thankYouMessage string
		if rsvp.Response == "Yes" {
			thankYouMessage = fmt.Sprintf("We are looking forward to seeing you +%d!", rsvp.ExtraGuests)
		} else {
			thankYouMessage = "Sorry you couldn’t make it!"
		}

		data := struct {
			Name            string
			Response        string
			ExtraGuests     int
			ThankYouMessage string
			Code            string
		}{
			Name:            rsvp.Name,
			Response:        rsvp.Response,
			ExtraGuests:     rsvp.ExtraGuests,
			ThankYouMessage: thankYouMessage,
			Code:            code,
		}

		if err := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, "thankyou.html", data); err != nil {
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			applicationContext.Logger.Printf("failed to render template thankyou.html: %v", err)
			return
		}
	}
}
