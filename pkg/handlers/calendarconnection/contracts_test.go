package calendarconnection_test

import (
	"bytes"
	"context"
	"encoding/base64"
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

type callbackAdapter struct{ synchronizationError error }

func (*callbackAdapter) AuthorizationURL(state string, _ string) (string, error) {
	return "https://provider.example.test/authorize?state=" + url.QueryEscape(state), nil
}
func (*callbackAdapter) ExchangeCode(context.Context, string, string) (services.CalendarProviderCredential, error) {
	return services.CalendarProviderCredential{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Date(2030, time.January, 2, 12, 0, 0, 0, time.UTC)}, nil
}
func (*callbackAdapter) RefreshCredential(_ context.Context, credential services.CalendarProviderCredential) (services.CalendarProviderCredential, error) {
	return credential, nil
}
func (*callbackAdapter) ListCalendars(context.Context, services.CalendarProviderCredential) ([]services.ProviderCalendar, error) {
	return []services.ProviderCalendar{{ID: "source", Name: "Source", Timezone: testsupport.TimezoneName, ColorToken: "source"}}, nil
}
func (adapter *callbackAdapter) SynchronizeEvents(context.Context, services.CalendarProviderCredential, string, string) (services.ProviderEventBatch, error) {
	return services.ProviderEventBatch{}, adapter.synchronizationError
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
	for _, requiredText := range []string{"data-calendar-confirmation", "Consent verified", "Read-only access", "View your calendar list", "View calendar events", "Create connection", "Back to Horizon"} {
		if !strings.Contains(body, requiredText) {
			testingContext.Fatalf("HTML callback does not contain %q", requiredText)
		}
	}
	if strings.Contains(body, "private-code") || strings.Contains(body, providerURL.Query().Get("state")) {
		testingContext.Fatal("HTML callback reflected OAuth credentials into the document")
	}
}

func TestSourceSelectionReturnsSynchronizationFailure(testingContext *testing.T) {
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
	connection, _, connectionError := service.CreateConnection(context.Background(), owner.ID, confirmation, "connection-key")
	if connectionError != nil {
		testingContext.Fatalf("create connection: %v", connectionError)
	}
	syncService, syncServiceError := services.NewCalendarSyncService(fixture.Database, adapter, cipher, func() time.Time { return referenceTime })
	if syncServiceError != nil {
		testingContext.Fatalf("construct synchronization service: %v", syncServiceError)
	}
	resources, resourceError := calendarconnection.NewWithServices(fixture.ApplicationContext, service, syncService)
	if resourceError != nil {
		testingContext.Fatalf("construct HTTP resources: %v", resourceError)
	}
	body := strings.NewReader(`{"provider_calendar_ids":["source"],"timezone":"` + testsupport.TimezoneName + `"}`)
	request := httptest.NewRequest(http.MethodPut, config.WebCalendarConnections+connection.ID+"/source-calendars/", body)
	request.Header.Set("Content-Type", handlers.JSONMediaType)
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, &owner))
	response := httptest.NewRecorder()

	resources.Connections().ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), "calendar_synchronization_failed") {
		testingContext.Fatalf("selection response = %d, body = %s", response.Code, response.Body.String())
	}
	var mappingCount int64
	if countError := fixture.Database.Model(&models.SourceCalendarMapping{}).Where("connection_id = ?", connection.ID).Count(&mappingCount).Error; countError != nil || mappingCount != 1 {
		testingContext.Fatalf("mapping count = %d, error = %v", mappingCount, countError)
	}
	var failedSynchronization models.CalendarSync
	if findError := fixture.Database.First(&failedSynchronization, "state = ?", models.CalendarSyncFailed).Error; findError != nil {
		testingContext.Fatalf("read failed synchronization: %v", findError)
	}
}
