package rsvp

import (
	"errors"
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/middleware"
	"github.com/temirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

// DeleteHandler handles DELETE requests (or POST with _method=DELETE override) to delete an RSVP.
func DeleteHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameRSVP, config.WebRSVPs)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodDelete) {
			return
		}

		currentUser := httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User)

		params, paramsOk := baseHandler.RequireParams(httpResponseWriter, httpRequest, config.RSVPIDParam)
		if !paramsOk {
			return
		}
		targetRsvpID := params[config.RSVPIDParam]

		var rsvpRecord models.RSVP
		if findError := applicationContext.Database.First(&rsvpRecord, "id = ?", targetRsvpID).Error; findError != nil {
			if errors.Is(findError, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findError, utils.NotFoundError, "RSVP not found.")
			} else {
				baseHandler.HandleError(httpResponseWriter, findError, utils.DatabaseError, "Error retrieving RSVP details.")
			}
			return
		}
		parentEventID := rsvpRecord.EventID

		var parentEvent models.Event
		eventFindError := applicationContext.Database.First(&parentEvent, "id = ?", parentEventID).Error
		if eventFindError != nil {
			if errors.Is(eventFindError, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, eventFindError, utils.NotFoundError, "Parent event not found for RSVP.")
			} else {
				baseHandler.HandleError(httpResponseWriter, eventFindError, utils.DatabaseError, "Error retrieving parent event.")
			}
			return
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, httpRequest, parentEvent.UserID, currentUser.ID) {
			return
		}

		if deleteError := applicationContext.Database.Delete(&rsvpRecord).Error; deleteError != nil {
			baseHandler.HandleError(httpResponseWriter, deleteError, utils.DatabaseError, "Failed to delete the RSVP.")
			return
		}

		redirectParams := map[string]string{
			config.EventIDParam: parentEventID,
		}
		baseHandler.RedirectWithParams(httpResponseWriter, httpRequest, redirectParams)
	}
}
