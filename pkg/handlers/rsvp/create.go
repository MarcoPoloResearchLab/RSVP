package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// CreateHandler handles POST requests to create a new RSVP.
func CreateHandler(appCtx *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(appCtx, "RSVP", config.WebRSVPs)

	return func(w http.ResponseWriter, r *http.Request) {
		if !baseHandler.ValidateMethod(w, r, http.MethodPost) {
			return
		}

		eventIdentifier := baseHandler.GetParam(r, config.EventIDParam)
		if eventIdentifier == "" {
			http.Error(w, "Event ID is required", http.StatusBadRequest)
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
		if !baseHandler.VerifyResourceOwnership(w, eventIdentifier, findEventOwnerID, currentUser.ID) {
			return
		}

		rsvpName := r.FormValue(config.NameParam)
		if rsvpName == "" {
			rsvpName = baseHandler.GetParam(r, config.NameParam)
		}
		if rsvpName == "" {
			baseHandler.HandleError(w, nil, utils.ValidationError, "RSVP name is required")
			return
		}

		if nameErr := utils.ValidateRSVPName(rsvpName); nameErr != nil {
			baseHandler.HandleError(w, nameErr, utils.ValidationError, nameErr.Error())
			return
		}

		newRSVP := models.RSVP{
			Name:    rsvpName,
			EventID: eventIdentifier,
		}
		if errCreate := newRSVP.Create(appCtx.Database); errCreate != nil {
			baseHandler.HandleError(w, errCreate, utils.DatabaseError, "Failed to create RSVP")
			return
		}

		baseHandler.RedirectWithParams(w, r, map[string]string{
			config.EventIDParam: eventIdentifier,
		})
	}
}
