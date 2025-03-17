package services

import (
	"log"
	"os"
	"path/filepath"

	"github.com/temirov/RSVP/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// InitDatabase opens the SQLite database and auto-migrates models.
// It ensures the database file exists and is properly initialized.
func InitDatabase(databaseName string, applicationLogger *log.Logger) *gorm.DB {
	// Ensure the directory exists
	dbDir := filepath.Dir(databaseName)
	if dbDir != "." && dbDir != "" {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			applicationLogger.Printf("Warning: Failed to create database directory: %v", err)
		}
	}

	// Check if the database file exists
	_, err := os.Stat(databaseName)
	if os.IsNotExist(err) {
		applicationLogger.Printf("Database file %s does not exist, it will be created", databaseName)
		// Create an empty file to ensure permissions are correct
		file, err := os.Create(databaseName)
		if err != nil {
			applicationLogger.Printf("Warning: Failed to create database file: %v", err)
		} else {
			file.Close()
		}
	}

	// Open the database connection
	databaseConnection, connectionError := gorm.Open(sqlite.Open(databaseName), &gorm.Config{})
	if connectionError != nil {
		applicationLogger.Fatal("Failed to connect to database:", connectionError)
	}

	// Auto-migrate the models
	if migrationError := databaseConnection.AutoMigrate(
		&models.User{},
		&models.Event{},
		&models.RSVP{},
	); migrationError != nil {
		applicationLogger.Fatal("Failed to migrate database:", migrationError)
	}

	return databaseConnection
}
