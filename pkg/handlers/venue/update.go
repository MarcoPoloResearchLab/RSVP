package venue

import (
	"net/http"
	"strconv"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/middleware"
	"github.com/temirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

// UpdateVenueHandler handles PUT/PATCH requests to update an existing venue.
func UpdateVenueHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHttpHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameVenue, config.WebVenues)
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if !baseHttpHandler.ValidateHttpMethod(responseWriter, request, http.MethodPut, http.MethodPatch) {
			return
		}
		currentUser := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		parameters, parametersAreOk := baseHttpHandler.RequireParams(responseWriter, request, config.VenueIDParam)
		if !parametersAreOk {
			return
		}
		targetVenueID := parameters[config.VenueIDParam]

		var existingVenue models.Venue
		if err := existingVenue.FindByIDAndOwner(applicationContext.Database, targetVenueID, currentUser.ID); err != nil {
			if err == gorm.ErrRecordNotFound {
				baseHttpHandler.HandleError(responseWriter, err, utils.NotFoundError, "Venue not found or you do not have permission to edit it.")
			} else {
				baseHttpHandler.HandleError(responseWriter, err, utils.DatabaseError, "Error retrieving venue for update.")
			}
			return
		}

		if err := request.ParseForm(); err != nil {
			baseHttpHandler.HandleError(responseWriter, err, utils.ValidationError, utils.ErrMsgInvalidFormData)
			return
		}

		newVenueName := request.FormValue(config.VenueNameParam)
		newVenueAddress := request.FormValue(config.VenueAddressParam)
		newVenueDescription := request.FormValue(config.VenueDescriptionParam)
		newVenueCapacityString := request.FormValue(config.VenueCapacityParam)
		newVenuePhone := request.FormValue(config.VenuePhoneParam)
		newVenueEmail := request.FormValue(config.VenueEmailParam)
		newVenueWebsite := request.FormValue(config.VenueWebsiteParam)

		newVenueCapacity := 0
		if newVenueCapacityString != "" {
			parsedCapacity, parseError := strconv.Atoi(newVenueCapacityString)
			if parseError == nil {
				newVenueCapacity = parsedCapacity
			} else {
				applicationContext.Logger.Printf("WARN: Could not parse venue capacity '%s' for venue %s. Using 0.", newVenueCapacityString, targetVenueID)
			}
		}

		existingVenue.Name = newVenueName
		existingVenue.Address = newVenueAddress
		existingVenue.Description = newVenueDescription
		existingVenue.Capacity = newVenueCapacity
		existingVenue.Phone = newVenuePhone
		existingVenue.Email = newVenueEmail
		existingVenue.Website = newVenueWebsite

		if err := existingVenue.Update(applicationContext.Database); err != nil {
			if validationErr := utils.IsValidationError(err); validationErr != nil {
				baseHttpHandler.HandleError(responseWriter, validationErr, utils.ValidationError, validationErr.Error())
			} else {
				baseHttpHandler.HandleError(responseWriter, err, utils.DatabaseError, "Failed to update venue.")
			}
			return
		}

		baseHttpHandler.RedirectToList(responseWriter, request)
	}
}
