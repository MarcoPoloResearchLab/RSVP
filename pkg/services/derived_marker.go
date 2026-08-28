package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tyemirov/RSVP/models"
	"gorm.io/gorm"
)

var (
	ErrDerivedAnchorAllDay = errors.New("all-day events cannot anchor derived markers")
	ErrDerivedMarkerCycle  = errors.New("derived marker relationship contains a cycle")
)

type DerivedMarkerService struct{ database *gorm.DB }

func NewDerivedMarkerService(database *gorm.DB) (*DerivedMarkerService, error) {
	if database == nil {
		return nil, errors.New("derived marker database is required")
	}
	return &DerivedMarkerService{database: database}, nil
}

func (service *DerivedMarkerService) Create(ctx context.Context, organizerID string, anchorType models.DerivedAnchorType, anchorID string, anchorEdge models.DerivedAnchorEdge, offsetSeconds int64) (*models.DerivedMarkerRule, *models.DerivedMarker, error) {
	var rule *models.DerivedMarkerRule
	var marker *models.DerivedMarker
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var createError error
		rule, marker, createError = CreateDerivedMarkerForAnchor(transaction, organizerID, anchorType, anchorID, anchorEdge, offsetSeconds)
		return createError
	})
	return rule, marker, transactionError
}

// CreateDerivedMarkerForAnchor creates one derived rule and marker in the caller transaction.
func CreateDerivedMarkerForAnchor(database *gorm.DB, organizerID string, anchorType models.DerivedAnchorType, anchorID string, anchorEdge models.DerivedAnchorEdge, offsetSeconds int64) (*models.DerivedMarkerRule, *models.DerivedMarker, error) {
	laneID, anchorAt, timezone, anchorError := derivedAnchor(database, organizerID, anchorType, anchorID, anchorEdge)
	if anchorError != nil {
		return nil, nil, anchorError
	}
	rule, ruleError := models.NewDerivedMarkerRule(laneID, anchorType, anchorID, anchorEdge, offsetSeconds)
	if ruleError != nil {
		return nil, nil, ruleError
	}
	if createError := database.Create(rule).Error; createError != nil {
		return nil, nil, createError
	}
	marker, markerError := models.NewDerivedMarker(rule.ID, laneID, anchorAt.Add(time.Duration(offsetSeconds)*time.Second), timezone)
	if markerError != nil {
		return nil, nil, markerError
	}
	if createError := database.Create(marker).Error; createError != nil {
		return nil, nil, createError
	}
	if boundsError := includeDerivedTimeInLane(database, laneID, marker.At); boundsError != nil {
		return nil, nil, boundsError
	}
	if boundsError := RecalculateTemporalLaneBounds(database, laneID); boundsError != nil {
		return nil, nil, boundsError
	}
	return rule, marker, nil
}

func (service *DerivedMarkerService) Update(ctx context.Context, organizerID string, ruleID string, anchorType models.DerivedAnchorType, anchorID string, anchorEdge models.DerivedAnchorEdge, offsetSeconds int64) (*models.DerivedMarkerRule, *models.DerivedMarker, error) {
	var rule models.DerivedMarkerRule
	var marker models.DerivedMarker
	transactionError := service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if findError := transaction.First(&rule, "id = ?", ruleID).Error; findError != nil {
			return findError
		}
		if ownerError := requireLaneOwner(transaction, rule.LaneID, organizerID); ownerError != nil {
			return ownerError
		}
		if findError := transaction.First(&marker, "rule_id = ?", rule.ID).Error; findError != nil {
			return findError
		}
		laneID, _, _, anchorError := derivedAnchor(transaction, organizerID, anchorType, anchorID, anchorEdge)
		if anchorError != nil {
			return anchorError
		}
		if laneID != rule.LaneID {
			return models.ErrEventMembershipInvalid
		}
		if cycleError := rejectDerivedCycle(transaction, marker.ID, anchorType, anchorID); cycleError != nil {
			return cycleError
		}
		candidate, candidateError := models.NewDerivedMarkerRule(rule.LaneID, anchorType, anchorID, anchorEdge, offsetSeconds)
		if candidateError != nil {
			return candidateError
		}
		rule.AnchorType, rule.AnchorID, rule.AnchorEdge, rule.OffsetSeconds = candidate.AnchorType, candidate.AnchorID, candidate.AnchorEdge, candidate.OffsetSeconds
		if saveError := transaction.Save(&rule).Error; saveError != nil {
			return saveError
		}
		if recalculateError := recalculateDerivedRule(transaction, organizerID, &rule, map[string]bool{}); recalculateError != nil {
			return recalculateError
		}
		return transaction.First(&marker, "rule_id = ?", rule.ID).Error
	})
	return &rule, &marker, transactionError
}

func (service *DerivedMarkerService) Delete(ctx context.Context, organizerID string, ruleID string) error {
	return service.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var rule models.DerivedMarkerRule
		if findError := transaction.First(&rule, "id = ?", ruleID).Error; findError != nil {
			return findError
		}
		if ownerError := requireLaneOwner(transaction, rule.LaneID, organizerID); ownerError != nil {
			return ownerError
		}
		var marker models.DerivedMarker
		if findError := transaction.First(&marker, "rule_id = ?", rule.ID).Error; findError != nil {
			return findError
		}
		if downstreamError := DeleteDerivedMarkersForAnchor(transaction, models.DerivedAnchorDerived, marker.ID); downstreamError != nil {
			return downstreamError
		}
		if deleteError := transaction.Unscoped().Delete(&marker).Error; deleteError != nil {
			return deleteError
		}
		if deleteError := transaction.Unscoped().Delete(&rule).Error; deleteError != nil {
			return deleteError
		}
		return RecalculateTemporalLaneBounds(transaction, rule.LaneID)
	})
}

// RecalculateDerivedMarkersForAnchor updates all transitive derived markers in the anchor transaction.
func RecalculateDerivedMarkersForAnchor(database *gorm.DB, organizerID string, anchorType models.DerivedAnchorType, anchorID string) error {
	var rules []models.DerivedMarkerRule
	if findError := database.Where("anchor_type = ? AND anchor_id = ?", anchorType, anchorID).Find(&rules).Error; findError != nil {
		return findError
	}
	visited := make(map[string]bool)
	for ruleIndex := range rules {
		if recalculateError := recalculateDerivedRule(database, organizerID, &rules[ruleIndex], visited); recalculateError != nil {
			return recalculateError
		}
	}
	return nil
}

// DeleteDerivedMarkersForAnchor removes all transitive rules whose anchor is being deleted.
func DeleteDerivedMarkersForAnchor(database *gorm.DB, anchorType models.DerivedAnchorType, anchorID string) error {
	var rules []models.DerivedMarkerRule
	if findError := database.Where("anchor_type = ? AND anchor_id = ?", anchorType, anchorID).Find(&rules).Error; findError != nil {
		return findError
	}
	for ruleIndex := range rules {
		var marker models.DerivedMarker
		if findError := database.First(&marker, "rule_id = ?", rules[ruleIndex].ID).Error; findError != nil {
			return findError
		}
		if downstreamError := DeleteDerivedMarkersForAnchor(database, models.DerivedAnchorDerived, marker.ID); downstreamError != nil {
			return downstreamError
		}
		if deleteError := database.Unscoped().Delete(&marker).Error; deleteError != nil {
			return deleteError
		}
		if deleteError := database.Unscoped().Delete(&rules[ruleIndex]).Error; deleteError != nil {
			return deleteError
		}
	}
	return nil
}

func recalculateDerivedRule(database *gorm.DB, organizerID string, rule *models.DerivedMarkerRule, visited map[string]bool) error {
	if visited[rule.ID] {
		return ErrDerivedMarkerCycle
	}
	visited[rule.ID] = true
	laneID, anchorAt, timezone, anchorError := derivedAnchor(database, organizerID, rule.AnchorType, rule.AnchorID, rule.AnchorEdge)
	if anchorError != nil {
		return anchorError
	}
	if laneID != rule.LaneID {
		return models.ErrEventMembershipInvalid
	}
	var marker models.DerivedMarker
	if findError := database.First(&marker, "rule_id = ?", rule.ID).Error; findError != nil {
		return findError
	}
	markerAt := anchorAt.Add(time.Duration(rule.OffsetSeconds) * time.Second).UTC()
	if updateError := database.Model(&models.DerivedMarker{}).Where("id = ?", marker.ID).UpdateColumns(map[string]any{"at": markerAt, "timezone": timezone.String()}).Error; updateError != nil {
		return updateError
	}
	if boundsError := includeDerivedTimeInLane(database, rule.LaneID, markerAt); boundsError != nil {
		return boundsError
	}
	if downstreamError := RecalculateDerivedMarkersForAnchor(database, organizerID, models.DerivedAnchorDerived, marker.ID); downstreamError != nil {
		return downstreamError
	}
	delete(visited, rule.ID)
	return RecalculateTemporalLaneBounds(database, rule.LaneID)
}

func derivedAnchor(database *gorm.DB, organizerID string, anchorType models.DerivedAnchorType, anchorID string, edge models.DerivedAnchorEdge) (string, time.Time, models.Timezone, error) {
	if edge != models.DerivedAnchorStart && edge != models.DerivedAnchorEnd {
		return "", time.Time{}, "", models.ErrDerivedMarkerRuleInvalid
	}
	switch anchorType {
	case models.DerivedAnchorEvent:
		var event models.Event
		if findError := database.First(&event, "id = ?", anchorID).Error; findError != nil {
			return "", time.Time{}, "", findError
		}
		if ownerError := requireLaneOwner(database, event.LaneID, organizerID); ownerError != nil {
			return "", time.Time{}, "", ownerError
		}
		if event.TimeShape == models.EventTimeAllDay {
			return "", time.Time{}, "", ErrDerivedAnchorAllDay
		}
		start, end, boundsError := event.MarkerBounds()
		if boundsError != nil {
			return "", time.Time{}, "", boundsError
		}
		anchorAt := start
		if edge == models.DerivedAnchorEnd {
			anchorAt = end
		}
		timezone, timezoneError := models.NewTimezone(event.Timezone)
		return event.LaneID, anchorAt, timezone, timezoneError
	case models.DerivedAnchorProbe:
		var probe models.Probe
		if findError := database.First(&probe, "id = ?", anchorID).Error; findError != nil {
			return "", time.Time{}, "", findError
		}
		timezone, ownerError := laneOwnerTimezone(database, probe.LaneID, organizerID)
		return probe.LaneID, probe.DueAt.UTC(), timezone, ownerError
	case models.DerivedAnchorDerived:
		var marker models.DerivedMarker
		if findError := database.First(&marker, "id = ?", anchorID).Error; findError != nil {
			return "", time.Time{}, "", findError
		}
		if ownerError := requireLaneOwner(database, marker.LaneID, organizerID); ownerError != nil {
			return "", time.Time{}, "", ownerError
		}
		timezone, timezoneError := models.NewTimezone(marker.Timezone)
		return marker.LaneID, marker.At.UTC(), timezone, timezoneError
	default:
		return "", time.Time{}, "", models.ErrDerivedMarkerRuleInvalid
	}
}

func rejectDerivedCycle(database *gorm.DB, targetMarkerID string, anchorType models.DerivedAnchorType, anchorID string) error {
	if anchorType != models.DerivedAnchorDerived {
		return nil
	}
	currentID := anchorID
	for currentID != "" {
		if currentID == targetMarkerID {
			return ErrDerivedMarkerCycle
		}
		var marker models.DerivedMarker
		if findError := database.First(&marker, "id = ?", currentID).Error; findError != nil {
			return findError
		}
		var rule models.DerivedMarkerRule
		if findError := database.First(&rule, "id = ?", marker.RuleID).Error; findError != nil {
			return findError
		}
		if rule.AnchorType != models.DerivedAnchorDerived {
			return nil
		}
		currentID = rule.AnchorID
	}
	return nil
}

func includeDerivedTimeInLane(database *gorm.DB, laneID string, markerAt time.Time) error {
	var lane models.Lane
	if findError := database.First(&lane, "id = ?", laneID).Error; findError != nil {
		return findError
	}
	updates := map[string]any{}
	if markerAt.Before(lane.StartsAt) {
		updates["starts_at"] = markerAt.UTC()
	}
	if lane.EndsAt != nil && markerAt.After(*lane.EndsAt) {
		updates["ends_at"] = markerAt.UTC()
	}
	if len(updates) == 0 {
		return nil
	}
	return database.Model(&lane).Updates(updates).Error
}

// RecalculateTemporalLaneBounds sets a finite lane end from all current marker variants.
func RecalculateTemporalLaneBounds(database *gorm.DB, laneID string) error {
	var lane models.Lane
	if findError := database.First(&lane, "id = ?", laneID).Error; findError != nil {
		return findError
	}
	if lane.Status != models.LaneStatusActive || lane.EndsAt == nil {
		return nil
	}
	var eventSeriesCount int64
	if countError := database.Model(&models.EventSeries{}).Where("lane_id = ?", laneID).Count(&eventSeriesCount).Error; countError != nil {
		return fmt.Errorf("count event series for temporal lane %s: %w", laneID, countError)
	}
	if eventSeriesCount != 0 {
		return nil
	}
	last := lane.StartsAt
	var events []models.Event
	if findError := database.Where("lane_id = ?", laneID).Find(&events).Error; findError != nil {
		return findError
	}
	for eventIndex := range events {
		_, end, boundsError := events[eventIndex].MarkerBounds()
		if boundsError != nil {
			return boundsError
		}
		if end.After(last) {
			last = end
		}
	}
	var probes []models.Probe
	if findError := database.Where("lane_id = ?", laneID).Find(&probes).Error; findError != nil {
		return findError
	}
	for _, probe := range probes {
		if probe.DueAt.After(last) {
			last = probe.DueAt
		}
	}
	var markers []models.DerivedMarker
	if findError := database.Where("lane_id = ?", laneID).Find(&markers).Error; findError != nil {
		return findError
	}
	for _, marker := range markers {
		if marker.At.After(last) {
			last = marker.At
		}
	}
	if !last.After(lane.StartsAt) {
		last = lane.StartsAt.Add(time.Nanosecond)
	}
	return database.Model(&lane).Update("ends_at", last.UTC()).Error
}

func requireLaneOwner(database *gorm.DB, laneID string, organizerID string) error {
	_, ownerError := laneOwnerTimezone(database, laneID, organizerID)
	return ownerError
}

func laneOwnerTimezone(database *gorm.DB, laneID string, organizerID string) (models.Timezone, error) {
	var user models.User
	queryError := database.Model(&models.User{}).
		Joins("JOIN calendars ON calendars.organizer_id = users.id AND calendars.deleted_at IS NULL").
		Joins("JOIN lanes ON lanes.calendar_id = calendars.id AND lanes.deleted_at IS NULL").
		Where("lanes.id = ?", laneID).First(&user).Error
	if queryError != nil {
		return "", queryError
	}
	if user.ID != organizerID {
		return "", ErrResourceForbidden
	}
	if user.Timezone == nil {
		return "", models.ErrOrganizerTimezoneRequired
	}
	timezone, timezoneError := models.NewTimezone(*user.Timezone)
	if timezoneError != nil {
		return "", fmt.Errorf("read lane owner timezone: %w", timezoneError)
	}
	return timezone, nil
}
