package rsvp

import (
	"fmt"
	"github.com/temirov/RSVP/pkg/handlers"
	"net/http"
	"strconv"
	"strings"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// GetSingleRSVPHandler handles GET /rsvps/{code} to display the RSVP form.
func GetSingleRSVPHandler(applicationContext *config.ApplicationContext, rsvpCode string) http.Handler {
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
		displayName := rsvpRecord.Name
		if displayName == "" {
			displayName = "Friend"
		}
		currentAnswer := fmt.Sprintf("%s,%d", rsvpRecord.Response, rsvpRecord.ExtraGuests)
		templateData := struct {
			Name          string
			Code          string
			CurrentAnswer string
		}{
			Name:          displayName,
			Code:          rsvpRecord.Code,
			CurrentAnswer: currentAnswer,
		}
		if templateError := applicationContext.Templates.ExecuteTemplate(responseWriter, "rsvp.html", templateData); templateError != nil {
			http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
			applicationContext.Logger.Printf("Failed to render rsvp.html: %v", templateError)
		}
	})
}

// Update handles POST /rsvps/{code} to update the RSVP with the user's response.
func Update(applicationContext *config.ApplicationContext, rsvpCode string) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if rsvpCode == "" || !handlers.ValidateRSVPCode(rsvpCode) {
			http.Error(responseWriter, "Invalid or missing RSVP code", http.StatusBadRequest)
			return
		}
		rawResponseValue := request.FormValue("response")
		if rawResponseValue == "" {
			rawResponseValue = "No,0"
		}
		responseParts := strings.Split(rawResponseValue, ",")
		if len(responseParts) != 2 {
			responseParts = []string{"No", "0"}
		}
		rsvpResponseValue := responseParts[0]
		// Only allow known responses.
		allowedResponses := map[string]bool{"Yes": true, "No": true, "Maybe": true}
		if !allowedResponses[rsvpResponseValue] {
			rsvpResponseValue = "No"
		}
		extraGuestsCount, conversionError := strconv.Atoi(responseParts[1])
		if conversionError != nil {
			http.Error(responseWriter, "Invalid extra guests count", http.StatusBadRequest)
			return
		}

		// Validate that the extra guests count is within acceptable limits (e.g., 0 to 10)
		if extraGuestsCount < 0 || extraGuestsCount > 10 {
			http.Error(responseWriter, "Extra guests count out of acceptable range", http.StatusBadRequest)
			return
		}

		var rsvpRecord models.RSVP
		if findError := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode); findError != nil {
			applicationContext.Logger.Println("RSVP not found for update with code:", rsvpCode, findError)
			http.Error(responseWriter, "RSVP not found", http.StatusNotFound)
			return
		}
		rsvpRecord.Response = rsvpResponseValue
		rsvpRecord.ExtraGuests = extraGuestsCount
		if saveError := rsvpRecord.Save(applicationContext.Database); saveError != nil {
			applicationContext.Logger.Println("Failed to save RSVP:", saveError)
			http.Error(responseWriter, "Could not update RSVP", http.StatusInternalServerError)
			return
		}
		thankYouURL := fmt.Sprintf("%s/%s%s", config.WebRSVPs, rsvpCode, config.WebThankYou)
		http.Redirect(responseWriter, request, thankYouURL, http.StatusSeeOther)
	})
}
