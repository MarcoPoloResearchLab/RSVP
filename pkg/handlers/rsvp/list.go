// File: pkg/handlers/rsvp/list.go
package rsvp

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// rsvpListViewData structure for the rsvps.tmpl template.
type rsvpListViewData struct {
	RsvpList            []models.RSVP
	SelectedItemForEdit *models.RSVP
	Event               models.Event
	// Config values passed to templates
	URLForRSVPActions       string
	URLForRSVPQRBase        string
	URLForEventList         string
	ParamNameEventID        string
	ParamNameRSVPID         string
	ParamNameName           string
	ParamNameResponse       string
	ParamNameMethodOverride string
}

// ListHandler handles GET requests for the RSVP list page for a specific event.
func ListHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, "RSVP", config.WebRSVPs)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		userSessionData, isAuthenticated := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)
		if !isAuthenticated {
			return
		}

		var currentUser models.User
		userFindErr := currentUser.FindByEmail(applicationContext.Database, userSessionData.UserEmail)
		if userFindErr != nil {
			if errors.Is(userFindErr, gorm.ErrRecordNotFound) {
				newUser, upsertErr := models.UpsertUser(applicationContext.Database, userSessionData.UserEmail, userSessionData.UserName, userSessionData.UserPicture)
				if upsertErr != nil {
					baseHandler.HandleError(httpResponseWriter, upsertErr, utils.DatabaseError, "Failed to create user record.")
					return
				}
				currentUser = *newUser
			} else {
				baseHandler.HandleError(httpResponseWriter, userFindErr, utils.DatabaseError, "Could not retrieve user info.")
				return
			}
		}

		eventID := baseHandler.GetParam(httpRequest, config.EventIDParam)
		rsvpID := baseHandler.GetParam(httpRequest, config.RSVPIDParam)

		var parentEvent models.Event
		var selectedRsvpForEdit *models.RSVP

		if rsvpID != "" {
			var rsvpToEdit models.RSVP
			rsvpFindErr := rsvpToEdit.FindByCode(applicationContext.Database, rsvpID)
			if rsvpFindErr != nil {
				if errors.Is(rsvpFindErr, gorm.ErrRecordNotFound) {
					baseHandler.HandleError(httpResponseWriter, rsvpFindErr, utils.NotFoundError, "The specified RSVP was not found.")
				} else {
					baseHandler.HandleError(httpResponseWriter, rsvpFindErr, utils.DatabaseError, "Error retrieving RSVP details for editing.")
				}
				return
			}

			eventFindErr := parentEvent.FindByID(applicationContext.Database, rsvpToEdit.EventID)
			if eventFindErr != nil {
				applicationContext.Logger.Printf("ERROR: Could not find parent event %s for RSVP %s during edit", rsvpToEdit.EventID, rsvpID)
				baseHandler.HandleError(httpResponseWriter, eventFindErr, utils.NotFoundError, "Could not find the parent event for this RSVP.")
				return
			}

			if parentEvent.UserID != currentUser.ID {
				baseHandler.HandleError(httpResponseWriter, nil, utils.ForbiddenError, "You do not have permission to edit RSVPs for this event.")
				return
			}

			if rsvpToEdit.Response == "" {
				rsvpToEdit.Response = "Pending"
			}
			selectedRsvpForEdit = &rsvpToEdit
			eventID = parentEvent.ID

		} else if eventID != "" {
			eventFindErr := parentEvent.FindByID(applicationContext.Database, eventID)
			if eventFindErr != nil {
				if errors.Is(eventFindErr, gorm.ErrRecordNotFound) {
					baseHandler.HandleError(httpResponseWriter, eventFindErr, utils.NotFoundError, "The specified event was not found.")
				} else {
					baseHandler.HandleError(httpResponseWriter, eventFindErr, utils.DatabaseError, "Error retrieving event details.")
				}
				return
			}

			if parentEvent.UserID != currentUser.ID {
				baseHandler.HandleError(httpResponseWriter, nil, utils.ForbiddenError, "You do not have permission to view RSVPs for this event.")
				return
			}

		} else {
			baseHandler.HandleError(httpResponseWriter, nil, utils.ValidationError, "An event ID or RSVP ID must be specified to view RSVPs.")
			return
		}

		rsvpRecords, rsvpRetrievalError := models.FindRSVPsByEventID(applicationContext.Database, eventID)
		if rsvpRetrievalError != nil {
			baseHandler.HandleError(httpResponseWriter, rsvpRetrievalError, utils.DatabaseError, "Could not retrieve the list of RSVPs for this event.")
			return
		}

		for index := range rsvpRecords {
			if rsvpRecords[index].Response == "" {
				rsvpRecords[index].Response = "Pending"
			}
		}

		// Prepare data payload, including config values
		viewData := rsvpListViewData{
			RsvpList:            rsvpRecords,
			SelectedItemForEdit: selectedRsvpForEdit,
			Event:               parentEvent,
			// Populate config values
			URLForRSVPActions:       config.WebRSVPs,
			URLForRSVPQRBase:        config.WebRSVPQR,
			URLForEventList:         config.WebEvents,
			ParamNameEventID:        config.EventIDParam,
			ParamNameRSVPID:         config.RSVPIDParam,
			ParamNameName:           config.NameParam,           // Populate
			ParamNameResponse:       config.ResponseParam,       // Populate
			ParamNameMethodOverride: config.MethodOverrideParam, // Populate
		}

		baseHandler.RenderView(httpResponseWriter, httpRequest, config.TemplateRSVPs, viewData)
	}
}
