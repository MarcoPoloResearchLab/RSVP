package event

import (
	"net/http"
	"time"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

type EventWithStats struct {
	ID                string
	Title             string
	StartTime         time.Time
	EndTime           time.Time
	RSVPCount         int
	RSVPAnsweredCount int
}

type EnhancedEvent struct {
	models.Event
	DurationHours int
}

// ListHandler handles GET requests to list all events.
func ListHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(applicationContext, "Event", config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		sessionData, _ := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)

		var currentUser models.User
		userFindError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail)
		if userFindError != nil {
			upsertedUser, upsertError := models.UpsertUser(
				applicationContext.Database,
				sessionData.UserEmail,
				sessionData.UserName,
				sessionData.UserPicture,
			)
			if upsertError != nil {
				baseHandler.HandleError(httpResponseWriter, upsertError, utils.DatabaseError, "Failed to upsert user")
				return
			}
			currentUser = *upsertedUser
		}

		var userEvents []models.Event
		eventsQueryError := applicationContext.Database.
			Preload("RSVPs").
			Where("user_id = ?", currentUser.ID).
			Find(&userEvents).Error
		if eventsQueryError != nil {
			baseHandler.HandleError(httpResponseWriter, eventsQueryError, utils.DatabaseError, "Error retrieving events")
			return
		}

		var eventsWithStats []EventWithStats
		for _, singleEvent := range userEvents {
			rsvpCount := len(singleEvent.RSVPs)
			rsvpAnsweredCount := 0
			for _, singleRSVP := range singleEvent.RSVPs {
				if singleRSVP.Response != "" && singleRSVP.Response != "Pending" {
					rsvpAnsweredCount++
				}
			}

			eventsWithStats = append(eventsWithStats, EventWithStats{
				ID:                singleEvent.ID,
				Title:             singleEvent.Title,
				StartTime:         singleEvent.StartTime,
				EndTime:           singleEvent.EndTime,
				RSVPCount:         rsvpCount,
				RSVPAnsweredCount: rsvpAnsweredCount,
			})
		}

		eventIDFromParams := baseHandler.GetParam(httpRequest, config.EventIDParam)
		var selectedEvent *EnhancedEvent

		if eventIDFromParams != "" {
			var foundEvent models.Event
			findEventError := foundEvent.FindByID(applicationContext.Database, eventIDFromParams)
			if findEventError == nil {
				calculatedDuration := foundEvent.EndTime.Sub(foundEvent.StartTime)
				durationHours := int(calculatedDuration.Hours())
				selectedEvent = &EnhancedEvent{
					Event:         foundEvent,
					DurationHours: durationHours,
				}
			} else {
				baseHandler.Logger().Printf("Error loading selected event: %v", findEventError)
			}
		}

		templateData := struct {
			UserPicture    string
			UserName       string
			Events         []EventWithStats
			SelectedEvent  *EnhancedEvent
			CreateEventURL string
		}{
			UserPicture:    sessionData.UserPicture,
			UserName:       sessionData.UserName,
			Events:         eventsWithStats,
			SelectedEvent:  selectedEvent,
			CreateEventURL: config.WebEvents,
		}

		baseHandler.RenderTemplate(httpResponseWriter, config.TemplateEvents, templateData)
	}
}
