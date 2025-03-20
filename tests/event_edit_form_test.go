package tests

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/temirov/RSVP/pkg/config"
)

// 2. The edit form contains the event information and allows editing.
func TestEventEditFormDisplay(testingInstance *testing.T) {
	// This test verifies that when clicking on an event in the table,
	// the edit form is displayed with the correct event information

	// Setup test context with real templates
	uiTestContext := SetupUITestContext(testingInstance)
	defer uiTestContext.Cleanup()

	// Create a test event
	testEvent := uiTestContext.CreateTestEvent()
	testingInstance.Logf("Created test event with ID: %s", testEvent.ID)

	// Step 1: Verify the event appears in the events table
	eventsListURL := uiTestContext.EventServer.URL + config.WebEvents
	listResponse, listError := http.Get(eventsListURL)
	if listError != nil {
		testingInstance.Fatalf("Failed to get events list page: %v", listError)
	}
	defer listResponse.Body.Close()

	// Read and parse the events list page
	listContent, listReadError := io.ReadAll(listResponse.Body)
	if listReadError != nil {
		testingInstance.Fatalf("Failed to read events list content: %v", listReadError)
	}

	listDocument, listParseError := goquery.NewDocumentFromReader(strings.NewReader(string(listContent)))
	if listParseError != nil {
		testingInstance.Fatalf("Failed to parse events list HTML: %v", listParseError)
	}

	// Verify event table exists
	eventTable := listDocument.Find("table")
	if eventTable.Length() == 0 {
		testingInstance.Fatalf("Events table not found on list page")
	}

	// Verify our test event is in the table
	eventRow := listDocument.Find(fmt.Sprintf(".event-row[data-event-id='%s']", testEvent.ID))
	if eventRow.Length() == 0 {
		testingInstance.Fatalf("Test event row not found in table")
	}

	// Step 2: When the event row is clicked, the edit form should be displayed
	// Clicking is simulated by making a request to /events/?id={id}
	// (This is what the JavaScript click handler in templates/event/event_index.html does)
	editURL := fmt.Sprintf("%s%s/?%s=%s", uiTestContext.EventServer.URL, config.WebEvents, config.EventIDParam, testEvent.ID)
	editResponse, editError := http.Get(editURL)
	if editError != nil {
		testingInstance.Fatalf("Failed to request edit page: %v", editError)
	}
	defer editResponse.Body.Close()

	// Read and parse the edit page
	editContent, editReadError := io.ReadAll(editResponse.Body)
	if editReadError != nil {
		testingInstance.Fatalf("Failed to read edit page content: %v", editReadError)
	}

	editDocument, editParseError := goquery.NewDocumentFromReader(strings.NewReader(string(editContent)))
	if editParseError != nil {
		testingInstance.Fatalf("Failed to parse edit page HTML: %v", editParseError)
	}

	// Debug: Output the entire HTML content
	testingInstance.Logf("HTML Content: %s", string(editContent))

	// Verify we're getting the edit event form
	editHeader := editDocument.Find("h4:contains('Edit Event')")
	if editHeader.Length() == 0 {
		testingInstance.Fatalf("Edit Event form not found on the page")
	}

	// Verify the edit form contains the event data
	titleInput := editDocument.Find("input[name='title']")
	if titleInput.Length() == 0 {
		testingInstance.Errorf("Event title input not found")
	} else {
		titleValue, exists := titleInput.Attr("value")
		if !exists || titleValue != testEvent.Title {
			testingInstance.Errorf("Event title value doesn't match expected. Got: %s, Expected: %s", titleValue, testEvent.Title)
		}
	}

	// Check for the description textarea
	descriptionTextarea := editDocument.Find("textarea[name='description']")
	if descriptionTextarea.Length() == 0 {
		testingInstance.Errorf("Event description textarea not found")
	} else if !strings.Contains(descriptionTextarea.Text(), testEvent.Description) {
		testingInstance.Errorf("Event description doesn't match expected. Got: %s, Expected: %s", descriptionTextarea.Text(), testEvent.Description)
	}

	// Check for Update button
	updateButton := editDocument.Find("button[type='submit'].btn-primary")
	if updateButton.Length() == 0 {
		testingInstance.Errorf("Update button not found on the edit form")
	} else if !strings.Contains(updateButton.Text(), "Update Event") {
		testingInstance.Errorf("Update button text doesn't match expected. Got: %s, Expected: Update Event", updateButton.Text())
	}

	// Check for RSVPs link
	rsvpsLink := editDocument.Find("a[href*='rsvps']")
	if rsvpsLink.Length() == 0 {
		testingInstance.Errorf("RSVPs link not found on the page")
	} else if !strings.Contains(rsvpsLink.Text(), "RSVPs") {
		testingInstance.Errorf("RSVPs link text doesn't match expected. Got: %s, Expected to contain: RSVPs", rsvpsLink.Text())
	}
}
