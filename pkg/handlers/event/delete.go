package event

import (
	"errors"
	"net/http"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
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
		var sourceLinkCount int64
		if countError := tx.Model(&models.ExternalEventLink{}).Where("event_id = ?", eventRecord.ID).Count(&sourceLinkCount).Error; countError != nil {
			tx.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, countError, utils.DatabaseError, "Failed to inspect event ownership.")
			return
		}
		if sourceLinkCount != 0 {
			tx.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, services.ErrSourceOwnedMarker, utils.ConflictError, services.ErrSourceOwnedMarker.Error())
			return
		}
		var independentLane *models.Lane
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
			var laneRecord models.Lane
			if laneError := tx.First(&laneRecord, "id = ?", eventRecord.LaneID).Error; laneError != nil {
				tx.Rollback()
				baseHttpHandler.HandleError(httpResponseWriter, laneError, utils.DatabaseError, "Failed to retrieve the event lane.")
				return
			}
			independentLane = &laneRecord
		}
		if deleteRSVPsErr := tx.Unscoped().Where("event_id = ?", targetEventID).Delete(&models.RSVP{}).Error; deleteRSVPsErr != nil {
			tx.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, deleteRSVPsErr, utils.DatabaseError, "Failed to delete associated RSVPs.")
			return
		}
		if derivedDeleteError := services.DeleteDerivedMarkersForAnchor(tx, models.DerivedAnchorEvent, targetEventID); derivedDeleteError != nil {
			tx.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, derivedDeleteError, utils.DatabaseError, "Failed to delete derived markers.")
			return
		}
		if deleteEventErr := tx.Unscoped().Delete(&eventRecord).Error; deleteEventErr != nil {
			tx.Rollback()
			baseHttpHandler.HandleError(httpResponseWriter, deleteEventErr, utils.DatabaseError, "Failed to delete the event.")
			return
		}
		if eventRecord.RelationType == models.EventRelationIndependent {
			if deleteLaneError := tx.Unscoped().Delete(&models.Lane{}, "id = ?", eventRecord.LaneID).Error; deleteLaneError != nil {
				tx.Rollback()
				baseHttpHandler.HandleError(httpResponseWriter, deleteLaneError, utils.DatabaseError, "Failed to delete the event lane.")
				return
			}
			if normalizeError := services.NormalizeLaneOrder(tx, independentLane.CalendarID); normalizeError != nil {
				tx.Rollback()
				baseHttpHandler.HandleError(httpResponseWriter, normalizeError, utils.DatabaseError, "Failed to normalize the event lane order.")
				return
			}
		} else if laneBoundsError := services.RecalculateTemporalLaneBounds(tx, eventRecord.LaneID); laneBoundsError != nil {
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
