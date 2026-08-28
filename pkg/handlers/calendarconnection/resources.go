// Package calendarconnection provides authenticated Google Calendar connection resources.
package calendarconnection

import (
	"context"
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
	return NewWithServices(applicationContext, service, syncService)
}

// NewWithServices constructs connection resources with existing services.
func NewWithServices(applicationContext *config.ApplicationContext, service *services.CalendarConnectionService, syncService *services.CalendarSyncService) (*Resources, error) {
	if applicationContext == nil || service == nil || syncService == nil {
		return nil, errors.New("calendar connection HTTP dependencies are required")
	}
	return &Resources{applicationContext: applicationContext, service: service, syncService: syncService}, nil
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
		responseWriter.Header().Set("Content-Security-Policy", "default-src 'none'; connect-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		responseWriter.Header().Set("Referrer-Policy", "no-referrer")
		responseWriter.Header().Set("X-Content-Type-Options", "nosniff")
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
		if executeError := calendarCallbackTemplate.Execute(responseWriter, confirmation); executeError != nil {
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
	if synchronizationError := resources.syncService.SynchronizeMappings(request.Context(), currentUser.ID, mappings); synchronizationError != nil {
		writeError(resources.applicationContext, responseWriter, http.StatusBadGateway, "calendar_synchronization_failed", "Google Calendar synchronization failed. RSVP will retry automatically.", synchronizationError)
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

// SynchronizeAll reconciles every selected source calendar.
func (resources *Resources) SynchronizeAll(ctx context.Context) error {
	return resources.syncService.SynchronizeAll(ctx)
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

var calendarCallbackTemplate = template.Must(template.New("calendar callback").Parse(calendarCallbackHTML))

const calendarCallbackHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="referrer" content="no-referrer">
<title>Confirm Google Calendar · RSVP</title>
<style>
:root {
  color-scheme: light;
  --ink: #18231f;
  --muted: #65716c;
  --paper: #f6f4ed;
  --panel: #fffdf7;
  --line: #d9ddd7;
  --accent: #176b52;
  --accent-soft: #e6f0eb;
  --danger: #a33b2f;
}
* { box-sizing: border-box; }
html { min-height: 100%; }
body {
  background: var(--paper);
  color: var(--ink);
  font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  margin: 0;
  min-height: 100vh;
}
.shell {
  margin: 0 auto;
  max-width: 60rem;
  padding: 1.25rem clamp(1rem, 4vw, 2.5rem) 3rem;
}
.masthead {
  align-items: center;
  border-bottom: 1px solid var(--line);
  display: flex;
  justify-content: space-between;
  min-height: 3.25rem;
}
.wordmark {
  color: var(--ink);
  font-size: 0.92rem;
  font-weight: 800;
  letter-spacing: 0.16em;
  text-decoration: none;
}
.context {
  color: var(--muted);
  font-size: 0.72rem;
}
.confirmation {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 0.75rem;
  box-shadow: 0 1.25rem 3rem rgba(35, 47, 42, 0.08);
  margin: clamp(2rem, 8vh, 5rem) auto 0;
  max-width: 34rem;
  overflow: hidden;
}
.confirmation-main { padding: clamp(1.4rem, 5vw, 2rem); }
.status-chip {
  align-items: center;
  background: var(--accent-soft);
  border: 1px solid #b8d4c7;
  border-radius: 999px;
  color: var(--accent);
  display: inline-flex;
  font-size: 0.7rem;
  font-weight: 750;
  gap: 0.42rem;
  letter-spacing: 0.04em;
  padding: 0.3rem 0.55rem;
  text-transform: uppercase;
}
.status-chip::before {
  background: var(--accent);
  border-radius: 50%;
  content: "";
  height: 0.45rem;
  width: 0.45rem;
}
h1 {
  font-size: clamp(1.6rem, 5vw, 2.2rem);
  letter-spacing: -0.045em;
  line-height: 1.05;
  margin: 1rem 0 0.7rem;
}
.summary {
  color: var(--muted);
  font-size: 0.9rem;
  line-height: 1.55;
  margin: 0;
}
.access {
  border: 1px solid var(--line);
  border-radius: 0.55rem;
  margin: 1.3rem 0 0;
  padding: 0.85rem 1rem;
}
.access h2 {
  font-size: 0.72rem;
  letter-spacing: 0.08em;
  margin: 0 0 0.55rem;
  text-transform: uppercase;
}
.access ul {
  color: var(--muted);
  display: grid;
  font-size: 0.82rem;
  gap: 0.4rem;
  list-style: none;
  margin: 0;
  padding: 0;
}
.access li::before {
  color: var(--accent);
  content: "✓";
  font-weight: 800;
  margin-right: 0.5rem;
}
.actions {
  align-items: center;
  background: #f1efe8;
  border-top: 1px solid var(--line);
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  justify-content: flex-end;
  padding: 0.9rem clamp(1.4rem, 5vw, 2rem);
}
button {
  border: 1px solid transparent;
  border-radius: 0.4rem;
  cursor: pointer;
  font: inherit;
  font-size: 0.82rem;
  font-weight: 700;
  min-height: 2.5rem;
  padding: 0.5rem 0.85rem;
}
button:focus-visible { outline: 3px solid rgba(23, 107, 82, 0.28); outline-offset: 2px; }
.secondary { background: transparent; border-color: var(--line); color: var(--ink); }
.primary { background: var(--ink); color: white; }
.primary:hover { background: var(--accent); }
button:disabled { cursor: wait; opacity: 0.65; }
.result {
  color: var(--muted);
  flex-basis: 100%;
  font-size: 0.76rem;
  margin: 0;
  min-height: 1.15rem;
  text-align: right;
}
.result[data-state="error"] { color: var(--danger); }
@media (max-width: 34rem) {
  .shell { padding-inline: 0.75rem; }
  .context { display: none; }
  .confirmation { margin-top: 1.25rem; }
  .actions { align-items: stretch; flex-direction: column-reverse; }
  .actions button { width: 100%; }
  .result { text-align: left; }
}
@media (prefers-reduced-motion: reduce) { * { scroll-behavior: auto !important; } }
</style>
</head>
<body>
<div class="shell">
  <header class="masthead">
    <span class="wordmark" aria-label="RSVP">RSVP</span>
    <span class="context">Calendar connection</span>
  </header>
  <main>
    <section class="confirmation" data-calendar-confirmation aria-labelledby="confirmation-title">
      <div class="confirmation-main">
        <span class="status-chip">Consent verified</span>
        <h1 id="confirmation-title">Confirm Google Calendar</h1>
        <p class="summary">Google returned you to RSVP. Create the connection to finish adding your calendars.</p>
        <section class="access" aria-labelledby="access-title">
          <h2 id="access-title">Read-only access</h2>
          <ul>
            <li>View your calendar list</li>
            <li>View calendar events</li>
          </ul>
        </section>
      </div>
      <div class="actions">
        <button class="secondary" id="cancel" type="button">Back to Horizon</button>
        <button class="primary" id="confirm" type="button" data-request-id="{{.RequestID}}">Create connection</button>
        <p class="result" id="status" role="status" aria-live="polite"></p>
      </div>
    </section>
  </main>
</div>
<script>
(() => {
  const button = document.getElementById('confirm');
  const cancel = document.getElementById('cancel');
  const status = document.getElementById('status');
  const query = new URLSearchParams(window.location.search);
  const state = query.get('state');
  const code = query.get('code');
  const idempotencyKey = crypto.randomUUID();

  cancel.addEventListener('click', () => window.location.replace('/horizon/'));
  button.addEventListener('click', async () => {
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
    status.dataset.state = 'working';
    status.textContent = 'Creating connection…';
    try {
      const response = await fetch('/calendar-connections/', {
        method: 'POST',
        credentials: 'same-origin',
        headers: {'Content-Type': 'application/json', 'Idempotency-Key': idempotencyKey},
        body: JSON.stringify({request_id: button.dataset.requestId, state, code})
      });
      if (!response.ok) throw new Error('connection request failed');
      status.textContent = 'Connection created. Returning to Horizon…';
      window.location.replace('/horizon/');
    } catch (_) {
      button.disabled = false;
      button.removeAttribute('aria-busy');
      status.dataset.state = 'error';
      status.textContent = 'RSVP could not create the connection. Try again.';
    }
  });
})();
</script>
</body>
</html>`
