package event_test

import (
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

func TestEventDeletionTransactionDeletesEventAndRSVPs(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	rsvpRecord := fixture.CreateRSVP(testsupport.RSVPID, eventRecord.ID)
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
	replacementEvent := fixture.CreateEvent("EVT00002", owner.ID, nil)
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
