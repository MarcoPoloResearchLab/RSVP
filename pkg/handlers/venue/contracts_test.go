package venue_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers/venue"
	"gorm.io/gorm"
)

func TestVenueDeletionTransactionDisassociatesEvents(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	venueRecord := fixture.CreateVenue(testsupport.VenueID, owner.ID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, &venueRecord.ID)
	request := testsupport.Request(
		testingContext,
		http.MethodDelete,
		config.WebVenues+"?"+config.VenueIDParam+"="+url.QueryEscape(venueRecord.ID),
		nil,
		&owner,
	)
	responseRecorder := httptest.NewRecorder()

	venue.DeleteVenueHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusSeeOther {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusSeeOther, responseRecorder.Body.String())
	}
	var deletedVenue models.Venue
	if findError := fixture.Database.First(&deletedVenue, "id = ?", venueRecord.ID).Error; !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("find deleted venue error = %v, want %v", findError, gorm.ErrRecordNotFound)
	}
	var storedEvent models.Event
	if findError := fixture.Database.First(&storedEvent, "id = ?", eventRecord.ID).Error; findError != nil {
		testingContext.Fatalf("find event after venue deletion: %v", findError)
	}
	if storedEvent.VenueID != nil {
		testingContext.Fatalf("event venue ID = %v, want nil", storedEvent.VenueID)
	}
}

func TestVenueDeletionTransactionRollsBackDisassociationWhenDeleteFails(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	venueRecord := fixture.CreateVenue(testsupport.VenueID, owner.ID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, &venueRecord.ID)
	triggerStatement := fmt.Sprintf(`
		CREATE TRIGGER reject_venue_delete
		BEFORE UPDATE OF deleted_at ON %s
		WHEN NEW.deleted_at IS NOT NULL
		BEGIN
			SELECT RAISE(ABORT, 'forced venue delete failure');
		END
	`, config.TableVenues)
	if triggerError := fixture.Database.Exec(triggerStatement).Error; triggerError != nil {
		testingContext.Fatalf("create venue delete failure trigger: %v", triggerError)
	}
	request := testsupport.Request(
		testingContext,
		http.MethodDelete,
		config.WebVenues+"?"+config.VenueIDParam+"="+url.QueryEscape(venueRecord.ID),
		nil,
		&owner,
	)
	responseRecorder := httptest.NewRecorder()

	venue.DeleteVenueHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusInternalServerError {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusInternalServerError, responseRecorder.Body.String())
	}
	var storedVenue models.Venue
	if findError := fixture.Database.First(&storedVenue, "id = ?", venueRecord.ID).Error; findError != nil {
		testingContext.Fatalf("find venue after rolled-back deletion: %v", findError)
	}
	var storedEvent models.Event
	if findError := fixture.Database.First(&storedEvent, "id = ?", eventRecord.ID).Error; findError != nil {
		testingContext.Fatalf("find event after rolled-back deletion: %v", findError)
	}
	if storedEvent.VenueID == nil || *storedEvent.VenueID != venueRecord.ID {
		testingContext.Fatalf("event venue ID = %v, want %q", storedEvent.VenueID, venueRecord.ID)
	}
}

func TestVenueOwnershipRejectsOtherOwner(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	otherOwner := fixture.CreateUser(testsupport.OtherUserID)
	venueRecord := fixture.CreateVenue(testsupport.VenueID, owner.ID)
	request := testsupport.Request(
		testingContext,
		http.MethodDelete,
		config.WebVenues+"?"+config.VenueIDParam+"="+url.QueryEscape(venueRecord.ID),
		nil,
		&otherOwner,
	)
	responseRecorder := httptest.NewRecorder()

	venue.DeleteVenueHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusNotFound {
		testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, http.StatusNotFound, responseRecorder.Body.String())
	}
	var storedVenue models.Venue
	if findError := fixture.Database.First(&storedVenue, "id = ?", venueRecord.ID).Error; findError != nil {
		testingContext.Fatalf("find venue after rejected delete: %v", findError)
	}
}
