// Package utils provides miscellaneous utility functions used across the application,
// including HTTP parameter handling, URL building, error handling, and validation logic.
package utils

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/temirov/RSVP/pkg/config"
)

// Predefined validation error messages.
var (
	ErrTitleRequired         = errors.New("title is required")
	ErrTitleTooLong          = fmt.Errorf("title is too long (maximum %d characters)", config.MaxTitleLength)
	ErrStartTimeRequired     = errors.New("start time is required")
	ErrStartTimeInPast       = errors.New("start time must be in the future")
	ErrDurationRequired      = errors.New("duration is required")
	ErrDurationInvalid       = fmt.Errorf("duration must be between %d and %d hours", config.MinEventDuration, config.MaxEventDuration)
	ErrNameRequired          = errors.New("name is required")
	ErrNameTooLong           = fmt.Errorf("name is too long (maximum %d characters)", config.MaxNameLength)
	ErrResponseInvalidFormat = fmt.Errorf("response status must be '%s', '%s', or '%s'", config.RSVPResponseYesPrefix, config.RSVPResponseNo, config.RSVPResponsePending)
	ErrGuestCountInvalid     = fmt.Errorf("guest count must be between 0 and %d", config.MaxGuestCount)
	ErrGuestCountRequired    = errors.New("extra guest count is required when responding 'Yes'")
)

// ValidateEventTitle checks if an event title is valid (not empty and within length limits).
// Returns nil if valid, or a specific error (ErrTitleRequired, ErrTitleTooLong) otherwise.
func ValidateEventTitle(eventTitle string) error {
	if eventTitle == "" {
		return ErrTitleRequired
	}
	if len(eventTitle) > config.MaxTitleLength {
		return ErrTitleTooLong
	}
	return nil
}

// ValidateEventStartTime checks if an event start time is valid (not zero and in the future).
// Returns nil if valid, or a specific error (ErrStartTimeRequired, ErrStartTimeInPast) otherwise.
func ValidateEventStartTime(startTime time.Time) error {
	if startTime.IsZero() {
		return ErrStartTimeRequired
	}
	if startTime.Before(time.Now()) {
		return ErrStartTimeInPast
	}
	return nil
}

// ValidateAndParseEventDuration checks if an event duration string is valid (represents an integer
// between MinEventDuration and MaxEventDuration) and returns the parsed integer duration.
// Returns the parsed duration and nil error if valid, or 0 and a specific error otherwise.
func ValidateAndParseEventDuration(durationString string) (int, error) {
	if durationString == "" {
		return 0, ErrDurationRequired
	}
	durationHours, err := strconv.Atoi(durationString)
	if err != nil {
		return 0, ErrDurationInvalid
	}
	if durationHours < config.MinEventDuration || durationHours > config.MaxEventDuration {
		return 0, ErrDurationInvalid
	}
	return durationHours, nil
}

// ValidateRSVPName checks if an RSVP name is valid (not empty and within length limits).
// Returns nil if valid, or a specific error (ErrNameRequired, ErrNameTooLong) otherwise.
func ValidateRSVPName(rsvpName string) error {
	if rsvpName == "" {
		return ErrNameRequired
	}
	if len(rsvpName) > config.MaxNameLength {
		return ErrNameTooLong
	}
	return nil
}

// ValidateRSVPResponseStatus checks if an RSVP response status string is valid.
// Valid values are defined by constants like config.RSVPResponseYesPrefix, config.RSVPResponseNo, config.RSVPResponsePending, or empty string.
// Returns nil if valid, ErrResponseInvalidFormat otherwise.
func ValidateRSVPResponseStatus(responseStatus string) error {
	switch responseStatus {
	// Use RSVPResponseYesPrefix here for validation
	case config.RSVPResponseYesPrefix, config.RSVPResponseNo, config.RSVPResponsePending, "":
		return nil
	default:
		return ErrResponseInvalidFormat
	}
}

// ValidateExtraGuests checks if the provided guest count is within the allowed range [0, MaxGuestCount].
// Returns nil if valid, ErrGuestCountInvalid otherwise.
func ValidateExtraGuests(guestCount int) error {
	if guestCount < 0 || guestCount > config.MaxGuestCount {
		return ErrGuestCountInvalid
	}
	return nil
}
