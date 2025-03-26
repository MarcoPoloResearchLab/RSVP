// File: pkg/handlers/event/list.go
package event

import (
	"errors"
	// "fmt" // No longer needed after removing fmt.Sprintf
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// EventStatisticsData holds calculated statistics for display in the event list.
type EventStatisticsData struct {
	ID                string
	Title             string
	StartTime         time.Time
	EndTime           time.Time
	RSVPCount         int
	RSVPAnsweredCount int
}

// EnhancedEventData extends models.Event with calculated fields needed for the edit form.
type EnhancedEventData struct {
	models.Event
	CalculatedDurationInHours int
}

// eventListViewData structure for the events.tmpl view.
// This is the struct passed as `PageData.Data` when calling RenderView.
type eventListViewData struct {
	EventList           []EventStatisticsData
	SelectedItemForEdit *EnhancedEventData
	// Config values passed to templates
	URLForEventActions      string
	URLForRSVPListBase      string
	ParamNameEventID        string
	ParamNameTitle          string // Added
	ParamNameDescription    string // Added
	ParamNameStartTime      string // Added
	ParamNameDuration       string // Added
	ParamNameMethodOverride string // Added
}

// ListEventsHandler handles GET requests for the events list page.
func ListEventsHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, "Event", config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		userSessionData, isAuthenticated := baseHandler.RequireAuthentication(httpResponseWriter, httpRequest)
		if !isAuthenticated {
			return
		}

		var currentUser models.User
		userFindError := currentUser.FindByEmail(applicationContext.Database, userSessionData.UserEmail)
		if userFindError != nil {
			if errors.Is(userFindError, gorm.ErrRecordNotFound) {
				newUser, upsertErr := models.UpsertUser(applicationContext.Database, userSessionData.UserEmail, userSessionData.UserName, userSessionData.UserPicture)
				if upsertErr != nil {
					baseHandler.HandleError(httpResponseWriter, upsertErr, utils.DatabaseError, "Failed to create user record during login.")
					return
				}
				currentUser = *newUser
			} else {
				baseHandler.HandleError(httpResponseWriter, userFindError, utils.DatabaseError, "Failed to retrieve user data.")
				return
			}
		}

		userEvents, eventsRetrievalError := models.FindEventsByUserID(applicationContext.Database, currentUser.ID, true)
		if eventsRetrievalError != nil {
			baseHandler.HandleError(httpResponseWriter, eventsRetrievalError, utils.DatabaseError, "Failed to retrieve your events.")
			return
		}

		eventStatistics := make([]EventStatisticsData, 0, len(userEvents))
		for _, event := range userEvents {
			totalRSVPCount := len(event.RSVPs)
			answeredRSVPCount := 0
			for _, rsvp := range event.RSVPs {
				if rsvp.Response != "" && rsvp.Response != "Pending" {
					answeredRSVPCount++
				}
			}
			eventStatistics = append(eventStatistics, EventStatisticsData{
				ID:                event.ID,
				Title:             event.Title,
				StartTime:         event.StartTime,
				EndTime:           event.EndTime,
				RSVPCount:         totalRSVPCount,
				RSVPAnsweredCount: answeredRSVPCount,
			})
		}

		var selectedEventForEdit *EnhancedEventData
		requestedEventID := baseHandler.GetParam(httpRequest, config.EventIDParam)

		if requestedEventID != "" {
			var eventToEdit models.Event
			findErr := eventToEdit.FindByIDAndOwner(applicationContext.Database, requestedEventID, currentUser.ID)
			if findErr == nil {
				duration := eventToEdit.EndTime.Sub(eventToEdit.StartTime)
				selectedEventForEdit = &EnhancedEventData{
					Event:                     eventToEdit,
					CalculatedDurationInHours: int(duration.Hours()),
				}
			} else if errors.Is(findErr, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findErr, utils.NotFoundError, "Event not found or you do not have permission to edit it.")
				return
			} else {
				baseHandler.HandleError(httpResponseWriter, findErr, utils.DatabaseError, "Error retrieving event details for editing.")
				return
			}
		}

		// Prepare the data payload, including the config values needed by templates
		viewData := eventListViewData{
			EventList:           eventStatistics,
			SelectedItemForEdit: selectedEventForEdit,
			// Populate config values
			URLForEventActions:      config.WebEvents,
			URLForRSVPListBase:      config.WebRSVPs,
			ParamNameEventID:        config.EventIDParam,
			ParamNameTitle:          config.TitleParam,          // Populate
			ParamNameDescription:    config.DescriptionParam,    // Populate
			ParamNameStartTime:      config.StartTimeParam,      // Populate
			ParamNameDuration:       config.DurationParam,       // Populate
			ParamNameMethodOverride: config.MethodOverrideParam, // Populate
		}

		baseHandler.RenderView(httpResponseWriter, httpRequest, config.TemplateEvents, viewData)
	}
}
