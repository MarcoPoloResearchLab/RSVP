package rsvp

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

// UpdateHandler handles PUT/PATCH requests (or POST with _method override) to update an existing RSVP.
func UpdateHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameRSVP, config.WebRSVPs)
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodPut, http.MethodPatch) {
			return
		}

		currentUser := httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User)

		params, paramsOk := baseHandler.RequireParams(httpResponseWriter, httpRequest, config.RSVPIDParam)
		if !paramsOk {
			return
		}
		targetRsvpID := params[config.RSVPIDParam]

		var existingRSVP models.RSVP
		if findError := applicationContext.Database.First(&existingRSVP, "id = ?", targetRsvpID).Error; findError != nil {
			if errors.Is(findError, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findError, utils.NotFoundError, "RSVP not found.")
			} else {
				baseHandler.HandleError(httpResponseWriter, findError, utils.DatabaseError, "Error retrieving RSVP details.")
			}
			return
		}
		parentEventID := existingRSVP.EventID

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

		if err := httpRequest.ParseForm(); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.ValidationError, utils.ErrMsgInvalidFormData)
			return
		}

		newName := httpRequest.FormValue(config.NameParam)
		if newName != "" {
			if validationError := utils.ValidateRSVPName(newName); validationError != nil {
				baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
				return
			}
			existingRSVP.Name = newName
		}

		newResponseStatus := httpRequest.FormValue(config.ResponseParam)
		newExtraGuestsStr := httpRequest.FormValue(config.ExtraGuestsParam)
		var newExtraGuests int = 0

		if validationError := utils.ValidateRSVPResponseStatus(newResponseStatus); validationError != nil {
			baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
			return
		}

		// Use config.RSVPResponseYesPrefix ("Yes") for comparison
		if newResponseStatus == config.RSVPResponseYesPrefix {
			var parseErr error
			newExtraGuests, parseErr = strconv.Atoi(newExtraGuestsStr)
			if parseErr != nil {
				baseHandler.HandleError(httpResponseWriter, parseErr, utils.ValidationError, utils.ErrGuestCountRequired.Error())
				return
			}
			if validationError := utils.ValidateExtraGuests(newExtraGuests); validationError != nil {
				baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
				return
			}
			existingRSVP.Response = config.RSVPResponseYesPrefix // Store "Yes"
			existingRSVP.ExtraGuests = newExtraGuests
		} else if newResponseStatus == config.RSVPResponseNo {
			existingRSVP.Response = config.RSVPResponseNoCommaZero // Store "No,0"
			existingRSVP.ExtraGuests = 0
		} else { // Includes Pending or empty string
			existingRSVP.Response = "" // Store "" for Pending
			existingRSVP.ExtraGuests = 0
		}

		if saveError := existingRSVP.Save(applicationContext.Database); saveError != nil {
			baseHandler.HandleError(httpResponseWriter, saveError, utils.DatabaseError, "Failed to update the RSVP.")
			return
		}

		redirectParams := map[string]string{
			config.EventIDParam: parentEventID,
		}
		baseHandler.RedirectWithParams(httpResponseWriter, httpRequest, redirectParams)
	}
}
