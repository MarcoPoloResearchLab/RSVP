package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// DeleteHandler handles DELETE requests to delete an RSVP.
func DeleteHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	// Create a base handler for RSVPs
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(w http.ResponseWriter, r *http.Request) {
		// Validate HTTP method
		if !baseHandler.ValidateMethod(w, r, http.MethodDelete, http.MethodPost) {
			return
		}

		// Get RSVP ID parameter
		rsvpID := baseHandler.GetParam(r, "id")
		if rsvpID == "" {
			http.Error(w, "RSVP ID is required", http.StatusBadRequest)
			return
		}

		// Load the RSVP from the database
		var rsvpRecord models.RSVP
		if findError := rsvpRecord.FindByCode(applicationContext.Database, rsvpID); findError != nil {
			baseHandler.HandleError(w, findError, utils.NotFoundError, "RSVP not found")
			return
		}

		// Get the event ID for redirection
		eventID := baseHandler.GetParam(r, "event_id")
		
		// If not provided in the query or form, use the one from the RSVP record
		if eventID == "" {
			eventID = rsvpRecord.EventID
		}
		
		// Get authenticated user data (authentication is guaranteed by middleware)
		sessionData, _ := baseHandler.RequireAuthentication(w, r)

		// Find the current user
		var currentUser models.User
		if findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); findUserError != nil {
			baseHandler.HandleError(w, findUserError, utils.DatabaseError, "User not found in database")
			return
		}
		
		// Define a function to find the owner ID of an event
		findEventOwnerID := func(eventID string) (string, error) {
			var event models.Event
			if err := event.FindByID(applicationContext.Database, eventID); err != nil {
				return "", err
			}
			return event.UserID, nil
		}
		
		// Verify that the current user owns the event that this RSVP belongs to
		if !baseHandler.VerifyResourceOwnership(w, rsvpRecord.EventID, findEventOwnerID, currentUser.ID) {
			return
		}

		// Delete the RSVP
		if deletionError := applicationContext.Database.Delete(&rsvpRecord).Error; deletionError != nil {
			baseHandler.HandleError(w, deletionError, utils.DatabaseError, "Failed to delete RSVP")
			return
		}

		// Redirect back to the event's RSVPs list
		baseHandler.RedirectWithParams(w, r, map[string]string{
			"event_id": eventID,
		})
	}
}
