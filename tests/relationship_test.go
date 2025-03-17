package tests

import (
	"net/http"
	"testing"

	"github.com/temirov/RSVP/models"
)

// TestEventDeletionCascade tests that RSVPs are deleted when their parent event is deleted
func TestEventDeletionCascade(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()
	
	// Create multiple RSVPs for the event
	rsvp1 := testContext.CreateTestRSVP(event.ID)
	rsvp2 := testContext.CreateTestRSVP(event.ID)
	
	// Verify RSVPs exist before deletion
	var initialRsvpCount int64
	testContext.DB.Model(&models.RSVP{}).Where("id IN ?", []string{rsvp1.ID, rsvp2.ID}).Count(&initialRsvpCount)
	if initialRsvpCount != 2 {
		testingContext.Errorf("Expected 2 RSVPs before deletion, but found %d", initialRsvpCount)
	}
	
	// Delete the event
	resp := testContext.DeleteEvent(testingContext, event.ID)
	defer resp.Body.Close()
	
	// Verify response
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := ReadResponseBody(testingContext, resp)
		testingContext.Errorf("Expected status 303, 204, or 200, got %d: %s", resp.StatusCode, body)
	}
	
	// Verify event was deleted
	var eventCount int64
	testContext.DB.Model(&models.Event{}).Where("id = ?", event.ID).Count(&eventCount)
	if eventCount != 0 {
		testingContext.Errorf("Expected event to be deleted, but it still exists")
	}
	
	// Verify RSVPs were also deleted (cascade)
	var rsvpCount int64
	testContext.DB.Model(&models.RSVP{}).Where("id IN ?", []string{rsvp1.ID, rsvp2.ID}).Count(&rsvpCount)
	if rsvpCount != 0 {
		testingContext.Errorf("Expected RSVPs to be deleted with event, but %d still exist", rsvpCount)
	}
}

// TestMultiEventRSVPIndependence tests that RSVPs for one event are not affected when another event is deleted
func TestMultiEventRSVPIndependence(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create two test events
	event1 := testContext.CreateTestEvent()
	event2 := testContext.CreateTestEvent()
	
	// Create RSVPs for both events
	rsvp1 := testContext.CreateTestRSVP(event1.ID)
	rsvp2 := testContext.CreateTestRSVP(event2.ID)
	
	// Delete the first event
	resp := testContext.DeleteEvent(testingContext, event1.ID)
	defer resp.Body.Close()
	
	// Verify response
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := ReadResponseBody(testingContext, resp)
		testingContext.Errorf("Expected status 303, 204, or 200, got %d: %s", resp.StatusCode, body)
	}
	
	// Verify first event was deleted
	var event1Count int64
	testContext.DB.Model(&models.Event{}).Where("id = ?", event1.ID).Count(&event1Count)
	if event1Count != 0 {
		testingContext.Errorf("Expected event1 to be deleted, but it still exists")
	}
	
	// Verify first event's RSVP was deleted
	var rsvp1Count int64
	testContext.DB.Model(&models.RSVP{}).Where("id = ?", rsvp1.ID).Count(&rsvp1Count)
	if rsvp1Count != 0 {
		testingContext.Errorf("Expected RSVP for event1 to be deleted, but it still exists")
	}
	
	// Verify second event still exists
	var event2Count int64
	testContext.DB.Model(&models.Event{}).Where("id = ?", event2.ID).Count(&event2Count)
	if event2Count != 1 {
		testingContext.Errorf("Expected event2 to still exist, but it was deleted")
	}
	
	// Verify second event's RSVP still exists
	var rsvp2Count int64
	testContext.DB.Model(&models.RSVP{}).Where("id = ?", rsvp2.ID).Count(&rsvp2Count)
	if rsvp2Count != 1 {
		testingContext.Errorf("Expected RSVP for event2 to still exist, but it was deleted")
	}
}

// TestMultipleRSVPsPerEvent tests creating and managing multiple RSVPs for a single event
func TestMultipleRSVPsPerEvent(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()
	
	// Create multiple RSVPs with different names
	rsvpNames := []string{"Attendee One", "Attendee Two", "Attendee Three"}
	rsvpIDs := make([]string, len(rsvpNames))
	
	for i, name := range rsvpNames {
		// Create RSVP with this name
		formValues := map[string][]string{
			"name": {name},
		}
		
		resp := testContext.CreateRSVP(testingContext, event.ID, formValues, true)
		defer resp.Body.Close()
		
		// Verify response
		if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			body := ReadResponseBody(testingContext, resp)
			testingContext.Errorf("Expected status 303, 201, or 200, got %d: %s", resp.StatusCode, body)
		}
		
		// Find the created RSVP
		var rsvp models.RSVP
		result := testContext.DB.Where("name = ? AND event_id = ?", name, event.ID).First(&rsvp)
		if result.Error != nil {
			testingContext.Errorf("Failed to find created RSVP: %v", result.Error)
		}
		
		// Store the RSVP ID
		rsvpIDs[i] = rsvp.ID
	}
	
	// Verify all RSVPs exist in the database
	var count int64
	testContext.DB.Model(&models.RSVP{}).Where("event_id = ?", event.ID).Count(&count)
	if count != int64(len(rsvpNames)) {
		testingContext.Errorf("Expected %d RSVPs, got %d", len(rsvpNames), count)
	}
	
	// Test listing RSVPs for the event
	resp := testContext.GetRSVP(testingContext, "", event.ID)
	defer resp.Body.Close()
	
	// Verify response
	if resp.StatusCode != http.StatusOK {
		testingContext.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// TestRSVPResponseChanges tests changing RSVP responses and guest counts
func TestRSVPResponseChanges(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()
	
	// Create a test RSVP
	rsvp := testContext.CreateTestRSVP(event.ID)
	
	// Test cases for different response changes
	responseChanges := []struct {
		name        string
		response    string
		extraGuests int
	}{
		{"Initial No Response", "No", 0},
		{"Change to Yes with no guests", "Yes,0", 0},
		{"Change to Yes with 2 guests", "Yes,2", 2},
		{"Change to Yes with 4 guests", "Yes,4", 4},
		{"Change back to No", "No", 0},
	}
	
	for _, change := range responseChanges {
		testingContext.Run(change.name, func(subTestContext *testing.T) {
			// Update RSVP with this response
			formValues := map[string][]string{
				"response": {change.response},
			}
			
			resp := testContext.UpdateRSVP(subTestContext, rsvp.ID, event.ID, formValues)
			defer resp.Body.Close()
			
			// Verify response
			if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
				body := ReadResponseBody(subTestContext, resp)
				subTestContext.Errorf("Expected status 303 or 200, got %d: %s", resp.StatusCode, body)
			}
			
			// Verify RSVP was updated in database
			var updatedRSVP models.RSVP
			testContext.DB.First(&updatedRSVP, "id = ?", rsvp.ID)
			
			if updatedRSVP.Response != change.response {
				subTestContext.Errorf("Expected response '%s', got '%s'", change.response, updatedRSVP.Response)
			}
		})
	}
}
