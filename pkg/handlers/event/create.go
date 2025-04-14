package event

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/middleware"
	"github.com/temirov/RSVP/pkg/utils"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
)

const (
	createVenuePrefix = "create_"
)

// CreateHandler handles POST requests to create a new event, potentially including venue association or creation.
func CreateHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHttpHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameEvent, config.WebEvents)
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHttpHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodPost) {
			return
		}
		if err := httpRequest.ParseForm(); err != nil {
			baseHttpHandler.HandleError(httpResponseWriter, err, utils.ValidationError, utils.ErrMsgInvalidFormData)
			return
		}
		eventTitle := httpRequest.FormValue(config.TitleParam)
		eventDescription := httpRequest.FormValue(config.DescriptionParam)
		eventStartTimeString := httpRequest.FormValue(config.StartTimeParam)
		durationHoursString := httpRequest.FormValue(config.DurationParam)
		selectedVenueIDValue := httpRequest.FormValue(config.VenueIDParam)
		createNewVenueName := httpRequest.FormValue(createVenuePrefix + config.VenueNameParam)
		shouldCreateNewVenue := createNewVenueName != ""
		if validationError := utils.ValidateEventTitle(eventTitle); validationError != nil {
			baseHttpHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
			return
		}
		parsedDurationHours, validationError := utils.ValidateAndParseEventDuration(durationHoursString)
		if validationError != nil {
			baseHttpHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
			return
		}
		parsedStartTime, timeParseError := time.Parse(config.TimeLayoutHTMLForm, eventStartTimeString)
		if timeParseError != nil {
			baseHttpHandler.HandleError(httpResponseWriter, timeParseError, utils.ValidationError, utils.ErrMsgInvalidStartTimeFormat)
			return
		}
		if validationError := utils.ValidateEventStartTime(parsedStartTime); validationError != nil {
			baseHttpHandler.HandleError(httpResponseWriter, validationError, utils.ValidationError, validationError.Error())
			return
		}
		calculatedEndTime := parsedStartTime.Add(time.Duration(parsedDurationHours) * time.Hour)
		newEvent := models.Event{
			Title:       eventTitle,
			Description: eventDescription,
			StartTime:   parsedStartTime,
			EndTime:     calculatedEndTime,
			UserID:      httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User).ID,
			VenueID:     nil,
		}
		transactionError := applicationContext.Database.Transaction(func(tx *gorm.DB) error {
			var venueIDToAssociate *string = nil
			if shouldCreateNewVenue {
				newVenueData := venueFromForm(httpRequest, createVenuePrefix)
				if err := newVenueData.Create(tx); err != nil {
					return err
				}
				venueIDToAssociate = &newVenueData.ID
			} else if selectedVenueIDValue != "" {
				allowedVenueIDs, err := models.FindVenueIDsAssociatedWithUserEvents(tx, newEvent.UserID)
				if err != nil {
					return fmt.Errorf("could not verify venue permissions")
				}
				if !slices.Contains(allowedVenueIDs, selectedVenueIDValue) {
					return fmt.Errorf("you do not have permission to use the selected venue")
				}
				venueIDToAssociate = &selectedVenueIDValue
			} else {
				venueIDToAssociate = nil
			}
			newEvent.VenueID = venueIDToAssociate
			if err := newEvent.Create(tx); err != nil {
				return err
			}
			return nil
		})
		if transactionError != nil {
			permissionErrorMsg := "you do not have permission to use the selected venue"
			permissionCheckErrorMsg := "could not verify venue permissions"
			if validationErr := isModelValidationError(transactionError); validationErr != nil {
				baseHttpHandler.HandleError(httpResponseWriter, validationErr, utils.ValidationError, validationErr.Error())
			} else if transactionError.Error() == permissionErrorMsg {
				baseHttpHandler.HandleError(httpResponseWriter, transactionError, utils.ValidationError, permissionErrorMsg)
			} else if transactionError.Error() == permissionCheckErrorMsg {
				baseHttpHandler.HandleError(httpResponseWriter, transactionError, utils.ServerError, "Could not verify venue permissions. Please try again.")
			} else {
				baseHttpHandler.HandleError(httpResponseWriter, transactionError, utils.DatabaseError, "Failed to save the event and/or associated venue.")
			}
			return
		}
		baseHttpHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}

func venueFromForm(httpRequest *http.Request, prefix string) models.Venue {
	venueName := httpRequest.FormValue(prefix + config.VenueNameParam)
	venueAddress := httpRequest.FormValue(prefix + config.VenueAddressParam)
	venueDescription := httpRequest.FormValue(prefix + config.VenueDescriptionParam)
	venueCapacityStr := httpRequest.FormValue(prefix + config.VenueCapacityParam)
	venuePhone := httpRequest.FormValue(prefix + config.VenuePhoneParam)
	venueEmail := httpRequest.FormValue(prefix + config.VenueEmailParam)
	venueWebsite := httpRequest.FormValue(prefix + config.VenueWebsiteParam)
	venueCapacity := 0
	if venueCapacityStr != "" {
		parsedCapacity, err := strconv.Atoi(venueCapacityStr)
		if err == nil {
			venueCapacity = parsedCapacity
		}
	}
	return models.Venue{
		Name:        venueName,
		Address:     venueAddress,
		Description: venueDescription,
		Capacity:    venueCapacity,
		Phone:       venuePhone,
		Email:       venueEmail,
		Website:     venueWebsite,
	}
}

func isModelValidationError(err error) error {
	if errors.Is(err, utils.ErrVenueNameRequired) || errors.Is(err, utils.ErrVenueNameTooLong) ||
		errors.Is(err, utils.ErrTitleRequired) || errors.Is(err, utils.ErrTitleTooLong) {
		return err
	}
	return nil
}
