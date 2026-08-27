package services_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

type deterministicCalendarAdapter struct {
	exchangeCount    int
	listedCredential services.CalendarProviderCredential
}

func (adapter *deterministicCalendarAdapter) AuthorizationURL(state string, redirectURI string) (string, error) {
	query := url.Values{"state": {state}, "redirect_uri": {redirectURI}}
	return "https://provider.example.test/authorize?" + query.Encode(), nil
}

func (adapter *deterministicCalendarAdapter) ExchangeCode(_ context.Context, code string, _ string) (services.CalendarProviderCredential, error) {
	adapter.exchangeCount++
	if code != "authorization-code" {
		return services.CalendarProviderCredential{}, errors.New("provider status 400")
	}
	return services.CalendarProviderCredential{AccessToken: "secret-access", RefreshToken: "secret-refresh", ExpiresAt: time.Date(2030, time.January, 1, 14, 0, 0, 0, time.UTC)}, nil
}

func (adapter *deterministicCalendarAdapter) ListCalendars(_ context.Context, credential services.CalendarProviderCredential) ([]services.ProviderCalendar, error) {
	adapter.listedCredential = credential
	return []services.ProviderCalendar{
		{ID: "personal", Name: "Personal", Timezone: testsupport.TimezoneName, ColorToken: "405060"},
		{ID: "work", Name: "Work", Timezone: testsupport.TimezoneName, ColorToken: "102030"},
	}, nil
}

func TestCalendarConnectionConsentEncryptionSelectionAndDeletion(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	referenceTime := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	adapter := &deterministicCalendarAdapter{}
	encodedKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	cipher, cipherError := services.NewCredentialCipher(encodedKey, bytes.NewReader(bytes.Repeat([]byte{0x17}, 128)))
	if cipherError != nil {
		testingContext.Fatalf("construct credential cipher: %v", cipherError)
	}
	service, serviceError := services.NewCalendarConnectionService(fixture.Database, adapter, cipher, func() time.Time { return referenceTime }, bytes.NewReader(bytes.Repeat([]byte{0x33}, 256)))
	if serviceError != nil {
		testingContext.Fatalf("construct connection service: %v", serviceError)
	}
	redirectURI := "https://rsvp.example.test/calendar-connection-callbacks/google/"
	start, startError := service.CreateAuthorizationRequest(context.Background(), owner.ID, redirectURI)
	if startError != nil {
		testingContext.Fatalf("create authorization request: %v", startError)
	}
	authorizationURL, parseError := url.Parse(start.AuthorizationURL)
	if parseError != nil {
		testingContext.Fatalf("parse authorization URL: %v", parseError)
	}
	state := authorizationURL.Query().Get("state")
	if state == "" || strings.Contains(string(mustAuthorizationRequest(testingContext, fixture.Database, start.RequestID).StateHash), state) {
		testingContext.Fatal("authorization state was not stored only as a hash")
	}
	confirmation, callbackError := service.ValidateCallback(context.Background(), owner.ID, state, "authorization-code")
	if callbackError != nil {
		testingContext.Fatalf("validate callback: %v", callbackError)
	}
	storedBeforeConnection := mustAuthorizationRequest(testingContext, fixture.Database, start.RequestID)
	if storedBeforeConnection.UsedAt != nil {
		testingContext.Fatal("callback changed the authorization request")
	}

	connection, reused, connectionError := service.CreateConnection(context.Background(), owner.ID, confirmation, "connection-key")
	if connectionError != nil || reused {
		testingContext.Fatalf("create connection reused = %t, error = %v", reused, connectionError)
	}
	credentialBytes := append(append([]byte(nil), connection.CredentialNonce...), connection.CredentialCiphertext...)
	if bytes.Contains(credentialBytes, []byte("secret-access")) || bytes.Contains(credentialBytes, []byte("secret-refresh")) {
		testingContext.Fatal("stored credential data contains plaintext")
	}
	repeatedConnection, repeated, repeatedError := service.CreateConnection(context.Background(), owner.ID, confirmation, "connection-key")
	if repeatedError != nil || !repeated || repeatedConnection.ID != connection.ID || adapter.exchangeCount != 1 {
		testingContext.Fatalf("repeat ID = %q, reused = %t, exchanges = %d, error = %v", repeatedConnection.ID, repeated, adapter.exchangeCount, repeatedError)
	}
	conflictingConfirmation := confirmation
	conflictingConfirmation.Code = "other-code"
	if _, _, conflictError := service.CreateConnection(context.Background(), owner.ID, conflictingConfirmation, "connection-key"); !errors.Is(conflictError, services.ErrIdempotencyConflict) {
		testingContext.Fatalf("idempotency conflict error = %v", conflictError)
	}

	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	mappings, selectionError := service.ReplaceSourceCalendars(context.Background(), &owner, timezone, connection.ID, []string{"work", "personal"})
	if selectionError != nil {
		testingContext.Fatalf("replace source calendars: %v", selectionError)
	}
	if len(mappings) != 2 || adapter.listedCredential.RefreshToken != "secret-refresh" {
		testingContext.Fatalf("mappings = %#v, listed credential = %#v", mappings, adapter.listedCredential)
	}
	var calendarCount int64
	if countError := fixture.Database.Model(&models.Calendar{}).Where("organizer_id = ?", owner.ID).Count(&calendarCount).Error; countError != nil {
		testingContext.Fatalf("count RSVP calendars: %v", countError)
	}
	if calendarCount != 2 {
		testingContext.Fatalf("RSVP calendar count = %d, want 2", calendarCount)
	}
	if _, selectionError := service.ReplaceSourceCalendars(context.Background(), &owner, timezone, connection.ID, []string{"absent"}); !errors.Is(selectionError, services.ErrSourceCalendarSelectionInvalid) {
		testingContext.Fatalf("invalid selection error = %v", selectionError)
	}

	if deleteError := service.DeleteConnection(context.Background(), owner.ID, connection.ID); deleteError != nil {
		testingContext.Fatalf("delete connection: %v", deleteError)
	}
	if findError := fixture.Database.Unscoped().First(&models.CalendarConnection{}, "id = ?", connection.ID).Error; !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("connection remains after delete: %v", findError)
	}
	if countError := fixture.Database.Unscoped().Model(&models.SourceCalendarMapping{}).Where("connection_id = ?", connection.ID).Count(&calendarCount).Error; countError != nil {
		testingContext.Fatalf("count deleted mappings: %v", countError)
	}
	if calendarCount != 0 {
		testingContext.Fatalf("mapping count after delete = %d, want 0", calendarCount)
	}
}

func TestCredentialCipherRejectsInvalidKey(testingContext *testing.T) {
	if _, cipherError := services.NewCredentialCipher(base64.StdEncoding.EncodeToString([]byte("short")), bytes.NewReader([]byte{1})); !errors.Is(cipherError, services.ErrCredentialEncryptionKeyInvalid) {
		testingContext.Fatalf("cipher error = %v", cipherError)
	}
}

func mustAuthorizationRequest(testingContext *testing.T, database *gorm.DB, identifier string) models.CalendarAuthorizationRequest {
	testingContext.Helper()
	var request models.CalendarAuthorizationRequest
	if findError := database.First(&request, "id = ?", identifier).Error; findError != nil {
		testingContext.Fatalf("find authorization request: %v", findError)
	}
	return request
}
