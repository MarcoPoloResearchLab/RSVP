package response

import (
	"fmt"
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"net/http"
	"strconv"
	"strings"
)

// Handler handles unprotected GET/POST requests at /response/ for RSVP responses.
func Handler(appContext *config.ApplicationContext) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, httpRequest *http.Request) {
		rsvpCode := httpRequest.URL.Query().Get(config.RSVPIDParam)
		if rsvpCode == "" {
			http.NotFound(responseWriter, httpRequest)
			return
		}

		var rsvpRecord models.RSVP
		if findError := rsvpRecord.FindByCode(appContext.Database, rsvpCode); findError != nil {
			http.NotFound(responseWriter, httpRequest)
			return
		}

		switch httpRequest.Method {
		case http.MethodGet:
			var eventRecord models.Event
			if eventError := eventRecord.FindByID(appContext.Database, rsvpRecord.EventID); eventError != nil {
				http.NotFound(responseWriter, httpRequest)
				return
			}

			type publicResponseData struct {
				RSVP   models.RSVP
				Event  models.Event
				Notice string
			}
			data := publicResponseData{
				RSVP:   rsvpRecord,
				Event:  eventRecord,
				Notice: "Please respond below",
			}
			if templateError := appContext.Templates.ExecuteTemplate(responseWriter, config.TemplateResponse, data); templateError != nil {
				http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
				appContext.Logger.Printf("Failed to render response.html: %v", templateError)
			}

		case http.MethodPost:
			if formParseError := httpRequest.ParseForm(); formParseError != nil {
				http.Error(responseWriter, "Invalid form data", http.StatusBadRequest)
				return
			}
			userResponse := httpRequest.FormValue("response")
			if userResponse == "" {
				http.Error(responseWriter, "Response is required", http.StatusBadRequest)
				return
			}

			// Parse response format, e.g. "Yes,2" or "No,0"
			if strings.HasPrefix(userResponse, "Yes") {
				// userResponse should be "Yes,<number>"
				parts := strings.SplitN(userResponse, ",", 2)
				if len(parts) == 2 {
					guestCount, parseErr := strconv.Atoi(parts[1])
					if parseErr != nil {
						http.Error(responseWriter, "Invalid guest count", http.StatusBadRequest)
						return
					}
					rsvpRecord.Response = userResponse
					rsvpRecord.ExtraGuests = guestCount
				} else {
					// If it’s just "Yes" with no comma, treat as 0 guests
					rsvpRecord.Response = "Yes,0"
					rsvpRecord.ExtraGuests = 0
				}
			} else if strings.HasPrefix(userResponse, "No") {
				// userResponse should be "No,0" or just "No"
				rsvpRecord.Response = "No,0"
				rsvpRecord.ExtraGuests = 0
			} else {
				http.Error(responseWriter, "Invalid response format", http.StatusBadRequest)
				return
			}

			// Save RSVP changes
			if saveError := rsvpRecord.Save(appContext.Database); saveError != nil {
				appContext.Logger.Printf("Error saving RSVP: %v", saveError)
				http.Error(responseWriter, "Could not save RSVP", http.StatusInternalServerError)
				return
			}

			// Redirect to the proper thank-you page
			redirectURL := config.WebResponseThankYou + "?rsvp_id=" + rsvpCode
			http.Redirect(responseWriter, httpRequest, redirectURL, http.StatusSeeOther)

		default:
			http.Error(responseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}

// ThankYouHandler handles unprotected GET requests at /response/thankyou to display a thank-you page.
func ThankYouHandler(appContext *config.ApplicationContext) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, httpRequest *http.Request) {
		rsvpCode := httpRequest.URL.Query().Get(config.RSVPIDParam)
		if rsvpCode == "" {
			http.NotFound(responseWriter, httpRequest)
			return
		}

		var rsvpRecord models.RSVP
		if findError := rsvpRecord.FindByCode(appContext.Database, rsvpCode); findError != nil {
			http.NotFound(responseWriter, httpRequest)
			return
		}

		// If "Yes,N", show a personalized message with N guests; if "No", show apology message
		var thankYouMessage string
		if strings.HasPrefix(rsvpRecord.Response, "Yes") {
			thankYouMessage = fmt.Sprintf("We look forward to seeing you with %d extra guest(s)!", rsvpRecord.ExtraGuests)
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
			Name:            rsvpRecord.Name,
			Response:        rsvpRecord.Response,
			ExtraGuests:     rsvpRecord.ExtraGuests,
			ThankYouMessage: thankYouMessage,
			Code:            rsvpRecord.ID,
		}
		if templateError := appContext.Templates.ExecuteTemplate(responseWriter, config.TemplateThankYou, data); templateError != nil {
			http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
			appContext.Logger.Printf("Failed to render thankyou.html: %v", templateError)
		}
	}
}
