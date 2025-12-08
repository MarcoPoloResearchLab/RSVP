// File: pkg/handlers/venue/list.go
// Package venue provides HTTP handler logic for venue-related operations.
package venue

import (
	"errors"
	"net/http"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

// ListVenuesHandler returns an HTTP handler that retrieves a list of venues owned by the current user
// and potentially prepares a specific venue for editing based on a query parameter.
func ListVenuesHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHttpHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameVenue, config.WebVenues)
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		currentUser := request.Context().Value(middleware.ContextKeyUser).(*models.User)
		requestedVenueIDForEdit := request.URL.Query().Get(config.VenueIDParam)
		var selectedVenueForEdit *models.Venue

		if requestedVenueIDForEdit != "" {
			var venueToEdit models.Venue
			err := venueToEdit.FindByIDAndOwner(applicationContext.Database, requestedVenueIDForEdit, currentUser.ID)
			if err == nil {
				selectedVenueForEdit = &venueToEdit
			} else {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					baseHttpHandler.ApplicationContext.Logger.Printf("WARN: Venue %s not found for editing or user %s does not own it.", requestedVenueIDForEdit, currentUser.ID)
				} else {
					baseHttpHandler.ApplicationContext.Logger.Printf("ERROR: Failed to retrieve venue %s for edit owned by user %s: %v", requestedVenueIDForEdit, currentUser.ID, err)
					baseHttpHandler.HandleError(responseWriter, err, utils.DatabaseError, "Failed to retrieve venue details.")
					return
				}
			}
		}

		venueList, err := models.FindVenuesByOwner(applicationContext.Database, currentUser.ID)
		if err != nil {
			baseHttpHandler.HandleError(responseWriter, err, utils.DatabaseError, "Failed to retrieve venues.")
			return
		}

		viewData := NewListViewData(venueList, selectedVenueForEdit)
		baseHttpHandler.RenderView(responseWriter, request, config.TemplateVenues, viewData)
	}
}
