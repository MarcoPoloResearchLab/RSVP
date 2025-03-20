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
func UpdateHandler(appCtx *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(appCtx, "RSVP", config.WebRSVPs)

	return func(w http.ResponseWriter, r *http.Request) {
		if !baseHandler.ValidateMethod(w, r, http.MethodPost, http.MethodPut, http.MethodPatch) {
			return
		}

		rsvpIdentifier := baseHandler.GetParam(r, config.RSVPIDParam)
		if rsvpIdentifier == "" {
			http.Error(w, "RSVP ID is required", http.StatusBadRequest)
			return
		}

		var existingRSVP models.RSVP
		if findErr := existingRSVP.FindByCode(appCtx.Database, rsvpIdentifier); findErr != nil {
			baseHandler.HandleError(w, findErr, utils.NotFoundError, "RSVP not found")
			return
		}

		sessionData, _ := baseHandler.RequireAuthentication(w, r)

		var currentUser models.User
		if errUser := currentUser.FindByEmail(appCtx.Database, sessionData.UserEmail); errUser != nil {
			baseHandler.HandleError(w, errUser, utils.DatabaseError, "User not found in database")
			return
		}

		findEventOwnerID := func(eid string) (string, error) {
			var ev models.Event
			if errEv := ev.FindByID(appCtx.Database, eid); errEv != nil {
				return "", errEv
			}
			return ev.UserID, nil
		}
		if !baseHandler.VerifyResourceOwnership(w, existingRSVP.EventID, findEventOwnerID, currentUser.ID) {
			return
		}

		formParams := baseHandler.GetParams(r, "name", "response", "extra_guests")

		if formParams["name"] != "" {
			if nameErr := utils.ValidateRSVPName(formParams["name"]); nameErr != nil {
				baseHandler.HandleError(w, nameErr, utils.ValidationError, nameErr.Error())
				return
			}
			existingRSVP.Name = formParams["name"]
		}

		if formParams["response"] != "" {
			if respErr := utils.ValidateRSVPResponse(formParams["response"]); respErr != nil {
				baseHandler.HandleError(w, respErr, utils.ValidationError, respErr.Error())
				return
			}
			existingRSVP.Response = formParams["response"]
			if strings.HasPrefix(formParams["response"], "Yes") && len(formParams["response"]) > 4 {
				parts := strings.Split(formParams["response"], ",")
				if len(parts) == 2 {
					if guestCount, parseErr := strconv.Atoi(parts[1]); parseErr == nil {
						existingRSVP.ExtraGuests = guestCount
					}
				}
			} else if formParams["response"] == "No,0" || formParams["response"] == "No" {
				existingRSVP.ExtraGuests = 0
			}
		} else if formParams["extra_guests"] != "" {
			newExtraGuests, parseErr := strconv.Atoi(formParams["extra_guests"])
			if parseErr != nil {
				baseHandler.HandleError(w, parseErr, utils.ValidationError, "Invalid extra guests")
				return
			}
			if newExtraGuests < 0 || newExtraGuests > utils.MaxGuestCount {
				baseHandler.HandleError(w,
					errors.New("invalid guest count"),
					utils.ValidationError,
					"Guest count must be between 0 and "+strconv.Itoa(utils.MaxGuestCount),
				)
				return
			}
			existingRSVP.ExtraGuests = newExtraGuests
		}

		if errSave := existingRSVP.Save(appCtx.Database); errSave != nil {
			baseHandler.HandleError(w, errSave, utils.DatabaseError, "Failed to update RSVP")
			return
		}

		// Return to the RSVP list for this event
		baseHandler.RedirectWithParams(w, r, map[string]string{
			config.EventIDParam: existingRSVP.EventID,
		})
	}
}
