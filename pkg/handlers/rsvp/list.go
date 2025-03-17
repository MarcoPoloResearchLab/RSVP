package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// ListHandler handles GET /rsvps?event_id={event_id} for listing RSVPs for a specific event.
// It also supports GET /rsvps?event_id={event_id}&rsvp_id={rsvp_id} for editing a specific RSVP.
func ListHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	// Create a base handler for RSVPs
	baseHandler := handlers.NewBaseHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(responseWriter http.ResponseWriter, request *http.Request) {
		// Validate HTTP method
		if !baseHandler.ValidateMethod(responseWriter, request, http.MethodGet) {
			return
		}

		// Get event ID from query parameter
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
		if findEventError := eventRecord.FindByID(applicationContext.Database, eventID); findEventError != nil {
			baseHandler.HandleError(responseWriter, findEventError, utils.NotFoundError, "Event not found")
			return
		}

		// Get RSVPs for this event
		var rsvpRecords []models.RSVP
		if rsvpQueryError := applicationContext.Database.Where("event_id = ?", eventID).Find(&rsvpRecords).Error; rsvpQueryError != nil {
			baseHandler.HandleError(responseWriter, rsvpQueryError, utils.DatabaseError, "Could not retrieve RSVPs")
			return
		}

		// Business logic: if Response is empty, set it to "Pending"
		for index, record := range rsvpRecords {
			if record.Response == "" {
				rsvpRecords[index].Response = "Pending"
			}
		}

		// Check if a specific RSVP is being edited
		rsvpID := baseHandler.GetParam(request, config.RSVPIDParam)
		var selectedRSVP *models.RSVP

		if rsvpID != "" {
			// Find the selected RSVP
			for _, record := range rsvpRecords {
				if record.ID == rsvpID {
					rsvpCopy := record // Create a copy to avoid modifying the slice element
					selectedRSVP = &rsvpCopy
					break
				}
			}

			// If not found in the already loaded records, try to load it directly
			if selectedRSVP == nil {
				var rsvp models.RSVP
				if err := rsvp.FindByCode(applicationContext.Database, rsvpID); err == nil && rsvp.EventID == eventID {
					selectedRSVP = &rsvp
				}
			}
		}

		// Use the session data we already have
		templateData := struct {
			RSVPRecords  []models.RSVP
			SelectedRSVP *models.RSVP
			Event        models.Event
			UserPicture  string
			UserName     string
		}{
			RSVPRecords:  rsvpRecords,
			SelectedRSVP: selectedRSVP,
			Event:        eventRecord,
			UserPicture:  sessionData.UserPicture,
			UserName:     sessionData.UserName,
		}

		// Render the template
		baseHandler.RenderTemplate(responseWriter, config.TemplateRSVPs, templateData)
	}
}
