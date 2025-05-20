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

// CreateVenueHandler handles POST requests to create a new venue.
func CreateVenueHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHttpHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameVenue, config.WebVenues)
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if !baseHttpHandler.ValidateHttpMethod(responseWriter, request, http.MethodPost) {
			return
		}
		if err := request.ParseForm(); err != nil {
			baseHttpHandler.HandleError(responseWriter, err, utils.ValidationError, utils.ErrMsgInvalidFormData)
			return
		}
		currentUser := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		venueName := request.FormValue(config.VenueNameParam)
		venueAddress := request.FormValue(config.VenueAddressParam)
		venueDescription := request.FormValue(config.VenueDescriptionParam)
		venueCapacityString := request.FormValue(config.VenueCapacityParam)
		venuePhone := request.FormValue(config.VenuePhoneParam)
		venueEmail := request.FormValue(config.VenueEmailParam)
		venueWebsite := request.FormValue(config.VenueWebsiteParam)
		venueCapacity := 0
		if venueCapacityString != "" {
			parsedCapacity, parseError := strconv.Atoi(venueCapacityString)
			if parseError == nil {
				venueCapacity = parsedCapacity
			}
		}
		newVenue := models.Venue{
			Name:        venueName,
			Address:     venueAddress,
			Description: venueDescription,
			Capacity:    venueCapacity,
			Phone:       venuePhone,
			Email:       venueEmail,
			Website:     venueWebsite,
			UserID:      currentUser.ID,
		}
		transactionError := applicationContext.Database.Transaction(func(databaseTransaction *gorm.DB) error {
			if err := newVenue.Create(databaseTransaction); err != nil {
				return err
			}
			return nil
		})
		if transactionError != nil {
			baseHttpHandler.HandleError(responseWriter, transactionError, utils.DatabaseError, "Failed to create venue.")
			return
		}
		http.Redirect(responseWriter, request, baseHttpHandler.ResourceBasePathForRoutes, http.StatusSeeOther)
	}
}
