package config

import (
	"html/template"
	"log"
	"os"

	"github.com/temirov/GAuss/pkg/gauss"
	"gorm.io/gorm"
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Name string // Database file name
}

const (
	WebRoot   = "/"
	WebEvents = "/events/"
	WebRSVPs  = "/rsvps/"
)

const (
	TemplateIndex     = "index.html"
	TemplateResponses = "responses.html"
	TemplateGenerate  = "generate.html"
	TemplateThankYou  = "thankyou.html"
	TemplateRSVP      = "rsvp.html"
	EventsList        = "event_index.html"
)

// ApplicationContext holds shared resources for the application.
type ApplicationContext struct {
	Database    *gorm.DB
	Templates   *template.Template
	Logger      *log.Logger
	AuthService *gauss.Service
}

// EnvConfig holds all environment variable configurations
type EnvConfig struct {
	SessionSecret       string
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleOauth2Base    string
	CertificateFilePath string
	KeyFilePath         string
	Database            DatabaseConfig
}

// NewEnvConfig creates and validates a new EnvConfig instance
func NewEnvConfig(logger *log.Logger) *EnvConfig { // Assuming applicationLogger is of type *Logger
	// Default database name
	dbName := "rsvps.db"
	
	// Override with environment variable if provided
	if envDBName := os.Getenv("DB_NAME"); envDBName != "" {
		dbName = envDBName
	}
	
	config := &EnvConfig{
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
		"SESSION_SECRET":       config.SessionSecret,
		"GOOGLE_CLIENT_ID":     config.GoogleClientID,
		"GOOGLE_CLIENT_SECRET": config.GoogleClientSecret,
		"GOOGLE_OAUTH2_BASE":   config.GoogleOauth2Base,
	}

	for envVar, value := range requiredFields {
		if value == "" {
			logger.Fatalf(envVar + " is not set")
		}
	}

	return config
}
