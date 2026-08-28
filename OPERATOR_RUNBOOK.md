# Time Horizon Operator Runbook

This runbook contains the current local and production operations for the RSVP time horizon.

## Required Private Values

Supply these values through the private environment:

- `APP_BASE_URL`
- `CALENDAR_CREDENTIAL_ENCRYPTION_KEY`
- `GOOGLE_CLIENT_ID`
- `GOOGLE_CLIENT_SECRET`
- `GOOGLE_OAUTH2_BASE`
- `NATURAL_LANGUAGE_PARSER_API_KEY`
- `NATURAL_LANGUAGE_PARSER_ENDPOINT`
- `SESSION_SECRET`

Set `DB_NAME` when the database path is not `rsvps.db`.
Set `TLS_CERT_PATH` and `TLS_KEY_PATH` only for direct TLS termination.

Do not put private values in tracked files or logs.
The production manifest gets each parser value from the `private` resource.

## Start The Application

1. Supply all required private values.
2. Set `DB_NAME` to an empty path or a database with a supported I006 predecessor schema.
3. Run `go run ./cmd/web`.
4. Confirm that the root health request returns `200`.

RSVP initializes an empty SQLite database with the complete canonical schema.
RSVP rejects an incomplete database during startup.
The I006 startup migrations add the CalendarList cursor, import cutover, and semantic group fields.
The grouping migration clears prior sync cursors for one complete grouped synchronization.
The next migration renames `provider_group` to `semantic_group` and clears each event sync cursor.
The runtime rejects a database that does not use the current or an accepted predecessor schema.

## Validate The Complete Capability

Run these commands from the repository root:

```shell
make ci
make browser-test
go test ./pkg/providers/googlecalendar ./pkg/providers/naturallanguage -count=1
go test ./pkg/services -run TestTimeHorizonSchemaInitializationRecord -v -count=1
```

The browser suite starts one deterministic local application and one deterministic provider boundary.
It verifies desktop and mobile views, calendar synchronization, drafts, markers, attention, QR codes, and public responses.

The schema test initializes an empty database.
It validates all canonical tables, constraints, indexes, and required relationships.

## Diagnose Google Calendar

Examine the connection state in **Manage horizon**.
Examine the last synchronization state for each source calendar.

A successful CalendarList reconciliation stores a cursor on the connection.
A successful event synchronization stores a cursor on the source calendar mapping.
A later scheduled run reconciles the CalendarList before it synchronizes events.

Confirm that a complete CalendarList request includes hidden entries.
Confirm that each readable entry has one general source mapping and one RSVP calendar.
Confirm that the primary Google calendar has a separate `Birthdays` mapping.
Confirm that the general and birthday mappings have separate event sync cursors.
Confirm that an unknown Google event type stays in its general calendar group.
Confirm that an explicit birthday title stays in the `Birthdays` calendar when Google returns `eventType=default`.
Confirm that a changed event moves between semantic groups without a duplicate.
Confirm that the first complete import removes prior unmapped calendars.
Confirm that a CalendarList cursor advances only after all mapping changes commit.

A rejected CalendarList cursor starts one complete CalendarList reconciliation.
A rejected event cursor starts one complete event reconciliation.
If local data stops source calendar deletion, RSVP records a failed synchronization.
The connection keeps its prior CalendarList cursor and retries the same change.

Use adapter tests to verify provider behavior without production credentials.
Do not write authorization codes or refresh credentials to logs.

## Diagnose Natural-Language Input

Make sure that both parser environment values are present.
Make sure that the parser endpoint accepts the current authenticated JSON contract.

An invalid provider response creates no draft.
An incomplete valid response creates one incomplete draft.
The organizer must supply all missing values before confirmation.

Use the deterministic parser tests when the provider boundary changes.
Do not write the input text or parser key to logs.

## Validate The Production Manifest

Run this command from the gateway repository:

```shell
make plan-app-release MPRLAB_APP_ROOT=/absolute/path/to/RSVP
```

The plan reads the committed application manifest.
Commit the intended manifest before this validation.

Use the separate `release`, `publish`, and `deploy` targets for their named lifecycle stages.
Treat each stage as a different result.

## Preserve Data

The production service stores SQLite data in the retained `rsvp-data` volume.
Preserve that volume during a service replacement.

Use the I006 startup migrations only for accepted predecessor databases.
For each other database schema, complete an approved one-time migration before the new runtime starts.
Remove the I006 migrations after all database files use the canonical schema.
