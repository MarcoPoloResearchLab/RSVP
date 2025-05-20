package event

import (
	"net/http"
	"strconv"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/middleware"
	"github.com/temirov/RSVP/pkg/utils"
)

// ListEventsHandler returns the Events main page (list + optional edit panel).
func ListEventsHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHttpHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameEvent, config.WebEvents)
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		currentUser := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		requestedEventIDForEdit := request.URL.Query().Get(config.EventIDParam)
		var selectedEventForEdit *EnhancedEventData

		userReusedVenues, err := models.FindVenuesByOwner(applicationContext.Database, currentUser.ID)
		if err != nil {
			baseHttpHandler.ApplicationContext.Logger.Printf(
				"ERROR: Failed to retrieve venues owned by user %s: %v",
				currentUser.ID, err,
			)
			userReusedVenues = []models.Venue{}
		}

		if requestedEventIDForEdit != "" {
			var eventToEdit models.Event
			err = eventToEdit.FindByIDAndOwner(applicationContext.Database, requestedEventIDForEdit, currentUser.ID)
			if err == nil {
				selectedVenueIdentifierString := ""
				if eventToEdit.VenueID != nil {
					selectedVenueIdentifierString = *eventToEdit.VenueID
				}
				selectedEventForEdit = &EnhancedEventData{
					Event:                     eventToEdit,
					CalculatedDurationInHours: float64(eventToEdit.DurationHours()),
					SelectedVenueID:           selectedVenueIdentifierString,
				}
			} else {
				baseHttpHandler.ApplicationContext.Logger.Printf(
					"WARN: Failed to find event %s for edit or user %s does not own it: %v",
					requestedEventIDForEdit, currentUser.ID, err,
				)
			}
		}

		eventsOwnedByUser, err := models.FindEventsByUserID(applicationContext.Database, currentUser.ID, true, true)
		if err != nil {
			baseHttpHandler.HandleError(responseWriter, err, utils.DatabaseError, "Failed to retrieve events list.")
			return
		}

		eventStatistics := make([]StatisticsData, len(eventsOwnedByUser))
		for index, eventDetails := range eventsOwnedByUser {
			totalRSVPCount := len(eventDetails.RSVPs)
			answeredRSVPCount := 0
			for _, rsvpDetails := range eventDetails.RSVPs {
				if rsvpDetails.Response != "" && rsvpDetails.Response != config.RSVPResponsePending {
					answeredRSVPCount++
				}
			}
			venueName := "N/A"
			if eventDetails.Venue != nil {
				venueName = eventDetails.Venue.Name
			}
			eventStatistics[index] = StatisticsData{
				ID:                eventDetails.ID,
				Title:             eventDetails.Title,
				StartTime:         eventDetails.StartTime,
				EndTime:           eventDetails.EndTime,
				VenueName:         venueName,
				RSVPCount:         totalRSVPCount,
				RSVPAnsweredCount: answeredRSVPCount,
			}
		}

		var formattedStartTime string
		var currentDuration string
		if selectedEventForEdit != nil {
			formattedStartTime = selectedEventForEdit.Event.StartTime.Format(config.TimeLayoutHTMLForm)
			currentDuration = strconv.Itoa(selectedEventForEdit.Event.DurationHours())
		}

		listViewData := ListViewData{
			EventList:                 eventStatistics,
			SelectedItemForEdit:       selectedEventForEdit,
			UserReusedVenues:          userReusedVenues,
			URLForEventActions:        config.WebEvents,
			URLForEventEdit:           config.WebEvents,
			URLForRSVPListBase:        config.WebRSVPs,
			URLForVenues:              config.WebVenues,
			ParamNameEventID:          config.EventIDParam,
			ParamNameVenueID:          config.VenueIDParam,
			ParamNameTitle:            config.TitleParam,
			ParamNameDescription:      config.DescriptionParam,
			ParamNameStartTime:        config.StartTimeParam,
			ParamNameDuration:         config.DurationParam,
			ParamNameMethodOverride:   config.MethodOverrideParam,
			ParamNameVenueName:        config.VenueNameParam,
			ParamNameVenueAddress:     config.VenueAddressParam,
			ParamNameVenueCapacity:    config.VenueCapacityParam,
			ParamNameVenuePhone:       config.VenuePhoneParam,
			ParamNameVenueEmail:       config.VenueEmailParam,
			ParamNameVenueWebsite:     config.VenueWebsiteParam,
			ParamNameVenueDescription: config.VenueDescriptionParam,
			VenueSelectCreateNewValue: config.VenueSelectCreateNewValue,
			ButtonCancelEdit:          config.ButtonCancelEdit,
			LabelEventTitle:           config.LabelEventTitle,
			LabelEventDescription:     config.LabelEventDescription,
			LabelStartTime:            config.LabelStartTime,
			LabelDuration:             config.LabelDuration,
			LabelVenueDetails:         config.LabelVenueDetails,
			ButtonDeleteVenue:         config.ButtonDeleteVenue,
			ButtonAddVenue:            config.ButtonAddVenue,
			ButtonCreateNewVenue:      config.ButtonCreateVenue,
			LabelAddVenue:             config.LabelAddVenue,
			LabelSelectVenue:          config.LabelSelectVenue,
			OptionNoVenue:             config.OptionNoVenue,
			OptionCreateNewVenue:      config.OptionCreateNewVenue,
			LabelVenueFormTitle:       config.LabelVenueFormTitle,
			LabelVenueName:            config.LabelVenueName,
			LabelVenueAddress:         config.LabelVenueAddress,
			LabelVenueDescription:     config.LabelVenueDescription,
			LabelVenueCapacity:        config.LabelVenueCapacity,
			LabelVenuePhone:           config.LabelVenuePhone,
			LabelVenueEmail:           config.LabelVenueEmail,
			LabelVenueWebsite:         config.LabelVenueWebsite,
			ButtonUpdateEvent:         config.ButtonUpdateEvent,
			ButtonDeleteEvent:         config.ButtonDeleteEvent,
			EventsManagerLabel:        config.ResourceLabelEventManager,
			FormattedStartTime:        formattedStartTime,
			CurrentDuration:           currentDuration,
		}

		baseHttpHandler.RenderView(responseWriter, request, config.TemplateEvents, listViewData)
	}
}
