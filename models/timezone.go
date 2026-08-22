package models

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrTimezoneRequired indicates that a temporal write has no client-supplied timezone.
	ErrTimezoneRequired = errors.New("timezone is required")
	// ErrTimezoneInvalid indicates that a timezone is not an IANA timezone name.
	ErrTimezoneInvalid = errors.New("timezone must be a valid IANA timezone name")
	// ErrLocalDateInvalid indicates that a local date does not use the canonical format.
	ErrLocalDateInvalid = errors.New("local date must use YYYY-MM-DD")
)

// Timezone is a validated IANA timezone name.
type Timezone string

// NewTimezone validates one client-supplied IANA timezone name.
func NewTimezone(name string) (Timezone, error) {
	if name == "" {
		return "", ErrTimezoneRequired
	}
	if name == "Local" {
		return "", fmt.Errorf("%w: %s", ErrTimezoneInvalid, name)
	}
	if _, loadError := time.LoadLocation(name); loadError != nil {
		return "", fmt.Errorf("%w: %s", ErrTimezoneInvalid, name)
	}
	return Timezone(name), nil
}

// String returns the canonical persisted timezone name.
func (timezone Timezone) String() string {
	return string(timezone)
}

// Location returns the location rules for the timezone.
func (timezone Timezone) Location() (*time.Location, error) {
	location, loadError := time.LoadLocation(timezone.String())
	if loadError != nil {
		return nil, fmt.Errorf("load timezone %s: %w", timezone, loadError)
	}
	return location, nil
}

// ParseLocalTime converts a client local time to its UTC instant.
func ParseLocalTime(value string, layout string, timezone Timezone) (time.Time, error) {
	location, locationError := timezone.Location()
	if locationError != nil {
		return time.Time{}, locationError
	}
	parsedTime, parseError := time.ParseInLocation(layout, value, location)
	if parseError != nil {
		return time.Time{}, parseError
	}
	return parsedTime.UTC(), nil
}

// LocalDate is a validated local calendar date in YYYY-MM-DD format.
type LocalDate string

// NewLocalDate validates a canonical local calendar date.
func NewLocalDate(value string) (LocalDate, error) {
	parsedDate, parseError := time.Parse(time.DateOnly, value)
	if parseError != nil || parsedDate.Format(time.DateOnly) != value {
		return "", ErrLocalDateInvalid
	}
	return LocalDate(value), nil
}

// String returns the canonical local date.
func (localDate LocalDate) String() string {
	return string(localDate)
}
