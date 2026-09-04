package services

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/tyemirov/RSVP/pkg/config"
)

func TestTimeHorizonSchemaInitializationRecord(testingContext *testing.T) {
	databasePath := filepath.Join(testingContext.TempDir(), "production-like", "rsvp.db")
	database, openError := OpenDatabase(databasePath)
	if openError != nil {
		testingContext.Fatalf("initialize empty canonical database: %v", openError)
	}
	if schemaError := validateCanonicalSchema(database); schemaError != nil {
		testingContext.Fatalf("validate initialized schema: %v", schemaError)
	}
	if len(canonicalTableNames) != 23 {
		testingContext.Fatalf("canonical table count = %d, want 23", len(canonicalTableNames))
	}
	requiredRelationships := map[string][]canonicalForeignKey{
		config.TableCalendars:                  {{"organizer_id", config.TableUsers, "id"}},
		config.TableLanes:                      {{"calendar_id", config.TableCalendars, "id"}},
		config.TableEvents:                     {{"lane_id", config.TableLanes, "id"}, {"anchor_event_id", config.TableEvents, "id"}},
		config.TableRSVPs:                      {{"event_id", config.TableEvents, "id"}},
		config.TableAttentionPolicies:          {{"lane_id", config.TableLanes, "id"}},
		config.TableProbes:                     {{"policy_id", config.TableAttentionPolicies, "id"}, {"lane_id", config.TableLanes, "id"}},
		config.TableProviderCalendarSyncStates: {{"connection_id", config.TableCalendarConnections, "id"}},
		config.TableSourceCalendarMappings:     {{"sync_state_id", config.TableProviderCalendarSyncStates, "id"}, {"calendar_id", config.TableCalendars, "id"}},
		config.TableTasks:                      {{"organizer_id", config.TableUsers, "id"}},
		config.TableDerivedMarkers:             {{"rule_id", config.TableDerivedMarkerRules, "id"}, {"lane_id", config.TableLanes, "id"}},
		config.TableIngestionDrafts:            {{"organizer_id", config.TableUsers, "id"}, {"calendar_id", config.TableCalendars, "id"}},
		config.TableDraftDerivedMarkerRules:    {{"draft_id", config.TableIngestionDrafts, "id"}},
		config.TableDraftConfirmations:         {{"draft_id", config.TableIngestionDrafts, "id"}, {"lane_id", config.TableLanes, "id"}},
	}
	for tableName, relationships := range requiredRelationships {
		for _, relationship := range relationships {
			if !containsCanonicalForeignKey(canonicalForeignKeys[tableName], relationship) {
				testingContext.Fatalf("%s relationship is absent: %#v", tableName, relationship)
			}
		}
	}
	relationshipCount := 0
	for _, relationships := range canonicalForeignKeys {
		relationshipCount += len(relationships)
	}
	record, marshalError := json.Marshal(map[string]any{
		"database":             "empty SQLite",
		"foreign_keys_enabled": true,
		"relationship_count":   relationshipCount,
		"table_count":          len(canonicalTableNames),
		"validation":           "canonical schema accepted",
	})
	if marshalError != nil {
		testingContext.Fatalf("encode schema record: %v", marshalError)
	}
	testingContext.Logf("TIME_HORIZON_SCHEMA_INITIALIZATION_RECORD %s", record)
}

func containsCanonicalForeignKey(haystack []canonicalForeignKey, needle canonicalForeignKey) bool {
	for _, candidate := range haystack {
		if candidate == needle {
			return true
		}
	}
	return false
}
