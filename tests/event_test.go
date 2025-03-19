package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/temirov/RSVP/models"
)

func TestListEvents(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Create some test events
	testContext.CreateTestEvent()
	testContext.CreateTestEvent()

	// Test listing events
	resp := testContext.GetEvent(testingContext, "")
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		testingContext.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify events in database
	var count int64
	testContext.DB.Model(&models.Event{}).Count(&count)
	if count < 2 {
		testingContext.Errorf("Expected at least 2 events, got %d", count)
	}
}

func TestCreateEvent(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Test creating an event
	startTime := time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")

	formValues := map[string][]string{
		"title":       {"New Test Event"},
		"description": {"New Test Description"},
		"start_time":  {startTime},
		"duration":    {"2"},
	}

	resp := testContext.CreateEvent(testingContext, formValues)
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body := ReadResponseBody(testingContext, resp)
		testingContext.Errorf("Expected status 303, 201, or 200, got %d: %s", resp.StatusCode, body)
	}

	// Verify event was created in database
	var event models.Event
	result := testContext.DB.Where("title = ?", "New Test Event").First(&event)
	if result.Error != nil {
		testingContext.Errorf("Failed to find created event: %v", result.Error)
	}
	if event.Title != "New Test Event" {
		testingContext.Errorf("Expected title 'New Test Event', got '%s'", event.Title)
	}
}

func TestShowEvent(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Create a test event
	event := testContext.CreateTestEvent()

	// Test showing the event
	resp := testContext.GetEvent(testingContext, event.ID)
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		testingContext.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestUpdateEvent(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Create a test event
	event := testContext.CreateTestEvent()

	// Test updating the event
	formValues := map[string][]string{
		"title":       {"Updated Event Title"},
		"description": {"Updated Event Description"},
		"start_time":  {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
		"duration":    {"3"},
	}

	resp := testContext.UpdateEvent(testingContext, event.ID, formValues)
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		body := ReadResponseBody(testingContext, resp)
		testingContext.Errorf("Expected status 303 or 200, got %d: %s", resp.StatusCode, body)
	}

	// Verify event was updated in database
	var updatedEvent models.Event
	testContext.DB.First(&updatedEvent, "id = ?", event.ID)
	if updatedEvent.Title != "Updated Event Title" {
		testingContext.Errorf("Expected title 'Updated Event Title', got '%s'", updatedEvent.Title)
	}
}

func TestDeleteEvent(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Create a test event
	event := testContext.CreateTestEvent()

	// Test deleting the event
	resp := testContext.DeleteEvent(testingContext, event.ID)
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := ReadResponseBody(testingContext, resp)
		testingContext.Errorf("Expected status 303, 204, or 200, got %d: %s", resp.StatusCode, body)
	}

	// Verify event was deleted from database
	var count int64
	testContext.DB.Model(&models.Event{}).Where("id = ?", event.ID).Count(&count)
	if count != 0 {
		testingContext.Errorf("Expected event to be deleted, but it still exists")
	}
}
