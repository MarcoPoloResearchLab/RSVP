package utils

import (
	"errors"
	"strings"
	"time"
)

const (
	MaxTitleLength = 255
	MaxNameLength  = 100
	MaxGuestCount  = 4
)

// ValidateEventTitle checks if an event title is valid.
func ValidateEventTitle(eventTitle string) error {
	if eventTitle == "" {
		return errors.New("title is required")
	}
	if len(eventTitle) > MaxTitleLength {
		return errors.New("title is too long (maximum 255 characters)")
	}
	return nil
}

// ValidateEventStartTime checks if an event start time is valid.
func ValidateEventStartTime(startTime time.Time) error {
	if startTime.IsZero() {
		return errors.New("start time is required")
	}
	if startTime.Before(time.Now()) {
		return errors.New("start time must be in the future")
	}
	return nil
}

// ValidateEventDuration checks if an event duration is valid.
func ValidateEventDuration(durationString string) error {
	if durationString == "" {
		return errors.New("duration is required")
	}
	if durationString != "1" && durationString != "2" && durationString != "3" && durationString != "4" {
		return errors.New("duration must be between 1 and 4 hours")
	}
	return nil
}

// ValidateRSVPName checks if an RSVP name is valid.
func ValidateRSVPName(rsvpName string) error {
	if rsvpName == "" {
		return errors.New("name is required")
	}
	if len(rsvpName) > MaxNameLength {
		return errors.New("name is too long (maximum 100 characters)")
	}
	return nil
}

// ValidateRSVPResponse checks if an RSVP response is valid.
func ValidateRSVPResponse(rsvpResponse string) error {
	if rsvpResponse == "" {
		return nil
	}
	if rsvpResponse == "No" {
		return nil
	}
	parts := strings.Split(rsvpResponse, ",")
	if len(parts) != 2 || parts[0] != "Yes" {
		return errors.New("response must be 'No' or 'Yes,N' where N is the number of guests")
	}
	if parts[1] != "0" && parts[1] != "1" && parts[1] != "2" && parts[1] != "3" && parts[1] != "4" {
		return errors.New("guest count must be between 0 and 4")
	}
	return nil
}
