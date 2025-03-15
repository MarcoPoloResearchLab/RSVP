package event

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	gconstants "github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
)

// eventWithStats aggregates an event with computed RSVP statistics.
type eventWithStats struct {
	ID                uint
	Title             string
	StartTime         time.Time
	EndTime           time.Time
	RSVPCount         int
	RSVPAnsweredCount int
}

// getCurrentUser retrieves the current user based on session data.
// It upserts the user if not found.
func getCurrentUser(httpRequest *http.Request, applicationContext *config.ApplicationContext) (models.User, error) {
	sessionData := handlers.GetUserData(httpRequest, applicationContext)
	if sessionData.UserEmail == "" {
		return models.User{}, errors.New("user not logged in")
	}

	var currentUser models.User
	if err := currentUser.FindByEmail(applicationContext.Database, sessionData.UserEmail); err != nil {
		newUser, upsertError := models.UpsertUser(
			applicationContext.Database,
			sessionData.UserEmail,
			sessionData.UserName,
			sessionData.UserPicture,
		)
		if upsertError != nil {
			return models.User{}, upsertError
		}
		currentUser = *newUser
	}
	return currentUser, nil
}

// parseEventCreationForm extracts and validates form values for event creation.
// Expected form fields are "title", "description", "start_time", and "duration".
func parseEventCreationForm(httpRequest *http.Request) (title string, description string, startTime time.Time, durationHours int, err error) {
	title = httpRequest.FormValue("title")
	description = httpRequest.FormValue("description")
	startTimeStr := httpRequest.FormValue("start_time")
	durationStr := httpRequest.FormValue("duration")

	if title == "" || startTimeStr == "" || durationStr == "" {
		err = errors.New("title, start time, and duration are required")
		return
	}

	const timeLayout = "2006-01-02T15:04"
	startTime, err = time.Parse(timeLayout, startTimeStr)
	if err != nil {
		err = errors.New("invalid start time format")
		return
	}

	durationHours, err = strconv.Atoi(durationStr)
	if err != nil {
		err = errors.New("invalid duration value")
		return
	}
	return
}

// createNewEvent computes the event's end time (start time + duration)
// and creates the event record in the database.
func createNewEvent(currentUser models.User, title, description string, startTime time.Time, durationHours int, applicationContext *config.ApplicationContext) error {
	endTime := startTime.Add(time.Duration(durationHours) * time.Hour)
	newEvent := models.Event{
		Title:       title,
		Description: description,
		StartTime:   startTime,
		EndTime:     endTime,
		UserID:      currentUser.ID,
	}
	return newEvent.Create(applicationContext.Database)
}

// listEvents retrieves events for the current user (preloading RSVPs)
// and computes RSVP statistics for each event.
func listEvents(currentUser models.User, applicationContext *config.ApplicationContext) ([]eventWithStats, error) {
	var userEvents []models.Event
	err := applicationContext.Database.
		Preload("RSVPs").
		Where("user_id = ?", currentUser.ID).
		Find(&userEvents).Error
	if err != nil {
		return nil, err
	}

	var events []eventWithStats
	for _, eventRecord := range userEvents {
		rsvpCount := len(eventRecord.RSVPs)
		rsvpAnsweredCount := 0
		for _, rsvp := range eventRecord.RSVPs {
			// Consider an RSVP "answered" if its response is neither empty nor "Pending".
			if rsvp.Response != "" && rsvp.Response != "Pending" {
				rsvpAnsweredCount++
			}
		}
		events = append(events, eventWithStats{
			ID:                eventRecord.ID,
			Title:             eventRecord.Title,
			StartTime:         eventRecord.StartTime,
			EndTime:           eventRecord.EndTime,
			RSVPCount:         rsvpCount,
			RSVPAnsweredCount: rsvpAnsweredCount,
		})
	}
	return events, nil
}

// EventIndexHandler is the public handler that supports both GET and POST on "/events".
// GET: Lists events with computed RSVP statistics.
// POST: Creates a new event based on the submitted form (using a duration field instead of end time).
// EventIndexHandler is the public handler for GET (list events) and POST (create event)
// at the /events route.
func EventIndexHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		currentUser, err := getCurrentUser(httpRequest, applicationContext)
		if err != nil {
			http.Redirect(httpResponseWriter, httpRequest, gconstants.LoginPath, http.StatusSeeOther)
			return
		}

		switch httpRequest.Method {
		case http.MethodPost:
			// Process new event creation.
			title, description, startTime, durationHours, err := parseEventCreationForm(httpRequest)
			if err != nil {
				http.Error(httpResponseWriter, err.Error(), http.StatusBadRequest)
				return
			}
			if err = createNewEvent(currentUser, title, description, startTime, durationHours, applicationContext); err != nil {
				applicationContext.Logger.Println("Error creating event:", err)
				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			http.Redirect(httpResponseWriter, httpRequest, config.WebEvents, http.StatusSeeOther)
		case http.MethodGet:
			// List events along with RSVP statistics.
			events, err := listEvents(currentUser, applicationContext)
			if err != nil {
				applicationContext.Logger.Println("Error retrieving events:", err)
				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			sessionData := handlers.GetUserData(httpRequest, applicationContext)
			templateData := struct {
				UserPicture    string
				UserName       string
				Events         []eventWithStats
				CreateEventURL string // New field for the event creation route.
			}{
				UserPicture:    sessionData.UserPicture,
				UserName:       sessionData.UserName,
				Events:         events,
				CreateEventURL: config.WebEvents, // Use your constant here.
			}
			if renderError := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, config.EventsList, templateData); renderError != nil {
				applicationContext.Logger.Printf("Error rendering template: %v", renderError)
				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			}
		default:
			http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}
