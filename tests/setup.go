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
	"github.com/temirov/RSVP/pkg/utils"
	"gorm.io/gorm"
)

// TestContext holds all the components needed for testing
type TestContext struct {
	DB          *gorm.DB
	AppContext  *config.ApplicationContext
	EventServer *httptest.Server
	RSVPServer  *httptest.Server
	TestUser    *models.User
}

// SetupTestContext creates a test context with a unique test database and test servers
func SetupTestContext(testingContext *testing.T) *TestContext {
	// Initialize session store with a test secret
	session.NewSession([]byte("test-secret-key-for-integration-tests"))

	// Generate unique database name for this test
	databaseName := fmt.Sprintf("test_%s.db", utils.Base36Encode(8))

	// Create logger for tests
	testLogger := log.New(os.Stdout, "TEST: ", log.LstdFlags)

	// Initialize database with the unique name
	databaseConnection := services.InitDatabase(databaseName, testLogger)

	// Create a template set with mock templates for testing
	templates := template.New("test")

	// Define the templates with the names expected by the handlers
	template.Must(templates.New("responses.html").Parse(`Mock responses template`))
	template.Must(templates.New("event_index.html").Parse(`Mock event index template`))
	template.Must(templates.New("event_detail.html").Parse(`Mock event detail template`))
	template.Must(templates.New("generate.html").Parse(`Mock generate template`))
	template.Must(templates.New("thankyou.html").Parse(`Mock thank you template`))
	template.Must(templates.New("rsvp.html").Parse(`Mock RSVP template`))
	template.Must(templates.New("index.html").Parse(`Mock index template`))

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

// CleanupTestContext closes test servers and cleans up resources
func (testContext *TestContext) Cleanup() {
	testContext.EventServer.Close()
	testContext.RSVPServer.Close()
	
	// Get the database file path
	var sequenceNumber int
	var databaseName string
	var databaseFilePath string
	
	databaseInfoRow := testContext.DB.Raw("PRAGMA database_list").Row()
	if scanError := databaseInfoRow.Scan(&sequenceNumber, &databaseName, &databaseFilePath); scanError != nil {
		// Log the error but continue with cleanup
		testContext.AppContext.Logger.Printf("Error getting database path: %v", scanError)
	}
	
	// Close the database connection
	sqlDatabase, databaseConnectionError := testContext.DB.DB()
	if databaseConnectionError != nil {
		testContext.AppContext.Logger.Printf("Error getting SQL DB: %v", databaseConnectionError)
	} else {
		if databaseCloseError := sqlDatabase.Close(); databaseCloseError != nil {
			testContext.AppContext.Logger.Printf("Error closing database: %v", databaseCloseError)
		}
	}
	
	// Remove the database file if we have a path
	if databaseFilePath != "" {
		if fileRemovalError := os.Remove(databaseFilePath); fileRemovalError != nil {
			testContext.AppContext.Logger.Printf("Error removing database file %s: %v", databaseFilePath, fileRemovalError)
		} else {
			testContext.AppContext.Logger.Printf("Successfully removed temporary database: %s", databaseFilePath)
		}
	} else {
		testContext.AppContext.Logger.Printf("No database path found to clean up")
	}
}

// CreateTestEvent creates a test event in the database
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

// CreateTestRSVP creates a test RSVP in the database
func (testContext *TestContext) CreateTestRSVP(eventID string) *models.RSVP {
	rsvp := &models.RSVP{
		Name:    "Test Attendee",
		EventID: eventID,
	}
	testContext.DB.Create(rsvp)
	return rsvp
}

// Helper functions for making HTTP requests
func (testContext *TestContext) GetEvent(testingContext *testing.T, eventID string) *http.Response {
	url := testContext.EventServer.URL + config.WebEvents
	if eventID != "" {
		url += "?id=" + eventID
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
	url := testContext.EventServer.URL + config.WebEvents + "?id=" + eventID
	resp, requestError := http.PostForm(url, formValues)
	if requestError != nil {
		testingContext.Fatalf("Failed to make POST request: %v", requestError)
	}
	return resp
}

func (testContext *TestContext) DeleteEvent(testingContext *testing.T, eventID string) *http.Response {
	req, createError := http.NewRequest(http.MethodDelete, testContext.EventServer.URL+config.WebEvents+"?id="+eventID, nil)
	if createError != nil {
		testingContext.Fatalf("Failed to create DELETE request: %v", createError)
	}
	resp, requestError := http.DefaultClient.Do(req)
	if requestError != nil {
		testingContext.Fatalf("Failed to make DELETE request: %v", requestError)
	}
	return resp
}

// Similar helper functions for RSVP operations
func (testContext *TestContext) GetRSVP(testingContext *testing.T, rsvpID string, eventID string) *http.Response {
	url := testContext.RSVPServer.URL + config.WebRSVPs
	params := []string{}
	if rsvpID != "" {
		params = append(params, "id="+rsvpID)
	}
	if eventID != "" {
		params = append(params, "event_id="+eventID)
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
		url += "?event_id=" + eventID
	} else if !useQueryParam && eventID != "" {
		formValues["event_id"] = []string{eventID}
	}
	resp, requestError := http.PostForm(url, formValues)
	if requestError != nil {
		testingContext.Fatalf("Failed to make POST request: %v", requestError)
	}
	return resp
}

func (testContext *TestContext) UpdateRSVP(testingContext *testing.T, rsvpID string, eventID string, formValues map[string][]string) *http.Response {
	url := testContext.RSVPServer.URL + config.WebRSVPs + "?id=" + rsvpID
	if eventID != "" {
		url += "&event_id=" + eventID
	}
	resp, requestError := http.PostForm(url, formValues)
	if requestError != nil {
		testingContext.Fatalf("Failed to make POST request: %v", requestError)
	}
	return resp
}

func (testContext *TestContext) DeleteRSVP(testingContext *testing.T, rsvpID string, eventID string) *http.Response {
	url := testContext.RSVPServer.URL + config.WebRSVPs + "?id=" + rsvpID
	if eventID != "" {
		url += "&event_id=" + eventID
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

// DeleteRSVPWithForm simulates a form submission with _method=DELETE
func (testContext *TestContext) DeleteRSVPWithForm(testingContext *testing.T, rsvpID string, eventID string) *http.Response {
	url := testContext.RSVPServer.URL + config.WebRSVPs + "?id=" + rsvpID
	if eventID != "" {
		url += "&event_id=" + eventID
	}
	formValues := make(map[string][]string)
	formValues["_method"] = []string{"DELETE"}
	resp, requestError := http.PostForm(url, formValues)
	if requestError != nil {
		testingContext.Fatalf("Failed to make POST request with _method=DELETE: %v", requestError)
	}
	return resp
}

// Helper function to read response body
func ReadResponseBody(testingContext *testing.T, resp *http.Response) string {
	body, readError := io.ReadAll(resp.Body)
	if readError != nil {
		testingContext.Fatalf("Failed to read response body: %v", readError)
	}
	defer resp.Body.Close()
	return string(body)
}
