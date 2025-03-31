package event

import (
	"errors"
	"net/http"
	"time"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/middleware"
	"github.com/temirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

// UpdateHandler handles PUT/PATCH requests (or POST with _method override) to update an existing event.
func UpdateHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameEvent, config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodPut, http.MethodPatch) {
			return
		}

		currentUser := httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User)

		params, paramsOk := baseHandler.RequireParams(httpResponseWriter, httpRequest, config.EventIDParam)
		if !paramsOk {
			return
		}
		targetEventID := params[config.EventIDParam]

		if err := httpRequest.ParseForm(); err != nil {
			baseHandler.HandleError(httpResponseWriter, err, utils.ValidationError, utils.ErrMsgInvalidFormData)
			return
		}
		newTitle := httpRequest.FormValue(config.TitleParam)
		newDescription := httpRequest.FormValue(config.DescriptionParam)
		newStartTimeString := httpRequest.FormValue(config.StartTimeParam)
		newDurationString := httpRequest.FormValue(config.DurationParam)

		if validationError := utils.ValidateEventTitle(newTitle); validationError != nil {
			baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
			return
		}
		parsedNewDurationHours, validationError := utils.ValidateAndParseEventDuration(newDurationString)
		if validationError != nil {
			baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
			return
		}
		parsedNewStartTime, timeParseError := time.Parse(config.TimeLayoutHTMLForm, newStartTimeString)
		if timeParseError != nil {
			baseHandler.HandleError(httpResponseWriter, timeParseError, utils.ValidationError, utils.ErrMsgInvalidStartTimeFormat)
			return
		}
		if validationError := utils.ValidateEventStartTime(parsedNewStartTime); validationError != nil {
			baseHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
			return
		}

		var eventRecord models.Event
		findError := applicationContext.Database.First(&eventRecord, "id = ?", targetEventID).Error
		if findError != nil {
			if errors.Is(findError, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findError, utils.NotFoundError, "Event not found.")
			} else {
				baseHandler.HandleError(httpResponseWriter, findError, utils.DatabaseError, "Error retrieving event for update.")
			}
			return
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, httpRequest, eventRecord.UserID, currentUser.ID) {
			return
		}

		calculatedNewEndTime := parsedNewStartTime.Add(time.Duration(parsedNewDurationHours) * time.Hour)
		eventRecord.Title = newTitle
		eventRecord.Description = newDescription
		eventRecord.StartTime = parsedNewStartTime
		eventRecord.EndTime = calculatedNewEndTime

		if saveError := eventRecord.Save(applicationContext.Database); saveError != nil {
			baseHandler.HandleError(httpResponseWriter, saveError, utils.DatabaseError, "Failed to update the event.")
			return
		}

		baseHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
