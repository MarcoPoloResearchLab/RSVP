package response

import (
	"fmt"
	"github.com/temirov/RSVP/pkg/handlers"
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// Show handles GET /rsvps/{code}/thankyou and displays the final thank-you message.
func Show(applicationContext *config.ApplicationContext, rsvpCode string) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if rsvpCode == "" || !handlers.ValidateRSVPCode(rsvpCode) {
			http.Error(responseWriter, "Invalid or missing RSVP code", http.StatusBadRequest)
			return
		}
		var rsvpRecord models.RSVP
		if findError := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode); findError != nil {
			applicationContext.Logger.Println("RSVP not found for code:", rsvpCode, findError)
			http.Error(responseWriter, "RSVP not found", http.StatusNotFound)
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
			Code:            rsvpRecord.Code,
		}
		if templateError := applicationContext.Templates.ExecuteTemplate(responseWriter, "thankyou.html", templateData); templateError != nil {
			http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
			applicationContext.Logger.Printf("Failed to render thankyou.html: %v", templateError)
		}
	})
}
