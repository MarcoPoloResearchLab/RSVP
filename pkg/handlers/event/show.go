package event

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// ShowHandler handles GET requests to view a specific event by ID.
func ShowHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	// Create a base handler for events
	baseHandler := handlers.NewBaseHandler(applicationContext, "Event", config.WebEvents)

	return func(responseWriter http.ResponseWriter, request *http.Request) {
		// Validate HTTP method
		if !baseHandler.ValidateMethod(responseWriter, request, http.MethodGet) {
			return
		}

		// Require event ID parameter
		params, valid := baseHandler.RequireParams(responseWriter, request, "id")
		if !valid {
			return
		}
		eventID := params["id"]

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
		
		// Load the event with its RSVPs
		var eventRecord models.Event
		if loadEventError := eventRecord.LoadWithRSVPs(applicationContext.Database, eventID); loadEventError != nil {
			baseHandler.HandleError(responseWriter, loadEventError, utils.NotFoundError, "Event not found")
			return
		}

		// Business logic: if an RSVP's Response is empty, set it to "Pending"
		for index, currentRSVP := range eventRecord.RSVPs {
			if currentRSVP.Response == "" {
				eventRecord.RSVPs[index].Response = "Pending"
			}
		}

		// Prepare template data
		templateData := struct {
			UserPicture string
			UserName    string
			Event       models.Event
		}{
			UserPicture: sessionData.UserPicture,
			UserName:    sessionData.UserName,
			Event:       eventRecord,
		}

		// Render the template
		baseHandler.RenderTemplate(responseWriter, "event_detail.html", templateData)
	}
}
