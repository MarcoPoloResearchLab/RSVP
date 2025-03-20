package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// CreateHandler handles POST requests to create a new RSVP.
func CreateHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateMethod(httpResponseWriter, httpRequest, http.MethodPost) {
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
			var eventRecord models.Event
			eventFindError := eventRecord.FindByID(applicationContext.Database, eventID)
			if eventFindError != nil {
				return "", eventFindError
			}
			return eventRecord.UserID, nil
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, eventIdentifier, findEventOwnerID, currentUser.ID) {
			return
		}

		rsvpName := httpRequest.FormValue(config.NameParam)
		if rsvpName == "" {
			rsvpName = baseHandler.GetParam(httpRequest, config.NameParam)
		}

		baseHandler.Logger().Printf("Creating RSVP with name: %s for event: %s", rsvpName, eventIdentifier)

		if rsvpName == "" {
			baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "RSVP name is required")
			return
		}

		nameValidationError := utils.ValidateRSVPName(rsvpName)
		if nameValidationError != nil {
			baseHandler.HandleError(httpResponseWriter, nameValidationError, utils.ValidationError, nameValidationError.Error())
			return
		}

		newRSVP := models.RSVP{
			Name:    rsvpName,
			EventID: eventIdentifier,
		}
		rsvpCreateError := newRSVP.Create(applicationContext.Database)
		if rsvpCreateError != nil {
			baseHandler.HandleError(httpResponseWriter, rsvpCreateError, utils.DatabaseError, "Failed to create RSVP")
			return
		}

		formEventID := baseHandler.GetParam(httpRequest, config.EventIDParam)
		if formEventID != "" {
			baseHandler.RedirectWithParams(httpResponseWriter, httpRequest, map[string]string{
				config.EventIDParam: eventIdentifier,
			})
		} else {
			baseHandler.RedirectWithParams(httpResponseWriter, httpRequest, map[string]string{
				config.RSVPIDParam: newRSVP.ID,
			})
		}
	}
}
