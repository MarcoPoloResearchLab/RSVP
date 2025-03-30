// Package config holds application-wide configuration, constants, and context structures.
package config

import (
	"log"
	"os"

	"gorm.io/gorm"
)

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Name string
}

// Constants defining common web paths used throughout the application.
const (
	WebRoot             = "/"
	WebEvents           = "/events/"
	WebRSVPs            = "/rsvps/"
	WebRSVPQR           = "/rsvps/qr/"
	WebResponse         = "/response/"
	WebResponseThankYou = "/response/thankyou"
)

// Constants defining template base names used for looking up precompiled templates
// by the main template loader (excluding standalone pages like landing).
const (
	TemplateEvents    = "events"
	TemplateRSVP      = "rsvp"
	TemplateRSVPs     = "rsvps"
	TemplateResponse  = "response"
	TemplateThankYou  = "thankyou"
	TemplateExtension = ".tmpl"
)

// TemplatesDir is the directory where application templates are stored.
const TemplatesDir = "templates"

// Constants defining parameter names used in HTTP requests (query and form values).
const (
	EventIDParam        = "event_id"
	RSVPIDParam         = "rsvp_id"
	NameParam           = "name"
	TitleParam          = "title"
	DescriptionParam    = "description"
	StartTimeParam      = "start_time"
	DurationParam       = "duration"
	ResponseParam       = "response"
	MethodOverrideParam = "_method"
)

// DefaultDBName is the default filename for the SQLite database.
const DefaultDBName = "rsvps.db"

// ApplicationContext holds shared dependencies accessible across handlers.
type ApplicationContext struct {
	Database *gorm.DB
	Logger   *log.Logger
}

// EnvConfig holds configuration values sourced from environment variables.
type EnvConfig struct {
	SessionSecret       string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleOauth2Base    string
	CertificateFilePath string
	KeyFilePath         string
	AppBaseURL          string
	Database            DatabaseConfig
}

// NewEnvConfig creates a new EnvConfig instance, populating it with values
// from environment variables and applying default settings where necessary.
// It ensures required environment variables are set, logging a fatal error if not.
func NewEnvConfig(appLogger *log.Logger) *EnvConfig {
	dbName := DefaultDBName
	if envDB := os.Getenv("DB_NAME"); envDB != "" {
		dbName = envDB
	}

	appBaseURL := os.Getenv("APP_BASE_URL")
	if appBaseURL != "" && appBaseURL[len(appBaseURL)-1:] != "/" {
		appBaseURL += "/"
	}

	cfg := &EnvConfig{
		SessionSecret:       os.Getenv("SESSION_SECRET"),
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleOauth2Base:    os.Getenv("GOOGLE_OAUTH2_BASE"),
		CertificateFilePath: os.Getenv("TLS_CERT_PATH"),
		KeyFilePath:         os.Getenv("TLS_KEY_PATH"),
		AppBaseURL:          appBaseURL,
		Database:            DatabaseConfig{Name: dbName},
	}

	required := map[string]string{
		"SESSION_SECRET":       cfg.SessionSecret,
		"GOOGLE_CLIENT_ID":     cfg.GoogleClientID,
		"GOOGLE_CLIENT_SECRET": cfg.GoogleClientSecret,
		"GOOGLE_OAUTH2_BASE":   cfg.GoogleOauth2Base,
		"APP_BASE_URL":         cfg.AppBaseURL,
	}
	for envVar, val := range required {
		if val == "" {
			appLogger.Fatalf(envVar + " environment variable is not set")
		}
	}
	return cfg
}
