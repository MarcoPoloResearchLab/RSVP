package rsvp

import (
	"net/http"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

func verifyEventOwnership(baseHandler *handlers.BaseHttpHandler, responseWriter http.ResponseWriter, request *http.Request, databaseConnection *gorm.DB, event *models.Event, currentUserID string) bool {
	ownerID, ownerError := event.OwnerID(databaseConnection)
	if ownerError != nil {
		baseHandler.HandleError(responseWriter, ownerError, utils.DatabaseError, "Error retrieving event ownership.")
		return false
	}
	return baseHandler.VerifyResourceOwnership(responseWriter, request, ownerID, currentUserID)
}
