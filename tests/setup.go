package tests

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

// TestContext holds all the components needed for testing.
type TestContext struct {
	DB          *gorm.DB
	AppContext  *config.ApplicationContext
	EventServer *httptest.Server
	RSVPServer  *httptest.Server
	TestUser    *models.User
}

// SetupTestContext creates a test context with a unique test database and test servers.
func SetupTestContext(testingContext *testing.T) *TestContext {
	// Initialize session store with a test secret
	session.NewSession([]byte("test-secret-key-for-integration-tests"))

	// Generate unique database name for this test with a proper path
	base36ID, err := models.GenerateBase36ID(8)
	if err != nil {
		return nil
	}
	dbBaseName := fmt.Sprintf("test_%s.db", base36ID)
	// Store the database path in a temporary directory
	tempDir := os.TempDir()
	databasePath := fmt.Sprintf("%s/%s", tempDir, dbBaseName)

	// Create logger for tests
	testLogger := log.New(os.Stdout, "TEST: ", log.LstdFlags)

	// Initialize database with the unique name and path
	databaseConnection := services.InitDatabase(databasePath, testLogger)

	// Create a template set with mock templates for testing
	templates := template.New("test")

	// Define the templates with the names expected by the handlers
	template.Must(templates.New(config.TemplateEvents).Parse(`Mock events template`))
	template.Must(templates.New(config.TemplateRSVPs).Parse(`Mock rsvps template`))
	template.Must(templates.New(config.TemplateResponse).Parse(`Mock response template`))
	template.Must(templates.New(config.TemplateThankYou).Parse(`Mock thank you template`))

	appContext := &config.ApplicationContext{
		Database:  databaseConnection,
		Templates: templates,
		Logger:    log.New(os.Stdout, "TEST: ", log.LstdFlags),
	}

	// Create HTTP router with integration routes (no auth middleware)
	httpRouter := http.NewServeMux()

	// Register routes without authentication middleware
	routes := New(appContext)
	routes.RegisterRoutes(httpRouter)

	// Create test servers with the integration router
	eventServer := httptest.NewServer(httpRouter)
	rsvpServer := httptest.NewServer(httpRouter)

	// Create test user from the shared test constants
	testUser := &models.User{
		Email:   DefaultTestUser.Email,
		Name:    DefaultTestUser.Name,
		Picture: DefaultTestUser.Picture,
	}
	databaseConnection.Create(testUser)

	return &TestContext{
		DB:          databaseConnection,
		AppContext:  appContext,
		EventServer: eventServer,
		RSVPServer:  rsvpServer,
		TestUser:    testUser,
	}
}

// CleanupTestContext closes test servers and cleans up resources.
func (testContext *TestContext) Cleanup() {
	testContext.EventServer.Close()
	testContext.RSVPServer.Close()

	// Get the database file path directly from the SQLITE connection
	var databaseFilePath string
	databaseInfoRow := testContext.DB.Raw("PRAGMA database_list").Row()
	if databaseInfoRow != nil {
		var sequenceNumber int
		var databaseName string
		if scanError := databaseInfoRow.Scan(&sequenceNumber, &databaseName, &databaseFilePath); scanError != nil {
			// Log the error but continue with cleanup
			testContext.AppContext.Logger.Printf("Error getting database path: %v", scanError)
		}
	}

	// Close the database connection first
	sqlDatabase, databaseConnectionError := testContext.DB.DB()
	if databaseConnectionError != nil {
		testContext.AppContext.Logger.Printf("Error getting SQL DB: %v", databaseConnectionError)
	} else {
		if databaseCloseError := sqlDatabase.Close(); databaseCloseError != nil {
			testContext.AppContext.Logger.Printf("Error closing database: %v", databaseCloseError)
		}
	}

	// Find all test database files in the current directory pattern
	testDbFiles, _ := os.ReadDir(".")
	for _, file := range testDbFiles {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "test_") && strings.HasSuffix(file.Name(), ".db") {
			// Found a test database file, attempt to remove it
			if fileRemovalError := os.Remove(file.Name()); fileRemovalError != nil {
				testContext.AppContext.Logger.Printf("Error removing database file %s: %v", file.Name(), fileRemovalError)
			} else {
				testContext.AppContext.Logger.Printf("Successfully removed temporary database: %s", file.Name())
			}
		}
	}

	// Also remove the database file if we have a path from PRAGMA
	if databaseFilePath != "" {
		if fileRemovalError := os.Remove(databaseFilePath); fileRemovalError != nil {
			testContext.AppContext.Logger.Printf("Error removing database file %s: %v", databaseFilePath, fileRemovalError)
		} else {
			testContext.AppContext.Logger.Printf("Successfully removed temporary database: %s", databaseFilePath)
		}
	}

	// Finally check for any test database files in the tests directory
	testsDbFiles, _ := os.ReadDir("tests")
	for _, file := range testsDbFiles {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "test_") && strings.HasSuffix(file.Name(), ".db") {
			filePath := fmt.Sprintf("tests/%s", file.Name())
			if fileRemovalError := os.Remove(filePath); fileRemovalError != nil {
				testContext.AppContext.Logger.Printf("Error removing database file %s: %v", filePath, fileRemovalError)
			} else {
				testContext.AppContext.Logger.Printf("Successfully removed temporary database: %s", filePath)
			}
		}
	}
}

// CreateTestEvent creates a test event in the database.
func (testContext *TestContext) CreateTestEvent() *models.Event {
	event := &models.Event{
		Title:       "Test Event",
		Description: "Test Description",
		StartTime:   time.Now().Add(24 * time.Hour),
		EndTime:     time.Now().Add(26 * time.Hour),
		UserID:      testContext.TestUser.ID,
	}
	testContext.DB.Create(event)
	return event
}

// CreateTestRSVP creates a test RSVP in the database.
func (testContext *TestContext) CreateTestRSVP(eventID string) *models.RSVP {
	rsvp := &models.RSVP{
		Name:    "Test Attendee",
		EventID: eventID,
	}
	testContext.DB.Create(rsvp)
	return rsvp
}

// Helper functions for making HTTP requests.
func (testContext *TestContext) GetEvent(testingContext *testing.T, eventID string) *http.Response {
	url := testContext.EventServer.URL + config.WebEvents
	if eventID != "" {
		url += "?" + config.EventIDParam + "=" + eventID
	}
	resp, requestError := http.Get(url)
	if requestError != nil {
		testingContext.Fatalf("Failed to make GET request: %v", requestError)
	}
	return resp
}

func (testContext *TestContext) CreateEvent(testingContext *testing.T, formValues map[string][]string) *http.Response {
	resp, requestError := http.PostForm(testContext.EventServer.URL+config.WebEvents, formValues)
	if requestError != nil {
		testingContext.Fatalf("Failed to make POST request: %v", requestError)
	}
	return resp
}

func (testContext *TestContext) UpdateEvent(testingContext *testing.T, eventID string, formValues map[string][]string) *http.Response {
	url := testContext.EventServer.URL + config.WebEvents + "?" + config.EventIDParam + "=" + eventID
	resp, requestError := http.PostForm(url, formValues)
	if requestError != nil {
		testingContext.Fatalf("Failed to make POST request: %v", requestError)
	}
	return resp
}

func (testContext *TestContext) DeleteEvent(testingContext *testing.T, eventID string) *http.Response {
	req, createError := http.NewRequest(http.MethodDelete, testContext.EventServer.URL+config.WebEvents+"?"+config.EventIDParam+"="+eventID, nil)
	if createError != nil {
		testingContext.Fatalf("Failed to create DELETE request: %v", createError)
	}
	resp, requestError := http.DefaultClient.Do(req)
	if requestError != nil {
		testingContext.Fatalf("Failed to make DELETE request: %v", requestError)
	}
	return resp
}

// Similar helper functions for RSVP operations.
func (testContext *TestContext) GetRSVP(testingContext *testing.T, rsvpID string, eventID string) *http.Response {
	url := testContext.RSVPServer.URL + config.WebRSVPs
	params := []string{}
	if rsvpID != "" {
		params = append(params, config.RSVPIDParam+"="+rsvpID)
	}
	if eventID != "" {
		params = append(params, config.EventIDParam+"="+eventID)
	}
	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}

	// Create a request
	req, createError := http.NewRequest(http.MethodGet, url, nil)
	if createError != nil {
		testingContext.Fatalf("Failed to create GET request: %v", createError)
	}

	// Send the request
	resp, requestError := http.DefaultClient.Do(req)
	if requestError != nil {
		testingContext.Fatalf("Failed to make GET request: %v", requestError)
	}

	return resp
}

func (testContext *TestContext) CreateRSVP(testingContext *testing.T, eventID string, formValues map[string][]string, useQueryParam bool) *http.Response {
	url := testContext.RSVPServer.URL + config.WebRSVPs
	if useQueryParam && eventID != "" {
		url += "?" + config.EventIDParam + "=" + eventID
	} else if !useQueryParam && eventID != "" {
		formValues[config.EventIDParam] = []string{eventID}
	}
	resp, requestError := http.PostForm(url, formValues)
	if requestError != nil {
		testingContext.Fatalf("Failed to make POST request: %v", requestError)
	}
	return resp
}

func (testContext *TestContext) UpdateRSVP(testingContext *testing.T, rsvpID string, eventID string, formValues map[string][]string) *http.Response {
	url := testContext.RSVPServer.URL + config.WebRSVPs + "?" + config.RSVPIDParam + "=" + rsvpID
	if eventID != "" {
		url += "&" + config.EventIDParam + "=" + eventID
	}
	resp, requestError := http.PostForm(url, formValues)
	if requestError != nil {
		testingContext.Fatalf("Failed to make POST request: %v", requestError)
	}
	return resp
}

func (testContext *TestContext) DeleteRSVP(testingContext *testing.T, rsvpID string, eventID string) *http.Response {
	url := testContext.RSVPServer.URL + config.WebRSVPs + "?" + config.RSVPIDParam + "=" + rsvpID
	if eventID != "" {
		url += "&" + config.EventIDParam + "=" + eventID
	}
	req, createError := http.NewRequest(http.MethodDelete, url, nil)
	if createError != nil {
		testingContext.Fatalf("Failed to create DELETE request: %v", createError)
	}
	resp, requestError := http.DefaultClient.Do(req)
	if requestError != nil {
		testingContext.Fatalf("Failed to make DELETE request: %v", requestError)
	}
	return resp
}

// DeleteRSVPWithForm simulates a form submission with _method=DELETE.
func (testContext *TestContext) DeleteRSVPWithForm(testingContext *testing.T, rsvpID string, eventID string) *http.Response {
	url := testContext.RSVPServer.URL + config.WebRSVPs + "?" + config.RSVPIDParam + "=" + rsvpID
	if eventID != "" {
		url += "&" + config.EventIDParam + "=" + eventID
	}
	formValues := make(map[string][]string)
	formValues[config.MethodOverrideParam] = []string{"DELETE"}
	resp, requestError := http.PostForm(url, formValues)
	if requestError != nil {
		testingContext.Fatalf("Failed to make POST request with _method=DELETE: %v", requestError)
	}
	return resp
}

// Helper function to read response body.
func ReadResponseBody(testingContext *testing.T, resp *http.Response) string {
	body, readError := io.ReadAll(resp.Body)
	if readError != nil {
		testingContext.Fatalf("Failed to read response body: %v", readError)
	}
	defer resp.Body.Close()
	return string(body)
}
