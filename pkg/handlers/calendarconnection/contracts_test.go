package calendarconnection_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers/calendarconnection"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
)

type callbackAdapter struct{}

func (*callbackAdapter) AuthorizationURL(state string, _ string) (string, error) {
	return "https://provider.example.test/authorize?state=" + url.QueryEscape(state), nil
}
func (*callbackAdapter) ExchangeCode(context.Context, string, string) (services.CalendarProviderCredential, error) {
	return services.CalendarProviderCredential{}, nil
}
func (*callbackAdapter) ListCalendars(context.Context, services.CalendarProviderCredential) ([]services.ProviderCalendar, error) {
	return nil, nil
}
func (*callbackAdapter) SynchronizeEvents(context.Context, services.CalendarProviderCredential, string, string) (services.ProviderEventBatch, error) {
	return services.ProviderEventBatch{}, nil
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
	resources, resourceError := calendarconnection.NewWithService(fixture.ApplicationContext, service)
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
}
