package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// DeleteHandler handles DELETE requests to delete an RSVP.
func DeleteHandler(appCtx *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(appCtx, "RSVP", config.WebRSVPs)

	return func(w http.ResponseWriter, r *http.Request) {
		if !baseHandler.ValidateMethod(w, r, http.MethodDelete, http.MethodPost) {
			return
		}

		rsvpIdentifier := baseHandler.GetParam(r, config.RSVPIDParam)
		if rsvpIdentifier == "" {
			http.Error(w, "RSVP ID is required", http.StatusBadRequest)
			return
		}

		var rsvpRecord models.RSVP
		if findErr := rsvpRecord.FindByCode(appCtx.Database, rsvpIdentifier); findErr != nil {
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
		if !baseHandler.VerifyResourceOwnership(w, rsvpRecord.EventID, findEventOwnerID, currentUser.ID) {
			return
		}

		if errDel := appCtx.Database.Delete(&rsvpRecord).Error; errDel != nil {
			baseHandler.HandleError(w, errDel, utils.DatabaseError, "Failed to delete RSVP")
			return
		}

		baseHandler.RedirectWithParams(w, r, map[string]string{
			config.EventIDParam: rsvpRecord.EventID,
		})
	}
}
