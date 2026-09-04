package organizer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers/organizer"
	"github.com/tyemirov/RSVP/pkg/middleware"
)

type organizerBody struct {
	ID       string  `json:"id"`
	Timezone *string `json:"timezone"`
}

type errorBody struct {
	Error struct {
		Code      string            `json:"code"`
		Message   string            `json:"message"`
		Details   map[string]string `json:"details"`
		RequestID string            `json:"request_id"`
	} `json:"error"`
}

func TestOrganizerTimezoneUpdatePersistsWithoutChangingEventTimezone(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture, &owner)
	event := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	server := newOrganizerServer(testingContext, fixture, &owner)

	readResponse := requestOrganizer(testingContext, server, http.MethodGet, config.WebOrganizers+owner.ID, "", "")
	assertStatus(testingContext, readResponse, http.StatusOK)
	readBody := decodeOrganizer(testingContext, readResponse)
	if readBody.ID != owner.ID || readBody.Timezone == nil || *readBody.Timezone != testsupport.TimezoneName {
		testingContext.Fatalf("organizer response = %#v", readBody)
	}

	for range 2 {
		updateResponse := requestOrganizer(testingContext, server, http.MethodPatch, config.WebOrganizers+owner.ID, `{"timezone":"America/New_York"}`, "application/json")
		assertStatus(testingContext, updateResponse, http.StatusOK)
		updateBody := decodeOrganizer(testingContext, updateResponse)
		if updateBody.Timezone == nil || *updateBody.Timezone != "America/New_York" {
			testingContext.Fatalf("updated timezone = %#v", updateBody.Timezone)
		}
	}

	var storedOrganizer models.User
	if findError := storedOrganizer.FindByID(fixture.Database, owner.ID); findError != nil {
		testingContext.Fatalf("find updated organizer: %v", findError)
	}
	if storedOrganizer.Timezone == nil || *storedOrganizer.Timezone != "America/New_York" {
		testingContext.Fatalf("stored timezone = %#v", storedOrganizer.Timezone)
	}
	var storedEvent models.Event
	if findError := fixture.Database.First(&storedEvent, "id = ?", event.ID).Error; findError != nil {
		testingContext.Fatalf("find stored event: %v", findError)
	}
	if storedEvent.Timezone != testsupport.TimezoneName {
		testingContext.Fatalf("event timezone = %q, want %q", storedEvent.Timezone, testsupport.TimezoneName)
	}
}

func TestOrganizerTimezoneUpdateRejectsInvalidRequestsWithoutChangingTimezone(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	otherOwner := fixture.CreateUser(testsupport.OtherUserID)
	confirmTimezone(testingContext, fixture, &owner)
	server := newOrganizerServer(testingContext, fixture, &owner)

	testCases := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		status      int
		code        string
	}{
		{name: "invalid timezone", method: http.MethodPatch, path: config.WebOrganizers + owner.ID, body: `{"timezone":"Local"}`, contentType: "application/json", status: http.StatusUnprocessableEntity, code: "invalid_timezone"},
		{name: "empty patch", method: http.MethodPatch, path: config.WebOrganizers + owner.ID, body: `{}`, contentType: "application/json", status: http.StatusUnprocessableEntity, code: "invalid_organizer_patch"},
		{name: "unknown field", method: http.MethodPatch, path: config.WebOrganizers + owner.ID, body: `{"zone":"UTC"}`, contentType: "application/json", status: http.StatusBadRequest, code: "malformed_json"},
		{name: "trailing JSON", method: http.MethodPatch, path: config.WebOrganizers + owner.ID, body: `{"timezone":"UTC"}{}`, contentType: "application/json", status: http.StatusBadRequest, code: "malformed_json"},
		{name: "unsupported media type", method: http.MethodPatch, path: config.WebOrganizers + owner.ID, body: `{"timezone":"UTC"}`, contentType: "text/plain", status: http.StatusUnsupportedMediaType, code: "unsupported_media_type"},
		{name: "other organizer", method: http.MethodGet, path: config.WebOrganizers + otherOwner.ID, status: http.StatusForbidden, code: "organizer_forbidden"},
		{name: "unsupported method", method: http.MethodDelete, path: config.WebOrganizers + owner.ID, status: http.StatusMethodNotAllowed, code: "method_not_allowed"},
	}
	for _, testCase := range testCases {
		testingContext.Run(testCase.name, func(testingContext *testing.T) {
			response := requestOrganizer(testingContext, server, testCase.method, testCase.path, testCase.body, testCase.contentType)
			assertStatus(testingContext, response, testCase.status)
			responseBody := decodeError(testingContext, response)
			if responseBody.Error.Code != testCase.code || responseBody.Error.Message == "" || responseBody.Error.Details == nil || responseBody.Error.RequestID == "" {
				testingContext.Fatalf("typed error = %#v, want code %q", responseBody.Error, testCase.code)
			}
			if testCase.status == http.StatusMethodNotAllowed && response.Header.Get("Allow") != "GET, PATCH" {
				testingContext.Fatalf("Allow = %q", response.Header.Get("Allow"))
			}
		})
	}

	invalidPathResponse := requestOrganizer(testingContext, server, http.MethodGet, config.WebOrganizers+"invalid/path", "", "")
	assertStatus(testingContext, invalidPathResponse, http.StatusNotFound)
	invalidPathResponse.Body.Close()

	var storedOrganizer models.User
	if findError := storedOrganizer.FindByID(fixture.Database, owner.ID); findError != nil {
		testingContext.Fatalf("find organizer after rejected requests: %v", findError)
	}
	if storedOrganizer.Timezone == nil || *storedOrganizer.Timezone != testsupport.TimezoneName {
		testingContext.Fatalf("stored timezone after rejected requests = %#v", storedOrganizer.Timezone)
	}
}

func newOrganizerServer(testingContext *testing.T, fixture *testsupport.Fixture, owner *models.User) *httptest.Server {
	testingContext.Helper()
	resource := organizer.Handler(fixture.ApplicationContext)
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		requestContext := context.WithValue(request.Context(), middleware.ContextKeyUser, owner)
		resource.ServeHTTP(responseWriter, request.WithContext(requestContext))
	}))
	testingContext.Cleanup(server.Close)
	return server
}

func requestOrganizer(testingContext *testing.T, server *httptest.Server, method string, path string, body string, contentType string) *http.Response {
	testingContext.Helper()
	request, requestError := http.NewRequest(method, server.URL+path, bytes.NewBufferString(body))
	if requestError != nil {
		testingContext.Fatalf("create organizer request: %v", requestError)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, responseError := server.Client().Do(request)
	if responseError != nil {
		testingContext.Fatalf("request organizer: %v", responseError)
	}
	if response.Header.Get("Cache-Control") != "private, no-store" {
		testingContext.Fatalf("Cache-Control = %q", response.Header.Get("Cache-Control"))
	}
	return response
}

func assertStatus(testingContext *testing.T, response *http.Response, want int) {
	testingContext.Helper()
	if response.StatusCode != want {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		testingContext.Fatalf("status = %d, want %d; body = %s", response.StatusCode, want, body)
	}
}

func decodeOrganizer(testingContext *testing.T, response *http.Response) organizerBody {
	testingContext.Helper()
	defer response.Body.Close()
	var body organizerBody
	if decodeError := json.NewDecoder(response.Body).Decode(&body); decodeError != nil {
		testingContext.Fatalf("decode organizer response: %v", decodeError)
	}
	return body
}

func decodeError(testingContext *testing.T, response *http.Response) errorBody {
	testingContext.Helper()
	defer response.Body.Close()
	var body errorBody
	if decodeError := json.NewDecoder(response.Body).Decode(&body); decodeError != nil {
		testingContext.Fatalf("decode organizer error: %v", decodeError)
	}
	return body
}

func confirmTimezone(testingContext *testing.T, fixture *testsupport.Fixture, owner *models.User) {
	testingContext.Helper()
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct fixture timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm fixture timezone: %v", confirmationError)
	}
}
