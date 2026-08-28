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

func TestDerivedMarkersRecalculateRejectCyclesAndDeleteWithRule(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	anchor := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	service, serviceError := services.NewDerivedMarkerService(fixture.Database)
	if serviceError != nil {
		testingContext.Fatalf("construct service: %v", serviceError)
	}
	beforeRule, beforeMarker, beforeError := service.Create(context.Background(), owner.ID, models.DerivedAnchorEvent, anchor.ID, models.DerivedAnchorStart, -3600)
	if beforeError != nil {
		testingContext.Fatalf("create before marker: %v", beforeError)
	}
	if beforeMarker.LaneID != anchor.LaneID || anchor.StartsAt == nil || !beforeMarker.At.Equal(anchor.StartsAt.Add(-time.Hour)) {
		testingContext.Fatalf("before marker = %#v, anchor = %#v", beforeMarker, anchor)
	}
	afterRule, afterMarker, afterError := service.Create(context.Background(), owner.ID, models.DerivedAnchorEvent, anchor.ID, models.DerivedAnchorEnd, 1800)
	if afterError != nil {
		testingContext.Fatalf("create after marker: %v", afterError)
	}
	if anchor.EndsAt == nil || !afterMarker.At.Equal(anchor.EndsAt.Add(30*time.Minute)) {
		testingContext.Fatalf("after marker = %#v", afterMarker)
	}

	originalBeforeAt := beforeMarker.At
	rollbackTransaction := fixture.Database.Begin()
	rollbackStart := anchor.StartsAt.Add(2 * time.Hour)
	rollbackEnd := anchor.EndsAt.Add(2 * time.Hour)
	var rollbackLane models.Lane
	if findError := rollbackTransaction.First(&rollbackLane, "id = ?", anchor.LaneID).Error; findError != nil {
		testingContext.Fatalf("read rollback lane: %v", findError)
	}
	if updateError := rollbackTransaction.Model(&rollbackLane).Update("ends_at", rollbackEnd.Add(time.Hour)).Error; updateError != nil {
		testingContext.Fatalf("expand rollback lane: %v", updateError)
	}
	if timeError := anchor.SetIntervalTime(rollbackStart, rollbackEnd, models.Timezone(anchor.Timezone)); timeError != nil {
		testingContext.Fatalf("set rollback anchor: %v", timeError)
	}
	if updateError := anchor.Update(rollbackTransaction); updateError != nil {
		testingContext.Fatalf("update rollback anchor: %v", updateError)
	}
	if derivedError := services.RecalculateDerivedMarkersForAnchor(rollbackTransaction, owner.ID, models.DerivedAnchorEvent, anchor.ID); derivedError != nil {
		testingContext.Fatalf("recalculate rollback marker: %v", derivedError)
	}
	rollbackTransaction.Rollback()
	var rolledBack models.DerivedMarker
	if findError := fixture.Database.First(&rolledBack, "id = ?", beforeMarker.ID).Error; findError != nil {
		testingContext.Fatalf("read rolled back marker: %v", findError)
	}
	if !rolledBack.At.Equal(originalBeforeAt) {
		testingContext.Fatalf("rolled back marker at = %v, want %v", rolledBack.At, originalBeforeAt)
	}

	var storedAnchor models.Event
	if findError := fixture.Database.First(&storedAnchor, "id = ?", anchor.ID).Error; findError != nil {
		testingContext.Fatalf("reload anchor: %v", findError)
	}
	commitStart := storedAnchor.StartsAt.Add(24 * time.Hour)
	commitEnd := storedAnchor.EndsAt.Add(24 * time.Hour)
	commitTransaction := fixture.Database.Begin()
	var lane models.Lane
	if findError := commitTransaction.First(&lane, "id = ?", storedAnchor.LaneID).Error; findError != nil {
		testingContext.Fatalf("read lane: %v", findError)
	}
	if updateError := commitTransaction.Model(&lane).Update("ends_at", commitEnd.Add(time.Hour)).Error; updateError != nil {
		testingContext.Fatalf("expand commit lane: %v", updateError)
	}
	if timeError := storedAnchor.SetIntervalTime(commitStart, commitEnd, models.Timezone(storedAnchor.Timezone)); timeError != nil {
		testingContext.Fatalf("set committed anchor: %v", timeError)
	}
	if updateError := storedAnchor.Update(commitTransaction); updateError != nil {
		testingContext.Fatalf("update committed anchor: %v", updateError)
	}
	if derivedError := services.RecalculateDerivedMarkersForAnchor(commitTransaction, owner.ID, models.DerivedAnchorEvent, storedAnchor.ID); derivedError != nil {
		testingContext.Fatalf("recalculate committed markers: %v", derivedError)
	}
	if commitError := commitTransaction.Commit().Error; commitError != nil {
		testingContext.Fatalf("commit anchor update: %v", commitError)
	}
	if findError := fixture.Database.First(&rolledBack, "id = ?", beforeMarker.ID).Error; findError != nil {
		testingContext.Fatalf("reload committed marker: %v", findError)
	}
	if !rolledBack.At.Equal(commitStart.Add(-time.Hour)) {
		testingContext.Fatalf("committed marker at = %v, want %v", rolledBack.At, commitStart.Add(-time.Hour))
	}

	chainRule, chainMarker, chainError := service.Create(context.Background(), owner.ID, models.DerivedAnchorDerived, beforeMarker.ID, models.DerivedAnchorStart, 300)
	if chainError != nil {
		testingContext.Fatalf("create derived chain: %v", chainError)
	}
	if _, _, cycleError := service.Update(context.Background(), owner.ID, beforeRule.ID, models.DerivedAnchorDerived, chainMarker.ID, models.DerivedAnchorStart, -3600); !errors.Is(cycleError, services.ErrDerivedMarkerCycle) {
		testingContext.Fatalf("cycle error = %v", cycleError)
	}

	rolledBack.At = rolledBack.At.Add(time.Hour)
	if saveError := fixture.Database.Save(&rolledBack).Error; !errors.Is(saveError, models.ErrDerivedMarkerImmutable) {
		testingContext.Fatalf("direct marker update error = %v", saveError)
	}
	if deleteError := service.Delete(context.Background(), owner.ID, afterRule.ID); deleteError != nil {
		testingContext.Fatalf("delete derived rule: %v", deleteError)
	}
	if findError := fixture.Database.Unscoped().First(&models.DerivedMarker{}, "id = ?", afterMarker.ID).Error; !errors.Is(findError, gorm.ErrRecordNotFound) {
		testingContext.Fatalf("deleted marker lookup = %v", findError)
	}
	if deleteError := service.Delete(context.Background(), owner.ID, chainRule.ID); deleteError != nil {
		testingContext.Fatalf("delete chain rule: %v", deleteError)
	}
}

func TestDerivedMarkerRejectsAllDayAnchor(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	seed := fixture.CreateEvent(testsupport.EventID, owner.ID, nil)
	var seedLane models.Lane
	if findError := fixture.Database.First(&seedLane, "id = ?", seed.LaneID).Error; findError != nil {
		testingContext.Fatalf("read seed lane: %v", findError)
	}
	order, _ := models.NextLaneDisplayOrder(fixture.Database, seedLane.CalendarID)
	start := time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)
	lane, laneError := models.NewFiniteLane(seedLane.CalendarID, "All day", start, end, order)
	if laneError != nil {
		testingContext.Fatalf("construct lane: %v", laneError)
	}
	if createError := fixture.Database.Create(lane).Error; createError != nil {
		testingContext.Fatalf("create lane: %v", createError)
	}
	timezone, _ := models.NewTimezone(testsupport.TimezoneName)
	startDate, _ := models.NewLocalDate("2030-01-02")
	endDate, _ := models.NewLocalDate("2030-01-03")
	eventTime, _ := models.NewAllDayEventTime(startDate, endDate, timezone)
	allDay, _ := models.NewEvent(lane.ID, "All day", "", nil, models.IndependentEventRelation(), eventTime)
	if createError := fixture.Database.Create(allDay).Error; createError != nil {
		testingContext.Fatalf("create all-day event: %v", createError)
	}
	service, _ := services.NewDerivedMarkerService(fixture.Database)
	if _, _, createError := service.Create(context.Background(), owner.ID, models.DerivedAnchorEvent, allDay.ID, models.DerivedAnchorStart, 60); !errors.Is(createError, services.ErrDerivedAnchorAllDay) {
		testingContext.Fatalf("all-day anchor error = %v", createError)
	}
}

func TestDerivedMarkerPreservesSeriesRecurrenceBounds(testingContext *testing.T) {
	fixture := testsupport.NewFixture(testingContext)
	owner := fixture.CreateUser(testsupport.OwnerUserID)
	timezone, timezoneError := models.NewTimezone(testsupport.TimezoneName)
	if timezoneError != nil {
		testingContext.Fatalf("construct timezone: %v", timezoneError)
	}
	if confirmationError := owner.ConfirmTimezone(fixture.Database, timezone); confirmationError != nil {
		testingContext.Fatalf("confirm timezone: %v", confirmationError)
	}
	calendar, calendarError := models.EnsureDefaultCalendar(fixture.Database, owner.ID)
	if calendarError != nil {
		testingContext.Fatalf("create calendar: %v", calendarError)
	}
	recurrenceLimit := testsupport.FixedStartTime().Add(30 * 24 * time.Hour)
	lane, laneError := models.NewFiniteLane(calendar.ID, "Series", testsupport.FixedStartTime().Add(-time.Hour), recurrenceLimit, 0)
	if laneError != nil {
		testingContext.Fatalf("construct series lane: %v", laneError)
	}
	if createError := fixture.Database.Create(lane).Error; createError != nil {
		testingContext.Fatalf("create series lane: %v", createError)
	}
	series, seriesError := models.NewEventSeries(lane.ID, timezone, models.EventSourceLocal, nil)
	if seriesError != nil {
		testingContext.Fatalf("construct series: %v", seriesError)
	}
	if createError := fixture.Database.Create(series).Error; createError != nil {
		testingContext.Fatalf("create series: %v", createError)
	}
	occurrence, occurrenceError := models.CreateSeriesOccurrenceIntervalEvent(
		fixture.Database,
		owner.ID,
		series.ID,
		"Occurrence",
		"",
		nil,
		testsupport.FixedStartTime(),
		testsupport.FixedStartTime().Add(time.Hour),
		timezone,
	)
	if occurrenceError != nil {
		testingContext.Fatalf("create occurrence: %v", occurrenceError)
	}
	service, serviceError := services.NewDerivedMarkerService(fixture.Database)
	if serviceError != nil {
		testingContext.Fatalf("construct derived marker service: %v", serviceError)
	}
	if _, _, createError := service.Create(context.Background(), owner.ID, models.DerivedAnchorEvent, occurrence.ID, models.DerivedAnchorEnd, int64(time.Hour.Seconds())); createError != nil {
		testingContext.Fatalf("create series derived marker: %v", createError)
	}
	var storedLane models.Lane
	if findError := fixture.Database.First(&storedLane, "id = ?", lane.ID).Error; findError != nil {
		testingContext.Fatalf("reload series lane: %v", findError)
	}
	if storedLane.EndsAt == nil || !storedLane.EndsAt.Equal(recurrenceLimit) {
		testingContext.Fatalf("series lane end = %v, want recurrence limit %v", storedLane.EndsAt, recurrenceLimit)
	}
}
