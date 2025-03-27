package handlers

import "regexp"

// ValidateRSVPCode ensures that the RSVP code is alphanumeric and up to 8 characters long.
func ValidateRSVPCode(rsvpCode string) bool {
	validCodePattern := regexp.MustCompile(`^[0-9a-zA-Z]{1,8}$`)
	return validCodePattern.MatchString(rsvpCode)
}
