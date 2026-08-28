package event_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers/event"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

func TestEventCreationTransactionCommitsEventAndVenue(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	formValues := url.Values{
		config.TitleParam:                        {"Launch Dinner"},
		config.DescriptionParam:                  {"Dinner after launch."},
		config.StartTimeParam:                    {testsupport.FixedStartTime().Format(config.TimeLayoutHTMLForm)},
		config.TimezoneParam:                     {testsupport.TimezoneName},
		config.DurationParam:                     {"2"},
		"create_" + config.VenueNameParam:        {"Launch Hall"},
		"create_" + config.VenueAddressParam:     {"100 Test Avenue"},
		"create_" + config.VenueCapacityParam:    {"40"},
		"create_" + config.VenueDescriptionParam: {"Main hall"},
	}
	request := testsupport.Request(testingContext, http.MethodPost, config.WebEvents, formValues, &owner)
	responseRecorder := httptest.NewRecorder()

	event.CreateHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusSeeOther {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusSeeOther, responseRecorder.Body.String())
	}

	var storedVenue models.Venue
	if findError := fixture.Database.Where("name = ?", "Launch Hall").First(&storedVenue).Error; findError != nil {
		testingContext.Fatalf("find committed venue: %v", findError)
	}
	var storedEvent models.Event
	if findError := fixture.Database.Where("title = ?", "Launch Dinner").First(&storedEvent).Error; findError != nil {
		testingContext.Fatalf("find committed event: %v", findError)
	}
	if storedEvent.VenueID == nil || *storedEvent.VenueID != storedVenue.ID {
		testingContext.Fatalf("event venue ID = %v, want %q", storedEvent.VenueID, storedVenue.ID)
	}
	storedOwnerID, ownerError := storedEvent.OwnerID(fixture.Database)
	if ownerError != nil {
		testingContext.Fatalf("find event owner: %v", ownerError)
	}
	if storedOwnerID != owner.ID {
		testingContext.Fatalf("event owner ID = %q, want %q", storedOwnerID, owner.ID)
	}
	if storedEvent.LaneID == "" {
		testingContext.Fatal("event lane ID is empty")
	}
	if storedEvent.Timezone != testsupport.TimezoneName {
		testingContext.Fatalf("event timezone = %q, want %q", storedEvent.Timezone, testsupport.TimezoneName)
	}
	wantStart := time.Date(2030, time.January, 2, 23, 0, 0, 0, time.UTC)
	if storedEvent.StartsAt == nil || !storedEvent.StartsAt.Equal(wantStart) {
		testingContext.Fatalf("event start = %v, want %v", storedEvent.StartsAt, wantStart)
	}
	var storedOwner models.User
	if findError := fixture.Database.First(&storedOwner, "id = ?", owner.ID).Error; findError != nil {
		testingContext.Fatalf("find organizer: %v", findError)
	}
	if storedOwner.Timezone == nil || *storedOwner.Timezone != testsupport.TimezoneName {
		testingContext.Fatalf("organizer timezone = %v, want %q", storedOwner.Timezone, testsupport.TimezoneName)
	}
	var storedLane models.Lane
	if findError := fixture.Database.First(&storedLane, "id = ?", storedEvent.LaneID).Error; findError != nil {
		testingContext.Fatalf("find event lane: %v", findError)
	}
	wantEnd := wantStart.Add(2 * time.Hour)
	if storedLane.EndsAt == nil || !storedLane.EndsAt.Equal(wantEnd) {
		testingContext.Fatalf("event lane end = %v, want %v", storedLane.EndsAt, wantEnd)
	}
}

func TestEventCreationTransactionRollsBackVenueWhenEventInsertFails(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	triggerStatement := fmt.Sprintf(`
		CREATE TRIGGER reject_event_insert
		BEFORE INSERT ON %s
		BEGIN
			SELECT RAISE(ABORT, 'forced event insert failure');
		END
	`, config.TableEvents)
	if triggerError := fixture.Database.Exec(triggerStatement).Error; triggerError != nil {
		testingContext.Fatalf("create event insert failure trigger: %v", triggerError)
	}
	formValues := url.Values{
		config.TitleParam:                        {"Rollback Dinner"},
		config.StartTimeParam:                    {testsupport.FixedStartTime().Format(config.TimeLayoutHTMLForm)},
		config.TimezoneParam:                     {testsupport.TimezoneName},
		config.DurationParam:                     {"2"},
		"create_" + config.VenueNameParam:        {"Rollback Hall"},
		"create_" + config.VenueAddressParam:     {"100 Test Avenue"},
		"create_" + config.VenueCapacityParam:    {"40"},
		"create_" + config.VenueDescriptionParam: {"Must roll back"},
	}
	request := testsupport.Request(testingContext, http.MethodPost, config.WebEvents, formValues, &owner)
	responseRecorder := httptest.NewRecorder()

	event.CreateHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusInternalServerError {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusInternalServerError, responseRecorder.Body.String())
	}
	var eventCount int64
	if countError := fixture.Database.Model(&models.Event{}).Where("title = ?", "Rollback Dinner").Count(&eventCount).Error; countError != nil {
		testingContext.Fatalf("count rolled-back events: %v", countError)
	}
	if eventCount != 0 {
		testingContext.Fatalf("rolled-back event count = %d, want 0", eventCount)
	}
	var venueCount int64
	if countError := fixture.Database.Model(&models.Venue{}).Count(&venueCount).Error; countError != nil {
		testingContext.Fatalf("count rolled-back venues: %v", countError)
	}
	if venueCount != 0 {
		testingContext.Fatalf("rolled-back venue count = %d, want 0", venueCount)
	}
	var calendarCount int64
	if countError := fixture.Database.Model(&models.Calendar{}).Count(&calendarCount).Error; countError != nil {
		testingContext.Fatalf("count rolled-back calendars: %v", countError)
	}
	if calendarCount != 0 {
		testingContext.Fatalf("rolled-back calendar count = %d, want 0", calendarCount)
	}
	var laneCount int64
	if countError := fixture.Database.Model(&models.Lane{}).Count(&laneCount).Error; countError != nil {
		testingContext.Fatalf("count rolled-back lanes: %v", countError)
	}
	if laneCount != 0 {
		testingContext.Fatalf("rolled-back lane count = %d, want 0", laneCount)
	}
	var storedOwner models.User
	if findError := fixture.Database.First(&storedOwner, "id = ?", owner.ID).Error; findError != nil {
		testingContext.Fatalf("find organizer after rollback: %v", findError)
	}
	if storedOwner.Timezone != nil {
		testingContext.Fatalf("organizer timezone after rollback = %v, want nil", storedOwner.Timezone)
	}
}

func TestEventCreationRequiresClientTimezone(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	formValues := url.Values{
		config.TitleParam:     {"Timezone Required"},
		config.StartTimeParam: {testsupport.FixedStartTime().Format(config.TimeLayoutHTMLForm)},
		config.DurationParam:  {"2"},
	}
	request := testsupport.Request(testingContext, http.MethodPost, config.WebEvents, formValues, &owner)
	responseRecorder := httptest.NewRecorder()

	event.CreateHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusBadRequest {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
	var calendarCount int64
	if countError := fixture.Database.Model(&models.Calendar{}).Count(&calendarCount).Error; countError != nil {
		testingContext.Fatalf("count calendars: %v", countError)
	}
	if calendarCount != 0 {
		testingContext.Fatalf("calendar count = %d, want 0", calendarCount)
	}
}

func TestEventCreationAssignsIndependentAndDependentLanes(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm timezone: %v", confirmationError)
	}
	firstCalendar, calendarError := models.NewCalendar(owner.ID, "Personal", "P", "personal", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct first calendar: %v", calendarError)
	}
	secondCalendar, calendarError := models.NewCalendar(owner.ID, "Travel", "T", "travel", 1)
	if calendarError != nil {
		testingContext.Fatalf("construct second calendar: %v", calendarError)
	}
	if createError := fixture.Database.Create(firstCalendar).Error; createError != nil {
		testingContext.Fatalf("create first calendar: %v", createError)
	}
	if createError := fixture.Database.Create(secondCalendar).Error; createError != nil {
		testingContext.Fatalf("create second calendar: %v", createError)
	}

	createEvent := func(title string, calendarID string, anchorID string, startOffset time.Duration) *httptest.ResponseRecorder {
		formValues := url.Values{
			config.TitleParam:         {title},
			config.StartTimeParam:     {testsupport.FixedStartTime().Add(startOffset).Format(config.TimeLayoutHTMLForm)},
			config.TimezoneParam:      {testsupport.TimezoneName},
			config.DurationParam:      {"2"},
			config.CalendarIDParam:    {calendarID},
			config.AnchorEventIDParam: {anchorID},
		}
		request := testsupport.Request(testingContext, http.MethodPost, config.WebEvents, formValues, &owner)
		responseRecorder := httptest.NewRecorder()
		event.CreateHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)
		return responseRecorder
	}

	if response := createEvent("First independent", firstCalendar.ID, "", 0); response.Code != http.StatusSeeOther {
		testingContext.Fatalf("first independent status = %d; body = %s", response.Code, response.Body.String())
	}
	if response := createEvent("Second independent", firstCalendar.ID, "", 24*time.Hour); response.Code != http.StatusSeeOther {
		testingContext.Fatalf("second independent status = %d; body = %s", response.Code, response.Body.String())
	}
	var firstEvent models.Event
	if findError := fixture.Database.Where("title = ?", "First independent").First(&firstEvent).Error; findError != nil {
		testingContext.Fatalf("find first independent event: %v", findError)
	}
	var secondEvent models.Event
	if findError := fixture.Database.Where("title = ?", "Second independent").First(&secondEvent).Error; findError != nil {
		testingContext.Fatalf("find second independent event: %v", findError)
	}
	if firstEvent.LaneID == secondEvent.LaneID {
		testingContext.Fatalf("independent events use lane %q", firstEvent.LaneID)
	}

	if response := createEvent("Dependent", firstCalendar.ID, firstEvent.ID, 48*time.Hour); response.Code != http.StatusSeeOther {
		testingContext.Fatalf("dependent status = %d; body = %s", response.Code, response.Body.String())
	}
	var dependentEvent models.Event
	if findError := fixture.Database.Where("title = ?", "Dependent").First(&dependentEvent).Error; findError != nil {
		testingContext.Fatalf("find dependent event: %v", findError)
	}
	if dependentEvent.LaneID != firstEvent.LaneID || dependentEvent.AnchorEventID == nil || *dependentEvent.AnchorEventID != firstEvent.ID {
		testingContext.Fatalf("dependent relationship = lane %q, anchor %v", dependentEvent.LaneID, dependentEvent.AnchorEventID)
	}
	var dependencyLane models.Lane
	if findError := fixture.Database.First(&dependencyLane, "id = ?", firstEvent.LaneID).Error; findError != nil {
		testingContext.Fatalf("reload dependency lane: %v", findError)
	}
	wantEnd := time.Date(2030, time.January, 5, 1, 0, 0, 0, time.UTC)
	if dependencyLane.EndsAt == nil || !dependencyLane.EndsAt.Equal(wantEnd) {
		testingContext.Fatalf("dependency lane end = %v, want %v", dependencyLane.EndsAt, wantEnd)
	}

	if response := createEvent("Invalid dependent", secondCalendar.ID, firstEvent.ID, 72*time.Hour); response.Code != http.StatusBadRequest {
		testingContext.Fatalf("calendar mismatch status = %d, want %d; body = %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
}

func TestEventListSuppliesClientTimezoneDefault(testingContext *testing.T) {
	testsupport.LoadTemplates(testingContext)
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	request := testsupport.Request(testingContext, http.MethodGet, config.WebEvents, nil, &owner)
	responseRecorder := httptest.NewRecorder()

	event.ListEventsHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusOK, responseRecorder.Body.String())
	}
	responseBody := responseRecorder.Body.String()
	if !strings.Contains(responseBody, `name="`+config.TimezoneParam+`"`) {
		testingContext.Fatalf("event form has no %q input", config.TimezoneParam)
	}
	if !strings.Contains(responseBody, "Intl.DateTimeFormat().resolvedOptions().timeZone") {
		testingContext.Fatal("event form has no client timezone default")
	}
}

func TestEventUpdateRecalculatesFiniteLaneEnd(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	formValues := url.Values{
		config.EventIDParam:     {eventRecord.ID},
		config.TitleParam:       {"Updated event"},
		config.DescriptionParam: {"Updated description."},
		config.StartTimeParam:   {"2030-01-03T10:00"},
		config.TimezoneParam:    {testsupport.TimezoneName},
		config.DurationParam:    {"3"},
		config.VenueIDParam:     {""},
	}
	request := testsupport.Request(testingContext, http.MethodPut, config.WebEvents, formValues, &owner)
	responseRecorder := httptest.NewRecorder()

	event.UpdateEventHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusSeeOther {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusSeeOther, responseRecorder.Body.String())
	}
	var storedEvent models.Event
	if findError := fixture.Database.First(&storedEvent, "id = ?", eventRecord.ID).Error; findError != nil {
		testingContext.Fatalf("find updated event: %v", findError)
	}
	wantStart := time.Date(2030, time.January, 3, 18, 0, 0, 0, time.UTC)
	wantEnd := wantStart.Add(3 * time.Hour)
	if storedEvent.StartsAt == nil || !storedEvent.StartsAt.Equal(wantStart) {
		testingContext.Fatalf("updated event start = %v, want %v", storedEvent.StartsAt, wantStart)
	}
	if storedEvent.EndsAt == nil || !storedEvent.EndsAt.Equal(wantEnd) {
		testingContext.Fatalf("updated event end = %v, want %v", storedEvent.EndsAt, wantEnd)
	}
	var storedLane models.Lane
	if findError := fixture.Database.First(&storedLane, "id = ?", storedEvent.LaneID).Error; findError != nil {
		testingContext.Fatalf("find updated event lane: %v", findError)
	}
	if storedLane.EndsAt == nil || !storedLane.EndsAt.Equal(wantEnd) {
		testingContext.Fatalf("updated lane end = %v, want %v", storedLane.EndsAt, wantEnd)
	}
	if storedLane.Title != "Updated event" {
		testingContext.Fatalf("updated lane title = %q, want %q", storedLane.Title, "Updated event")
	}
}

func TestEventUpdateRejectsProviderOwnedMarker(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	var lane models.Lane
	if findError := fixture.Database.First(&lane, "id = ?", eventRecord.LaneID).Error; findError != nil {
		testingContext.Fatalf("find event lane: %v", findError)
	}
	connection, connectionError := models.NewCalendarConnection(owner.ID, make([]byte, 12), []byte{1})
	if connectionError != nil {
		testingContext.Fatalf("construct connection: %v", connectionError)
	}
	if createError := fixture.Database.Create(connection).Error; createError != nil {
		testingContext.Fatalf("create connection: %v", createError)
	}
	mapping, mappingError := models.NewSourceCalendarMapping(connection.ID, lane.CalendarID, "source", models.SourceCalendarGroupCalendar)
	if mappingError != nil {
		testingContext.Fatalf("construct source mapping: %v", mappingError)
	}
	if createError := fixture.Database.Create(mapping).Error; createError != nil {
		testingContext.Fatalf("create source mapping: %v", createError)
	}
	link, linkError := models.NewExternalEventLink(mapping.ID, eventRecord.ID, "provider-event", nil)
	if linkError != nil {
		testingContext.Fatalf("construct external event link: %v", linkError)
	}
	if createError := fixture.Database.Create(link).Error; createError != nil {
		testingContext.Fatalf("create external event link: %v", createError)
	}
	formValues := url.Values{
		config.EventIDParam: {eventRecord.ID}, config.TitleParam: {"Local overwrite"}, config.DescriptionParam: {""},
		config.StartTimeParam: {"2030-01-03T10:00"}, config.TimezoneParam: {testsupport.TimezoneName}, config.DurationParam: {"1"}, config.VenueIDParam: {""},
	}
	request := testsupport.Request(testingContext, http.MethodPut, config.WebEvents, formValues, &owner)
	responseRecorder := httptest.NewRecorder()
	event.UpdateEventHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusConflict {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusConflict, responseRecorder.Body.String())
	}
	var stored models.Event
	if findError := fixture.Database.First(&stored, "id = ?", eventRecord.ID).Error; findError != nil {
		testingContext.Fatalf("reload source-owned event: %v", findError)
	}
	if stored.Title != eventRecord.Title {
		testingContext.Fatalf("source-owned title = %q, want %q", stored.Title, eventRecord.Title)
	}
}

func TestEventDeletionTransactionDeletesEventAndRSVPs(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	rsvpRecord := fixture.CreateRSVP(testsupport.RSVPID, eventRecord.ID)
	siblingEvent := fixture.CreateEvent("EVT00002", owner.ID, nil)
	request := testsupport.Request(
		testingContext,
		http.MethodDelete,
		config.WebEvents+"?"+config.EventIDParam+"="+url.QueryEscape(eventRecord.ID),
		nil,
		&owner,
	)
	responseRecorder := httptest.NewRecorder()

	event.DeleteHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusSeeOther {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusSeeOther, responseRecorder.Body.String())
	}
	var deletedEvent models.Event
	if findError := fixture.Database.First(&deletedEvent, "id = ?", eventRecord.ID).Error; !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("find deleted event error = %v, want %v", findError, gorm.ErrRecordNotFound)
	}
	var deletedRSVP models.RSVP
	if findError := fixture.Database.First(&deletedRSVP, "id = ?", rsvpRecord.ID).Error; !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("find deleted RSVP error = %v, want %v", findError, gorm.ErrRecordNotFound)
	}
	var siblingLane models.Lane
	if findError := fixture.Database.First(&siblingLane, "id = ?", siblingEvent.LaneID).Error; findError != nil {
		testingContext.Fatalf("find sibling lane: %v", findError)
	}
	if siblingLane.DisplayOrder != 0 {
		testingContext.Fatalf("sibling lane display order = %d, want 0", siblingLane.DisplayOrder)
	}
	replacementEvent := fixture.CreateEvent("EVT00003", owner.ID, nil)
	if replacementEvent.LaneID == eventRecord.LaneID {
		testingContext.Fatalf("replacement event reused deleted lane %q", eventRecord.LaneID)
	}
	var replacementLane models.Lane
	if findError := fixture.Database.First(&replacementLane, "id = ?", replacementEvent.LaneID).Error; findError != nil {
		testingContext.Fatalf("find replacement lane: %v", findError)
	}
	if replacementLane.DisplayOrder != 1 {
		testingContext.Fatalf("replacement lane display order = %d, want 1", replacementLane.DisplayOrder)
	}
}

func TestEventDeletionRejectsAnchorWithDependents(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	anchor := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	relation, relationError := models.DependentEventRelation(anchor.ID)
	if relationError != nil {
		testingContext.Fatalf("construct dependent relation: %v", relationError)
	}
	eventTime, eventTimeError := models.NewIntervalEventTime(
		testsupport.FixedStartTime().Add(30*time.Minute),
		testsupport.FixedStartTime().Add(90*time.Minute),
		timezone,
	)
	if eventTimeError != nil {
		testingContext.Fatalf("construct dependent time: %v", eventTimeError)
	}
	dependent, dependentError := models.NewEvent(anchor.LaneID, "Dependent event", "", nil, relation, eventTime)
	if dependentError != nil {
		testingContext.Fatalf("construct dependent event: %v", dependentError)
	}
	dependent.BaseModel.ID = "EVT00002"
	if createError := dependent.Create(fixture.Database); createError != nil {
		testingContext.Fatalf("create dependent event: %v", createError)
	}
	request := testsupport.Request(
		testingContext,
		http.MethodDelete,
		config.WebEvents+"?"+config.EventIDParam+"="+url.QueryEscape(anchor.ID),
		nil,
		&owner,
	)
	responseRecorder := httptest.NewRecorder()

	event.DeleteHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusConflict {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusConflict, responseRecorder.Body.String())
	}
	for recordName, modelValue := range map[string]any{
		"anchor":    &models.Event{},
		"dependent": &models.Event{},
		"lane":      &models.Lane{},
	} {
		recordID := anchor.ID
		if recordName == "dependent" {
			recordID = dependent.ID
		} else if recordName == "lane" {
			recordID = anchor.LaneID
		}
		if findError := fixture.Database.First(modelValue, "id = ?", recordID).Error; findError != nil {
			testingContext.Fatalf("find active %s after rejected delete: %v", recordName, findError)
		}
	}
}

func TestDependentEventDeletionPreservesSurvivingDerivedMarkerBounds(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	anchor := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	relation, relationError := models.DependentEventRelation(anchor.ID)
	if relationError != nil {
		testingContext.Fatalf("construct dependent relation: %v", relationError)
	}
	dependencyEnd := testsupport.FixedStartTime().Add(4 * time.Hour)
	var dependencyLane models.Lane
	if findError := fixture.Database.First(&dependencyLane, "id = ?", anchor.LaneID).Error; findError != nil {
		testingContext.Fatalf("read dependency lane: %v", findError)
	}
	dependencyLane.EndsAt = &dependencyEnd
	if updateError := fixture.Database.Save(&dependencyLane).Error; updateError != nil {
		testingContext.Fatalf("expand dependency lane: %v", updateError)
	}
	eventTime, eventTimeError := models.NewIntervalEventTime(
		testsupport.FixedStartTime().Add(2*time.Hour),
		dependencyEnd,
		timezone,
	)
	if eventTimeError != nil {
		testingContext.Fatalf("construct dependent time: %v", eventTimeError)
	}
	dependent, dependentError := models.NewEvent(anchor.LaneID, "Dependent event", "", nil, relation, eventTime)
	if dependentError != nil {
		testingContext.Fatalf("construct dependent event: %v", dependentError)
	}
	dependent.BaseModel.ID = "EVT00002"
	if createError := dependent.Create(fixture.Database); createError != nil {
		testingContext.Fatalf("create dependent event: %v", createError)
	}
	derivedService, serviceError := services.NewDerivedMarkerService(fixture.Database)
	if serviceError != nil {
		testingContext.Fatalf("construct derived marker service: %v", serviceError)
	}
	_, marker, markerError := derivedService.Create(context.Background(), owner.ID, models.DerivedAnchorEvent, anchor.ID, models.DerivedAnchorEnd, int64((5 * time.Hour).Seconds()))
	if markerError != nil {
		testingContext.Fatalf("create surviving marker: %v", markerError)
	}
	request := testsupport.Request(
		testingContext,
		http.MethodDelete,
		config.WebEvents+"?"+config.EventIDParam+"="+url.QueryEscape(dependent.ID),
		nil,
		&owner,
	)
	responseRecorder := httptest.NewRecorder()

	event.DeleteHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusSeeOther {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusSeeOther, responseRecorder.Body.String())
	}
	var storedMarker models.DerivedMarker
	if findError := fixture.Database.First(&storedMarker, "id = ?", marker.ID).Error; findError != nil {
		testingContext.Fatalf("reload surviving marker: %v", findError)
	}
	var storedLane models.Lane
	if findError := fixture.Database.First(&storedLane, "id = ?", anchor.LaneID).Error; findError != nil {
		testingContext.Fatalf("reload dependency lane: %v", findError)
	}
	if storedLane.EndsAt == nil || !storedLane.EndsAt.Equal(marker.At) {
		testingContext.Fatalf("dependency lane end = %v, want surviving marker %v", storedLane.EndsAt, marker.At)
	}
}

func TestEventOwnershipRejectsOtherOwner(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	otherOwner := fixture.CreateUser(testsupport.OtherUserID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	request := testsupport.Request(
		testingContext,
		http.MethodDelete,
		config.WebEvents+"?"+config.EventIDParam+"="+url.QueryEscape(eventRecord.ID),
		nil,
		&otherOwner,
	)
	responseRecorder := httptest.NewRecorder()

	event.DeleteHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusForbidden {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusForbidden, responseRecorder.Body.String())
	}
	var storedEvent models.Event
	if findError := fixture.Database.First(&storedEvent, "id = ?", eventRecord.ID).Error; findError != nil {
		testingContext.Fatalf("find event after rejected delete: %v", findError)
	}
}
