package attentionpolicy_test

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
	"github.com/tyemirov/RSVP/pkg/handlers/attentionpolicy"
	"github.com/tyemirov/RSVP/pkg/handlers/probe"
	"github.com/tyemirov/RSVP/pkg/middleware"
)

func TestAttentionPolicyAndProbeResourceContracts(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm timezone: %v", confirmationError)
	}
	calendar, calendarError := models.NewCalendar(owner.ID, "Waiting", "W", "waiting", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct calendar: %v", calendarError)
	}
	calendar.BaseModel.ID = "CAL00001"
	if createError := fixture.Database.Create(calendar).Error; createError != nil {
		testingContext.Fatalf("create calendar: %v", createError)
	}
	startTime := time.Date(2030, time.January, 1, 8, 0, 0, 0, time.UTC)
	lane, laneError := models.NewOpenLane(calendar.ID, "Appeal", startTime, 0)
	if laneError != nil {
		testingContext.Fatalf("construct lane: %v", laneError)
	}
	lane.BaseModel.ID = "LAN00001"
	if createError := fixture.Database.Create(lane).Error; createError != nil {
		testingContext.Fatalf("create lane: %v", createError)
	}

	currentTime := startTime
	policyHandler := attentionpolicy.Handler(fixture.ApplicationContext, func() time.Time { return currentTime })
	createResponse := requestResource(testingContext, policyHandler, &owner, http.MethodPost, config.WebAttentionPolicies,
		`{"lane_id":"LAN00001","review_interval_seconds":604800,"next_probe_at":"2030-01-02T08:00:00Z","escalation_interval_seconds":86400}`)
	if createResponse.Code != http.StatusCreated {
		testingContext.Fatalf("create status = %d, want %d; body = %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	var policyBody struct {
		ID string `json:"id"`
	}
	if decodeError := json.Unmarshal(createResponse.Body.Bytes(), &policyBody); decodeError != nil {
		testingContext.Fatalf("decode policy: %v", decodeError)
	}
	var pending models.Probe
	if findError := fixture.Database.First(&pending, "policy_id = ? AND state = ?", policyBody.ID, models.ProbeStatePending).Error; findError != nil {
		testingContext.Fatalf("find pending probe: %v", findError)
	}

	currentTime = pending.DueAt.Add(time.Hour)
	probeHandler := probe.Handler(fixture.ApplicationContext, func() time.Time { return currentTime })
	completeResponse := requestResource(testingContext, probeHandler, &owner, http.MethodPatch, config.WebProbes+pending.ID, `{"state":"completed"}`)
	if completeResponse.Code != http.StatusOK || !strings.Contains(completeResponse.Body.String(), `"state":"completed"`) {
		testingContext.Fatalf("complete status = %d; body = %s", completeResponse.Code, completeResponse.Body.String())
	}
	var pendingCount int64
	if countError := fixture.Database.Model(&models.Probe{}).Where("policy_id = ? AND state = ?", policyBody.ID, models.ProbeStatePending).Count(&pendingCount).Error; countError != nil {
		testingContext.Fatalf("count next probe: %v", countError)
	}
	if pendingCount != 1 {
		testingContext.Fatalf("next pending count = %d, want 1", pendingCount)
	}

	deleteResponse := requestResource(testingContext, policyHandler, &owner, http.MethodDelete, config.WebAttentionPolicies+policyBody.ID, "")
	if deleteResponse.Code != http.StatusNoContent {
		testingContext.Fatalf("delete status = %d, want %d; body = %s", deleteResponse.Code, http.StatusNoContent, deleteResponse.Body.String())
	}
	var probeCount int64
	if countError := fixture.Database.Unscoped().Model(&models.Probe{}).Where("policy_id = ?", policyBody.ID).Count(&probeCount).Error; countError != nil {
		testingContext.Fatalf("count deleted probes: %v", countError)
	}
	if probeCount != 0 {
		testingContext.Fatalf("stored probe count after delete = %d, want 0", probeCount)
	}
}

func requestResource(testingContext *testing.T, handler http.Handler, owner *models.User, method string, target string, body string) *httptest.ResponseRecorder {
	testingContext.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyUser, owner))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
