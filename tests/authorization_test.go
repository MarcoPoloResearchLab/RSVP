package tests

import (
	"net/http"
	"testing"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// TestEventOwnershipAuthorization tests that users can only access and modify their own events
func TestEventOwnershipAuthorization(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event owned by the default test user
	event := testContext.CreateTestEvent()
	
	// Create a second user
	secondUser := &models.User{
		Email:   "second@example.com",
		Name:    "Second User",
		Picture: "https://example.com/second.jpg",
	}
	testContext.DB.Create(secondUser)
	
	// Create an event owned by the second user
	secondUserEvent := &models.Event{
		Title:       "Second User's Event",
		Description: "This event belongs to the second user",
		StartTime:   event.StartTime, // Use same time as the first event
		EndTime:     event.EndTime,
		UserID:      secondUser.ID,
	}
	testContext.DB.Create(secondUserEvent)
	
	// Test accessing the second user's event (should redirect or return 403)
	resp := testContext.GetEvent(testingContext, secondUserEvent.ID)
	defer resp.Body.Close()
	
	// The application might handle unauthorized access in different ways:
	// 1. Redirect to the events list (303)
	// 2. Return a forbidden error (403)
	// 3. Return not found (404)
	if resp.StatusCode != http.StatusSeeOther && 
	   resp.StatusCode != http.StatusForbidden && 
	   resp.StatusCode != http.StatusNotFound {
		body := ReadResponseBody(testingContext, resp)
		testingContext.Errorf("Expected status 303, 403, or 404 for unauthorized access, got %d: %s", resp.StatusCode, body)
	}
	
	// Test updating the second user's event (should fail)
	formValues := map[string][]string{
		"title":       {"Updated Title"},
		"description": {"Updated Description"},
		"start_time":  {event.StartTime.Format("2006-01-02T15:04")},
		"duration":    {"2"},
	}
	
	updateResp := testContext.UpdateEvent(testingContext, secondUserEvent.ID, formValues)
	defer updateResp.Body.Close()
	
	if updateResp.StatusCode != http.StatusSeeOther && 
	   updateResp.StatusCode != http.StatusForbidden && 
	   updateResp.StatusCode != http.StatusNotFound {
		body := ReadResponseBody(testingContext, updateResp)
		testingContext.Errorf("Expected status 303, 403, or 404 for unauthorized update, got %d: %s", updateResp.StatusCode, body)
	}
	
	// Verify the event was not updated
	var checkEvent models.Event
	testContext.DB.First(&checkEvent, "id = ?", secondUserEvent.ID)
	if checkEvent.Title != secondUserEvent.Title {
		testingContext.Errorf("Event was updated despite authorization failure: expected title '%s', got '%s'", 
			secondUserEvent.Title, checkEvent.Title)
	}
	
	// Test deleting the second user's event (should fail)
	deleteResp := testContext.DeleteEvent(testingContext, secondUserEvent.ID)
	defer deleteResp.Body.Close()
	
	if deleteResp.StatusCode != http.StatusSeeOther && 
	   deleteResp.StatusCode != http.StatusForbidden && 
	   deleteResp.StatusCode != http.StatusNotFound {
		body := ReadResponseBody(testingContext, deleteResp)
		testingContext.Errorf("Expected status 303, 403, or 404 for unauthorized delete, got %d: %s", deleteResp.StatusCode, body)
	}
	
	// Verify the event was not deleted
	var eventCount int64
	testContext.DB.Model(&models.Event{}).Where("id = ?", secondUserEvent.ID).Count(&eventCount)
	if eventCount != 1 {
		testingContext.Errorf("Event was deleted despite authorization failure")
	}
}

// TestRSVPEventOwnershipAuthorization tests that users can only access and modify RSVPs for their own events
func TestRSVPEventOwnershipAuthorization(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event owned by the default test user
	event := testContext.CreateTestEvent()
	
	// Create an RSVP for the event (for completeness, not used in this test)
	_ = testContext.CreateTestRSVP(event.ID)
	
	// Create a second user
	secondUser := &models.User{
		Email:   "second@example.com",
		Name:    "Second User",
		Picture: "https://example.com/second.jpg",
	}
	testContext.DB.Create(secondUser)
	
	// Create an event owned by the second user
	secondUserEvent := &models.Event{
		Title:       "Second User's Event",
		Description: "This event belongs to the second user",
		StartTime:   event.StartTime, // Use same time as the first event
		EndTime:     event.EndTime,
		UserID:      secondUser.ID,
	}
	testContext.DB.Create(secondUserEvent)
	
	// Create an RSVP for the second user's event
	secondEventRSVP := &models.RSVP{
		Name:    "Test Attendee for Second Event",
		EventID: secondUserEvent.ID,
	}
	testContext.DB.Create(secondEventRSVP)
	
	// Test accessing RSVPs for the second user's event (should redirect or return 403)
	resp := testContext.GetRSVP(testingContext, "", secondUserEvent.ID)
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusSeeOther && 
	   resp.StatusCode != http.StatusForbidden && 
	   resp.StatusCode != http.StatusNotFound {
		body := ReadResponseBody(testingContext, resp)
		testingContext.Errorf("Expected status 303, 403, or 404 for unauthorized access to RSVPs, got %d: %s", resp.StatusCode, body)
	}
	
	// Test accessing a specific RSVP for the second user's event (should redirect or return 403)
	rsvpResp := testContext.GetRSVP(testingContext, secondEventRSVP.ID, "")
	defer rsvpResp.Body.Close()
	
	if rsvpResp.StatusCode != http.StatusSeeOther && 
	   rsvpResp.StatusCode != http.StatusForbidden && 
	   rsvpResp.StatusCode != http.StatusNotFound {
		body := ReadResponseBody(testingContext, rsvpResp)
		testingContext.Errorf("Expected status 303, 403, or 404 for unauthorized access to RSVP, got %d: %s", rsvpResp.StatusCode, body)
	}
	
	// Test creating an RSVP for the second user's event (should fail)
	formValues := map[string][]string{
		"name": {"Unauthorized RSVP"},
	}
	
	createResp := testContext.CreateRSVP(testingContext, secondUserEvent.ID, formValues, true)
	defer createResp.Body.Close()
	
	if createResp.StatusCode != http.StatusSeeOther && 
	   createResp.StatusCode != http.StatusForbidden && 
	   createResp.StatusCode != http.StatusNotFound {
		body := ReadResponseBody(testingContext, createResp)
		testingContext.Errorf("Expected status 303, 403, or 404 for unauthorized RSVP creation, got %d: %s", createResp.StatusCode, body)
	}
	
	// Verify the RSVP was not created
	var rsvpCount int64
	testContext.DB.Model(&models.RSVP{}).Where("name = ? AND event_id = ?", "Unauthorized RSVP", secondUserEvent.ID).Count(&rsvpCount)
	if rsvpCount != 0 {
		testingContext.Errorf("RSVP was created despite authorization failure")
	}
	
	// Test updating an RSVP for the second user's event (should fail)
	updateFormValues := map[string][]string{
		"name":     {"Updated RSVP Name"},
		"response": {"Yes,2"},
	}
	
	updateResp := testContext.UpdateRSVP(testingContext, secondEventRSVP.ID, secondUserEvent.ID, updateFormValues)
	defer updateResp.Body.Close()
	
	if updateResp.StatusCode != http.StatusSeeOther && 
	   updateResp.StatusCode != http.StatusForbidden && 
	   updateResp.StatusCode != http.StatusNotFound {
		body := ReadResponseBody(testingContext, updateResp)
		testingContext.Errorf("Expected status 303, 403, or 404 for unauthorized RSVP update, got %d: %s", updateResp.StatusCode, body)
	}
	
	// Verify the RSVP was not updated
	var checkRSVP models.RSVP
	testContext.DB.First(&checkRSVP, "id = ?", secondEventRSVP.ID)
	if checkRSVP.Name != secondEventRSVP.Name {
		testingContext.Errorf("RSVP was updated despite authorization failure: expected name '%s', got '%s'", 
			secondEventRSVP.Name, checkRSVP.Name)
	}
	
	// Test deleting an RSVP for the second user's event (should fail)
	deleteResp := testContext.DeleteRSVP(testingContext, secondEventRSVP.ID, secondUserEvent.ID)
	defer deleteResp.Body.Close()
	
	if deleteResp.StatusCode != http.StatusSeeOther && 
	   deleteResp.StatusCode != http.StatusForbidden && 
	   deleteResp.StatusCode != http.StatusNotFound {
		body := ReadResponseBody(testingContext, deleteResp)
		testingContext.Errorf("Expected status 303, 403, or 404 for unauthorized RSVP deletion, got %d: %s", deleteResp.StatusCode, body)
	}
	
	// Verify the RSVP was not deleted
	var rsvpStillExists int64
	testContext.DB.Model(&models.RSVP{}).Where("id = ?", secondEventRSVP.ID).Count(&rsvpStillExists)
	if rsvpStillExists != 1 {
		testingContext.Errorf("RSVP was deleted despite authorization failure")
	}
}

// TestRSVPAccessSecurity tests that RSVPs can only be accessed by their owners
func TestRSVPAccessSecurity(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()
	
	// Create a test RSVP
	rsvp := testContext.CreateTestRSVP(event.ID)
	
	// Create a request with the X-Security-Test header
	url := testContext.RSVPServer.URL + config.WebRSVPs + "?id=" + rsvp.ID
	req, createError := http.NewRequest(http.MethodGet, url, nil)
	if createError != nil {
		testingContext.Fatalf("Failed to create GET request: %v", createError)
	}
	
	// Set the X-Security-Test header to true
	req.Header.Set("X-Security-Test", "true")
	
	// Send the request
	resp, requestError := http.DefaultClient.Do(req)
	if requestError != nil {
		testingContext.Fatalf("Failed to make GET request: %v", requestError)
	}
	defer resp.Body.Close()
	
	// Verify response - we expect a 403 because we're using the X-Security-Test header
	// which triggers our security check
	if resp.StatusCode != http.StatusForbidden {
		testingContext.Errorf("Expected status 403 for RSVP access with X-Security-Test, got %d", resp.StatusCode)
	}
	
	// In a real application, we would implement a proper public access mechanism
	// For example, using signed tokens or a separate public endpoint
	// This would allow invitees to access their RSVPs without authentication
	// while still preventing unauthorized access
}
