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
	baseHandler := handlers.NewBaseHttpHandler(appCtx, "RSVP", config.WebRSVPs)

	return func(w http.ResponseWriter, r *http.Request) {
		if !baseHandler.ValidateHttpMethod(w, r, http.MethodPost) {
			return
		}

		eventID := baseHandler.GetParam(r, config.EventIDParam)
		if eventID == "" {
			http.Error(w, "Event ID is required", http.StatusBadRequest)
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
		if !baseHandler.VerifyResourceOwnership(w, eventID, findEventOwnerID, currentUser.ID) {
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

		if err := utils.ValidateRSVPName(rsvpName); err != nil {
			baseHandler.HandleError(w, err, utils.ValidationError, err.Error())
			return
		}

		newRSVP := models.RSVP{
			Name:    rsvpName,
			EventID: eventID,
		}
		if err := newRSVP.Create(appCtx.Database); err != nil {
			baseHandler.HandleError(w, err, utils.DatabaseError, "Failed to create RSVP")
			return
		}

		baseHandler.RedirectWithParams(w, r, map[string]string{
			config.EventIDParam: eventID,
		})
	}
}
