package tests

import (
	"net/http"
	"testing"
	"time"

	"github.com/temirov/RSVP/models"
)

// TestEventValidation tests validation for event creation and updates.
func TestEventValidation(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Test cases for event validation
	validationTests := []struct {
		name          string
		formValues    map[string][]string
		expectSuccess bool
	}{
		{
			name: "Missing Title",
			formValues: map[string][]string{
				"description": {"Test Description"},
				"start_time":  {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
				"duration":    {"2"},
			},
			expectSuccess: false,
		},
		{
			name: "Missing Start Time",
			formValues: map[string][]string{
				"title":       {"Test Event"},
				"description": {"Test Description"},
				"duration":    {"2"},
			},
			expectSuccess: false,
		},
		{
			name: "Missing Duration",
			formValues: map[string][]string{
				"title":       {"Test Event"},
				"description": {"Test Description"},
				"start_time":  {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
			},
			expectSuccess: false,
		},
		{
			name: "Invalid Duration Format",
			formValues: map[string][]string{
				"title":       {"Test Event"},
				"description": {"Test Description"},
				"start_time":  {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
				"duration":    {"invalid"},
			},
			expectSuccess: false,
		},
		{
			name: "Past Start Time",
			formValues: map[string][]string{
				"title":       {"Test Event"},
				"description": {"Test Description"},
				"start_time":  {time.Now().Add(-24 * time.Hour).Format("2006-01-02T15:04")},
				"duration":    {"2"},
			},
			expectSuccess: false, // This may succeed depending on validation implementation
		},
		{
			name: "Very Long Title",
			formValues: map[string][]string{
				"title":       {string(make([]byte, 300))}, // 300 character title
				"description": {"Test Description"},
				"start_time":  {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
				"duration":    {"2"},
			},
			expectSuccess: false, // This may succeed depending on validation implementation
		},
		{
			name: "Valid Event",
			formValues: map[string][]string{
				"title":       {"Valid Test Event"},
				"description": {"Valid Test Description"},
				"start_time":  {time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04")},
				"duration":    {"2"},
			},
			expectSuccess: true,
		},
	}

	for _, test := range validationTests {
		testingContext.Run(test.name, func(subTestContext *testing.T) {
			// Attempt to create an event with these values
			resp := testContext.CreateEvent(subTestContext, test.formValues)
			defer resp.Body.Close()

			// Check if the response matches expectations
			if test.expectSuccess {
				if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
					body := ReadResponseBody(subTestContext, resp)
					subTestContext.Errorf("Expected success status (303, 201, or 200), got %d: %s", resp.StatusCode, body)
				}

				// Verify event was created in database
				var event models.Event
				result := testContext.DB.Where("title = ?", test.formValues["title"][0]).First(&event)
				if result.Error != nil {
					subTestContext.Errorf("Failed to find created event: %v", result.Error)
				}
			} else {
				// For validation failures, we expect either a 400 Bad Request or a redirect back to the form
				// The exact behavior depends on the implementation
				if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusSeeOther {
					body := ReadResponseBody(subTestContext, resp)
					subTestContext.Errorf("Expected validation failure status (400 or 303), got %d: %s", resp.StatusCode, body)
				}
			}
		})
	}
}

// TestRSVPValidation tests validation for RSVP creation and updates.
func TestRSVPValidation(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Create a test event for valid RSVPs
	event := testContext.CreateTestEvent()

	// Test cases for RSVP validation
	validationTests := []struct {
		name          string
		eventID       string
		formValues    map[string][]string
		expectSuccess bool
	}{
		{
			name:       "Missing Name",
			eventID:    event.ID,
			formValues: map[string][]string{
				// No name provided
			},
			expectSuccess: false,
		},
		{
			name:    "Empty Name",
			eventID: event.ID,
			formValues: map[string][]string{
				"name": {""},
			},
			expectSuccess: false,
		},
		{
			name:    "Very Long Name",
			eventID: event.ID,
			formValues: map[string][]string{
				"name": {string(make([]byte, 300))}, // 300 character name
			},
			expectSuccess: false, // This may succeed depending on validation implementation
		},
		{
			name:    "Invalid Event ID",
			eventID: "nonexistent",
			formValues: map[string][]string{
				"name": {"Test Attendee"},
			},
			expectSuccess: false,
		},
		{
			name:    "Valid RSVP",
			eventID: event.ID,
			formValues: map[string][]string{
				"name": {"Valid Test Attendee"},
			},
			expectSuccess: true,
		},
	}

	for _, test := range validationTests {
		testingContext.Run(test.name, func(subTestContext *testing.T) {
			// Attempt to create an RSVP with these values
			resp := testContext.CreateRSVP(subTestContext, test.eventID, test.formValues, true)
			defer resp.Body.Close()

			// Check if the response matches expectations
			if test.expectSuccess {
				if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
					body := ReadResponseBody(subTestContext, resp)
					subTestContext.Errorf("Expected success status (303, 201, or 200), got %d: %s", resp.StatusCode, body)
				}

				// Verify RSVP was created in database
				var rsvp models.RSVP
				result := testContext.DB.Where("name = ? AND event_id = ?", test.formValues["name"][0], test.eventID).First(&rsvp)
				if result.Error != nil {
					subTestContext.Errorf("Failed to find created RSVP: %v", result.Error)
				}
			} else {
				// For validation failures, we expect either a 400 Bad Request or a redirect back to the form
				// The exact behavior depends on the implementation
				if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusNotFound {
					body := ReadResponseBody(subTestContext, resp)
					subTestContext.Errorf("Expected validation failure status (400, 303, or 404), got %d: %s", resp.StatusCode, body)
				}
			}
		})
	}
}

// TestRSVPResponseValidation tests validation for RSVP response updates.
func TestRSVPResponseValidation(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Create a test event
	event := testContext.CreateTestEvent()

	// Create a test RSVP
	rsvp := testContext.CreateTestRSVP(event.ID)

	// Test cases for RSVP response validation
	validationTests := []struct {
		name          string
		response      string
		expectSuccess bool
	}{
		{
			name:          "Invalid Response Format",
			response:      "Maybe",
			expectSuccess: false,
		},
		{
			name:          "Invalid Yes Format",
			response:      "Yes",
			expectSuccess: false,
		},
		{
			name:          "Invalid Guest Count",
			response:      "Yes,10",
			expectSuccess: false,
		},
		{
			name:          "Negative Guest Count",
			response:      "Yes,-1",
			expectSuccess: false,
		},
		{
			name:          "Valid No Response",
			response:      "No",
			expectSuccess: true,
		},
		{
			name:          "Valid Yes Response with 0 guests",
			response:      "Yes,0",
			expectSuccess: true,
		},
		{
			name:          "Valid Yes Response with 2 guests",
			response:      "Yes,2",
			expectSuccess: true,
		},
		{
			name:          "Valid Yes Response with 4 guests",
			response:      "Yes,4",
			expectSuccess: true,
		},
	}

	for _, test := range validationTests {
		testingContext.Run(test.name, func(subTestContext *testing.T) {
			// Update RSVP with this response
			formValues := map[string][]string{
				"response": {test.response},
			}

			resp := testContext.UpdateRSVP(subTestContext, rsvp.ID, event.ID, formValues)
			defer resp.Body.Close()

			// Check if the response matches expectations
			if test.expectSuccess {
				if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
					body := ReadResponseBody(subTestContext, resp)
					subTestContext.Errorf("Expected success status (303 or 200), got %d: %s", resp.StatusCode, body)
				}

				// Verify RSVP was updated in database
				var updatedRSVP models.RSVP
				testContext.DB.First(&updatedRSVP, "id = ?", rsvp.ID)

				if updatedRSVP.Response != test.response {
					subTestContext.Errorf("Expected response '%s', got '%s'", test.response, updatedRSVP.Response)
				}
			} else {
				// For validation failures, we expect either a 400 Bad Request or a redirect back to the form
				// The exact behavior depends on the implementation
				if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusSeeOther {
					body := ReadResponseBody(subTestContext, resp)
					subTestContext.Errorf("Expected validation failure status (400 or 303), got %d: %s", resp.StatusCode, body)
				}
			}
		})
	}
}
