package tests

import (
	"net/http"
	"testing"

	"github.com/temirov/RSVP/models"
)

func TestListRSVPs(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()

	// Create some test RSVPs
	testContext.CreateTestRSVP(event.ID)
	testContext.CreateTestRSVP(event.ID)

	// Test listing RSVPs
	resp := testContext.GetRSVP(testingContext, "", event.ID)
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		testingContext.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify RSVPs in database
	var count int64
	testContext.DB.Model(&models.RSVP{}).Where("event_id = ?", event.ID).Count(&count)
	if count != 2 {
		testingContext.Errorf("Expected 2 RSVPs, got %d", count)
	}
}

func TestCreateRSVP(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()

	// Test cases for different ways to provide event_id
	testCases := []struct {
		name          string
		useQueryParam bool
		expectError   bool
	}{
		{
			name:          "Query Parameter",
			useQueryParam: true,
			expectError:   false,
		},
		{
			name:          "Form Value",
			useQueryParam: false,
			expectError:   false,
		},
	}

	for _, testCase := range testCases {
		testingContext.Run(testCase.name, func(subTestContext *testing.T) {
			// Test creating an RSVP
			formValues := map[string][]string{
				"name": {"New Test RSVP"},
			}

			resp := testContext.CreateRSVP(subTestContext, event.ID, formValues, testCase.useQueryParam)
			defer resp.Body.Close()

			// Verify response
			if testCase.expectError {
				if resp.StatusCode != http.StatusBadRequest {
					subTestContext.Errorf("Expected status 400, got %d", resp.StatusCode)
				}
			} else {
				if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
					body := ReadResponseBody(subTestContext, resp)
					subTestContext.Errorf("Expected status 303, 201, or 200, got %d: %s", resp.StatusCode, body)
				}

				// Verify RSVP was created in database
				var rsvp models.RSVP
				result := testContext.DB.Where("name = ? AND event_id = ?", "New Test RSVP", event.ID).First(&rsvp)
				if result.Error != nil {
					subTestContext.Errorf("Failed to find created RSVP: %v", result.Error)
				}
				if rsvp.Name != "New Test RSVP" {
					subTestContext.Errorf("Expected name 'New Test RSVP', got '%s'", rsvp.Name)
				}
			}
		})
	}
}

func TestShowRSVP(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()

	// Create a test RSVP
	rsvp := testContext.CreateTestRSVP(event.ID)

	// Test showing the RSVP
	resp := testContext.GetRSVP(testingContext, rsvp.ID, "")
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		testingContext.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestUpdateRSVP(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()

	// Create a test RSVP
	rsvp := testContext.CreateTestRSVP(event.ID)

	// Test updating the RSVP
	formValues := map[string][]string{
		"name":     {"Updated RSVP Name"},
		"response": {"Yes,2"},
	}

	resp := testContext.UpdateRSVP(testingContext, rsvp.ID, event.ID, formValues)
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		body := ReadResponseBody(testingContext, resp)
		testingContext.Errorf("Expected status 303 or 200, got %d: %s", resp.StatusCode, body)
	}

	// Verify RSVP was updated in database
	var updatedRSVP models.RSVP
	testContext.DB.First(&updatedRSVP, "id = ?", rsvp.ID)
	if updatedRSVP.Name != "Updated RSVP Name" {
		testingContext.Errorf("Expected name 'Updated RSVP Name', got '%s'", updatedRSVP.Name)
	}
	if updatedRSVP.Response != "Yes,2" {
		testingContext.Errorf("Expected response 'Yes,2', got '%s'", updatedRSVP.Response)
	}
}

func TestDeleteRSVP(testingContext *testing.T) {
	// Setup test context
	testContext := SetupTestContext(testingContext)
	defer testContext.Cleanup()
	
	// Create a test event
	event := testContext.CreateTestEvent()

	// Create test RSVPs
	rsvp1 := testContext.CreateTestRSVP(event.ID)
	rsvp2 := testContext.CreateTestRSVP(event.ID)

	// Test deleting with DELETE method
	resp1 := testContext.DeleteRSVP(testingContext, rsvp1.ID, event.ID)
	defer resp1.Body.Close()

	// Verify response
	if resp1.StatusCode != http.StatusSeeOther && resp1.StatusCode != http.StatusNoContent && resp1.StatusCode != http.StatusOK {
		body := ReadResponseBody(testingContext, resp1)
		testingContext.Errorf("Expected status 303, 204, or 200, got %d: %s", resp1.StatusCode, body)
	}

	// Test deleting with form method override
	resp2 := testContext.DeleteRSVPWithForm(testingContext, rsvp2.ID, event.ID)
	defer resp2.Body.Close()

	// Verify response
	if resp2.StatusCode != http.StatusSeeOther && resp2.StatusCode != http.StatusNoContent && resp2.StatusCode != http.StatusOK {
		body := ReadResponseBody(testingContext, resp2)
		testingContext.Errorf("Expected status 303, 204, or 200, got %d: %s", resp2.StatusCode, body)
	}

	// Verify RSVPs were deleted from database
	var count int64
	testContext.DB.Model(&models.RSVP{}).Where("id IN ?", []string{rsvp1.ID, rsvp2.ID}).Count(&count)
	if count != 0 {
		testingContext.Errorf("Expected RSVPs to be deleted, but %d still exist", count)
	}
}
