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
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, "Event", config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodDelete, http.MethodPost) {
			return
		}

		params, ok := baseHandler.RequireParams(httpResponseWriter, httpRequest, config.EventIDParam)
		if !ok {
			return
		}
		targetEventID := params[config.EventIDParam]

		sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

		var currentUser models.User
		if err := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.DatabaseError, "User not found in database")
			return
		}

		findEventOwnerID := func(eventID string) (string, error) {
			var eventInstance models.Event
			if err := eventInstance.FindByID(applicationContext.Database, eventID); err != nil {
				return "", err
			}
			return eventInstance.UserID, nil
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, targetEventID, findEventOwnerID, currentUser.ID) {
			return
		}

		var eventRecord models.Event
		if err := eventRecord.FindByID(applicationContext.Database, targetEventID); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.NotFoundError, "Event not found")
			return
		}

		if err := applicationContext.Database.Where("event_id = ?", targetEventID).Delete(&models.RSVP{}).Error; err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.DatabaseError, "Failed to delete associated RSVPs")
			return
		}

		if err := applicationContext.Database.Delete(&eventRecord).Error; err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.DatabaseError, "Failed to delete event")
			return
		}

		baseHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
