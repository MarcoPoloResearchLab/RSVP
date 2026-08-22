// Package testsupport provides isolated RSVP integration-test fixtures.
package testsupport

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tyemirov/GAuss/pkg/session"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
	"github.com/tyemirov/RSVP/pkg/templates"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	// ApplicationBaseURL is the canonical public base URL used by handler tests.
	ApplicationBaseURL = "https://rsvp.example.test/"
	// OwnerUserID is the deterministic identifier for the primary fixture owner.
	OwnerUserID = "USR00001"
	// OtherUserID is the deterministic identifier for the secondary fixture owner.
	OtherUserID = "USR00002"
	// VenueID is the deterministic identifier for the default fixture venue.
	VenueID = "VEN00001"
	// EventID is the deterministic identifier for the default fixture event.
	EventID = "EVT00001"
	// RSVPID is the deterministic public code for the default fixture RSVP.
	RSVPID = "RSVP0001"
)

var (
	fixedStartTime = time.Date(2030, time.January, 2, 15, 0, 0, 0, time.UTC)
	templateLoad   sync.Once
)

// Fixture owns one isolated database and the application context that uses it.
type Fixture struct {
	T                  *testing.T
	Database           *gorm.DB
	ApplicationContext *config.ApplicationContext
}

// NewFixture creates an isolated SQLite database in the test temporary directory.
func NewFixture(testingContext *testing.T) *Fixture {
	testingContext.Helper()

	databasePath := filepath.Join(testingContext.TempDir(), "data", "rsvp-test.db")
	applicationLogger := log.New(io.Discard, "", 0)
	databaseConnection := services.InitDatabase(databasePath, applicationLogger)
	databaseConnection = databaseConnection.Session(&gorm.Session{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	sqlDatabase, databaseError := databaseConnection.DB()
	if databaseError != nil {
		testingContext.Fatalf("get test database handle: %v", databaseError)
	}
	testingContext.Cleanup(func() {
		if closeError := sqlDatabase.Close(); closeError != nil {
			testingContext.Errorf("close test database: %v", closeError)
		}
	})

	session.NewSession([]byte("0123456789abcdef0123456789abcdef"))

	return &Fixture{
		T:        testingContext,
		Database: databaseConnection,
		ApplicationContext: &config.ApplicationContext{
			Database:   databaseConnection,
			Logger:     applicationLogger,
			AppBaseURL: ApplicationBaseURL,
		},
	}
}

// FixedStartTime returns the fixed future timestamp used by event fixtures.
func FixedStartTime() time.Time {
	return fixedStartTime
}

// LoadTemplates loads the repository templates once for the current test process.
func LoadTemplates(testingContext *testing.T) {
	testingContext.Helper()
	templateLoad.Do(func() {
		_, currentFilePath, _, callerAvailable := runtime.Caller(0)
		if !callerAvailable {
			testingContext.Fatal("locate test-support source file")
		}
		repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFilePath), "..", ".."))
		templates.LoadAllPrecompiledTemplates(filepath.Join(repositoryRoot, config.TemplatesDir))
	})
}

// CreateUser inserts a deterministic user fixture.
func (fixture *Fixture) CreateUser(identifier string) models.User {
	fixture.T.Helper()
	userRecord := models.User{
		BaseModel: models.BaseModel{ID: identifier},
		Email:     strings.ToLower(identifier) + "@example.test",
		Name:      "User " + identifier,
	}
	if createError := userRecord.Create(fixture.Database); createError != nil {
		fixture.T.Fatalf("create user %s: %v", identifier, createError)
	}
	return userRecord
}

// CreateVenue inserts a deterministic venue fixture for an owner.
func (fixture *Fixture) CreateVenue(identifier string, ownerIdentifier string) models.Venue {
	fixture.T.Helper()
	venueRecord := models.Venue{
		BaseModel: models.BaseModel{ID: identifier},
		UserID:    ownerIdentifier,
		Name:      "Venue " + identifier,
		Address:   "100 Test Avenue",
		Capacity:  40,
	}
	if createError := venueRecord.Create(fixture.Database); createError != nil {
		fixture.T.Fatalf("create venue %s: %v", identifier, createError)
	}
	return venueRecord
}

// CreateEvent inserts a deterministic event fixture for an owner.
func (fixture *Fixture) CreateEvent(identifier string, ownerIdentifier string, venueIdentifier *string) models.Event {
	fixture.T.Helper()
	eventRecord := models.Event{
		BaseModel:   models.BaseModel{ID: identifier},
		Title:       "Event " + identifier,
		StartTime:   fixedStartTime,
		EndTime:     fixedStartTime.Add(2 * time.Hour),
		UserID:      ownerIdentifier,
		VenueID:     venueIdentifier,
		Description: "A deterministic event fixture.",
	}
	if createError := eventRecord.Create(fixture.Database); createError != nil {
		fixture.T.Fatalf("create event %s: %v", identifier, createError)
	}
	return eventRecord
}

// CreateRSVP inserts a deterministic RSVP fixture for an event.
func (fixture *Fixture) CreateRSVP(identifier string, eventIdentifier string) models.RSVP {
	fixture.T.Helper()
	rsvpRecord := models.RSVP{
		BaseModel: models.BaseModel{ID: identifier},
		Name:      "Invitee " + identifier,
		Response:  config.RSVPResponsePending,
		EventID:   eventIdentifier,
	}
	if createError := rsvpRecord.Create(fixture.Database); createError != nil {
		fixture.T.Fatalf("create RSVP %s: %v", identifier, createError)
	}
	return rsvpRecord
}

// Request creates an HTTP request with optional form data and an authenticated user.
func Request(
	testingContext *testing.T,
	method string,
	target string,
	formValues url.Values,
	currentUser *models.User,
) *http.Request {
	testingContext.Helper()

	var request *http.Request
	if formValues == nil {
		request = httptest.NewRequest(method, target, nil)
	} else {
		request = httptest.NewRequest(method, target, strings.NewReader(formValues.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if currentUser == nil {
		return request
	}

	requestContext := context.WithValue(request.Context(), middleware.ContextKeyUser, currentUser)
	return request.WithContext(requestContext)
}
