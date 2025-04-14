// Package event provides HTTP handler logic for event-related operations.
package event

import (
	"net/http"
	"strconv"
	"time"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/middleware"
	"github.com/temirov/RSVP/pkg/utils"
	"golang.org/x/exp/slices"
)

// UpdateEventHandler handles PUT/PATCH requests to update an event and its associated venue.
func UpdateEventHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHttpHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameEvent, config.WebEvents)
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if !baseHttpHandler.ValidateHttpMethod(responseWriter, request, http.MethodPut, http.MethodPatch) {
			return
		}
		currentUser := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		if err := request.ParseForm(); err != nil {
			baseHttpHandler.HandleError(responseWriter, err, utils.ValidationError, config.ErrMsgInvalidFormData)
			return
		}
		action := request.FormValue(config.ActionParam)
		tx := applicationContext.Database.Begin()
		if tx.Error != nil {
			baseHttpHandler.HandleError(responseWriter, tx.Error, utils.DatabaseError, config.ErrMsgTransactionStart)
			return
		}
		var eventRecord models.Event
		if err := eventRecord.FindByIDAndOwner(tx, request.FormValue(config.EventIDParam), currentUser.ID); err != nil {
			tx.Rollback()
			baseHttpHandler.HandleError(responseWriter, err, utils.NotFoundError, config.ErrMsgEventNotFound)
			return
		}
		switch action {
		case config.ActionUpdateEventDetails:
			eventRecord.Title = request.FormValue(config.TitleParam)
			eventRecord.Description = request.FormValue(config.DescriptionParam)
			newStartTimeString := request.FormValue(config.StartTimeParam)
			newDurationString := request.FormValue(config.DurationParam)
			parsedNewDuration, durationError := utils.ValidateAndParseEventDuration(newDurationString)
			if durationError != nil {
				tx.Rollback()
				baseHttpHandler.HandleError(responseWriter, durationError, utils.ValidationError, durationError.Error())
				return
			}
			parsedNewStartTime, startError := time.Parse(config.TimeLayoutHTMLForm, newStartTimeString)
			if startError != nil {
				tx.Rollback()
				baseHttpHandler.HandleError(responseWriter, startError, utils.ValidationError, config.ErrMsgInvalidStartTimeFormat)
				return
			}
			eventRecord.StartTime = parsedNewStartTime
			eventRecord.EndTime = parsedNewStartTime.Add(time.Duration(parsedNewDuration) * time.Hour)
			if err := eventRecord.Update(tx); err != nil {
				tx.Rollback()
				baseHttpHandler.HandleError(responseWriter, err, utils.DatabaseError, config.ErrMsgEventUpdate)
				return
			}
			tx.Commit()
			renderEventEditPage(applicationContext, responseWriter, request, eventRecord, false)
		case config.ActionShowAddVenue:
			tx.Rollback()
			renderEventEditPage(applicationContext, responseWriter, request, eventRecord, true)
		case config.ActionAddExistingVenue:
			venueID := request.FormValue(config.VenueIDParam)
			if venueID == "" {
				tx.Rollback()
				renderEventEditPage(applicationContext, responseWriter, request, eventRecord, true)
				return
			}
			allowedVenueIDs, err := models.FindVenueIDsAssociatedWithUserEvents(tx, currentUser.ID)
			if err != nil || !slices.Contains(allowedVenueIDs, venueID) {
				tx.Rollback()
				baseHttpHandler.HandleError(responseWriter, nil, utils.ForbiddenError, config.ErrMsgVenuePermission)
				return
			}
			eventRecord.VenueID = &venueID
			if err := eventRecord.Update(tx); err != nil {
				tx.Rollback()
				baseHttpHandler.HandleError(responseWriter, err, utils.DatabaseError, config.ErrMsgVenueAssociation)
				return
			}
			tx.Commit()
			renderEventEditPage(applicationContext, responseWriter, request, eventRecord, false)
		case config.ActionCreateNewVenue:
			newVenue := models.Venue{
				Name:        request.FormValue(config.VenueNameParam),
				Address:     request.FormValue(config.VenueAddressParam),
				Description: request.FormValue(config.VenueDescriptionParam),
				Capacity:    utils.MustParseInt(request.FormValue(config.VenueCapacityParam)),
				Phone:       request.FormValue(config.VenuePhoneParam),
				Email:       request.FormValue(config.VenueEmailParam),
				Website:     request.FormValue(config.VenueWebsiteParam),
			}
			if err := newVenue.Create(tx); err != nil {
				tx.Rollback()
				baseHttpHandler.HandleError(responseWriter, err, utils.DatabaseError, config.ErrMsgVenueCreation)
				return
			}
			eventRecord.VenueID = &newVenue.ID
			if err := eventRecord.Update(tx); err != nil {
				tx.Rollback()
				baseHttpHandler.HandleError(responseWriter, err, utils.DatabaseError, config.ErrMsgVenueAssociation)
				return
			}
			tx.Commit()
			// Reload the event record with associated venue details.
			if err := applicationContext.Database.Preload("Venue").Where("id = ?", eventRecord.ID).First(&eventRecord).Error; err != nil {
				baseHttpHandler.HandleError(responseWriter, err, utils.DatabaseError, "Failed to load updated event record")
				return
			}
			renderEventEditPage(applicationContext, responseWriter, request, eventRecord, false)
		default:
			tx.Rollback()
			http.Error(responseWriter, config.ErrMsgUnknownAction, http.StatusBadRequest)
		}
	}
}

// renderEventEditPage builds an EditViewData value using composed fields and renders the event edit view.
func renderEventEditPage(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, request *http.Request, eventRecord models.Event, showAddVenueSubform bool) {
	allowedVenueIDs, err := models.FindVenueIDsAssociatedWithUserEvents(applicationContext.Database, eventRecord.UserID)
	if err != nil {
		allowedVenueIDs = []string{}
	}
	userReusedVenues, err := models.FindVenuesByIDs(applicationContext.Database, allowedVenueIDs)
	if err != nil {
		userReusedVenues = []models.Venue{}
	}
	eventsOwnedByUser, err := models.FindEventsByUserID(applicationContext.Database, eventRecord.UserID, true, true)
	if err != nil {
		eventsOwnedByUser = []models.Event{}
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
	selectedVenueID := ""
	if eventRecord.VenueID != nil {
		selectedVenueID = *eventRecord.VenueID
	}
	editViewData := EditViewData{
		ListViewData: ListViewData{
			EventList: eventStatistics,
			SelectedItemForEdit: &EnhancedEventData{
				Event:                     eventRecord,
				CalculatedDurationInHours: float64(eventRecord.DurationHours()),
				SelectedVenueID:           selectedVenueID,
			},
			UserReusedVenues:          userReusedVenues,
			URLForEventActions:        config.WebEvents,
			URLForEventEdit:           config.WebEvents,
			URLForRSVPListBase:        config.WebRSVPs,
			URLForVenues:              config.WebVenues,
			ParamNameEventID:          config.EventIDParam,
			ParamNameTitle:            config.TitleParam,
			ParamNameDescription:      config.DescriptionParam,
			ParamNameStartTime:        config.StartTimeParam,
			ParamNameDuration:         config.DurationParam,
			ParamNameMethodOverride:   config.MethodOverrideParam,
			ParamNameVenueID:          config.VenueIDParam,
			ParamNameVenueName:        config.VenueNameParam,
			ParamNameVenueAddress:     config.VenueAddressParam,
			ParamNameVenueCapacity:    config.VenueCapacityParam,
			ParamNameVenuePhone:       config.VenuePhoneParam,
			ParamNameVenueEmail:       config.VenueEmailParam,
			ParamNameVenueWebsite:     config.VenueWebsiteParam,
			ParamNameVenueDescription: config.VenueDescriptionParam,
			VenueSelectCreateNewValue: config.VenueSelectCreateNewValue,
			ShowAddVenueSubform:       showAddVenueSubform,
			ButtonCancelEdit:          config.ButtonCancelEdit,
			LabelEventTitle:           config.LabelEventTitle,
			LabelEventDescription:     config.LabelEventDescription,
			LabelStartTime:            config.LabelStartTime,
			LabelDuration:             config.LabelDuration,
			LabelVenueDetails:         config.LabelVenueDetails,
			ButtonDeleteVenue:         config.ButtonDeleteVenue,
			ButtonAddVenue:            config.ButtonAddVenue,
			ButtonCreateNewVenue:      config.ButtonCreateNewVenue,
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
			FormattedStartTime:        "",
			CurrentDuration:           strconv.Itoa(eventRecord.DurationHours()),
		},
		ActionAddExistingVenue: config.ActionAddExistingVenue,
		ActionCreateNewVenue:   config.ActionCreateNewVenue,
	}
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameEvent, config.WebEvents)
	baseHandler.RenderView(responseWriter, request, config.TemplateEvents, editViewData)
}
