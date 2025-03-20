package rsvp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// UpdateHandler handles PUT/POST requests to update an existing RSVP.
func UpdateHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateMethod(httpResponseWriter, httpRequest, http.MethodPost, http.MethodPut, http.MethodPatch) {
			return
		}

		rsvpIdentifier := baseHandler.GetParam(httpRequest, config.RSVPIDParam)
		if rsvpIdentifier == "" {
			http.Error(httpResponseWriter, "RSVP ID is required", http.StatusBadRequest)
			return
		}

		var existingRSVP models.RSVP
		findError := existingRSVP.FindByCode(applicationContext.Database, rsvpIdentifier)
		if findError != nil {
			baseHandler.HandleError(httpResponseWriter, findError, utils.NotFoundError, "RSVP not found")
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
			loadError := eventRecord.FindByID(applicationContext.Database, eventID)
			if loadError != nil {
				return "", loadError
			}
			return eventRecord.UserID, nil
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, existingRSVP.EventID, findEventOwnerID, currentUser.ID) {
			return
		}

		formParams := baseHandler.GetParams(httpRequest, "name", "response", "extra_guests")

		if formParams["name"] != "" {
			nameError := utils.ValidateRSVPName(formParams["name"])
			if nameError != nil {
				baseHandler.HandleError(httpResponseWriter, nameError, utils.ValidationError, nameError.Error())
				return
			}
			existingRSVP.Name = formParams["name"]
		}

		if formParams["response"] != "" {
			responseError := utils.ValidateRSVPResponse(formParams["response"])
			if responseError != nil {
				baseHandler.HandleError(httpResponseWriter, responseError, utils.ValidationError, responseError.Error())
				return
			}
			existingRSVP.Response = formParams["response"]

			if formParams["response"] != "No" && len(formParams["response"]) > 4 {
				parts := strings.Split(formParams["response"], ",")
				if len(parts) == 2 {
					guestCount, parseError := strconv.Atoi(parts[1])
					if parseError == nil {
						existingRSVP.ExtraGuests = guestCount
					}
				}
			} else if formParams["response"] == "No" {
				existingRSVP.ExtraGuests = 0
			}
		} else if formParams["extra_guests"] != "" {
			newExtraGuests, parseError := strconv.Atoi(formParams["extra_guests"])
			if parseError != nil {
				baseHandler.HandleError(httpResponseWriter, parseError, utils.ValidationError, "Invalid extra guests value")
				return
			}
			if newExtraGuests < 0 || newExtraGuests > utils.MaxGuestCount {
				baseHandler.HandleError(
					httpResponseWriter,
					errors.New("invalid guest count"),
					utils.ValidationError,
					"Guest count must be between 0 and "+strconv.Itoa(utils.MaxGuestCount),
				)
				return
			}
			existingRSVP.ExtraGuests = newExtraGuests
		}

		saveError := existingRSVP.Save(applicationContext.Database)
		if saveError != nil {
			baseHandler.HandleError(httpResponseWriter, saveError, utils.DatabaseError, "Failed to update RSVP")
			return
		}

		eventIDFromForm := baseHandler.GetParam(httpRequest, config.EventIDParam)
		if eventIDFromForm != "" {
			baseHandler.RedirectWithParams(httpResponseWriter, httpRequest, map[string]string{
				config.EventIDParam: eventIDFromForm,
			})
		} else {
			baseHandler.RedirectWithParams(httpResponseWriter, httpRequest, map[string]string{
				config.RSVPIDParam: rsvpIdentifier,
			})
		}
	}
}
