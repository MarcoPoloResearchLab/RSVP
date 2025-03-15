package event

import (
	"net/http"
	"strconv"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// DeleteEventHandler handles POST requests to delete an event.
func DeleteEventHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
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

		// Load the event from the database
		var eventRecord models.Event
		if findError := eventRecord.FindByID(applicationContext.Database, uint(eventID)); findError != nil {
			http.Error(responseWriter, "Event not found", http.StatusNotFound)
			return
		}

		// Delete the event
		if deletionError := applicationContext.Database.Delete(&eventRecord).Error; deletionError != nil {
			applicationContext.Logger.Println("Failed to delete event:", deletionError)
			http.Error(responseWriter, "Could not delete event", http.StatusInternalServerError)
			return
		}

		// Redirect back to the events list after deletion
		http.Redirect(responseWriter, httpRequest, config.WebEvents, http.StatusSeeOther)
	}
}
