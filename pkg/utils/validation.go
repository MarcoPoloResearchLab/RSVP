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
	ErrTitleRequired         = errors.New("event title is required")
	ErrTitleTooLong          = fmt.Errorf("event title is too long (maximum %d characters)", config.MaxTitleLength)
	ErrStartTimeRequired     = errors.New("start time is required")
	ErrStartTimeInPast       = errors.New("start time must be in the future")
	ErrDurationRequired      = errors.New("duration is required")
	ErrDurationInvalid       = fmt.Errorf("duration must be between %d and %d hours", config.MinEventDuration, config.MaxEventDuration)
	ErrNameRequired          = errors.New("name is required")
	ErrNameTooLong           = fmt.Errorf("name is too long (maximum %d characters)", config.MaxNameLength)
	ErrResponseInvalidFormat = fmt.Errorf("response status must be '%s', '%s', or '%s'", config.RSVPResponseYesPrefix, config.RSVPResponseNo, config.RSVPResponsePending)
	ErrGuestCountInvalid     = fmt.Errorf("guest count must be between 0 and %d", config.MaxGuestCount)
	ErrGuestCountRequired    = errors.New("extra guest count is required when responding 'Yes'")
	ErrVenueNameRequired     = errors.New("venue name is required")
	ErrVenueNameTooLong      = fmt.Errorf("venue name is too long (maximum %d characters)", config.MaxVenueNameLength)
	ErrUserIDRequired        = errors.New("user association is required") // Added error for missing UserID
)

// IsValidationError checks if the provided error is one of the known validation errors.
// Returns the original error if it's a validation error, nil otherwise.
func IsValidationError(err error) error {
	if errors.Is(err, ErrTitleRequired) || errors.Is(err, ErrTitleTooLong) ||
		errors.Is(err, ErrStartTimeRequired) || errors.Is(err, ErrStartTimeInPast) ||
		errors.Is(err, ErrDurationRequired) || errors.Is(err, ErrDurationInvalid) ||
		errors.Is(err, ErrNameRequired) || errors.Is(err, ErrNameTooLong) ||
		errors.Is(err, ErrResponseInvalidFormat) || errors.Is(err, ErrGuestCountInvalid) ||
		errors.Is(err, ErrGuestCountRequired) ||
		errors.Is(err, ErrVenueNameRequired) || errors.Is(err, ErrVenueNameTooLong) ||
		errors.Is(err, ErrUserIDRequired) {
		return err
	}
	return nil
}

// ValidateEventTitle checks if an event title is valid.
func ValidateEventTitle(eventTitle string) error {
	if eventTitle == "" {
		return ErrTitleRequired
	}
	if len(eventTitle) > config.MaxTitleLength {
		return ErrTitleTooLong
	}
	return nil
}

// ValidateEventStartTime checks if an event start time is valid.
func ValidateEventStartTime(startTime time.Time) error {
	if startTime.IsZero() {
		return ErrStartTimeRequired
	}
	return nil
}

// ValidateAndParseEventDuration checks and parses an event duration string.
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

// ValidateRSVPName checks if an RSVP name is valid.
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
func ValidateRSVPResponseStatus(responseStatus string) error {
	switch responseStatus {
	case config.RSVPResponseYesPrefix, config.RSVPResponseNo, config.RSVPResponsePending, "":
		return nil
	default:
		return ErrResponseInvalidFormat
	}
}

// ValidateExtraGuests checks if the provided guest count is valid.
func ValidateExtraGuests(guestCount int) error {
	if guestCount < 0 || guestCount > config.MaxGuestCount {
		return ErrGuestCountInvalid
	}
	return nil
}

// ValidateVenueName checks if a venue name is valid.
func ValidateVenueName(venueName string) error {
	if venueName == "" {
		return ErrVenueNameRequired
	}
	if len(venueName) > config.MaxVenueNameLength {
		return ErrVenueNameTooLong
	}
	return nil
}

// MustParseInt safely parses an integer string, returning 0 on error.
func MustParseInt(input string) int {
	parsedValue, parseError := strconv.Atoi(input)
	if parseError != nil {
		return 0
	}
	return parsedValue
}
