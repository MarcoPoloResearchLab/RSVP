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
		durationHoursStr := request.FormValue("duration")

		// Validate title
		if titleValidationError := utils.ValidateEventTitle(eventTitle); titleValidationError != nil {
			baseHandler.HandleError(responseWriter, titleValidationError, utils.ValidationError, titleValidationError.Error())
			return
		}

		// Validate duration
		if durationValidationError := utils.ValidateEventDuration(durationHoursStr); durationValidationError != nil {
			baseHandler.HandleError(responseWriter, durationValidationError, utils.ValidationError, durationValidationError.Error())
			return
		}

		// Parse and validate start time (expected format: "2006-01-02T15:04")
		const timeLayout = "2006-01-02T15:04"
		eventStartTime, timeParseError := time.Parse(timeLayout, eventStartTimeStr)
		if timeParseError != nil {
			baseHandler.HandleError(responseWriter, timeParseError, utils.ValidationError, "Invalid start time format")
			return
		}

		// Validate that start time is in the future
		if startTimeValidationError := utils.ValidateEventStartTime(eventStartTime); startTimeValidationError != nil {
			baseHandler.HandleError(responseWriter, startTimeValidationError, utils.ValidationError, startTimeValidationError.Error())
			return
		}

		// Parse duration (in hours)
		durationHours, durationParseError := strconv.Atoi(durationHoursStr)
		if durationParseError != nil {
			baseHandler.HandleError(responseWriter, durationParseError, utils.ValidationError, "Invalid duration value")
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

		if eventCreationError := newEvent.Create(applicationContext.Database); eventCreationError != nil {
			baseHandler.HandleError(responseWriter, eventCreationError, utils.DatabaseError, "Failed to create event")
			return
		}

		// Redirect back to the events list
		baseHandler.RedirectToList(responseWriter, request)
	}
}
