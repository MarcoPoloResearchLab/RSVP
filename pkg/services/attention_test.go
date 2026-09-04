package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/services"
)

func TestAttentionCadenceEscalationAndResolution(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm timezone: %v", confirmationError)
	}
	calendar, calendarError := models.NewCalendar(owner.ID, "Waiting", "waiting", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct calendar: %v", calendarError)
	}
	calendar.BaseModel.ID = "CAL00001"
	if createError := fixture.Database.Create(calendar).Error; createError != nil {
		testingContext.Fatalf("create calendar: %v", createError)
	}
	startTime := time.Date(2030, time.January, 1, 8, 0, 0, 0, time.UTC)
	lane, laneError := models.NewOpenLane(calendar.ID, "Unresolved appeal", startTime, 0)
	if laneError != nil {
		testingContext.Fatalf("construct lane: %v", laneError)
	}
	lane.BaseModel.ID = "LAN00001"
	if createError := fixture.Database.Create(lane).Error; createError != nil {
		testingContext.Fatalf("create lane: %v", createError)
	}

	currentTime := startTime
	attentionService, serviceError := services.NewAttentionService(fixture.Database, func() time.Time { return currentTime })
	if serviceError != nil {
		testingContext.Fatalf("construct attention service: %v", serviceError)
	}
	reviewInterval := 7 * 24 * time.Hour
	escalationInterval := 24 * time.Hour
	firstDue := startTime.Add(2 * time.Hour)
	policy, createError := attentionService.CreatePolicy(context.Background(), owner.ID, lane.ID, reviewInterval, firstDue, &escalationInterval)
	if createError != nil {
		testingContext.Fatalf("create policy: %v", createError)
	}
	firstProbe := pendingProbe(testingContext, fixture, policy.ID)
	if !firstProbe.DueAt.Equal(firstDue) || firstProbe.EscalatesAt == nil || !firstProbe.EscalatesAt.Equal(firstDue.Add(escalationInterval)) {
		testingContext.Fatalf("first probe = %#v", firstProbe)
	}

	currentTime = firstDue.Add(30 * time.Minute)
	completedProbe, completeError := attentionService.CompleteProbe(context.Background(), owner.ID, firstProbe.ID)
	if completeError != nil {
		testingContext.Fatalf("complete probe: %v", completeError)
	}
	if completedProbe.State != models.ProbeStateCompleted || completedProbe.CompletedAt == nil || !completedProbe.CompletedAt.Equal(currentTime) {
		testingContext.Fatalf("completed probe = %#v", completedProbe)
	}
	secondProbe := pendingProbe(testingContext, fixture, policy.ID)
	if !secondProbe.DueAt.Equal(currentTime.Add(reviewInterval)) {
		testingContext.Fatalf("second due = %s, want %s", secondProbe.DueAt, currentTime.Add(reviewInterval))
	}

	currentTime = secondProbe.DueAt.Add(escalationInterval)
	missedCount, missedError := attentionService.MarkMissedProbes(context.Background(), owner.ID)
	if missedError != nil || missedCount != 1 {
		testingContext.Fatalf("mark missed count = %d, error = %v", missedCount, missedError)
	}
	var storedSecond models.Probe
	if findError := fixture.Database.First(&storedSecond, "id = ?", secondProbe.ID).Error; findError != nil {
		testingContext.Fatalf("reload missed probe: %v", findError)
	}
	if storedSecond.State != models.ProbeStateMissed {
		testingContext.Fatalf("second state = %q, want %q", storedSecond.State, models.ProbeStateMissed)
	}
	thirdProbe := pendingProbe(testingContext, fixture, policy.ID)

	managementService, managementError := services.NewTemporalManagementService(fixture.Database, func() time.Time { return currentTime })
	if managementError != nil {
		testingContext.Fatalf("construct management service: %v", managementError)
	}
	resolutionTime := currentTime.Add(time.Hour)
	if _, resolutionError := managementService.UpdateLane(context.Background(), owner.ID, lane.ID, services.LanePatch{ResolutionTime: &resolutionTime}); resolutionError != nil {
		testingContext.Fatalf("resolve lane: %v", resolutionError)
	}
	var canceledProbe models.Probe
	if findError := fixture.Database.First(&canceledProbe, "id = ?", thirdProbe.ID).Error; findError != nil {
		testingContext.Fatalf("reload canceled probe: %v", findError)
	}
	if canceledProbe.State != models.ProbeStateCanceled {
		testingContext.Fatalf("third state = %q, want %q", canceledProbe.State, models.ProbeStateCanceled)
	}
	if _, completeError := attentionService.CompleteProbe(context.Background(), owner.ID, thirdProbe.ID); !errors.Is(completeError, services.ErrProbeTransitionInvalid) {
		testingContext.Fatalf("complete canceled probe error = %v, want %v", completeError, services.ErrProbeTransitionInvalid)
	}
	var pendingCount int64
	if countError := fixture.Database.Model(&models.Probe{}).Where("policy_id = ? AND state = ?", policy.ID, models.ProbeStatePending).Count(&pendingCount).Error; countError != nil {
		testingContext.Fatalf("count pending probes: %v", countError)
	}
	if pendingCount != 0 {
		testingContext.Fatalf("pending probe count = %d, want 0", pendingCount)
	}
}

func TestAttentionOwnershipAndUniqueOccurrence(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	otherOwner := fixture.CreateUser(testsupport.OtherUserID)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm owner timezone: %v", confirmationError)
	}
	if confirmationError := otherOwner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm other owner timezone: %v", confirmationError)
	}
	calendar, calendarError := models.NewCalendar(owner.ID, "Waiting", "waiting", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct calendar: %v", calendarError)
	}
	calendar.BaseModel.ID = "CAL00001"
	if createError := fixture.Database.Create(calendar).Error; createError != nil {
		testingContext.Fatalf("create calendar: %v", createError)
	}
	lane, laneError := models.NewOpenLane(calendar.ID, "Waiting", testsupport.FixedStartTime().Add(-time.Hour), 0)
	if laneError != nil {
		testingContext.Fatalf("construct lane: %v", laneError)
	}
	lane.BaseModel.ID = "LAN00001"
	if createError := fixture.Database.Create(lane).Error; createError != nil {
		testingContext.Fatalf("create lane: %v", createError)
	}
	service, serviceError := services.NewAttentionService(fixture.Database, testsupport.FixedStartTime)
	if serviceError != nil {
		testingContext.Fatalf("construct service: %v", serviceError)
	}
	policy, createError := service.CreatePolicy(context.Background(), owner.ID, lane.ID, time.Hour, testsupport.FixedStartTime(), nil)
	if createError != nil {
		testingContext.Fatalf("create policy: %v", createError)
	}
	if _, readError := service.ReadPolicy(context.Background(), otherOwner.ID, policy.ID); !errors.Is(readError, services.ErrResourceForbidden) {
		testingContext.Fatalf("cross-owner read error = %v, want %v", readError, services.ErrResourceForbidden)
	}
	duplicate, duplicateError := models.NewProbe(policy.ID, lane.ID, testsupport.FixedStartTime(), nil)
	if duplicateError != nil {
		testingContext.Fatalf("construct duplicate probe: %v", duplicateError)
	}
	if createError := fixture.Database.Create(duplicate).Error; createError == nil {
		testingContext.Fatal("duplicate policy occurrence was stored")
	}
}

func pendingProbe(testingContext *testing.T, fixture *testsupport.Fixture, policyID string) models.Probe {
	testingContext.Helper()
	var probes []models.Probe
	if findError := fixture.Database.Where("policy_id = ? AND state = ?", policyID, models.ProbeStatePending).Find(&probes).Error; findError != nil {
		testingContext.Fatalf("find pending probe: %v", findError)
	}
	if len(probes) != 1 {
		testingContext.Fatalf("pending probe count = %d, want 1", len(probes))
	}
	return probes[0]
}
