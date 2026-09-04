// Package services provides core functionalities for external system interactions.
package services

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tyemirov/RSVP/models"
	"github.com/tyemirov/RSVP/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	// ErrNonCanonicalDatabase indicates that a database contains an incomplete or obsolete RSVP schema.
	ErrNonCanonicalDatabase = errors.New("database does not use the canonical calendar lane schema")
)

const (
	calendarListSyncCursorColumn  = "calendar_list_sync_cursor"
	calendarImportCutoverAtColumn = "calendar_import_cutover_at"
	semanticGroupColumn           = "semantic_group"
)

var canonicalModels = []any{
	&models.User{},
	&models.Calendar{},
	&models.Lane{},
	&models.EventSeries{},
	&models.Venue{},
	&models.Event{},
	&models.RSVP{},
	&models.AttentionPolicy{},
	&models.Probe{},
	&models.CalendarAuthorizationRequest{},
	&models.CalendarConnection{},
	&models.ProviderCalendarSyncState{},
	&models.SourceCalendarMapping{},
	&models.IdempotencyRecord{},
	&models.ExternalEventSeriesLink{},
	&models.ExternalEventLink{},
	&models.CalendarSync{},
	&models.Task{},
	&models.DerivedMarkerRule{},
	&models.DerivedMarker{},
	&models.IngestionDraft{},
	&models.DraftDerivedMarkerRule{},
	&models.DraftConfirmation{},
}

var canonicalTableNames = []string{
	config.TableUsers,
	config.TableCalendars,
	config.TableLanes,
	config.TableEventSeries,
	config.TableVenues,
	config.TableEvents,
	config.TableRSVPs,
	config.TableAttentionPolicies,
	config.TableProbes,
	config.TableCalendarAuthorizationRequests,
	config.TableCalendarConnections,
	config.TableProviderCalendarSyncStates,
	config.TableSourceCalendarMappings,
	config.TableIdempotencyRecords,
	config.TableExternalEventSeriesLinks,
	config.TableExternalEventLinks,
	config.TableCalendarSyncs,
	config.TableTasks,
	config.TableDerivedMarkerRules,
	config.TableDerivedMarkers,
	config.TableIngestionDrafts,
	config.TableDraftDerivedMarkerRules,
	config.TableDraftConfirmations,
}

var canonicalColumns = map[string][]string{
	config.TableUsers:                         {"id", "created_at", "updated_at", "deleted_at", "email", "name", "picture", "timezone"},
	config.TableCalendars:                     {"id", "created_at", "updated_at", "deleted_at", "organizer_id", "name", "color_token", "display_order", "visible"},
	config.TableLanes:                         {"id", "created_at", "updated_at", "deleted_at", "calendar_id", "title", "status", "starts_at", "ends_at", "resolved_at", "display_order"},
	config.TableEventSeries:                   {"id", "created_at", "updated_at", "deleted_at", "lane_id", "timezone", "source_kind", "recurrence_rule"},
	config.TableVenues:                        {"id", "created_at", "updated_at", "deleted_at", "user_id", "name", "address", "capacity", "website", "phone", "email", "description"},
	config.TableEvents:                        {"id", "created_at", "updated_at", "deleted_at", "lane_id", "event_series_id", "anchor_event_id", "relation_type", "time_shape", "at", "starts_at", "ends_at", "start_date", "end_date", "timezone", "title", "description", "venue_id"},
	config.TableRSVPs:                         {"id", "created_at", "updated_at", "deleted_at", "name", "response", "extra_guests", "event_id"},
	config.TableAttentionPolicies:             {"id", "created_at", "updated_at", "deleted_at", "lane_id", "review_interval_seconds", "next_probe_at", "escalation_interval_seconds"},
	config.TableProbes:                        {"id", "created_at", "updated_at", "deleted_at", "policy_id", "lane_id", "due_at", "escalates_at", "state", "completed_at"},
	config.TableCalendarAuthorizationRequests: {"id", "created_at", "updated_at", "deleted_at", "organizer_id", "provider", "state_hash", "redirect_uri", "expires_at", "used_at"},
	config.TableCalendarConnections:           {"id", "created_at", "updated_at", "deleted_at", "organizer_id", "provider", "credential_nonce", "credential_ciphertext", "status", calendarListSyncCursorColumn, calendarImportCutoverAtColumn},
	config.TableProviderCalendarSyncStates:    {"id", "created_at", "updated_at", "deleted_at", "connection_id", "provider_calendar_id", "default_calendar", "sync_cursor"},
	config.TableSourceCalendarMappings:        {"id", "created_at", "updated_at", "deleted_at", "sync_state_id", "calendar_id", semanticGroupColumn},
	config.TableIdempotencyRecords:            {"id", "created_at", "updated_at", "deleted_at", "organizer_id", "operation", "key_hash", "request_hash", "response_status", "resource_type", "resource_id", "expires_at"},
	config.TableExternalEventSeriesLinks:      {"id", "created_at", "updated_at", "deleted_at", "sync_state_id", "event_series_id", "provider_series_id"},
	config.TableExternalEventLinks:            {"id", "created_at", "updated_at", "deleted_at", "sync_state_id", "event_id", "provider_event_id", "provider_series_id", semanticGroupColumn, "diagnostic_code"},
	config.TableCalendarSyncs:                 {"id", "created_at", "updated_at", "deleted_at", "sync_state_id", "state", "started_at", "finished_at", "error_code"},
	config.TableTasks:                         {"id", "created_at", "updated_at", "deleted_at", "organizer_id", "kind", "resource_type", "resource_id", "state", "scheduled_for", "retry_count", "last_attempted_at", "finished_at", "error_code"},
	config.TableDerivedMarkerRules:            {"id", "created_at", "updated_at", "deleted_at", "lane_id", "anchor_type", "anchor_id", "anchor_edge", "offset_seconds"},
	config.TableDerivedMarkers:                {"id", "created_at", "updated_at", "deleted_at", "rule_id", "lane_id", "at", "timezone"},
	config.TableIngestionDrafts:               {"id", "created_at", "updated_at", "deleted_at", "organizer_id", "status", "mode", "source", "calendar_id", "title", "anchor_event_id", "starts_at", "ends_at", "review_interval_seconds", "next_probe_at", "escalation_interval_seconds", "reference_time", "timezone", "missing_fields_json"},
	config.TableDraftDerivedMarkerRules:       {"id", "created_at", "updated_at", "deleted_at", "draft_id", "anchor_edge", "offset_seconds"},
	config.TableDraftConfirmations:            {"id", "created_at", "updated_at", "deleted_at", "draft_id", "lane_id", "event_id", "attention_policy_id"},
}

type canonicalForeignKey struct {
	column           string
	referencedTable  string
	referencedColumn string
}

var canonicalForeignKeys = map[string][]canonicalForeignKey{
	config.TableCalendars:                     {{"organizer_id", config.TableUsers, "id"}},
	config.TableLanes:                         {{"calendar_id", config.TableCalendars, "id"}},
	config.TableEventSeries:                   {{"lane_id", config.TableLanes, "id"}},
	config.TableEvents:                        {{"lane_id", config.TableLanes, "id"}, {"event_series_id", config.TableEventSeries, "id"}, {"anchor_event_id", config.TableEvents, "id"}, {"venue_id", config.TableVenues, "id"}},
	config.TableRSVPs:                         {{"event_id", config.TableEvents, "id"}},
	config.TableAttentionPolicies:             {{"lane_id", config.TableLanes, "id"}},
	config.TableProbes:                        {{"policy_id", config.TableAttentionPolicies, "id"}, {"lane_id", config.TableLanes, "id"}},
	config.TableCalendarAuthorizationRequests: {{"organizer_id", config.TableUsers, "id"}},
	config.TableCalendarConnections:           {{"organizer_id", config.TableUsers, "id"}},
	config.TableProviderCalendarSyncStates:    {{"connection_id", config.TableCalendarConnections, "id"}},
	config.TableSourceCalendarMappings:        {{"sync_state_id", config.TableProviderCalendarSyncStates, "id"}, {"calendar_id", config.TableCalendars, "id"}},
	config.TableIdempotencyRecords:            {{"organizer_id", config.TableUsers, "id"}},
	config.TableExternalEventSeriesLinks:      {{"sync_state_id", config.TableProviderCalendarSyncStates, "id"}, {"event_series_id", config.TableEventSeries, "id"}},
	config.TableExternalEventLinks:            {{"sync_state_id", config.TableProviderCalendarSyncStates, "id"}, {"event_id", config.TableEvents, "id"}},
	config.TableCalendarSyncs:                 {{"sync_state_id", config.TableProviderCalendarSyncStates, "id"}},
	config.TableTasks:                         {{"organizer_id", config.TableUsers, "id"}},
	config.TableDerivedMarkerRules:            {{"lane_id", config.TableLanes, "id"}},
	config.TableDerivedMarkers:                {{"rule_id", config.TableDerivedMarkerRules, "id"}, {"lane_id", config.TableLanes, "id"}},
	config.TableIngestionDrafts:               {{"organizer_id", config.TableUsers, "id"}, {"calendar_id", config.TableCalendars, "id"}},
	config.TableDraftDerivedMarkerRules:       {{"draft_id", config.TableIngestionDrafts, "id"}},
	config.TableDraftConfirmations:            {{"draft_id", config.TableIngestionDrafts, "id"}, {"lane_id", config.TableLanes, "id"}, {"event_id", config.TableEvents, "id"}, {"attention_policy_id", config.TableAttentionPolicies, "id"}},
}

var canonicalUniqueIndexes = map[string][][]string{
	config.TableUsers:                         {{"email"}},
	config.TableCalendars:                     {{"organizer_id", "display_order"}},
	config.TableLanes:                         {{"calendar_id", "display_order"}},
	config.TableEventSeries:                   {{"lane_id"}},
	config.TableAttentionPolicies:             {{"lane_id"}},
	config.TableProbes:                        {{"policy_id", "due_at"}},
	config.TableCalendarAuthorizationRequests: {{"state_hash"}},
	config.TableCalendarConnections:           {{"organizer_id", "provider"}},
	config.TableProviderCalendarSyncStates:    {{"connection_id", "provider_calendar_id"}},
	config.TableSourceCalendarMappings:        {{"sync_state_id", semanticGroupColumn}},
	config.TableIdempotencyRecords:            {{"organizer_id", "operation", "key_hash"}},
	config.TableExternalEventSeriesLinks:      {{"sync_state_id", "provider_series_id"}, {"event_series_id"}},
	config.TableExternalEventLinks:            {{"sync_state_id", "provider_event_id"}, {"event_id"}},
	config.TableTasks:                         {{"kind", "resource_type", "resource_id"}},
	config.TableDerivedMarkers:                {{"rule_id"}},
	config.TableDraftDerivedMarkerRules:       {{"draft_id", "anchor_edge", "offset_seconds"}},
	config.TableDraftConfirmations:            {{"draft_id"}},
}

var canonicalCheckConstraints = map[string][]string{
	config.TableCalendars:                     {"calendar_display_order"},
	config.TableLanes:                         {"lane_state", "lane_display_order"},
	config.TableEventSeries:                   {"event_series_source_kind"},
	config.TableEvents:                        {"event_relation", "event_time_shape", "event_timezone"},
	config.TableAttentionPolicies:             {"attention_review_interval", "attention_escalation_interval"},
	config.TableProbes:                        {"probe_state"},
	config.TableCalendarAuthorizationRequests: {"calendar_authorization_provider"},
	config.TableCalendarConnections:           {"calendar_connection_provider", "calendar_connection_status"},
	config.TableSourceCalendarMappings:        {"source_calendar_group"},
	config.TableExternalEventLinks:            {"external_event_semantic_group"},
	config.TableCalendarSyncs:                 {"calendar_sync_state"},
	config.TableTasks:                         {"task_kind", "task_resource_type", "task_state", "task_retry_count"},
	config.TableDerivedMarkerRules:            {"derived_anchor_type", "derived_anchor_edge"},
	config.TableIngestionDrafts:               {"ingestion_draft_status", "ingestion_draft_mode", "ingestion_draft_source"},
	config.TableDraftDerivedMarkerRules:       {"draft_derived_anchor_edge"},
}

// InitDatabase opens a canonical database or stops application startup.
func InitDatabase(databaseFileName string, applicationLogger *log.Logger) *gorm.DB {
	databaseConnection, connectionError := OpenDatabase(databaseFileName)
	if connectionError != nil {
		applicationLogger.Fatalf("Open canonical database %s: %v", databaseFileName, connectionError)
	}
	applicationLogger.Printf("Canonical database connection established to %s", databaseFileName)
	return databaseConnection
}

// OpenDatabase opens a complete canonical database or initializes an empty database.
func OpenDatabase(databaseFileName string) (*gorm.DB, error) {
	databaseDirectoryName := filepath.Dir(databaseFileName)
	if databaseDirectoryName != "." && databaseDirectoryName != "" {
		if directoryError := os.MkdirAll(databaseDirectoryName, 0755); directoryError != nil {
			return nil, fmt.Errorf("create database directory %s: %w", databaseDirectoryName, directoryError)
		}
	}

	databaseConnection, connectionError := gorm.Open(sqlite.Open(databaseFileName+"?_foreign_keys=on"), &gorm.Config{})
	if connectionError != nil {
		return nil, fmt.Errorf("connect to database %s: %w", databaseFileName, connectionError)
	}

	presentTableCount := 0
	for _, tableName := range canonicalTableNames {
		if databaseConnection.Migrator().HasTable(tableName) {
			presentTableCount++
		}
	}
	var userTableCount int
	if countError := databaseConnection.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&userTableCount).Error; countError != nil {
		return nil, fmt.Errorf("inspect database tables: %w", countError)
	}

	if userTableCount == 0 {
		if createError := databaseConnection.Transaction(func(transaction *gorm.DB) error {
			return transaction.Migrator().CreateTable(canonicalModels...)
		}); createError != nil {
			return nil, fmt.Errorf("initialize canonical schema: %w", createError)
		}
	} else if presentTableCount == len(canonicalTableNames) && userTableCount == len(canonicalTableNames) {
		if databaseConnection.Migrator().HasColumn(config.TableCalendars, "symbol") {
			if migrationError := migrateCalendarSymbolContract(databaseConnection); migrationError != nil {
				return nil, migrationError
			}
		}
	} else if userTableCount == len(canonicalTableNames)-1 && !databaseConnection.Migrator().HasTable(config.TableTasks) {
		if migrationError := migrateTaskContract(databaseConnection, userTableCount); migrationError != nil {
			return nil, migrationError
		}
	} else if migrationError := migrateProviderCalendarSyncContract(databaseConnection, userTableCount); migrationError != nil {
		return nil, migrationError
	}

	if validationError := validateCanonicalSchema(databaseConnection); validationError != nil {
		return nil, validationError
	}
	return databaseConnection, nil
}

func validateCanonicalSchema(databaseConnection *gorm.DB) error {
	return validateSchemaContract(databaseConnection, canonicalTableNames, canonicalColumns, canonicalForeignKeys, canonicalUniqueIndexes, canonicalCheckConstraints)
}

func migrateCalendarSymbolContract(databaseConnection *gorm.DB) error {
	columns := cloneColumns(canonicalColumns)
	columns[config.TableCalendars] = append(columns[config.TableCalendars], "symbol")
	if validationError := validateSchemaContract(databaseConnection, canonicalTableNames, columns, canonicalForeignKeys, canonicalUniqueIndexes, canonicalCheckConstraints); validationError != nil {
		return validationError
	}
	if migrationError := databaseConnection.Exec("ALTER TABLE " + config.TableCalendars + " DROP COLUMN symbol").Error; migrationError != nil {
		return fmt.Errorf("remove obsolete calendar symbol: %w", migrationError)
	}
	return nil
}

func migrateTaskContract(databaseConnection *gorm.DB, userTableCount int) error {
	tableNames, columns, foreignKeys, uniqueIndexes, checkConstraints := taskPredecessorContract()
	if userTableCount != len(tableNames) || databaseConnection.Migrator().HasTable(config.TableTasks) {
		return fmt.Errorf("%w: found an unsupported database table set", ErrNonCanonicalDatabase)
	}
	if validationError := validateSchemaContract(databaseConnection, tableNames, columns, foreignKeys, uniqueIndexes, checkConstraints); validationError != nil {
		return validationError
	}
	if createError := databaseConnection.Transaction(func(transaction *gorm.DB) error {
		if createError := transaction.Migrator().CreateTable(&models.Task{}); createError != nil {
			return createError
		}
		if createError := createConnectionTasks(transaction, true); createError != nil {
			return createError
		}
		return transaction.Exec("ALTER TABLE " + config.TableCalendars + " DROP COLUMN symbol").Error
	}); createError != nil {
		return fmt.Errorf("migrate task contract: %w", createError)
	}
	return nil
}

func taskPredecessorContract() ([]string, map[string][]string, map[string][]canonicalForeignKey, map[string][][]string, map[string][]string) {
	tableNames := make([]string, 0, len(canonicalTableNames)-1)
	for _, tableName := range canonicalTableNames {
		if tableName != config.TableTasks {
			tableNames = append(tableNames, tableName)
		}
	}
	columns := cloneColumns(canonicalColumns)
	columns[config.TableCalendars] = append(columns[config.TableCalendars], "symbol")
	foreignKeys := cloneForeignKeys(canonicalForeignKeys)
	uniqueIndexes := cloneUniqueIndexes(canonicalUniqueIndexes)
	checkConstraints := cloneCheckConstraints(canonicalCheckConstraints)
	delete(columns, config.TableTasks)
	delete(foreignKeys, config.TableTasks)
	delete(uniqueIndexes, config.TableTasks)
	delete(checkConstraints, config.TableTasks)
	return tableNames, columns, foreignKeys, uniqueIndexes, checkConstraints
}

func validateSchemaContract(databaseConnection *gorm.DB, tableNames []string, expectedColumns map[string][]string, expectedForeignKeys map[string][]canonicalForeignKey, expectedUniqueIndexes map[string][][]string, expectedCheckConstraints map[string][]string) error {
	var foreignKeysEnabled int
	if pragmaError := databaseConnection.Raw("PRAGMA foreign_keys").Scan(&foreignKeysEnabled).Error; pragmaError != nil {
		return fmt.Errorf("read SQLite foreign key state: %w", pragmaError)
	}
	if foreignKeysEnabled != 1 {
		return fmt.Errorf("%w: SQLite foreign keys are disabled", ErrNonCanonicalDatabase)
	}
	for _, tableName := range tableNames {
		if columnError := validateCanonicalColumns(databaseConnection, tableName, expectedColumns[tableName]); columnError != nil {
			return columnError
		}
		if foreignKeyError := validateCanonicalForeignKeys(databaseConnection, tableName, expectedForeignKeys[tableName]); foreignKeyError != nil {
			return foreignKeyError
		}
		if indexError := validateCanonicalUniqueIndexes(databaseConnection, tableName, expectedUniqueIndexes[tableName]); indexError != nil {
			return indexError
		}
		if constraintError := validateCanonicalCheckConstraints(databaseConnection, tableName, expectedCheckConstraints[tableName]); constraintError != nil {
			return constraintError
		}
	}
	return nil
}

func migrateProviderCalendarSyncContract(databaseConnection *gorm.DB, userTableCount int) error {
	tableNames, columns, foreignKeys, uniqueIndexes, checkConstraints := providerCalendarSyncPredecessorContract()
	if userTableCount != len(tableNames) || databaseConnection.Migrator().HasTable(config.TableProviderCalendarSyncStates) {
		return fmt.Errorf("%w: found an unsupported database table set", ErrNonCanonicalDatabase)
	}
	if validationError := validateSchemaContract(databaseConnection, tableNames, columns, foreignKeys, uniqueIndexes, checkConstraints); validationError != nil {
		return validationError
	}

	type providerCalendarSource struct {
		ConnectionID       string
		ProviderCalendarID string
	}
	var sources []providerCalendarSource
	if findError := databaseConnection.Table(config.TableSourceCalendarMappings).
		Select("connection_id, provider_calendar_id").
		Where("deleted_at IS NULL").
		Group("connection_id, provider_calendar_id").
		Order("connection_id, provider_calendar_id").
		Scan(&sources).Error; findError != nil {
		return fmt.Errorf("read provider calendar migration sources: %w", findError)
	}

	if pragmaError := databaseConnection.Exec("PRAGMA foreign_keys = OFF").Error; pragmaError != nil {
		return fmt.Errorf("disable foreign keys for provider calendar migration: %w", pragmaError)
	}
	migrationError := databaseConnection.Transaction(func(transaction *gorm.DB) error {
		for _, column := range []string{calendarListSyncCursorColumn, calendarImportCutoverAtColumn} {
			if createError := transaction.Migrator().AddColumn(&models.CalendarConnection{}, column); createError != nil {
				return createError
			}
		}
		if createError := transaction.Migrator().CreateTable(&models.ProviderCalendarSyncState{}); createError != nil {
			return createError
		}
		if createError := transaction.Migrator().CreateTable(&models.Task{}); createError != nil {
			return createError
		}
		if createError := createConnectionTasks(transaction, false); createError != nil {
			return createError
		}
		if createError := transaction.Exec("CREATE TABLE b047_sync_state_lookup (connection_id text NOT NULL, provider_calendar_id text NOT NULL, sync_state_id text NOT NULL, PRIMARY KEY (connection_id, provider_calendar_id))").Error; createError != nil {
			return createError
		}
		for _, source := range sources {
			state, stateError := models.NewProviderCalendarSyncState(source.ConnectionID, source.ProviderCalendarID)
			if stateError != nil {
				return stateError
			}
			if createError := transaction.Create(state).Error; createError != nil {
				return createError
			}
			if insertError := transaction.Exec("INSERT INTO b047_sync_state_lookup (connection_id, provider_calendar_id, sync_state_id) VALUES (?, ?, ?)", source.ConnectionID, source.ProviderCalendarID, state.ID).Error; insertError != nil {
				return insertError
			}
		}

		statements := []string{
			"CREATE TABLE source_calendar_mappings_b047 (id text PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime, sync_state_id text NOT NULL, calendar_id text NOT NULL, semantic_group text NOT NULL CONSTRAINT source_calendar_group CHECK (semantic_group IN ('calendar','birthdays')), CONSTRAINT fk_source_calendar_mappings_sync_state FOREIGN KEY (sync_state_id) REFERENCES provider_calendar_sync_states(id) ON UPDATE CASCADE ON DELETE CASCADE, CONSTRAINT fk_source_calendar_mappings_calendar FOREIGN KEY (calendar_id) REFERENCES calendars(id) ON UPDATE CASCADE ON DELETE RESTRICT)",
			"INSERT INTO source_calendar_mappings_b047 SELECT mappings.id, mappings.created_at, mappings.updated_at, mappings.deleted_at, lookup.sync_state_id, mappings.calendar_id, 'calendar' FROM source_calendar_mappings AS mappings JOIN b047_sync_state_lookup AS lookup ON lookup.connection_id = mappings.connection_id AND lookup.provider_calendar_id = mappings.provider_calendar_id WHERE mappings.deleted_at IS NULL",
			"CREATE TABLE external_event_series_links_b047 (id text PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime, sync_state_id text NOT NULL, event_series_id text NOT NULL, provider_series_id text NOT NULL, CONSTRAINT fk_external_event_series_links_sync_state FOREIGN KEY (sync_state_id) REFERENCES provider_calendar_sync_states(id) ON UPDATE CASCADE ON DELETE CASCADE, CONSTRAINT fk_external_event_series_links_series FOREIGN KEY (event_series_id) REFERENCES event_series(id) ON UPDATE CASCADE ON DELETE CASCADE)",
			"INSERT INTO external_event_series_links_b047 SELECT links.id, links.created_at, links.updated_at, links.deleted_at, lookup.sync_state_id, links.event_series_id, links.provider_series_id FROM external_event_series_links AS links JOIN source_calendar_mappings AS mappings ON mappings.id = links.mapping_id JOIN b047_sync_state_lookup AS lookup ON lookup.connection_id = mappings.connection_id AND lookup.provider_calendar_id = mappings.provider_calendar_id WHERE links.deleted_at IS NULL AND mappings.deleted_at IS NULL",
			"CREATE TABLE external_event_links_b047 (id text PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime, sync_state_id text NOT NULL, event_id text NOT NULL, provider_event_id text NOT NULL, provider_series_id text, semantic_group text NOT NULL CONSTRAINT external_event_semantic_group CHECK (semantic_group IN ('calendar','birthdays')), diagnostic_code text, CONSTRAINT fk_external_event_links_sync_state FOREIGN KEY (sync_state_id) REFERENCES provider_calendar_sync_states(id) ON UPDATE CASCADE ON DELETE CASCADE, CONSTRAINT fk_external_event_links_event FOREIGN KEY (event_id) REFERENCES events(id) ON UPDATE CASCADE ON DELETE CASCADE)",
			"INSERT INTO external_event_links_b047 SELECT links.id, links.created_at, links.updated_at, links.deleted_at, lookup.sync_state_id, links.event_id, links.provider_event_id, links.provider_series_id, 'calendar', NULL FROM external_event_links AS links JOIN source_calendar_mappings AS mappings ON mappings.id = links.mapping_id JOIN b047_sync_state_lookup AS lookup ON lookup.connection_id = mappings.connection_id AND lookup.provider_calendar_id = mappings.provider_calendar_id WHERE links.deleted_at IS NULL AND mappings.deleted_at IS NULL",
			"CREATE TABLE calendar_syncs_b047 (id text PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime, sync_state_id text NOT NULL, state text NOT NULL CONSTRAINT calendar_sync_state CHECK (state IN ('pending','running','succeeded','failed')), started_at datetime NOT NULL, finished_at datetime, error_code text, CONSTRAINT fk_calendar_syncs_sync_state FOREIGN KEY (sync_state_id) REFERENCES provider_calendar_sync_states(id) ON UPDATE CASCADE ON DELETE CASCADE)",
			"INSERT INTO calendar_syncs_b047 SELECT synchronizations.id, synchronizations.created_at, synchronizations.updated_at, synchronizations.deleted_at, lookup.sync_state_id, synchronizations.state, synchronizations.started_at, synchronizations.finished_at, synchronizations.error_code FROM calendar_syncs AS synchronizations JOIN source_calendar_mappings AS mappings ON mappings.id = synchronizations.mapping_id JOIN b047_sync_state_lookup AS lookup ON lookup.connection_id = mappings.connection_id AND lookup.provider_calendar_id = mappings.provider_calendar_id WHERE mappings.deleted_at IS NULL",
			"DROP TABLE calendar_syncs",
			"DROP TABLE external_event_links",
			"DROP TABLE external_event_series_links",
			"DROP TABLE source_calendar_mappings",
			"ALTER TABLE source_calendar_mappings_b047 RENAME TO source_calendar_mappings",
			"ALTER TABLE external_event_series_links_b047 RENAME TO external_event_series_links",
			"ALTER TABLE external_event_links_b047 RENAME TO external_event_links",
			"ALTER TABLE calendar_syncs_b047 RENAME TO calendar_syncs",
			"CREATE UNIQUE INDEX source_semantic_group ON source_calendar_mappings (sync_state_id, semantic_group)",
			"CREATE INDEX idx_source_calendar_mappings_calendar_id ON source_calendar_mappings (calendar_id)",
			"CREATE UNIQUE INDEX external_provider_series ON external_event_series_links (sync_state_id, provider_series_id)",
			"CREATE UNIQUE INDEX external_event_series_identity ON external_event_series_links (event_series_id)",
			"CREATE UNIQUE INDEX external_provider_event ON external_event_links (sync_state_id, provider_event_id)",
			"CREATE UNIQUE INDEX external_event_identity ON external_event_links (event_id)",
			"CREATE INDEX idx_calendar_syncs_sync_state_id ON calendar_syncs (sync_state_id)",
			"DROP TABLE b047_sync_state_lookup",
			"ALTER TABLE calendars DROP COLUMN symbol",
		}
		for _, statement := range statements {
			if executionError := transaction.Exec(statement).Error; executionError != nil {
				return executionError
			}
		}
		return nil
	})
	if pragmaError := databaseConnection.Exec("PRAGMA foreign_keys = ON").Error; pragmaError != nil {
		return fmt.Errorf("enable foreign keys after provider calendar migration: %w", pragmaError)
	}
	if migrationError != nil {
		return fmt.Errorf("migrate provider calendar sync contract: %w", migrationError)
	}
	var foreignKeyViolations int64
	if checkError := databaseConnection.Raw("SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&foreignKeyViolations).Error; checkError != nil {
		return fmt.Errorf("check provider calendar migration foreign keys: %w", checkError)
	}
	if foreignKeyViolations != 0 {
		return fmt.Errorf("%w: provider calendar migration has %d foreign key violations", ErrNonCanonicalDatabase, foreignKeyViolations)
	}
	return nil
}

func createConnectionTasks(database *gorm.DB, completed bool) error {
	var connections []models.CalendarConnection
	if findError := database.Where("status = ?", models.CalendarConnectionConnected).Order("id ASC").Find(&connections).Error; findError != nil {
		return fmt.Errorf("read predecessor calendar connections: %w", findError)
	}
	completedAt := time.Now().UTC()
	for connectionIndex := range connections {
		connection := &connections[connectionIndex]
		task, taskError := models.NewCalendarConnectionImportTask(connection.OrganizerID, connection.ID, completedAt)
		if taskError != nil {
			return taskError
		}
		if completed {
			task.State = models.TaskSucceeded
			task.RetryCount = 1
			task.LastAttemptedAt = &completedAt
			task.FinishedAt = &completedAt
		}
		if createError := database.Create(task).Error; createError != nil {
			return fmt.Errorf("create predecessor task: %w", createError)
		}
	}
	return nil
}

func providerCalendarSyncPredecessorContract() ([]string, map[string][]string, map[string][]canonicalForeignKey, map[string][][]string, map[string][]string) {
	tableNames := make([]string, 0, len(canonicalTableNames)-2)
	for _, tableName := range canonicalTableNames {
		if tableName != config.TableProviderCalendarSyncStates && tableName != config.TableTasks {
			tableNames = append(tableNames, tableName)
		}
	}
	columns := cloneColumns(canonicalColumns)
	columns[config.TableCalendars] = append(columns[config.TableCalendars], "symbol")
	delete(columns, config.TableProviderCalendarSyncStates)
	delete(columns, config.TableTasks)
	columns[config.TableCalendarConnections] = []string{"id", "created_at", "updated_at", "deleted_at", "organizer_id", "provider", "credential_nonce", "credential_ciphertext", "status"}
	columns[config.TableSourceCalendarMappings] = []string{"id", "created_at", "updated_at", "deleted_at", "connection_id", "calendar_id", "provider_calendar_id", "sync_cursor"}
	columns[config.TableExternalEventSeriesLinks] = []string{"id", "created_at", "updated_at", "deleted_at", "mapping_id", "event_series_id", "provider_series_id"}
	columns[config.TableExternalEventLinks] = []string{"id", "created_at", "updated_at", "deleted_at", "mapping_id", "event_id", "provider_event_id", "provider_series_id"}
	columns[config.TableCalendarSyncs] = []string{"id", "created_at", "updated_at", "deleted_at", "mapping_id", "state", "started_at", "finished_at", "error_code"}
	foreignKeys := cloneForeignKeys(canonicalForeignKeys)
	delete(foreignKeys, config.TableProviderCalendarSyncStates)
	delete(foreignKeys, config.TableTasks)
	foreignKeys[config.TableSourceCalendarMappings] = []canonicalForeignKey{{"connection_id", config.TableCalendarConnections, "id"}, {"calendar_id", config.TableCalendars, "id"}}
	foreignKeys[config.TableExternalEventSeriesLinks] = []canonicalForeignKey{{"mapping_id", config.TableSourceCalendarMappings, "id"}, {"event_series_id", config.TableEventSeries, "id"}}
	foreignKeys[config.TableExternalEventLinks] = []canonicalForeignKey{{"mapping_id", config.TableSourceCalendarMappings, "id"}, {"event_id", config.TableEvents, "id"}}
	foreignKeys[config.TableCalendarSyncs] = []canonicalForeignKey{{"mapping_id", config.TableSourceCalendarMappings, "id"}}
	uniqueIndexes := cloneUniqueIndexes(canonicalUniqueIndexes)
	delete(uniqueIndexes, config.TableProviderCalendarSyncStates)
	delete(uniqueIndexes, config.TableTasks)
	uniqueIndexes[config.TableSourceCalendarMappings] = [][]string{{"connection_id", "provider_calendar_id"}, {"calendar_id"}}
	uniqueIndexes[config.TableExternalEventSeriesLinks] = [][]string{{"mapping_id", "provider_series_id"}, {"event_series_id"}}
	uniqueIndexes[config.TableExternalEventLinks] = [][]string{{"mapping_id", "provider_event_id"}, {"event_id"}}
	checkConstraints := cloneCheckConstraints(canonicalCheckConstraints)
	delete(checkConstraints, config.TableProviderCalendarSyncStates)
	delete(checkConstraints, config.TableTasks)
	delete(checkConstraints, config.TableExternalEventLinks)
	delete(checkConstraints, config.TableSourceCalendarMappings)
	return tableNames, columns, foreignKeys, uniqueIndexes, checkConstraints
}

func cloneColumns(source map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(source))
	for tableName, columns := range source {
		cloned[tableName] = append([]string(nil), columns...)
	}
	return cloned
}

func cloneForeignKeys(source map[string][]canonicalForeignKey) map[string][]canonicalForeignKey {
	cloned := make(map[string][]canonicalForeignKey, len(source))
	for tableName, foreignKeys := range source {
		cloned[tableName] = append([]canonicalForeignKey(nil), foreignKeys...)
	}
	return cloned
}

func cloneUniqueIndexes(source map[string][][]string) map[string][][]string {
	cloned := make(map[string][][]string, len(source))
	for tableName, indexes := range source {
		cloned[tableName] = make([][]string, len(indexes))
		for index := range indexes {
			cloned[tableName][index] = append([]string(nil), indexes[index]...)
		}
	}
	return cloned
}

func cloneCheckConstraints(source map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(source))
	for tableName, constraints := range source {
		cloned[tableName] = append([]string(nil), constraints...)
	}
	return cloned
}

type sqliteColumn struct {
	Name       string
	PrimaryKey int `gorm:"column:pk"`
}

func validateCanonicalColumns(databaseConnection *gorm.DB, tableName string, expectedColumns []string) error {
	var actualColumns []sqliteColumn
	if columnsError := databaseConnection.Raw("SELECT name, pk FROM pragma_table_info(?)", tableName).Scan(&actualColumns).Error; columnsError != nil {
		return fmt.Errorf("read %s columns: %w", tableName, columnsError)
	}
	actualNames := make([]string, 0, len(actualColumns))
	idIsPrimaryKey := false
	for _, column := range actualColumns {
		actualNames = append(actualNames, column.Name)
		if column.Name == "id" && column.PrimaryKey == 1 {
			idIsPrimaryKey = true
		}
	}
	sort.Strings(actualNames)
	expectedNames := append([]string(nil), expectedColumns...)
	sort.Strings(expectedNames)
	if strings.Join(actualNames, ",") != strings.Join(expectedNames, ",") {
		return fmt.Errorf("%w: table %s columns are [%s], want [%s]", ErrNonCanonicalDatabase, tableName, strings.Join(actualNames, ", "), strings.Join(expectedNames, ", "))
	}
	if !idIsPrimaryKey {
		return fmt.Errorf("%w: table %s id is not the primary key", ErrNonCanonicalDatabase, tableName)
	}
	return nil
}

type sqliteForeignKey struct {
	Column           string `gorm:"column:from"`
	ReferencedTable  string `gorm:"column:table"`
	ReferencedColumn string `gorm:"column:to"`
}

func validateCanonicalForeignKeys(databaseConnection *gorm.DB, tableName string, expectedForeignKeys []canonicalForeignKey) error {
	var actualForeignKeys []sqliteForeignKey
	if foreignKeyError := databaseConnection.Raw("SELECT `from`, `table`, `to` FROM pragma_foreign_key_list(?)", tableName).Scan(&actualForeignKeys).Error; foreignKeyError != nil {
		return fmt.Errorf("read %s foreign keys: %w", tableName, foreignKeyError)
	}
	actualKeys := make([]string, 0, len(actualForeignKeys))
	for _, foreignKey := range actualForeignKeys {
		actualKeys = append(actualKeys, foreignKey.Column+"->"+foreignKey.ReferencedTable+"."+foreignKey.ReferencedColumn)
	}
	expectedKeys := make([]string, 0, len(expectedForeignKeys))
	for _, foreignKey := range expectedForeignKeys {
		expectedKeys = append(expectedKeys, foreignKey.column+"->"+foreignKey.referencedTable+"."+foreignKey.referencedColumn)
	}
	sort.Strings(actualKeys)
	sort.Strings(expectedKeys)
	if strings.Join(actualKeys, ",") != strings.Join(expectedKeys, ",") {
		return fmt.Errorf("%w: table %s foreign keys are [%s], want [%s]", ErrNonCanonicalDatabase, tableName, strings.Join(actualKeys, ", "), strings.Join(expectedKeys, ", "))
	}
	return nil
}

type sqliteIndex struct {
	Name   string
	Unique int
}

func validateCanonicalUniqueIndexes(databaseConnection *gorm.DB, tableName string, expectedIndexes [][]string) error {
	var indexes []sqliteIndex
	if indexError := databaseConnection.Raw("SELECT name, `unique` FROM pragma_index_list(?)", tableName).Scan(&indexes).Error; indexError != nil {
		return fmt.Errorf("read %s indexes: %w", tableName, indexError)
	}
	actualUniqueIndexes := make(map[string]struct{})
	for _, index := range indexes {
		if index.Unique != 1 {
			continue
		}
		var columnNames []string
		if columnError := databaseConnection.Raw("SELECT name FROM pragma_index_info(?) ORDER BY seqno", index.Name).Scan(&columnNames).Error; columnError != nil {
			return fmt.Errorf("read %s index %s: %w", tableName, index.Name, columnError)
		}
		actualUniqueIndexes[strings.Join(columnNames, ",")] = struct{}{}
	}
	for _, expectedIndex := range expectedIndexes {
		if _, exists := actualUniqueIndexes[strings.Join(expectedIndex, ",")]; !exists {
			return fmt.Errorf("%w: table %s has no unique index on (%s)", ErrNonCanonicalDatabase, tableName, strings.Join(expectedIndex, ", "))
		}
	}
	return nil
}

func validateCanonicalCheckConstraints(databaseConnection *gorm.DB, tableName string, expectedConstraints []string) error {
	if len(expectedConstraints) == 0 {
		return nil
	}
	var tableSQL string
	if schemaError := databaseConnection.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&tableSQL).Error; schemaError != nil {
		return fmt.Errorf("read %s table schema: %w", tableName, schemaError)
	}
	for _, constraintName := range expectedConstraints {
		if !strings.Contains(tableSQL, constraintName) {
			return fmt.Errorf("%w: table %s has no %s constraint", ErrNonCanonicalDatabase, tableName, constraintName)
		}
	}
	return nil
}
