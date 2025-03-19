package response

import (
	"fmt"
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
)

// Show handles GET /rsvps/{code}/thankyou and displays the final thank-you message.
func Show(applicationContext *config.ApplicationContext, rsvpCode string) http.Handler {
	return http.HandlerFunc(func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if rsvpCode == "" || !handlers.ValidateRSVPCode(rsvpCode) {
			http.Error(httpResponseWriter, "Invalid or missing RSVP code", http.StatusBadRequest)
			return
		}
		var rsvpRecord models.RSVP
		findError := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode)
		if findError != nil {
			applicationContext.Logger.Println("RSVP not found for code:", rsvpCode, findError)
			http.Error(httpResponseWriter, "RSVP not found", http.StatusNotFound)
			return
		}
		var thankYouMessage string
		if rsvpRecord.Response == "Yes" {
			thankYouMessage = fmt.Sprintf("We look forward to seeing you with %d extra guest(s)!", rsvpRecord.ExtraGuests)
		} else {
			thankYouMessage = "Sorry you couldn’t make it!"
		}
		templateData := struct {
			Name            string
			Response        string
			ExtraGuests     int
			ThankYouMessage string
			Code            string
		}{
			Name:            rsvpRecord.Name,
			Response:        rsvpRecord.Response,
			ExtraGuests:     rsvpRecord.ExtraGuests,
			ThankYouMessage: thankYouMessage,
			Code:            rsvpRecord.ID,
		}
		templateError := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, "thankyou.html", templateData)
		if templateError != nil {
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			applicationContext.Logger.Printf("Failed to render thankyou.html: %v", templateError)
		}
	})
}
