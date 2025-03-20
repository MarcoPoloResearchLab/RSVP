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
func InitDatabase(databaseName string, applicationLogger *log.Logger) *gorm.DB {
	databaseDirectory := filepath.Dir(databaseName)
	if databaseDirectory != "." && databaseDirectory != "" {
		makeDirectoryError := os.MkdirAll(databaseDirectory, 0o755)
		if makeDirectoryError != nil {
			applicationLogger.Printf("Warning: Failed to create database directory: %v", makeDirectoryError)
		}
	}

	_, statError := os.Stat(databaseName)
	if os.IsNotExist(statError) {
		applicationLogger.Printf("Database file %s does not exist, it will be created", databaseName)
		fileHandle, createFileError := os.Create(databaseName)
		if createFileError != nil {
			applicationLogger.Printf("Warning: Failed to create database file: %v", createFileError)
		} else {
			fileHandle.Close()
		}
	}

	databaseConnection, connectionError := gorm.Open(sqlite.Open(databaseName), &gorm.Config{})
	if connectionError != nil {
		applicationLogger.Fatal("Failed to connect to database:", connectionError)
	}

	migrationError := databaseConnection.AutoMigrate(
		&models.User{},
		&models.Event{},
		&models.RSVP{},
	)
	if migrationError != nil {
		applicationLogger.Fatal("Failed to migrate database:", migrationError)
	}

	return databaseConnection
}
