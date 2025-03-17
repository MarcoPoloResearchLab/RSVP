package event

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// DeleteHandler handles DELETE requests to delete an event.
func DeleteHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	// Create a base handler for events
	baseHandler := handlers.NewBaseHandler(applicationContext, "Event", config.WebEvents)

	return func(responseWriter http.ResponseWriter, request *http.Request) {
		// Validate HTTP method
		if !baseHandler.ValidateMethod(responseWriter, request, http.MethodDelete, http.MethodPost) {
			return
		}

		// Require event ID parameter
		params, valid := baseHandler.RequireParams(responseWriter, request, config.EventIDParam)
		if !valid {
			return
		}
		eventID := params[config.EventIDParam]

		// Get authenticated user data (authentication is guaranteed by middleware)
		sessionData, _ := baseHandler.RequireAuthentication(responseWriter, request)

		// Find the user in the database
		var currentUser models.User
		if findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); findUserError != nil {
			baseHandler.HandleError(responseWriter, findUserError, utils.DatabaseError, "User not found in database")
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
		if !baseHandler.VerifyResourceOwnership(responseWriter, eventID, findEventOwnerID, currentUser.ID) {
			return
		}

		// Load the event from the database
		var eventRecord models.Event
		if findError := eventRecord.FindByID(applicationContext.Database, eventID); findError != nil {
			baseHandler.HandleError(responseWriter, findError, utils.NotFoundError, "Event not found")
			return
		}

		// First, delete all RSVPs associated with this event
		if deleteRSVPsError := applicationContext.Database.Where("event_id = ?", eventID).Delete(&models.RSVP{}).Error; deleteRSVPsError != nil {
			baseHandler.HandleError(responseWriter, deleteRSVPsError, utils.DatabaseError, "Failed to delete associated RSVPs")
			return
		}

		// Then delete the event
		if deletionError := applicationContext.Database.Delete(&eventRecord).Error; deletionError != nil {
			baseHandler.HandleError(responseWriter, deletionError, utils.DatabaseError, "Failed to delete event")
			return
		}

		// Redirect back to the events list after deletion
		baseHandler.RedirectToList(responseWriter, request)
	}
}
