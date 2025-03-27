// File: pkg/config/config.go
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

// Common web paths.
const (
	WebRoot             = "/"
	WebEvents           = "/events/"
	WebRSVPs            = "/rsvps/"
	WebRSVPQR           = "/rsvps/qr/" // Added constant for QR code path
	WebResponse         = "/response/"
	WebResponseThankYou = "/response/thankyou"
)

// Template name constants (base names used for lookup in PrecompiledTemplatesMap).
const (
	TemplateEvents   = "events"
	TemplateRSVP     = "rsvp" // QR Code page view name
	TemplateRSVPs    = "rsvps"
	TemplateResponse = "response"
	TemplateThankYou = "thankyou"
)

// Parameter name constants for consistent use throughout the application.
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

// DefaultDBName is the default name for the database.
const DefaultDBName = "rsvps.db"

// ApplicationContext holds the shared context for the application.
type ApplicationContext struct {
	Database *gorm.DB
	// Templates field removed - PrecompiledTemplatesMap in templates pkg is used directly
	Logger *log.Logger
}

// EnvConfig holds environment configuration.
type EnvConfig struct {
	SessionSecret       string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleOauth2Base    string
	CertificateFilePath string
	KeyFilePath         string
	AppBaseURL          string // Added for constructing full public URLs
	Database            DatabaseConfig
}

// NewEnvConfig creates a new environment configuration based on environment variables.
func NewEnvConfig(appLogger *log.Logger) *EnvConfig {
	dbName := DefaultDBName
	if envDB := os.Getenv("DB_NAME"); envDB != "" {
		dbName = envDB
	}

	// Ensure APP_BASE_URL ends with a slash if set
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
		AppBaseURL:          appBaseURL, // Use the processed base URL
		Database: DatabaseConfig{
			Name: dbName,
		},
	}

	required := map[string]string{
		"SESSION_SECRET":       cfg.SessionSecret,
		"GOOGLE_CLIENT_ID":     cfg.GoogleClientID,
		"GOOGLE_CLIENT_SECRET": cfg.GoogleClientSecret,
		"GOOGLE_OAUTH2_BASE":   cfg.GoogleOauth2Base,
		"APP_BASE_URL":         cfg.AppBaseURL, // Make AppBaseURL required
	}
	for envVar, val := range required {
		if val == "" {
			appLogger.Fatalf(envVar + " environment variable is not set")
		}
	}
	return cfg
}
