// Package services provides core functionalities for external system interactions.
package services

import (
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"os"
	"path/filepath"
)

// InitDatabase initializes and returns the database connection.
// It ensures that the database file and its directory exist, conditionally performs a migration on venue records
// with missing user_id values by setting user_id from an associated event if available or by associating the venue with the first user in the system,
// and runs auto-migrations.
func InitDatabase(databaseFileName string, applicationLogger *log.Logger) *gorm.DB {
	databaseDirectoryName := filepath.Dir(databaseFileName)
	if databaseDirectoryName != "." && databaseDirectoryName != "" {
		directoryCreationError := os.MkdirAll(databaseDirectoryName, 0755)
		if directoryCreationError != nil {
			applicationLogger.Printf("Database directory creation failed for %s: %v", databaseDirectoryName, directoryCreationError)
		}
	}
	_, fileStatError := os.Stat(databaseFileName)
	if os.IsNotExist(fileStatError) {
		applicationLogger.Printf("Database file %s does not exist and will be created", databaseFileName)
		fileHandle, fileCreationError := os.Create(databaseFileName)
		if fileCreationError != nil {
			applicationLogger.Printf("Database file creation failed for %s: %v", databaseFileName, fileCreationError)
		} else {
			fileHandle.Close()
		}
	}
	databaseConnection, connectionError := gorm.Open(sqlite.Open(databaseFileName), &gorm.Config{})
	if connectionError != nil {
		applicationLogger.Fatalf("Failed to connect to database %s: %v", databaseFileName, connectionError)
	}
	applicationLogger.Printf("Database connection established to %s", databaseFileName)
	migrationError := performConditionalVenueUserIdMigration(databaseConnection, applicationLogger)
	if migrationError != nil {
		applicationLogger.Fatalf("Conditional venue user_id migration failed: %v", migrationError)
	}
	applicationLogger.Println("Running database auto-migrations...")
	autoMigrationError := databaseConnection.AutoMigrate(
		&models.User{},
		&models.Venue{},
		&models.Event{},
		&models.RSVP{},
	)
	if autoMigrationError != nil {
		applicationLogger.Fatalf("Failed to migrate database: %v", autoMigrationError)
	}
	applicationLogger.Println("Database migrations completed successfully.")
	return databaseConnection
}

// performConditionalVenueUserIdMigration checks and updates venue records with missing user_id values.
// For venues associated with an event, it sets user_id to the user_id from the associated event.
// For venues not associated with any event, it retrieves the first user in the system and sets user_id accordingly.
// The migration is executed only if missing user_id records exist.
func performConditionalVenueUserIdMigration(databaseConnection *gorm.DB, applicationLogger *log.Logger) error {
	var recoverableMissingCount int64
	recoverableQuery := "SELECT COUNT(*) FROM " + config.TableVenues + " WHERE (user_id IS NULL OR user_id = '') AND id IN (SELECT venue_id FROM " + config.TableEvents + " WHERE venue_id IS NOT NULL)"
	recoverableQueryResult := databaseConnection.Raw(recoverableQuery).Scan(&recoverableMissingCount)
	if recoverableQueryResult.Error != nil {
		applicationLogger.Printf("Failed to check recoverable missing user_id condition: %v", recoverableQueryResult.Error)
		return recoverableQueryResult.Error
	}
	var nonRecoverableMissingCount int64
	nonRecoverableQuery := "SELECT COUNT(*) FROM " + config.TableVenues + " WHERE (user_id IS NULL OR user_id = '') AND id NOT IN (SELECT venue_id FROM " + config.TableEvents + " WHERE venue_id IS NOT NULL)"
	nonRecoverableQueryResult := databaseConnection.Raw(nonRecoverableQuery).Scan(&nonRecoverableMissingCount)
	if nonRecoverableQueryResult.Error != nil {
		applicationLogger.Printf("Failed to check non-recoverable missing user_id condition: %v", nonRecoverableQueryResult.Error)
		return nonRecoverableQueryResult.Error
	}
	if recoverableMissingCount > 0 {
		updateRecoverableSQL := "UPDATE " + config.TableVenues + " SET user_id = (SELECT user_id FROM " + config.TableEvents + " WHERE " + config.TableEvents + ".venue_id = " + config.TableVenues + ".id LIMIT 1) WHERE (user_id IS NULL OR user_id = '') AND id IN (SELECT venue_id FROM " + config.TableEvents + " WHERE venue_id IS NOT NULL)"
		recoverableUpdateResult := databaseConnection.Exec(updateRecoverableSQL)
		if recoverableUpdateResult.Error != nil {
			applicationLogger.Printf("Failed to update recoverable venue records: %v", recoverableUpdateResult.Error)
			return recoverableUpdateResult.Error
		}
		applicationLogger.Printf("Updated %d recoverable venue record(s) with missing user_id", recoverableUpdateResult.RowsAffected)
	}
	if nonRecoverableMissingCount > 0 {
		var firstUser models.User
		firstUserQueryResult := databaseConnection.First(&firstUser)
		if firstUserQueryResult.Error != nil {
			applicationLogger.Printf("Failed to retrieve first user for non-recoverable venue update: %v", firstUserQueryResult.Error)
			return firstUserQueryResult.Error
		}
		updateNonRecoverableSQL := "UPDATE " + config.TableVenues + " SET user_id = ? WHERE (user_id IS NULL OR user_id = '') AND id NOT IN (SELECT venue_id FROM " + config.TableEvents + " WHERE venue_id IS NOT NULL)"
		nonRecoverableUpdateResult := databaseConnection.Exec(updateNonRecoverableSQL, firstUser.ID)
		if nonRecoverableUpdateResult.Error != nil {
			applicationLogger.Printf("Failed to update non-recoverable venue records: %v", nonRecoverableUpdateResult.Error)
			return nonRecoverableUpdateResult.Error
		}
		applicationLogger.Printf("Updated %d non-recoverable venue record(s) with missing user_id using first user ID", nonRecoverableUpdateResult.RowsAffected)
	}
	return nil
}
