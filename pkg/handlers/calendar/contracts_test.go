package calendar_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers/calendar"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

func TestCalendarResourceOperationsPersistOrderAndVisibility(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)

	firstResponse := requestCalendar(testingContext, fixture, owner, http.MethodPost, config.WebCalendars, `{"name":"Personal","color_token":"personal","timezone":"America/Los_Angeles"}`, "application/json")
	if firstResponse.Code != http.StatusCreated {
		testingContext.Fatalf("first create status = %d, want %d; body = %s", firstResponse.Code, http.StatusCreated, firstResponse.Body.String())
	}
	first := decodeCalendar(testingContext, firstResponse)
	if strings.Contains(firstResponse.Body.String(), `"symbol"`) {
		testingContext.Fatalf("calendar response contains removed symbol field: %s", firstResponse.Body.String())
	}
	if first.DisplayOrder != 0 || !first.Visible || firstResponse.Header().Get("Location") != config.WebCalendars+first.ID {
		testingContext.Fatalf("first calendar response = %#v; Location = %q", first, firstResponse.Header().Get("Location"))
	}

	secondResponse := requestCalendar(testingContext, fixture, owner, http.MethodPost, config.WebCalendars, `{"name":"Work","color_token":"work","timezone":"America/Los_Angeles"}`, "application/json")
	if secondResponse.Code != http.StatusCreated {
		testingContext.Fatalf("second create status = %d, want %d; body = %s", secondResponse.Code, http.StatusCreated, secondResponse.Body.String())
	}
	second := decodeCalendar(testingContext, secondResponse)

	updateResponse := requestCalendar(testingContext, fixture, owner, http.MethodPatch, config.WebCalendars+second.ID, `{"name":"Focused Work","display_order":0,"visible":false}`, "application/json")
	if updateResponse.Code != http.StatusOK {
		testingContext.Fatalf("update status = %d, want %d; body = %s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}
	updated := decodeCalendar(testingContext, updateResponse)
	if updated.Name != "Focused Work" || updated.DisplayOrder != 0 || updated.Visible {
		testingContext.Fatalf("updated calendar = %#v", updated)
	}
	readResponse := requestCalendar(testingContext, fixture, owner, http.MethodGet, config.WebCalendars+second.ID, "", "")
	if readResponse.Code != http.StatusOK || decodeCalendar(testingContext, readResponse).Name != "Focused Work" {
		testingContext.Fatalf("read status = %d; body = %s", readResponse.Code, readResponse.Body.String())
	}

	var reorderedFirst models.Calendar
	if findError := fixture.Database.First(&reorderedFirst, "id = ?", first.ID).Error; findError != nil {
		testingContext.Fatalf("reload first calendar: %v", findError)
	}
	if reorderedFirst.DisplayOrder != 1 {
		testingContext.Fatalf("first calendar order = %d, want 1", reorderedFirst.DisplayOrder)
	}

	deleteResponse := requestCalendar(testingContext, fixture, owner, http.MethodDelete, config.WebCalendars+first.ID, "", "")
	if deleteResponse.Code != http.StatusNoContent {
		testingContext.Fatalf("delete status = %d, want %d; body = %s", deleteResponse.Code, http.StatusNoContent, deleteResponse.Body.String())
	}
	var storedOwner models.User
	if findError := fixture.Database.First(&storedOwner, "id = ?", owner.ID).Error; findError != nil {
		testingContext.Fatalf("reload organizer: %v", findError)
	}
	if storedOwner.Timezone == nil || *storedOwner.Timezone != testsupport.TimezoneName {
		testingContext.Fatalf("organizer timezone = %v, want %q", storedOwner.Timezone, testsupport.TimezoneName)
	}
}

func TestCalendarDeletionAndOwnershipConflictsChangeNoResource(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	otherOwner := fixture.CreateUser(testsupport.OtherUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	confirmTimezone(testingContext, fixture.Database, &otherOwner)
	calendarRecord := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001", 0)
	laneRecord, laneError := models.NewOpenLane(calendarRecord.ID, "Protected lane", testsupport.FixedStartTime(), 0)
	if laneError != nil {
		testingContext.Fatalf("construct lane: %v", laneError)
	}
	if createError := fixture.Database.Create(laneRecord).Error; createError != nil {
		testingContext.Fatalf("create lane: %v", createError)
	}

	conflictResponse := requestCalendar(testingContext, fixture, owner, http.MethodDelete, config.WebCalendars+calendarRecord.ID, "", "")
	if conflictResponse.Code != http.StatusConflict {
		testingContext.Fatalf("nonempty delete status = %d, want %d; body = %s", conflictResponse.Code, http.StatusConflict, conflictResponse.Body.String())
	}
	for _, method := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete} {
		body := ""
		contentType := ""
		if method == http.MethodPatch {
			body = `{"visible":false}`
			contentType = "application/json"
		}
		response := requestCalendar(testingContext, fixture, otherOwner, method, config.WebCalendars+calendarRecord.ID, body, contentType)
		if response.Code != http.StatusForbidden {
			testingContext.Fatalf("other-owner %s status = %d, want %d; body = %s", method, response.Code, http.StatusForbidden, response.Body.String())
		}
	}
	var storedCalendar models.Calendar
	if findError := fixture.Database.First(&storedCalendar, "id = ?", calendarRecord.ID).Error; findError != nil {
		testingContext.Fatalf("reload protected calendar: %v", findError)
	}
	if !storedCalendar.Visible {
		testingContext.Fatal("another organizer changed calendar visibility")
	}
}

func TestCalendarResourceRejectsInvalidBoundaryRequests(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	calendarRecord := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001", 0)
	testCases := []struct {
		name        string
		method      string
		target      string
		body        string
		contentType string
		wantStatus  int
	}{
		{name: "collection method", method: http.MethodGet, target: config.WebCalendars, wantStatus: http.StatusMethodNotAllowed},
		{name: "item method", method: http.MethodPut, target: config.WebCalendars + calendarRecord.ID, body: `{}`, contentType: "application/json", wantStatus: http.StatusMethodNotAllowed},
		{name: "path", method: http.MethodGet, target: config.WebCalendars + calendarRecord.ID + "/extra", wantStatus: http.StatusNotFound},
		{name: "media type", method: http.MethodPatch, target: config.WebCalendars + calendarRecord.ID, body: `{"visible":false}`, contentType: "text/plain", wantStatus: http.StatusUnsupportedMediaType},
		{name: "empty patch", method: http.MethodPatch, target: config.WebCalendars + calendarRecord.ID, body: `{}`, contentType: "application/json", wantStatus: http.StatusUnprocessableEntity},
		{name: "unknown field", method: http.MethodPatch, target: config.WebCalendars + calendarRecord.ID, body: `{"legacy":true}`, contentType: "application/json", wantStatus: http.StatusBadRequest},
		{name: "removed symbol field", method: http.MethodPatch, target: config.WebCalendars + calendarRecord.ID, body: `{"symbol":"C"}`, contentType: "application/json", wantStatus: http.StatusBadRequest},
		{name: "removed create symbol field", method: http.MethodPost, target: config.WebCalendars, body: `{"name":"Legacy","symbol":"L","color_token":"legacy","timezone":"America/Los_Angeles"}`, contentType: "application/json", wantStatus: http.StatusBadRequest},
		{name: "trailing value", method: http.MethodPatch, target: config.WebCalendars + calendarRecord.ID, body: `{"visible":false}{}`, contentType: "application/json", wantStatus: http.StatusBadRequest},
	}
	for _, testCase := range testCases {
		testingContext.Run(testCase.name, func(testingContext *testing.T) {
			response := requestCalendar(testingContext, fixture, owner, testCase.method, testCase.target, testCase.body, testCase.contentType)
			if response.Code != testCase.wantStatus {
				testingContext.Fatalf("status = %d, want %d; body = %s", response.Code, testCase.wantStatus, response.Body.String())
			}
			if testCase.wantStatus != http.StatusNotFound {
				assertTypedError(testingContext, response)
			}
		})
	}
}

func TestCalendarResourceEnforcesVisibleCalendarLimit(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	calendars := make([]models.Calendar, 0, services.MaxVisibleCalendars+1)
	for calendarIndex := 0; calendarIndex <= services.MaxVisibleCalendars; calendarIndex++ {
		calendars = append(calendars, createCalendar(testingContext, fixture.Database, owner.ID, fmt.Sprintf("CAL%05d", calendarIndex), calendarIndex))
	}
	hiddenCalendar := calendars[len(calendars)-1]
	if updateError := fixture.Database.Model(&hiddenCalendar).Update("visible", false).Error; updateError != nil {
		testingContext.Fatalf("hide overflow calendar: %v", updateError)
	}

	overflowResponse := requestCalendar(testingContext, fixture, owner, http.MethodPatch, config.WebCalendars+hiddenCalendar.ID, `{"visible":true}`, "application/json")
	if overflowResponse.Code != http.StatusConflict {
		testingContext.Fatalf("overflow visibility status = %d, want %d; body = %s", overflowResponse.Code, http.StatusConflict, overflowResponse.Body.String())
	}
	createResponse := requestCalendar(testingContext, fixture, owner, http.MethodPost, config.WebCalendars, `{"name":"Overflow","color_token":"overflow","timezone":"America/Los_Angeles"}`, "application/json")
	if createResponse.Code != http.StatusConflict {
		testingContext.Fatalf("overflow create status = %d, want %d; body = %s", createResponse.Code, http.StatusConflict, createResponse.Body.String())
	}

	hideResponse := requestCalendar(testingContext, fixture, owner, http.MethodPatch, config.WebCalendars+calendars[0].ID, `{"visible":false}`, "application/json")
	if hideResponse.Code != http.StatusOK {
		testingContext.Fatalf("hide status = %d, want %d; body = %s", hideResponse.Code, http.StatusOK, hideResponse.Body.String())
	}
	showResponse := requestCalendar(testingContext, fixture, owner, http.MethodPatch, config.WebCalendars+hiddenCalendar.ID, `{"visible":true}`, "application/json")
	if showResponse.Code != http.StatusOK || !decodeCalendar(testingContext, showResponse).Visible {
		testingContext.Fatalf("replacement visibility status = %d; body = %s", showResponse.Code, showResponse.Body.String())
	}
}

type calendarBody struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DisplayOrder int    `json:"display_order"`
	Visible      bool   `json:"visible"`
}

func requestCalendar(testingContext *testing.T, fixture *testsupport.Fixture, owner models.User, method string, target string, body string, contentType string) *httptest.ResponseRecorder {
	testingContext.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, &owner))
	responseRecorder := httptest.NewRecorder()
	calendar.Handler(fixture.ApplicationContext).ServeHTTP(responseRecorder, request)
	return responseRecorder
}

func decodeCalendar(testingContext *testing.T, responseRecorder *httptest.ResponseRecorder) calendarBody {
	testingContext.Helper()
	var body calendarBody
	if decodeError := json.Unmarshal(responseRecorder.Body.Bytes(), &body); decodeError != nil {
		testingContext.Fatalf("decode calendar: %v", decodeError)
	}
	return body
}

func assertTypedError(testingContext *testing.T, responseRecorder *httptest.ResponseRecorder) {
	testingContext.Helper()
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

func createCalendar(testingContext *testing.T, database *gorm.DB, ownerID string, identifier string, order int) models.Calendar {
	testingContext.Helper()
	calendarRecord, calendarError := models.NewCalendar(ownerID, "Calendar", "test", order)
	if calendarError != nil {
		testingContext.Fatalf("construct calendar: %v", calendarError)
	}
	calendarRecord.BaseModel.ID = identifier
	if createError := database.Create(calendarRecord).Error; createError != nil {
		testingContext.Fatalf("create calendar: %v", createError)
	}
	return *calendarRecord
}
