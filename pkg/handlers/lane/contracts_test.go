package lane_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"github.com/tyemirov/RSVP/pkg/handlers/lane"
	"github.com/tyemirov/RSVP/pkg/middleware"
	"gorm.io/gorm"
)

var laneReferenceTime = time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)

func TestLaneResourceOperationsPersistOrderMoveAndResolution(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	firstCalendar := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001", 0)
	secondCalendar := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00002", 1)

	openResponse := requestLane(testingContext, fixture, owner, http.MethodPost, config.WebLanes, `{"calendar_id":"CAL00001","title":"Open subject","kind":"open","timezone":"America/Los_Angeles"}`, "application/json")
	if openResponse.Code != http.StatusCreated {
		testingContext.Fatalf("open create status = %d, want %d; body = %s", openResponse.Code, http.StatusCreated, openResponse.Body.String())
	}
	openLane := decodeLane(testingContext, openResponse)
	if openLane.StartsAt != laneReferenceTime.Format(time.RFC3339Nano) || openLane.EndsAt != nil || openLane.DisplayOrder != 0 {
		testingContext.Fatalf("open lane = %#v", openLane)
	}

	finiteEnd := laneReferenceTime.Add(72 * time.Hour).Format(time.RFC3339)
	finiteResponse := requestLane(testingContext, fixture, owner, http.MethodPost, config.WebLanes, `{"calendar_id":"CAL00001","title":"Finite subject","kind":"finite","ends_at":"`+finiteEnd+`","timezone":"America/Los_Angeles"}`, "application/json")
	if finiteResponse.Code != http.StatusCreated {
		testingContext.Fatalf("finite create status = %d, want %d; body = %s", finiteResponse.Code, http.StatusCreated, finiteResponse.Body.String())
	}
	finiteLane := decodeLane(testingContext, finiteResponse)
	relatedEvent := createEvent(testingContext, fixture.Database, finiteLane.ID, "EVTREL01")

	reorderResponse := requestLane(testingContext, fixture, owner, http.MethodPatch, config.WebLanes+finiteLane.ID, `{"title":"First finite subject","display_order":0}`, "application/json")
	if reorderResponse.Code != http.StatusOK {
		testingContext.Fatalf("reorder status = %d, want %d; body = %s", reorderResponse.Code, http.StatusOK, reorderResponse.Body.String())
	}
	reordered := decodeLane(testingContext, reorderResponse)
	if reordered.Title != "First finite subject" || reordered.DisplayOrder != 0 {
		testingContext.Fatalf("reordered lane = %#v", reordered)
	}

	moveResponse := requestLane(testingContext, fixture, owner, http.MethodPatch, config.WebLanes+finiteLane.ID, `{"calendar_id":"`+secondCalendar.ID+`","display_order":0}`, "application/json")
	if moveResponse.Code != http.StatusOK {
		testingContext.Fatalf("move status = %d, want %d; body = %s", moveResponse.Code, http.StatusOK, moveResponse.Body.String())
	}
	moved := decodeLane(testingContext, moveResponse)
	if moved.CalendarID != secondCalendar.ID || moved.DisplayOrder != 0 {
		testingContext.Fatalf("moved lane = %#v", moved)
	}
	var storedRelatedEvent models.Event
	if findError := fixture.Database.First(&storedRelatedEvent, "id = ?", relatedEvent.ID).Error; findError != nil {
		testingContext.Fatalf("reload related event: %v", findError)
	}
	if storedRelatedEvent.LaneID != finiteLane.ID {
		testingContext.Fatalf("event lane ID = %q, want %q", storedRelatedEvent.LaneID, finiteLane.ID)
	}
	var storedOpen models.Lane
	if findError := fixture.Database.First(&storedOpen, "id = ?", openLane.ID).Error; findError != nil {
		testingContext.Fatalf("reload open lane: %v", findError)
	}
	if storedOpen.CalendarID != firstCalendar.ID || storedOpen.DisplayOrder != 0 {
		testingContext.Fatalf("open lane changed during finite move: %#v", storedOpen)
	}

	resolutionTime := laneReferenceTime.Add(24 * time.Hour)
	resolveResponse := requestLane(testingContext, fixture, owner, http.MethodPatch, config.WebLanes+openLane.ID, `{"resolved_at":"`+resolutionTime.Format(time.RFC3339)+`"}`, "application/json")
	if resolveResponse.Code != http.StatusOK {
		testingContext.Fatalf("resolve status = %d, want %d; body = %s", resolveResponse.Code, http.StatusOK, resolveResponse.Body.String())
	}
	resolved := decodeLane(testingContext, resolveResponse)
	if resolved.Status != models.LaneStatusResolved || resolved.EndsAt == nil || resolved.ResolvedAt == nil || *resolved.EndsAt != *resolved.ResolvedAt {
		testingContext.Fatalf("resolved lane = %#v", resolved)
	}

	readResponse := requestLane(testingContext, fixture, owner, http.MethodGet, config.WebLanes+finiteLane.ID, "", "")
	if readResponse.Code != http.StatusOK || decodeLane(testingContext, readResponse).CalendarID != secondCalendar.ID {
		testingContext.Fatalf("read status = %d; body = %s", readResponse.Code, readResponse.Body.String())
	}
	deleteResponse := requestLane(testingContext, fixture, owner, http.MethodDelete, config.WebLanes+finiteLane.ID, "", "")
	if deleteResponse.Code != http.StatusNoContent {
		testingContext.Fatalf("delete status = %d, want %d; body = %s", deleteResponse.Code, http.StatusNoContent, deleteResponse.Body.String())
	}
	if findError := fixture.Database.Unscoped().First(&models.Event{}, "id = ?", relatedEvent.ID).Error; findError == nil {
		testingContext.Fatal("related event remains after lane deletion")
	}
}

func TestLaneDeletionRulesCommitOrRejectOneTransaction(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	calendarRecord := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001", 0)
	laneRecord := createFiniteLane(testingContext, fixture.Database, calendarRecord.ID, "LAN00001", 0)
	eventRecord := createEvent(testingContext, fixture.Database, laneRecord.ID, "EVT00001")
	rsvpRecord := models.RSVP{BaseModel: models.BaseModel{ID: "RSVP0001"}, EventID: eventRecord.ID, Name: "Invitee", Response: config.RSVPResponsePending}
	if createError := rsvpRecord.Create(fixture.Database); createError != nil {
		testingContext.Fatalf("create RSVP: %v", createError)
	}

	conflictResponse := requestLane(testingContext, fixture, owner, http.MethodDelete, config.WebLanes+laneRecord.ID, "", "")
	if conflictResponse.Code != http.StatusConflict {
		testingContext.Fatalf("RSVP conflict status = %d, want %d; body = %s", conflictResponse.Code, http.StatusConflict, conflictResponse.Body.String())
	}
	if findError := fixture.Database.First(&models.Lane{}, "id = ?", laneRecord.ID).Error; findError != nil {
		testingContext.Fatalf("lane changed after conflict: %v", findError)
	}
	if deleteError := fixture.Database.Unscoped().Delete(&rsvpRecord).Error; deleteError != nil {
		testingContext.Fatalf("remove RSVP fixture: %v", deleteError)
	}
	deleteResponse := requestLane(testingContext, fixture, owner, http.MethodDelete, config.WebLanes+laneRecord.ID, "", "")
	if deleteResponse.Code != http.StatusNoContent {
		testingContext.Fatalf("eligible delete status = %d, want %d; body = %s", deleteResponse.Code, http.StatusNoContent, deleteResponse.Body.String())
	}
	if findError := fixture.Database.Unscoped().First(&models.Event{}, "id = ?", eventRecord.ID).Error; findError == nil {
		testingContext.Fatal("lane event remains after lane deletion")
	}

	sourceLane := createFiniteLane(testingContext, fixture.Database, calendarRecord.ID, "LAN00002", 0)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	series, seriesError := models.NewEventSeries(sourceLane.ID, timezone, models.EventSourceGoogle, nil)
	if seriesError != nil {
		testingContext.Fatalf("construct source series: %v", seriesError)
	}
	if createError := fixture.Database.Create(series).Error; createError != nil {
		testingContext.Fatalf("create source series: %v", createError)
	}
	sourceResponse := requestLane(testingContext, fixture, owner, http.MethodDelete, config.WebLanes+sourceLane.ID, "", "")
	if sourceResponse.Code != http.StatusConflict {
		testingContext.Fatalf("source lane status = %d, want %d; body = %s", sourceResponse.Code, http.StatusConflict, sourceResponse.Body.String())
	}
}

func TestLaneOwnershipAndBoundaryValidation(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	otherOwner := fixture.CreateUser(testsupport.OtherUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	confirmTimezone(testingContext, fixture.Database, &otherOwner)
	calendarRecord := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001", 0)
	laneRecord := createFiniteLane(testingContext, fixture.Database, calendarRecord.ID, "LAN00001", 0)

	for _, method := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete} {
		body := ""
		contentType := ""
		if method == http.MethodPatch {
			body = `{"title":"Changed"}`
			contentType = "application/json"
		}
		response := requestLane(testingContext, fixture, otherOwner, method, config.WebLanes+laneRecord.ID, body, contentType)
		if response.Code != http.StatusForbidden {
			testingContext.Fatalf("other-owner %s status = %d, want %d; body = %s", method, response.Code, http.StatusForbidden, response.Body.String())
		}
	}

	testCases := []struct {
		name        string
		method      string
		target      string
		body        string
		contentType string
		wantStatus  int
	}{
		{name: "collection method", method: http.MethodGet, target: config.WebLanes, wantStatus: http.StatusMethodNotAllowed},
		{name: "item method", method: http.MethodPut, target: config.WebLanes + laneRecord.ID, wantStatus: http.StatusMethodNotAllowed},
		{name: "path", method: http.MethodGet, target: config.WebLanes + laneRecord.ID + "/extra", wantStatus: http.StatusNotFound},
		{name: "media type", method: http.MethodPatch, target: config.WebLanes + laneRecord.ID, body: `{"title":"Changed"}`, contentType: "text/plain", wantStatus: http.StatusUnsupportedMediaType},
		{name: "unknown field", method: http.MethodPatch, target: config.WebLanes + laneRecord.ID, body: `{"legacy":true}`, contentType: "application/json", wantStatus: http.StatusBadRequest},
		{name: "finite without end", method: http.MethodPost, target: config.WebLanes, body: `{"calendar_id":"CAL00001","title":"Bad finite","kind":"finite","timezone":"America/Los_Angeles"}`, contentType: "application/json", wantStatus: http.StatusUnprocessableEntity},
		{name: "invalid resolution", method: http.MethodPatch, target: config.WebLanes + laneRecord.ID, body: `{"resolved_at":"2030-01-02T12:00:00Z"}`, contentType: "application/json", wantStatus: http.StatusUnprocessableEntity},
	}
	for _, testCase := range testCases {
		testingContext.Run(testCase.name, func(testingContext *testing.T) {
			response := requestLane(testingContext, fixture, owner, testCase.method, testCase.target, testCase.body, testCase.contentType)
			if response.Code != testCase.wantStatus {
				testingContext.Fatalf("status = %d, want %d; body = %s", response.Code, testCase.wantStatus, response.Body.String())
			}
		})
	}
}

func TestLaneResolutionRejectsFutureEvent(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	calendarRecord := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001", 0)
	openLane, laneError := models.NewOpenLane(calendarRecord.ID, "Open series", laneReferenceTime, 0)
	if laneError != nil {
		testingContext.Fatalf("construct open lane: %v", laneError)
	}
	openLane.BaseModel.ID = "LAN00001"
	if createError := fixture.Database.Create(openLane).Error; createError != nil {
		testingContext.Fatalf("create open lane: %v", createError)
	}
	createEvent(testingContext, fixture.Database, openLane.ID, "EVT00001")

	resolutionTime := laneReferenceTime.Add(12 * time.Hour)
	response := requestLane(testingContext, fixture, owner, http.MethodPatch, config.WebLanes+openLane.ID, `{"resolved_at":"`+resolutionTime.Format(time.RFC3339)+`"}`, "application/json")
	if response.Code != http.StatusUnprocessableEntity {
		testingContext.Fatalf("resolution status = %d, want %d; body = %s", response.Code, http.StatusUnprocessableEntity, response.Body.String())
	}
	var storedLane models.Lane
	if findError := fixture.Database.First(&storedLane, "id = ?", openLane.ID).Error; findError != nil {
		testingContext.Fatalf("reload open lane: %v", findError)
	}
	if storedLane.Status != models.LaneStatusActive || storedLane.EndsAt != nil || storedLane.ResolvedAt != nil {
		testingContext.Fatalf("lane changed after rejected resolution: %#v", storedLane)
	}
}

func TestLaneReorderMovesSoftDeletedOrdersAside(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	confirmTimezone(testingContext, fixture.Database, &owner)
	calendarRecord := createCalendar(testingContext, fixture.Database, owner.ID, "CAL00001", 0)
	deletedLane := createFiniteLane(testingContext, fixture.Database, calendarRecord.ID, "LAN00001", 0)
	activeLane := createFiniteLane(testingContext, fixture.Database, calendarRecord.ID, "LAN00002", 1)
	if deleteError := fixture.Database.Delete(&deletedLane).Error; deleteError != nil {
		testingContext.Fatalf("soft-delete lane: %v", deleteError)
	}

	response := requestLane(testingContext, fixture, owner, http.MethodPatch, config.WebLanes+activeLane.ID, `{"display_order":0}`, "application/json")
	if response.Code != http.StatusOK {
		testingContext.Fatalf("reorder status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if reordered := decodeLane(testingContext, response); reordered.DisplayOrder != 0 {
		testingContext.Fatalf("active lane order = %d, want 0", reordered.DisplayOrder)
	}
	var storedDeleted models.Lane
	if findError := fixture.Database.Unscoped().First(&storedDeleted, "id = ?", deletedLane.ID).Error; findError != nil {
		testingContext.Fatalf("reload soft-deleted lane: %v", findError)
	}
	if storedDeleted.DisplayOrder == 0 {
		testingContext.Fatal("soft-deleted lane still occupies active order 0")
	}
}

type laneBody struct {
	ID           string            `json:"id"`
	CalendarID   string            `json:"calendar_id"`
	Title        string            `json:"title"`
	Status       models.LaneStatus `json:"status"`
	StartsAt     string            `json:"starts_at"`
	EndsAt       *string           `json:"ends_at"`
	ResolvedAt   *string           `json:"resolved_at"`
	DisplayOrder int               `json:"display_order"`
}

func requestLane(testingContext *testing.T, fixture *testsupport.Fixture, owner models.User, method string, target string, body string, contentType string) *httptest.ResponseRecorder {
	testingContext.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, &owner))
	responseRecorder := httptest.NewRecorder()
	lane.Handler(fixture.ApplicationContext, func() time.Time { return laneReferenceTime }).ServeHTTP(responseRecorder, request)
	return responseRecorder
}

func decodeLane(testingContext *testing.T, responseRecorder *httptest.ResponseRecorder) laneBody {
	testingContext.Helper()
	var body laneBody
	if decodeError := json.Unmarshal(responseRecorder.Body.Bytes(), &body); decodeError != nil {
		testingContext.Fatalf("decode lane: %v", decodeError)
	}
	return body
}

func confirmTimezone(testingContext *testing.T, database *gorm.DB, owner *models.User) {
	testingContext.Helper()
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm timezone: %v", confirmationError)
	}
}

func createCalendar(testingContext *testing.T, database *gorm.DB, ownerID string, identifier string, order int) models.Calendar {
	testingContext.Helper()
	calendarRecord, calendarError := models.NewCalendar(ownerID, "Calendar "+identifier, "C", "test", order)
	if calendarError != nil {
		testingContext.Fatalf("construct calendar: %v", calendarError)
	}
	calendarRecord.BaseModel.ID = identifier
	if createError := database.Create(calendarRecord).Error; createError != nil {
		testingContext.Fatalf("create calendar: %v", createError)
	}
	return *calendarRecord
}

func createFiniteLane(testingContext *testing.T, database *gorm.DB, calendarID string, identifier string, order int) models.Lane {
	testingContext.Helper()
	laneRecord, laneError := models.NewFiniteLane(calendarID, "Lane "+identifier, laneReferenceTime, laneReferenceTime.Add(72*time.Hour), order)
	if laneError != nil {
		testingContext.Fatalf("construct lane: %v", laneError)
	}
	laneRecord.BaseModel.ID = identifier
	if createError := database.Create(laneRecord).Error; createError != nil {
		testingContext.Fatalf("create lane: %v", createError)
	}
	return *laneRecord
}

func createEvent(testingContext *testing.T, database *gorm.DB, laneID string, identifier string) models.Event {
	testingContext.Helper()
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct event timezone: %v", timezoneError)
	}
	eventTime, timeError := models.NewPointEventTime(laneReferenceTime.Add(24*time.Hour), timezone)
	if timeError != nil {
		testingContext.Fatalf("construct event time: %v", timeError)
	}
	eventRecord, eventError := models.NewEvent(laneID, "Event", "", nil, models.IndependentEventRelation(), eventTime)
	if eventError != nil {
		testingContext.Fatalf("construct event: %v", eventError)
	}
	eventRecord.BaseModel.ID = identifier
	if createError := eventRecord.Create(database); createError != nil {
		testingContext.Fatalf("create event: %v", createError)
	}
	return *eventRecord
}
