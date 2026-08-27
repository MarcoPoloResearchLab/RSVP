package services_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDatabaseInitializationCreatesCanonicalTables(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "nested", "rsvp.db")
	databaseConnection, initializationError := services.OpenDatabase(databasePath)
	if initializationError != nil {
		testingContext.Fatalf("initialize canonical database: %v", initializationError)
	}

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
		&models.Calendar{},
		&models.Lane{},
		&models.EventSeries{},
		&models.Venue{},
		&models.Event{},
		&models.RSVP{},
		&models.AttentionPolicy{},
		&models.Probe{},
	}
	for _, canonicalModel := range canonicalModels {
		if !databaseConnection.Migrator().HasTable(canonicalModel) {
			testingContext.Errorf("canonical table for %T is absent", canonicalModel)
		}
	}
}

func TestDatabaseInitializationRejectsEventOnlySchema(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "event-only.db")
	eventOnlyDatabase, openError := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if openError != nil {
		testingContext.Fatalf("open event-only database: %v", openError)
	}
	if schemaError := eventOnlyDatabase.Exec("CREATE TABLE users (id TEXT PRIMARY KEY); CREATE TABLE events (id TEXT PRIMARY KEY, user_id TEXT NOT NULL)").Error; schemaError != nil {
		testingContext.Fatalf("create event-only schema: %v", schemaError)
	}

	_, databaseError := services.OpenDatabase(databasePath)
	if !errors.Is(databaseError, services.ErrNonCanonicalDatabase) {
		testingContext.Fatalf("open event-only schema error = %v, want %v", databaseError, services.ErrNonCanonicalDatabase)
	}
}

func TestDatabaseInitializationRejectsUnknownOnlySchema(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "unknown-only.db")
	unknownDatabase, openError := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if openError != nil {
		testingContext.Fatalf("open unknown database: %v", openError)
	}
	if schemaError := unknownDatabase.Exec("CREATE TABLE imported_data (id TEXT PRIMARY KEY)").Error; schemaError != nil {
		testingContext.Fatalf("create unknown schema: %v", schemaError)
	}

	_, databaseError := services.OpenDatabase(databasePath)
	if !errors.Is(databaseError, services.ErrNonCanonicalDatabase) {
		testingContext.Fatalf("open unknown-only schema error = %v, want %v", databaseError, services.ErrNonCanonicalDatabase)
	}
	if unknownDatabase.Migrator().HasTable(&models.User{}) {
		testingContext.Fatal("unknown database was initialized with canonical tables")
	}
}

func TestDatabaseInitializationRejectsIncompleteCanonicalColumns(testingContext *testing.T) {
	for _, testCase := range []struct {
		name       string
		alterQuery string
	}{
		{name: "venue", alterQuery: "ALTER TABLE venues DROP COLUMN capacity"},
		{name: "rsvp", alterQuery: "ALTER TABLE rsvps DROP COLUMN extra_guests"},
	} {
		testingContext.Run(testCase.name, func(testingContext *testing.T) {
			databasePath := filepath.Join(testingContext.TempDir(), testCase.name+".db")
			databaseConnection, openError := services.OpenDatabase(databasePath)
			if openError != nil {
				testingContext.Fatalf("open canonical database: %v", openError)
			}
			if alterError := databaseConnection.Exec(testCase.alterQuery).Error; alterError != nil {
				testingContext.Fatalf("alter canonical schema: %v", alterError)
			}

			_, databaseError := services.OpenDatabase(databasePath)
			if !errors.Is(databaseError, services.ErrNonCanonicalDatabase) {
				testingContext.Fatalf("open incomplete schema error = %v, want %v", databaseError, services.ErrNonCanonicalDatabase)
			}
		})
	}
}

func TestDatabaseInitializationRejectsMissingCanonicalUniqueIndex(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "missing-index.db")
	databaseConnection, openError := services.OpenDatabase(databasePath)
	if openError != nil {
		testingContext.Fatalf("open canonical database: %v", openError)
	}
	if dropError := databaseConnection.Exec("DROP INDEX lane_calendar_order").Error; dropError != nil {
		testingContext.Fatalf("drop canonical index: %v", dropError)
	}

	_, databaseError := services.OpenDatabase(databasePath)
	if !errors.Is(databaseError, services.ErrNonCanonicalDatabase) {
		testingContext.Fatalf("open schema without unique index error = %v, want %v", databaseError, services.ErrNonCanonicalDatabase)
	}
}

func TestCanonicalSchemaEnforcesLaneAndEventConstraints(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "constraints.db")
	databaseConnection, openError := services.OpenDatabase(databasePath)
	if openError != nil {
		testingContext.Fatalf("open canonical database: %v", openError)
	}
	timezoneName := "America/Los_Angeles"
	owner := models.User{
		BaseModel: models.BaseModel{ID: "USR00001"},
		Email:     "owner@example.test",
		Timezone:  &timezoneName,
	}
	if createError := databaseConnection.Create(&owner).Error; createError != nil {
		testingContext.Fatalf("create owner: %v", createError)
	}
	calendar, calendarError := models.NewCalendar(owner.ID, "Calendar", "calendar", "test", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct calendar: %v", calendarError)
	}
	calendar.BaseModel.ID = "CAL00001"
	if createError := databaseConnection.Create(calendar).Error; createError != nil {
		testingContext.Fatalf("create calendar: %v", createError)
	}

	start := "2030-01-02 15:00:00+00:00"
	invalidLaneError := databaseConnection.Exec(
		"INSERT INTO lanes (id, calendar_id, title, status, starts_at, ends_at, display_order) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"LAN00001", calendar.ID, "Invalid lane", models.LaneStatusActive, start, start, 0,
	).Error
	if invalidLaneError == nil {
		testingContext.Fatal("invalid finite lane insert succeeded")
	}

	missingLaneError := databaseConnection.Exec(
		"INSERT INTO events (id, relation_type, time_shape, starts_at, ends_at, timezone, title) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"EVT00001", models.EventRelationIndependent, models.EventTimeInterval, start, "2030-01-02 16:00:00+00:00", timezoneName, "Missing lane",
	).Error
	if missingLaneError == nil {
		testingContext.Fatal("event without lane insert succeeded")
	}
	if databaseConnection.Migrator().HasColumn("events", "user_id") {
		testingContext.Fatal("events table contains obsolete user_id column")
	}
}
