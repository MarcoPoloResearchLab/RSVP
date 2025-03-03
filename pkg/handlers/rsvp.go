package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// RsvpHandler handles both retrieving an RSVP form (GET) and submitting an RSVP (POST).
// - GET: Renders the RSVP form based on the "code" query parameter.
// - POST: Processes the RSVP submission and updates the RSVP record.
func RsvpHandler(applicationContext *config.App) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		switch httpRequest.Method {
		case http.MethodGet:
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

		case http.MethodPost:
			code := httpRequest.FormValue("code")
			responseValue := httpRequest.FormValue("response")
			parts := strings.Split(responseValue, ",")
			if len(parts) != 2 {
				parts = []string{"No", "0"}
			}
			rsvpResponse := parts[0]
			extraGuests, conversionErr := strconv.Atoi(parts[1])
			if conversionErr != nil {
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

		default:
			http.Error(httpResponseWriter, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
