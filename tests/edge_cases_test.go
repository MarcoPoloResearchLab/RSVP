package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/temirov/RSVP/models"
)

// TestConcurrentEventCreation tests creating multiple events concurrently
func TestConcurrentEventCreation(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create multiple events with the same data
	// In a real concurrent scenario, these would be created in separate goroutines
	// For this test, we'll create them sequentially but verify they get unique IDs
	eventCount := 5
	eventIDs := make([]string, eventCount)
	
	startTime := time.Now().Add(24 * time.Hour)
	
	for i := 0; i < eventCount; i++ {
		event := &models.Event{
			Title:       "Concurrent Test Event",
			Description: "Testing concurrent event creation",
			StartTime:   startTime,
			EndTime:     startTime.Add(2 * time.Hour),
			UserID:      testContext.TestUser.ID,
		}
		testContext.DB.Create(event)
		eventIDs[i] = event.ID
	}
	
	// Verify all events have unique IDs
	uniqueIDs := make(map[string]bool)
	for _, id := range eventIDs {
		if uniqueIDs[id] {
			testingContext.Errorf("Duplicate event ID found: %s", id)
		}
		uniqueIDs[id] = true
	}
	
	if len(uniqueIDs) != eventCount {
		testingContext.Errorf("Expected %d unique event IDs, got %d", eventCount, len(uniqueIDs))
	}
}

// TestConcurrentRSVPCreation tests creating multiple RSVPs concurrently
func TestConcurrentRSVPCreation(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()
	
	// Create multiple RSVPs with the same data
	// In a real concurrent scenario, these would be created in separate goroutines
	// For this test, we'll create them sequentially but verify they get unique IDs
	rsvpCount := 5
	rsvpIDs := make([]string, rsvpCount)
	
	for i := 0; i < rsvpCount; i++ {
		rsvp := &models.RSVP{
			Name:    "Concurrent Test Attendee",
			EventID: event.ID,
		}
		testContext.DB.Create(rsvp)
		rsvpIDs[i] = rsvp.ID
	}
	
	// Verify all RSVPs have unique IDs
	uniqueIDs := make(map[string]bool)
	for _, id := range rsvpIDs {
		if uniqueIDs[id] {
			testingContext.Errorf("Duplicate RSVP ID found: %s", id)
		}
		uniqueIDs[id] = true
	}
	
	if len(uniqueIDs) != rsvpCount {
		testingContext.Errorf("Expected %d unique RSVP IDs, got %d", rsvpCount, len(uniqueIDs))
	}
}

// TestNonExistentEventAccess tests accessing a non-existent event
func TestNonExistentEventAccess(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Try to access a non-existent event
	resp := testContext.GetEvent(testingContext, "nonexistent")
	defer resp.Body.Close()
	
	// Verify response
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusSeeOther {
		testingContext.Errorf("Expected status 404 or 303 for non-existent event, got %d", resp.StatusCode)
	}
}

// TestNonExistentRSVPAccess tests accessing a non-existent RSVP
func TestNonExistentRSVPAccess(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Try to access a non-existent RSVP
	resp := testContext.GetRSVP(testingContext, "nonexistent", "")
	defer resp.Body.Close()
	
	// Verify response
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusSeeOther {
		testingContext.Errorf("Expected status 404 or 303 for non-existent RSVP, got %d", resp.StatusCode)
	}
}

// TestEventWithManyRSVPs tests an event with a large number of RSVPs
func TestEventWithManyRSVPs(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()
	
	// Create a large number of RSVPs for the event
	rsvpCount := 50 // Adjust based on what's reasonable for your application
	for i := 0; i < rsvpCount; i++ {
		rsvp := &models.RSVP{
			Name:    "Attendee " + string(rune(65+i%26)) + string(rune(48+i/26)), // Generate names like A0, B0, ..., Z0, A1, B1, etc.
			EventID: event.ID,
		}
		testContext.DB.Create(rsvp)
	}
	
	// Verify all RSVPs were created
	var count int64
	testContext.DB.Model(&models.RSVP{}).Where("event_id = ?", event.ID).Count(&count)
	if count != int64(rsvpCount) {
		testingContext.Errorf("Expected %d RSVPs, got %d", rsvpCount, count)
	}
	
	// Test listing RSVPs for the event
	resp := testContext.GetRSVP(testingContext, "", event.ID)
	defer resp.Body.Close()
	
	// Verify response
	if resp.StatusCode != http.StatusOK {
		testingContext.Errorf("Expected status 200 for event with many RSVPs, got %d", resp.StatusCode)
	}
}

// TestRSVPWithSpecialCharacters tests creating and updating RSVPs with special characters in the name
func TestRSVPWithSpecialCharacters(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()
	
	// Test cases with special characters
	specialNames := []string{
		"John O'Connor",
		"María Rodríguez",
		"Zhang Wei (张伟)",
		"Müller-Schmidt",
		"Person; with semicolon",
		"Person with <html> tags",
		"Person with & ampersand",
		"Person with \" quotes",
	}
	
	for _, name := range specialNames {
		testingContext.Run(name, func(subTestContext *testing.T) {
			// Create RSVP with special character name
			formValues := map[string][]string{
				"name": {name},
			}
			
			resp := testContext.CreateRSVP(subTestContext, event.ID, formValues, true)
			defer resp.Body.Close()
			
			// Verify response
			if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				body := ReadResponseBody(subTestContext, resp)
				subTestContext.Errorf("Expected status 303, 201, or 200, got %d: %s", resp.StatusCode, body)
			}
			
			// Verify RSVP was created with correct name
			var rsvp models.RSVP
			result := testContext.DB.Where("name = ? AND event_id = ?", name, event.ID).First(&rsvp)
			if result.Error != nil {
				subTestContext.Errorf("Failed to find created RSVP with special characters: %v", result.Error)
			}
			
			if rsvp.Name != name {
				subTestContext.Errorf("Expected name '%s', got '%s'", name, rsvp.Name)
			}
			
			// Test updating the RSVP
			updateFormValues := map[string][]string{
				"name":     {name + " (updated)"},
				"response": {"Yes,2"},
			}
			
			updateResp := testContext.UpdateRSVP(subTestContext, rsvp.ID, event.ID, updateFormValues)
			defer updateResp.Body.Close()
			
			// Verify response
			if updateResp.StatusCode != http.StatusSeeOther && updateResp.StatusCode != http.StatusOK {
				body := ReadResponseBody(subTestContext, updateResp)
				subTestContext.Errorf("Expected status 303 or 200, got %d: %s", updateResp.StatusCode, body)
			}
			
			// Verify RSVP was updated
			var updatedRSVP models.RSVP
			testContext.DB.First(&updatedRSVP, "id = ?", rsvp.ID)
			
			expectedName := name + " (updated)"
			if updatedRSVP.Name != expectedName {
				subTestContext.Errorf("Expected updated name '%s', got '%s'", expectedName, updatedRSVP.Name)
			}
		})
	}
}

// TestEventTimeEdgeCases tests various edge cases with event times
func TestEventTimeEdgeCases(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Test cases for event times
	timeTestCases := []struct {
		name          string
		startTime     time.Time
		endTime       time.Time
		expectSuccess bool
	}{
		{
			name:          "Normal Future Event",
			startTime:     time.Now().Add(24 * time.Hour),
			endTime:       time.Now().Add(26 * time.Hour),
			expectSuccess: true,
		},
		{
			name:          "Event Starting Now",
			startTime:     time.Now(),
			endTime:       time.Now().Add(2 * time.Hour),
			expectSuccess: true,
		},
		{
			name:          "Very Short Event",
			startTime:     time.Now().Add(24 * time.Hour),
			endTime:       time.Now().Add(24*time.Hour + 5*time.Minute),
			expectSuccess: true,
		},
		{
			name:          "Very Long Event",
			startTime:     time.Now().Add(24 * time.Hour),
			endTime:       time.Now().Add(24*time.Hour + 72*time.Hour),
			expectSuccess: true,
		},
		{
			name:          "Far Future Event",
			startTime:     time.Now().Add(365 * 24 * time.Hour),
			endTime:       time.Now().Add(365*24*time.Hour + 2*time.Hour),
			expectSuccess: true,
		},
	}
	
	for _, testCase := range timeTestCases {
		testingContext.Run(testCase.name, func(subTestContext *testing.T) {
			// Create event with these times
			event := &models.Event{
				Title:       "Time Test: " + testCase.name,
				Description: "Testing event time edge cases",
				StartTime:   testCase.startTime,
				EndTime:     testCase.endTime,
				UserID:      testContext.TestUser.ID,
			}
			
			result := testContext.DB.Create(event)
			
			if testCase.expectSuccess {
				if result.Error != nil {
					subTestContext.Errorf("Failed to create event: %v", result.Error)
				}
				
				// Verify event was created with correct times
				var createdEvent models.Event
				testContext.DB.First(&createdEvent, "id = ?", event.ID)
				
				// Compare times with a small tolerance for database rounding
				startDiff := testCase.startTime.Sub(createdEvent.StartTime).Seconds()
				endDiff := testCase.endTime.Sub(createdEvent.EndTime).Seconds()
				
				if startDiff > 1 || startDiff < -1 {
					subTestContext.Errorf("Start time mismatch: expected %v, got %v", testCase.startTime, createdEvent.StartTime)
				}
				
				if endDiff > 1 || endDiff < -1 {
					subTestContext.Errorf("End time mismatch: expected %v, got %v", testCase.endTime, createdEvent.EndTime)
				}
			} else {
				if result.Error == nil {
					subTestContext.Errorf("Expected event creation to fail, but it succeeded")
				}
			}
		})
	}
}
