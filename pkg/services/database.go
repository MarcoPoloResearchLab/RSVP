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
	calendarListSyncCursorColumn   = "calendar_list_sync_cursor"
	calendarImportCutoverAtColumn  = "calendar_import_cutover_at"
	semanticGroupColumn            = "semantic_group"
	providerGroupPredecessorColumn = "provider_group"
	sourceProviderCalendarIndex    = "source_provider_calendar"
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
	&models.SourceCalendarMapping{},
	&models.IdempotencyRecord{},
	&models.ExternalEventSeriesLink{},
	&models.ExternalEventLink{},
	&models.CalendarSync{},
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
	config.TableSourceCalendarMappings,
	config.TableIdempotencyRecords,
	config.TableExternalEventSeriesLinks,
	config.TableExternalEventLinks,
	config.TableCalendarSyncs,
	config.TableDerivedMarkerRules,
	config.TableDerivedMarkers,
	config.TableIngestionDrafts,
	config.TableDraftDerivedMarkerRules,
	config.TableDraftConfirmations,
}

var canonicalColumns = map[string][]string{
	config.TableUsers:                         {"id", "created_at", "updated_at", "deleted_at", "email", "name", "picture", "timezone"},
	config.TableCalendars:                     {"id", "created_at", "updated_at", "deleted_at", "organizer_id", "name", "symbol", "color_token", "display_order", "visible"},
	config.TableLanes:                         {"id", "created_at", "updated_at", "deleted_at", "calendar_id", "title", "status", "starts_at", "ends_at", "resolved_at", "display_order"},
	config.TableEventSeries:                   {"id", "created_at", "updated_at", "deleted_at", "lane_id", "timezone", "source_kind", "recurrence_rule"},
	config.TableVenues:                        {"id", "created_at", "updated_at", "deleted_at", "user_id", "name", "address", "capacity", "website", "phone", "email", "description"},
	config.TableEvents:                        {"id", "created_at", "updated_at", "deleted_at", "lane_id", "event_series_id", "anchor_event_id", "relation_type", "time_shape", "at", "starts_at", "ends_at", "start_date", "end_date", "timezone", "title", "description", "venue_id"},
	config.TableRSVPs:                         {"id", "created_at", "updated_at", "deleted_at", "name", "response", "extra_guests", "event_id"},
	config.TableAttentionPolicies:             {"id", "created_at", "updated_at", "deleted_at", "lane_id", "review_interval_seconds", "next_probe_at", "escalation_interval_seconds"},
	config.TableProbes:                        {"id", "created_at", "updated_at", "deleted_at", "policy_id", "lane_id", "due_at", "escalates_at", "state", "completed_at"},
	config.TableCalendarAuthorizationRequests: {"id", "created_at", "updated_at", "deleted_at", "organizer_id", "provider", "state_hash", "redirect_uri", "expires_at", "used_at"},
	config.TableCalendarConnections:           {"id", "created_at", "updated_at", "deleted_at", "organizer_id", "provider", "credential_nonce", "credential_ciphertext", "status", calendarListSyncCursorColumn, calendarImportCutoverAtColumn},
	config.TableSourceCalendarMappings:        {"id", "created_at", "updated_at", "deleted_at", "connection_id", "calendar_id", "provider_calendar_id", semanticGroupColumn, "sync_cursor"},
	config.TableIdempotencyRecords:            {"id", "created_at", "updated_at", "deleted_at", "organizer_id", "operation", "key_hash", "request_hash", "response_status", "resource_type", "resource_id", "expires_at"},
	config.TableExternalEventSeriesLinks:      {"id", "created_at", "updated_at", "deleted_at", "mapping_id", "event_series_id", "provider_series_id"},
	config.TableExternalEventLinks:            {"id", "created_at", "updated_at", "deleted_at", "mapping_id", "event_id", "provider_event_id", "provider_series_id"},
	config.TableCalendarSyncs:                 {"id", "created_at", "updated_at", "deleted_at", "mapping_id", "state", "started_at", "finished_at", "error_code"},
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
	config.TableSourceCalendarMappings:        {{"connection_id", config.TableCalendarConnections, "id"}, {"calendar_id", config.TableCalendars, "id"}},
	config.TableIdempotencyRecords:            {{"organizer_id", config.TableUsers, "id"}},
	config.TableExternalEventSeriesLinks:      {{"mapping_id", config.TableSourceCalendarMappings, "id"}, {"event_series_id", config.TableEventSeries, "id"}},
	config.TableExternalEventLinks:            {{"mapping_id", config.TableSourceCalendarMappings, "id"}, {"event_id", config.TableEvents, "id"}},
	config.TableCalendarSyncs:                 {{"mapping_id", config.TableSourceCalendarMappings, "id"}},
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
	config.TableSourceCalendarMappings:        {{"connection_id", "provider_calendar_id", semanticGroupColumn}, {"calendar_id"}},
	config.TableIdempotencyRecords:            {{"organizer_id", "operation", "key_hash"}},
	config.TableExternalEventSeriesLinks:      {{"mapping_id", "provider_series_id"}, {"event_series_id"}},
	config.TableExternalEventLinks:            {{"mapping_id", "provider_event_id"}, {"event_id"}},
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
	config.TableCalendarSyncs:                 {"calendar_sync_state"},
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
	} else if presentTableCount != len(canonicalTableNames) || userTableCount != len(canonicalTableNames) {
		return nil, fmt.Errorf("%w: found %d of %d required tables among %d user tables", ErrNonCanonicalDatabase, presentTableCount, len(canonicalTableNames), userTableCount)
	} else {
		if migrationError := migrateCalendarListSyncCursor(databaseConnection); migrationError != nil {
			return nil, migrationError
		}
		if migrationError := migrateCalendarGroupingContract(databaseConnection); migrationError != nil {
			return nil, migrationError
		}
	}

	if validationError := validateCanonicalSchema(databaseConnection); validationError != nil {
		return nil, validationError
	}
	return databaseConnection, nil
}

func validateCanonicalSchema(databaseConnection *gorm.DB) error {
	return validateCanonicalSchemaContract(databaseConnection, canonicalColumns, canonicalUniqueIndexes, canonicalCheckConstraints)
}

func validateCanonicalSchemaContract(databaseConnection *gorm.DB, expectedColumns map[string][]string, expectedUniqueIndexes map[string][][]string, expectedCheckConstraints map[string][]string) error {
	var foreignKeysEnabled int
	if pragmaError := databaseConnection.Raw("PRAGMA foreign_keys").Scan(&foreignKeysEnabled).Error; pragmaError != nil {
		return fmt.Errorf("read SQLite foreign key state: %w", pragmaError)
	}
	if foreignKeysEnabled != 1 {
		return fmt.Errorf("%w: SQLite foreign keys are disabled", ErrNonCanonicalDatabase)
	}
	for _, tableName := range canonicalTableNames {
		if columnError := validateCanonicalColumns(databaseConnection, tableName, expectedColumns[tableName]); columnError != nil {
			return columnError
		}
		if foreignKeyError := validateCanonicalForeignKeys(databaseConnection, tableName, canonicalForeignKeys[tableName]); foreignKeyError != nil {
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

func migrateCalendarListSyncCursor(databaseConnection *gorm.DB) error {
	if databaseConnection.Migrator().HasColumn(&models.CalendarConnection{}, "CalendarListSyncCursor") {
		return nil
	}

	predecessorColumns, predecessorUniqueIndexes, predecessorCheckConstraints := calendarGroupingPredecessorContract()
	predecessorColumns[config.TableCalendarConnections] = columnsWithout(predecessorColumns[config.TableCalendarConnections], calendarListSyncCursorColumn)
	if validationError := validateCanonicalSchemaContract(databaseConnection, predecessorColumns, predecessorUniqueIndexes, predecessorCheckConstraints); validationError != nil {
		return validationError
	}

	if migrationError := databaseConnection.Transaction(func(transaction *gorm.DB) error {
		return transaction.Exec("ALTER TABLE " + config.TableCalendarConnections + " ADD COLUMN " + calendarListSyncCursorColumn + " TEXT").Error
	}); migrationError != nil {
		return fmt.Errorf("add CalendarList sync cursor to canonical schema: %w", migrationError)
	}
	return nil
}

func migrateCalendarGroupingContract(databaseConnection *gorm.DB) error {
	hasCutoverColumn := databaseConnection.Migrator().HasColumn(&models.CalendarConnection{}, "CalendarImportCutoverAt")
	hasSemanticGroupColumn := databaseConnection.Migrator().HasColumn(&models.SourceCalendarMapping{}, "SemanticGroup")
	hasProviderGroupPredecessorColumn := databaseConnection.Migrator().HasColumn(config.TableSourceCalendarMappings, providerGroupPredecessorColumn)
	if hasCutoverColumn && hasSemanticGroupColumn && !hasProviderGroupPredecessorColumn {
		return nil
	}
	if hasCutoverColumn && !hasSemanticGroupColumn && hasProviderGroupPredecessorColumn {
		predecessorColumns, predecessorUniqueIndexes, predecessorCheckConstraints := providerGroupingPredecessorContract()
		if validationError := validateCanonicalSchemaContract(databaseConnection, predecessorColumns, predecessorUniqueIndexes, predecessorCheckConstraints); validationError != nil {
			return validationError
		}
		if migrationError := databaseConnection.Transaction(func(transaction *gorm.DB) error {
			if renameError := transaction.Exec("ALTER TABLE " + config.TableSourceCalendarMappings + " RENAME COLUMN " + providerGroupPredecessorColumn + " TO " + semanticGroupColumn).Error; renameError != nil {
				return renameError
			}
			return transaction.Exec("UPDATE " + config.TableSourceCalendarMappings + " SET sync_cursor = NULL").Error
		}); migrationError != nil {
			return fmt.Errorf("migrate provider groups to semantic groups: %w", migrationError)
		}
		return nil
	}
	if hasCutoverColumn || hasSemanticGroupColumn || hasProviderGroupPredecessorColumn {
		return fmt.Errorf("%w: calendar grouping migration is incomplete", ErrNonCanonicalDatabase)
	}

	predecessorColumns, predecessorUniqueIndexes, predecessorCheckConstraints := calendarGroupingPredecessorContract()
	if validationError := validateCanonicalSchemaContract(databaseConnection, predecessorColumns, predecessorUniqueIndexes, predecessorCheckConstraints); validationError != nil {
		return validationError
	}

	migrationError := databaseConnection.Transaction(func(transaction *gorm.DB) error {
		statements := []string{
			"ALTER TABLE " + config.TableCalendarConnections + " ADD COLUMN " + calendarImportCutoverAtColumn + " datetime",
			"ALTER TABLE " + config.TableSourceCalendarMappings + " ADD COLUMN " + semanticGroupColumn + " text NOT NULL DEFAULT 'calendar' CONSTRAINT source_calendar_group CHECK (" + semanticGroupColumn + " IN ('calendar','birthdays'))",
			"DROP INDEX " + sourceProviderCalendarIndex,
			"CREATE UNIQUE INDEX " + sourceProviderCalendarIndex + " ON " + config.TableSourceCalendarMappings + " (connection_id, provider_calendar_id, " + semanticGroupColumn + ")",
			"UPDATE " + config.TableCalendarConnections + " SET " + calendarListSyncCursorColumn + " = NULL",
			"UPDATE " + config.TableSourceCalendarMappings + " SET sync_cursor = NULL",
		}
		for _, statement := range statements {
			if executionError := transaction.Exec(statement).Error; executionError != nil {
				return executionError
			}
		}
		return nil
	})
	if migrationError != nil {
		return fmt.Errorf("migrate canonical calendar grouping contract: %w", migrationError)
	}
	return nil
}

func calendarGroupingPredecessorContract() (map[string][]string, map[string][][]string, map[string][]string) {
	predecessorColumns := cloneColumns(canonicalColumns)
	predecessorColumns[config.TableCalendarConnections] = columnsWithout(predecessorColumns[config.TableCalendarConnections], calendarImportCutoverAtColumn)
	predecessorColumns[config.TableSourceCalendarMappings] = columnsWithout(predecessorColumns[config.TableSourceCalendarMappings], semanticGroupColumn)
	predecessorUniqueIndexes := cloneUniqueIndexes(canonicalUniqueIndexes)
	predecessorUniqueIndexes[config.TableSourceCalendarMappings] = [][]string{{"connection_id", "provider_calendar_id"}, {"calendar_id"}}
	predecessorCheckConstraints := cloneCheckConstraints(canonicalCheckConstraints)
	delete(predecessorCheckConstraints, config.TableSourceCalendarMappings)
	return predecessorColumns, predecessorUniqueIndexes, predecessorCheckConstraints
}

func providerGroupingPredecessorContract() (map[string][]string, map[string][][]string, map[string][]string) {
	predecessorColumns := cloneColumns(canonicalColumns)
	predecessorColumns[config.TableSourceCalendarMappings] = columnsWithReplacement(predecessorColumns[config.TableSourceCalendarMappings], semanticGroupColumn, providerGroupPredecessorColumn)
	predecessorUniqueIndexes := cloneUniqueIndexes(canonicalUniqueIndexes)
	predecessorUniqueIndexes[config.TableSourceCalendarMappings] = [][]string{{"connection_id", "provider_calendar_id", providerGroupPredecessorColumn}, {"calendar_id"}}
	return predecessorColumns, predecessorUniqueIndexes, cloneCheckConstraints(canonicalCheckConstraints)
}

func cloneColumns(source map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(source))
	for tableName, columns := range source {
		cloned[tableName] = append([]string(nil), columns...)
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

func columnsWithout(columns []string, excludedColumn string) []string {
	filtered := make([]string, 0, len(columns)-1)
	for _, column := range columns {
		if column != excludedColumn {
			filtered = append(filtered, column)
		}
	}
	return filtered
}

func columnsWithReplacement(columns []string, oldColumn string, newColumn string) []string {
	replaced := append([]string(nil), columns...)
	for index, column := range replaced {
		if column == oldColumn {
			replaced[index] = newColumn
		}
	}
	return replaced
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
