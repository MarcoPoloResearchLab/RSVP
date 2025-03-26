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

// UpdateHandler handles POST/PUT/PATCH requests to update an existing RSVP.
func UpdateHandler(appCtx *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(appCtx, "RSVP", config.WebRSVPs)
	return func(w http.ResponseWriter, r *http.Request) {
		if !baseHandler.ValidateHttpMethod(w, r, http.MethodPost, http.MethodPut, http.MethodPatch) {
			return
		}

		rsvpIdentifier := baseHandler.GetParam(r, config.RSVPIDParam)
		if rsvpIdentifier == "" {
			http.Error(w, "RSVP ID is required", http.StatusBadRequest)
			return
		}

		var existingRSVP models.RSVP
		if err := existingRSVP.FindByCode(appCtx.Database, rsvpIdentifier); err != nil {
			baseHandler.HandleError(w, err, utils.NotFoundError, "RSVP not found")
			return
		}

		sessionData, _ := baseHandler.RequireAuthentication(w, r)
		var currentUser models.User
		if err := currentUser.FindByEmail(appCtx.Database, sessionData.UserEmail); err != nil {
			baseHandler.HandleError(w, err, utils.DatabaseError, "User not found in database")
			return
		}

		findEventOwnerID := func(eid string) (string, error) {
			var ev models.Event
			if err := ev.FindByID(appCtx.Database, eid); err != nil {
				return "", err
			}
			return ev.UserID, nil
		}
		if !baseHandler.VerifyResourceOwnership(w, existingRSVP.EventID, findEventOwnerID, currentUser.ID) {
			return
		}

		// Retrieve parameters "name", "response", and "extra_guests".
		formParams := baseHandler.GetParams(r, "name", "response", "extra_guests")

		if formParams["name"] != "" {
			if err := utils.ValidateRSVPName(formParams["name"]); err != nil {
				baseHandler.HandleError(w, err, utils.ValidationError, err.Error())
				return
			}
			existingRSVP.Name = formParams["name"]
		}

		if formParams["response"] != "" {
			if err := utils.ValidateRSVPResponse(formParams["response"]); err != nil {
				baseHandler.HandleError(w, err, utils.ValidationError, err.Error())
				return
			}
			existingRSVP.Response = formParams["response"]
			if strings.HasPrefix(formParams["response"], "Yes") && len(formParams["response"]) > 4 {
				parts := strings.Split(formParams["response"], ",")
				if len(parts) == 2 {
					if guestCount, errConv := strconv.Atoi(parts[1]); errConv == nil {
						existingRSVP.ExtraGuests = guestCount
					}
				}
			} else if formParams["response"] == "No,0" || formParams["response"] == "No" {
				existingRSVP.ExtraGuests = 0
			}
		} else if formParams["extra_guests"] != "" {
			newExtraGuests, errConv := strconv.Atoi(formParams["extra_guests"])
			if errConv != nil {
				baseHandler.HandleError(w, errConv, utils.ValidationError, "Invalid extra guests")
				return
			}
			if newExtraGuests < 0 || newExtraGuests > utils.MaxGuestCount {
				baseHandler.HandleError(w, errors.New("invalid guest count"), utils.ValidationError, "Guest count must be between 0 and "+strconv.Itoa(utils.MaxGuestCount))
				return
			}
			existingRSVP.ExtraGuests = newExtraGuests
		}

		if err := existingRSVP.Save(appCtx.Database); err != nil {
			baseHandler.HandleError(w, err, utils.DatabaseError, "Failed to update RSVP")
			return
		}

		baseHandler.RedirectWithParams(w, r, map[string]string{
			config.EventIDParam: existingRSVP.EventID,
		})
	}
}
