package utils

import (
	"log"
	"os"
)

// NewLogger creates a new logger that writes to stdout.
func NewLogger() *log.Logger {
	return log.New(os.Stdout, "[APP] ", log.LstdFlags)
}
