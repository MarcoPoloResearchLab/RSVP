package event

import (
	"errors"
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/middleware"
	"github.com/temirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

// DeleteHandler handles DELETE requests (or POST with _method=DELETE override) to delete an event.
func DeleteHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameEvent, config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodDelete) {
			return
		}

		currentUser := httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User)

		params, paramsOk := baseHandler.RequireParams(httpResponseWriter, httpRequest, config.EventIDParam)
		if !paramsOk {
			return
		}
		targetEventID := params[config.EventIDParam]

		// Start Transaction
		tx := applicationContext.Database.Begin()
		if tx.Error != nil {
			baseHandler.HandleError(httpResponseWriter, tx.Error, utils.DatabaseError, "Failed to start database transaction.")
			return
		}

		var eventRecord models.Event
		// Use transaction 'tx' for database operations
		findError := tx.First(&eventRecord, "id = ?", targetEventID).Error
		if findError != nil {
			tx.Rollback() // Rollback on error
			if errors.Is(findError, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findError, utils.NotFoundError, "Event not found.")
			} else {
				baseHandler.HandleError(httpResponseWriter, findError, utils.DatabaseError, "Error retrieving event.")
			}
			return
		}

		if !baseHandler.VerifyResourceOwnership(httpResponseWriter, httpRequest, eventRecord.UserID, currentUser.ID) {
			tx.Rollback() // Rollback on ownership failure
			return
		}

		// Explicitly delete associated RSVPs (soft delete) within the transaction
		deleteRSVPsError := tx.Where("event_id = ?", targetEventID).Delete(&models.RSVP{}).Error
		if deleteRSVPsError != nil {
			tx.Rollback()
			baseHandler.HandleError(httpResponseWriter, deleteRSVPsError, utils.DatabaseError, "Failed to delete associated RSVPs.")
			return
		}

		// Delete the event itself (soft delete) within the transaction
		deleteEventError := tx.Delete(&eventRecord).Error
		if deleteEventError != nil {
			tx.Rollback()
			baseHandler.HandleError(httpResponseWriter, deleteEventError, utils.DatabaseError, "Failed to delete the event.")
			return
		}

		// Commit Transaction
		commitError := tx.Commit().Error
		if commitError != nil {
			applicationContext.Logger.Printf("CRITICAL: Failed to commit transaction for event deletion %s: %v", targetEventID, commitError)
			baseHandler.HandleError(httpResponseWriter, commitError, utils.DatabaseError, "Failed to finalize event deletion.")
			return
		}

		baseHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
