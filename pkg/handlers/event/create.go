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
	baseHandler := handlers.NewBaseHandler(applicationContext, "Event", config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateMethod(httpResponseWriter, httpRequest, http.MethodPost) {
			return
		}

		sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

		var currentUser models.User
		userFindError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail)
		if userFindError != nil {
			newlyUpsertedUser, upsertError := models.UpsertUser(
				applicationContext.Database,
				sessionData.UserEmail,
				sessionData.UserName,
				sessionData.UserPicture,
			)
			if upsertError != nil {
				baseHandler.HandleError(httpResponseWriter, upsertError, utils.DatabaseError, "Failed to upsert user")
				return
			}
			currentUser = *newlyUpsertedUser
		}

		eventTitle := httpRequest.FormValue(config.TitleParam)
		eventDescription := httpRequest.FormValue(config.DescriptionParam)
		eventStartTimeString := httpRequest.FormValue(config.StartTimeParam)
		durationHoursString := httpRequest.FormValue(config.DurationParam)

		titleValidationError := utils.ValidateEventTitle(eventTitle)
		if titleValidationError != nil {
			baseHandler.HandleError(httpResponseWriter, titleValidationError, utils.ValidationError, titleValidationError.Error())
			return
		}

		durationValidationError := utils.ValidateEventDuration(durationHoursString)
		if durationValidationError != nil {
			baseHandler.HandleError(httpResponseWriter, durationValidationError, utils.ValidationError, durationValidationError.Error())
			return
		}

		const timeLayout = "2006-01-02T15:04"
		parsedStartTime, parseStartTimeError := time.Parse(timeLayout, eventStartTimeString)
		if parseStartTimeError != nil {
			baseHandler.HandleError(httpResponseWriter, parseStartTimeError, utils.ValidationError, "Invalid start time format")
			return
		}

		startTimeValidationError := utils.ValidateEventStartTime(parsedStartTime)
		if startTimeValidationError != nil {
			baseHandler.HandleError(httpResponseWriter, startTimeValidationError, utils.ValidationError, startTimeValidationError.Error())
			return
		}

		parsedDurationHours, parseDurationError := strconv.Atoi(durationHoursString)
		if parseDurationError != nil {
			baseHandler.HandleError(httpResponseWriter, parseDurationError, utils.ValidationError, "Invalid duration value")
			return
		}

		calculatedEndTime := parsedStartTime.Add(time.Duration(parsedDurationHours) * time.Hour)

		newEvent := models.Event{
			Title:       eventTitle,
			Description: eventDescription,
			StartTime:   parsedStartTime,
			EndTime:     calculatedEndTime,
			UserID:      currentUser.ID,
		}
		eventCreationError := newEvent.Create(applicationContext.Database)
		if eventCreationError != nil {
			baseHandler.HandleError(httpResponseWriter, eventCreationError, utils.DatabaseError, "Failed to create event")
			return
		}

		baseHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
