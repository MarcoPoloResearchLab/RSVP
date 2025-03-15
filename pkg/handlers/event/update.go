package event

import (
	"net/http"
	"strconv"
	"time"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// UpdateEventHandler handles POST requests to update an existing event.
func UpdateEventHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.Method != http.MethodPost {
			http.Error(responseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Retrieve event ID from the form
		eventIDStr := httpRequest.FormValue("event_id")
		if eventIDStr == "" {
			http.Error(responseWriter, "Event ID is required", http.StatusBadRequest)
			return
		}
		eventID, conversionError := strconv.Atoi(eventIDStr)
		if conversionError != nil {
			http.Error(responseWriter, "Invalid Event ID", http.StatusBadRequest)
			return
		}

		// Retrieve new event details from the form
		newTitle := httpRequest.FormValue("title")
		newDescription := httpRequest.FormValue("description")
		newStartTimeStr := httpRequest.FormValue("start_time")
		newDurationStr := httpRequest.FormValue("duration")
		if newTitle == "" || newStartTimeStr == "" || newDurationStr == "" {
			http.Error(responseWriter, "Title, start time, and duration are required", http.StatusBadRequest)
			return
		}

		const timeLayout = "2006-01-02T15:04"
		newStartTime, parseError := time.Parse(timeLayout, newStartTimeStr)
		if parseError != nil {
			http.Error(responseWriter, "Invalid start time format", http.StatusBadRequest)
			return
		}

		newDuration, durationError := strconv.Atoi(newDurationStr)
		if durationError != nil {
			http.Error(responseWriter, "Invalid duration value", http.StatusBadRequest)
			return
		}
		newEndTime := newStartTime.Add(time.Duration(newDuration) * time.Hour)

		// Load the existing event from the database
		var eventRecord models.Event
		if findError := eventRecord.FindByID(applicationContext.Database, uint(eventID)); findError != nil {
			http.Error(responseWriter, "Event not found", http.StatusNotFound)
			return
		}

		// Update event fields
		eventRecord.Title = newTitle
		eventRecord.Description = newDescription
		eventRecord.StartTime = newStartTime
		eventRecord.EndTime = newEndTime

		if saveError := eventRecord.Save(applicationContext.Database); saveError != nil {
			applicationContext.Logger.Println("Failed to update event:", saveError)
			http.Error(responseWriter, "Could not update event", http.StatusInternalServerError)
			return
		}

		// Redirect back to the events list after updating
		http.Redirect(responseWriter, httpRequest, config.WebEvents, http.StatusSeeOther)
	}
}
