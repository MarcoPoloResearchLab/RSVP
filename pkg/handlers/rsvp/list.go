package rsvp

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/middleware"
	"github.com/temirov/RSVP/pkg/utils"
)

// rsvpListViewData is the structure passed as PageData.Data to the rsvps.tmpl template.
type rsvpListViewData struct {
	RsvpList                []models.RSVP
	SelectedItemForEdit     *models.RSVP
	Event                   models.Event
	URLForRSVPActions       string
	URLForRSVPQRBase        string
	URLForEventList         string
	ParamNameEventID        string
	ParamNameRSVPID         string
	ParamNameName           string
	ParamNameResponse       string
	ParamNameExtraGuests    string
	ParamNameMethodOverride string
	MaxGuestCount           int
}

// ListHandler handles GET requests for the RSVP list page (/rsvps/).
func ListHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameRSVP, config.WebRSVPs)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		currentUser := httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User)

		eventID := baseHandler.GetParam(httpRequest, config.EventIDParam)
		rsvpIDForEdit := baseHandler.GetParam(httpRequest, config.RSVPIDParam)

		var parentEvent models.Event
		var selectedRsvpForEdit *models.RSVP

		if rsvpIDForEdit != "" {
			var rsvpToEdit models.RSVP
			rsvpFindError := applicationContext.Database.First(&rsvpToEdit, "id = ?", rsvpIDForEdit).Error
			if rsvpFindError != nil {
				if errors.Is(rsvpFindError, gorm.ErrRecordNotFound) {
					baseHandler.HandleError(httpResponseWriter, rsvpFindError, utils.NotFoundError, "The specified RSVP was not found.")
				} else {
					baseHandler.HandleError(httpResponseWriter, rsvpFindError, utils.DatabaseError, "Error retrieving RSVP details for editing.")
				}
				return
			}

			eventFindError := applicationContext.Database.First(&parentEvent, "id = ?", rsvpToEdit.EventID).Error
			if eventFindError != nil {
				applicationContext.Logger.Printf("ERROR: Could not find parent event %s for RSVP %s during edit request to %s", rsvpToEdit.EventID, rsvpIDForEdit, httpRequest.URL.Path)
				if errors.Is(eventFindError, gorm.ErrRecordNotFound) {
					baseHandler.HandleError(httpResponseWriter, eventFindError, utils.NotFoundError, "Could not find the parent event for this RSVP.")
				} else {
					baseHandler.HandleError(httpResponseWriter, eventFindError, utils.DatabaseError, "Error retrieving parent event details.")
				}
				return
			}

			if !baseHandler.VerifyResourceOwnership(httpResponseWriter, httpRequest, parentEvent.UserID, currentUser.ID) {
				return
			}

			if rsvpToEdit.Response == config.RSVPResponsePending {
				rsvpToEdit.Response = ""
			}
			if rsvpToEdit.Response == config.RSVPResponseNoCommaZero {
				rsvpToEdit.Response = config.RSVPResponseNo
			}

			selectedRsvpForEdit = &rsvpToEdit
			eventID = parentEvent.ID

		} else if eventID != "" {
			eventFindError := applicationContext.Database.First(&parentEvent, "id = ?", eventID).Error
			if eventFindError != nil {
				if errors.Is(eventFindError, gorm.ErrRecordNotFound) {
					baseHandler.HandleError(httpResponseWriter, eventFindError, utils.NotFoundError, "The specified event was not found.")
				} else {
					baseHandler.HandleError(httpResponseWriter, eventFindError, utils.DatabaseError, "Error retrieving event details.")
				}
				return
			}

			if !baseHandler.VerifyResourceOwnership(httpResponseWriter, httpRequest, parentEvent.UserID, currentUser.ID) {
				return
			}
			selectedRsvpForEdit = nil

		} else {
			baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "An event ID or RSVP ID must be specified to view RSVPs.")
			return
		}

		rsvpRecords, rsvpRetrievalError := models.FindRSVPsByEventID(applicationContext.Database, eventID)
		if rsvpRetrievalError != nil {
			baseHandler.HandleError(httpResponseWriter, rsvpRetrievalError, utils.DatabaseError, "Could not retrieve the list of RSVPs for this event.")
			return
		}

		viewData := rsvpListViewData{
			RsvpList:                rsvpRecords,
			SelectedItemForEdit:     selectedRsvpForEdit,
			Event:                   parentEvent,
			URLForRSVPActions:       config.WebRSVPs,
			URLForRSVPQRBase:        config.WebRSVPQR,
			URLForEventList:         config.WebEvents,
			ParamNameEventID:        config.EventIDParam,
			ParamNameRSVPID:         config.RSVPIDParam,
			ParamNameName:           config.NameParam,
			ParamNameResponse:       config.ResponseParam,
			ParamNameExtraGuests:    config.ExtraGuestsParam,
			ParamNameMethodOverride: config.MethodOverrideParam,
			MaxGuestCount:           config.MaxGuestCount,
		}

		baseHandler.RenderView(httpResponseWriter, httpRequest, config.TemplateRSVPs, viewData)
	}
}
