package event

import (
	"net/http"
	"time"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// EventWithStats represents an event with additional statistics
type EventWithStats struct {
	ID                string
	Title             string
	StartTime         time.Time
	EndTime           time.Time
	RSVPCount         int
	RSVPAnsweredCount int
}

// EnhancedEvent represents an event with additional calculated fields
type EnhancedEvent struct {
	models.Event
	DurationHours int
}

// ListHandler handles GET requests to list all events.
func ListHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	// Create a base handler for events
	baseHandler := handlers.NewBaseHandler(applicationContext, "Event", config.WebEvents)

	return func(responseWriter http.ResponseWriter, request *http.Request) {
		// Validate HTTP method
		if !baseHandler.ValidateMethod(responseWriter, request, http.MethodGet) {
			return
		}

		// Get authenticated user data (authentication is guaranteed by middleware)
		sessionData, _ := baseHandler.RequireAuthentication(responseWriter, request)

		// Find or upsert the user
		var currentUser models.User
		if findUserError := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); findUserError != nil {
			newUser, upsertError := models.UpsertUser(
				applicationContext.Database,
				sessionData.UserEmail,
				sessionData.UserName,
				sessionData.UserPicture,
			)
			if upsertError != nil {
				baseHandler.HandleError(responseWriter, upsertError, utils.DatabaseError, "Failed to upsert user")
				return
			}
			currentUser = *newUser
		}

		// Load events for this user, also preload RSVPs so we can count them
		var userEvents []models.Event
		if eventsQueryError := applicationContext.Database.
			Preload("RSVPs").
			Where("user_id = ?", currentUser.ID).
			Find(&userEvents).Error; eventsQueryError != nil {
			baseHandler.HandleError(responseWriter, eventsQueryError, utils.DatabaseError, "Error retrieving events")
			return
		}

		// Compute total RSVPs (RSVPCount) and answered RSVPs (RSVPAnsweredCount)
		var eventsWithStats []EventWithStats
		for _, eventRecord := range userEvents {
			rsvpCount := len(eventRecord.RSVPs)
			rsvpAnsweredCount := 0
			for _, rsvp := range eventRecord.RSVPs {
				// "Answered" if the response is not empty/"Pending"
				if rsvp.Response != "" && rsvp.Response != "Pending" {
					rsvpAnsweredCount++
				}
			}

			eventsWithStats = append(eventsWithStats, EventWithStats{
				ID:                eventRecord.ID,
				Title:             eventRecord.Title,
				StartTime:         eventRecord.StartTime,
				EndTime:           eventRecord.EndTime,
				RSVPCount:         rsvpCount,
				RSVPAnsweredCount: rsvpAnsweredCount,
			})
		}

		// Check if an event ID is provided in the query parameters
		eventID := baseHandler.GetParam(request, config.EventIDParam)
		var selectedEvent *EnhancedEvent

		if eventID != "" {
			// Load the selected event
			var eventRecord models.Event
			if findEventError := eventRecord.FindByID(applicationContext.Database, eventID); findEventError == nil {
				// Calculate duration in hours
				durationTime := eventRecord.EndTime.Sub(eventRecord.StartTime)
				durationHours := int(durationTime.Hours())

				// Create enhanced event with duration
				selectedEvent = &EnhancedEvent{
					Event:         eventRecord,
					DurationHours: durationHours,
				}
			} else {
				baseHandler.Logger().Printf("Error loading selected event: %v", findEventError)
			}
		}

		// Pass the final slice to the template
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

		// Render template
		baseHandler.RenderTemplate(responseWriter, config.TemplateEvents, templateData)
	}
}
