package tests

import (
	"testing"

	"github.com/temirov/RSVP/models"
)

// TestEventDelegation tests transferring event ownership between users.
func TestEventDelegation(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Create a test event owned by the default test user
	event := testContext.CreateTestEvent()

	// Create RSVPs for the event
	rsvp1 := testContext.CreateTestRSVP(event.ID)
	rsvp2 := testContext.CreateTestRSVP(event.ID)

	// Create a second user to delegate the event to
	secondUser := &models.User{
		Email:   "delegate@example.com",
		Name:    "Delegate User",
		Picture: "https://example.com/delegate.jpg",
	}
	testContext.DB.Create(secondUser)

	// Verify the event is owned by the original user
	var originalEvent models.Event
	testContext.DB.First(&originalEvent, "id = ?", event.ID)
	if originalEvent.UserID != testContext.TestUser.ID {
		testingContext.Errorf("Expected event to be owned by user %s, but it's owned by %s",
			testContext.TestUser.ID, originalEvent.UserID)
	}

	// Delegate the event to the second user
	// This would typically be done through a form submission or API call
	// For this test, we'll update the database directly
	originalEvent.UserID = secondUser.ID
	if updateError := testContext.DB.Save(&originalEvent).Error; updateError != nil {
		testingContext.Fatalf("Failed to update event ownership: %v", updateError)
	}

	// Verify the event is now owned by the second user
	var updatedEvent models.Event
	testContext.DB.First(&updatedEvent, "id = ?", event.ID)
	if updatedEvent.UserID != secondUser.ID {
		testingContext.Errorf("Expected event to be owned by user %s after delegation, but it's owned by %s",
			secondUser.ID, updatedEvent.UserID)
	}

	// Verify RSVPs still exist and are associated with the event
	var rsvpCount int64
	testContext.DB.Model(&models.RSVP{}).Where("event_id = ?", event.ID).Count(&rsvpCount)
	if rsvpCount != 2 {
		testingContext.Errorf("Expected 2 RSVPs after delegation, but found %d", rsvpCount)
	}

	// Verify specific RSVPs still exist
	var rsvp1Count, rsvp2Count int64
	testContext.DB.Model(&models.RSVP{}).Where("id = ?", rsvp1.ID).Count(&rsvp1Count)
	testContext.DB.Model(&models.RSVP{}).Where("id = ?", rsvp2.ID).Count(&rsvp2Count)

	if rsvp1Count != 1 || rsvp2Count != 1 {
		testingContext.Errorf("Expected both RSVPs to exist after delegation, but found %d and %d", rsvp1Count, rsvp2Count)
	}
}

// TestEventDelegationWithRSVPUpdates tests that RSVPs can be updated after event delegation.
func TestEventDelegationWithRSVPUpdates(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Create a test event owned by the default test user
	event := testContext.CreateTestEvent()

	// Create an RSVP for the event
	rsvp := testContext.CreateTestRSVP(event.ID)

	// Create a second user to delegate the event to
	secondUser := &models.User{
		Email:   "delegate@example.com",
		Name:    "Delegate User",
		Picture: "https://example.com/delegate.jpg",
	}
	testContext.DB.Create(secondUser)

	// Delegate the event to the second user
	event.UserID = secondUser.ID
	if updateError := testContext.DB.Save(&event).Error; updateError != nil {
		testingContext.Fatalf("Failed to update event ownership: %v", updateError)
	}

	// Create a test session for the second user
	// In a real implementation, we would need to modify the test auth middleware
	// For this test, we'll simulate by directly updating the RSVP

	// Update the RSVP after delegation
	rsvp.Response = "Yes,2"
	if updateError := testContext.DB.Save(&rsvp).Error; updateError != nil {
		testingContext.Fatalf("Failed to update RSVP after delegation: %v", updateError)
	}

	// Verify the RSVP was updated
	var updatedRSVP models.RSVP
	testContext.DB.First(&updatedRSVP, "id = ?", rsvp.ID)
	if updatedRSVP.Response != "Yes,2" {
		testingContext.Errorf("Expected RSVP response to be 'Yes,2' after delegation, but got '%s'", updatedRSVP.Response)
	}
}

// TestMultipleEventDelegation tests delegating multiple events between users.
func TestMultipleEventDelegation(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Create multiple test events owned by the default test user
	event1 := testContext.CreateTestEvent()
	event2 := testContext.CreateTestEvent()
	event3 := testContext.CreateTestEvent()

	// Create RSVPs for each event
	rsvp1 := testContext.CreateTestRSVP(event1.ID)
	rsvp2 := testContext.CreateTestRSVP(event2.ID)
	rsvp3 := testContext.CreateTestRSVP(event3.ID)

	// Create a second user to delegate events to
	secondUser := &models.User{
		Email:   "delegate@example.com",
		Name:    "Delegate User",
		Picture: "https://example.com/delegate.jpg",
	}
	testContext.DB.Create(secondUser)

	// Delegate events 1 and 3 to the second user, leaving event 2 with the original user
	event1.UserID = secondUser.ID
	if updateError := testContext.DB.Save(&event1).Error; updateError != nil {
		testingContext.Fatalf("Failed to update event1 ownership: %v", updateError)
	}

	event3.UserID = secondUser.ID
	if updateError := testContext.DB.Save(&event3).Error; updateError != nil {
		testingContext.Fatalf("Failed to update event3 ownership: %v", updateError)
	}

	// Verify event ownerships
	var updatedEvent1, updatedEvent2, updatedEvent3 models.Event
	testContext.DB.First(&updatedEvent1, "id = ?", event1.ID)
	testContext.DB.First(&updatedEvent2, "id = ?", event2.ID)
	testContext.DB.First(&updatedEvent3, "id = ?", event3.ID)

	if updatedEvent1.UserID != secondUser.ID {
		testingContext.Errorf("Expected event1 to be owned by second user, but it's owned by %s", updatedEvent1.UserID)
	}

	if updatedEvent2.UserID != testContext.TestUser.ID {
		testingContext.Errorf("Expected event2 to be owned by original user, but it's owned by %s", updatedEvent2.UserID)
	}

	if updatedEvent3.UserID != secondUser.ID {
		testingContext.Errorf("Expected event3 to be owned by second user, but it's owned by %s", updatedEvent3.UserID)
	}

	// Verify RSVPs still exist and are associated with the correct events
	var rsvp1Count, rsvp2Count, rsvp3Count int64
	testContext.DB.Model(&models.RSVP{}).Where("id = ? AND event_id = ?", rsvp1.ID, event1.ID).Count(&rsvp1Count)
	testContext.DB.Model(&models.RSVP{}).Where("id = ? AND event_id = ?", rsvp2.ID, event2.ID).Count(&rsvp2Count)
	testContext.DB.Model(&models.RSVP{}).Where("id = ? AND event_id = ?", rsvp3.ID, event3.ID).Count(&rsvp3Count)

	if rsvp1Count != 1 || rsvp2Count != 1 || rsvp3Count != 1 {
		testingContext.Errorf("Expected all RSVPs to exist with correct event associations after delegation, but found %d, %d, and %d",
			rsvp1Count, rsvp2Count, rsvp3Count)
	}
}
