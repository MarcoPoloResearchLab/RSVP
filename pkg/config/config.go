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

const (
	WebRoot   = "/"
	WebEvents = "/events/"
	WebRSVPs  = "/rsvps/"
)

const (
	TemplateEvents   = "events.html"
	TemplateRSVPs    = "rsvps.html"
	TemplateQR       = "qr.html"
	TemplateResponse = "response.html"
	TemplateThankYou = "thankyou.html"
)

// Parameter name constants for consistent use throughout the application.
const (
	EventIDParam = "event_id"
	RSVPIDParam  = "rsvp_id"
	UserIDParam  = "user_id"

	NameParam        = "name"
	TitleParam       = "title"
	DescriptionParam = "description"
	StartTimeParam   = "start_time"
	DurationParam    = "duration"
	ResponseParam    = "response"
	GuestsParam      = "guests"
	CodeParam        = "code"

	MethodOverrideParam = "_method"
)

const (
	DefaultDBName = "rsvps.db"
)

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
