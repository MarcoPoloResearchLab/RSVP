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

// UpdateHandler handles POST/PUT/PATCH requests to update an existing event.
func UpdateHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, "Event", config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodPost, http.MethodPut, http.MethodPatch) {
			return
		}

		params, ok := baseHandler.RequireParams(httpResponseWriter, httpRequest, config.EventIDParam)
		if !ok {
			return
		}
		targetEventID := params[config.EventIDParam]

		sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

		var currentUser models.User
		if err := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.DatabaseError, "User not found in database")
			return
		}

		newTitle := httpRequest.FormValue(config.TitleParam)
		newDescription := httpRequest.FormValue(config.DescriptionParam)
		newStartTimeString := httpRequest.FormValue(config.StartTimeParam)
		newDurationString := httpRequest.FormValue(config.DurationParam)

		if err := utils.ValidateEventTitle(newTitle); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.ValidationError, err.Error())
			return
		}

		if err := utils.ValidateEventDuration(newDurationString); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.ValidationError, err.Error())
			return
		}

		const timeLayout = "2006-01-02T15:04"
		parsedNewStartTime, errParse := time.Parse(timeLayout, newStartTimeString)
		if errParse != nil {
			baseHandler.HandleError(httpResponseWriter, errParse, utils.ValidationError, "Invalid start time format")
			return
		}

		if err := utils.ValidateEventStartTime(parsedNewStartTime); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.ValidationError, err.Error())
			return
		}

		parsedDuration, errConv := strconv.Atoi(newDurationString)
		if errConv != nil {
			baseHandler.HandleError(httpResponseWriter, errConv, utils.ValidationError, "Invalid duration value")
			return
		}
		calculatedNewEndTime := parsedNewStartTime.Add(time.Duration(parsedDuration) * time.Hour)

		findEventOwnerID := func(eventID string) (string, error) {
			var ev models.Event
			if err := ev.FindByID(applicationContext.Database, eventID); err != nil {
				return "", err
			}
			return ev.UserID, nil
		}
		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, targetEventID, findEventOwnerID, currentUser.ID) {
			return
		}

		var eventRecord models.Event
		if err := eventRecord.FindByID(applicationContext.Database, targetEventID); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.NotFoundError, "Event not found")
			return
		}

		eventRecord.Title = newTitle
		eventRecord.Description = newDescription
		eventRecord.StartTime = parsedNewStartTime
		eventRecord.EndTime = calculatedNewEndTime

		if err := eventRecord.Save(applicationContext.Database); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.DatabaseError, "Failed to update event")
			return
		}

		baseHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
