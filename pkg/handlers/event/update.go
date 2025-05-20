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

// UpdateEventHandler updates basic event data and optionally associates or
// disassociates a venue. The form always includes the venue_id field; an empty
// value explicitly removes any existing association.
func UpdateEventHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHttpHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameEvent, config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHttpHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodPut, http.MethodPatch) {
			return
		}
		currentUser := httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User)

		parseFormError := httpRequest.ParseForm()
		if parseFormError != nil {
			baseHttpHandler.HandleError(httpResponseWriter, parseFormError, utils.ValidationError, config.ErrMsgInvalidFormData)
			return
		}

		activeTransaction := applicationContext.Database.Begin()
		if activeTransaction.Error != nil {
			baseHttpHandler.HandleError(httpResponseWriter, activeTransaction.Error, utils.DatabaseError, config.ErrMsgTransactionStart)
			return
		}

		targetEventIdentifier := httpRequest.FormValue(config.EventIDParam)

		var existingEventRecord models.Event
		// FindByIDAndOwner preloads Venue, so existingEventRecord.Venue will be populated if associated.
		findEventError := existingEventRecord.FindByIDAndOwner(activeTransaction, targetEventIdentifier, currentUser.ID)
		if findEventError != nil {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, findEventError, utils.NotFoundError, config.ErrMsgEventNotFound)
			return
		}

		existingEventRecord.Title = httpRequest.FormValue(config.TitleParam)
		existingEventRecord.Description = httpRequest.FormValue(config.DescriptionParam)

		parsedDurationHours, durationValidationError := utils.ValidateAndParseEventDuration(httpRequest.FormValue(config.DurationParam))
		if durationValidationError != nil {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, durationValidationError, utils.ValidationError, durationValidationError.Error())
			return
		}

		parsedStartTime, startTimeParseError := time.Parse(config.TimeLayoutHTMLForm, httpRequest.FormValue(config.StartTimeParam))
		if startTimeParseError != nil {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, startTimeParseError, utils.ValidationError, config.ErrMsgInvalidStartTimeFormat)
			return
		}

		existingEventRecord.StartTime = parsedStartTime
		existingEventRecord.EndTime = parsedStartTime.Add(time.Duration(parsedDurationHours) * time.Hour)

		if _, venueParameterPresent := httpRequest.Form[config.VenueIDParam]; venueParameterPresent {
			selectedVenueIdentifierString := httpRequest.FormValue(config.VenueIDParam)

			if selectedVenueIdentifierString == "" {
				existingEventRecord.VenueID = nil
			} else {
				if existingEventRecord.VenueID == nil || *existingEventRecord.VenueID != selectedVenueIdentifierString {
					var verifiedVenueRecord models.Venue
					findVenueError := verifiedVenueRecord.FindByIDAndOwner(activeTransaction, selectedVenueIdentifierString, currentUser.ID)
					if findVenueError != nil {
						activeTransaction.Rollback()
						baseHttpHandler.HandleError(httpResponseWriter, findVenueError, utils.ForbiddenError, config.ErrMsgVenuePermission)
						return
					}
					existingEventRecord.VenueID = &selectedVenueIdentifierString
				}
			}
		}

		updateEventError := existingEventRecord.Update(activeTransaction)
		if updateEventError != nil {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, updateEventError, utils.DatabaseError, config.ErrMsgEventUpdate)
			return
		}

		commitTransactionError := activeTransaction.Commit().Error
		if commitTransactionError != nil {
			baseHttpHandler.HandleError(httpResponseWriter, commitTransactionError, utils.DatabaseError, config.ErrMsgEventUpdate)
			return
		}

		baseHttpHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
