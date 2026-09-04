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
The B047 migration creates one provider calendar sync state for each provider calendar.
The migration clears CalendarList and event cursors for one complete reconciliation.
The runtime rejects a database that does not use the current or an accepted predecessor schema.

`make up` stops the `rsvp-local` Compose project and deletes its volumes.
It preserves `.env.docker` and the calendar credential encryption key.
It then builds the services and creates a new `rsvp-data` volume.
This local reset does not change the production retained volume.

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

## Validate Account Settings

Run the organizer resource contract test:

```shell
go test ./pkg/handlers/organizer -count=1
```

Open **Settings** and select **Account**.
Save one valid IANA timezone and refresh the page.
Confirm that Settings shows the saved value.
Confirm that the default Horizon window uses the saved value.
Submit `Local` and confirm that RSVP returns a validation error.
Confirm that Settings and the database keep the prior valid value.
Confirm that an existing event keeps its marker timezone.

## Diagnose Google Calendar

Examine the connection state in **Manage horizon**.
Examine the calendar import task state in the Integrations rubric.
Examine the last synchronization state for each source calendar.

The connection request must return `202 Accepted` before provider calendar or event requests complete.
The task worker claims the pending task within its scheduler interval.
A failed attempt stores `failed`, increments `retry_count`, and uses exponential retry delay.
The task stops automatic retries after five failed attempts.
The Integrations rubric shows `Needs attention` after the last failed attempt.

A successful CalendarList reconciliation stores a cursor on the connection.
A successful event synchronization stores a cursor on the provider calendar sync state.
A later scheduled run reconciles the CalendarList before it synchronizes events.

Confirm that a complete CalendarList request includes hidden entries.
Confirm that connection creation stores the browser IANA timezone for a new organizer.
Confirm that an absent or invalid browser timezone stores `UTC`.
Confirm that each readable general entry has one RSVP calendar.
Confirm that the Contacts birthday entry creates no visible RSVP calendar.
Confirm that each provider calendar has one event sync cursor.
Confirm that each provider calendar request uses one unfiltered event feed.
Confirm that an unknown Google event type stays in its general calendar group.
Confirm that its external event link stores a provider-safe diagnostic code.
Confirm that an explicit birthday title stays in the `Birthdays` calendar when Google returns `eventType=default`.
Confirm that a changed event moves between semantic groups without a duplicate.
Confirm that a recurring exception does not split its provider series lane.
Confirm that the first complete import removes prior unmapped calendars.
Confirm that a CalendarList cursor advances only after all mapping changes commit.
Confirm that the organizer can show at most eight calendars.

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

Use the B047 and B048 startup migrations only for their accepted predecessor databases.
Both migrations remove the predecessor calendar symbol while they create the current task contract.
For each other database schema, complete an approved one-time migration before the new runtime starts.
Remove the startup migrations after all database files use the canonical schema.
