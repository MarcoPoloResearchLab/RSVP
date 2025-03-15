package event

import (
	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"net/http"
	"strconv"
	"time"
)

func List(applicationContext *config.ApplicationContext) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.Method != http.MethodGet {
			http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Ensure user is logged in
		sessionData := handlers.GetUserData(httpRequest, applicationContext)
		if sessionData.UserEmail == "" {
			http.Redirect(httpResponseWriter, httpRequest, gconstants.LoginPath, http.StatusSeeOther)
			return
		}

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
				applicationContext.Logger.Println("Failed to upsert user:", upsertError)
				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			currentUser = *newUser
		}

		// Load events for this user, also preload RSVPs so we can count them
		var userEvents []models.Event
		if queryError := applicationContext.Database.
			Preload("RSVPs").
			Where("user_id = ?", currentUser.ID).
			Find(&userEvents).Error; queryError != nil {
			applicationContext.Logger.Println("Error retrieving events:", queryError)
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// We’ll store the computed stats in a separate struct
		type EventWithStats struct {
			ID                uint
			Title             string
			StartTime         time.Time
			EndTime           time.Time
			RSVPCount         int
			RSVPAnsweredCount int
		}

		// Compute total RSVPs (RSVPCount) and answered RSVPs (RSVPAnsweredCount)
		var eventsWithStats []EventWithStats
		for _, eventRecord := range userEvents {
			rsvpCount := len(eventRecord.RSVPs)
			rsvpAnsweredCount := 0
			for _, rsvp := range eventRecord.RSVPs {
				// "Answered" if the response is not empty/“Pending”
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

		// Pass the final slice to the template
		templateData := struct {
			UserPicture string
			UserName    string
			Events      []EventWithStats
		}{
			UserPicture: sessionData.UserPicture,
			UserName:    sessionData.UserName,
			Events:      eventsWithStats,
		}

		if renderError := applicationContext.Templates.ExecuteTemplate(
			httpResponseWriter,
			config.EventsList, // "event_index.html"
			templateData,
		); renderError != nil {
			applicationContext.Logger.Printf("Error rendering template: %v", renderError)
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// New handles POST requests for creating new events.
func New(applicationContext *config.ApplicationContext) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.Method != http.MethodPost {
			http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Ensure the user is logged in.
		sessionData := handlers.GetUserData(httpRequest, applicationContext)
		if sessionData.UserEmail == "" {
			http.Redirect(httpResponseWriter, httpRequest, gconstants.LoginPath, http.StatusSeeOther)
			return
		}

		// Find or upsert the current user.
		var currentUser models.User
		if err := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); err != nil {
			newUser, upsertError := models.UpsertUser(
				applicationContext.Database,
				sessionData.UserEmail,
				sessionData.UserName,
				sessionData.UserPicture,
			)
			if upsertError != nil {
				applicationContext.Logger.Println("Failed to upsert user:", upsertError)
				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			currentUser = *newUser
		}

		// Retrieve and validate form values.
		eventTitle := httpRequest.FormValue("title")
		eventDescription := httpRequest.FormValue("description")
		eventStartTimeStr := httpRequest.FormValue("start_time")
		durationStr := httpRequest.FormValue("duration")

		if eventTitle == "" || eventStartTimeStr == "" || durationStr == "" {
			http.Error(httpResponseWriter, "Title, start time, and duration are required", http.StatusBadRequest)
			return
		}

		// Parse start time (expected format: "2006-01-02T15:04").
		const timeLayout = "2006-01-02T15:04"
		eventStartTime, startTimeError := time.Parse(timeLayout, eventStartTimeStr)
		if startTimeError != nil {
			http.Error(httpResponseWriter, "Invalid start time format", http.StatusBadRequest)
			return
		}

		// Parse duration (in hours).
		durationHours, parseError := strconv.Atoi(durationStr)
		if parseError != nil {
			http.Error(httpResponseWriter, "Invalid duration value", http.StatusBadRequest)
			return
		}

		// Compute end time by adding the duration.
		eventEndTime := eventStartTime.Add(time.Duration(durationHours) * time.Hour)

		// Create a new event.
		newEvent := models.Event{
			Title:       eventTitle,
			Description: eventDescription,
			StartTime:   eventStartTime,
			EndTime:     eventEndTime,
			UserID:      currentUser.ID,
		}

		if creationError := newEvent.Create(applicationContext.Database); creationError != nil {
			applicationContext.Logger.Println("Error creating event:", creationError)
			http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Redirect back to the events list.
		http.Redirect(httpResponseWriter, httpRequest, config.WebEvents, http.StatusSeeOther)
	}
}
