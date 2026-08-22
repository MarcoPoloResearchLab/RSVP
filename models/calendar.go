package models

import (
	"errors"
	"fmt"

	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/gorm"
)

const (
	// DefaultCalendarName is the calendar name for the existing RSVP event workflow.
	DefaultCalendarName       = "RSVP Events"
	defaultCalendarSymbol     = "calendar"
	defaultCalendarColorToken = "rsvp-primary"
)

var (
	// ErrOrganizerIDRequired indicates that a calendar has no organizer relationship.
	ErrOrganizerIDRequired = errors.New("organizer ID is required")
	// ErrOrganizerTimezoneRequired indicates that an organizer has not confirmed a client-supplied timezone.
	ErrOrganizerTimezoneRequired = errors.New("organizer timezone confirmation is required")
	// ErrCalendarIDRequired indicates that a lane has no calendar relationship.
	ErrCalendarIDRequired = errors.New("calendar ID is required")
	// ErrCalendarNameRequired indicates that a calendar has no name.
	ErrCalendarNameRequired = errors.New("calendar name is required")
	// ErrCalendarPresentationRequired indicates that a calendar has no symbol or color token.
	ErrCalendarPresentationRequired = errors.New("calendar symbol and color token are required")
	// ErrDisplayOrderInvalid indicates that a display order is negative.
	ErrDisplayOrderInvalid = errors.New("display order must not be negative")
)

// Calendar is an organizer-owned visibility family for lanes.
type Calendar struct {
	BaseModel
	OrganizerID  string `gorm:"type:varchar(8);not null;uniqueIndex:calendar_organizer_order,priority:1"`
	Name         string `gorm:"not null"`
	Symbol       string `gorm:"not null"`
	ColorToken   string `gorm:"not null"`
	DisplayOrder int    `gorm:"not null;uniqueIndex:calendar_organizer_order,priority:2;check:calendar_display_order,display_order >= 0"`
	Visible      bool   `gorm:"not null;default:true"`
	Organizer    User   `gorm:"foreignKey:OrganizerID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Lanes        []Lane `gorm:"foreignKey:CalendarID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// NewCalendar constructs one valid calendar.
func NewCalendar(organizerID string, name string, symbol string, colorToken string, displayOrder int) (*Calendar, error) {
	calendar := &Calendar{
		OrganizerID:  organizerID,
		Name:         name,
		Symbol:       symbol,
		ColorToken:   colorToken,
		DisplayOrder: displayOrder,
		Visible:      true,
	}
	if validationError := calendar.Validate(); validationError != nil {
		return nil, validationError
	}
	return calendar, nil
}

// Validate checks the calendar invariants.
func (calendar *Calendar) Validate() error {
	if calendar.OrganizerID == "" {
		return ErrOrganizerIDRequired
	}
	if calendar.Name == "" {
		return ErrCalendarNameRequired
	}
	if calendar.Symbol == "" || calendar.ColorToken == "" {
		return ErrCalendarPresentationRequired
	}
	if calendar.DisplayOrder < 0 {
		return ErrDisplayOrderInvalid
	}
	return nil
}

// BeforeCreate generates the identifier and validates the calendar.
func (calendar *Calendar) BeforeCreate(databaseConnection *gorm.DB) error {
	if validationError := calendar.Validate(); validationError != nil {
		return validationError
	}
	var organizer User
	if findError := databaseConnection.First(&organizer, "id = ?", calendar.OrganizerID).Error; findError != nil {
		return fmt.Errorf("find organizer %s for calendar: %w", calendar.OrganizerID, findError)
	}
	if organizer.Timezone == nil {
		return ErrOrganizerTimezoneRequired
	}
	if _, timezoneError := NewTimezone(*organizer.Timezone); timezoneError != nil {
		return timezoneError
	}
	return calendar.BaseModel.GenerateID(databaseConnection, calendar)
}

// BeforeUpdate validates the calendar.
func (calendar *Calendar) BeforeUpdate(*gorm.DB) error {
	return calendar.Validate()
}

// GetTableName returns the calendar table name.
func (calendar *Calendar) GetTableName() string {
	return config.TableCalendars
}

// GetIDGeneratorFunc returns the calendar identifier generator.
func (calendar *Calendar) GetIDGeneratorFunc() func(int) (string, error) {
	return GenerateBase62ID
}

// EnsureDefaultCalendar returns the canonical event calendar for an organizer.
func EnsureDefaultCalendar(databaseConnection *gorm.DB, organizerID string) (*Calendar, error) {
	var calendar Calendar
	findError := databaseConnection.Where("organizer_id = ? AND display_order = ?", organizerID, 0).First(&calendar).Error
	if findError == nil {
		return &calendar, nil
	}
	if !errors.Is(findError, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find default calendar for organizer %s: %w", organizerID, findError)
	}
	newCalendar, constructionError := NewCalendar(organizerID, DefaultCalendarName, defaultCalendarSymbol, defaultCalendarColorToken, 0)
	if constructionError != nil {
		return nil, constructionError
	}
	if createError := databaseConnection.Create(newCalendar).Error; createError != nil {
		return nil, fmt.Errorf("create default calendar for organizer %s: %w", organizerID, createError)
	}
	return newCalendar, nil
}
