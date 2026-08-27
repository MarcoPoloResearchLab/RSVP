package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/gorm"
)

func TestIngestionDraftCancellationCreatesNoTemporalResources(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	anchor := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	calendarID := eventCalendarID(testingContext, fixture.Database, anchor)
	service, serviceError := services.NewIngestionDraftService(fixture.Database, func() time.Time { return testsupport.FixedStartTime() })
	if serviceError != nil {
		testingContext.Fatalf("construct ingestion draft service: %v", serviceError)
	}
	before := temporalResourceCounts(testingContext, fixture.Database)
	start := testsupport.FixedStartTime().Add(48 * time.Hour)
	end := start.Add(time.Hour)
	draft, createError := service.Create(context.Background(), owner.ID, models.IngestionDraftProposal{
		Mode: models.IngestionModeDatedEvent, CalendarID: calendarID, Title: "Independent birthday",
		StartsAt: &start, EndsAt: &end, ReferenceTime: testsupport.FixedStartTime(), Timezone: testsupport.TimezoneName,
	})
	if createError != nil {
		testingContext.Fatalf("create draft: %v", createError)
	}
	if got := temporalResourceCounts(testingContext, fixture.Database); got != before {
		testingContext.Fatalf("temporal counts after draft = %#v, want %#v", got, before)
	}
	canceled, cancelError := service.Cancel(context.Background(), owner.ID, draft.ID)
	if cancelError != nil || canceled.Status != models.IngestionDraftCanceled {
		testingContext.Fatalf("canceled draft = %#v, error = %v", canceled, cancelError)
	}
	if got := temporalResourceCounts(testingContext, fixture.Database); got != before {
		testingContext.Fatalf("temporal counts after cancellation = %#v, want %#v", got, before)
	}
	if _, _, confirmError := service.Confirm(context.Background(), owner.ID, draft.ID, "canceled-draft"); !errors.Is(confirmError, services.ErrIngestionDraftNotReady) {
		testingContext.Fatalf("canceled confirmation error = %v", confirmError)
	}
}

func TestIngestionDraftConfirmsOpenLaneAttentionAndReplaysIdempotently(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	anchor := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	calendarID := eventCalendarID(testingContext, fixture.Database, anchor)
	now := testsupport.FixedStartTime()
	service, _ := services.NewIngestionDraftService(fixture.Database, func() time.Time { return now })
	reviewSeconds := int64((7 * 24 * time.Hour) / time.Second)
	nextProbe := now.Add(7 * 24 * time.Hour)
	draft, createError := service.Create(context.Background(), owner.ID, models.IngestionDraftProposal{
		Mode: models.IngestionModeOpenLane, CalendarID: calendarID, Title: "Waiting for weekly review",
		ReviewIntervalSeconds: &reviewSeconds, NextProbeAt: &nextProbe, ReferenceTime: now, Timezone: testsupport.TimezoneName,
	})
	if createError != nil {
		testingContext.Fatalf("create open-lane draft: %v", createError)
	}
	confirmation, reused, confirmError := service.Confirm(context.Background(), owner.ID, draft.ID, "weekly-waiting")
	if confirmError != nil || reused {
		testingContext.Fatalf("confirmation = %#v, reused = %t, error = %v", confirmation, reused, confirmError)
	}
	var lane models.Lane
	if findError := fixture.Database.First(&lane, "id = ?", confirmation.LaneID).Error; findError != nil {
		testingContext.Fatalf("read open lane: %v", findError)
	}
	if lane.Status != models.LaneStatusActive || lane.EndsAt != nil || lane.CalendarID != calendarID || lane.Title != "Waiting for weekly review" {
		testingContext.Fatalf("open lane = %#v", lane)
	}
	var policy models.AttentionPolicy
	if findError := fixture.Database.First(&policy, "id = ?", *confirmation.AttentionPolicyID).Error; findError != nil {
		testingContext.Fatalf("read attention policy: %v", findError)
	}
	if policy.ReviewIntervalSeconds != reviewSeconds || !policy.NextProbeAt.Equal(nextProbe) {
		testingContext.Fatalf("attention policy = %#v", policy)
	}
	var probe models.Probe
	if findError := fixture.Database.First(&probe, "policy_id = ?", policy.ID).Error; findError != nil {
		testingContext.Fatalf("read attention probe: %v", findError)
	}
	if probe.LaneID != lane.ID || !probe.DueAt.Equal(nextProbe) || probe.State != models.ProbeStatePending {
		testingContext.Fatalf("attention probe = %#v", probe)
	}
	replayed, replayedReuse, replayError := service.Confirm(context.Background(), owner.ID, draft.ID, "weekly-waiting")
	if replayError != nil || !replayedReuse || replayed.ID != confirmation.ID {
		testingContext.Fatalf("replayed confirmation = %#v, reused = %t, error = %v", replayed, replayedReuse, replayError)
	}
}

func TestIngestionDraftCorrectsTimezoneAndAssignsCanonicalEventLanes(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	anchor := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	calendarID := eventCalendarID(testingContext, fixture.Database, anchor)
	now := testsupport.FixedStartTime()
	service, _ := services.NewIngestionDraftService(fixture.Database, func() time.Time { return now })
	losAngeles, _ := time.LoadLocation(testsupport.TimezoneName)
	initialStart := time.Date(2030, time.February, 3, 10, 0, 0, 0, losAngeles)
	initialEnd := initialStart.Add(time.Hour)
	draft, createError := service.Create(context.Background(), owner.ID, models.IngestionDraftProposal{
		Mode: models.IngestionModeDatedEvent, CalendarID: calendarID, Title: "Independent birthday",
		StartsAt: &initialStart, EndsAt: &initialEnd, ReferenceTime: now, Timezone: testsupport.TimezoneName,
	})
	if createError != nil {
		testingContext.Fatalf("create independent draft: %v", createError)
	}
	newYork, _ := time.LoadLocation("America/New_York")
	correctedStart := time.Date(2030, time.February, 4, 10, 30, 0, 0, newYork)
	correctedEnd := correctedStart.Add(90 * time.Minute)
	if _, updateError := service.Update(context.Background(), owner.ID, draft.ID, models.IngestionDraftProposal{
		Mode: models.IngestionModeDatedEvent, CalendarID: calendarID, Title: "Corrected independent birthday",
		StartsAt: &correctedStart, EndsAt: &correctedEnd, ReferenceTime: now, Timezone: "America/New_York",
	}); updateError != nil {
		testingContext.Fatalf("correct timezone and time: %v", updateError)
	}
	independent, _, confirmError := service.Confirm(context.Background(), owner.ID, draft.ID, "independent-birthday")
	if confirmError != nil {
		testingContext.Fatalf("confirm independent event: %v", confirmError)
	}
	if independent.EventID == nil || independent.LaneID == anchor.LaneID {
		testingContext.Fatalf("independent confirmation = %#v, anchor lane = %q", independent, anchor.LaneID)
	}
	var birthday models.Event
	if findError := fixture.Database.First(&birthday, "id = ?", *independent.EventID).Error; findError != nil {
		testingContext.Fatalf("read independent event: %v", findError)
	}
	if birthday.StartsAt == nil || !birthday.StartsAt.Equal(correctedStart.UTC()) || birthday.EndsAt == nil || !birthday.EndsAt.Equal(correctedEnd.UTC()) || birthday.Timezone != "America/New_York" {
		testingContext.Fatalf("corrected event = %#v", birthday)
	}

	travelStart := correctedEnd.Add(24 * time.Hour)
	travelEnd := travelStart.Add(3 * time.Hour)
	dependentDraft, dependentError := service.Create(context.Background(), owner.ID, models.IngestionDraftProposal{
		Mode: models.IngestionModeDatedEvent, CalendarID: calendarID, Title: "Travel after birthday", AnchorEventID: &anchor.ID,
		StartsAt: &travelStart, EndsAt: &travelEnd, ReferenceTime: now, Timezone: "America/New_York",
	})
	if dependentError != nil {
		testingContext.Fatalf("create dependent draft: %v", dependentError)
	}
	dependent, _, dependentConfirmError := service.Confirm(context.Background(), owner.ID, dependentDraft.ID, "dependent-travel")
	if dependentConfirmError != nil || dependent.LaneID != anchor.LaneID {
		testingContext.Fatalf("dependent confirmation = %#v, error = %v", dependent, dependentConfirmError)
	}
	var travel models.Event
	if findError := fixture.Database.First(&travel, "id = ?", *dependent.EventID).Error; findError != nil {
		testingContext.Fatalf("read dependent event: %v", findError)
	}
	if travel.RelationType != models.EventRelationDependent || travel.AnchorEventID == nil || *travel.AnchorEventID != anchor.ID {
		testingContext.Fatalf("dependent travel event = %#v", travel)
	}

	secondStart := travelEnd.Add(24 * time.Hour)
	secondEnd := secondStart.Add(time.Hour)
	secondDraft, secondError := service.Create(context.Background(), owner.ID, models.IngestionDraftProposal{
		Mode: models.IngestionModeDatedEvent, CalendarID: calendarID, Title: "Idempotency conflict",
		StartsAt: &secondStart, EndsAt: &secondEnd, ReferenceTime: now, Timezone: "America/New_York",
	})
	if secondError != nil {
		testingContext.Fatalf("create second draft: %v", secondError)
	}
	if _, _, conflictError := service.Confirm(context.Background(), owner.ID, secondDraft.ID, "independent-birthday"); !errors.Is(conflictError, services.ErrIdempotencyConflict) {
		testingContext.Fatalf("idempotency conflict error = %v", conflictError)
	}
}

type temporalCounts struct {
	lanes    int64
	events   int64
	policies int64
	probes   int64
}

func temporalResourceCounts(testingContext *testing.T, database *gorm.DB) temporalCounts {
	testingContext.Helper()
	counts := temporalCounts{}
	checks := []struct {
		model any
		value *int64
	}{{&models.Lane{}, &counts.lanes}, {&models.Event{}, &counts.events}, {&models.AttentionPolicy{}, &counts.policies}, {&models.Probe{}, &counts.probes}}
	for _, check := range checks {
		if countError := database.Model(check.model).Count(check.value).Error; countError != nil {
			testingContext.Fatalf("count %T: %v", check.model, countError)
		}
	}
	return counts
}

func eventCalendarID(testingContext *testing.T, database *gorm.DB, event models.Event) string {
	testingContext.Helper()
	var lane models.Lane
	if findError := database.First(&lane, "id = ?", event.LaneID).Error; findError != nil {
		testingContext.Fatalf("read fixture lane: %v", findError)
	}
	return lane.CalendarID
}
