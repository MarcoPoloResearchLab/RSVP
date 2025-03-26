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
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, "Event", config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodPost) {
			return
		}

		sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

		var currentUser models.User
		if err := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); err != nil {
			upsertedUser, errUpsert := models.UpsertUser(applicationContext.Database, sessionData.UserEmail, sessionData.UserName, sessionData.UserPicture)
			if errUpsert != nil {
				baseHandler.HandleError(httpResponseWriter, errUpsert, utils.DatabaseError, "Failed to upsert user")
				return
			}
			currentUser = *upsertedUser
		}

		eventTitle := httpRequest.FormValue(config.TitleParam)
		eventDescription := httpRequest.FormValue(config.DescriptionParam)
		eventStartTimeString := httpRequest.FormValue(config.StartTimeParam)
		durationHoursString := httpRequest.FormValue(config.DurationParam)

		if err := utils.ValidateEventTitle(eventTitle); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.ValidationError, err.Error())
			return
		}

		if err := utils.ValidateEventDuration(durationHoursString); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.ValidationError, err.Error())
			return
		}

		const timeLayout = "2006-01-02T15:04"
		parsedStartTime, errParse := time.Parse(timeLayout, eventStartTimeString)
		if errParse != nil {
			baseHandler.HandleError(httpResponseWriter, errParse, utils.ValidationError, "Invalid start time format")
			return
		}

		if err := utils.ValidateEventStartTime(parsedStartTime); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.ValidationError, err.Error())
			return
		}

		parsedDuration, errConv := strconv.Atoi(durationHoursString)
		if errConv != nil {
			baseHandler.HandleError(httpResponseWriter, errConv, utils.ValidationError, "Invalid duration value")
			return
		}

		calculatedEndTime := parsedStartTime.Add(time.Duration(parsedDuration) * time.Hour)

		newEvent := models.Event{
			Title:       eventTitle,
			Description: eventDescription,
			StartTime:   parsedStartTime,
			EndTime:     calculatedEndTime,
			UserID:      currentUser.ID,
		}
		if err := newEvent.Create(applicationContext.Database); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.DatabaseError, "Failed to create event")
			return
		}

		baseHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
