package event

import (
	"net/http"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
	"github.com/tyemirov/RSVP/pkg/utils"
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

		var sourceLinkCount int64
		if countError := activeTransaction.Model(&models.ExternalEventLink{}).Where("event_id = ?", existingEventRecord.ID).Count(&sourceLinkCount).Error; countError != nil {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, countError, utils.DatabaseError, config.ErrMsgEventUpdate)
			return
		}
		if sourceLinkCount != 0 {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, services.ErrSourceOwnedMarker, utils.ConflictError, services.ErrSourceOwnedMarker.Error())
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

		timezone, timezoneError := models.NewTimezone(httpRequest.FormValue(config.TimezoneParam))
		if timezoneError != nil {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, timezoneError, utils.ValidationError, timezoneError.Error())
			return
		}

		parsedStartTime, startTimeParseError := models.ParseLocalTime(httpRequest.FormValue(config.StartTimeParam), config.TimeLayoutHTMLForm, timezone)
		if startTimeParseError != nil {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, startTimeParseError, utils.ValidationError, config.ErrMsgInvalidStartTimeFormat)
			return
		}

		parsedEndTime := parsedStartTime.Add(time.Duration(parsedDurationHours) * time.Hour)
		if timeValidationError := existingEventRecord.SetIntervalTime(parsedStartTime, parsedEndTime, timezone); timeValidationError != nil {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, timeValidationError, utils.ValidationError, timeValidationError.Error())
			return
		}

		var eventLane models.Lane
		if laneFindError := activeTransaction.First(&eventLane, "id = ?", existingEventRecord.LaneID).Error; laneFindError != nil {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, laneFindError, utils.DatabaseError, config.ErrMsgEventUpdate)
			return
		}
		if parsedStartTime.Before(eventLane.StartsAt) {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, models.ErrMarkerOutsideLane, utils.ValidationError, models.ErrMarkerOutsideLane.Error())
			return
		}
		if eventLane.EndsAt != nil && parsedEndTime.After(*eventLane.EndsAt) {
			eventLane.EndsAt = &parsedEndTime
			if laneUpdateError := activeTransaction.Save(&eventLane).Error; laneUpdateError != nil {
				activeTransaction.Rollback()
				baseHttpHandler.HandleError(httpResponseWriter, laneUpdateError, utils.DatabaseError, config.ErrMsgEventUpdate)
				return
			}
		}

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
			if validationError := isModelValidationError(updateEventError); validationError != nil {
				baseHttpHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
			} else {
				baseHttpHandler.HandleError(httpResponseWriter, updateEventError, utils.DatabaseError, config.ErrMsgEventUpdate)
			}
			return
		}
		if laneBoundsError := models.RecalculateFiniteLaneEnd(activeTransaction, existingEventRecord.LaneID); laneBoundsError != nil {
			activeTransaction.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, laneBoundsError, utils.DatabaseError, config.ErrMsgEventUpdate)
			return
		}
		if existingEventRecord.RelationType == models.EventRelationIndependent {
			eventLane.Title = existingEventRecord.Title
			if laneTitleError := activeTransaction.Model(&eventLane).Update("title", eventLane.Title).Error; laneTitleError != nil {
				activeTransaction.Rollback()
				baseHttpHandler.HandleError(httpResponseWriter, laneTitleError, utils.DatabaseError, config.ErrMsgEventUpdate)
				return
			}
		}

		commitTransactionError := activeTransaction.Commit().Error
		if commitTransactionError != nil {
			baseHttpHandler.HandleError(httpResponseWriter, commitTransactionError, utils.DatabaseError, config.ErrMsgEventUpdate)
			return
		}

		baseHttpHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
