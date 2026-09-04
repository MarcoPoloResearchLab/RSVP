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
		&models.ProviderCalendarSyncState{},
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

func TestDatabaseInitializationRemovesCalendarSymbolColumn(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "calendar-symbol.db")
	databaseConnection, openError := services.OpenDatabase(databasePath)
	if openError != nil {
		testingContext.Fatalf("open canonical database: %v", openError)
	}
	timezoneName := "America/Los_Angeles"
	owner := models.User{BaseModel: models.BaseModel{ID: "USR00001"}, Email: "owner@example.test", Timezone: &timezoneName}
	if createError := databaseConnection.Create(&owner).Error; createError != nil {
		testingContext.Fatalf("create owner: %v", createError)
	}
	calendar, calendarError := models.NewCalendar(owner.ID, "Personal", "personal", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct calendar: %v", calendarError)
	}
	if createError := databaseConnection.Create(calendar).Error; createError != nil {
		testingContext.Fatalf("create calendar: %v", createError)
	}
	if alterError := databaseConnection.Exec("ALTER TABLE calendars ADD COLUMN symbol text NOT NULL DEFAULT 'P'").Error; alterError != nil {
		testingContext.Fatalf("add predecessor calendar symbol: %v", alterError)
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
		testingContext.Fatalf("migrate calendar symbol contract: %v", migrationError)
	}
	if migratedDatabase.Migrator().HasColumn("calendars", "symbol") {
		testingContext.Fatal("canonical calendar table contains obsolete symbol column")
	}
	var migratedCalendar models.Calendar
	if findError := migratedDatabase.First(&migratedCalendar, "id = ?", calendar.ID).Error; findError != nil {
		testingContext.Fatalf("read migrated calendar: %v", findError)
	}
	if migratedCalendar.Name != "Personal" || migratedCalendar.ColorToken != "personal" {
		testingContext.Fatalf("migrated calendar = %#v", migratedCalendar)
	}
}

func TestDatabaseInitializationMigratesTaskPredecessor(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "task-predecessor.db")
	databaseConnection, openError := services.OpenDatabase(databasePath)
	if openError != nil {
		testingContext.Fatalf("open canonical database: %v", openError)
	}
	owner := models.User{BaseModel: models.BaseModel{ID: "USR00001"}, Email: "owner@example.test"}
	if createError := databaseConnection.Create(&owner).Error; createError != nil {
		testingContext.Fatalf("create task predecessor owner: %v", createError)
	}
	connection, connectionError := models.NewCalendarConnection(owner.ID, bytes.Repeat([]byte{0x11}, 12), []byte{0x22})
	if connectionError != nil {
		testingContext.Fatalf("construct task predecessor connection: %v", connectionError)
	}
	if createError := databaseConnection.Create(connection).Error; createError != nil {
		testingContext.Fatalf("create task predecessor connection: %v", createError)
	}
	if alterError := databaseConnection.Exec("ALTER TABLE calendars ADD COLUMN symbol text NOT NULL DEFAULT 'P'").Error; alterError != nil {
		testingContext.Fatalf("add task predecessor calendar symbol: %v", alterError)
	}
	if dropError := databaseConnection.Migrator().DropTable(&models.Task{}); dropError != nil {
		testingContext.Fatalf("construct task predecessor: %v", dropError)
	}
	sqlDatabase, databaseError := databaseConnection.DB()
	if databaseError != nil {
		testingContext.Fatalf("get task predecessor database handle: %v", databaseError)
	}
	if closeError := sqlDatabase.Close(); closeError != nil {
		testingContext.Fatalf("close task predecessor database: %v", closeError)
	}

	migratedDatabase, migrationError := services.OpenDatabase(databasePath)
	if migrationError != nil {
		testingContext.Fatalf("migrate task predecessor: %v", migrationError)
	}
	migratedSQLDatabase, databaseError := migratedDatabase.DB()
	if databaseError != nil {
		testingContext.Fatalf("get migrated task database handle: %v", databaseError)
	}
	testingContext.Cleanup(func() {
		if closeError := migratedSQLDatabase.Close(); closeError != nil {
			testingContext.Errorf("close migrated task database: %v", closeError)
		}
	})
	if !migratedDatabase.Migrator().HasTable(&models.Task{}) {
		testingContext.Fatal("canonical task table is absent after migration")
	}
	if migratedDatabase.Migrator().HasColumn("calendars", "symbol") {
		testingContext.Fatal("canonical calendar table contains obsolete symbol column after task migration")
	}
	var migratedTask models.Task
	if findError := migratedDatabase.First(&migratedTask, "resource_type = ? AND resource_id = ?", models.TaskResourceCalendarConnection, connection.ID).Error; findError != nil {
		testingContext.Fatalf("read completed predecessor task: %v", findError)
	}
	if migratedTask.State != models.TaskSucceeded || migratedTask.RetryCount != 1 {
		testingContext.Fatalf("completed predecessor task = %#v", migratedTask)
	}
}

func TestDatabaseInitializationMigratesMasterSchema(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "provider-calendar-predecessor.db")
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
	calendar, calendarError := models.NewCalendar(owner.ID, "Existing", "existing", 0)
	if calendarError != nil {
		testingContext.Fatalf("construct existing calendar: %v", calendarError)
	}
	if createError := databaseConnection.Create(calendar).Error; createError != nil {
		testingContext.Fatalf("create existing calendar: %v", createError)
	}
	syncState, stateError := models.NewProviderCalendarSyncState(connection.ID, "source")
	if stateError != nil {
		testingContext.Fatalf("construct provider calendar state: %v", stateError)
	}
	eventCursor := "event-cursor"
	syncState.SyncCursor = &eventCursor
	if createError := databaseConnection.Create(syncState).Error; createError != nil {
		testingContext.Fatalf("create provider calendar state: %v", createError)
	}
	mapping, mappingError := models.NewSourceCalendarMapping(syncState.ID, calendar.ID, models.SourceCalendarGroupCalendar)
	if mappingError != nil {
		testingContext.Fatalf("construct source mapping: %v", mappingError)
	}
	if createError := databaseConnection.Create(mapping).Error; createError != nil {
		testingContext.Fatalf("create source mapping: %v", createError)
	}
	sqlDatabase, databaseError := databaseConnection.DB()
	if databaseError != nil {
		testingContext.Fatalf("get predecessor database handle: %v", databaseError)
	}
	sqlDatabase.SetMaxOpenConns(1)
	predecessorStatements := []string{
		"PRAGMA foreign_keys = OFF",
		"ALTER TABLE calendars ADD COLUMN symbol text NOT NULL DEFAULT 'E'",
		// The master merge base 881c0bc has no semantic groups or connection cursors.
		"ALTER TABLE calendar_connections DROP COLUMN calendar_list_sync_cursor",
		"ALTER TABLE calendar_connections DROP COLUMN calendar_import_cutover_at",
		"CREATE TABLE source_calendar_mappings_predecessor (id text PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime, connection_id text NOT NULL, calendar_id text NOT NULL, provider_calendar_id text NOT NULL, sync_cursor text, FOREIGN KEY (connection_id) REFERENCES calendar_connections(id) ON UPDATE CASCADE ON DELETE CASCADE, FOREIGN KEY (calendar_id) REFERENCES calendars(id) ON UPDATE CASCADE ON DELETE RESTRICT)",
		"INSERT INTO source_calendar_mappings_predecessor SELECT mappings.id, mappings.created_at, mappings.updated_at, mappings.deleted_at, states.connection_id, mappings.calendar_id, states.provider_calendar_id, states.sync_cursor FROM source_calendar_mappings AS mappings JOIN provider_calendar_sync_states AS states ON states.id = mappings.sync_state_id",
		"CREATE TABLE external_event_series_links_predecessor (id text PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime, mapping_id text NOT NULL, event_series_id text NOT NULL, provider_series_id text NOT NULL, FOREIGN KEY (mapping_id) REFERENCES source_calendar_mappings(id) ON UPDATE CASCADE ON DELETE CASCADE, FOREIGN KEY (event_series_id) REFERENCES event_series(id) ON UPDATE CASCADE ON DELETE CASCADE)",
		"CREATE TABLE external_event_links_predecessor (id text PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime, mapping_id text NOT NULL, event_id text NOT NULL, provider_event_id text NOT NULL, provider_series_id text, FOREIGN KEY (mapping_id) REFERENCES source_calendar_mappings(id) ON UPDATE CASCADE ON DELETE CASCADE, FOREIGN KEY (event_id) REFERENCES events(id) ON UPDATE CASCADE ON DELETE CASCADE)",
		"CREATE TABLE calendar_syncs_predecessor (id text PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime, mapping_id text NOT NULL, state text NOT NULL CONSTRAINT calendar_sync_state CHECK (state IN ('pending','running','succeeded','failed')), started_at datetime NOT NULL, finished_at datetime, error_code text, FOREIGN KEY (mapping_id) REFERENCES source_calendar_mappings(id) ON UPDATE CASCADE ON DELETE CASCADE)",
		"DROP TABLE calendar_syncs", "DROP TABLE external_event_links", "DROP TABLE external_event_series_links", "DROP TABLE source_calendar_mappings", "DROP TABLE provider_calendar_sync_states", "DROP TABLE tasks",
		"ALTER TABLE source_calendar_mappings_predecessor RENAME TO source_calendar_mappings", "ALTER TABLE external_event_series_links_predecessor RENAME TO external_event_series_links", "ALTER TABLE external_event_links_predecessor RENAME TO external_event_links", "ALTER TABLE calendar_syncs_predecessor RENAME TO calendar_syncs",
		"CREATE UNIQUE INDEX source_provider_calendar ON source_calendar_mappings (connection_id, provider_calendar_id)", "CREATE UNIQUE INDEX source_calendar_identity ON source_calendar_mappings (calendar_id)",
		"CREATE UNIQUE INDEX external_provider_series ON external_event_series_links (mapping_id, provider_series_id)", "CREATE UNIQUE INDEX external_series_identity ON external_event_series_links (event_series_id)",
		"CREATE UNIQUE INDEX external_provider_event ON external_event_links (mapping_id, provider_event_id)", "CREATE UNIQUE INDEX external_event_identity ON external_event_links (event_id)",
	}
	for _, statement := range predecessorStatements {
		if _, alterError := sqlDatabase.Exec(statement); alterError != nil {
			testingContext.Fatalf("construct provider calendar predecessor with %q: %v", statement, alterError)
		}
	}
	for _, insert := range []struct {
		statement string
		arguments []any
	}{
		{"INSERT INTO lanes (id, calendar_id, title, status, starts_at, ends_at, display_order) VALUES ('LANBASE1', ?, 'Imported series', 'active', '2030-01-01', '2030-02-01', 0)", []any{calendar.ID}},
		{"INSERT INTO event_series (id, lane_id, timezone, source_kind) VALUES ('SERBASE1', 'LANBASE1', 'America/Los_Angeles', 'google')", nil},
		{"INSERT INTO events (id, lane_id, event_series_id, relation_type, time_shape, at, timezone, title) VALUES ('EVTBASE1', 'LANBASE1', 'SERBASE1', 'series_occurrence', 'point', '2030-01-15', 'America/Los_Angeles', 'Imported occurrence')", nil},
		{"INSERT INTO external_event_series_links (id, mapping_id, event_series_id, provider_series_id) VALUES ('ESLBASE1', ?, 'SERBASE1', 'provider-series')", []any{mapping.ID}},
		{"INSERT INTO external_event_links (id, mapping_id, event_id, provider_event_id, provider_series_id) VALUES ('ELKBASE1', ?, 'EVTBASE1', 'provider-event', 'provider-series')", []any{mapping.ID}},
		{"INSERT INTO calendar_syncs (id, mapping_id, state, started_at, finished_at) VALUES ('SYNBASE1', ?, 'succeeded', '2030-01-01', '2030-01-01')", []any{mapping.ID}},
	} {
		if _, insertError := sqlDatabase.Exec(insert.statement, insert.arguments...); insertError != nil {
			testingContext.Fatalf("populate master database: %v", insertError)
		}
	}
	if closeError := sqlDatabase.Close(); closeError != nil {
		testingContext.Fatalf("close predecessor database: %v", closeError)
	}

	migratedDatabase, migrationError := services.OpenDatabase(databasePath)
	if migrationError != nil {
		testingContext.Fatalf("migrate provider calendar predecessor: %v", migrationError)
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
	var migratedState models.ProviderCalendarSyncState
	if findError := migratedDatabase.First(&migratedState, "connection_id = ? AND provider_calendar_id = ?", connection.ID, "source").Error; findError != nil {
		testingContext.Fatalf("read migrated provider calendar state: %v", findError)
	}
	if migratedState.SyncCursor != nil {
		testingContext.Fatalf("migrated provider calendar cursor = %#v", migratedState.SyncCursor)
	}
	var migratedConnection models.CalendarConnection
	if findError := migratedDatabase.First(&migratedConnection, "id = ?", connection.ID).Error; findError != nil {
		testingContext.Fatalf("read migrated calendar connection: %v", findError)
	}
	if migratedConnection.CalendarListSyncCursor != nil {
		testingContext.Fatalf("migrated CalendarList cursor = %#v", migratedConnection.CalendarListSyncCursor)
	}
	var migratedTask models.Task
	if findError := migratedDatabase.First(&migratedTask, "resource_type = ? AND resource_id = ?", models.TaskResourceCalendarConnection, connection.ID).Error; findError != nil {
		testingContext.Fatalf("read migrated connection task: %v", findError)
	}
	if migratedTask.State != models.TaskPending || migratedTask.RetryCount != 0 {
		testingContext.Fatalf("migrated connection task = %#v", migratedTask)
	}
	if migratedDatabase.Migrator().HasColumn("calendars", "symbol") {
		testingContext.Fatal("canonical calendar table contains obsolete symbol column after provider migration")
	}
	var migratedMapping models.SourceCalendarMapping
	if findError := migratedDatabase.First(&migratedMapping, "id = ?", mapping.ID).Error; findError != nil {
		testingContext.Fatalf("read preserved source mapping: %v", findError)
	}
	if migratedMapping.SyncStateID != migratedState.ID || migratedMapping.CalendarID != calendar.ID || migratedMapping.SemanticGroup != models.SourceCalendarGroupCalendar {
		testingContext.Fatalf("migrated source mapping = %#v", migratedMapping)
	}
	var importedEvent models.Event
	if findError := migratedDatabase.First(&importedEvent, "id = ?", "EVTBASE1").Error; findError != nil {
		testingContext.Fatalf("read preserved event: %v", findError)
	}
	if importedEvent.Title != "Imported occurrence" || importedEvent.LaneID != "LANBASE1" || importedEvent.EventSeriesID == nil || *importedEvent.EventSeriesID != "SERBASE1" {
		testingContext.Fatalf("preserved event = %#v", importedEvent)
	}
	var eventLink models.ExternalEventLink
	if findError := migratedDatabase.First(&eventLink, "id = ?", "ELKBASE1").Error; findError != nil {
		testingContext.Fatalf("read preserved event link: %v", findError)
	}
	if eventLink.SyncStateID != migratedState.ID || eventLink.EventID != importedEvent.ID || eventLink.ProviderEventID != "provider-event" || eventLink.SemanticGroup != models.SourceCalendarGroupCalendar {
		testingContext.Fatalf("preserved event link = %#v", eventLink)
	}
	var seriesLink models.ExternalEventSeriesLink
	if findError := migratedDatabase.First(&seriesLink, "id = ?", "ESLBASE1").Error; findError != nil {
		testingContext.Fatalf("read preserved series link: %v", findError)
	}
	if seriesLink.SyncStateID != migratedState.ID || seriesLink.EventSeriesID != "SERBASE1" || seriesLink.ProviderSeriesID != "provider-series" {
		testingContext.Fatalf("preserved series link = %#v", seriesLink)
	}
	var synchronization models.CalendarSync
	if findError := migratedDatabase.First(&synchronization, "id = ?", "SYNBASE1").Error; findError != nil {
		testingContext.Fatalf("read preserved synchronization: %v", findError)
	}
	if synchronization.SyncStateID != migratedState.ID || synchronization.State != models.CalendarSyncSucceeded {
		testingContext.Fatalf("preserved synchronization = %#v", synchronization)
	}
	if migratedConnection.CalendarImportCutoverAt != nil || !bytes.Equal(migratedConnection.CredentialCiphertext, connection.CredentialCiphertext) || !bytes.Equal(migratedConnection.CredentialNonce, connection.CredentialNonce) {
		testingContext.Fatal("migration changed credentials or completed the import cutover")
	}
	if updateError := migratedDatabase.Model(&migratedState).Update("sync_cursor", "canonical-cursor").Error; updateError != nil {
		testingContext.Fatalf("store canonical cursor: %v", updateError)
	}
	reopenedDatabase, reopenError := services.OpenDatabase(databasePath)
	if reopenError != nil {
		testingContext.Fatalf("reopen migrated database: %v", reopenError)
	}
	reopenedSQLDatabase, databaseError := reopenedDatabase.DB()
	if databaseError != nil {
		testingContext.Fatalf("get reopened database handle: %v", databaseError)
	}
	testingContext.Cleanup(func() {
		if closeError := reopenedSQLDatabase.Close(); closeError != nil {
			testingContext.Errorf("close reopened database: %v", closeError)
		}
	})
	var reopenedState models.ProviderCalendarSyncState
	if findError := reopenedDatabase.First(&reopenedState, "id = ?", migratedState.ID).Error; findError != nil {
		testingContext.Fatalf("read reopened state: %v", findError)
	}
	if reopenedState.SyncCursor == nil || *reopenedState.SyncCursor != "canonical-cursor" {
		testingContext.Fatal("reopening repeated the migration")
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
	calendar, calendarError := models.NewCalendar(owner.ID, "Calendar", "test", 0)
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
	if databaseConnection.Migrator().HasColumn("calendars", "symbol") {
		testingContext.Fatal("calendars table contains obsolete symbol column")
	}
}
