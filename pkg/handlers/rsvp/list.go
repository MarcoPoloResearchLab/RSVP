package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// ListHandler handles GET /rsvps?event_id={event_id} for listing RSVPs for a specific event.
func ListHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		eventIdentifier := baseHandler.GetParam(httpRequest, config.EventIDParam)
		if eventIdentifier == "" {
			http.Error(httpResponseWriter, "Event ID is required", http.StatusBadRequest)
			return
		}

		sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

		var currentUser models.User
		findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail)
		if findUserError != nil {
			baseHandler.HandleError(httpResponseWriter, findUserError, utils.DatabaseError, "User not found in database")
			return
		}

		findEventOwnerID := func(eventID string) (string, error) {
			var loadedEvent models.Event
			eventLoadError := loadedEvent.FindByID(applicationContext.Database, eventID)
			if eventLoadError != nil {
				return "", eventLoadError
			}
			return loadedEvent.UserID, nil
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, eventIdentifier, findEventOwnerID, currentUser.ID) {
			return
		}

		var eventRecord models.Event
		findEventError := eventRecord.FindByID(applicationContext.Database, eventIdentifier)
		if findEventError != nil {
			baseHandler.HandleError(httpResponseWriter, findEventError, utils.NotFoundError, "Event not found")
			return
		}

		var rsvpRecords []models.RSVP
		rsvpQueryError := applicationContext.Database.Where("event_id = ?", eventIdentifier).Find(&rsvpRecords).Error
		if rsvpQueryError != nil {
			baseHandler.HandleError(httpResponseWriter, rsvpQueryError, utils.DatabaseError, "Could not retrieve RSVPs")
			return
		}

		for index, record := range rsvpRecords {
			if record.Response == "" {
				rsvpRecords[index].Response = "Pending"
			}
		}

		rsvpIDFromParams := baseHandler.GetParam(httpRequest, config.RSVPIDParam)
		var selectedRSVP *models.RSVP

		if rsvpIDFromParams != "" {
			for _, existingRSVP := range rsvpRecords {
				if existingRSVP.ID == rsvpIDFromParams {
					copyOfRSVP := existingRSVP
					selectedRSVP = &copyOfRSVP
					break
				}
			}

			if selectedRSVP == nil {
				var directLoadRSVP models.RSVP
				loadError := directLoadRSVP.FindByCode(applicationContext.Database, rsvpIDFromParams)
				if loadError == nil && directLoadRSVP.EventID == eventIdentifier {
					selectedRSVP = &directLoadRSVP
				}
			}
		}

		templateData := struct {
			RSVPRecords  []models.RSVP
			SelectedRSVP *models.RSVP
			Event        models.Event
			UserPicture  string
			UserName     string
		}{
			RSVPRecords:  rsvpRecords,
			SelectedRSVP: selectedRSVP,
			Event:        eventRecord,
			UserPicture:  sessionData.UserPicture,
			UserName:     sessionData.UserName,
		}

		baseHandler.RenderTemplate(httpResponseWriter, config.TemplateRSVPs, templateData)
	}
}
