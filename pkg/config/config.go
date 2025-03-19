// Package config provides environment-based configuration and application context.
package config

import (
	"html/template"
	"log"
	"os"

	"github.com/temirov/GAuss/pkg/gauss"
	"gorm.io/gorm"
)

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Name string // Database file name
}

// Common web paths.
const (
	// WebRoot is the root path for the application.
	WebRoot = "/"

	// WebEvents is the base path for event routes.
	WebEvents = "/events/"

	// WebRSVPs is the base path for RSVP routes.
	WebRSVPs = "/rsvps/"
)

// Template constants.
const (
	// TemplateEvents is the filename for events page template.
	TemplateEvents = "events.html"

	// TemplateRSVPs is the filename for RSVPs page template.
	TemplateRSVPs = "rsvps.html"

	// TemplateQR is the filename for QR code visualization template.
	TemplateQR = "qr.html"

	// TemplateResponse is the filename for the RSVP response template.
	TemplateResponse = "response.html"

	// TemplateThankYou is the filename for the RSVP thank you template.
	TemplateThankYou = "thankyou.html"
)

// Parameter name constants for consistent use throughout the application.
const (
	EventIDParam        = "event_id"
	RSVPIDParam         = "rsvp_id"
	UserIDParam         = "user_id"
	NameParam           = "name"
	TitleParam          = "title"
	DescriptionParam    = "description"
	StartTimeParam      = "start_time"
	DurationParam       = "duration"
	ResponseParam       = "response"
	GuestsParam         = "guests"
	CodeParam           = "code"
	MethodOverrideParam = "_method"
)

// DefaultDBName is the default SQLite file if none is set.
const DefaultDBName = "rsvps.db"

// ApplicationContext holds shared resources for the application.
type ApplicationContext struct {
	Database    *gorm.DB
	Templates   *template.Template
	Logger      *log.Logger
	AuthService *gauss.Service
}

// EnvConfig holds all environment variable configurations.
type EnvConfig struct {
	SessionSecret       string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleOauth2Base    string
	CertificateFilePath string
	KeyFilePath         string
	Database            DatabaseConfig
}

// NewEnvConfig creates and validates a new EnvConfig instance.
func NewEnvConfig(applicationLogger *log.Logger) *EnvConfig {
	dbName := DefaultDBName

	if envDBName := os.Getenv("DB_NAME"); envDBName != "" {
		dbName = envDBName
	}

	configuration := &EnvConfig{
		SessionSecret:       os.Getenv("SESSION_SECRET"),
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleOauth2Base:    os.Getenv("GOOGLE_OAUTH2_BASE"),
		CertificateFilePath: os.Getenv("TLS_CERT_PATH"),
		KeyFilePath:         os.Getenv("TLS_KEY_PATH"),
		Database: DatabaseConfig{
			Name: dbName,
		},
	}

	// Validate required fields
	requiredFields := map[string]string{
		"SESSION_SECRET":       configuration.SessionSecret,
		"GOOGLE_CLIENT_ID":     configuration.GoogleClientID,
		"GOOGLE_CLIENT_SECRET": configuration.GoogleClientSecret,
		"GOOGLE_OAUTH2_BASE":   configuration.GoogleOauth2Base,
	}

	for environmentVariable, value := range requiredFields {
		if value == "" {
			applicationLogger.Fatalf(environmentVariable + " is not set")
		}
	}

	return configuration
}
