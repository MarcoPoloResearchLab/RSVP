package calendarconnection_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers"
	"github.com/tyemirov/RSVP/pkg/handlers/calendarconnection"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
)

type callbackAdapter struct {
	synchronizationError error
	calendarBatch        *services.ProviderCalendarBatch
	synchronizationCount int
}

func (*callbackAdapter) AuthorizationURL(state string, _ string) (string, error) {
	return "https://provider.example.test/authorize?state=" + url.QueryEscape(state), nil
}
func (*callbackAdapter) ExchangeCode(context.Context, string, string) (services.CalendarProviderCredential, error) {
	return services.CalendarProviderCredential{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Date(2030, time.January, 2, 12, 0, 0, 0, time.UTC)}, nil
}
func (*callbackAdapter) RefreshCredential(_ context.Context, credential services.CalendarProviderCredential) (services.CalendarProviderCredential, error) {
	return credential, nil
}

func (adapter *callbackAdapter) ListCalendars(context.Context, services.CalendarProviderCredential, string) (services.ProviderCalendarBatch, error) {
	if adapter.calendarBatch != nil {
		return *adapter.calendarBatch, nil
	}
	return services.ProviderCalendarBatch{
		Calendars:      []services.ProviderCalendar{{ID: "source", Name: "Source", ColorToken: "source", Readable: true, Visible: true}},
		NextSyncCursor: "calendar-cursor",
	}, nil
}
func (adapter *callbackAdapter) SynchronizeEvents(context.Context, services.CalendarProviderCredential, string, string) (services.ProviderEventBatch, error) {
	adapter.synchronizationCount++
	return services.ProviderEventBatch{NextSyncCursor: "event-cursor"}, adapter.synchronizationError
}

func TestCallbackHasNoStoreNoReferrerAndNoDatabaseChange(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	referenceTime := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	cipher, cipherError := services.NewCredentialCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)), bytes.NewReader(bytes.Repeat([]byte{0x12}, 64)))
	if cipherError != nil {
		testingContext.Fatalf("construct cipher: %v", cipherError)
	}
	service, serviceError := services.NewCalendarConnectionService(fixture.Database, &callbackAdapter{}, cipher, func() time.Time { return referenceTime }, bytes.NewReader(bytes.Repeat([]byte{0x31}, 128)))
	if serviceError != nil {
		testingContext.Fatalf("construct service: %v", serviceError)
	}
	start, startError := service.CreateAuthorizationRequest(context.Background(), owner.ID, "https://rsvp.example.test"+config.WebCalendarConnectionCallbacksGoogle)
	if startError != nil {
		testingContext.Fatalf("create authorization request: %v", startError)
	}
	providerURL, parseError := url.Parse(start.AuthorizationURL)
	if parseError != nil {
		testingContext.Fatalf("parse provider URL: %v", parseError)
	}
	var before models.CalendarAuthorizationRequest
	if findError := fixture.Database.First(&before, "id = ?", start.RequestID).Error; findError != nil {
		testingContext.Fatalf("read authorization request: %v", findError)
	}
	syncService, syncServiceError := services.NewCalendarSyncService(fixture.Database, &callbackAdapter{}, cipher, func() time.Time { return referenceTime })
	if syncServiceError != nil {
		testingContext.Fatalf("construct synchronization service: %v", syncServiceError)
	}
	resources, resourceError := calendarconnection.NewWithServices(fixture.ApplicationContext, service, syncService)
	if resourceError != nil {
		testingContext.Fatalf("construct HTTP resources: %v", resourceError)
	}
	target := config.WebCalendarConnectionCallbacksGoogle + "?state=" + url.QueryEscape(providerURL.Query().Get("state")) + "&code=private-code"
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Accept", "application/json")
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, &owner))
	response := httptest.NewRecorder()
	resources.Callback().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		testingContext.Fatalf("callback status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
		testingContext.Fatalf("callback headers = %v", response.Header())
	}
	if response.Header().Get("Content-Security-Policy") == "" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		testingContext.Fatalf("callback security headers = %v", response.Header())
	}
	var after models.CalendarAuthorizationRequest
	if findError := fixture.Database.First(&after, "id = ?", start.RequestID).Error; findError != nil {
		testingContext.Fatalf("reload authorization request: %v", findError)
	}
	if after.UsedAt != nil || !after.UpdatedAt.Equal(before.UpdatedAt) {
		testingContext.Fatalf("callback changed request: before = %#v, after = %#v", before, after)
	}
	var connectionCount int64
	if countError := fixture.Database.Model(&models.CalendarConnection{}).Count(&connectionCount).Error; countError != nil || connectionCount != 0 {
		testingContext.Fatalf("connection count = %d, error = %v", connectionCount, countError)
	}

	htmlRequest := httptest.NewRequest(http.MethodGet, target, nil)
	htmlRequest = htmlRequest.WithContext(context.WithValue(htmlRequest.Context(), middleware.ContextKeyUser, &owner))
	htmlResponse := httptest.NewRecorder()
	resources.Callback().ServeHTTP(htmlResponse, htmlRequest)
	if htmlResponse.Code != http.StatusOK {
		testingContext.Fatalf("HTML callback status = %d, want %d; body = %s", htmlResponse.Code, http.StatusOK, htmlResponse.Body.String())
	}
	if contentType := htmlResponse.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		testingContext.Fatalf("HTML callback Content-Type = %q", contentType)
	}
	body := htmlResponse.Body.String()
	for _, requiredText := range []string{"data-calendar-confirmation", "data-calendar-task-status", "Consent verified", "Read-only access", "View your calendar list", "View calendar events", "Create connection", "Back to Horizon", "Calendar import task is queued", "#settings/integrations", "resolvedOptions().timeZone"} {
		if !strings.Contains(body, requiredText) {
			testingContext.Fatalf("HTML callback does not contain %q", requiredText)
		}
	}
	for _, forbiddenText := range []string{"cursor: wait", "aria-busy"} {
		if strings.Contains(body, forbiddenText) {
			testingContext.Fatalf("HTML callback contains implicit wait contract %q", forbiddenText)
		}
	}
	if strings.Contains(body, "private-code") || strings.Contains(body, providerURL.Query().Get("state")) {
		testingContext.Fatal("HTML callback reflected OAuth credentials into the document")
	}
}

func TestConnectionCreationQueuesImportAndTaskRecordsSynchronizationFailure(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	referenceTime := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	adapter := &callbackAdapter{synchronizationError: errors.New("provider unavailable")}
	cipher, cipherError := services.NewCredentialCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)), bytes.NewReader(bytes.Repeat([]byte{0x12}, 256)))
	if cipherError != nil {
		testingContext.Fatalf("construct cipher: %v", cipherError)
	}
	service, serviceError := services.NewCalendarConnectionService(fixture.Database, adapter, cipher, func() time.Time { return referenceTime }, bytes.NewReader(bytes.Repeat([]byte{0x31}, 256)))
	if serviceError != nil {
		testingContext.Fatalf("construct connection service: %v", serviceError)
	}
	start, startError := service.CreateAuthorizationRequest(context.Background(), owner.ID, "https://rsvp.example.test"+config.WebCalendarConnectionCallbacksGoogle)
	if startError != nil {
		testingContext.Fatalf("create authorization request: %v", startError)
	}
	providerURL, parseError := url.Parse(start.AuthorizationURL)
	if parseError != nil {
		testingContext.Fatalf("parse authorization URL: %v", parseError)
	}
	confirmation, confirmationError := service.ValidateCallback(context.Background(), owner.ID, providerURL.Query().Get("state"), "code")
	if confirmationError != nil {
		testingContext.Fatalf("validate callback: %v", confirmationError)
	}
	confirmation.Timezone = "Local"
	syncService, syncServiceError := services.NewCalendarSyncService(fixture.Database, adapter, cipher, func() time.Time { return referenceTime })
	if syncServiceError != nil {
		testingContext.Fatalf("construct synchronization service: %v", syncServiceError)
	}
	resources, resourceError := calendarconnection.NewWithServices(fixture.ApplicationContext, service, syncService)
	if resourceError != nil {
		testingContext.Fatalf("construct HTTP resources: %v", resourceError)
	}
	bodyBytes, marshalError := json.Marshal(confirmation)
	if marshalError != nil {
		testingContext.Fatalf("marshal confirmation: %v", marshalError)
	}
	request := httptest.NewRequest(http.MethodPost, config.WebCalendarConnections, bytes.NewReader(bodyBytes))
	request.Header.Set("Content-Type", handlers.JSONMediaType)
	request.Header.Set(config.IdempotencyKeyHeader, "connection-key")
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, &owner))
	response := httptest.NewRecorder()

	resources.Connections().ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		testingContext.Fatalf("connection response = %d, body = %s", response.Code, response.Body.String())
	}
	var connectionBody struct {
		ID   string `json:"id"`
		Task struct {
			ID     string           `json:"id"`
			State  models.TaskState `json:"state"`
			Active bool             `json:"active"`
		} `json:"task"`
	}
	if decodeError := json.Unmarshal(response.Body.Bytes(), &connectionBody); decodeError != nil {
		testingContext.Fatalf("decode connection response: %v", decodeError)
	}
	if connectionBody.ID == "" || connectionBody.Task.ID == "" || connectionBody.Task.State != models.TaskPending || !connectionBody.Task.Active {
		testingContext.Fatalf("connection task response = %#v", connectionBody)
	}
	var storedOwner models.User
	if findError := fixture.Database.First(&storedOwner, "id = ?", owner.ID).Error; findError != nil {
		testingContext.Fatalf("reload organizer: %v", findError)
	}
	if storedOwner.Timezone == nil || *storedOwner.Timezone != "UTC" {
		testingContext.Fatalf("confirmed organizer timezone = %#v", storedOwner.Timezone)
	}
	var connection models.CalendarConnection
	if findError := fixture.Database.First(&connection, "organizer_id = ?", owner.ID).Error; findError != nil {
		testingContext.Fatalf("read connection: %v", findError)
	}
	var mappingCount int64
	if countError := fixture.Database.Model(&models.SourceCalendarMapping{}).
		Joins("JOIN provider_calendar_sync_states ON provider_calendar_sync_states.id = source_calendar_mappings.sync_state_id").
		Where("provider_calendar_sync_states.connection_id = ?", connection.ID).Count(&mappingCount).Error; countError != nil || mappingCount != 0 {
		testingContext.Fatalf("mapping count = %d, error = %v", mappingCount, countError)
	}
	if adapter.synchronizationCount != 0 {
		testingContext.Fatalf("request synchronization count = %d, want 0", adapter.synchronizationCount)
	}
	if updateError := fixture.Database.Model(&models.Task{}).Where("id = ?", connectionBody.Task.ID).Update("scheduled_for", time.Now().UTC().Add(-time.Second)).Error; updateError != nil {
		testingContext.Fatalf("make import task due: %v", updateError)
	}
	resources.RunTaskCycle(context.Background())
	if countError := fixture.Database.Model(&models.SourceCalendarMapping{}).
		Joins("JOIN provider_calendar_sync_states ON provider_calendar_sync_states.id = source_calendar_mappings.sync_state_id").
		Where("provider_calendar_sync_states.connection_id = ?", connection.ID).Count(&mappingCount).Error; countError != nil || mappingCount != 2 {
		testingContext.Fatalf("post-task mapping count = %d, error = %v", mappingCount, countError)
	}
	sourceRequest := httptest.NewRequest(http.MethodGet, config.WebCalendarConnections+connection.ID+"/source-calendars/", nil)
	sourceRequest.Header.Set("Accept", handlers.JSONMediaType)
	sourceRequest = sourceRequest.WithContext(context.WithValue(sourceRequest.Context(), middleware.ContextKeyUser, &owner))
	sourceResponse := httptest.NewRecorder()
	resources.Connections().ServeHTTP(sourceResponse, sourceRequest)
	if sourceResponse.Code != http.StatusOK {
		testingContext.Fatalf("source response = %d, body = %s", sourceResponse.Code, sourceResponse.Body.String())
	}
	var sourceBody struct {
		Sources []struct {
			SemanticGroup models.SourceCalendarGroup `json:"semantic_group"`
		} `json:"sources"`
	}
	if decodeError := json.Unmarshal(sourceResponse.Body.Bytes(), &sourceBody); decodeError != nil {
		testingContext.Fatalf("decode source response: %v", decodeError)
	}
	groups := map[models.SourceCalendarGroup]bool{}
	for _, source := range sourceBody.Sources {
		groups[source.SemanticGroup] = true
	}
	if len(sourceBody.Sources) != 2 || !groups[models.SourceCalendarGroupCalendar] || !groups[models.SourceCalendarGroupBirthdays] {
		testingContext.Fatalf("source response = %#v", sourceBody.Sources)
	}
	var failedSynchronization models.CalendarSync
	if findError := fixture.Database.First(&failedSynchronization, "state = ?", models.CalendarSyncFailed).Error; findError != nil {
		testingContext.Fatalf("read failed synchronization: %v", findError)
	}
	var failedTask models.Task
	if findError := fixture.Database.First(&failedTask, "id = ?", connectionBody.Task.ID).Error; findError != nil {
		testingContext.Fatalf("read failed import task: %v", findError)
	}
	if failedTask.State != models.TaskFailed || failedTask.RetryCount != 1 || failedTask.ErrorCode == nil || *failedTask.ErrorCode != "calendar_import_failed" {
		testingContext.Fatalf("failed import task = %#v", failedTask)
	}
}

func TestSynchronizeAllSkipsEventsAfterSourceReconciliationFailure(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	referenceTime := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm organizer timezone: %v", confirmationError)
	}
	adapter := &callbackAdapter{}
	cipher, cipherError := services.NewCredentialCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)), bytes.NewReader(bytes.Repeat([]byte{0x12}, 256)))
	if cipherError != nil {
		testingContext.Fatalf("construct cipher: %v", cipherError)
	}
	connectionService, serviceError := services.NewCalendarConnectionService(fixture.Database, adapter, cipher, func() time.Time { return referenceTime }, bytes.NewReader(bytes.Repeat([]byte{0x31}, 256)))
	if serviceError != nil {
		testingContext.Fatalf("construct connection service: %v", serviceError)
	}
	start, startError := connectionService.CreateAuthorizationRequest(context.Background(), owner.ID, "https://rsvp.example.test"+config.WebCalendarConnectionCallbacksGoogle)
	if startError != nil {
		testingContext.Fatalf("create authorization request: %v", startError)
	}
	providerURL, parseError := url.Parse(start.AuthorizationURL)
	if parseError != nil {
		testingContext.Fatalf("parse authorization URL: %v", parseError)
	}
	confirmation, confirmationError := connectionService.ValidateCallback(context.Background(), owner.ID, providerURL.Query().Get("state"), "code")
	if confirmationError != nil {
		testingContext.Fatalf("validate callback: %v", confirmationError)
	}
	confirmation.Timezone = testsupport.TimezoneName
	connection, _, connectionError := connectionService.CreateConnection(context.Background(), owner.ID, confirmation, "scheduler-connection")
	if connectionError != nil {
		testingContext.Fatalf("create connection: %v", connectionError)
	}
	completedAt := referenceTime.UTC()
	if updateError := fixture.Database.Model(&models.Task{}).
		Where("resource_type = ? AND resource_id = ?", models.TaskResourceCalendarConnection, connection.ID).
		Updates(map[string]any{"state": models.TaskSucceeded, "retry_count": 1, "last_attempted_at": completedAt, "finished_at": completedAt}).Error; updateError != nil {
		testingContext.Fatalf("complete initial import task: %v", updateError)
	}
	calendar, calendarError := models.NewCalendar(owner.ID, "Source", "source", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct source calendar: %v", calendarError)
	}
	if createError := fixture.Database.Create(calendar).Error; createError != nil {
		testingContext.Fatalf("create source calendar: %v", createError)
	}
	syncState, stateError := models.NewProviderCalendarSyncState(connection.ID, "source")
	if stateError != nil {
		testingContext.Fatalf("construct provider calendar state: %v", stateError)
	}
	if createError := fixture.Database.Create(syncState).Error; createError != nil {
		testingContext.Fatalf("create provider calendar state: %v", createError)
	}
	mapping, mappingError := models.NewSourceCalendarMapping(syncState.ID, calendar.ID, models.SourceCalendarGroupCalendar)
	if mappingError != nil {
		testingContext.Fatalf("construct source mapping: %v", mappingError)
	}
	if createError := fixture.Database.Create(mapping).Error; createError != nil {
		testingContext.Fatalf("create source mapping: %v", createError)
	}
	localLane, laneError := models.NewOpenLane(calendar.ID, "Local plan", referenceTime, 0)
	if laneError != nil {
		testingContext.Fatalf("construct local lane: %v", laneError)
	}
	if createError := fixture.Database.Create(localLane).Error; createError != nil {
		testingContext.Fatalf("create local lane: %v", createError)
	}
	adapter.calendarBatch = &services.ProviderCalendarBatch{
		Calendars:      []services.ProviderCalendar{{ID: "source", Deleted: true}},
		NextSyncCursor: "next-calendar-cursor",
	}
	syncService, syncServiceError := services.NewCalendarSyncService(fixture.Database, adapter, cipher, func() time.Time { return referenceTime })
	if syncServiceError != nil {
		testingContext.Fatalf("construct synchronization service: %v", syncServiceError)
	}
	resources, resourceError := calendarconnection.NewWithServices(fixture.ApplicationContext, connectionService, syncService)
	if resourceError != nil {
		testingContext.Fatalf("construct HTTP resources: %v", resourceError)
	}

	synchronizationError := resources.SynchronizeAll(context.Background())
	if !errors.Is(synchronizationError, services.ErrCalendarConnectionHasLocalUse) {
		testingContext.Fatalf("synchronize all error = %v", synchronizationError)
	}
	if adapter.synchronizationCount != 0 {
		testingContext.Fatalf("event synchronization count = %d, want 0", adapter.synchronizationCount)
	}
	if findError := fixture.Database.First(&models.Lane{}, "id = ?", localLane.ID).Error; findError != nil {
		testingContext.Fatalf("protected local lane was removed: %v", findError)
	}
	var latestSynchronization models.CalendarSync
	if findError := fixture.Database.Order("created_at DESC").First(&latestSynchronization, "sync_state_id = ?", syncState.ID).Error; findError != nil {
		testingContext.Fatalf("read reconciliation failure: %v", findError)
	}
	if latestSynchronization.State != models.CalendarSyncFailed || latestSynchronization.ErrorCode == nil || *latestSynchronization.ErrorCode != "source_calendar_has_local_use" {
		testingContext.Fatalf("latest synchronization = %#v", latestSynchronization)
	}
	request := httptest.NewRequest(http.MethodGet, config.WebCalendarConnections+connection.ID, nil)
	request.Header.Set("Accept", handlers.JSONMediaType)
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, &owner))
	response := httptest.NewRecorder()
	resources.Connections().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		testingContext.Fatalf("connection status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	var body struct {
		Synchronization struct {
			State models.CalendarSyncState `json:"state"`
			Error bool                     `json:"error"`
		} `json:"synchronization"`
	}
	if decodeError := json.Unmarshal(response.Body.Bytes(), &body); decodeError != nil {
		testingContext.Fatalf("decode connection response: %v", decodeError)
	}
	if body.Synchronization.State != models.CalendarSyncFailed || !body.Synchronization.Error {
		testingContext.Fatalf("connection synchronization = %#v", body.Synchronization)
	}
}

func TestReadConnectionReturnsCurrentSynchronizationSummary(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm organizer timezone: %v", confirmationError)
	}
	calendar, calendarError := models.NewCalendar(owner.ID, "Imported", "imported", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct calendar: %v", calendarError)
	}
	if createError := fixture.Database.Create(calendar).Error; createError != nil {
		testingContext.Fatalf("create calendar: %v", createError)
	}
	connection, connectionError := models.NewCalendarConnection(owner.ID, bytes.Repeat([]byte{0x11}, 12), []byte{0x22})
	if connectionError != nil {
		testingContext.Fatalf("construct connection: %v", connectionError)
	}
	if createError := fixture.Database.Create(connection).Error; createError != nil {
		testingContext.Fatalf("create connection: %v", createError)
	}
	syncState, stateError := models.NewProviderCalendarSyncState(connection.ID, "source")
	if stateError != nil {
		testingContext.Fatalf("construct provider calendar state: %v", stateError)
	}
	if createError := fixture.Database.Create(syncState).Error; createError != nil {
		testingContext.Fatalf("create provider calendar state: %v", createError)
	}
	mapping, mappingError := models.NewSourceCalendarMapping(syncState.ID, calendar.ID, models.SourceCalendarGroupCalendar)
	if mappingError != nil {
		testingContext.Fatalf("construct source mapping: %v", mappingError)
	}
	if createError := fixture.Database.Create(mapping).Error; createError != nil {
		testingContext.Fatalf("create source mapping: %v", createError)
	}

	referenceTime := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	for synchronizationIndex := 0; synchronizationIndex < 24; synchronizationIndex++ {
		startedAt := referenceTime.Add(time.Duration(synchronizationIndex-24) * time.Hour)
		synchronization, synchronizationError := models.NewCalendarSync(syncState.ID, startedAt)
		if synchronizationError != nil {
			testingContext.Fatalf("construct synchronization %d: %v", synchronizationIndex, synchronizationError)
		}
		finishedAt := startedAt.Add(time.Minute)
		synchronization.State = models.CalendarSyncSucceeded
		synchronization.FinishedAt = &finishedAt
		if createError := fixture.Database.Create(synchronization).Error; createError != nil {
			testingContext.Fatalf("create synchronization %d: %v", synchronizationIndex, createError)
		}
	}
	latestSuccessfulSync := referenceTime.Add(-time.Hour + time.Minute)
	failed, failedError := models.NewCalendarSync(syncState.ID, referenceTime)
	if failedError != nil {
		testingContext.Fatalf("construct failed synchronization: %v", failedError)
	}
	failedAt := referenceTime.Add(time.Minute)
	failureCode := "provider_failed"
	failed.State = models.CalendarSyncFailed
	failed.FinishedAt = &failedAt
	failed.ErrorCode = &failureCode
	if createError := fixture.Database.Create(failed).Error; createError != nil {
		testingContext.Fatalf("create failed synchronization: %v", createError)
	}

	adapter := &callbackAdapter{}
	cipher, cipherError := services.NewCredentialCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)), bytes.NewReader(bytes.Repeat([]byte{0x12}, 64)))
	if cipherError != nil {
		testingContext.Fatalf("construct cipher: %v", cipherError)
	}
	connectionService, serviceError := services.NewCalendarConnectionService(fixture.Database, adapter, cipher, func() time.Time { return referenceTime }, bytes.NewReader(bytes.Repeat([]byte{0x31}, 128)))
	if serviceError != nil {
		testingContext.Fatalf("construct connection service: %v", serviceError)
	}
	syncService, syncServiceError := services.NewCalendarSyncService(fixture.Database, adapter, cipher, func() time.Time { return referenceTime })
	if syncServiceError != nil {
		testingContext.Fatalf("construct synchronization service: %v", syncServiceError)
	}
	resources, resourceError := calendarconnection.NewWithServices(fixture.ApplicationContext, connectionService, syncService)
	if resourceError != nil {
		testingContext.Fatalf("construct HTTP resources: %v", resourceError)
	}
	request := httptest.NewRequest(http.MethodGet, config.WebCalendarConnections+connection.ID, nil)
	request.Header.Set("Accept", handlers.JSONMediaType)
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, &owner))
	response := httptest.NewRecorder()

	resources.Connections().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		testingContext.Fatalf("connection status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	var body struct {
		Synchronization struct {
			State              models.CalendarSyncState `json:"state"`
			Error              bool                     `json:"error"`
			LastSuccessfulSync string                   `json:"last_successful_sync"`
		} `json:"synchronization"`
	}
	if decodeError := json.Unmarshal(response.Body.Bytes(), &body); decodeError != nil {
		testingContext.Fatalf("decode connection response: %v", decodeError)
	}
	if body.Synchronization.State != models.CalendarSyncFailed || !body.Synchronization.Error {
		testingContext.Fatalf("synchronization state = %#v", body.Synchronization)
	}
	if body.Synchronization.LastSuccessfulSync != latestSuccessfulSync.Format(time.RFC3339) {
		testingContext.Fatalf("last successful synchronization = %q, want %q", body.Synchronization.LastSuccessfulSync, latestSuccessfulSync.Format(time.RFC3339))
	}
}
