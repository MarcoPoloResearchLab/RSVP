package event_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
	if storedEvent.UserID != owner.ID {
		testingContext.Fatalf("event owner ID = %q, want %q", storedEvent.UserID, owner.ID)
	}
}

func TestEventCreationTransactionRollsBackInvalidVenue(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	formValues := url.Values{
		config.TitleParam:                 {"Rollback Dinner"},
		config.StartTimeParam:             {testsupport.FixedStartTime().Format(config.TimeLayoutHTMLForm)},
		config.DurationParam:              {"2"},
		"create_" + config.VenueNameParam: {strings.Repeat("v", config.MaxVenueNameLength+1)},
	}
	request := testsupport.Request(testingContext, http.MethodPost, config.WebEvents, formValues, &owner)
	responseRecorder := httptest.NewRecorder()

	event.CreateHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusBadRequest {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
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
