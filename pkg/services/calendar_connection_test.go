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
	exchangeCount       int
	refreshCount        int
	listedCredential    services.CalendarProviderCredential
	calendarBatches     map[string]services.ProviderCalendarBatch
	rejectedCursors     map[string]bool
	calendarListCursors []string
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

func (adapter *deterministicCalendarAdapter) RefreshCredential(_ context.Context, credential services.CalendarProviderCredential) (services.CalendarProviderCredential, error) {
	adapter.refreshCount++
	if credential.RefreshToken != "secret-refresh" {
		return services.CalendarProviderCredential{}, errors.New("unexpected refresh token")
	}
	return services.CalendarProviderCredential{AccessToken: "renewed-access", RefreshToken: credential.RefreshToken, ExpiresAt: time.Date(2030, time.January, 1, 16, 0, 0, 0, time.UTC)}, nil
}

func (adapter *deterministicCalendarAdapter) ListCalendars(_ context.Context, credential services.CalendarProviderCredential, cursor string) (services.ProviderCalendarBatch, error) {
	adapter.listedCredential = credential
	adapter.calendarListCursors = append(adapter.calendarListCursors, cursor)
	if adapter.rejectedCursors[cursor] {
		return services.ProviderCalendarBatch{}, services.ErrCalendarListSyncCursorRejected
	}
	batch, found := adapter.calendarBatches[cursor]
	if !found {
		return services.ProviderCalendarBatch{}, errors.New("unexpected CalendarList cursor")
	}
	return batch, nil
}

func (adapter *deterministicCalendarAdapter) SynchronizeEvents(context.Context, services.CalendarProviderCredential, string, services.ProviderCalendarGroupKey, string) (services.ProviderEventBatch, error) {
	return services.ProviderEventBatch{}, nil
}

func providerCalendar(id string, name string, colorToken string, visible bool) services.ProviderCalendar {
	return services.ProviderCalendar{ID: id, Readable: true, Groups: []services.ProviderCalendarGroup{{
		Key: services.ProviderCalendarGroupCalendar, Name: name, ColorToken: colorToken, Visible: visible,
	}}}
}

func primaryProviderCalendar(id string, name string, colorToken string, visible bool) services.ProviderCalendar {
	calendar := providerCalendar(id, name, colorToken, visible)
	calendar.Groups = append(calendar.Groups, services.ProviderCalendarGroup{
		Key: services.ProviderCalendarGroupBirthdays, Name: "Birthdays", ColorToken: "birthdays", Visible: true,
	})
	return calendar
}

func TestCalendarConnectionConsentEncryptionReconciliationAndDeletion(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	referenceTime := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	currentTime := referenceTime
	adapter := &deterministicCalendarAdapter{
		calendarBatches: map[string]services.ProviderCalendarBatch{
			"": {Calendars: []services.ProviderCalendar{
				primaryProviderCalendar("personal", "Personal", "405060", true),
				providerCalendar("work", "Work", "102030", false),
			}, NextSyncCursor: "calendar-cursor-1"},
			"calendar-cursor-1": {Calendars: []services.ProviderCalendar{
				providerCalendar("work", "Work renamed", "abcdef", true),
				providerCalendar("family", "Family", "708090", true),
			}, NextSyncCursor: "calendar-cursor-2"},
			"calendar-cursor-2": {Calendars: []services.ProviderCalendar{{ID: "personal", Deleted: true}}, NextSyncCursor: "calendar-cursor-3"},
			"calendar-cursor-3": {NextSyncCursor: "calendar-cursor-4"},
		},
		rejectedCursors: map[string]bool{},
	}
	encodedKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	cipher, cipherError := services.NewCredentialCipher(encodedKey, bytes.NewReader(bytes.Repeat([]byte{0x17}, 128)))
	if cipherError != nil {
		testingContext.Fatalf("construct credential cipher: %v", cipherError)
	}
	service, serviceError := services.NewCalendarConnectionService(fixture.Database, adapter, cipher, func() time.Time { return currentTime }, bytes.NewReader(bytes.Repeat([]byte{0x33}, 256)))
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
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm organizer timezone: %v", confirmationError)
	}
	legacyCalendar, legacyCalendarError := models.EnsureDefaultCalendar(fixture.Database, owner.ID)
	if legacyCalendarError != nil {
		testingContext.Fatalf("create legacy calendar: %v", legacyCalendarError)
	}
	legacyEvent, legacyEventError := models.CreateLocalIntervalEvent(fixture.Database, &owner, legacyCalendar.ID, nil, "Legacy event", "", nil, referenceTime, referenceTime.Add(time.Hour), referenceTime, timezone)
	if legacyEventError != nil {
		testingContext.Fatalf("create legacy event: %v", legacyEventError)
	}
	mappings, reconciliationError := service.ReconcileSourceCalendars(context.Background(), owner.ID, connection.ID)
	if reconciliationError != nil {
		testingContext.Fatalf("reconcile source calendars: %v", reconciliationError)
	}
	if len(mappings) != 3 || adapter.listedCredential.RefreshToken != "secret-refresh" {
		testingContext.Fatalf("mappings = %#v, listed credential = %#v", mappings, adapter.listedCredential)
	}
	var personalMapping models.SourceCalendarMapping
	var birthdayMapping models.SourceCalendarMapping
	var workMapping models.SourceCalendarMapping
	for _, mapping := range mappings {
		switch mapping.ProviderCalendarID {
		case "personal":
			if mapping.SemanticGroup == models.SourceCalendarGroupBirthdays {
				birthdayMapping = mapping
			} else {
				personalMapping = mapping
			}
		case "work":
			workMapping = mapping
		}
	}
	if personalMapping.ID == "" || birthdayMapping.ID == "" || workMapping.ID == "" {
		testingContext.Fatalf("initial mappings = %#v", mappings)
	}
	if findError := fixture.Database.First(&models.Calendar{}, "id = ?", legacyCalendar.ID).Error; !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("legacy calendar remains after import cutover: %v", findError)
	}
	if findError := fixture.Database.First(&models.Event{}, "id = ?", legacyEvent.ID).Error; !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("legacy event remains after import cutover: %v", findError)
	}
	var birthdayCalendar models.Calendar
	if findError := fixture.Database.First(&birthdayCalendar, "id = ?", birthdayMapping.CalendarID).Error; findError != nil || birthdayCalendar.Name != "Birthdays" || birthdayCalendar.Symbol != "B" {
		testingContext.Fatalf("birthday calendar = %#v, error = %v", birthdayCalendar, findError)
	}
	var personalCalendar models.Calendar
	if findError := fixture.Database.First(&personalCalendar, "id = ?", personalMapping.CalendarID).Error; findError != nil || !personalCalendar.Visible {
		testingContext.Fatalf("personal calendar = %#v, error = %v", personalCalendar, findError)
	}
	var workCalendar models.Calendar
	if findError := fixture.Database.First(&workCalendar, "id = ?", workMapping.CalendarID).Error; findError != nil || workCalendar.Visible {
		testingContext.Fatalf("work calendar = %#v, error = %v", workCalendar, findError)
	}
	workCalendar.Symbol = "★"
	workCalendar.ColorToken = "local-work"
	if saveError := fixture.Database.Save(&workCalendar).Error; saveError != nil {
		testingContext.Fatalf("store local work presentation: %v", saveError)
	}
	currentTime = referenceTime.Add(119*time.Minute + 30*time.Second)
	mappings, reconciliationError = service.ReconcileSourceCalendars(context.Background(), owner.ID, connection.ID)
	if reconciliationError != nil {
		testingContext.Fatalf("incremental source calendar reconciliation: %v", reconciliationError)
	}
	if adapter.refreshCount != 1 || adapter.listedCredential.AccessToken != "renewed-access" || adapter.listedCredential.RefreshToken != "secret-refresh" {
		testingContext.Fatalf("refreshes = %d, listed credential = %#v", adapter.refreshCount, adapter.listedCredential)
	}
	var refreshedConnection models.CalendarConnection
	if findError := fixture.Database.First(&refreshedConnection, "id = ?", connection.ID).Error; findError != nil {
		testingContext.Fatalf("read refreshed connection: %v", findError)
	}
	refreshedCredentialBytes := append(append([]byte(nil), refreshedConnection.CredentialNonce...), refreshedConnection.CredentialCiphertext...)
	if bytes.Equal(credentialBytes, refreshedCredentialBytes) {
		testingContext.Fatal("refreshed credential was not re-encrypted")
	}
	if len(mappings) != 4 {
		testingContext.Fatalf("incremental mappings = %#v", mappings)
	}
	if findError := fixture.Database.First(&workCalendar, "id = ?", workMapping.CalendarID).Error; findError != nil {
		testingContext.Fatalf("reload work calendar: %v", findError)
	}
	if workCalendar.Name != "Work renamed" || workCalendar.Symbol != "★" || workCalendar.ColorToken != "local-work" || workCalendar.Visible {
		testingContext.Fatalf("reconciled work calendar = %#v", workCalendar)
	}
	var familyMapping models.SourceCalendarMapping
	for _, mapping := range mappings {
		if mapping.ProviderCalendarID == "family" {
			familyMapping = mapping
		}
	}
	var familyCalendar models.Calendar
	if familyMapping.ID == "" {
		testingContext.Fatalf("family mapping is absent: %#v", mappings)
	}
	if findError := fixture.Database.First(&familyCalendar, "id = ?", familyMapping.CalendarID).Error; findError != nil || !familyCalendar.Visible {
		testingContext.Fatalf("family calendar = %#v, error = %v", familyCalendar, findError)
	}
	lane, laneError := models.NewFiniteLane(personalMapping.CalendarID, "Imported event", referenceTime, referenceTime.Add(time.Hour), 0)
	if laneError != nil {
		testingContext.Fatalf("create imported lane: %v", laneError)
	}
	if createError := fixture.Database.Create(lane).Error; createError != nil {
		testingContext.Fatalf("persist imported lane: %v", createError)
	}
	eventTime, eventTimeError := models.NewIntervalEventTime(referenceTime, referenceTime.Add(time.Hour), timezone)
	if eventTimeError != nil {
		testingContext.Fatalf("construct imported event time: %v", eventTimeError)
	}
	event, eventError := models.NewEvent(lane.ID, "Imported event", "", nil, models.IndependentEventRelation(), eventTime)
	if eventError != nil {
		testingContext.Fatalf("create imported event: %v", eventError)
	}
	if createError := fixture.Database.Create(event).Error; createError != nil {
		testingContext.Fatalf("persist imported event: %v", createError)
	}
	venue := fixture.CreateVenue("VENUE001", owner.ID)
	if updateError := fixture.Database.Model(event).Update("venue_id", venue.ID).Error; updateError != nil {
		testingContext.Fatalf("add local venue to imported event: %v", updateError)
	}
	link, linkError := models.NewExternalEventLink(personalMapping.ID, event.ID, "provider-event", nil)
	if linkError != nil {
		testingContext.Fatalf("create imported event link: %v", linkError)
	}
	if createError := fixture.Database.Create(link).Error; createError != nil {
		testingContext.Fatalf("persist imported event link: %v", createError)
	}
	rsvp := fixture.CreateRSVP("LOCAL001", event.ID)
	if _, reconciliationError := service.ReconcileSourceCalendars(context.Background(), owner.ID, connection.ID); !errors.Is(reconciliationError, services.ErrCalendarConnectionHasLocalUse) {
		testingContext.Fatalf("delete protected source error = %v", reconciliationError)
	}
	if findError := fixture.Database.First(&models.SourceCalendarMapping{}, "id = ?", personalMapping.ID).Error; findError != nil {
		testingContext.Fatalf("protected mapping was removed: %v", findError)
	}
	if findError := fixture.Database.First(&models.RSVP{}, "id = ? AND event_id = ?", rsvp.ID, event.ID).Error; findError != nil {
		testingContext.Fatalf("protected RSVP was removed: %v", findError)
	}
	var failedReconciliation models.CalendarSync
	if findError := fixture.Database.Order("created_at DESC").First(&failedReconciliation, "mapping_id = ?", personalMapping.ID).Error; findError != nil {
		testingContext.Fatalf("read failed source reconciliation: %v", findError)
	}
	if failedReconciliation.State != models.CalendarSyncFailed || failedReconciliation.ErrorCode == nil || *failedReconciliation.ErrorCode != "source_calendar_has_local_use" {
		testingContext.Fatalf("failed source reconciliation = %#v", failedReconciliation)
	}
	if deleteError := fixture.Database.Unscoped().Delete(&rsvp).Error; deleteError != nil {
		testingContext.Fatalf("remove protected RSVP fixture: %v", deleteError)
	}
	if _, reconciliationError := service.ReconcileSourceCalendars(context.Background(), owner.ID, connection.ID); !errors.Is(reconciliationError, services.ErrCalendarConnectionHasLocalUse) {
		testingContext.Fatalf("delete source with local venue error = %v", reconciliationError)
	}
	if updateError := fixture.Database.Model(event).Update("venue_id", nil).Error; updateError != nil {
		testingContext.Fatalf("remove local venue fixture: %v", updateError)
	}
	localLane, localLaneError := models.NewOpenLane(personalMapping.CalendarID, "Local plan", referenceTime, 1)
	if localLaneError != nil {
		testingContext.Fatalf("construct local lane: %v", localLaneError)
	}
	if createError := fixture.Database.Create(localLane).Error; createError != nil {
		testingContext.Fatalf("create local lane: %v", createError)
	}
	if _, reconciliationError := service.ReconcileSourceCalendars(context.Background(), owner.ID, connection.ID); !errors.Is(reconciliationError, services.ErrCalendarConnectionHasLocalUse) {
		testingContext.Fatalf("delete source with local lane error = %v", reconciliationError)
	}
	if findError := fixture.Database.First(&models.Lane{}, "id = ?", localLane.ID).Error; findError != nil {
		testingContext.Fatalf("protected local lane was removed: %v", findError)
	}
	var protectedConnection models.CalendarConnection
	if findError := fixture.Database.First(&protectedConnection, "id = ?", connection.ID).Error; findError != nil || protectedConnection.CalendarListSyncCursor == nil || *protectedConnection.CalendarListSyncCursor != "calendar-cursor-2" {
		testingContext.Fatalf("protected CalendarList cursor = %#v, error = %v", protectedConnection.CalendarListSyncCursor, findError)
	}
	if deleteError := fixture.Database.Unscoped().Delete(localLane).Error; deleteError != nil {
		testingContext.Fatalf("remove local lane fixture: %v", deleteError)
	}
	if _, reconciliationError := service.ReconcileSourceCalendars(context.Background(), owner.ID, connection.ID); reconciliationError != nil {
		testingContext.Fatalf("delete unused source: %v", reconciliationError)
	}
	if findError := fixture.Database.First(&models.SourceCalendarMapping{}, "id = ?", personalMapping.ID).Error; !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("deleted source mapping remains: %v", findError)
	}
	if findError := fixture.Database.First(&models.Calendar{}, "id = ?", personalMapping.CalendarID).Error; !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("deleted source calendar remains: %v", findError)
	}
	if findError := fixture.Database.First(&models.SourceCalendarMapping{}, "id = ?", birthdayMapping.ID).Error; !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("deleted birthday source mapping remains: %v", findError)
	}
	adapter.rejectedCursors["calendar-cursor-3"] = true
	adapter.calendarBatches[""] = services.ProviderCalendarBatch{Calendars: []services.ProviderCalendar{
		providerCalendar("work", "Work renamed", "abcdef", true),
		providerCalendar("family", "Family", "708090", true),
	}, NextSyncCursor: "calendar-cursor-4"}
	if _, reconciliationError := service.ReconcileSourceCalendars(context.Background(), owner.ID, connection.ID); reconciliationError != nil {
		testingContext.Fatalf("complete source reconciliation: %v", reconciliationError)
	}
	if strings.Join(adapter.calendarListCursors, ",") != ",calendar-cursor-1,calendar-cursor-2,calendar-cursor-2,calendar-cursor-2,calendar-cursor-2,calendar-cursor-3," {
		testingContext.Fatalf("CalendarList cursors = %#v", adapter.calendarListCursors)
	}
	var currentConnection models.CalendarConnection
	if findError := fixture.Database.First(&currentConnection, "id = ?", connection.ID).Error; findError != nil || currentConnection.CalendarListSyncCursor == nil || *currentConnection.CalendarListSyncCursor != "calendar-cursor-4" {
		testingContext.Fatalf("connection CalendarList cursor = %#v, error = %v", currentConnection.CalendarListSyncCursor, findError)
	}

	if deleteError := service.DeleteConnection(context.Background(), owner.ID, connection.ID); deleteError != nil {
		testingContext.Fatalf("delete connection: %v", deleteError)
	}
	if findError := fixture.Database.Unscoped().First(&models.CalendarConnection{}, "id = ?", connection.ID).Error; !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("connection remains after delete: %v", findError)
	}
	var mappingCount int64
	if countError := fixture.Database.Unscoped().Model(&models.SourceCalendarMapping{}).Where("connection_id = ?", connection.ID).Count(&mappingCount).Error; countError != nil {
		testingContext.Fatalf("count deleted mappings: %v", countError)
	}
	if mappingCount != 0 {
		testingContext.Fatalf("mapping count after delete = %d, want 0", mappingCount)
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
