package venue

import (
	"net/http"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

// DeleteVenueHandler handles DELETE requests to remove a venue.
func DeleteVenueHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHttpHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameVenue, config.WebVenues)
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if !baseHttpHandler.ValidateHttpMethod(responseWriter, request, http.MethodDelete) {
			return
		}

		requestParameters, parametersAreOk := baseHttpHandler.RequireParams(responseWriter, request, config.VenueIDParam)
		if !parametersAreOk {
			return
		}
		targetVenueIdentifier := requestParameters[config.VenueIDParam]
		currentUserData := request.Context().Value(middleware.ContextKeyUser).(*models.User)

		var venueRecord models.Venue
		err := venueRecord.FindByIDAndOwner(applicationContext.Database, targetVenueIdentifier, currentUserData.ID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				baseHttpHandler.HandleError(responseWriter, err, utils.NotFoundError, "Venue not found or you do not have permission to delete it.")
			} else {
				baseHttpHandler.HandleError(responseWriter, err, utils.DatabaseError, "Failed to verify venue ownership.")
			}
			return
		}

		tx := applicationContext.Database.Begin()
		if tx.Error != nil {
			baseHttpHandler.HandleError(responseWriter, tx.Error, utils.DatabaseError, "Failed to start transaction.")
			return
		}

		if deleteError := venueRecord.Delete(tx); deleteError != nil {
			tx.Rollback()
			baseHttpHandler.HandleError(responseWriter, deleteError, utils.DatabaseError, "Failed to delete venue.")
			return
		}

		if commitErr := tx.Commit().Error; commitErr != nil {
			applicationContext.Logger.Printf("CRITICAL: Failed to commit transaction for venue deletion %s: %v", targetVenueIdentifier, commitErr)
			baseHttpHandler.HandleError(responseWriter, commitErr, utils.DatabaseError, "Failed to finalize venue deletion.")
			return
		}

		baseHttpHandler.RedirectToList(responseWriter, request)
	}
}
