package rsvp

import (
	"net/http"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// ListHandler handles:
//
//	GET /rsvps/?event_id=XYZ => protected list of RSVPs for that event
//	PUT /rsvps/?rsvp_id=ABC  => merges that event's RSVP list with an edit form for ABC
func ListHandler(appCtx *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(appCtx, "RSVP", config.WebRSVPs)

	return func(w http.ResponseWriter, r *http.Request) {
		if !baseHandler.ValidateMethod(w, r, http.MethodGet, http.MethodPut) {
			return
		}

		eventID := baseHandler.GetParam(r, config.EventIDParam)
		rsvpID := baseHandler.GetParam(r, config.RSVPIDParam)

		// CASE 1: GET => show event’s RSVP list (requires event_id)
		if r.Method == http.MethodGet {
			if eventID == "" {
				http.Error(w, "event_id is required for GET listing", http.StatusBadRequest)
				return
			}
			sessionData, authed := baseHandler.RequireAuthentication(w, r)
			if !authed {
				return
			}

			var currentUser models.User
			if errUser := currentUser.FindByEmail(appCtx.Database, sessionData.UserEmail); errUser != nil {
				baseHandler.HandleError(w, errUser, utils.DatabaseError, "User not found in DB")
				return
			}

			// Ensure ownership
			findOwner := func(eid string) (string, error) {
				var evt models.Event
				if loadErr := evt.FindByID(appCtx.Database, eid); loadErr != nil {
					return "", loadErr
				}
				return evt.UserID, nil
			}
			if !baseHandler.VerifyResourceOwnership(w, eventID, findOwner, currentUser.ID) {
				return
			}

			var eventRec models.Event
			if errEvt := eventRec.FindByID(appCtx.Database, eventID); errEvt != nil {
				baseHandler.HandleError(w, errEvt, utils.NotFoundError, "Event not found")
				return
			}

			var rsvps []models.RSVP
			if qErr := appCtx.Database.Where("event_id = ?", eventID).Find(&rsvps).Error; qErr != nil {
				baseHandler.HandleError(w, qErr, utils.DatabaseError, "Could not retrieve RSVPs")
				return
			}
			for i, rec := range rsvps {
				if rec.Response == "" {
					rsvps[i].Response = "Pending"
				}
			}

			data := struct {
				RSVPRecords  []models.RSVP
				SelectedRSVP *models.RSVP
				Event        models.Event
				UserPicture  string
				UserName     string
			}{
				RSVPRecords:  rsvps,
				SelectedRSVP: nil,
				Event:        eventRec,
				UserPicture:  sessionData.UserPicture,
				UserName:     sessionData.UserName,
			}
			baseHandler.RenderTemplate(w, config.TemplateRSVPs, data)
			return
		}

		// CASE 2: PUT => merges event’s RSVP list with an edit form for ?rsvp_id=
		if r.Method == http.MethodPut {
			if rsvpID == "" {
				http.Error(w, "rsvp_id required for PUT editing", http.StatusBadRequest)
				return
			}
			sessionData, authed := baseHandler.RequireAuthentication(w, r)
			if !authed {
				return
			}

			var currentUser models.User
			if errUser := currentUser.FindByEmail(appCtx.Database, sessionData.UserEmail); errUser != nil {
				baseHandler.HandleError(w, errUser, utils.DatabaseError, "User not found")
				return
			}

			var rsvpRec models.RSVP
			if loadErr := rsvpRec.FindByCode(appCtx.Database, rsvpID); loadErr != nil {
				baseHandler.HandleError(w, loadErr, utils.NotFoundError, "RSVP not found")
				return
			}
			var eventRec models.Event
			if evtErr := eventRec.FindByID(appCtx.Database, rsvpRec.EventID); evtErr != nil {
				baseHandler.HandleError(w, evtErr, utils.NotFoundError, "Event not found")
				return
			}
			if eventRec.UserID != currentUser.ID {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			var allRSVPs []models.RSVP
			if qErr := appCtx.Database.Where("event_id = ?", eventRec.ID).Find(&allRSVPs).Error; qErr != nil {
				baseHandler.HandleError(w, qErr, utils.DatabaseError, "Could not load RSVPs")
				return
			}
			for i, rec := range allRSVPs {
				if rec.Response == "" {
					allRSVPs[i].Response = "Pending"
				}
			}

			data := struct {
				RSVPRecords  []models.RSVP
				SelectedRSVP *models.RSVP
				Event        models.Event
				UserPicture  string
				UserName     string
			}{
				RSVPRecords:  allRSVPs,
				SelectedRSVP: &rsvpRec,
				Event:        eventRec,
				UserPicture:  sessionData.UserPicture,
				UserName:     sessionData.UserName,
			}
			baseHandler.RenderTemplate(w, config.TemplateRSVPs, data)
			return
		}
	}
}
