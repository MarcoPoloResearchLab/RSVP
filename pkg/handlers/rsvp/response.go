package rsvp

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// ResponseHandler handles GET and POST requests to /rsvp?code={code} (unprotected).
func ResponseHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		rsvpCode := httpRequest.URL.Query().Get("code")

		if rsvpCode == "" {
			http.Error(httpResponseWriter, "Missing RSVP code", http.StatusBadRequest)
			return
		}

		var rsvpRecord models.RSVP
		rsvpError := rsvpRecord.FindByCode(applicationContext.Database, rsvpCode)
		if rsvpError != nil {
			http.Error(httpResponseWriter, "Invalid RSVP code", http.StatusNotFound)
			return
		}

		var associatedEvent models.Event
		eventError := applicationContext.Database.First(&associatedEvent, "id = ?", rsvpRecord.EventID).Error
		if eventError != nil {
			http.Error(httpResponseWriter, "Event not found", http.StatusNotFound)
			return
		}

		if httpRequest.Method == http.MethodPost {
			parseFormError := httpRequest.ParseForm()
			if parseFormError != nil {
				http.Error(httpResponseWriter, "Invalid form data", http.StatusBadRequest)
				return
			}

			responseValue := httpRequest.FormValue("response")
			if responseValue == "" {
				http.Error(httpResponseWriter, "Response is required", http.StatusBadRequest)
				return
			}

			rsvpRecord.Response = responseValue

			responseParts := strings.Split(responseValue, ",")
			if len(responseParts) == 2 {
				var parsedExtraGuests int
				_, scanError := fmt.Sscanf(responseParts[1], "%d", &parsedExtraGuests)
				if scanError == nil {
					rsvpRecord.ExtraGuests = parsedExtraGuests
				}
			}

			saveError := rsvpRecord.Save(applicationContext.Database)
			if saveError != nil {
				applicationContext.Logger.Printf("Error saving RSVP response: %v", saveError)
				http.Error(httpResponseWriter, "Error saving response", http.StatusInternalServerError)
				return
			}

			thankYouTemplateData := struct {
				Name            string
				ThankYouMessage string
				ID              string
			}{
				Name:            rsvpRecord.Name,
				ThankYouMessage: "Your RSVP has been recorded. Thank you!",
				ID:              rsvpRecord.ID,
			}

			templateRenderError := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, config.TemplateThankYou, thankYouTemplateData)
			if templateRenderError != nil {
				applicationContext.Logger.Printf("Error rendering thank you template: %v", templateRenderError)
				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}

		responseTemplateData := struct {
			RSVP  models.RSVP
			Event models.Event
		}{
			RSVP:  rsvpRecord,
			Event: associatedEvent,
		}

		templateError := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, config.TemplateResponse, responseTemplateData)
		if templateError != nil {
			applicationContext.Logger.Printf("Error rendering response template: %v", templateError)
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}
