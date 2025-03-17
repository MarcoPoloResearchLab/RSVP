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
	// Create a base handler for RSVPs
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(w http.ResponseWriter, r *http.Request) {
		// Validate HTTP method
		if !baseHandler.ValidateMethod(w, r, http.MethodPost) {
			return
		}

		// Get event ID parameter
		eventID := baseHandler.GetParam(r, "event_id")
		if eventID == "" {
			http.Error(w, "Event ID is required", http.StatusBadRequest)
			return
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
		
		// Verify that the current user owns the event
		if !baseHandler.VerifyResourceOwnership(w, eventID, findEventOwnerID, currentUser.ID) {
			return
		}

		// Get RSVP name parameter
		rsvpName := baseHandler.GetParam(r, "name")
		
		// Validate RSVP name
		if nameError := utils.ValidateRSVPName(rsvpName); nameError != nil {
			baseHandler.HandleError(w, nameError, utils.ValidationError, nameError.Error())
			return
		}

		// Create the new RSVP
		newRSVP := models.RSVP{
			Name:    rsvpName,
			EventID: eventID,
		}
		if createError := newRSVP.Create(applicationContext.Database); createError != nil {
			baseHandler.HandleError(w, createError, utils.DatabaseError, "Failed to create RSVP")
			return
		}

		// Check if the request came from the admin UI (form has event_id)
		formEventID := baseHandler.GetParam(r, "event_id")
		if formEventID != "" {
			// Redirect back to the RSVP list for this event
			baseHandler.RedirectWithParams(w, r, map[string]string{
				"event_id": eventID,
			})
		} else {
			// Redirect to the RSVP detail route using the RSVP ID as the code
			baseHandler.RedirectWithParams(w, r, map[string]string{
				"id": newRSVP.ID,
			})
		}
	}
}
