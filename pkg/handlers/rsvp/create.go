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

	return func(responseWriter http.ResponseWriter, request *http.Request) {
		// Validate HTTP method
		if !baseHandler.ValidateMethod(responseWriter, request, http.MethodPost) {
			return
		}

		// Get event ID parameter
		eventID := baseHandler.GetParam(request, config.EventIDParam)
		if eventID == "" {
			http.Error(responseWriter, "Event ID is required", http.StatusBadRequest)
			return
		}

		// Get authenticated user data (authentication is guaranteed by middleware)
		sessionData, _ := baseHandler.RequireAuthentication(responseWriter, request)

		// Find the current user
		var currentUser models.User
		if findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); findUserError != nil {
			baseHandler.HandleError(responseWriter, findUserError, utils.DatabaseError, "User not found in database")
			return
		}

		// Define a function to find the owner ID of an event
		findEventOwnerID := func(eventID string) (string, error) {
			var eventRecord models.Event
			if eventFindError := eventRecord.FindByID(applicationContext.Database, eventID); eventFindError != nil {
				return "", eventFindError
			}
			return eventRecord.UserID, nil
		}

		// Verify that the current user owns the event
		if !baseHandler.VerifyResourceOwnership(responseWriter, eventID, findEventOwnerID, currentUser.ID) {
			return
		}

		// Get RSVP name parameter - first try from the form directly since this is POST
		rsvpName := request.FormValue(config.NameParam)
		if rsvpName == "" {
			// Fall back to GetParam which checks both URL and form params
			rsvpName = baseHandler.GetParam(request, config.NameParam)
		}

		// Log debug info
		baseHandler.Logger().Printf("Creating RSVP with name: %s for event: %s", rsvpName, eventID)

		// Validate RSVP name
		if rsvpName == "" {
			baseHandler.HandleError(responseWriter, nil, utils.ValidationError, "RSVP name is required")
			return
		}

		if nameValidationError := utils.ValidateRSVPName(rsvpName); nameValidationError != nil {
			baseHandler.HandleError(responseWriter, nameValidationError, utils.ValidationError, nameValidationError.Error())
			return
		}

		// Create the new RSVP
		newRSVP := models.RSVP{
			Name:    rsvpName,
			EventID: eventID,
		}
		if rsvpCreateError := newRSVP.Create(applicationContext.Database); rsvpCreateError != nil {
			baseHandler.HandleError(responseWriter, rsvpCreateError, utils.DatabaseError, "Failed to create RSVP")
			return
		}

		// Check if the request came from the admin UI (form has event_id)
		formEventID := baseHandler.GetParam(request, config.EventIDParam)
		if formEventID != "" {
			// Redirect back to the RSVP list for this event
			baseHandler.RedirectWithParams(responseWriter, request, map[string]string{
				config.EventIDParam: eventID,
			})
		} else {
			// Redirect to the RSVP detail route using the RSVP ID as the code
			baseHandler.RedirectWithParams(responseWriter, request, map[string]string{
				config.RSVPIDParam: newRSVP.ID,
			})
		}
	}
}
