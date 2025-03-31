// Package config holds application-wide configuration, constants, and context structures.
package config

import (
	"log"
	"os"
	"strings" // Import strings package

	"gorm.io/gorm"
)

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	// Name specifies the filename or connection string for the database.
	Name string
}

// ApplicationContext holds shared dependencies accessible across handlers.
type ApplicationContext struct {
	// Database is the active GORM database connection instance.
	Database *gorm.DB
	// Logger is the application-wide logger instance.
	Logger *log.Logger
	// AppBaseURL is the public base URL of the application, including trailing slash.
	AppBaseURL string // Added to centralize access
}

// EnvConfig holds configuration values sourced from environment variables.
type EnvConfig struct {
	// SessionSecret is the secret key used for securing user sessions.
	SessionSecret string
	// GoogleClientID is the Client ID obtained from Google Cloud Console for OAuth.
	GoogleClientID string
	// GoogleClientSecret is the Client Secret obtained from Google Cloud Console for OAuth.
	GoogleClientSecret string
	// GoogleOauth2Base is the base URL for Google OAuth2 endpoints.
	GoogleOauth2Base string
	// CertificateFilePath is the path to the TLS certificate file (optional).
	CertificateFilePath string
	// KeyFilePath is the path to the TLS private key file (optional).
	KeyFilePath string
	// AppBaseURL is the public base URL of the application (e.g., "https://example.com/"). Must include trailing slash.
	AppBaseURL string
	// Database contains database-specific configuration.
	Database DatabaseConfig
}

// NewEnvConfig creates a new EnvConfig instance, populating it with values
// from environment variables and applying default settings where necessary.
// It ensures required environment variables are set, logging a fatal error if not.
func NewEnvConfig(applicationLogger *log.Logger) *EnvConfig {
	databaseName := DefaultDBName
	if envDatabaseName := os.Getenv("DB_NAME"); envDatabaseName != "" {
		databaseName = envDatabaseName
	}

	// Ensure APP_BASE_URL ends with a slash if set
	appBaseURL := os.Getenv("APP_BASE_URL")
	if appBaseURL != "" && !strings.HasSuffix(appBaseURL, "/") {
		appBaseURL += "/"
	}

	envConfigData := &EnvConfig{
		SessionSecret:       os.Getenv("SESSION_SECRET"),
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleOauth2Base:    os.Getenv("GOOGLE_OAUTH2_BASE"),
		CertificateFilePath: os.Getenv("TLS_CERT_PATH"),
		KeyFilePath:         os.Getenv("TLS_KEY_PATH"),
		AppBaseURL:          appBaseURL, // Use the processed base URL
		Database: DatabaseConfig{
			Name: databaseName,
		},
	}

	// Define required environment variables and their corresponding values from the config struct.
	requiredEnvVars := map[string]string{
		"SESSION_SECRET":       envConfigData.SessionSecret,
		"GOOGLE_CLIENT_ID":     envConfigData.GoogleClientID,
		"GOOGLE_CLIENT_SECRET": envConfigData.GoogleClientSecret,
		"GOOGLE_OAUTH2_BASE":   envConfigData.GoogleOauth2Base,
		"APP_BASE_URL":         envConfigData.AppBaseURL, // Make AppBaseURL required
	}

	// Check if all required environment variables are set.
	for envVarName, value := range requiredEnvVars {
		if value == "" {
			// Log a fatal error and exit if a required variable is missing.
			applicationLogger.Fatalf("%s environment variable is not set", envVarName)
		}
	}
	return envConfigData
}
