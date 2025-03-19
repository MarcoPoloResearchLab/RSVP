package tests

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
)

// to ensure it works properly regardless of UI concerns.
func TestCreateRSVPFunctionality(t *testing.T) {
	// Setup test context
	testContext := SetupTestContext(t)
	defer testContext.Cleanup()

	// Create a test event
	testEvent := testContext.CreateTestEvent()

	// Prepare form data for RSVP creation
	rsvpName := "Test RSVP Direct"
	formData := make(url.Values)
	formData.Set("name", rsvpName)

	// Use the helper function to create an RSVP with query param
	resp := testContext.CreateRSVP(t, testEvent.ID, formData, true)
	defer resp.Body.Close()

	// Verify successful response code
	if resp.StatusCode != 200 && resp.StatusCode != 302 {
		t.Fatalf("Expected status 200 or 302 but got %d", resp.StatusCode)
	}

	// Verify the RSVP was created by checking the database directly
	var rsvp models.RSVP
	result := testContext.DB.Where("name = ? AND event_id = ?", rsvpName, testEvent.ID).First(&rsvp)
	if result.Error != nil {
		t.Fatalf("Failed to find created RSVP: %v", result.Error)
	}

	if rsvp.Name != rsvpName {
		t.Errorf("RSVP name mismatch. Expected: %s, Got: %s", rsvpName, rsvp.Name)
	}
}

// has the proper action URL to match what the handler expects.
func TestRSVPFormCorrectness(t *testing.T) {
	// Setup test context with real templates instead of mocks
	uiTestContext := SetupUITestContext(t)
	defer uiTestContext.Cleanup()

	// Create a test event
	testEvent := uiTestContext.CreateTestEvent()

	// Step 1: Get the RSVPs page for this event
	url := fmt.Sprintf("%s%s?%s=%s", uiTestContext.RSVPServer.URL, config.WebRSVPs, config.EventIDParam, testEvent.ID)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to get RSVPs page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status OK but got %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Parse the HTML
	document, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("Failed to parse HTML: %v", err)
	}

	// Find the RSVP form
	formFound := false
	formActionOK := false

	document.Find("form").Each(func(i int, s *goquery.Selection) {
		// Check if this form contains a name input field (this would be the RSVP form)
		if s.Find("input[name='name']").Length() > 0 {
			formFound = true

			// Check the form's action URL
			action, exists := s.Attr("action")
			if !exists {
				t.Errorf("RSVP form has no action attribute")
				return
			}

			// Verify the form action is compatible with handler expectations
			// It should have a query parameter format or at least point to the rsvps endpoint
			if strings.Contains(action, fmt.Sprintf("/rsvps?%s=%s", config.EventIDParam, testEvent.ID)) ||
				strings.Contains(action, "/rsvps") {
				formActionOK = true
			} else {
				t.Errorf("Form action '%s' doesn't match expected format", action)
			}
		}
	})

	if !formFound {
		t.Errorf("RSVP form not found on the page")
	}

	if !formActionOK {
		t.Errorf("RSVP form action URL doesn't match handler expectations")
	}

	// Step 2: Now test the form submission
	formValues := map[string][]string{
		"name": {"Test RSVP via Form Test"},
	}

	// Use the helper function to create an RSVP
	resp1 := uiTestContext.CreateRSVP(t, testEvent.ID, formValues, true)
	defer resp1.Body.Close()

	// Verify the RSVP was created by checking the database directly
	var count int64
	uiTestContext.DB.Model(&models.RSVP{}).Where("name = ? AND event_id = ?", "Test RSVP via Form Test", testEvent.ID).Count(&count)

	if count == 0 {
		t.Errorf("RSVP was not created in the database after form submission")
	} else {
		t.Logf("RSVP creation successful")
	}
}
