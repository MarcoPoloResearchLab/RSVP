package calendar_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers/calendar"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"gorm.io/gorm"
)

func TestCalendarVisibilityPersistsForOwner(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	calendarRecord := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001")

	responseRecorder := requestVisibility(testingContext, fixture, owner, http.MethodPatch, config.WebCalendars+calendarRecord.ID, `{"visible":false}`, "application/json")
	if responseRecorder.Code != http.StatusNoContent {
		testingContext.Fatalf("calendar visibility status = %d, want %d; body = %s", responseRecorder.Code, http.StatusNoContent, responseRecorder.Body.String())
	}
	if responseRecorder.Header().Get("Cache-Control") != "private, no-store" {
		testingContext.Fatalf("Cache-Control = %q, want %q", responseRecorder.Header().Get("Cache-Control"), "private, no-store")
	}
	var storedCalendar models.Calendar
	if findError := fixture.Database.First(&storedCalendar, "id = ?", calendarRecord.ID).Error; findError != nil {
		testingContext.Fatalf("reload calendar: %v", findError)
	}
	if storedCalendar.Visible {
		testingContext.Fatal("calendar remains visible after owner update")
	}
}

func TestCalendarVisibilityDoesNotExposeAnotherOwnerCalendar(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	otherOwner := fixture.CreateUser(testsupport.OtherUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	confirmTimezone(testingContext, fixture.Database, &otherOwner)
	calendarRecord := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001")

	responseRecorder := requestVisibility(testingContext, fixture, otherOwner, http.MethodPatch, config.WebCalendars+calendarRecord.ID, `{"visible":false}`, "application/json")
	if responseRecorder.Code != http.StatusNotFound {
		testingContext.Fatalf("other-owner status = %d, want %d; body = %s", responseRecorder.Code, http.StatusNotFound, responseRecorder.Body.String())
	}
	var storedCalendar models.Calendar
	if findError := fixture.Database.First(&storedCalendar, "id = ?", calendarRecord.ID).Error; findError != nil {
		testingContext.Fatalf("reload calendar: %v", findError)
	}
	if !storedCalendar.Visible {
		testingContext.Fatal("another owner changed calendar visibility")
	}
}

func TestCalendarVisibilityRejectsNoncanonicalRequests(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	calendarRecord := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001")
	testCases := []struct {
		name        string
		method      string
		target      string
		body        string
		contentType string
		wantStatus  int
		wantAllow   string
	}{
		{name: "method", method: http.MethodPut, target: config.WebCalendars + calendarRecord.ID, body: `{"visible":false}`, contentType: "application/json", wantStatus: http.StatusMethodNotAllowed, wantAllow: http.MethodPatch},
		{name: "path", method: http.MethodPatch, target: config.WebCalendars + calendarRecord.ID + "/extra", body: `{"visible":false}`, contentType: "application/json", wantStatus: http.StatusNotFound},
		{name: "media type", method: http.MethodPatch, target: config.WebCalendars + calendarRecord.ID, body: `{"visible":false}`, contentType: "text/plain", wantStatus: http.StatusBadRequest},
		{name: "missing field", method: http.MethodPatch, target: config.WebCalendars + calendarRecord.ID, body: `{}`, contentType: "application/json", wantStatus: http.StatusBadRequest},
		{name: "unknown field", method: http.MethodPatch, target: config.WebCalendars + calendarRecord.ID, body: `{"visible":false,"legacy":true}`, contentType: "application/json", wantStatus: http.StatusBadRequest},
		{name: "trailing value", method: http.MethodPatch, target: config.WebCalendars + calendarRecord.ID, body: `{"visible":false}{}`, contentType: "application/json", wantStatus: http.StatusBadRequest},
	}
	for _, testCase := range testCases {
		testingContext.Run(testCase.name, func(testingContext *testing.T) {
			responseRecorder := requestVisibility(testingContext, fixture, owner, testCase.method, testCase.target, testCase.body, testCase.contentType)
			if responseRecorder.Code != testCase.wantStatus {
				testingContext.Fatalf("status = %d, want %d; body = %s", responseRecorder.Code, testCase.wantStatus, responseRecorder.Body.String())
			}
			if responseRecorder.Header().Get("Allow") != testCase.wantAllow {
				testingContext.Fatalf("Allow = %q, want %q", responseRecorder.Header().Get("Allow"), testCase.wantAllow)
			}
			if testCase.wantStatus != http.StatusNotFound {
				var responseBody struct {
					Error struct {
						Code      string            `json:"code"`
						Details   map[string]string `json:"details"`
						RequestID string            `json:"request_id"`
					} `json:"error"`
				}
				if decodeError := json.Unmarshal(responseRecorder.Body.Bytes(), &responseBody); decodeError != nil {
					testingContext.Fatalf("decode typed error: %v", decodeError)
				}
				if responseBody.Error.Code == "" || responseBody.Error.Details == nil || responseBody.Error.RequestID == "" {
					testingContext.Fatalf("typed error is incomplete: %#v", responseBody.Error)
				}
			}
		})
	}
}

func requestVisibility(testingContext *testing.T, fixture *testsupport.Fixture, owner models.User, method string, target string, body string, contentType string) *httptest.ResponseRecorder {
	testingContext.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, &owner))
	responseRecorder := httptest.NewRecorder()
	calendar.VisibilityHandler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)
	return responseRecorder
}

func confirmTimezone(testingContext *testing.T, database *gorm.DB, owner *models.User) {
	testingContext.Helper()
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct fixture timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm fixture timezone: %v", confirmationError)
	}
}

func createCalendar(testingContext *testing.T, database *gorm.DB, ownerID string, identifier string) models.Calendar {
	testingContext.Helper()
	calendarRecord, calendarError := models.NewCalendar(ownerID, "Calendar", "calendar", "test", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct calendar: %v", calendarError)
	}
	calendarRecord.BaseModel.ID = identifier
	if createError := database.Create(calendarRecord).Error; createError != nil {
		testingContext.Fatalf("create calendar: %v", createError)
	}
	return *calendarRecord
}
