package tests

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// for deleting an event using the form with _method=DELETE parameter.
func (testContext *TestContext) DeleteEventWithForm(testingContext *testing.T, eventID string) *http.Response {
	// Create form values with the method override parameter
	formValues := url.Values{}
	formValues.Set(config.MethodOverrideParam, "DELETE")
	formValues.Set(config.EventIDParam, eventID)

	// Create the form data reader
	formDataReader := strings.NewReader(formValues.Encode())

	// Create a new request with the form data
	// Notice we're using the trailing slash in the URL to avoid redirect issues
	req, createError := http.NewRequest(http.MethodPost, testContext.EventServer.URL+"/events/", formDataReader)
	if createError != nil {
		testingContext.Fatalf("Failed to create form POST request: %v", createError)
	}

	// Set the content type for form data
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Use a client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Send the request
	resp, requestError := client.Do(req)
	if requestError != nil {
		testingContext.Fatalf("Failed to make form POST request: %v", requestError)
	}

	return resp
}

// TestFormBasedEventDeletion tests the exact delete form submission pattern used in the UI.
func TestFormBasedEventDeletion(t *testing.T) {
	// Setup test context
	testContext := SetupTestContext(t)
	defer testContext.Cleanup()

	// Create a test event
	event := testContext.CreateTestEvent()

	// Create some RSVPs for this event to verify cascade deletion
	rsvp1 := testContext.CreateTestRSVP(event.ID)
	rsvp2 := testContext.CreateTestRSVP(event.ID)

	// Log test information
	t.Logf("INTEGRATION TEST: Form-based deletion of event ID: %s with RSVPs: %s, %s",
		event.ID, rsvp1.ID, rsvp2.ID)

	// Verify the event exists before attempting deletion
	var countBefore int64
	testContext.DB.Model(&models.Event{}).Where("id = ?", event.ID).Count(&countBefore)
	if countBefore != 1 {
		t.Fatalf("Event count before deletion expected 1, got %d", countBefore)
	}

	// Verify RSVPs exist before attempting deletion
	var rsvpCountBefore int64
	testContext.DB.Model(&models.RSVP{}).Where("event_id = ?", event.ID).Count(&rsvpCountBefore)
	if rsvpCountBefore != 2 {
		t.Fatalf("RSVP count before deletion expected 2, got %d", rsvpCountBefore)
	}

	// Test deleting the event using the form submission pattern
	resp := testContext.DeleteEventWithForm(t, event.ID)
	defer resp.Body.Close()

	// Read response body and log detailed information for debugging
	respBody, _ := io.ReadAll(resp.Body)
	t.Logf("Delete response status: %d", resp.StatusCode)
	t.Logf("Response headers: %v", resp.Header)
	t.Logf("Response body: %s", string(respBody))

	// Verify response status code - should be a redirect or success
	if resp.StatusCode != http.StatusSeeOther &&
		resp.StatusCode != http.StatusFound &&
		resp.StatusCode != http.StatusOK {
		t.Errorf("Expected redirect or success status, got %d", resp.StatusCode)
	}

	// Most importantly - verify the event was actually deleted from database
	var countAfter int64
	testContext.DB.Model(&models.Event{}).Where("id = ?", event.ID).Count(&countAfter)
	t.Logf("Event count after deletion attempt: %d", countAfter)

	if countAfter != 0 {
		t.Errorf("❌ TEST FAILED: Event was not deleted from the database!")
	} else {
		t.Logf("✅ TEST PASSED: Event was successfully deleted")
	}

	// Also verify that any associated RSVPs were deleted (cascade deletion)
	var rsvpCount int64
	testContext.DB.Model(&models.RSVP{}).Where("event_id = ?", event.ID).Count(&rsvpCount)
	if rsvpCount != 0 {
		t.Errorf("❌ TEST FAILED: RSVPs were not cascade deleted, found %d", rsvpCount)
	} else {
		t.Logf("✅ TEST PASSED: All associated RSVPs were cascade deleted")
	}
}

// TestDifferentFormSubmissionPatterns tests various ways the event deletion form could be submitted.
func TestDifferentFormSubmissionPatterns(t *testing.T) {
	testContext := SetupTestContext(t)
	defer testContext.Cleanup()

	scenarios := []struct {
		name        string
		urlPath     string
		formValues  url.Values
		expectError bool
	}{
		{
			name:    "With Trailing Slash and Form Data",
			urlPath: "/events/",
			formValues: url.Values{
				config.MethodOverrideParam: []string{"DELETE"},
				config.EventIDParam:        []string{""}, // Will be filled in dynamically
			},
			expectError: false, // This should succeed since we are using trailing slashes correctly
		},
		{
			name:    "Without Trailing Slash but with Form Data",
			urlPath: "/events",
			formValues: url.Values{
				config.MethodOverrideParam: []string{"DELETE"},
				config.EventIDParam:        []string{""}, // Will be filled in dynamically
			},
			expectError: true, // Without trailing slash, the POST gets redirected to a GET and form data is lost
		},
		{
			name:    "With Event ID in Query String",
			urlPath: "/events/?event_id=", // Will be completed dynamically
			formValues: url.Values{
				config.MethodOverrideParam: []string{"DELETE"},
			},
			expectError: true, // This should fail, event_id should be in form data
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Create a new event for each scenario
			event := testContext.CreateTestEvent()

			// Add the event ID to form values dynamically
			if _, exists := scenario.formValues[config.EventIDParam]; exists {
				scenario.formValues[config.EventIDParam] = []string{event.ID}
			}

			// Complete URL if it contains a placeholder
			urlPath := scenario.urlPath
			if strings.Contains(urlPath, "event_id=") {
				urlPath = urlPath + event.ID
			}

			// Create and send the request
			formDataReader := strings.NewReader(scenario.formValues.Encode())
			req, _ := http.NewRequest(http.MethodPost, testContext.EventServer.URL+urlPath, formDataReader)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			t.Logf("[%s] Response status: %d", scenario.name, resp.StatusCode)

			// Check database to see if deletion occurred
			var countAfter int64
			testContext.DB.Model(&models.Event{}).Where("id = ?", event.ID).Count(&countAfter)

			if scenario.expectError {
				if countAfter == 0 {
					t.Errorf("[%s] Event was deleted when it should not have been", scenario.name)
				}
			} else {
				if countAfter != 0 {
					t.Errorf("[%s] Event was not deleted when it should have been", scenario.name)
				}
			}
		})
	}
}
