package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// SubmitHandler updates an RSVP's response and extra guests.
func SubmitHandler(applicationContext *config.App) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.Method == http.MethodPost {
			code := httpRequest.FormValue("code")
			responseValue := httpRequest.FormValue("response")

			parts := strings.Split(responseValue, ",")
			if len(parts) != 2 {
				parts = []string{"No", "0"}
			}
			rsvpResponse := parts[0]
			extraGuests, conversionError := strconv.Atoi(parts[1])
			if conversionError != nil {
				extraGuests = 0
			}

			var rsvp models.RSVP
			if err := rsvp.FindByCode(applicationContext.Database, code); err != nil {
				applicationContext.Logger.Println("Could not find invite to update for code:", code, err)
				http.Error(httpResponseWriter, "Invite not found", http.StatusNotFound)
				return
			}

			rsvp.Response = rsvpResponse
			rsvp.ExtraGuests = extraGuests
			if err := rsvp.Save(applicationContext.Database); err != nil {
				applicationContext.Logger.Println("Could not save RSVP:", err)
				http.Error(httpResponseWriter, "Could not update RSVP", http.StatusInternalServerError)
				return
			}

			http.Redirect(httpResponseWriter, httpRequest, config.WebThankYou+"?code="+code, http.StatusSeeOther)
			return
		}
		http.Redirect(httpResponseWriter, httpRequest, config.WebRoot, http.StatusSeeOther)
	}
}
