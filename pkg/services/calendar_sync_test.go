package services_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

type synchronizationAdapter struct {
	calendarBatch           services.ProviderCalendarBatch
	batches                 map[string]services.ProviderEventBatch
	rejected                map[string]bool
	providerErrors          map[string]error
	providerCalls           []string
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
func (adapter *synchronizationAdapter) ListCalendars(context.Context, services.CalendarProviderCredential, string) (services.ProviderCalendarBatch, error) {
	return adapter.calendarBatch, nil
}
func (adapter *synchronizationAdapter) SynchronizeEvents(_ context.Context, credential services.CalendarProviderCredential, providerCalendarID string, _ services.ProviderCalendarGroupKey, cursor string) (services.ProviderEventBatch, error) {
	adapter.providerCalls = append(adapter.providerCalls, providerCalendarID)
	adapter.cursors = append(adapter.cursors, cursor)
	adapter.synchronizedCredentials = append(adapter.synchronizedCredentials, credential)
	if providerError := adapter.providerErrors[providerCalendarID]; providerError != nil {
		return services.ProviderEventBatch{}, providerError
	}
	if adapter.rejected[cursor] {
		return services.ProviderEventBatch{}, services.ErrCalendarSyncCursorRejected
	}
	batch, found := adapter.batches[cursor]
	if !found {
		return services.ProviderEventBatch{}, errors.New("unexpected cursor")
	}
	return batch, nil
}

func TestCalendarSynchronizationContinuesAndRetriesFailedMappings(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	referenceTime := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	adapter := &synchronizationAdapter{
		calendarBatch: services.ProviderCalendarBatch{NextSyncCursor: "calendar-cursor-1", Calendars: []services.ProviderCalendar{
			providerCalendar("source-a", "Source A", "source-a", true),
			providerCalendar("source-b", "Source B", "source-b", true),
		}},
		batches: map[string]services.ProviderEventBatch{
			"":         {NextSyncCursor: "cursor-1"},
			"cursor-1": {NextSyncCursor: "cursor-2"},
		},
		rejected:       map[string]bool{},
		providerErrors: map[string]error{},
	}
	cipher, cipherError := services.NewCredentialCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)), bytes.NewReader(bytes.Repeat([]byte{0x21}, 512)))
	if cipherError != nil {
		testingContext.Fatalf("construct cipher: %v", cipherError)
	}
	connectionService, connectionError := services.NewCalendarConnectionService(fixture.Database, adapter, cipher, func() time.Time { return referenceTime }, bytes.NewReader(bytes.Repeat([]byte{0x32}, 512)))
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
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm organizer timezone: %v", confirmationError)
	}
	mappings, mappingError := connectionService.ReconcileSourceCalendars(context.Background(), owner.ID, connection.ID)
	if mappingError != nil {
		testingContext.Fatalf("reconcile sources: %v", mappingError)
	}
	slices.SortFunc(mappings, func(left models.SourceCalendarMapping, right models.SourceCalendarMapping) int {
		return strings.Compare(left.ID, right.ID)
	})
	adapter.providerErrors[mappings[0].ProviderCalendarID] = errors.New("provider unavailable")
	syncService, syncServiceError := services.NewCalendarSyncService(fixture.Database, adapter, cipher, func() time.Time { return referenceTime })
	if syncServiceError != nil {
		testingContext.Fatalf("construct sync service: %v", syncServiceError)
	}
	if synchronizationError := syncService.SynchronizeMappings(context.Background(), owner.ID, mappings); synchronizationError == nil {
		testingContext.Fatal("first synchronization error is absent")
	}
	if !reflect.DeepEqual(adapter.providerCalls, []string{mappings[0].ProviderCalendarID, mappings[1].ProviderCalendarID}) {
		testingContext.Fatalf("provider calls = %#v", adapter.providerCalls)
	}
	var successfulMapping models.SourceCalendarMapping
	if findError := fixture.Database.First(&successfulMapping, "id = ?", mappings[1].ID).Error; findError != nil || successfulMapping.SyncCursor == nil || *successfulMapping.SyncCursor != "cursor-1" {
		testingContext.Fatalf("successful mapping = %#v, error = %v", successfulMapping, findError)
	}
	delete(adapter.providerErrors, mappings[0].ProviderCalendarID)
	adapter.providerCalls = nil
	if synchronizationError := syncService.SynchronizeMappings(context.Background(), owner.ID, mappings); synchronizationError != nil {
		testingContext.Fatalf("retry synchronization: %v", synchronizationError)
	}
	if !reflect.DeepEqual(adapter.providerCalls, []string{mappings[0].ProviderCalendarID, mappings[1].ProviderCalendarID}) {
		testingContext.Fatalf("retry provider calls = %#v", adapter.providerCalls)
	}
}

func TestCalendarSynchronizationIsIncrementalAndReconcilesRejectedCursor(testingContext *testing.T) {
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
	pointAt := time.Date(2026, time.September, 4, 16, 0, 0, 0, time.UTC)
	adapter := &synchronizationAdapter{
		calendarBatch: services.ProviderCalendarBatch{NextSyncCursor: "calendar-cursor-1", Calendars: []services.ProviderCalendar{
			providerCalendar("source", "Source", "source", true),
		}},
		batches:  map[string]services.ProviderEventBatch{},
		rejected: map[string]bool{},
	}
	adapter.batches[""] = services.ProviderEventBatch{NextSyncCursor: "cursor-1", Events: []services.ProviderEvent{
		{ID: "birthday", Title: "Birthday", Status: "confirmed", Timezone: testsupport.TimezoneName, StartDate: "2026-09-01", EndDate: "2026-09-02"},
		{ID: "holiday", Title: "Holiday", Status: "confirmed", Timezone: testsupport.TimezoneName, StartsAt: &timedStart, EndsAt: &timedEnd},
		{ID: "series-1", SeriesID: "series", Title: "Weekly review", Status: "confirmed", Timezone: testsupport.TimezoneName, StartsAt: &seriesStartOne, EndsAt: &seriesEndOne},
		{ID: "series-2", SeriesID: "series", Title: "Weekly review", Status: "confirmed", Timezone: testsupport.TimezoneName, StartsAt: &seriesStartTwo, EndsAt: &seriesEndTwo},
		{ID: "deadline", Title: "Deadline", Status: "confirmed", Timezone: testsupport.TimezoneName, At: &pointAt},
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
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm organizer timezone: %v", confirmationError)
	}
	mappings, mappingError := connectionService.ReconcileSourceCalendars(context.Background(), owner.ID, connection.ID)
	if mappingError != nil {
		testingContext.Fatalf("reconcile source: %v", mappingError)
	}
	currentTime = referenceTime.Add(119*time.Minute + 30*time.Second)
	syncService, syncServiceError := services.NewCalendarSyncService(fixture.Database, adapter, cipher, func() time.Time { return currentTime })
	if syncServiceError != nil {
		testingContext.Fatalf("construct sync service: %v", syncServiceError)
	}

	initial, initialError := syncService.Synchronize(context.Background(), owner.ID, mappings[0].ID)
	if initialError != nil || initial.State != models.CalendarSyncSucceeded {
		testingContext.Fatalf("initial sync = %#v, error = %v", initial, initialError)
	}
	if adapter.refreshCount != 1 || len(adapter.synchronizedCredentials) != 1 || adapter.synchronizedCredentials[0].AccessToken != "renewed-access" {
		testingContext.Fatalf("refreshes = %d, synchronized credentials = %#v", adapter.refreshCount, adapter.synchronizedCredentials)
	}
	assertSourceCounts(testingContext, fixture.Database, 5, 4, 1)
	var holidayLink models.ExternalEventLink
	if findError := fixture.Database.First(&holidayLink, "provider_event_id = ?", "holiday").Error; findError != nil {
		testingContext.Fatalf("find holiday link: %v", findError)
	}
	protectedRSVP := fixture.CreateRSVP("PROTECT1", holidayLink.EventID)
	if _, incrementalError := syncService.Synchronize(context.Background(), owner.ID, mappings[0].ID); !errors.Is(incrementalError, services.ErrCalendarConnectionHasLocalUse) {
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
	if _, incrementalError := syncService.Synchronize(context.Background(), owner.ID, mappings[0].ID); incrementalError != nil {
		testingContext.Fatalf("incremental sync: %v", incrementalError)
	}
	assertSourceCounts(testingContext, fixture.Database, 4, 3, 1)
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
	if _, reconciliationError := syncService.Synchronize(context.Background(), owner.ID, mappings[0].ID); reconciliationError != nil {
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
