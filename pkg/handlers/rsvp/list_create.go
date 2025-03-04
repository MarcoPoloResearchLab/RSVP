package rsvp

import (
	"fmt"
	"github.com/temirov/RSVP/pkg/handlers"
	"net/http"
	"strings"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/utils"
)

// ListCreateHandler handles GET /rsvps (listing RSVPs) and POST /rsvps (creating a new RSVP).
func ListCreateHandler(applicationContext *config.App) http.Handler {
	return http.HandlerFunc(func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.Method == http.MethodGet {
			var rsvpRecords []models.RSVP
			if databaseError := applicationContext.Database.Find(&rsvpRecords).Error; databaseError != nil {
				applicationContext.Logger.Println("Error fetching RSVPs:", databaseError)
				http.Error(httpResponseWriter, "Could not retrieve RSVPs", http.StatusInternalServerError)
				return
			}
			// Mark empty responses as "Pending"
			for index, record := range rsvpRecords {
				if record.Response == "" {
					rsvpRecords[index].Response = "Pending"
				}
			}
			userSessionData := handlers.GetUserData(httpRequest, applicationContext)
			templateData := struct {
				RSVPRecords []models.RSVP
				UserPicture string
				UserName    string
			}{
				RSVPRecords: rsvpRecords,
				UserPicture: userSessionData.UserPicture,
				UserName:    userSessionData.UserName,
			}
			if templateError := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, "responses.html", templateData); templateError != nil {
				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
				applicationContext.Logger.Printf("Failed to render responses.html: %v", templateError)
				return
			}
		} else if httpRequest.Method == http.MethodPost {
			inviteeNameValue := strings.TrimSpace(httpRequest.FormValue("name"))
			if inviteeNameValue == "" {
				http.Error(httpResponseWriter, "Name is required", http.StatusBadRequest)
				return
			}
			newRSVPRecord := models.RSVP{
				Name: inviteeNameValue,
				Code: utils.Base36Encode6(),
			}
			if creationError := newRSVPRecord.Create(applicationContext.Database); creationError != nil {
				applicationContext.Logger.Println("Error creating RSVP:", creationError)
				http.Error(httpResponseWriter, "Could not create RSVP", http.StatusInternalServerError)
				return
			}
			redirectURL := fmt.Sprintf("/rsvps/%s/qr", newRSVPRecord.Code)
			http.Redirect(httpResponseWriter, httpRequest, redirectURL, http.StatusSeeOther)
		} else {
			http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})
}
