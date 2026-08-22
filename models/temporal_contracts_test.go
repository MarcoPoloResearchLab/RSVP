package models_test

import (
	"errors"
	"testing"
	"time"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/models"
)

func TestLaneConstructorsRejectInvalidStates(testingContext *testing.T) {
	start := time.Date(2032, time.January, 1, 12, 0, 0, 0, time.UTC)
	openLane, openError := models.NewOpenLane("CAL00001", "Open subject", start, 0)
	if openError != nil {
		testingContext.Fatalf("construct open lane: %v", openError)
	}
	if openLane.EndsAt != nil {
		testingContext.Fatalf("open lane end = %v, want nil", openLane.EndsAt)
	}

	if _, finiteError := models.NewFiniteLane("CAL00001", "Invalid finite lane", start, start, 0); !errors.Is(finiteError, models.ErrLaneEndInvalid) {
		testingContext.Fatalf("invalid finite lane error = %v, want %v", finiteError, models.ErrLaneEndInvalid)
	}

	resolutionTime := start.Add(time.Hour)
	invalidResolvedLane := models.Lane{
		CalendarID:   "CAL00001",
		Title:        "Invalid resolved lane",
		Status:       models.LaneStatusResolved,
		StartsAt:     start,
		EndsAt:       &resolutionTime,
		DisplayOrder: 0,
	}
	if stateError := invalidResolvedLane.Validate(); !errors.Is(stateError, models.ErrLaneStateInvalid) {
		testingContext.Fatalf("invalid resolved lane error = %v, want %v", stateError, models.ErrLaneStateInvalid)
	}
}

func TestEventRequiresLaneAndValidTimezone(testingContext *testing.T) {
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	eventTime, eventTimeError := models.NewIntervalEventTime(testsupport.FixedStartTime(), testsupport.FixedStartTime().Add(time.Hour), timezone)
	if eventTimeError != nil {
		testingContext.Fatalf("construct event time: %v", eventTimeError)
	}
	if _, eventError := models.NewEvent("", "Missing lane", "", nil, models.IndependentEventRelation(), eventTime); !errors.Is(eventError, models.ErrLaneIDRequired) {
		testingContext.Fatalf("event without lane error = %v, want %v", eventError, models.ErrLaneIDRequired)
	}
	if _, invalidTimezoneError := models.NewTimezone("Pacific/Not_A_Zone"); !errors.Is(invalidTimezoneError, models.ErrTimezoneInvalid) {
		testingContext.Fatalf("invalid timezone error = %v, want %v", invalidTimezoneError, models.ErrTimezoneInvalid)
	}
	if _, localTimezoneError := models.NewTimezone("Local"); !errors.Is(localTimezoneError, models.ErrTimezoneInvalid) {
		testingContext.Fatalf("host-local timezone error = %v, want %v", localTimezoneError, models.ErrTimezoneInvalid)
	}
}

func TestIndependentEventsUseDifferentLanesInOneCalendar(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	firstEvent := fixture.CreateEvent("EVT00001", owner.ID, nil)
	secondEvent := fixture.CreateEvent("EVT00002", owner.ID, nil)
	if firstEvent.LaneID == secondEvent.LaneID {
		testingContext.Fatalf("independent event lane IDs are both %q", firstEvent.LaneID)
	}
	var firstLane models.Lane
	if findError := fixture.Database.First(&firstLane, "id = ?", firstEvent.LaneID).Error; findError != nil {
		testingContext.Fatalf("find first lane: %v", findError)
	}
	var secondLane models.Lane
	if findError := fixture.Database.First(&secondLane, "id = ?", secondEvent.LaneID).Error; findError != nil {
		testingContext.Fatalf("find second lane: %v", findError)
	}
	if firstLane.CalendarID != secondLane.CalendarID {
		testingContext.Fatalf("calendar IDs = %q and %q, want one calendar", firstLane.CalendarID, secondLane.CalendarID)
	}
}

func TestDependentEventsUseAnchorLane(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	anchor := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	relation, relationError := models.DependentEventRelation(anchor.ID)
	if relationError != nil {
		testingContext.Fatalf("construct dependent relation: %v", relationError)
	}
	eventTime, eventTimeError := models.NewIntervalEventTime(testsupport.FixedStartTime().Add(30*time.Minute), testsupport.FixedStartTime().Add(90*time.Minute), timezone)
	if eventTimeError != nil {
		testingContext.Fatalf("construct dependent time: %v", eventTimeError)
	}
	dependent, eventError := models.NewEvent(anchor.LaneID, "Dependent event", "", nil, relation, eventTime)
	if eventError != nil {
		testingContext.Fatalf("construct dependent event: %v", eventError)
	}
	if createError := dependent.Create(fixture.Database); createError != nil {
		testingContext.Fatalf("create dependent event: %v", createError)
	}
	if dependent.LaneID != anchor.LaneID {
		testingContext.Fatalf("dependent lane ID = %q, want %q", dependent.LaneID, anchor.LaneID)
	}
	anchor.Title = "Updated anchor"
	if updateError := anchor.Update(fixture.Database); updateError != nil {
		testingContext.Fatalf("update anchor with dependency chain: %v", updateError)
	}
}

func TestEventSeriesOccurrencesUseOneLane(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm organizer timezone: %v", confirmationError)
	}
	calendar, calendarError := models.EnsureDefaultCalendar(fixture.Database, owner.ID)
	if calendarError != nil {
		testingContext.Fatalf("create calendar: %v", calendarError)
	}
	lane, laneError := models.NewFiniteLane(calendar.ID, "Weekly series", testsupport.FixedStartTime().Add(-time.Hour), testsupport.FixedStartTime().Add(8*24*time.Hour), 0)
	if laneError != nil {
		testingContext.Fatalf("construct series lane: %v", laneError)
	}
	if createError := fixture.Database.Create(lane).Error; createError != nil {
		testingContext.Fatalf("create series lane: %v", createError)
	}
	series, seriesError := models.NewEventSeries(lane.ID, timezone, models.EventSourceLocal, nil)
	if seriesError != nil {
		testingContext.Fatalf("construct event series: %v", seriesError)
	}
	if createError := fixture.Database.Create(series).Error; createError != nil {
		testingContext.Fatalf("create event series: %v", createError)
	}
	relation, relationError := models.SeriesOccurrenceRelation(series.ID)
	if relationError != nil {
		testingContext.Fatalf("construct series relation: %v", relationError)
	}
	for occurrenceIndex, startOffset := range []time.Duration{0, 7 * 24 * time.Hour} {
		eventTime, eventTimeError := models.NewPointEventTime(testsupport.FixedStartTime().Add(startOffset), timezone)
		if eventTimeError != nil {
			testingContext.Fatalf("construct occurrence %d time: %v", occurrenceIndex, eventTimeError)
		}
		occurrence, eventError := models.NewEvent(lane.ID, "Series occurrence", "", nil, relation, eventTime)
		if eventError != nil {
			testingContext.Fatalf("construct occurrence %d: %v", occurrenceIndex, eventError)
		}
		if createError := occurrence.Create(fixture.Database); createError != nil {
			testingContext.Fatalf("create occurrence %d: %v", occurrenceIndex, createError)
		}
		if occurrence.LaneID != lane.ID {
			testingContext.Fatalf("occurrence %d lane ID = %q, want %q", occurrenceIndex, occurrence.LaneID, lane.ID)
		}
	}
}

func TestProbeRequiresAttentionPolicyLane(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm organizer timezone: %v", confirmationError)
	}
	calendar, calendarError := models.EnsureDefaultCalendar(fixture.Database, owner.ID)
	if calendarError != nil {
		testingContext.Fatalf("create calendar: %v", calendarError)
	}
	start := testsupport.FixedStartTime().Add(-time.Hour)
	policyLane, policyLaneError := models.NewOpenLane(calendar.ID, "Policy lane", start, 0)
	if policyLaneError != nil {
		testingContext.Fatalf("construct policy lane: %v", policyLaneError)
	}
	otherLane, otherLaneError := models.NewOpenLane(calendar.ID, "Other lane", start, 1)
	if otherLaneError != nil {
		testingContext.Fatalf("construct other lane: %v", otherLaneError)
	}
	if createError := fixture.Database.Create(policyLane).Error; createError != nil {
		testingContext.Fatalf("create policy lane: %v", createError)
	}
	if createError := fixture.Database.Create(otherLane).Error; createError != nil {
		testingContext.Fatalf("create other lane: %v", createError)
	}
	policy, policyError := models.NewAttentionPolicy(policyLane.ID, 7*24*time.Hour, testsupport.FixedStartTime(), nil)
	if policyError != nil {
		testingContext.Fatalf("construct policy: %v", policyError)
	}
	if createError := fixture.Database.Create(policy).Error; createError != nil {
		testingContext.Fatalf("create policy: %v", createError)
	}
	probe, probeError := models.NewProbe(policy.ID, otherLane.ID, testsupport.FixedStartTime(), nil)
	if probeError != nil {
		testingContext.Fatalf("construct probe: %v", probeError)
	}
	if createError := fixture.Database.Create(probe).Error; !errors.Is(createError, models.ErrProbeLaneMismatch) {
		testingContext.Fatalf("mismatched probe lane error = %v, want %v", createError, models.ErrProbeLaneMismatch)
	}
}

func TestAttentionAndProbeConstructorsRejectInvalidStates(testingContext *testing.T) {
	if _, policyError := models.NewAttentionPolicy("LAN00001", 0, testsupport.FixedStartTime(), nil); !errors.Is(policyError, models.ErrReviewIntervalInvalid) {
		testingContext.Fatalf("zero review interval error = %v, want %v", policyError, models.ErrReviewIntervalInvalid)
	}
	zeroEscalation := time.Duration(0)
	if _, policyError := models.NewAttentionPolicy("LAN00001", time.Hour, testsupport.FixedStartTime(), &zeroEscalation); !errors.Is(policyError, models.ErrEscalationIntervalInvalid) {
		testingContext.Fatalf("zero escalation interval error = %v, want %v", policyError, models.ErrEscalationIntervalInvalid)
	}
	if _, probeError := models.NewProbe("POL00001", "LAN00001", time.Time{}, nil); !errors.Is(probeError, models.ErrProbeDueRequired) {
		testingContext.Fatalf("missing probe due time error = %v, want %v", probeError, models.ErrProbeDueRequired)
	}
	dueAt := testsupport.FixedStartTime()
	escalatesAt := dueAt.Add(-time.Minute)
	if _, probeError := models.NewProbe("POL00001", "LAN00001", dueAt, &escalatesAt); !errors.Is(probeError, models.ErrProbeStateInvalid) {
		testingContext.Fatalf("invalid probe escalation error = %v, want %v", probeError, models.ErrProbeStateInvalid)
	}
}

func TestAllDayEventBoundsFollowDaylightTimeRules(testingContext *testing.T) {
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	startDate, startDateError := models.NewLocalDate("2026-03-08")
	if startDateError != nil {
		testingContext.Fatalf("construct start date: %v", startDateError)
	}
	endDate, endDateError := models.NewLocalDate("2026-03-09")
	if endDateError != nil {
		testingContext.Fatalf("construct end date: %v", endDateError)
	}
	eventTime, eventTimeError := models.NewAllDayEventTime(startDate, endDate, timezone)
	if eventTimeError != nil {
		testingContext.Fatalf("construct all-day time: %v", eventTimeError)
	}
	event, eventError := models.NewEvent("LAN00001", "Daylight event", "", nil, models.IndependentEventRelation(), eventTime)
	if eventError != nil {
		testingContext.Fatalf("construct all-day event: %v", eventError)
	}
	start, end, boundsError := event.MarkerBounds()
	if boundsError != nil {
		testingContext.Fatalf("read all-day bounds: %v", boundsError)
	}
	if duration := end.Sub(start); duration != 23*time.Hour {
		testingContext.Fatalf("all-day daylight duration = %s, want %s", duration, 23*time.Hour)
	}
}
