package event

import (
	"net/http"
	"strconv"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/utils"
)

// ListEventsHandler returns the Events main page (list + optional edit panel).
func ListEventsHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHttpHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameEvent, config.WebEvents)

	return func(w http.ResponseWriter, r *http.Request) {
		currentUser := r.Context().Value(middleware.ContextKeyUser).(*models.User)

		requestedEventIDForEdit := r.URL.Query().Get(config.EventIDParam)
		var selectedEventForEdit *EnhancedEventData
		calendarList, calendarError := models.FindCalendarsByOwner(applicationContext.Database, currentUser.ID)
		if calendarError != nil {
			baseHttpHandler.HandleError(w, calendarError, utils.DatabaseError, "Failed to retrieve calendars.")
			return
		}

		/* load user venues (for selector) */
		userReusedVenues, err := models.FindVenuesByOwner(applicationContext.Database, currentUser.ID)
		if err != nil {
			baseHttpHandler.ApplicationContext.Logger.Printf(
				"ERROR: Failed to retrieve venues owned by user %s: %v",
				currentUser.ID, err,
			)
			userReusedVenues = []models.Venue{}
		}

		/* if an event is selected for editing – load it */
		if requestedEventIDForEdit != "" {
			var eventToEdit models.Event
			err = eventToEdit.FindByIDAndOwner(applicationContext.Database, requestedEventIDForEdit, currentUser.ID)
			if err == nil {
				durationHours, durationError := eventToEdit.IntervalDurationHours()
				if durationError != nil {
					baseHttpHandler.HandleError(w, durationError, utils.ServerError, "The event time cannot be edited.")
					return
				}
				venueID := ""
				if eventToEdit.VenueID != nil {
					venueID = *eventToEdit.VenueID
				}
				selectedEventForEdit = &EnhancedEventData{
					Event:                     eventToEdit,
					CalculatedDurationInHours: float64(durationHours),
					SelectedVenueID:           venueID,
				}
			} else {
				baseHttpHandler.ApplicationContext.Logger.Printf(
					"WARN: Failed to find event %s for edit or user %s does not own it: %v",
					requestedEventIDForEdit, currentUser.ID, err,
				)
			}
		}

		/* gather statistics for list */
		eventsOwnedByUser, err := models.FindEventsByUserID(applicationContext.Database, currentUser.ID, true, true)
		if err != nil {
			baseHttpHandler.HandleError(w, err, utils.DatabaseError, "Failed to retrieve events list.")
			return
		}

		eventStatistics := make([]StatisticsData, len(eventsOwnedByUser))
		anchorEvents := make([]AnchorEventData, 0, len(eventsOwnedByUser))
		for i, ev := range eventsOwnedByUser {
			localStart, localEnd, boundsError := ev.LocalMarkerBounds()
			if boundsError != nil {
				baseHttpHandler.HandleError(w, boundsError, utils.ServerError, "Failed to read an event time.")
				return
			}
			total := len(ev.RSVPs)
			answered := 0
			for _, rsvp := range ev.RSVPs {
				if rsvp.Response != "" && rsvp.Response != config.RSVPResponsePending {
					answered++
				}
			}
			venueName := "N/A"
			if ev.Venue != nil {
				venueName = ev.Venue.Name
			}

			eventStatistics[i] = StatisticsData{
				ID:                ev.ID,
				Title:             ev.Title,
				StartTime:         localStart,
				EndTime:           localEnd,
				VenueName:         venueName,
				RSVPCount:         total,
				RSVPAnsweredCount: answered,
			}
			if ev.RelationType != models.EventRelationDependent {
				anchorEvents = append(anchorEvents, AnchorEventData{ID: ev.ID, Title: ev.Title, CalendarID: ev.Lane.CalendarID})
			}
		}

		var formattedStartTime, currentDuration string
		if selectedEventForEdit != nil {
			localStart, _, boundsError := selectedEventForEdit.Event.LocalMarkerBounds()
			if boundsError != nil {
				baseHttpHandler.HandleError(w, boundsError, utils.ServerError, "Failed to read the event time.")
				return
			}
			durationHours, durationError := selectedEventForEdit.Event.IntervalDurationHours()
			if durationError != nil {
				baseHttpHandler.HandleError(w, durationError, utils.ServerError, "Failed to read the event duration.")
				return
			}
			formattedStartTime = localStart.Format(config.TimeLayoutHTMLForm)
			currentDuration = strconv.Itoa(durationHours)
		}

		listViewData := ListViewData{
			/* navigation */
			AppTitle:           config.AppTitle,
			EventsManagerLabel: config.ResourceLabelEventManager,
			VenueManagerLabel:  config.ResourceLabelVenueManager,
			RSVPManagerLabel:   "RSVPs",

			URLForEventActions: config.WebEvents,
			URLForRSVPListBase: config.WebRSVPs,
			URLForRSVPManager:  config.WebRSVPs,
			URLForVenues:       config.WebVenues,

			/* data */
			EventList:           eventStatistics,
			CalendarList:        calendarList,
			AnchorEventList:     anchorEvents,
			SelectedItemForEdit: selectedEventForEdit,
			UserReusedVenues:    userReusedVenues,

			/* helpers */
			ParamNameEventID:          config.EventIDParam,
			ParamNameAnchorEventID:    config.AnchorEventIDParam,
			ParamNameCalendarID:       config.CalendarIDParam,
			ParamNameVenueID:          config.VenueIDParam,
			ParamNameTitle:            config.TitleParam,
			ParamNameDescription:      config.DescriptionParam,
			ParamNameStartTime:        config.StartTimeParam,
			ParamNameTimezone:         config.TimezoneParam,
			ParamNameDuration:         config.DurationParam,
			ParamNameMethodOverride:   config.MethodOverrideParam,
			ParamNameVenueName:        config.VenueNameParam,
			ParamNameVenueAddress:     config.VenueAddressParam,
			ParamNameVenueDescription: config.VenueDescriptionParam,
			ParamNameVenueCapacity:    config.VenueCapacityParam,
			ParamNameVenuePhone:       config.VenuePhoneParam,
			ParamNameVenueEmail:       config.VenueEmailParam,
			ParamNameVenueWebsite:     config.VenueWebsiteParam,

			/* labels / buttons */
			LabelEventTitle:       config.LabelEventTitle,
			LabelEventDescription: config.LabelEventDescription,
			LabelStartTime:        config.LabelStartTime,
			LabelTimezone:         config.LabelTimezone,
			LabelDuration:         config.LabelDuration,
			LabelSelectVenue:      config.LabelSelectVenue,
			LabelAddVenue:         config.LabelAddVenue,
			LabelVenueDetails:     config.LabelVenueDetails,
			LabelVenueName:        config.LabelVenueName,
			LabelVenueAddress:     config.LabelVenueAddress,
			LabelVenueDescription: config.LabelVenueDescription,
			LabelVenueCapacity:    config.LabelVenueCapacity,
			LabelVenuePhone:       config.LabelVenuePhone,
			LabelVenueEmail:       config.LabelVenueEmail,
			LabelVenueWebsite:     config.LabelVenueWebsite,

			ButtonCancelEdit:     config.ButtonCancelEdit,
			ButtonAddVenue:       config.ButtonAddVenue,
			ButtonCreateNewVenue: config.ButtonCreateVenue,
			ButtonUpdateEvent:    config.ButtonUpdateEvent,
			ButtonDeleteEvent:    config.ButtonDeleteEvent,
			ButtonDeleteVenue:    config.ButtonDeleteVenue,

			OptionNoVenue:             config.OptionNoVenue,
			OptionCreateNewVenue:      config.OptionCreateNewVenue,
			VenueSelectCreateNewValue: config.VenueSelectCreateNewValue,

			FormattedStartTime: formattedStartTime,
			CurrentDuration:    currentDuration,
		}

		baseHttpHandler.RenderView(w, r, config.TemplateEvents, listViewData)
	}
}
