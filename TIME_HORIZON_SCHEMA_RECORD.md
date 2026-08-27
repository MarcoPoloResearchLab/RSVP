# Time Horizon Schema Initialization Record

Record date: 2026-08-26.

## Scope

The record uses an empty SQLite database.
`services.OpenDatabase` initializes the complete canonical schema in one transaction.

The initialized schema contains 21 application tables.
SQLite foreign-key enforcement is active.

## Relationship Checks

The schema validator checks these relationship chains:

- Organizer to calendar.
- Calendar to lane.
- Lane to event series and event.
- Event to dependent event, RSVP, and venue.
- Lane to attention policy and probe.
- Lane to derived marker rule and derived marker.
- Organizer and calendar to ingestion draft.
- Ingestion draft to proposed rule and draft confirmation.
- Calendar connection to source mapping and external links.

The validator also checks canonical columns, unique indexes, and closed-value constraints.

## Reproduction

Run this command from the repository root:

```shell
go test ./pkg/services -run TestTimeHorizonSchemaInitializationRecord -v -count=1
```

The test emits one `TIME_HORIZON_SCHEMA_INITIALIZATION_RECORD` JSON value.
The record includes the table count, relationship count, foreign-key state, and validation result.

## Acceptance Result

The empty database initialized successfully.
The canonical schema validator accepted all 21 tables and their required relationships.
The runtime source scan found no schema conversion or data repair path.
