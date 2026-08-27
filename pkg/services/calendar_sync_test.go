package services_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

type synchronizationAdapter struct {
	batches                 map[string]services.ProviderEventBatch
	rejected                map[string]bool
	cursors                 []string
	refreshCount            int
	synchronizedCredentials []services.CalendarProviderCredential
}

func (*synchronizationAdapter) AuthorizationURL(state string, redirectURI string) (string, error) {
	query := url.Values{"state": {state}, "redirect_uri": {redirectURI}}
	return "https://provider.example.test/authorize?" + query.Encode(), nil
}
func (*synchronizationAdapter) ExchangeCode(context.Context, string, string) (services.CalendarProviderCredential, error) {
	return services.CalendarProviderCredential{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Date(2026, time.August, 26, 14, 0, 0, 0, time.UTC)}, nil
}
func (adapter *synchronizationAdapter) RefreshCredential(_ context.Context, credential services.CalendarProviderCredential) (services.CalendarProviderCredential, error) {
	adapter.refreshCount++
	return services.CalendarProviderCredential{AccessToken: "renewed-access", RefreshToken: credential.RefreshToken, ExpiresAt: time.Date(2026, time.August, 26, 16, 0, 0, 0, time.UTC)}, nil
}
func (*synchronizationAdapter) ListCalendars(context.Context, services.CalendarProviderCredential) ([]services.ProviderCalendar, error) {
	return []services.ProviderCalendar{{ID: "source", Name: "Source", Timezone: testsupport.TimezoneName, ColorToken: "source"}}, nil
}
func (adapter *synchronizationAdapter) SynchronizeEvents(_ context.Context, credential services.CalendarProviderCredential, providerCalendarID string, cursor string) (services.ProviderEventBatch, error) {
	if providerCalendarID != "source" {
		return services.ProviderEventBatch{}, errors.New("unexpected provider calendar")
	}
	adapter.cursors = append(adapter.cursors, cursor)
	adapter.synchronizedCredentials = append(adapter.synchronizedCredentials, credential)
	if adapter.rejected[cursor] {
		return services.ProviderEventBatch{}, services.ErrCalendarSyncCursorRejected
	}
	batch, found := adapter.batches[cursor]
	if !found {
		return services.ProviderEventBatch{}, errors.New("unexpected cursor")
	}
	return batch, nil
}

func TestCalendarSynchronizationIsIdempotentIncrementalAndReconcilesRejectedCursor(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	referenceTime := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	currentTime := referenceTime
	timedStart := time.Date(2026, time.September, 2, 16, 0, 0, 0, time.UTC)
	timedEnd := timedStart.Add(time.Hour)
	seriesStartOne := time.Date(2026, time.September, 3, 16, 0, 0, 0, time.UTC)
	seriesEndOne := seriesStartOne.Add(time.Hour)
	seriesStartTwo := seriesStartOne.AddDate(0, 0, 7)
	seriesEndTwo := seriesStartTwo.Add(time.Hour)
	adapter := &synchronizationAdapter{batches: map[string]services.ProviderEventBatch{}, rejected: map[string]bool{}}
	adapter.batches[""] = services.ProviderEventBatch{NextSyncCursor: "cursor-1", Events: []services.ProviderEvent{
		{ID: "birthday", Title: "Birthday", Status: "confirmed", Timezone: testsupport.TimezoneName, StartDate: "2026-09-01", EndDate: "2026-09-02"},
		{ID: "holiday", Title: "Holiday", Status: "confirmed", Timezone: testsupport.TimezoneName, StartsAt: &timedStart, EndsAt: &timedEnd},
		{ID: "series-1", SeriesID: "series", Title: "Weekly review", Status: "confirmed", Timezone: testsupport.TimezoneName, StartsAt: &seriesStartOne, EndsAt: &seriesEndOne},
		{ID: "series-2", SeriesID: "series", Title: "Weekly review", Status: "confirmed", Timezone: testsupport.TimezoneName, StartsAt: &seriesStartTwo, EndsAt: &seriesEndTwo},
	}}
	updatedStart := timedStart.Add(2 * time.Hour)
	updatedEnd := updatedStart.Add(2 * time.Hour)
	adapter.batches["cursor-1"] = services.ProviderEventBatch{NextSyncCursor: "cursor-2", Events: []services.ProviderEvent{
		{ID: "birthday", Title: "Updated Birthday", Status: "confirmed", Timezone: testsupport.TimezoneName, StartDate: "2026-09-01", EndDate: "2026-09-02"},
		{ID: "holiday", Status: "cancelled"},
		{ID: "series-1", SeriesID: "series", Title: "Weekly review updated", Status: "confirmed", Timezone: testsupport.TimezoneName, StartsAt: &updatedStart, EndsAt: &updatedEnd},
	}}
	cipher, cipherError := services.NewCredentialCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)), bytes.NewReader(bytes.Repeat([]byte{0x21}, 512)))
	if cipherError != nil {
		testingContext.Fatalf("construct cipher: %v", cipherError)
	}
	connectionService, connectionError := services.NewCalendarConnectionService(fixture.Database, adapter, cipher, func() time.Time { return currentTime }, bytes.NewReader(bytes.Repeat([]byte{0x32}, 512)))
	if connectionError != nil {
		testingContext.Fatalf("construct connection service: %v", connectionError)
	}
	start, startError := connectionService.CreateAuthorizationRequest(context.Background(), owner.ID, "https://rsvp.example.test/callback")
	if startError != nil {
		testingContext.Fatalf("create authorization request: %v", startError)
	}
	providerURL, _ := url.Parse(start.AuthorizationURL)
	confirmation, confirmationError := connectionService.ValidateCallback(context.Background(), owner.ID, providerURL.Query().Get("state"), "code")
	if confirmationError != nil {
		testingContext.Fatalf("validate callback: %v", confirmationError)
	}
	connection, _, createError := connectionService.CreateConnection(context.Background(), owner.ID, confirmation, "connection")
	if createError != nil {
		testingContext.Fatalf("create connection: %v", createError)
	}
	timezone, _ := models.NewTimezone(testsupport.TimezoneName)
	mappings, mappingError := connectionService.ReplaceSourceCalendars(context.Background(), &owner, timezone, connection.ID, []string{"source"})
	if mappingError != nil {
		testingContext.Fatalf("select source: %v", mappingError)
	}
	currentTime = referenceTime.Add(119*time.Minute + 30*time.Second)
	syncService, syncServiceError := services.NewCalendarSyncService(fixture.Database, adapter, cipher, func() time.Time { return currentTime })
	if syncServiceError != nil {
		testingContext.Fatalf("construct sync service: %v", syncServiceError)
	}

	initial, reused, initialError := syncService.Create(context.Background(), owner.ID, mappings[0].ID, "initial")
	if initialError != nil || reused || initial.State != models.CalendarSyncSucceeded {
		testingContext.Fatalf("initial sync = %#v, reused = %t, error = %v", initial, reused, initialError)
	}
	if adapter.refreshCount != 1 || len(adapter.synchronizedCredentials) != 1 || adapter.synchronizedCredentials[0].AccessToken != "renewed-access" {
		testingContext.Fatalf("refreshes = %d, synchronized credentials = %#v", adapter.refreshCount, adapter.synchronizedCredentials)
	}
	assertSourceCounts(testingContext, fixture.Database, 4, 3, 1)
	repeated, repeatedReuse, repeatedError := syncService.Create(context.Background(), owner.ID, mappings[0].ID, "initial")
	if repeatedError != nil || !repeatedReuse || repeated.ID != initial.ID || len(adapter.cursors) != 1 {
		testingContext.Fatalf("repeated sync = %#v, reused = %t, cursors = %#v, error = %v", repeated, repeatedReuse, adapter.cursors, repeatedError)
	}

	var holidayLink models.ExternalEventLink
	if findError := fixture.Database.First(&holidayLink, "provider_event_id = ?", "holiday").Error; findError != nil {
		testingContext.Fatalf("find holiday link: %v", findError)
	}
	protectedRSVP := fixture.CreateRSVP("PROTECT1", holidayLink.EventID)
	if _, _, incrementalError := syncService.Create(context.Background(), owner.ID, mappings[0].ID, "incremental"); !errors.Is(incrementalError, services.ErrCalendarConnectionHasLocalUse) {
		testingContext.Fatalf("protected incremental sync error = %v", incrementalError)
	}
	if findError := fixture.Database.First(&models.ExternalEventLink{}, "id = ? AND event_id = ?", holidayLink.ID, holidayLink.EventID).Error; findError != nil {
		testingContext.Fatalf("protected event link was removed: %v", findError)
	}
	if findError := fixture.Database.First(&models.RSVP{}, "id = ? AND event_id = ?", protectedRSVP.ID, holidayLink.EventID).Error; findError != nil {
		testingContext.Fatalf("protected RSVP was removed: %v", findError)
	}
	var protectedMapping models.SourceCalendarMapping
	if findError := fixture.Database.First(&protectedMapping, "id = ?", mappings[0].ID).Error; findError != nil || protectedMapping.SyncCursor == nil || *protectedMapping.SyncCursor != "cursor-1" {
		testingContext.Fatalf("protected mapping = %#v, error = %v", protectedMapping, findError)
	}
	if deleteError := fixture.Database.Unscoped().Delete(&protectedRSVP).Error; deleteError != nil {
		testingContext.Fatalf("remove protected RSVP fixture: %v", deleteError)
	}
	if _, _, incrementalError := syncService.Create(context.Background(), owner.ID, mappings[0].ID, "incremental"); incrementalError != nil {
		testingContext.Fatalf("incremental sync: %v", incrementalError)
	}
	assertSourceCounts(testingContext, fixture.Database, 3, 2, 1)
	var birthdayLink models.ExternalEventLink
	if findError := fixture.Database.First(&birthdayLink, "provider_event_id = ?", "birthday").Error; findError != nil {
		testingContext.Fatalf("find birthday link: %v", findError)
	}
	var birthday models.Event
	if findError := fixture.Database.First(&birthday, "id = ?", birthdayLink.EventID).Error; findError != nil {
		testingContext.Fatalf("find birthday: %v", findError)
	}
	if birthday.Title != "Updated Birthday" || birthday.StartDate == nil || *birthday.StartDate != "2026-09-01" {
		testingContext.Fatalf("updated birthday = %#v", birthday)
	}

	adapter.rejected["cursor-2"] = true
	adapter.batches[""] = services.ProviderEventBatch{NextSyncCursor: "cursor-3", Events: []services.ProviderEvent{
		{ID: "birthday", Title: "Reconciled Birthday", Status: "confirmed", Timezone: testsupport.TimezoneName, StartDate: "2026-09-01", EndDate: "2026-09-02"},
	}}
	if _, _, reconciliationError := syncService.Create(context.Background(), owner.ID, mappings[0].ID, "reconcile"); reconciliationError != nil {
		testingContext.Fatalf("reconcile sync: %v", reconciliationError)
	}
	assertSourceCounts(testingContext, fixture.Database, 1, 1, 0)
	if !reflect.DeepEqual(adapter.cursors, []string{"", "cursor-1", "cursor-1", "cursor-2", ""}) {
		testingContext.Fatalf("sync cursors = %#v", adapter.cursors)
	}
	var mapping models.SourceCalendarMapping
	if findError := fixture.Database.First(&mapping, "id = ?", mappings[0].ID).Error; findError != nil {
		testingContext.Fatalf("read mapping: %v", findError)
	}
	if mapping.SyncCursor == nil || *mapping.SyncCursor != "cursor-3" {
		testingContext.Fatalf("mapping cursor = %#v", mapping.SyncCursor)
	}
	var reconciledBirthdayLink models.ExternalEventLink
	if findError := fixture.Database.First(&reconciledBirthdayLink, "provider_event_id = ?", "birthday").Error; findError != nil {
		testingContext.Fatalf("find reconciled birthday link: %v", findError)
	}
	if reconciledBirthdayLink.EventID != birthday.ID {
		testingContext.Fatalf("reconciled event ID = %q, want stable %q", reconciledBirthdayLink.EventID, birthday.ID)
	}
}

func assertSourceCounts(testingContext *testing.T, database *gorm.DB, events int64, lanes int64, series int64) {
	testingContext.Helper()
	checks := []struct {
		model any
		want  int64
	}{{&models.ExternalEventLink{}, events}, {&models.Lane{}, lanes}, {&models.ExternalEventSeriesLink{}, series}}
	for _, check := range checks {
		var count int64
		if countError := database.Model(check.model).Count(&count).Error; countError != nil {
			testingContext.Fatalf("count %T: %v", check.model, countError)
		}
		if count != check.want {
			testingContext.Fatalf("count %T = %d, want %d", check.model, count, check.want)
		}
	}
}
