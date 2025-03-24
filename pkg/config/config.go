// Package config provides environment-based configuration and application context.
package config

import (
	"html/template"
	"log"
	"os"

	"gorm.io/gorm"
)

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Name string
}

// Common web paths.
const (
	WebRoot             = "/"
	WebEvents           = "/events/"
	WebRSVPs            = "/rsvps/"
	WebResponse         = "/response/"
	WebResponseThankYou = "/response/thankyou"
)

// Template constants.
const (
	TemplateEvents   = "events.html"
	TemplateRSVP     = "rsvp.html"
	TemplateRSVPs    = "rsvps.html"
	TemplateResponse = "response.html"
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

// DefaultDBName is the default name for the database.
const DefaultDBName = "rsvps.db"

// ApplicationContext holds the shared context for the application.
type ApplicationContext struct {
	Database  *gorm.DB
	Templates *template.Template
	Logger    *log.Logger
}

// EnvConfig holds environment configuration.
type EnvConfig struct {
	SessionSecret       string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleOauth2Base    string
	CertificateFilePath string
	KeyFilePath         string
	Database            DatabaseConfig
}

// NewEnvConfig creates a new environment configuration based on environment variables.
func NewEnvConfig(appLogger *log.Logger) *EnvConfig {
	dbName := DefaultDBName
	if envDB := os.Getenv("DB_NAME"); envDB != "" {
		dbName = envDB
	}

	cfg := &EnvConfig{
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

	required := map[string]string{
		"SESSION_SECRET":       cfg.SessionSecret,
		"GOOGLE_CLIENT_ID":     cfg.GoogleClientID,
		"GOOGLE_CLIENT_SECRET": cfg.GoogleClientSecret,
		"GOOGLE_OAUTH2_BASE":   cfg.GoogleOauth2Base,
	}
	for envVar, val := range required {
		if val == "" {
			appLogger.Fatalf(envVar + " is not set")
		}
	}
	return cfg
}
