package handlers

import (
	"regexp"

	"github.com/temirov/RSVP/pkg/config" // Import config for constant
)

// rsvpCodeValidationRegex is the compiled regular expression for validating RSVP codes.
var rsvpCodeValidationRegex = regexp.MustCompile(config.RSVPCodeValidationRegexPattern)

// ValidateRSVPCode checks if the provided string is a valid RSVP code format.
// A valid code consists of 1 to 8 alphanumeric characters (0-9, a-z, A-Z).
// Returns true if the code is valid, false otherwise.
func ValidateRSVPCode(rsvpCode string) bool {
	// Use the precompiled regex to match the input string.
	return rsvpCodeValidationRegex.MatchString(rsvpCode)
}
