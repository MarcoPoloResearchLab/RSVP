package event

import (
	"errors"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/middleware"
	"github.com/temirov/RSVP/pkg/utils"
)

// StatisticsData holds calculated statistics for a single event, used for display in the list view.
type StatisticsData struct {
	ID                string
	Title             string
	StartTime         time.Time
	EndTime           time.Time
	RSVPCount         int
	RSVPAnsweredCount int
}

// EnhancedEventData extends models.Event with calculated fields needed specifically for the edit form view.
type EnhancedEventData struct {
	models.Event
	CalculatedDurationInHours int
}

// eventListViewData is the structure passed as PageData.Data to the events.tmpl template.
type eventListViewData struct {
	EventList               []StatisticsData
	SelectedItemForEdit     *EnhancedEventData
	URLForEventActions      string
	URLForRSVPListBase      string
	ParamNameEventID        string
	ParamNameTitle          string
	ParamNameDescription    string
	ParamNameStartTime      string
	ParamNameDuration       string
	ParamNameMethodOverride string
}

// ListEventsHandler handles GET requests for the events list page (/events/).
func ListEventsHandler(applicationContext *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameEvent, config.WebEvents)

	return func(httpResponseWriter http.ResponseWriter, httpRequest *http.Request) {
		if !baseHandler.ValidateHttpMethod(httpResponseWriter, httpRequest, http.MethodGet) {
			return
		}

		currentUser := httpRequest.Context().Value(middleware.ContextKeyUser).(*models.User)

		userEvents, eventsRetrievalError := models.FindEventsByUserID(applicationContext.Database, currentUser.ID, true)
		if eventsRetrievalError != nil {
			baseHandler.HandleError(httpResponseWriter, eventsRetrievalError, utils.DatabaseError, "Failed to retrieve your events.")
			return
		}

		eventStatistics := make([]StatisticsData, 0, len(userEvents))
		for _, event := range userEvents {
			totalRSVPCount := len(event.RSVPs)
			answeredRSVPCount := 0
			for _, rsvp := range event.RSVPs {
				if rsvp.Response != "" && rsvp.Response != config.RSVPResponsePending {
					answeredRSVPCount++
				}
			}
			eventStatistics = append(eventStatistics, StatisticsData{
				ID:                event.ID,
				Title:             event.Title,
				StartTime:         event.StartTime,
				EndTime:           event.EndTime,
				RSVPCount:         totalRSVPCount,
				RSVPAnsweredCount: answeredRSVPCount,
			})
		}

		var selectedEventForEdit *EnhancedEventData
		requestedEventIDForEdit := baseHandler.GetParam(httpRequest, config.EventIDParam)

		if requestedEventIDForEdit != "" {
			var eventToEdit models.Event
			findError := eventToEdit.FindByIDAndOwner(applicationContext.Database, requestedEventIDForEdit, currentUser.ID)
			if findError == nil {
				selectedEventForEdit = &EnhancedEventData{
					Event:                     eventToEdit,
					CalculatedDurationInHours: eventToEdit.DurationHours(),
				}
			} else if errors.Is(findError, gorm.ErrRecordNotFound) {
				baseHandler.HandleError(httpResponseWriter, findError, utils.NotFoundError, "Event not found or you do not have permission to edit it.")
				return
			} else {
				baseHandler.HandleError(httpResponseWriter, findError, utils.DatabaseError, "Error retrieving event details for editing.")
				return
			}
		}

		viewData := eventListViewData{
			EventList:               eventStatistics,
			SelectedItemForEdit:     selectedEventForEdit,
			URLForEventActions:      config.WebEvents,
			URLForRSVPListBase:      config.WebRSVPs,
			ParamNameEventID:        config.EventIDParam,
			ParamNameTitle:          config.TitleParam,
			ParamNameDescription:    config.DescriptionParam,
			ParamNameStartTime:      config.StartTimeParam,
			ParamNameDuration:       config.DurationParam,
			ParamNameMethodOverride: config.MethodOverrideParam,
		}

		baseHandler.RenderView(httpResponseWriter, httpRequest, config.TemplateEvents, viewData)
	}
}
