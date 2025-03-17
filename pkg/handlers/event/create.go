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

// CreateHandler handles POST requests to create a new event.
func CreateHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	// Create a base handler for events
	baseHandler := handlers.NewBaseHandler(applicationContext, "Event", config.WebEvents)

	return func(responseWriter http.ResponseWriter, request *http.Request) {
		// Validate HTTP method
		if !baseHandler.ValidateMethod(responseWriter, request, http.MethodPost) {
			return
		}

		// Get authenticated user data (authentication is guaranteed by middleware)
		sessionData, _ := baseHandler.RequireAuthentication(responseWriter, request)

		// Find or upsert the current user
		var currentUser models.User
		if findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); findUserError != nil {
			newUser, upsertError := models.UpsertUser(
				applicationContext.Database,
				sessionData.UserEmail,
				sessionData.UserName,
				sessionData.UserPicture,
			)
			if upsertError != nil {
				baseHandler.HandleError(responseWriter, upsertError, utils.DatabaseError, "Failed to upsert user")
				return
			}
			currentUser = *newUser
		}

		// Retrieve form values
		eventTitle := request.FormValue("title")
		eventDescription := request.FormValue("description")
		eventStartTimeStr := request.FormValue("start_time")
		durationStr := request.FormValue("duration")

		// Validate title
		if titleError := utils.ValidateEventTitle(eventTitle); titleError != nil {
			baseHandler.HandleError(responseWriter, titleError, utils.ValidationError, titleError.Error())
			return
		}

		// Validate duration
		if durationError := utils.ValidateEventDuration(durationStr); durationError != nil {
			baseHandler.HandleError(responseWriter, durationError, utils.ValidationError, durationError.Error())
			return
		}

		// Parse and validate start time (expected format: "2006-01-02T15:04")
		const timeLayout = "2006-01-02T15:04"
		eventStartTime, startTimeError := time.Parse(timeLayout, eventStartTimeStr)
		if startTimeError != nil {
			baseHandler.HandleError(responseWriter, startTimeError, utils.ValidationError, "Invalid start time format")
			return
		}

		// Validate that start time is in the future
		if startTimeValidationError := utils.ValidateEventStartTime(eventStartTime); startTimeValidationError != nil {
			baseHandler.HandleError(responseWriter, startTimeValidationError, utils.ValidationError, startTimeValidationError.Error())
			return
		}

		// Parse duration (in hours)
		durationHours, parseError := strconv.Atoi(durationStr)
		if parseError != nil {
			baseHandler.HandleError(responseWriter, parseError, utils.ValidationError, "Invalid duration value")
			return
		}

		// Compute end time by adding the duration
		eventEndTime := eventStartTime.Add(time.Duration(durationHours) * time.Hour)

		// Create a new event
		newEvent := models.Event{
			Title:       eventTitle,
			Description: eventDescription,
			StartTime:   eventStartTime,
			EndTime:     eventEndTime,
			UserID:      currentUser.ID,
		}

		if creationError := newEvent.Create(applicationContext.Database); creationError != nil {
			baseHandler.HandleError(responseWriter, creationError, utils.DatabaseError, "Failed to create event")
			return
		}

		// Redirect back to the events list
		baseHandler.RedirectToList(responseWriter, request)
	}
}
