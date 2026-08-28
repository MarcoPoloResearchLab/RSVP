// Package horizon provides the authenticated horizon projection handler.
package horizon

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	gaussConstants "github.com/tyemirov/GAuss/pkg/constants"
	"github.com/tyemirov/GAuss/pkg/session"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

const (
	horizonCacheControl               = "private, no-store"
	horizonAuthenticationErrorCode    = "authentication_required"
	horizonAuthenticationErrorMessage = "Authentication is required."
	horizonTimezoneErrorCode          = "organizer_timezone_required"
	horizonTimezoneErrorMessage       = "The organizer timezone is required."
	horizonMethodErrorCode            = "method_not_allowed"
	horizonRepresentationErrorCode    = "representation_not_acceptable"
	horizonNotFoundErrorCode          = "horizon_not_found"
	horizonInternalErrorCode          = "horizon_projection_failed"
	horizonInternalErrorMessage       = "The Horizon projection failed."
	horizonErrorCode                  = "invalid_time_window"
	horizonErrorMessage               = "The time window is invalid."
	horizonJSONMediaType              = handlers.JSONMediaType
	horizonHTMLMediaType              = "text/html"
)

var errMalformedHorizonWindow = errors.New("malformed horizon window")

type handler struct {
	baseHandler *handlers.BaseHttpHandler
	projector   *services.HorizonProjectionService
	now         func() time.Time
}

type headResponseWriter struct {
	http.ResponseWriter
}

func (writer headResponseWriter) Write(body []byte) (int, error) {
	return len(body), nil
}

// Handler returns the authenticated horizon HTTP handler.
func Handler(applicationContext *config.ApplicationContext, now func() time.Time) http.Handler {
	baseHandler := handlers.NewBaseHttpHandler(applicationContext, config.ResourceNameHorizon, config.WebHorizon)
	projector, projectionServiceError := services.NewHorizonProjectionService(applicationContext.Database)
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if projectionServiceError != nil || now == nil {
			writeTypedError(applicationContext, responseWriter, http.StatusInternalServerError, horizonInternalErrorCode, horizonInternalErrorMessage, projectionServiceError)
			return
		}
		(&handler{baseHandler: &baseHandler, projector: projector, now: now}).serveHTTP(responseWriter, request)
	})
}

// AuthenticationMiddleware enforces the horizon authentication response contract.
func AuthenticationMiddleware(applicationContext *config.ApplicationContext, next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		webSession, sessionError := session.Store().Get(request, gaussConstants.SessionName)
		if sessionError == nil {
			userEmail, emailFound := webSession.Values[gaussConstants.SessionKeyUserEmail].(string)
			if emailFound && userEmail != "" {
				next.ServeHTTP(responseWriter, request)
				return
			}
		}

		setProjectionHeaders(responseWriter.Header())
		representation, representationAccepted := selectRepresentation(request.Header.Get("Accept"))
		if representationAccepted && representation == horizonHTMLMediaType {
			http.Redirect(responseWriter, request, gaussConstants.LoginPath, http.StatusFound)
			return
		}
		if responseError := handlers.WriteTypedError(
			responseWriter,
			http.StatusUnauthorized,
			horizonAuthenticationErrorCode,
			horizonAuthenticationErrorMessage,
		); responseError != nil {
			applicationContext.Logger.Printf("ERROR: Write horizon authentication response: %v", responseError)
		}
	})
}

func (horizonHandler *handler) serveHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	setProjectionHeaders(responseWriter.Header())
	if request.URL.Path != config.WebHorizon {
		horizonHandler.writeError(responseWriter, http.StatusNotFound, horizonNotFoundErrorCode, "Horizon was not found.", nil)
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		responseWriter.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		horizonHandler.writeError(responseWriter, http.StatusMethodNotAllowed, horizonMethodErrorCode, http.StatusText(http.StatusMethodNotAllowed), nil)
		return
	}
	if request.Method == http.MethodHead {
		responseWriter = headResponseWriter{ResponseWriter: responseWriter}
	}

	representation, representationAccepted := selectRepresentation(request.Header.Get("Accept"))
	if !representationAccepted {
		horizonHandler.writeError(responseWriter, http.StatusNotAcceptable, horizonRepresentationErrorCode, http.StatusText(http.StatusNotAcceptable), nil)
		return
	}

	currentUser, userFound := request.Context().Value(middleware.ContextKeyUser).(*models.User)
	if !userFound || currentUser == nil || currentUser.ID == "" {
		horizonHandler.writeError(responseWriter, http.StatusUnauthorized, horizonAuthenticationErrorCode, horizonAuthenticationErrorMessage, nil)
		return
	}
	window, windowError := horizonHandler.windowFromRequest(request, currentUser)
	if windowError != nil {
		if errors.Is(windowError, models.ErrOrganizerTimezoneRequired) {
			if representation == horizonHTMLMediaType {
				responseWriter.Header().Set("Content-Type", horizonHTMLMediaType+"; charset=utf-8")
				horizonHandler.baseHandler.RenderView(responseWriter, request, config.TemplateHorizon, newHorizonSetupViewData())
				return
			}
			horizonHandler.writeTimezoneRequired(responseWriter, windowError)
			return
		}
		statusCode := http.StatusUnprocessableEntity
		if errors.Is(windowError, errMalformedHorizonWindow) {
			statusCode = http.StatusBadRequest
		}
		horizonHandler.writeInvalidWindow(responseWriter, statusCode, windowError)
		return
	}
	projection, projectionError := horizonHandler.projector.Project(request.Context(), currentUser.ID, window)
	if projectionError != nil {
		horizonHandler.writeError(responseWriter, http.StatusInternalServerError, horizonInternalErrorCode, horizonInternalErrorMessage, projectionError)
		return
	}

	if representation == horizonJSONMediaType {
		responseWriter.Header().Set("Content-Type", horizonJSONMediaType)
		if encodeError := json.NewEncoder(responseWriter).Encode(projection); encodeError != nil {
			horizonHandler.baseHandler.ApplicationContext.Logger.Printf("ERROR: Encode horizon projection for organizer %s: %v", currentUser.ID, encodeError)
		}
		return
	}
	viewData, viewDataError := newHorizonViewData(projection, horizonHandler.now())
	if viewDataError != nil {
		horizonHandler.writeError(responseWriter, http.StatusInternalServerError, horizonInternalErrorCode, horizonInternalErrorMessage, viewDataError)
		return
	}
	var connection models.CalendarConnection
	connectionError := horizonHandler.baseHandler.ApplicationContext.Database.WithContext(request.Context()).First(&connection, "organizer_id = ?", currentUser.ID).Error
	if connectionError == nil {
		viewData.CalendarConnection = &horizonConnectionView{
			ID: connection.ID, Status: connection.Status,
			ManagementURL:     config.WebCalendarConnections + connection.ID,
			SourceCalendarURL: config.WebCalendarConnections + connection.ID + "/source-calendars/",
		}
		if syncStateError := loadConnectionSyncState(horizonHandler.baseHandler.ApplicationContext.Database.WithContext(request.Context()), connection.ID, viewData.CalendarConnection); syncStateError != nil {
			horizonHandler.writeError(responseWriter, http.StatusInternalServerError, horizonInternalErrorCode, horizonInternalErrorMessage, syncStateError)
			return
		}
	} else if !errors.Is(connectionError, gorm.ErrRecordNotFound) {
		horizonHandler.writeError(responseWriter, http.StatusInternalServerError, horizonInternalErrorCode, horizonInternalErrorMessage, connectionError)
		return
	}
	var drafts []models.IngestionDraft
	if draftsError := horizonHandler.baseHandler.ApplicationContext.Database.WithContext(request.Context()).Preload("Calendar").Preload("DerivedMarkerRules").Where("organizer_id = ? AND status IN ?", currentUser.ID, []models.IngestionDraftStatus{models.IngestionDraftReady, models.IngestionDraftIncomplete}).Order("created_at ASC").Find(&drafts).Error; draftsError != nil {
		horizonHandler.writeError(responseWriter, http.StatusInternalServerError, horizonInternalErrorCode, horizonInternalErrorMessage, draftsError)
		return
	}
	for draftIndex := range drafts {
		draft := &drafts[draftIndex]
		location, locationError := time.LoadLocation(draft.Timezone)
		if locationError != nil {
			horizonHandler.writeError(responseWriter, http.StatusInternalServerError, horizonInternalErrorCode, horizonInternalErrorMessage, locationError)
			return
		}
		missingFields, missingError := draft.MissingFields()
		if missingError != nil {
			horizonHandler.writeError(responseWriter, http.StatusInternalServerError, horizonInternalErrorCode, horizonInternalErrorMessage, missingError)
			return
		}
		draftView := horizonDraftView{ID: draft.ID, Status: draft.Status, Mode: draft.Mode, Source: draft.Source, CalendarID: draft.CalendarID, CalendarName: draft.Calendar.Name, Title: draft.Title, ReferenceTime: draft.ReferenceTime.In(location).Format("2006-01-02T15:04"), Timezone: draft.Timezone, ManagementURL: config.WebIngestionDrafts + draft.ID, ConfirmationURL: config.WebIngestionDrafts + draft.ID + "/confirmations/", ProposedLane: "new lane", MissingFields: missingFields, DerivedRuleSummaries: make([]string, 0, len(draft.DerivedMarkerRules))}
		for ruleIndex := range draft.DerivedMarkerRules {
			rule := draft.DerivedMarkerRules[ruleIndex]
			draftView.DerivedRuleSummaries = append(draftView.DerivedRuleSummaries, fmt.Sprintf("%d seconds from %s", rule.OffsetSeconds, rule.AnchorEdge))
		}
		if draft.AnchorEventID != nil {
			draftView.AnchorEventID = *draft.AnchorEventID
			draftView.ProposedLane = "anchor event lane"
		}
		if draft.StartsAt != nil {
			draftView.StartsAt = draft.StartsAt.In(location).Format("2006-01-02T15:04")
		}
		if draft.EndsAt != nil {
			draftView.EndsAt = draft.EndsAt.In(location).Format("2006-01-02T15:04")
		}
		if draft.ReviewIntervalSeconds != nil {
			draftView.ReviewIntervalSeconds = strconv.FormatInt(*draft.ReviewIntervalSeconds, 10)
		}
		if draft.NextProbeAt != nil {
			draftView.NextProbeAt = draft.NextProbeAt.In(location).Format("2006-01-02T15:04")
		}
		if draft.EscalationIntervalSeconds != nil {
			draftView.EscalationIntervalSeconds = strconv.FormatInt(*draft.EscalationIntervalSeconds, 10)
		}
		viewData.IngestionDraftViews = append(viewData.IngestionDraftViews, draftView)
	}
	responseWriter.Header().Set("Content-Type", horizonHTMLMediaType+"; charset=utf-8")
	horizonHandler.baseHandler.RenderView(responseWriter, request, config.TemplateHorizon, viewData)
}

func loadConnectionSyncState(database *gorm.DB, connectionID string, connectionView *horizonConnectionView) error {
	var synchronizations []models.CalendarSync
	queryError := database.Model(&models.CalendarSync{}).
		Joins("JOIN source_calendar_mappings ON source_calendar_mappings.id = calendar_syncs.mapping_id AND source_calendar_mappings.deleted_at IS NULL").
		Where("source_calendar_mappings.connection_id = ?", connectionID).
		Order("calendar_syncs.started_at DESC").
		Find(&synchronizations).Error
	if queryError != nil {
		return fmt.Errorf("read synchronization state for calendar connection %s: %w", connectionID, queryError)
	}
	seenMappings := make(map[string]bool)
	for synchronizationIndex := range synchronizations {
		synchronization := &synchronizations[synchronizationIndex]
		if synchronization.State == models.CalendarSyncSucceeded && synchronization.FinishedAt != nil {
			formattedTime := synchronization.FinishedAt.UTC().Format(time.RFC3339)
			if connectionView.LastSuccessfulSync == "" || formattedTime > connectionView.LastSuccessfulSync {
				connectionView.LastSuccessfulSync = formattedTime
			}
		}
		if seenMappings[synchronization.MappingID] {
			continue
		}
		seenMappings[synchronization.MappingID] = true
		switch synchronization.State {
		case models.CalendarSyncFailed:
			connectionView.SyncState = models.CalendarSyncFailed
			connectionView.SyncError = true
		case models.CalendarSyncRunning:
			if !connectionView.SyncError {
				connectionView.SyncState = models.CalendarSyncRunning
			}
		case models.CalendarSyncPending:
			if connectionView.SyncState == "" || connectionView.SyncState == models.CalendarSyncSucceeded {
				connectionView.SyncState = models.CalendarSyncPending
			}
		case models.CalendarSyncSucceeded:
			if connectionView.SyncState == "" {
				connectionView.SyncState = models.CalendarSyncSucceeded
			}
		}
	}
	return nil
}

func (horizonHandler *handler) writeError(responseWriter http.ResponseWriter, statusCode int, code string, message string, cause error) {
	writeTypedError(horizonHandler.baseHandler.ApplicationContext, responseWriter, statusCode, code, message, cause)
}

func writeTypedError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, statusCode int, code string, message string, cause error) {
	if cause != nil {
		applicationContext.Logger.Printf("ERROR: %s: %v", message, cause)
	}
	if responseError := handlers.WriteTypedError(responseWriter, statusCode, code, message); responseError != nil {
		applicationContext.Logger.Printf("ERROR: Write Horizon error response: %v", responseError)
	}
}

func (horizonHandler *handler) writeTimezoneRequired(responseWriter http.ResponseWriter, timezoneError error) {
	if responseError := handlers.WriteTypedError(responseWriter, http.StatusUnprocessableEntity, horizonTimezoneErrorCode, horizonTimezoneErrorMessage); responseError != nil {
		horizonHandler.baseHandler.ApplicationContext.Logger.Printf("ERROR: Write organizer timezone response: %v", errors.Join(timezoneError, responseError))
	}
}

func (horizonHandler *handler) windowFromRequest(request *http.Request, currentUser *models.User) (services.HorizonWindow, error) {
	if currentUser.Timezone == nil {
		return services.HorizonWindow{}, models.ErrOrganizerTimezoneRequired
	}
	timezone, timezoneError := models.NewTimezone(*currentUser.Timezone)
	if timezoneError != nil {
		return services.HorizonWindow{}, timezoneError
	}
	query := request.URL.Query()
	startValues, startPresent := query[config.HorizonStartParam]
	endValues, endPresent := query[config.HorizonEndParam]
	if !startPresent && !endPresent {
		return services.NewDefaultHorizonWindow(horizonHandler.now(), timezone)
	}
	if !startPresent || !endPresent || len(startValues) != 1 || len(endValues) != 1 || startValues[0] == "" || endValues[0] == "" {
		return services.HorizonWindow{}, errMalformedHorizonWindow
	}
	start, startError := time.Parse(time.RFC3339, startValues[0])
	if startError != nil {
		return services.HorizonWindow{}, errMalformedHorizonWindow
	}
	end, endError := time.Parse(time.RFC3339, endValues[0])
	if endError != nil {
		return services.HorizonWindow{}, errMalformedHorizonWindow
	}
	return services.NewHorizonWindow(start, end, timezone)
}

func (horizonHandler *handler) writeInvalidWindow(responseWriter http.ResponseWriter, statusCode int, windowError error) {
	if responseError := handlers.WriteTypedError(responseWriter, statusCode, horizonErrorCode, horizonErrorMessage); responseError != nil {
		horizonHandler.baseHandler.ApplicationContext.Logger.Printf("ERROR: Write invalid horizon window response: %v", errors.Join(windowError, responseError))
	}
}

func setProjectionHeaders(responseHeaders http.Header) {
	responseHeaders.Set("Cache-Control", horizonCacheControl)
	responseHeaders.Set("Vary", "Accept")
}

func selectRepresentation(acceptHeader string) (string, bool) {
	if strings.TrimSpace(acceptHeader) == "" {
		return horizonHTMLMediaType, true
	}
	selectedType := ""
	selectedQuality := -1.0
	for _, acceptValue := range strings.Split(acceptHeader, ",") {
		mediaType, parameters, parseError := mime.ParseMediaType(strings.TrimSpace(acceptValue))
		if parseError != nil {
			continue
		}
		quality := 1.0
		if qualityValue, qualityFound := parameters["q"]; qualityFound {
			parsedQuality, qualityError := strconv.ParseFloat(qualityValue, 64)
			if qualityError != nil || parsedQuality < 0 || parsedQuality > 1 {
				continue
			}
			quality = parsedQuality
		}
		candidate := ""
		switch mediaType {
		case horizonJSONMediaType:
			candidate = horizonJSONMediaType
		case horizonHTMLMediaType, "*/*":
			candidate = horizonHTMLMediaType
		}
		if candidate != "" && quality > 0 && quality > selectedQuality {
			selectedType = candidate
			selectedQuality = quality
		}
	}
	return selectedType, selectedType != ""
}
