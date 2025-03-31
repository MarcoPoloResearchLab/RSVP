package utils

import (
	"log"
	"os"

	"github.com/temirov/RSVP/pkg/config" // Import config for constant
)

// NewLogger creates and returns a new standard Go logger instance configured
// to write to standard output (os.Stdout) with a specific prefix and standard log flags
// (date and time).
func NewLogger() *log.Logger {
	// Create a logger writing to Stdout, with the app prefix, and standard flags.
	return log.New(os.Stdout, config.LogPrefixApp, log.LstdFlags)
}
