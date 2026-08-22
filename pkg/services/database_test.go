package services_test

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/services"
)

func TestDatabaseInitializationCreatesCanonicalTables(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "nested", "rsvp.db")
	databaseConnection := services.InitDatabase(databasePath, log.New(io.Discard, "", 0))

	sqlDatabase, databaseError := databaseConnection.DB()
	if databaseError != nil {
		testingContext.Fatalf("get initialized database handle: %v", databaseError)
	}
	testingContext.Cleanup(func() {
		if closeError := sqlDatabase.Close(); closeError != nil {
			testingContext.Errorf("close initialized database: %v", closeError)
		}
	})

	if _, statError := os.Stat(databasePath); statError != nil {
		testingContext.Fatalf("stat initialized database: %v", statError)
	}

	canonicalModels := []any{
		&models.User{},
		&models.Venue{},
		&models.Event{},
		&models.RSVP{},
	}
	for _, canonicalModel := range canonicalModels {
		if !databaseConnection.Migrator().HasTable(canonicalModel) {
			testingContext.Errorf("canonical table for %T is absent", canonicalModel)
		}
	}
}
