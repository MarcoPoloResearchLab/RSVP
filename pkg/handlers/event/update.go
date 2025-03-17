package event

import (
	"net/http"
	"strconv"
	"time"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// UpdateHandler handles POST requests to update an existing event.
func UpdateHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	// Create a base handler for events
	baseHandler := handlers.NewBaseHandler(applicationContext, "Event", config.WebEvents)

	return func(responseWriter http.ResponseWriter, request *http.Request) {
		// Validate HTTP method
		if !baseHandler.ValidateMethod(responseWriter, request, http.MethodPost, http.MethodPut, http.MethodPatch) {
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

		// Retrieve new event details from the form
		newTitle := request.FormValue("title")
		newDescription := request.FormValue("description")
		newStartTimeStr := request.FormValue("start_time")
		newDurationStr := request.FormValue("duration")
		
		// Validate title
		if titleError := utils.ValidateEventTitle(newTitle); titleError != nil {
			baseHandler.HandleError(responseWriter, titleError, utils.ValidationError, titleError.Error())
			return
		}

		// Validate duration
		if durationError := utils.ValidateEventDuration(newDurationStr); durationError != nil {
			baseHandler.HandleError(responseWriter, durationError, utils.ValidationError, durationError.Error())
			return
		}

		// Parse and validate start time (expected format: "2006-01-02T15:04")
		const timeLayout = "2006-01-02T15:04"
		newStartTime, parseError := time.Parse(timeLayout, newStartTimeStr)
		if parseError != nil {
			baseHandler.HandleError(responseWriter, parseError, utils.ValidationError, "Invalid start time format")
			return
		}
		
		// Validate that start time is in the future
		if startTimeValidationError := utils.ValidateEventStartTime(newStartTime); startTimeValidationError != nil {
			baseHandler.HandleError(responseWriter, startTimeValidationError, utils.ValidationError, startTimeValidationError.Error())
			return
		}

		// Parse duration (in hours)
		newDuration, durationError := strconv.Atoi(newDurationStr)
		if durationError != nil {
			baseHandler.HandleError(responseWriter, durationError, utils.ValidationError, "Invalid duration value")
			return
		}
		newEndTime := newStartTime.Add(time.Duration(newDuration) * time.Hour)

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
		
		// Load the existing event from the database
		var eventRecord models.Event
		if findError := eventRecord.FindByID(applicationContext.Database, eventID); findError != nil {
			baseHandler.HandleError(responseWriter, findError, utils.NotFoundError, "Event not found")
			return
		}

		// Update event fields
		eventRecord.Title = newTitle
		eventRecord.Description = newDescription
		eventRecord.StartTime = newStartTime
		eventRecord.EndTime = newEndTime

		// Save the updated event
		if saveError := eventRecord.Save(applicationContext.Database); saveError != nil {
			baseHandler.HandleError(responseWriter, saveError, utils.DatabaseError, "Failed to update event")
			return
		}

		// Redirect back to the events list after updating
		baseHandler.RedirectToList(responseWriter, request)
	}
}
