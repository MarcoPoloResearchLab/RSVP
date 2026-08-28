package services_test

import (
	"bytes"
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

func TestDatabaseInitializationMigratesPredecessorCalendarConnectionSchema(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "predecessor.db")
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
	connection, connectionError := models.NewCalendarConnection(owner.ID, bytes.Repeat([]byte{0x11}, 12), []byte{0x22})
	if connectionError != nil {
		testingContext.Fatalf("construct calendar connection: %v", connectionError)
	}
	if createError := databaseConnection.Create(connection).Error; createError != nil {
		testingContext.Fatalf("create calendar connection: %v", createError)
	}
	calendar, calendarError := models.NewCalendar(owner.ID, "Existing", "E", "existing", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct existing calendar: %v", calendarError)
	}
	if createError := databaseConnection.Create(calendar).Error; createError != nil {
		testingContext.Fatalf("create existing calendar: %v", createError)
	}
	mapping, mappingError := models.NewSourceCalendarMapping(connection.ID, calendar.ID, "source", models.SourceCalendarGroupCalendar)
	if mappingError != nil {
		testingContext.Fatalf("construct existing source mapping: %v", mappingError)
	}
	eventCursor := "event-cursor"
	mapping.SyncCursor = &eventCursor
	if createError := databaseConnection.Create(mapping).Error; createError != nil {
		testingContext.Fatalf("create existing source mapping: %v", createError)
	}
	sqlDatabase, databaseError := databaseConnection.DB()
	if databaseError != nil {
		testingContext.Fatalf("get canonical database handle: %v", databaseError)
	}
	sqlDatabase.SetMaxOpenConns(1)
	predecessorStatements := []string{
		"PRAGMA foreign_keys = OFF",
		"CREATE TABLE source_calendar_mappings_predecessor (id varchar(8), created_at datetime, updated_at datetime, deleted_at datetime, connection_id varchar(8) NOT NULL, calendar_id varchar(8) NOT NULL, provider_calendar_id text NOT NULL, sync_cursor text, PRIMARY KEY (id), CONSTRAINT fk_source_calendar_mappings_calendar FOREIGN KEY (calendar_id) REFERENCES calendars(id) ON DELETE RESTRICT ON UPDATE CASCADE, CONSTRAINT fk_calendar_connections_mappings FOREIGN KEY (connection_id) REFERENCES calendar_connections(id) ON DELETE CASCADE ON UPDATE CASCADE)",
		"INSERT INTO source_calendar_mappings_predecessor (id, created_at, updated_at, deleted_at, connection_id, calendar_id, provider_calendar_id, sync_cursor) SELECT id, created_at, updated_at, deleted_at, connection_id, calendar_id, provider_calendar_id, sync_cursor FROM source_calendar_mappings",
		"DROP TABLE source_calendar_mappings",
		"ALTER TABLE source_calendar_mappings_predecessor RENAME TO source_calendar_mappings",
		"CREATE UNIQUE INDEX idx_source_calendar_mappings_calendar_id ON source_calendar_mappings (calendar_id)",
		"CREATE UNIQUE INDEX source_provider_calendar ON source_calendar_mappings (connection_id, provider_calendar_id)",
		"ALTER TABLE calendar_connections DROP COLUMN calendar_import_cutover_at",
		"ALTER TABLE calendar_connections DROP COLUMN calendar_list_sync_cursor",
	}
	for _, statement := range predecessorStatements {
		if _, alterError := sqlDatabase.Exec(statement); alterError != nil {
			testingContext.Fatalf("construct predecessor schema with %q: %v", statement, alterError)
		}
	}
	if closeError := sqlDatabase.Close(); closeError != nil {
		testingContext.Fatalf("close predecessor database: %v", closeError)
	}

	migratedDatabase, migrationError := services.OpenDatabase(databasePath)
	if migrationError != nil {
		testingContext.Fatalf("migrate predecessor database: %v", migrationError)
	}
	migratedSQLDatabase, databaseError := migratedDatabase.DB()
	if databaseError != nil {
		testingContext.Fatalf("get migrated database handle: %v", databaseError)
	}
	testingContext.Cleanup(func() {
		if closeError := migratedSQLDatabase.Close(); closeError != nil {
			testingContext.Errorf("close migrated database: %v", closeError)
		}
	})
	if !migratedDatabase.Migrator().HasColumn(&models.CalendarConnection{}, "CalendarListSyncCursor") {
		testingContext.Fatal("migrated database does not contain CalendarList sync cursor")
	}
	if !migratedDatabase.Migrator().HasColumn(&models.CalendarConnection{}, "CalendarImportCutoverAt") ||
		!migratedDatabase.Migrator().HasColumn(&models.SourceCalendarMapping{}, "SemanticGroup") {
		testingContext.Fatal("migrated database does not contain the calendar grouping columns")
	}
	var migratedConnection models.CalendarConnection
	if findError := migratedDatabase.First(&migratedConnection, "id = ?", connection.ID).Error; findError != nil {
		testingContext.Fatalf("read preserved calendar connection: %v", findError)
	}
	if migratedConnection.OrganizerID != owner.ID || migratedConnection.CalendarListSyncCursor != nil {
		testingContext.Fatalf("migrated calendar connection = %#v", migratedConnection)
	}
	var migratedMapping models.SourceCalendarMapping
	if findError := migratedDatabase.First(&migratedMapping, "id = ?", mapping.ID).Error; findError != nil {
		testingContext.Fatalf("read preserved source mapping: %v", findError)
	}
	if migratedMapping.CalendarID != calendar.ID || migratedMapping.SemanticGroup != models.SourceCalendarGroupCalendar || migratedMapping.SyncCursor != nil {
		testingContext.Fatalf("migrated source mapping = %#v", migratedMapping)
	}
}

func TestDatabaseInitializationMigratesProviderGroupToSemanticGroup(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "provider-group-predecessor.db")
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
	connection, connectionError := models.NewCalendarConnection(owner.ID, bytes.Repeat([]byte{0x11}, 12), []byte{0x22})
	if connectionError != nil {
		testingContext.Fatalf("construct calendar connection: %v", connectionError)
	}
	listCursor := "calendar-list-cursor"
	connection.CalendarListSyncCursor = &listCursor
	if createError := databaseConnection.Create(connection).Error; createError != nil {
		testingContext.Fatalf("create calendar connection: %v", createError)
	}
	calendar, calendarError := models.NewCalendar(owner.ID, "Existing", "E", "existing", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct existing calendar: %v", calendarError)
	}
	if createError := databaseConnection.Create(calendar).Error; createError != nil {
		testingContext.Fatalf("create existing calendar: %v", createError)
	}
	mapping, mappingError := models.NewSourceCalendarMapping(connection.ID, calendar.ID, "source", models.SourceCalendarGroupCalendar)
	if mappingError != nil {
		testingContext.Fatalf("construct source mapping: %v", mappingError)
	}
	eventCursor := "event-cursor"
	mapping.SyncCursor = &eventCursor
	if createError := databaseConnection.Create(mapping).Error; createError != nil {
		testingContext.Fatalf("create source mapping: %v", createError)
	}
	if renameError := databaseConnection.Exec("ALTER TABLE source_calendar_mappings RENAME COLUMN semantic_group TO provider_group").Error; renameError != nil {
		testingContext.Fatalf("construct provider group predecessor: %v", renameError)
	}
	sqlDatabase, databaseError := databaseConnection.DB()
	if databaseError != nil {
		testingContext.Fatalf("get predecessor database handle: %v", databaseError)
	}
	if closeError := sqlDatabase.Close(); closeError != nil {
		testingContext.Fatalf("close predecessor database: %v", closeError)
	}

	migratedDatabase, migrationError := services.OpenDatabase(databasePath)
	if migrationError != nil {
		testingContext.Fatalf("migrate provider group predecessor: %v", migrationError)
	}
	migratedSQLDatabase, databaseError := migratedDatabase.DB()
	if databaseError != nil {
		testingContext.Fatalf("get migrated database handle: %v", databaseError)
	}
	testingContext.Cleanup(func() {
		if closeError := migratedSQLDatabase.Close(); closeError != nil {
			testingContext.Errorf("close migrated database: %v", closeError)
		}
	})
	if !migratedDatabase.Migrator().HasColumn(&models.SourceCalendarMapping{}, "SemanticGroup") ||
		migratedDatabase.Migrator().HasColumn("source_calendar_mappings", "provider_group") {
		testingContext.Fatal("migrated database does not use the semantic group column")
	}
	var migratedConnection models.CalendarConnection
	if findError := migratedDatabase.First(&migratedConnection, "id = ?", connection.ID).Error; findError != nil {
		testingContext.Fatalf("read preserved calendar connection: %v", findError)
	}
	if migratedConnection.CalendarListSyncCursor == nil || *migratedConnection.CalendarListSyncCursor != listCursor {
		testingContext.Fatalf("migrated calendar list cursor = %#v", migratedConnection.CalendarListSyncCursor)
	}
	var migratedMapping models.SourceCalendarMapping
	if findError := migratedDatabase.First(&migratedMapping, "id = ?", mapping.ID).Error; findError != nil {
		testingContext.Fatalf("read preserved source mapping: %v", findError)
	}
	if migratedMapping.CalendarID != calendar.ID || migratedMapping.SemanticGroup != models.SourceCalendarGroupCalendar || migratedMapping.SyncCursor != nil {
		testingContext.Fatalf("migrated source mapping = %#v", migratedMapping)
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
