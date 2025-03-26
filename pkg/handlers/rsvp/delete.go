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
	baseHandler := handlers.NewBaseHttpHandler(appCtx, "RSVP", config.WebRSVPs)

	return func(w http.ResponseWriter, r *http.Request) {
		if !baseHandler.ValidateHttpMethod(w, r, http.MethodDelete, http.MethodPost) {
			return
		}

		rsvpID := baseHandler.GetParam(r, config.RSVPIDParam)
		if rsvpID == "" {
			http.Error(w, "RSVP ID is required", http.StatusBadRequest)
			return
		}

		var rsvpRecord models.RSVP
		if err := rsvpRecord.FindByCode(appCtx.Database, rsvpID); err != nil {
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
		if !baseHandler.VerifyResourceOwnership(w, rsvpRecord.EventID, findEventOwnerID, currentUser.ID) {
			return
		}

		if err := appCtx.Database.Delete(&rsvpRecord).Error; err != nil {
			baseHandler.HandleError(w, err, utils.DatabaseError, "Failed to delete RSVP")
			return
		}

		baseHandler.RedirectWithParams(w, r, map[string]string{
			config.EventIDParam: rsvpRecord.EventID,
		})
	}
}
