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
2. Set `DB_NAME` to an empty path or a canonical RSVP database.
3. Run `go run ./cmd/web`.
4. Confirm that the root health request returns `200`.

RSVP initializes an empty SQLite database with the complete canonical schema.
RSVP rejects an incomplete database during startup.
The runtime contains no schema conversion or data repair path.

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

A successful initial synchronization stores a sync cursor.
A successful later synchronization replaces that cursor.
A rejected cursor starts one complete source reconciliation.

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

Do not start a runtime with an incomplete or obsolete database.
Use an approved one-time migration before the new runtime starts.
Remove the migration bridge after the migration completes.
