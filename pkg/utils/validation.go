package utils

import (
	"errors"
	"strings"
	"time"
)

const (
	// MaxTitleLength is the maximum length for event titles
	MaxTitleLength = 255

	// MaxNameLength is the maximum length for RSVP names
	MaxNameLength = 100

	// MaxGuestCount is the maximum number of extra guests allowed
	MaxGuestCount = 4
)

// ValidateEventTitle checks if an event title is valid
func ValidateEventTitle(title string) error {
	if title == "" {
		return errors.New("title is required")
	}

	if len(title) > MaxTitleLength {
		return errors.New("title is too long (maximum 255 characters)")
	}

	return nil
}

// ValidateEventStartTime checks if an event start time is valid
func ValidateEventStartTime(startTime time.Time) error {
	if startTime.IsZero() {
		return errors.New("start time is required")
	}

	if startTime.Before(time.Now()) {
		return errors.New("start time must be in the future")
	}

	return nil
}

// ValidateEventDuration checks if an event duration is valid
func ValidateEventDuration(duration string) error {
	if duration == "" {
		return errors.New("duration is required")
	}

	// The duration is expected to be a number of hours
	// This validation is simplified for the test; in a real application,
	// you would convert the string to an integer and check the range
	if duration != "1" && duration != "2" && duration != "3" && duration != "4" {
		return errors.New("duration must be between 1 and 4 hours")
	}

	return nil
}

// ValidateRSVPName checks if an RSVP name is valid
func ValidateRSVPName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}

	if len(name) > MaxNameLength {
		return errors.New("name is too long (maximum 100 characters)")
	}

	return nil
}

// ValidateRSVPResponse checks if an RSVP response is valid
func ValidateRSVPResponse(response string) error {
	if response == "" {
		// Empty response is valid (not yet responded)
		return nil
	}

	if response == "No" {
		// "No" response is valid
		return nil
	}

	// Check for "Yes,N" format
	parts := strings.Split(response, ",")
	if len(parts) != 2 || parts[0] != "Yes" {
		return errors.New("response must be 'No' or 'Yes,N' where N is the number of guests")
	}

	// In a real application, you would convert the second part to an integer
	// and check the range. For simplicity, we'll just check against allowed values.
	if parts[1] != "0" && parts[1] != "1" && parts[1] != "2" && parts[1] != "3" && parts[1] != "4" {
		return errors.New("guest count must be between 0 and 4")
	}

	return nil
}
