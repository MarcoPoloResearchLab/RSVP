// Package rsvp contains HTTP handlers related to RSVP resource management.
package rsvp

import (
	"errors"
	"net/http"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

// CreateHandler handles POST requests to create a new RSVP for a specific event.
func CreateHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameRSVP, config.WebRSVPs)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodPost) {
			return
		}

		currentUser := httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User)

		if err := httpRequest.ParseForm(); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.ValidationError, utils.ErrMsgInvalidFormData)
			return
		}

		eventID := httpRequest.FormValue(config.EventIDParam)
		if eventID == "" {
			eventID = httpRequest.URL.Query().Get(config.EventIDParam)
			if eventID == "" {
				baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "Event ID is required to create an RSVP.")
				return
			}
		}

		var parentEvent models.Event
		eventFindError := applicationContext.Database.First(&parentEvent, "id = ?", eventID).Error
		if eventFindError != nil {
			if errors.Is(eventFindError, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, eventFindError, utils.NotFoundError, "Parent event not found.")
			} else {
				baseHandler.HandleError(httpResponseWriter, eventFindError, utils.DatabaseError, "Error retrieving parent event.")
			}
			return
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, httpRequest, parentEvent.UserID, currentUser.ID) {
			return
		}

		rsvpName := httpRequest.FormValue(config.NameParam)
		if validationError := utils.ValidateRSVPName(rsvpName); validationError != nil {
			baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
			return
		}

		newRSVP := models.RSVP{
			Name:    rsvpName,
			EventID: eventID,
		}

		if createError := newRSVP.Create(applicationContext.Database); createError != nil {
			baseHandler.HandleError(httpResponseWriter, createError, utils.DatabaseError, "Failed to create the RSVP.")
			return
		}

		redirectParams := map[string]string{
			config.EventIDParam: eventID,
		}
		baseHandler.RedirectWithParams(httpResponseWriter, httpRequest, redirectParams)
	}
}
