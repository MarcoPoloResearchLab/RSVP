# Time Horizon Operator Runbook

This runbook contains the current local and production operations for the RSVP time horizon.

## Configuration

`config.yml` owns the backend configuration.
The loader rejects unknown fields, extra YAML documents, absent references, and invalid values before startup.
The tracked file contains environment references for private values.

Supply these environment variables for a direct backend start:

- `APP_BASE_URL`
- `APP_WEBSITE_URL`
- `DB_NAME`
- `CALENDAR_CREDENTIAL_ENCRYPTION_KEY`
- `GOOGLE_CLIENT_ID`
- `GOOGLE_CLIENT_SECRET`
- `TAUTH_JWT_SIGNING_KEY`
- `TAUTH_COOKIE_NAME`
- `TAUTH_TENANT_ID`
- `TAUTH_PUBLIC_URL`
- `TAUTH_UPSTREAM_URL`
- `LLM_PROXY_BASE_URL`
- `LLM_PROXY_SECRET`

The LLM Proxy provider and model fields are explicitly empty.
The tenant text default controls both values.
The backend sends `low` reasoning effort and a 120-second work budget.
Keep private values out of tracked files and logs.

## Local Orchestration

1. Supply the Google client values in `.env.docker`.
2. Add `http://localhost:8080` to the Google client origins.
3. Run `make up`.
4. Open `http://localhost:8080/`.
5. Run `make down` when the local test is completed.

`make up` builds RSVP and the deterministic LLM Proxy fixture.
It starts the official TAuth container with `configs/tauth-local.yml`.

The tenant ID is `rsvp-development`.
The session and refresh cookies are `rsvp_development_session` and `rsvp_development_refresh`.
TAuth uses issuer `tauth`, host-only cookies, and HTTP for localhost.

RSVP proxies the `/auth/` paths to TAuth.
Google sign-in uses a popup exchange at `/auth/google`, not a redirect callback.
Google Calendar consent uses `/calendar-connection-callbacks/google/` as a separate callback.

Live Google sign-in and session restoration succeeded under the user account on September 29, 2026.
The existing local organizer and Calendar workspace remained available.

The stack creates private keys in `.cache/rsvp-local` on the first start.
Keep these keys with the retained database volumes.
The deterministic language fixture copies the input title into an open-lane proposal.
It does not prove live model behavior.

## Start The Backend Directly

1. Supply the environment references that `config.yml` requires.
2. Set `DB_NAME` to an empty database path or an accepted predecessor database.
3. Run `go run ./cmd/web` from the repository root.
4. Confirm that `GET /healthz` returns `200`.

RSVP uses one connection for the SQLite file.
This connection serializes database writes from HTTP requests and background tasks.
Provider HTTP requests run outside database transactions.

RSVP initializes an empty SQLite database with the complete canonical schema.
RSVP rejects an incomplete database during startup.
The B047 migration creates one provider calendar sync state for each provider calendar.
The migration clears CalendarList and event cursors for one complete reconciliation.
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

Verify `LLM_PROXY_BASE_URL` and `LLM_PROXY_SECRET`.
Verify the tenant text default in the LLM Proxy console.
Use the official client tests to inspect the native messages contract.

An invalid provider response creates no draft.
An incomplete valid response creates one incomplete draft.
The organizer must supply all missing values before confirmation.

Use the deterministic parser tests when the provider boundary changes.
Do not write input text or the tenant API key to logs.

## Production Preparation

The selected manifest defines a GitHub Pages website and a separate backend hostname.
The website is `https://rsvp.mprlab.com`.
The backend and proxied TAuth surface are `https://rsvp-api.mprlab.com`.

The proposed TAuth tenant ID is `rsvp-production`.
Its session and refresh cookies are `rsvp_production_session` and `rsvp_production_refresh`.
The tenant cookie domain is `mprlab.com`.
Production uses HTTPS and issuer `tauth`.
The manifest declares the tenant through the `tauth.tenants` capability.

The API container owns `/healthz` and protected HTML resource representations.
The static website loads its protected workspace after the shared authentication event.

The production LLM Proxy tenant `RSVP` belongs to the existing user account.
It uses the Default OpenAI connection and text model `gpt-5.6-terra` with `low` reasoning effort.
Its tenant API key is in `.cache/rsvp-production/llm-proxy-secret`.
The `/v2/identity` check returned `200` for tenant `managed-ec333400e22baad5498fee4034804d43`.
No live model request was part of this preparation.

The production TAuth console at `https://tauth.mprlab.com/app/` returned `404` on September 29, 2026.
The discovery resource at `https://tauth-api.mprlab.com/.well-known/tauth-console` also returned `404`.
Account-owned TAuth registration cannot proceed through that service yet.

Complete these operations before a production release:

1. Make the current TAuth account console available through its owning service workflow.
2. Create the RSVP App and tenant under the user account.
3. Confirm the proposed tenant ID, origins, cookie names, Google client, and issuer.
4. Export the matching tenant key through the TAuth account interface.
5. Supply the active `tauth.tenants` provisioning authority in the operator configuration.
6. Add the production website and API origins to the Google client.
7. Add the production Calendar callback URI to the Google client.
8. Supply the private assignments in the canonical `.mprlab/deploy/.env` file.
9. Preserve the encryption key for existing production calendar credentials.
10. Confirm that the public Google client in `configs/ui-production.yaml` matches the tenant.
11. Validate the committed selected manifest through the current Gateway release plan.
12. Complete release, publication, and deployment through their separate repository targets when authorized.
13. Verify the website, `/.mprlab-release.json`, API health, sign-in, and protected workspace.
14. Repeat deployment and confirm that it changes no resources.

I005 preparation did not release, publish, or deploy this source.
The production calendar encryption key remains unresolved.
Do not substitute the local key for existing production credentials.

## Preserve Data

The production service stores SQLite data in the retained `rsvp-data` volume.
Preserve that volume during a service replacement.

Use the B047 and B048 startup migrations only for their accepted predecessor databases.
Both migrations remove the predecessor calendar symbol while they create the current task contract.
For each other database schema, complete an approved one-time migration before the new runtime starts.
Remove the startup migrations after all database files use the canonical schema.
