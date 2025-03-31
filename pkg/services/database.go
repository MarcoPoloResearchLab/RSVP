// Package services contains components that provide core functionalities or interact with external systems,
// such as database initialization.
package services

import (
	"log"
	"os"
	"path/filepath"

	"github.com/temirov/RSVP/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// InitDatabase initializes the connection to the SQLite database specified by databaseName.
// It ensures the directory for the database file exists, creates the file if it doesn't exist,
// establishes a GORM connection, and performs auto-migration for the application's models (User, Event, RSVP).
// It logs progress and fatally exits if critical steps like connection or migration fail.
// Returns the initialized *gorm.DB connection pool.
func InitDatabase(databaseName string, applicationLogger *log.Logger) *gorm.DB {
	// Ensure the directory containing the database file exists.
	databaseDirectory := filepath.Dir(databaseName)
	// Check if the directory path is non-trivial (not "." or empty).
	if databaseDirectory != "." && databaseDirectory != "" {
		// Create the directory and any necessary parent directories. 0o755 permissions are standard.
		makeDirectoryError := os.MkdirAll(databaseDirectory, 0o755)
		if makeDirectoryError != nil {
			// Log a warning if directory creation fails, but attempt to continue.
			applicationLogger.Printf("Warning: Failed to create database directory '%s': %v", databaseDirectory, makeDirectoryError)
		}
	}

	// Check if the database file already exists.
	_, statError := os.Stat(databaseName)
	if os.IsNotExist(statError) {
		// Database file does not exist, log and attempt to create it.
		applicationLogger.Printf("Database file %s does not exist, it will be created", databaseName)
		fileHandle, createFileError := os.Create(databaseName)
		if createFileError != nil {
			// Log a warning if file creation fails, but attempt to continue (gorm.Open might still work or provide a better error).
			applicationLogger.Printf("Warning: Failed to create database file '%s': %v", databaseName, createFileError)
		} else {
			// Close the file handle immediately after creation.
			fileHandle.Close()
		}
	}

	// Open the GORM connection to the SQLite database.
	databaseConnection, connectionError := gorm.Open(sqlite.Open(databaseName), &gorm.Config{})
	if connectionError != nil {
		// Log a fatal error and exit if the database connection fails.
		applicationLogger.Fatalf("FATAL: Failed to connect to database '%s': %v", databaseName, connectionError)
	}
	applicationLogger.Printf("Database connection established to %s", databaseName)

	// Perform automatic database migrations. GORM will create or update tables
	// based on the provided model structs (User, Event, RSVP).
	applicationLogger.Println("Running database auto-migrations...")
	migrationError := databaseConnection.AutoMigrate(
		&models.User{},
		&models.Event{},
		&models.RSVP{},
	)
	if migrationError != nil {
		// Log a fatal error and exit if migrations fail.
		applicationLogger.Fatalf("FATAL: Failed to migrate database: %v", migrationError)
	}
	applicationLogger.Println("Database migrations completed successfully.")

	// Return the established database connection.
	return databaseConnection
}
