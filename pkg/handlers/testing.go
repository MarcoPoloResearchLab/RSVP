package handlers

import (
	"github.com/temirov/RSVP/pkg/config"
)

// IsTestingContext checks if the current context is a testing context
func IsTestingContext(applicationContext *config.ApplicationContext) bool {
	// In a testing context, AuthService will be nil
	return applicationContext.AuthService == nil
}
