// Package calendarconnection provides authenticated Google Calendar connection resources.
package calendarconnection

import (
	"crypto/rand"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/providers/googlecalendar"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

const requestBodyBytes = 8192

// Resources contains the Google Calendar connection HTTP resources.
type Resources struct {
	applicationContext *config.ApplicationContext
	service            *services.CalendarConnectionService
	syncService        *services.CalendarSyncService
}

// New constructs the Google Calendar connection resources.
func New(applicationContext *config.ApplicationContext, environmentConfig config.EnvConfig, now func() time.Time) (*Resources, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	adapterConfig := googlecalendar.DefaultConfig(environmentConfig.GoogleClientID, environmentConfig.GoogleClientSecret)
	if environmentConfig.GoogleCalendarAuthorizationEndpoint != "" {
		adapterConfig.AuthorizationEndpoint = environmentConfig.GoogleCalendarAuthorizationEndpoint
	}
	if environmentConfig.GoogleCalendarTokenEndpoint != "" {
		adapterConfig.TokenEndpoint = environmentConfig.GoogleCalendarTokenEndpoint
	}
	if environmentConfig.GoogleCalendarListEndpoint != "" {
		adapterConfig.CalendarListEndpoint = environmentConfig.GoogleCalendarListEndpoint
	}
	if environmentConfig.GoogleCalendarEventsEndpoint != "" {
		adapterConfig.EventsEndpoint = environmentConfig.GoogleCalendarEventsEndpoint
	}
	adapter, adapterError := googlecalendar.New(adapterConfig, httpClient, now)
	if adapterError != nil {
		return nil, adapterError
	}
	credentialCipher, cipherError := services.NewCredentialCipher(environmentConfig.CalendarCredentialEncryptionKey, rand.Reader)
	if cipherError != nil {
		return nil, cipherError
	}
	service, serviceError := services.NewCalendarConnectionService(applicationContext.Database, adapter, credentialCipher, now, rand.Reader)
	if serviceError != nil {
		return nil, serviceError
	}
	syncService, syncServiceError := services.NewCalendarSyncService(applicationContext.Database, adapter, credentialCipher, now)
	if syncServiceError != nil {
		return nil, syncServiceError
	}
	return &Resources{applicationContext: applicationContext, service: service, syncService: syncService}, nil
}

// CalendarSyncs returns the synchronization collection and item handler.
func (resources *Resources) CalendarSyncs() http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		setPrivateHeaders(responseWriter)
		currentUser, userFound := currentOrganizer(request)
		if !userFound {
			writeError(resources.applicationContext, responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		if resources.syncService == nil {
			writeError(resources.applicationContext, responseWriter, http.StatusServiceUnavailable, "calendar_sync_unavailable", "Calendar synchronization is unavailable.", nil)
			return
		}
		if request.URL.Path == config.WebCalendarSyncs {
			if request.Method != http.MethodPost {
				responseWriter.Header().Set("Allow", http.MethodPost)
				writeError(resources.applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
				return
			}
			var body struct {
				MappingID string `json:"mapping_id"`
			}
			if decodeError := handlers.DecodeJSON(responseWriter, request, requestBodyBytes, &body); decodeError != nil {
				writeDecodeError(resources.applicationContext, responseWriter, decodeError)
				return
			}
			synchronization, reused, createError := resources.syncService.Create(request.Context(), currentUser.ID, body.MappingID, request.Header.Get(config.IdempotencyKeyHeader))
			if createError != nil {
				writeServiceError(resources.applicationContext, responseWriter, createError)
				return
			}
			statusCode := http.StatusAccepted
			if reused {
				statusCode = http.StatusOK
			}
			responseWriter.Header().Set("Location", config.WebCalendarSyncs+synchronization.ID)
			if encodeError := handlers.WriteJSON(responseWriter, statusCode, synchronization); encodeError != nil {
				resources.applicationContext.Logger.Printf("ERROR: Write calendar synchronization response: %v", encodeError)
			}
			return
		}
		syncID := strings.Trim(strings.TrimPrefix(request.URL.Path, config.WebCalendarSyncs), "/")
		if syncID == "" || strings.Contains(syncID, "/") {
			http.NotFound(responseWriter, request)
			return
		}
		if request.Method != http.MethodGet {
			responseWriter.Header().Set("Allow", http.MethodGet)
			writeError(resources.applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
			return
		}
		synchronization, readError := resources.syncService.Read(request.Context(), currentUser.ID, syncID)
		if readError != nil {
			writeServiceError(resources.applicationContext, responseWriter, readError)
			return
		}
		if encodeError := handlers.WriteJSON(responseWriter, http.StatusOK, synchronization); encodeError != nil {
			resources.applicationContext.Logger.Printf("ERROR: Write calendar synchronization response: %v", encodeError)
		}
	})
}

// NewWithService constructs connection resources with an existing service.
func NewWithService(applicationContext *config.ApplicationContext, service *services.CalendarConnectionService) (*Resources, error) {
	if applicationContext == nil || service == nil {
		return nil, errors.New("calendar connection HTTP dependencies are required")
	}
	return &Resources{applicationContext: applicationContext, service: service}, nil
}

// AuthorizationRequests returns the consent request collection handler.
func (resources *Resources) AuthorizationRequests() http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		setPrivateHeaders(responseWriter)
		currentUser, userFound := currentOrganizer(request)
		if !userFound {
			writeError(resources.applicationContext, responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		if request.URL.Path != config.WebCalendarAuthorizationRequests {
			http.NotFound(responseWriter, request)
			return
		}
		if request.Method != http.MethodPost {
			responseWriter.Header().Set("Allow", http.MethodPost)
			writeError(resources.applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
			return
		}
		var body struct{}
		if decodeError := handlers.DecodeJSON(responseWriter, request, requestBodyBytes, &body); decodeError != nil {
			writeDecodeError(resources.applicationContext, responseWriter, decodeError)
			return
		}
		callbackURL, callbackError := resolveCallbackURL(resources.applicationContext.AppBaseURL)
		if callbackError != nil {
			writeError(resources.applicationContext, responseWriter, http.StatusInternalServerError, "calendar_authorization_unavailable", "Calendar authorization is unavailable.", callbackError)
			return
		}
		start, createError := resources.service.CreateAuthorizationRequest(request.Context(), currentUser.ID, callbackURL)
		if createError != nil {
			writeServiceError(resources.applicationContext, responseWriter, createError)
			return
		}
		responseWriter.Header().Set("Location", config.WebCalendarAuthorizationRequests+start.RequestID)
		if encodeError := handlers.WriteJSON(responseWriter, http.StatusCreated, start); encodeError != nil {
			resources.applicationContext.Logger.Printf("ERROR: Write calendar authorization response: %v", encodeError)
		}
	})
}

// Callback returns the consent callback handler without a database change.
func (resources *Resources) Callback() http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Cache-Control", "no-store")
		responseWriter.Header().Set("Referrer-Policy", "no-referrer")
		currentUser, userFound := currentOrganizer(request)
		if !userFound {
			writeError(resources.applicationContext, responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		if request.URL.Path != config.WebCalendarConnectionCallbacksGoogle {
			http.NotFound(responseWriter, request)
			return
		}
		if request.Method != http.MethodGet {
			responseWriter.Header().Set("Allow", http.MethodGet)
			writeError(resources.applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
			return
		}
		confirmation, validationError := resources.service.ValidateCallback(request.Context(), currentUser.ID, request.URL.Query().Get("state"), request.URL.Query().Get("code"))
		if validationError != nil {
			writeServiceError(resources.applicationContext, responseWriter, validationError)
			return
		}
		if strings.Contains(request.Header.Get("Accept"), handlers.JSONMediaType) {
			if encodeError := handlers.WriteJSON(responseWriter, http.StatusOK, confirmation); encodeError != nil {
				resources.applicationContext.Logger.Printf("ERROR: Write calendar callback response: %v", encodeError)
			}
			return
		}
		responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
		callbackTemplate := template.Must(template.New("callback").Parse(calendarCallbackHTML))
		if executeError := callbackTemplate.Execute(responseWriter, confirmation); executeError != nil {
			resources.applicationContext.Logger.Printf("ERROR: Render calendar callback: %v", executeError)
		}
	})
}

// Connections returns the connection and selected source calendar handler.
func (resources *Resources) Connections() http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		setPrivateHeaders(responseWriter)
		currentUser, userFound := currentOrganizer(request)
		if !userFound {
			writeError(resources.applicationContext, responseWriter, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		if request.URL.Path == config.WebCalendarConnections {
			if request.Method != http.MethodPost {
				responseWriter.Header().Set("Allow", http.MethodPost)
				writeError(resources.applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
				return
			}
			resources.createConnection(currentUser, responseWriter, request)
			return
		}
		connectionID, sourceCollection, pathValid := connectionPath(request.URL.Path)
		if !pathValid {
			http.NotFound(responseWriter, request)
			return
		}
		if sourceCollection {
			switch request.Method {
			case http.MethodGet:
				resources.listSources(currentUser.ID, connectionID, responseWriter, request)
			case http.MethodPut:
				resources.replaceSources(currentUser, connectionID, responseWriter, request)
			default:
				responseWriter.Header().Set("Allow", http.MethodGet+", "+http.MethodPut)
				writeError(resources.applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
			}
			return
		}
		switch request.Method {
		case http.MethodGet:
			connection, readError := resources.service.ReadConnection(request.Context(), currentUser.ID, connectionID)
			if readError != nil {
				writeServiceError(resources.applicationContext, responseWriter, readError)
				return
			}
			writeConnection(resources.applicationContext, responseWriter, http.StatusOK, connection)
		case http.MethodDelete:
			if deleteError := resources.service.DeleteConnection(request.Context(), currentUser.ID, connectionID); deleteError != nil {
				writeServiceError(resources.applicationContext, responseWriter, deleteError)
				return
			}
			responseWriter.WriteHeader(http.StatusNoContent)
		default:
			responseWriter.Header().Set("Allow", http.MethodGet+", "+http.MethodDelete)
			writeError(resources.applicationContext, responseWriter, http.StatusMethodNotAllowed, "method_not_allowed", http.StatusText(http.StatusMethodNotAllowed), nil)
		}
	})
}

func (resources *Resources) createConnection(currentUser *models.User, responseWriter http.ResponseWriter, request *http.Request) {
	var body services.AuthorizationConfirmation
	if decodeError := handlers.DecodeJSON(responseWriter, request, requestBodyBytes, &body); decodeError != nil {
		writeDecodeError(resources.applicationContext, responseWriter, decodeError)
		return
	}
	connection, reused, createError := resources.service.CreateConnection(request.Context(), currentUser.ID, body, request.Header.Get(config.IdempotencyKeyHeader))
	if createError != nil {
		writeServiceError(resources.applicationContext, responseWriter, createError)
		return
	}
	statusCode := http.StatusCreated
	if reused {
		statusCode = http.StatusOK
	}
	responseWriter.Header().Set("Location", config.WebCalendarConnections+connection.ID)
	writeConnection(resources.applicationContext, responseWriter, statusCode, connection)
}

func (resources *Resources) listSources(organizerID string, connectionID string, responseWriter http.ResponseWriter, request *http.Request) {
	available, listError := resources.service.ListSourceCalendars(request.Context(), organizerID, connectionID)
	if listError != nil {
		writeServiceError(resources.applicationContext, responseWriter, listError)
		return
	}
	connection, readError := resources.service.ReadConnection(request.Context(), organizerID, connectionID)
	if readError != nil {
		writeServiceError(resources.applicationContext, responseWriter, readError)
		return
	}
	selected := make(map[string]string, len(connection.Mappings))
	for _, mapping := range connection.Mappings {
		selected[mapping.ProviderCalendarID] = mapping.CalendarID
	}
	type sourceRepresentation struct {
		services.ProviderCalendar
		Selected   bool   `json:"selected"`
		CalendarID string `json:"calendar_id,omitempty"`
	}
	response := make([]sourceRepresentation, 0, len(available))
	for _, calendar := range available {
		calendarID, isSelected := selected[calendar.ID]
		response = append(response, sourceRepresentation{ProviderCalendar: calendar, Selected: isSelected, CalendarID: calendarID})
	}
	if encodeError := handlers.WriteJSON(responseWriter, http.StatusOK, map[string]any{"connection_id": connectionID, "sources": response}); encodeError != nil {
		resources.applicationContext.Logger.Printf("ERROR: Write source calendar response: %v", encodeError)
	}
}

func (resources *Resources) replaceSources(currentUser *models.User, connectionID string, responseWriter http.ResponseWriter, request *http.Request) {
	var body struct {
		ProviderCalendarIDs []string `json:"provider_calendar_ids"`
		Timezone            string   `json:"timezone"`
	}
	if decodeError := handlers.DecodeJSON(responseWriter, request, requestBodyBytes, &body); decodeError != nil {
		writeDecodeError(resources.applicationContext, responseWriter, decodeError)
		return
	}
	timezone, timezoneError := models.NewTimezone(body.Timezone)
	if timezoneError != nil {
		writeError(resources.applicationContext, responseWriter, http.StatusUnprocessableEntity, "invalid_timezone", "Timezone is invalid.", timezoneError)
		return
	}
	mappings, replaceError := resources.service.ReplaceSourceCalendars(request.Context(), currentUser, timezone, connectionID, body.ProviderCalendarIDs)
	if replaceError != nil {
		writeServiceError(resources.applicationContext, responseWriter, replaceError)
		return
	}
	type mappingRepresentation struct {
		ID                 string `json:"id"`
		CalendarID         string `json:"calendar_id"`
		ProviderCalendarID string `json:"provider_calendar_id"`
	}
	response := make([]mappingRepresentation, 0, len(mappings))
	for _, mapping := range mappings {
		response = append(response, mappingRepresentation{ID: mapping.ID, CalendarID: mapping.CalendarID, ProviderCalendarID: mapping.ProviderCalendarID})
	}
	if encodeError := handlers.WriteJSON(responseWriter, http.StatusOK, map[string]any{"mappings": response}); encodeError != nil {
		resources.applicationContext.Logger.Printf("ERROR: Write source calendar selection response: %v", encodeError)
	}
}

func writeConnection(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, statusCode int, connection *models.CalendarConnection) {
	value := map[string]any{"id": connection.ID, "provider": connection.Provider, "status": connection.Status}
	if encodeError := handlers.WriteJSON(responseWriter, statusCode, value); encodeError != nil {
		applicationContext.Logger.Printf("ERROR: Write calendar connection response: %v", encodeError)
	}
}

func connectionPath(path string) (string, bool, bool) {
	remainder := strings.TrimPrefix(path, config.WebCalendarConnections)
	if remainder == path || remainder == "" {
		return "", false, false
	}
	if strings.HasSuffix(remainder, "/source-calendars/") {
		identifier := strings.TrimSuffix(remainder, "/source-calendars/")
		return identifier, true, validIdentifier(identifier)
	}
	return remainder, false, validIdentifier(remainder)
}

func validIdentifier(identifier string) bool {
	_, valid := handlers.ResourceIDFromPath(config.WebCalendarConnections, config.WebCalendarConnections+identifier)
	return valid
}

func currentOrganizer(request *http.Request) (*models.User, bool) {
	currentUser, found := request.Context().Value(middleware.ContextKeyUser).(*models.User)
	return currentUser, found && currentUser != nil && currentUser.ID != ""
}

func resolveCallbackURL(baseURL string) (string, error) {
	parsedBase, parseError := url.Parse(baseURL)
	if parseError != nil || parsedBase.Scheme == "" || parsedBase.Host == "" {
		return "", errors.New("application base URL is invalid")
	}
	return parsedBase.ResolveReference(&url.URL{Path: config.CalendarConnectionCallbackPath}).String(), nil
}

func setPrivateHeaders(responseWriter http.ResponseWriter) {
	responseWriter.Header().Set("Cache-Control", "private, no-store")
}

func writeDecodeError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, decodeError error) {
	if errors.Is(decodeError, handlers.ErrUnsupportedJSONMediaType) {
		writeError(applicationContext, responseWriter, http.StatusUnsupportedMediaType, "unsupported_media_type", http.StatusText(http.StatusUnsupportedMediaType), decodeError)
		return
	}
	writeError(applicationContext, responseWriter, http.StatusBadRequest, "malformed_json", "Request JSON is malformed.", decodeError)
}

func writeServiceError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, serviceError error) {
	switch {
	case errors.Is(serviceError, services.ErrResourceForbidden):
		writeError(applicationContext, responseWriter, http.StatusForbidden, "calendar_connection_forbidden", "Calendar connection access is forbidden.", serviceError)
	case errors.Is(serviceError, gorm.ErrRecordNotFound):
		writeError(applicationContext, responseWriter, http.StatusNotFound, "calendar_connection_not_found", "Calendar connection was not found.", serviceError)
	case errors.Is(serviceError, services.ErrIdempotencyKeyRequired):
		writeError(applicationContext, responseWriter, http.StatusBadRequest, "idempotency_key_required", "An Idempotency-Key header is required.", serviceError)
	case errors.Is(serviceError, services.ErrIdempotencyConflict):
		writeError(applicationContext, responseWriter, http.StatusConflict, "idempotency_conflict", "The Idempotency-Key identifies a different request.", serviceError)
	case errors.Is(serviceError, services.ErrCalendarAuthorizationInvalid):
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "calendar_authorization_invalid", "Calendar authorization is invalid or expired.", serviceError)
	case errors.Is(serviceError, services.ErrSourceCalendarSelectionInvalid), errors.Is(serviceError, models.ErrTimezoneInvalid), errors.Is(serviceError, models.ErrTimezoneRequired):
		writeError(applicationContext, responseWriter, http.StatusUnprocessableEntity, "source_calendar_selection_invalid", "Source calendar selection is invalid.", serviceError)
	case errors.Is(serviceError, services.ErrCalendarConnectionHasLocalUse), errors.Is(serviceError, services.ErrSourceOwnedMarker):
		writeError(applicationContext, responseWriter, http.StatusConflict, "source_calendar_conflict", serviceError.Error(), serviceError)
	default:
		writeError(applicationContext, responseWriter, http.StatusBadGateway, "calendar_provider_failed", "Google Calendar did not complete the request.", serviceError)
	}
}

func writeError(applicationContext *config.ApplicationContext, responseWriter http.ResponseWriter, statusCode int, code string, message string, cause error) {
	if cause != nil {
		applicationContext.Logger.Printf("ERROR: %s: %v", message, cause)
	}
	if responseError := handlers.WriteTypedError(responseWriter, statusCode, code, message); responseError != nil {
		applicationContext.Logger.Printf("ERROR: Write calendar connection error response: %v", responseError)
	}
}

const calendarCallbackHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="referrer" content="no-referrer"><title>Confirm Google Calendar</title></head>
<body><main><h1>Confirm Google Calendar</h1><p>RSVP validated the Google consent response.</p>
<button id="confirm" type="button" data-request-id="{{.RequestID}}" data-state="{{.State}}" data-code="{{.Code}}">Create connection</button><p id="status" role="status"></p></main>
<script>
document.getElementById('confirm').addEventListener('click', async () => {
  const button = document.getElementById('confirm');
  const response = await fetch('/calendar-connections/', {method:'POST', credentials:'same-origin', headers:{'Content-Type':'application/json','Idempotency-Key':crypto.randomUUID()}, body:JSON.stringify({request_id:button.dataset.requestId,state:button.dataset.state,code:button.dataset.code})});
  if (!response.ok) { document.getElementById('status').textContent = 'Connection failed.'; return; }
  window.location.assign('/horizon/');
});
</script></body></html>`
