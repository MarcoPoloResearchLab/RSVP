package event

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// DeleteHandler handles DELETE requests to delete an event.
func DeleteHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(applicationContext, "Event", config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateMethod(httpResponseWriter, httpRequest, http.MethodDelete, http.MethodPost) {
			return
		}

		parameterMap, validParameters := baseHandler.RequireParams(httpResponseWriter, httpRequest, config.EventIDParam)
		if !validParameters {
			return
		}
		targetEventID := parameterMap[config.EventIDParam]

		sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

		var currentUser models.User
		findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail)
		if findUserError != nil {
			baseHandler.HandleError(httpResponseWriter, findUserError, utils.DatabaseError, "User not found in database")
			return
		}

		findEventOwnerID := func(eventID string) (string, error) {
			var eventInstance models.Event
			loadError := eventInstance.FindByID(applicationContext.Database, eventID)
			if loadError != nil {
				return "", loadError
			}
			return eventInstance.UserID, nil
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, targetEventID, findEventOwnerID, currentUser.ID) {
			return
		}

		var eventRecord models.Event
		findEventError := eventRecord.FindByID(applicationContext.Database, targetEventID)
		if findEventError != nil {
			baseHandler.HandleError(httpResponseWriter, findEventError, utils.NotFoundError, "Event not found")
			return
		}

		deleteRSVPsError := applicationContext.Database.Where("event_id = ?", targetEventID).Delete(&models.RSVP{}).Error
		if deleteRSVPsError != nil {
			baseHandler.HandleError(httpResponseWriter, deleteRSVPsError, utils.DatabaseError, "Failed to delete associated RSVPs")
			return
		}

		deletionError := applicationContext.Database.Delete(&eventRecord).Error
		if deletionError != nil {
			baseHandler.HandleError(httpResponseWriter, deletionError, utils.DatabaseError, "Failed to delete event")
			return
		}

		baseHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
