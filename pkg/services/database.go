package services

import (
	"log"

	"github.com/temirov/RSVP/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// InitDatabase opens the SQLite database and auto-migrates models.
func InitDatabase(databaseName string, applicationLogger *log.Logger) *gorm.DB {
	databaseConnection, connectionError := gorm.Open(sqlite.Open(databaseName), &gorm.Config{})
	if connectionError != nil {
		applicationLogger.Fatal("Failed to connect to database:", connectionError)
	}
	if migrationError := databaseConnection.AutoMigrate(
		&models.User{},
		&models.Event{},
		&models.RSVP{},
	); migrationError != nil {
		applicationLogger.Fatal("Failed to migrate database:", migrationError)
	}
	return databaseConnection
}
