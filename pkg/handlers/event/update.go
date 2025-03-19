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
	baseHandler := handlers.NewBaseHandler(applicationContext, "Event", config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateMethod(httpResponseWriter, httpRequest, http.MethodPost, http.MethodPut, http.MethodPatch) {
			return
		}

		parameterMap, validParams := baseHandler.RequireParams(httpResponseWriter, httpRequest, config.EventIDParam)
		if !validParams {
			return
		}
		targetEventID := parameterMap[config.EventIDParam]

		sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

		var currentUser models.User
		findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail)
		if findUserError != nil {
			baseHandler.HandleError(httpResponseWriter, findUserError, utils.DatabaseError, "User not found in database")
			return
		}

		newTitle := httpRequest.FormValue(config.TitleParam)
		newDescription := httpRequest.FormValue(config.DescriptionParam)
		newStartTimeString := httpRequest.FormValue(config.StartTimeParam)
		newDurationString := httpRequest.FormValue(config.DurationParam)

		titleError := utils.ValidateEventTitle(newTitle)
		if titleError != nil {
			baseHandler.HandleError(httpResponseWriter, titleError, utils.ValidationError, titleError.Error())
			return
		}

		durationError := utils.ValidateEventDuration(newDurationString)
		if durationError != nil {
			baseHandler.HandleError(httpResponseWriter, durationError, utils.ValidationError, durationError.Error())
			return
		}

		const timeLayout = "2006-01-02T15:04"
		parsedNewStartTime, parseError := time.Parse(timeLayout, newStartTimeString)
		if parseError != nil {
			baseHandler.HandleError(httpResponseWriter, parseError, utils.ValidationError, "Invalid start time format")
			return
		}

		startTimeValidationError := utils.ValidateEventStartTime(parsedNewStartTime)
		if startTimeValidationError != nil {
			baseHandler.HandleError(httpResponseWriter, startTimeValidationError, utils.ValidationError, startTimeValidationError.Error())
			return
		}

		parsedDuration, durationParseError := strconv.Atoi(newDurationString)
		if durationParseError != nil {
			baseHandler.HandleError(httpResponseWriter, durationParseError, utils.ValidationError, "Invalid duration value")
			return
		}
		calculatedNewEndTime := parsedNewStartTime.Add(time.Duration(parsedDuration) * time.Hour)

		findEventOwnerID := func(eventID string) (string, error) {
			var eventInstance models.Event
			loadErr := eventInstance.FindByID(applicationContext.Database, eventID)
			if loadErr != nil {
				return "", loadErr
			}
			return eventInstance.UserID, nil
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, targetEventID, findEventOwnerID, currentUser.ID) {
			return
		}

		var eventRecord models.Event
		findEventError := eventRecord.FindByID(applicationContext.Database, targetEventID)
		if findEventError != nil {
			baseHandler.HandleError(httpResponseWriter, findEventError, utils.NotFoundError, "Event not found")
			return
		}

		eventRecord.Title = newTitle
		eventRecord.Description = newDescription
		eventRecord.StartTime = parsedNewStartTime
		eventRecord.EndTime = calculatedNewEndTime

		saveError := eventRecord.Save(applicationContext.Database)
		if saveError != nil {
			baseHandler.HandleError(httpResponseWriter, saveError, utils.DatabaseError, "Failed to update event")
			return
		}

		baseHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
