package models_test

import (
	"errors"
	"testing"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"gorm.io/gorm"
)

func TestEventOwnershipQueryReturnsOnlyOwnedEvent(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	otherOwner := fixture.CreateUser(testsupport.OtherUserID)
	eventRecord := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)

	var ownedEvent models.Event
	if findError := ownedEvent.FindByIDAndOwner(fixture.Database, eventRecord.ID, owner.ID); findError != nil {
		testingContext.Fatalf("find event for owner: %v", findError)
	}
	if ownedEvent.ID != eventRecord.ID {
		testingContext.Fatalf("event ID = %q, want %q", ownedEvent.ID, eventRecord.ID)
	}

	var foreignEvent models.Event
	findError := foreignEvent.FindByIDAndOwner(fixture.Database, eventRecord.ID, otherOwner.ID)
	if !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("find event for other owner error = %v, want %v", findError, gorm.ErrRecordNotFound)
	}
}

func TestVenueOwnershipQueryReturnsOnlyOwnedVenue(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	otherOwner := fixture.CreateUser(testsupport.OtherUserID)
	venueRecord := fixture.CreateVenue(testsupport.VenueID, owner.ID)

	var ownedVenue models.Venue
	if findError := ownedVenue.FindByIDAndOwner(fixture.Database, venueRecord.ID, owner.ID); findError != nil {
		testingContext.Fatalf("find venue for owner: %v", findError)
	}
	if ownedVenue.ID != venueRecord.ID {
		testingContext.Fatalf("venue ID = %q, want %q", ownedVenue.ID, venueRecord.ID)
	}

	var foreignVenue models.Venue
	findError := foreignVenue.FindByIDAndOwner(fixture.Database, venueRecord.ID, otherOwner.ID)
	if !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("find venue for other owner error = %v, want %v", findError, gorm.ErrRecordNotFound)
	}
}
