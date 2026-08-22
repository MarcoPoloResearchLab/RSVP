package event

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

func DeleteHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHttpHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameEvent, config.WebEvents)
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHttpHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodDelete) {
			return
		}
		currentUser := httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User)
		params, paramsOk := baseHttpHandler.RequireParams(httpResponseWriter, httpRequest, config.EventIDParam)
		if !paramsOk {
			return
		}
		targetEventID := params[config.EventIDParam]
		tx := applicationContext.Database.Begin()
		if tx.Error != nil {
			baseHttpHandler.HandleError(httpResponseWriter, tx.Error, utils.DatabaseError, "Failed to start database transaction.")
			return
		}
		var eventRecord models.Event
		if err := eventRecord.FindByID(tx, targetEventID); err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				baseHttpHandler.HandleError(httpResponseWriter, err, utils.NotFoundError, "Event not found.")
			} else {
				baseHttpHandler.HandleError(httpResponseWriter, err, utils.DatabaseError, "Error retrieving event.")
			}
			return
		}
		eventOwnerID, ownerError := eventRecord.OwnerID(tx)
		if ownerError != nil {
			tx.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, ownerError, utils.DatabaseError, "Error retrieving event ownership.")
			return
		}
		if !baseHttpHandler.VerifyResourceOwnership(httpResponseWriter, httpRequest, eventOwnerID, currentUser.ID) {
			tx.Rollback()
			return
		}
		if eventRecord.RelationType == models.EventRelationIndependent {
			hasDependents, dependentsError := eventRecord.HasDependentEvents(tx)
			if dependentsError != nil {
				tx.Rollback()
				baseHttpHandler.HandleError(httpResponseWriter, dependentsError, utils.DatabaseError, "Failed to inspect dependent events.")
				return
			}
			if hasDependents {
				tx.Rollback()
				baseHttpHandler.HandleError(httpResponseWriter, models.ErrEventHasDependents, utils.ConflictError, "Delete dependent events before deleting their anchor event.")
				return
			}
		}
		if deleteRSVPsErr := tx.Where("event_id = ?", targetEventID).Delete(&models.RSVP{}).Error; deleteRSVPsErr != nil {
			tx.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, deleteRSVPsErr, utils.DatabaseError, "Failed to delete associated RSVPs.")
			return
		}
		if deleteEventErr := tx.Delete(&eventRecord).Error; deleteEventErr != nil {
			tx.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, deleteEventErr, utils.DatabaseError, "Failed to delete the event.")
			return
		}
		if eventRecord.RelationType == models.EventRelationIndependent {
			if deleteLaneError := tx.Delete(&models.Lane{}, "id = ?", eventRecord.LaneID).Error; deleteLaneError != nil {
				tx.Rollback()
				baseHttpHandler.HandleError(httpResponseWriter, deleteLaneError, utils.DatabaseError, "Failed to delete the event lane.")
				return
			}
		} else if laneBoundsError := models.RecalculateFiniteLaneEnd(tx, eventRecord.LaneID); laneBoundsError != nil {
			tx.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, laneBoundsError, utils.DatabaseError, "Failed to update the event lane.")
			return
		}
		if commitErr := tx.Commit().Error; commitErr != nil {
			applicationContext.Logger.Printf("CRITICAL: Failed to commit transaction for event deletion %s: %v", targetEventID, commitErr)
			baseHttpHandler.HandleError(httpResponseWriter, commitErr, utils.DatabaseError, "Failed to finalize event deletion.")
			return
		}
		baseHttpHandler.RedirectToList(httpResponseWriter, httpRequest)
	}
}
