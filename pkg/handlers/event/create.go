// Package event contains HTTP handlers related to Event resource management.
package event

import (
	"net/http"
	"time"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/middleware"
	"github.com/temirov/RSVP/pkg/utils"
)

// CreateHandler handles POST requests to create a new event.
func CreateHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameEvent, config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodPost) {
			return
		}

		// Assume user exists in context due to AddUserToContext middleware
		currentUser := httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User)

		if err := httpRequest.ParseForm(); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.ValidationError, utils.ErrMsgInvalidFormData)
			return
		}
		eventTitle := httpRequest.FormValue(config.TitleParam)
		eventDescription := httpRequest.FormValue(config.DescriptionParam)
		eventStartTimeString := httpRequest.FormValue(config.StartTimeParam)
		durationHoursString := httpRequest.FormValue(config.DurationParam)

		if validationError := utils.ValidateEventTitle(eventTitle); validationError != nil {
			baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
			return
		}

		parsedDurationHours, validationError := utils.ValidateAndParseEventDuration(durationHoursString)
		if validationError != nil {
			baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
			return
		}

		parsedStartTime, timeParseError := time.Parse(config.TimeLayoutHTMLForm, eventStartTimeString)
		if timeParseError != nil {
			baseHandler.HandleError(httpResponseWriter, timeParseError, utils.ValidationError, utils.ErrMsgInvalidStartTimeFormat)
			return
		}
		if validationError := utils.ValidateEventStartTime(parsedStartTime); validationError != nil {
			baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
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

		if createError := newEvent.Create(applicationContext.Database); createError != nil {
			baseHandler.HandleError(httpResponseWriter, createError, utils.DatabaseError, "Failed to create the event.")
			return
		}

		baseHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
