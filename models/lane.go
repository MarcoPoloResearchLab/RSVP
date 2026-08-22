package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

// LaneStatus is one closed lane state.
type LaneStatus string

const (
	LaneStatusActive   LaneStatus = "active"
	LaneStatusResolved LaneStatus = "resolved"
)

var (
	ErrLaneIDRequired    = errors.New("lane ID is required")
	ErrLaneTitleRequired = errors.New("lane title is required")
	ErrLaneStartRequired = errors.New("lane start is required")
	ErrLaneEndInvalid    = errors.New("lane end must be after lane start")
	ErrLaneStateInvalid  = errors.New("lane state is invalid")
	ErrMarkerOutsideLane = errors.New("marker must stay within lane bounds")
)

// Lane is one timeline row in a calendar.
type Lane struct {
	BaseModel
	CalendarID   string     `gorm:"type:varchar(8);not null;index;uniqueIndex:lane_calendar_order,priority:1"`
	Title        string     `gorm:"not null"`
	Status       LaneStatus `gorm:"type:text;not null;check:lane_state,((status = 'active' AND resolved_at IS NULL AND (ends_at IS NULL OR ends_at > starts_at)) OR (status = 'resolved' AND ends_at > starts_at AND resolved_at = ends_at))"`
	StartsAt     time.Time  `gorm:"not null"`
	EndsAt       *time.Time
	ResolvedAt   *time.Time
	DisplayOrder int      `gorm:"not null;uniqueIndex:lane_calendar_order,priority:2;check:lane_display_order,display_order >= 0"`
	Calendar     Calendar `gorm:"foreignKey:CalendarID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Events       []Event  `gorm:"foreignKey:LaneID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// NewOpenLane constructs one active lane without an end time.
func NewOpenLane(calendarID string, title string, startsAt time.Time, displayOrder int) (*Lane, error) {
	lane := &Lane{CalendarID: calendarID, Title: title, Status: LaneStatusActive, StartsAt: startsAt.UTC(), DisplayOrder: displayOrder}
	if validationError := lane.Validate(); validationError != nil {
		return nil, validationError
	}
	return lane, nil
}

// NewFiniteLane constructs one active lane with a known end time.
func NewFiniteLane(calendarID string, title string, startsAt time.Time, endsAt time.Time, displayOrder int) (*Lane, error) {
	canonicalEnd := endsAt.UTC()
	lane := &Lane{CalendarID: calendarID, Title: title, Status: LaneStatusActive, StartsAt: startsAt.UTC(), EndsAt: &canonicalEnd, DisplayOrder: displayOrder}
	if validationError := lane.Validate(); validationError != nil {
		return nil, validationError
	}
	return lane, nil
}

// Validate checks the lane state and bounds.
func (lane *Lane) Validate() error {
	if lane.CalendarID == "" {
		return ErrCalendarIDRequired
	}
	if lane.Title == "" {
		return ErrLaneTitleRequired
	}
	if lane.StartsAt.IsZero() {
		return ErrLaneStartRequired
	}
	if lane.DisplayOrder < 0 {
		return ErrDisplayOrderInvalid
	}
	switch lane.Status {
	case LaneStatusActive:
		if lane.ResolvedAt != nil {
			return ErrLaneStateInvalid
		}
		if lane.EndsAt != nil && !lane.EndsAt.After(lane.StartsAt) {
			return ErrLaneEndInvalid
		}
	case LaneStatusResolved:
		if lane.EndsAt == nil || lane.ResolvedAt == nil || !lane.EndsAt.After(lane.StartsAt) || !lane.EndsAt.Equal(*lane.ResolvedAt) {
			return ErrLaneStateInvalid
		}
	default:
		return ErrLaneStateInvalid
	}
	return nil
}

func (lane *Lane) BeforeCreate(databaseConnection *gorm.DB) error {
	if validationError := lane.Validate(); validationError != nil {
		return validationError
	}
	return lane.BaseModel.GenerateID(databaseConnection, lane)
}

func (lane *Lane) BeforeUpdate(*gorm.DB) error {
	return lane.Validate()
}

func (lane *Lane) GetTableName() string                          { return config.TableLanes }
func (lane *Lane) GetIDGeneratorFunc() func(int) (string, error) { return GenerateBase62ID }

// NextLaneDisplayOrder returns the next order value in a calendar.
func NextLaneDisplayOrder(databaseConnection *gorm.DB, calendarID string) (int, error) {
	var maximumDisplayOrder *int
	if maximumError := databaseConnection.Unscoped().Model(&Lane{}).
		Select("MAX(display_order)").
		Where("calendar_id = ?", calendarID).
		Scan(&maximumDisplayOrder).Error; maximumError != nil {
		return 0, fmt.Errorf("find maximum lane order for calendar %s: %w", calendarID, maximumError)
	}
	if maximumDisplayOrder == nil {
		return 0, nil
	}
	return *maximumDisplayOrder + 1, nil
}
