package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// DeleteHandler handles DELETE requests to delete an RSVP.
func DeleteHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateMethod(httpResponseWriter, httpRequest, http.MethodDelete, http.MethodPost) {
			return
		}

		rsvpIdentifier := baseHandler.GetParam(httpRequest, config.RSVPIDParam)
		if rsvpIdentifier == "" {
			http.Error(httpResponseWriter, "RSVP ID is required", http.StatusBadRequest)
			return
		}

		var rsvpRecord models.RSVP
		findError := rsvpRecord.FindByCode(applicationContext.Database, rsvpIdentifier)
		if findError != nil {
			baseHandler.HandleError(httpResponseWriter, findError, utils.NotFoundError, "RSVP not found")
			return
		}

		eventIdentifier := baseHandler.GetParam(httpRequest, config.EventIDParam)
		if eventIdentifier == "" {
			eventIdentifier = rsvpRecord.EventID
		}

		sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

		var currentUser models.User
		findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail)
		if findUserError != nil {
			baseHandler.HandleError(httpResponseWriter, findUserError, utils.DatabaseError, "User not found in database")
			return
		}

		findEventOwnerID := func(eventID string) (string, error) {
			var eventRecord models.Event
			loadError := eventRecord.FindByID(applicationContext.Database, eventID)
			if loadError != nil {
				return "", loadError
			}
			return eventRecord.UserID, nil
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, rsvpRecord.EventID, findEventOwnerID, currentUser.ID) {
			return
		}

		deletionError := applicationContext.Database.Delete(&rsvpRecord).Error
		if deletionError != nil {
			baseHandler.HandleError(httpResponseWriter, deletionError, utils.DatabaseError, "Failed to delete RSVP")
			return
		}

		baseHandler.RedirectWithParams(httpResponseWriter, httpRequest, map[string]string{
			config.EventIDParam: eventIdentifier,
		})
	}
}
