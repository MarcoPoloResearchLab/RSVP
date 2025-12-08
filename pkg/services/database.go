// Package services provides core functionalities for external system interactions.
package services

import (
	"log"
	"os"
	"path/filepath"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// InitDatabase establishes connection, runs migrations, and performs conditional data fixes.
func InitDatabase(databaseFileName string, applicationLogger *log.Logger) *gorm.DB {
	databaseDirectoryName := filepath.Dir(databaseFileName)
	if databaseDirectoryName != "." && databaseDirectoryName != "" {
		_ = os.MkdirAll(databaseDirectoryName, 0755)
	}
	if _, err := os.Stat(databaseFileName); os.IsNotExist(err) {
		if f, createErr := os.Create(databaseFileName); createErr == nil {
			_ = f.Close()
		}
	}

	databaseConnection, connectionError := gorm.Open(sqlite.Open(databaseFileName), &gorm.Config{})
	if connectionError != nil {
		applicationLogger.Fatalf("Failed to connect to database %s: %v", databaseFileName, connectionError)
	}
	applicationLogger.Printf("Database connection established to %s", databaseFileName)

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

	migrationError := performConditionalVenueUserIdMigration(databaseConnection, applicationLogger)
	if migrationError != nil {
		applicationLogger.Fatalf("Conditional venue user_id migration failed: %v", migrationError)
	}

	return databaseConnection
}

// performConditionalVenueUserIdMigration updates legacy venue records lacking user_id.
func performConditionalVenueUserIdMigration(databaseConnection *gorm.DB, applicationLogger *log.Logger) error {
	var recoverableMissingCount int64
	recoverableQuery := "SELECT COUNT(*) FROM " + config.TableVenues + " WHERE (user_id IS NULL OR user_id = '') AND id IN (SELECT venue_id FROM " + config.TableEvents + " WHERE venue_id IS NOT NULL)"
	if err := databaseConnection.Raw(recoverableQuery).Scan(&recoverableMissingCount).Error; err != nil {
		applicationLogger.Printf("Failed to check recoverable missing user_id condition: %v", err)
		return err
	}

	var nonRecoverableMissingCount int64
	nonRecoverableQuery := "SELECT COUNT(*) FROM " + config.TableVenues + " WHERE (user_id IS NULL OR user_id = '') AND id NOT IN (SELECT venue_id FROM " + config.TableEvents + " WHERE venue_id IS NOT NULL)"
	if err := databaseConnection.Raw(nonRecoverableQuery).Scan(&nonRecoverableMissingCount).Error; err != nil {
		applicationLogger.Printf("Failed to check non-recoverable missing user_id condition: %v", err)
		return err
	}

	if recoverableMissingCount > 0 {
		updateRecoverableSQL := "UPDATE " + config.TableVenues + " SET user_id = (SELECT user_id FROM " + config.TableEvents + " WHERE " + config.TableEvents + ".venue_id = " + config.TableVenues + ".id LIMIT 1) WHERE (user_id IS NULL OR user_id = '') AND id IN (SELECT venue_id FROM " + config.TableEvents + " WHERE venue_id IS NOT NULL)"
		if res := databaseConnection.Exec(updateRecoverableSQL); res.Error != nil {
			applicationLogger.Printf("Failed to update recoverable venue records: %v", res.Error)
			return res.Error
		}
	}

	if nonRecoverableMissingCount > 0 {
		var firstUser models.User
		if err := databaseConnection.First(&firstUser).Error; err != nil {
			applicationLogger.Printf("Failed to retrieve first user for non-recoverable venue update: %v", err)
			return err
		}
		updateNonRecoverableSQL := "UPDATE " + config.TableVenues + " SET user_id = ? WHERE (user_id IS NULL OR user_id = '') AND id NOT IN (SELECT venue_id FROM " + config.TableEvents + " WHERE venue_id IS NOT NULL)"
		if res := databaseConnection.Exec(updateNonRecoverableSQL, firstUser.ID); res.Error != nil {
			applicationLogger.Printf("Failed to update non-recoverable venue records: %v", res.Error)
			return res.Error
		}
	}
	return nil
}
