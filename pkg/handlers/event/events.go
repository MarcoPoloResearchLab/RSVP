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

// EventWithStats aggregates an event with computed RSVP statistics.
type EventWithStats struct {
	ID                uint
	Title             string
	StartTime         time.Time
	EndTime           time.Time
	RSVPCount         int
	RSVPAnsweredCount int
}

// SelectedEventData represents the full details of a selected event.
type SelectedEventData struct {
	ID            uint
	Title         string
	Description   string
	StartTime     time.Time
	EndTime       time.Time
	RSVPs         []models.RSVP
	DurationHours int
}

// getCurrentUser retrieves the current user based on session data.
// It upserts the user if not found.
func getCurrentUser(
	httpRequest *http.Request,
	applicationContext *config.ApplicationContext,
) (models.User, error) {

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
func parseEventCreationForm(
	httpRequest *http.Request,
) (
	eventTitle string,
	eventDescription string,
	parsedStartTime time.Time,
	parsedDurationHours int,
	parsedError error,
) {
	eventTitle = httpRequest.FormValue("title")
	eventDescription = httpRequest.FormValue("description")
	startTimeString := httpRequest.FormValue("start_time")
	durationString := httpRequest.FormValue("duration")

	if eventTitle == "" || startTimeString == "" || durationString == "" {
		parsedError = errors.New("title, start time, and duration are required")
		return
	}

	const timeLayout = "2006-01-02T15:04"
	parsedStartTime, parsedError = time.Parse(timeLayout, startTimeString)
	if parsedError != nil {
		parsedError = errors.New("invalid start time format")
		return
	}

	parsedDurationHours, parsedError = strconv.Atoi(durationString)
	if parsedError != nil {
		parsedError = errors.New("invalid duration value")
		return
	}
	return
}

// createNewEvent computes the event's end time (start time + duration)
// and creates the event record in the database.
func createNewEvent(
	currentUser models.User,
	eventTitle string,
	eventDescription string,
	eventStartTime time.Time,
	durationHours int,
	applicationContext *config.ApplicationContext,
) error {
	eventEndTime := eventStartTime.Add(time.Duration(durationHours) * time.Hour)
	newEvent := models.Event{
		Title:       eventTitle,
		Description: eventDescription,
		StartTime:   eventStartTime,
		EndTime:     eventEndTime,
		UserID:      currentUser.ID,
	}
	return newEvent.Create(applicationContext.Database)
}

// listEvents retrieves events for the current user (preloading RSVPs)
// and computes RSVP statistics for each event.
func listEvents(
	currentUser models.User,
	applicationContext *config.ApplicationContext,
) ([]EventWithStats, error) {
	var userEvents []models.Event
	queryError := applicationContext.Database.
		Preload("RSVPs").
		Where("user_id = ?", currentUser.ID).
		Find(&userEvents).Error
	if queryError != nil {
		return nil, queryError
	}

	var eventList []EventWithStats
	for _, eventRecord := range userEvents {
		rsvpCount := len(eventRecord.RSVPs)
		rsvpAnsweredCount := 0
		for _, rsvpRecord := range eventRecord.RSVPs {
			if rsvpRecord.Response != "" && rsvpRecord.Response != "Pending" {
				rsvpAnsweredCount++
			}
		}
		eventList = append(eventList, EventWithStats{
			ID:                eventRecord.ID,
			Title:             eventRecord.Title,
			StartTime:         eventRecord.StartTime,
			EndTime:           eventRecord.EndTime,
			RSVPCount:         rsvpCount,
			RSVPAnsweredCount: rsvpAnsweredCount,
		})
	}
	return eventList, nil
}

// loadSelectedEvent attempts to load a selected event from the query parameter "event_id".
// It returns a pointer to SelectedEventData if successful, or nil if not provided.
func loadSelectedEvent(
	httpRequest *http.Request,
	currentUser models.User,
	applicationContext *config.ApplicationContext,
) (*SelectedEventData, error) {
	eventIDParam := httpRequest.URL.Query().Get("event_id")
	if eventIDParam == "" {
		return nil, nil // No selected event provided
	}
	parsedEventID, parseError := strconv.ParseUint(eventIDParam, 10, 32)
	if parseError != nil {
		return nil, parseError
	}
	var eventRecord models.Event
	loadError := eventRecord.LoadWithRSVPs(applicationContext.Database, uint(parsedEventID))
	if loadError != nil {
		return nil, loadError
	}
	// Ensure the event belongs to the current user.
	if eventRecord.UserID != currentUser.ID {
		return nil, errors.New("selected event does not belong to current user")
	}
	// Mark empty RSVP responses as "Pending"
	for index, rsvpRecord := range eventRecord.RSVPs {
		if rsvpRecord.Response == "" {
			eventRecord.RSVPs[index].Response = "Pending"
		}
	}
	selectedEvent := &SelectedEventData{
		ID:            eventRecord.ID,
		Title:         eventRecord.Title,
		Description:   eventRecord.Description,
		StartTime:     eventRecord.StartTime,
		EndTime:       eventRecord.EndTime,
		RSVPs:         eventRecord.RSVPs,
		DurationHours: int(eventRecord.EndTime.Sub(eventRecord.StartTime).Hours()),
	}
	return selectedEvent, nil
}

// EventIndexHandler is the public handler for GET (list events) and POST (create event)
// at the /events route.
// GET: Lists events with computed RSVP statistics, and if an "event_id" query parameter is provided,
// loads that event's details to be displayed above the events table.
// POST: Creates a new event based on the submitted form (using a duration field instead of end time).
func EventIndexHandler(
	applicationContext *config.ApplicationContext,
) http.HandlerFunc {
	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		currentUser, userError := getCurrentUser(httpRequest, applicationContext)
		if userError != nil {
			http.Redirect(httpResponseWriter, httpRequest, gconstants.LoginPath, http.StatusSeeOther)
			return
		}

		switch httpRequest.Method {
		case http.MethodPost:
			eventTitle, eventDescription, parsedStartTime, parsedDurationHours, formParsingError :=
				parseEventCreationForm(httpRequest)
			if formParsingError != nil {
				http.Error(httpResponseWriter, formParsingError.Error(), http.StatusBadRequest)
				return
			}
			creationError := createNewEvent(
				currentUser,
				eventTitle,
				eventDescription,
				parsedStartTime,
				parsedDurationHours,
				applicationContext,
			)
			if creationError != nil {
				applicationContext.Logger.Println("Error creating event:", creationError)
				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			http.Redirect(httpResponseWriter, httpRequest, config.WebEvents, http.StatusSeeOther)

		case http.MethodGet:
			eventList, listError := listEvents(currentUser, applicationContext)
			if listError != nil {
				applicationContext.Logger.Println("Error retrieving events:", listError)
				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			selectedEvent, selectedEventError := loadSelectedEvent(httpRequest, currentUser, applicationContext)
			if selectedEventError != nil {
				applicationContext.Logger.Println("Error loading selected event:", selectedEventError)
				selectedEvent = nil // Ignore the error and do not display a selected event
			}

			sessionData := handlers.GetUserData(httpRequest, applicationContext)
			templateData := struct {
				UserPicture    string
				UserName       string
				Events         []EventWithStats
				CreateEventURL string
				SelectedEvent  *SelectedEventData
			}{
				UserPicture:    sessionData.UserPicture,
				UserName:       sessionData.UserName,
				Events:         eventList,
				CreateEventURL: config.WebEvents,
				SelectedEvent:  selectedEvent,
			}

			renderError := applicationContext.Templates.ExecuteTemplate(httpResponseWriter, config.EventsList, templateData)
			if renderError != nil {
				applicationContext.Logger.Printf("Error rendering template: %v", renderError)
				http.Error(httpResponseWriter, "Internal Server Error", http.StatusInternalServerError)
			}

		default:
			http.Error(httpResponseWriter, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}
}
