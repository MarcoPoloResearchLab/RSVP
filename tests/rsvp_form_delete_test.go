package tests

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// Unlike the method in setup.go, this explicitly tests the form input pattern.
func (testContext *TestContext) DeleteRSVPWithFormData(testingContext *testing.T, rsvpID string, eventID string) *http.Response {
	// Create form values with the method override parameter
	formValues := url.Values{}
	formValues.Set(config.MethodOverrideParam, "DELETE")

	// Important: Include rsvp_id and event_id as form data
	// This matches how the HTML form is structured
	formValues.Set(config.RSVPIDParam, rsvpID)
	if eventID != "" {
		formValues.Set(config.EventIDParam, eventID)
	}

	// Create the form data reader
	formDataReader := strings.NewReader(formValues.Encode())

	// Create a new request with the form data
	req, createError := http.NewRequest(http.MethodPost, testContext.RSVPServer.URL+config.WebRSVPs, formDataReader)
	if createError != nil {
		testingContext.Fatalf("Failed to create form POST request: %v", createError)
	}

	// Set the content type for form data
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	resp, requestError := http.DefaultClient.Do(req)
	if requestError != nil {
		testingContext.Fatalf("Failed to make form POST request: %v", requestError)
	}

	return resp
}

// This test would catch bugs where the RSVP wasn't being deleted through the UI form.
func TestFormBasedRSVPDeletion(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Create a test event and RSVP
	event := testContext.CreateTestEvent()
	rsvp := testContext.CreateTestRSVP(event.ID)

	// Verify the RSVP exists before attempting deletion
	var countBefore int64
	testContext.DB.Model(&models.RSVP{}).Where("id = ?", rsvp.ID).Count(&countBefore)
	if countBefore != 1 {
		testingContext.Fatalf("Expected RSVP to exist before deletion, but count was %d", countBefore)
	}

	// Test deleting the RSVP using the form submission pattern
	resp := testContext.DeleteRSVPWithFormData(testingContext, rsvp.ID, event.ID)
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := ReadResponseBody(testingContext, resp)
		testingContext.Fatalf("Expected redirect status (303, 204, or 200), got %d: %s", resp.StatusCode, body)
	}

	// Verify the RSVP was actually deleted from the database
	var countAfter int64
	testContext.DB.Model(&models.RSVP{}).Where("id = ?", rsvp.ID).Count(&countAfter)
	if countAfter != 0 {
		testingContext.Errorf("Expected RSVP to be deleted, but count after deletion was %d", countAfter)
	}
}

// This verifies the system can handle RSVP deletion with only the RSVP ID.
func TestRSVPDeletionWithoutEventID(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()

	// Create a test event and RSVP
	event := testContext.CreateTestEvent()
	rsvp := testContext.CreateTestRSVP(event.ID)

	// Test deleting the RSVP using the form submission pattern without event_id
	resp := testContext.DeleteRSVPWithFormData(testingContext, rsvp.ID, "")
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := ReadResponseBody(testingContext, resp)
		testingContext.Fatalf("Expected redirect status (303, 204, or 200), got %d: %s", resp.StatusCode, body)
	}

	// Verify the RSVP was actually deleted from the database
	var countAfter int64
	testContext.DB.Model(&models.RSVP{}).Where("id = ?", rsvp.ID).Count(&countAfter)
	if countAfter != 0 {
		testingContext.Errorf("Expected RSVP to be deleted, but count after deletion was %d", countAfter)
	}
}
