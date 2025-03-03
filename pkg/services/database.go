package services

import (
	"log"

	"github.com/temirov/RSVP/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// InitDatabase opens the SQLite database and auto-migrates models.
func InitDatabase(databaseName string, applicationLogger *log.Logger) *gorm.DB {
	databaseConnection, errorValue := gorm.Open(sqlite.Open(databaseName), &gorm.Config{})
	if errorValue != nil {
		applicationLogger.Fatal("Failed to connect to database:", errorValue)
	}
	if errorValue = databaseConnection.AutoMigrate(
		&models.User{},
		&models.Event{},
		&models.RSVP{},
	); errorValue != nil {
		applicationLogger.Fatal("Failed to migrate database:", errorValue)
	}
	return databaseConnection
}
