# ISSUES

Entries record newly discovered requests or changes.

Read `AGENTS.md`, `.mprlab/POLICY.md`, `.mprlab/issues-md-format.md`, and relevant stack guides before implementing changes.

Format: `- [ ] [B042] (P1) {I007} Title`

## BugFixes

- [x] [B051] (P1) {I013,F002} Make the Horizon window match the selected scale
  Goal:
  The scale controls change the day width but keep the 90-day Horizon window.
  The selected scale must control the visible time window.
  Requirements:
  - Use one local day for the day scale.
  - Use seven local days for the week scale.
  - Use one calendar month for the month scale.
  - Use one calendar year for the year scale.
  - Use the month scale when the browser has no stored scale.
  - Store the selected scale as a browser preference.
  - Use the stored scale for each later Horizon load.
  - Move the window by one selected scale when the user pans.
  - Render time ticks that agree with the selected scale.
  - Keep explicit projection windows available for the JSON representation.
  Validation:
  - Select each scale and confirm the exact visible time window.
  - Reload the Horizon and confirm that the last scale stays selected.
  - Pan in each direction and confirm one selected-scale movement.
  - Confirm that only the selected scale has the active state.
  - Run `make ci` and `make browser-test`.

- [x] [B050] (P1) {B049,F002} End each finite lane at its final marker
  Goal:
  A finite lane can extend past its final marker and show a second visual endpoint.
  The terminal circle is a visual part of the lane and must end the rail.
  Requirements:
  - Keep the stored lane end time unchanged.
  - End the visible rail at the final marker when the stored lane end is in the horizon window.
  - Render the terminal circle as part of the finite lane.
  - Use the final marker only as the interaction target for the terminal circle.
  - Align the rail end with the marker center at a horizon window boundary.
  - Do not render a separate marker circle at the lane end.
  - Keep the stored lane end when the horizon window contains no marker.
  - Keep the continuation arrow when the stored lane end is after the horizon window.
  Validation:
  - Confirm that the rail does not extend past the final marker.
  - Confirm that the lane renders one terminal circle.
  - Confirm that the final marker has no separate marker circle.
  - Confirm the boundary alignment with a browser test.
  - Run `make ci` and `make browser-test`.

- [x] [B049] (P1) {B046,F002} Correct the lane design
  Goal:
  The horizon view uses a thin lane rail and a narrow finite endpoint.
  The horizon view also cuts long lane titles at the fixed label boundary.
  Requirements:
  - Use a 16-pixel lane rail.
  - Use a round finite endpoint with the same size as the lane rail.
  - Mask the finite endpoint behind a marker at the same position.
  - Make the lane label width responsive.
  - Wrap a long lane title inside its lane label.
  - Keep the calendar symbol and lane management control visible.
  - Keep the sticky lane label separate from the time track.
  - Keep the Today label inside the time track at each window boundary.
  Validation:
  - Confirm that the lane rail is 16 pixels high.
  - Confirm that the finite endpoint has the same width and height as the lane rail.
  - Render a long event title at desktop and mobile widths.
  - Confirm that the complete title is visible.
  - Confirm that the lane management control stays inside the lane label.
  - Confirm that the time rail keeps its date positions.
  - Run `make ci` and `make browser-test`.

- [x] [B048] (P1) {B047,I006} Run calendar connection work as a background task
  Goal:
  Calendar connection creation must return before RSVP imports source calendars and events.
  The current request imports all provider data before it returns and makes the connection control appear inactive.
  Requirements:
  - Store one durable task for each initial calendar connection import.
  - Use the `tyemirov/utils` scheduler for task claims, retries, and attempt results.
  - Keep provider credentials out of the task payload.
  - Return the connection and task state without waiting for calendar import.
  - Show the task state on the consent callback page.
  - Show the task state in the Integrations rubric.
  - Poll the connection resource while the initial task is active.
  - Keep scheduled calendar synchronization separate from the initial task.
  Deliverables:
  - Add the provider-neutral task model and canonical database contract.
  - Add the scheduler repository and calendar import dispatcher.
  - Add the task state to the calendar connection representation.
  - Start the task worker with the application runtime.
  - Replace the callback wait cursor with an explicit task status.
  - Update the architecture, API, acceptance matrix, user guide, and operator runbook.
  - Add model, service, HTTP, and browser contract tests.
  Validation:
  - Create a connection while the provider event request remains blocked.
  - Confirm that the connection request returns with an active task.
  - Confirm that the callback page reports the background import.
  - Confirm that Settings reports the task state.
  - Release the provider request and confirm that the task succeeds.
  - Return a provider error and confirm a scheduled retry.
  - Run `make ci` and `make browser-test`.
  - Run the Governor check and the language checker for each changed technical document.

- [x] [B047] (P1) {I006} Correct Google calendar import semantics
  Goal:
  Google calendar import must preserve the organizer's source calendar groupings and normalized birthday meaning.
  RSVP currently misclassifies Google events and can retain obsolete local groupings between local starts.
  Requirements:
  - Treat each readable CalendarList entry as one provider calendar.
  - Include hidden and unselected entries during CalendarList synchronization.
  - Use `selected` only as the initial RSVP visibility value.
  - Include the browser IANA timezone in the connection confirmation request.
  - Use `UTC` when the browser timezone is absent or invalid.
  - Confirm the organizer timezone in the connection creation transaction.
  - Use `summaryOverride` before `summary` for the initial RSVP name.
  - Keep a source rename separate from RSVP group identity.
  - Exclude `freeBusyReader` entries because they do not supply event details.
  - Treat a deleted CalendarList entry as a provider calendar deletion.
  - Treat the Google Contacts birthday entry as a semantic source, not a visible source calendar.
  - Create `Birthdays` from normalized birthday meaning.
  - Do not use a provider name, event type, or RSVP source as a semantic group.
  - Preserve Holidays, Family, and other readable entries as separate RSVP calendars.
  - Give each visible RSVP calendar one distinct presentation color.
  - Derive each presentation color only from the calendar identifier and color token.
  - Keep each presentation color when visibility or the calendar set changes.
  - Allow at most eight visible calendars for one organizer.

  - Map `birthdayProperties.type=self` to `Birthdays`.
  - Map `birthdayProperties.type=birthday` to `Birthdays`.
  - Map absent `birthdayProperties.type` to `Birthdays` when `birthdayProperties` exists.
  - Keep `anniversary`, `custom`, and `other` subtypes in the source calendar.
  - Do not use `birthdayProperties.contact` as a grouping requirement.
  - Do not use `customTypeName` as an RSVP group name.
  - Map `eventType=birthday` without birthday properties to `Birthdays`.
  - Map `eventType=default` to the provider calendar by default.
  - Use complete title words as a fallback only for an untyped `default` event.
  - Map `birthday`, `birthdays`, and `bday` title words to `Birthdays`.
  - Do not match a partial word such as `Unbirthday`.
  - Keep `focusTime`, `fromGmail`, `outOfOffice`, and `workingLocation` in the provider calendar.
  - Keep an unknown `eventType` or birthday subtype in the provider calendar.
  - Record a provider-safe diagnostic code for each unknown value.
  - Use `Busy` when Google withholds event details.
  - Apply available type metadata before the `Busy` title.
  - Keep all occurrences of one recurring event in the same semantic group.
  - Keep each occurrence classification for recurring-series reconciliation.
  - Do not derive a semantic group from recurrence, date shape, visibility, or transparency.

  - Evaluate grouping rules in one ordered Google classification table.
  - Apply cancellation before all classification rules.
  - Apply an explicit special-date subtype before `eventType`.
  - Apply `eventType` before the title fallback.
  - Apply the provider-calendar default after all other rules.
  - Return one normalized event change from the provider adapter.
  - Include the target semantic group in each normalized event change.
  - Keep Google fields out of the synchronization service and database models.
  - Request one unfiltered event feed for each provider calendar.
  - Do not request one duplicate feed for each semantic group.
  - Store one event sync cursor for each provider calendar.
  - Keep each initial and incremental request parameter set consistent.
  - Store `nextSyncToken` only after the final page and successful persistence.
  - On `410 Gone`, clear the provider calendar state and complete a full reconciliation.

  - Treat a canceled event as a deletion across all semantic group mappings.
  - Use provider calendar ID and provider event ID as the event source identity.
  - Do not require a canceled record to contain title or birthday metadata.
  - Move a changed event between semantic groups in one database transaction.
  - Remove the prior event placement before the transaction stores the new placement.
  - Keep one external event link after a semantic group move.
  - Remove absent source-owned events during a complete reconciliation.
  - Keep provider copies from different source calendars as distinct source events.

  - Stop the `rsvp-local` Compose project before each `make up` startup.
  - Delete the local `rsvp-data` volume before each `make up` startup.
  - Preserve the local environment file and calendar credential encryption key.
  - Keep the production retained volume unchanged.
  - Build and start the local services after the reset.

  Deliverables:
  - Add one closed provider-neutral semantic group type.
  - Add one normalized provider event change type with source identity and semantic group.
  - Remove the semantic group argument from `CalendarProviderAdapter.SynchronizeEvents`.
  - Add one provider calendar sync-state model that owns the event cursor.
  - Make each semantic group mapping reference that sync-state model.
  - Make external event and series links use provider calendar source identity.
  - Replace group-specific synchronization with one source-calendar reconciliation transaction.
  - Add one declarative Google classification table with explicit precedence.
  - Add one table-driven contract matrix for all documented Google event types.
  - Add cases for every documented birthday subtype and each missing-field form.
  - Add cancellation, semantic move, unknown-value, and complete-reconciliation cases.
  - Add browser acceptance for personal, Birthdays, Holidays, Family, and unknown provider types.
  - Add browser acceptance for stable colors with eight visible calendars and one hidden calendar.
  - Add connection acceptance for an organizer without a confirmed timezone.
  - Add connection acceptance for the `UTC` fallback.
  - Update the architecture, acceptance matrix, user guide, and operator runbook.
  - Use the official Google Event types, Events, CalendarList, and synchronization documents as source evidence.
  Validation:
  - Run the classification matrix without network access.
  - Run one deterministic CalendarList and Events API server.
  - Synchronize one provider calendar and confirm that RSVP makes one Events request.
  - Return a Google self-birthday event and confirm that RSVP puts it only in `Birthdays`.
  - Return a contact birthday and confirm that RSVP puts it only in `Birthdays`.
  - Return an anniversary and confirm that RSVP keeps it only in the source calendar.
  - Confirm that the browser shows `Happy birthday!` only in `Birthdays`.
  - Change a default title to an explicit birthday title and confirm one atomic move.
  - Change the title back and confirm the reverse move.
  - Cancel each event form with a sparse canceled record and confirm complete deletion.
  - Reject a sync cursor and confirm one complete source reconciliation.
  - Confirm that a repeated synchronization creates no duplicate calendar, lane, event, or link.
  - Hide one calendar and confirm that each assigned color stays the same.
  - Add one calendar and confirm that each prior assigned color stays the same.
  - Run `make up` with an existing local database.
  - Confirm that Docker creates a new local `rsvp-data` volume.
  - Confirm that the new local database contains no organizer data.
  - Confirm that the local health request succeeds.
  - Run `make ci` and `make browser-test`.
  - Run the Governor check and the language checker for each changed technical document.

- [x] [B046] (P1) {F002} Refine lane and marker geometry
  Goal:
  The current finite lane cap overlaps the event marker and creates a clunky endpoint.
  The lane rail and marker border also use too much visual weight.
  Requirements:
  - Keep the lane trajectory visible across the time grid.
  - Use a narrow terminal tick for a finite lane.
  - Mask the terminal tick behind a marker at the same position.
  - Reduce the visible event marker size and border weight.
  - Preserve a minimum 44-pixel marker interaction target.
  - Keep selected, focus, pending, and missed marker states distinct.
  Validation:
  - Confirm that the lane rail height is between 8 and 12 pixels.
  - Confirm that the event marker is between 16 and 20 pixels.
  - Confirm that the finite terminal tick is no more than 4 pixels wide.
  - Confirm that each marker interaction target is at least 44 pixels.
  - Run `make ci` and `make browser-test`.

- [x] [B045] (P1) {I006} Correct imported calendar groupings
  Goal:
  Google Calendar import must replace prior local groupings with distinct semantic calendar groupings.
  Google currently labels some explicit birthday events as `default` events without birthday properties.
  RSVP currently puts these events in the primary calendar instead of `Birthdays`.
  Requirements:
  - Remove all prior unmapped calendars and their temporal resources during the first complete import.
  - Preserve calendars that already have a source calendar mapping during the I006 data migration.
  - Use `Personal` as the local default calendar name.
  - Normalize Google API calendar and event values into RSVP calendar groups at the provider boundary.
  - Use `calendar` and `birthdays` as the current closed semantic group keys.
  - Map Google birthday metadata to a separate `Birthdays` calendar.
  - Map the complete title words `birthday`, `birthdays`, and `bday` to `Birthdays`.
  - Do not classify a title from a partial word match.
  - Request the same Google event feed without an event-type filter for each semantic group.
  - Remove an event from its prior semantic group when its meaning changes.
  - Keep the general calendar and birthday group cursors separate.
  - Keep unknown or non-semantic Google event types in the general calendar group.
  - Do not use a raw Google event type as an RSVP calendar name.
  - Use the calendar meaning for each visible symbol and name.
  - Do not create a visible grouping for the RSVP or Google source.
  - Give each calendar one stable presentation color.
  - Clear prior Google sync cursors during the schema migration.
  - Store the source mapping group in `semantic_group`.
  - Rename the exact `provider_group` predecessor column to `semantic_group`.
  Validation:
  - Import a Google account after a local calendar exists and confirm that RSVP removes the local calendar.
  - Confirm that RSVP imports birthday events into `Birthdays`.
  - Confirm that RSVP excludes birthday events from the primary calendar.
  - Return a `default` event with an explicit birthday title and confirm that it appears only in `Birthdays`.
  - Change a general event title to a birthday title and confirm that the event moves to `Birthdays`.
  - Return `Unbirthday` in a title and confirm that the event stays in the general group.
  - Return an unknown Google event type and confirm that it stays in the general calendar group.
  - Confirm that all visible calendar colors are unique.
  - Confirm that calendar visibility changes do not change another calendar color.
  - Migrate each I006 predecessor and confirm that RSVP preserves mapped calendars and clears event sync cursors.
  - Run `make ci` and `make browser-test`.
  - Run the Governor check and the language checker for each changed technical document.

- [x] [B044] (P1) {I003} Use one REST interface for Horizon resources
  Goal:
  Horizon must use the repository REST contract for reads, writes, and errors.
  Requirements:
  - Support safe `HEAD` requests for the Horizon projection.
  - Return one typed error shape for Horizon API failures.
  - Use standard HTTP methods without method override on Horizon resources.
  - Create quick and natural-language drafts through `/ingestion-drafts/`.
  - Keep the legacy event, RSVP, and venue pages outside the Horizon API surface.
  Validation:
  - Do a test of methods, headers, representations, and typed errors through HTTP.
  - Do a test of both ingestion-draft creation representations.
  - Run `make ci` and `make browser-test`.

- [x] [B043] (P1) {I003} Show Horizon setup after first authentication
  Goal:
  A new organizer must see an HTML setup state instead of an invalid time-window response.
  Requirements:
  - Use the browser IANA timezone for the first temporal write.
  - Create the first calendar before RSVP calculates the default Horizon window.
  - Return an explicit organizer-timezone error for a JSON Horizon request.
  Validation:
  - Do a test of the HTML setup state and the JSON error contract.
  - Complete the setup in a browser and confirm that Horizon renders.
  - Run `make ci`.

## Improvements

- [x] [I001] (P0) Add RSVP contract tests
  Goal:
  This issue adds test evidence for current RSVP behavior before the temporal schema replacement.

  Requirements:
  - Add isolated SQLite test fixtures that do not share state.
  - Do a test of owner checks for events, venues, and RSVPs.
  - Do a test of event and venue transactions.
  - Do a test of the public RSVP response route and response update.
  - Do a test of database initialization with an empty database.
  - Do a test of the current QR code URL contract.
  - Keep all tests deterministic and independent.

  Deliverables:
  - Add Go tests for handlers, models, and database services.
  - Add reusable test builders for users, events, venues, and RSVPs.
  - Keep `make test` as the canonical test command.

  Validation:
  - Run `make ci`.
  - Run `go test -count=1 ./...`.
  - Confirm that named tests verify each specified behavior.

- [x] [I002] (P0) {P001,I001} Replace events with calendar lanes
  Goal:
  This issue replaces the event-only temporal schema with the approved calendar and lane schema.

  Requirements:
  - Add calendar, lane, event series, probe, and attention policy domain types.
  - Add one required organizer timezone with an IANA timezone name.
  - The client must supply each timezone default.
  - For a new organizer, store the client timezone with the first temporal write.
  - Represent an open lane with no end time.
  - Add a required `lane_id` field to each event.
  - Start each lane when RSVP starts to track its temporal subject.
  - Keep each marker within its lane bounds.
  - Store each timestamp with explicit IANA timezone context.
  - Enforce valid lane, event, probe, and attention policy states at construction.
  - Give each independent event one lane.
  - Put all occurrences of one event series on one lane.
  - Permit distinct events on one lane only through an explicit dependency chain.
  - Put each probe on the lane for the related attention policy.
  - Enforce ownership through the calendar and lane relationships.
  - Remove the duplicate event owner field from the canonical schema.
  - Create one `RSVP Events` calendar with the first temporal write.
  - Create one finite lane for each independent event.
  - End each finite event lane at the event end time.
  - Initialize only the complete canonical schema in an empty database.
  - Reject an incomplete or event-only database at startup.
  - Keep the runtime free of schema conversion and data repair paths.

  Deliverables:
  - Add the canonical GORM models and database constraints.
  - Add tested canonical schema initialization and validation.
  - Update all ownership queries for the canonical relationship chain.

  Validation:
  - Initialize a fresh fixture database with the complete canonical schema.
  - Reject an event-only fixture database.
  - Confirm that invalid open and finite lane states fail.
  - Confirm that an event without a lane fails.
  - Confirm that two independent events in one calendar use different lanes.
  - Confirm that dependent events use the anchor event lane.
  - Confirm that all occurrences of one event series use one lane.
  - Make sure that an absent client timezone stops the temporal write.
  - Run `make ci`.

- [x] [I003] (P1) {F004,F006,F007,F009} Complete time horizon acceptance
  Goal:
  This issue supplies integrated acceptance evidence for the complete time horizon capability.

  Requirements:
  - Create representative data for independent events, event series, dependency chains, open lanes, and calendars.
  - Initialize a production-like SQLite fixture with the canonical schema.
  - Do a test of the horizon view on desktop and mobile browser sizes.
  - Verify the current QR code and public RSVP response flows.
  - Verify initial and incremental Google Calendar synchronization.
  - Verify attention policies, probes, derived markers, and ingestion drafts.
  - Review keyboard access, focus order, labels, and color-independent meaning.
  - Remove obsolete temporal code paths and unused interface assets.
  - Update the architecture document, user guide, and operator runbook.

  Deliverables:
  - Add one deterministic end-to-end acceptance suite.
  - Add one schema initialization record with relationship checks.
  - Add current technical documentation for the complete capability.

  Validation:
  - Run `make ci`.
  - Run the complete browser test target.
  - Run the adapter tests with the deterministic test server.
  - Run schema initialization with an empty database.
  - Run the documentation language checker on each changed technical document.
  - Make sure that the repository contains only the canonical temporal schema and runtime paths.

- [ ] [I004] (P1) {B044,I005} Use one REST contract for all HTTP resources
  Goal:
  This change gives each RSVP API resource one current REST contract.
  It defines separate contracts for HTML documents, static assets, and OAuth protocol endpoints.
  Requirements:
  - Classify each registered route as an API resource, HTML document, static asset, or external protocol endpoint.
  - Use `GET` and `POST` on `/events/`.
  - Use `GET`, `HEAD`, `PATCH`, and `DELETE` on `/events/{event_id}`.
  - Use `GET` and `POST` on `/venues/`.
  - Use `GET`, `HEAD`, `PATCH`, and `DELETE` on `/venues/{venue_id}`.
  - Use `GET` and `POST` on `/events/{event_id}/rsvps/`.
  - Use `GET`, `HEAD`, `PATCH`, and `DELETE` on `/rsvps/{rsvp_id}`.
  - Use `GET`, `HEAD`, and `PUT` on `/rsvp-responses/{response_id}`.
  - Use `GET` and `HEAD` on `/rsvps/{rsvp_id}/qr-code`.
  - Keep each B044 Horizon path as the current resource path.
  - Remove `/response/`, `/response/thankyou`, and `/rsvps/qr/`.
  - Remove resource identity from query parameters and form payloads.
  - Delete the HTTP method override middleware, constants, form fields, and tests.
  - Use browser `fetch` requests for each JSON mutation.
  - Use content negotiation when a resource has HTML and JSON representations.
  - Return `Vary: Accept` for each negotiated representation.
  - Return the `GET` metadata without a body for each `HEAD` request.
  - Return the supported methods for each `OPTIONS` request.
  - Return `405 Method Not Allowed` with `Allow` for each unsupported method.
  - Return one typed error shape for all API errors.
  - Reject each unsupported request media type.
  - Honor `Accept` for each resource representation.
  - Return `201 Created` with `Location` for each synchronous creation.
  - Require `Idempotency-Key` for each retry-sensitive creation.
  - Return the same result for each valid idempotent retry.
  - Return an `ETag` for each mutable resource representation.
  - Require `If-Match` for each concurrent `PATCH`, `PUT`, or `DELETE` operation.
  - Return `428 Precondition Required` when a required precondition is absent.
  - Return `412 Precondition Failed` when a resource version changed.
  - Define `Cache-Control` for each API, document, asset, and protocol response.
  - Use one bounded cursor contract for each collection that can increase without a fixed limit.
  - Serialize each timestamp in UTC with the RFC 3339 format.
  - Propagate request cancellation and deadlines to each database and provider operation.
  - Remove obsolete routes without aliases, redirects, or dual handlers.
  - Change the GAuss authentication routes in the GAuss repository.
  - Use `GET` and `HEAD` for the `/login` HTML document.
  - Use `POST /authentication-attempts/` to create OAuth authorization state.
  - Use `GET /oauth-callbacks/google/` only as an OAuth protocol endpoint.
  - Use `DELETE /sessions/current` to delete the current session.
  - Apply `Cache-Control: no-store` to authentication and OAuth protocol responses.
  - Remove `/auth/google`, `/auth/google/callback`, and `/logout` after the GAuss update.
  - Consume one released GAuss version without an RSVP compatibility adapter.
  - Document the exact local and production Google redirect URIs.
  Deliverables:
  - Add one complete OpenAPI schema for all RSVP API resources.
  - Add one route classification table to the architecture document.
  - Centralize route paths, operation identifiers, schemas, headers, and error codes.
  - Update each API handler and each repository-owned browser client.
  - Release the GAuss route contract from its owning repository.
  - Update RSVP to use the released GAuss dependency.
  - Delete all obsolete route code and interface assets.
  - Add registered HTTP contract tests for the complete route table.
  - Add browser tests for organizer and public RSVP operations.
  Validation:
  - Compare all registered API routes with the OpenAPI schema.
  - Compare all other registered routes with the route classification table.
  - Do a test of methods, identifiers, status codes, headers, bodies, and state changes through a real HTTP listener.
  - Do a test of `HEAD` metadata and the empty response body for each readable resource.
  - Do a test of `OPTIONS` and `Allow` for each API resource.
  - Do a test of typed errors for each API error class.
  - Do a test of valid retries with the same idempotency key.
  - Do a test of stale writes with two resource versions.
  - Do a test of each bounded cursor and its documented order.
  - Confirm that each obsolete path returns `404 Not Found`.
  - Do a test of the event, venue, RSVP, public response, and QR code flows in a browser.
  - Verify local Google sign-in through the configured OAuth callback.
  - Run `make ci` and `make browser-test`.
  - Run the Governor check and the language checker for each changed technical document.

- [!] [I005] (P1) Use `mpr-ui`, TAuth, LLM Proxy, and MPR styling
  Goal:
  This change makes RSVP use the shared MPR Lab browser, authentication, language, style, and deployment contracts.
  It preserves temporal resources, Google Calendar behavior, and the Horizon task flow.
  Requirements:
  - Keep the Go backend, SQLite database, temporal resource contracts, and Horizon information architecture.
  - Create one canonical backend `config.yml`.
  - Move server, database, TAuth, Google Calendar, and LLM Proxy config into `config.yml`.
  - Store only secret references in tracked config.
  - Store secret values in the current private input channels.
  - Serve `/config-ui.yaml` as the only browser authentication input.
  - Use `mpr-ui@latest` through literal URLs for every MPR Lab browser library.
  - Render `<mpr-header>`, `<mpr-user>`, and `<mpr-footer>` in the shared layout.
  - Request protected RSVP data only after `mpr-ui:auth:authenticated`.
  - Clear app-owned browser data after `mpr-ui:auth:unauthenticated`.
  - Let TAuth own Google sign-in, session restoration, refresh, logout, and authentication cookies.
  - Use the published TAuth validator to authorize protected RSVP resources.
  - Use issuer `tauth` and exact environment-specific cookie names.
  - Keep local TAuth traffic on `http://localhost:8080`.
  - Keep production TAuth traffic on `https://rsvp.mprlab.com`.
  - Define the tenant ID, cookie names, auth paths, callback URL, TLS, and cookie policy for each environment.
  - Remove GAuss and all RSVP-owned login, session, cookie, refresh, and logout code.
  - Keep Google Calendar consent separate from browser sign-in.
  - Add a `tauth_tenant` resource through the active `tauth.tenants` capability.
  - Use the current gateway capability contract for TAuth and LLM Proxy endpoints.
  - Resolve `github.com/tyemirov/llm-proxy/pkg/llmproxyclient@latest`.
  - Use one startup-owned client with `NewConfig`, `NewClient`, `NewMessagesRequest`, and `PostMessages`.
  - Add one `llm_proxy` block with every required current field.
  - Keep prompt construction, response interpretation, persistence, and product policy in RSVP.
  - Map the request work budget to `MessagesRequestInput.RequestTimeoutSeconds`.
  - Remove the custom natural-language parser HTTP adapter and its private values.
  - Apply the MPR neutral and semantic tokens to all RSVP browser pages.
  - Use centered 960-pixel surfaces and the expanded 1180-pixel Horizon surface.
  - Use compact controls, thin borders, restrained spacing, semantic chips, and subtle motion.
  - Remove Bootstrap, Montserrat, light dashboard surfaces, large soft cards, and decorative styles.
  - Preserve form meaning, task order, calendar lanes, and settings placement.
  - Keep local Compose and the current gateway lifecycle as separate contracts.
  - Keep `.mprlab/deploy/resources.yml` versionless with `owner`, `release`, and `resources`.
  - Remove obsolete code without compatibility paths or dual authentication.
  - Migrate existing organizer ownership once if the TAuth identity shape requires it.
  - Delete the identity migration after the data cutover.
  - Update I004 before implementation to remove all GAuss migration requirements.
  Deliverables:
  - Add exact local and production RSVP configuration for TAuth and `mpr-ui`.
  - Add the canonical backend config schema, loader, validation, and startup composition.
  - Add the shared `mpr-ui` layout and the MPR-styled RSVP browser frontend.
  - Add the official LLM Proxy client adapter for natural-language ingestion.
  - Add current TAuth, LLM Proxy, private-value, route, runtime, and health resources to the selected manifest.
  - Add deterministic TAuth, LLM Proxy, browser, and deployment contract tests.
  - Update the architecture document, user guide, operator runbook, and I004 scope.
  - Delete GAuss, Bootstrap, the custom parser transport, and obsolete configuration assets.
  Validation:
  - Confirm that `/config-ui.yaml` contains only documented browser-facing values.
  - Confirm that the browser sends no protected request before `mpr-ui:auth:authenticated`.
  - Confirm that each protected route returns `401` without a valid TAuth session.
  - Complete Google sign-in through the shared `mpr-ui` and TAuth flow.
  - Confirm that the browser contains no app-owned authentication state or private values.
  - Confirm that each MPR Lab library reference uses the literal `@latest` tag.
  - Capture the official LLM Proxy request through a local deterministic server.
  - Verify authentication, provider, model, reasoning effort, work budget, response, and error propagation.
  - Do a test of natural-language ingestion through the official client boundary.
  - Do a test of Google Calendar consent after the authentication migration.
  - Do a test of each RSVP page at the supported desktop and mobile widths.
  - Confirm that the visual result uses the MPR tokens and preserves product meaning.
  - Run `make ci` and `make browser-test`.
  - Run the current gateway selected-manifest validation for RSVP.
  - Record user-owned test-host acceptance through the canonical release, publish, and deploy targets.
  - Repeat the same test-host deployment and confirm that it changes no resources.
  - Verify the public frontend, authentication surface, authorization boundary, and protected workspace separately.
  - Run the Governor check and the language checker for each changed technical document.
  Blocked: Approve the production TAuth configuration and LLM Proxy routing values before implementation.

- [x] [I006] (P1) {F005,F006} Preserve Google source calendars during import
  Goal:
  This change preserves each Google source calendar as one RSVP calendar during connection and synchronization.
  The current browser creates mappings only after manual source selection.
  Background synchronization processes existing mappings and cannot discover other source calendars.
  Requirements:
  - Treat each non-deleted CalendarList entry with event read access as one source calendar.
  - Request hidden CalendarList entries during each complete source calendar synchronization.
  - Store one CalendarList sync cursor for each calendar connection.
  - Store the cursor only after all source calendar mapping changes commit.
  - Use `summaryOverride` when present for the RSVP calendar name.
  - Use `summary` when `summaryOverride` is absent.
  - Create one RSVP calendar and one source calendar mapping for each new provider calendar.
  - Use Google `selected` as the initial RSVP calendar visibility value.
  - Use Google `backgroundColor` as the initial RSVP calendar color token.
  - Preserve the RSVP symbol, color token, visibility, and display order after initial creation.
  - Update the RSVP calendar name when the source calendar name changes.
  - Synchronize each source calendar mapping into its related RSVP calendar.
  - Keep calendar membership separate from lane membership.
  - Reconcile the source calendar list before each background event synchronization.
  - When Google rejects a CalendarList sync cursor, do a complete source calendar synchronization.
  - Apply the current local-use protection rules when Google deletes a source calendar.
  - Record a source synchronization error when local use prevents source calendar deletion.
  - Remove the manual source calendar selection controls and request contract.
  - Keep the current Google Calendar read-only scopes.
  Deliverables:
  - Make `CalendarProviderAdapter.ListCalendars` accept a cursor and return a `ProviderCalendarBatch`.
  - Add CalendarList pages, hidden entries, deletions, and sync cursors to `pkg/providers/googlecalendar/adapter.go`.
  - Add the CalendarList sync cursor to `CalendarConnection` and the canonical database contract.
  - Replace `ReplaceSourceCalendars` with `ReconcileSourceCalendars` in the calendar connection service.
  - Run `ReconcileSourceCalendars` during connection creation and scheduled synchronization.
  - Delete the manual source selection `PUT` operation.
  - Delete the source calendar selection interface and its browser code.
  - Update `ARCHITECTURE.md`, `TIME_HORIZON_ACCEPTANCE.md`, `USER_GUIDE.md`, and `OPERATOR_RUNBOOK.md`.
  - Add adapter, service, HTTP, and browser contract tests.
  Validation:
  - Connect an account that contains Birthdays, Holidays, and Family calendars.
  - Confirm that RSVP creates three source calendar mappings and three RSVP calendars.
  - Confirm that each imported marker belongs to the RSVP calendar for its source calendar.
  - Confirm that independent events use separate lanes inside each imported calendar.
  - Confirm that occurrences from one event series use one lane.
  - Repeat source calendar reconciliation and confirm that it creates no duplicate resource.
  - Add a Google source calendar and confirm that scheduled synchronization creates its RSVP calendar.
  - Rename a Google source calendar and confirm that RSVP keeps the mapping identifier.
  - Import a source calendar with `selected=false` and confirm its initial RSVP visibility value.
  - Reject a CalendarList sync cursor and confirm a complete reconciliation.
  - Remove a source calendar and confirm the current local-use protection result.
  - Confirm that connection completion requires no source selection step.
  - Run `make ci` and `make browser-test`.
  - Run the Governor check and the language checker for each changed technical document.

- [x] [I008] (P1) {F002} Remove the Horizon tagline
  Goal:
  The Horizon heading uses only the product title.
  The `Your time, in motion` tagline does not supply necessary interface information.
  Requirements:
  - Remove `Your time, in motion` from the configured Horizon view.
  - Remove `Your time, in motion` from the Horizon setup view.
  - Keep `Horizon` as the configured view heading.
  - Keep `Set up Horizon` as the setup view heading.
  - Preserve the accessible section name in both views.
  - Remove the obsolete tagline style when no element uses it.
  Deliverables:
  - Update `templates/horizon.tmpl` and `static/horizon.css`.
  - Update the Horizon HTML and browser contract tests.
  Validation:
  - Open the configured Horizon view and confirm that the tagline is absent.
  - Open the Horizon setup view and confirm that the tagline is absent.
  - Confirm that each view has one visible level-one heading.
  - Confirm that the removed element leaves no empty space above the heading.
  - Run `make ci` and `make browser-test`.

- [x] [I009] (P1) {F002} Put Horizon controls on the time window row
  Goal:
  The time window and its four view controls need one horizontal action row.
  Requirements:
  - Put the window start, window end, and organizer timezone in one time window group.
  - Put the pan and scale control group on the same row.
  - Keep the control order as backward pan, scale out, scale in, and forward pan.
  - Keep each current button label and keyboard operation.
  - Keep the complete time window text available to assistive technology.
  - Prevent the time window text from overlapping the controls.
  - Keep the row usable at each supported browser width.
  Deliverables:
  - Add one time window row to `templates/horizon.tmpl`.
  - Update the row and control layout in `static/horizon.css`.
  - Update the Horizon HTML and browser contract tests.
  Validation:
  - Confirm that the time window group and control group share one row at the supported desktop width.
  - Confirm that all four controls remain visible at the supported mobile width.
  - Confirm that the row shows the complete start, end, and timezone values.
  - Use each button and confirm the current pan or scale result.
  - Use each keyboard operation and confirm the same result.
  - Run `make ci` and `make browser-test`.

- [x] [I010] (P1) {I009} Put the time window row above the Horizon timeline
  Goal:
  The time window row belongs directly above the timeline that uses its range.
  Requirements:
  - Render the time window row after the Quick add interface.
  - Render the time window row immediately before the Horizon timeline viewport.
  - Keep the calendar visibility controls and Quick add interface above the time window row.
  - Remove the time window row from the Horizon heading.
  - Keep the document order and visible order the same.
  - Keep each control in the existing keyboard focus order.
  - Prevent a visible element from separating the row and timeline.
  Deliverables:
  - Move the time window row in `templates/horizon.tmpl`.
  - Update the related spacing rules in `static/horizon.css`.
  - Add HTML order and browser position coverage.
  Validation:
  - Confirm that Quick add appears before the time window row.
  - Confirm that the time window row appears directly above the timeline border.
  - Confirm that the heading contains only heading content.
  - Use the controls from the new position and confirm the current results.
  - Confirm the same order at the supported desktop and mobile widths.
  - Run `make ci` and `make browser-test`.

- [x] [I011] (P1) {F002,F011} Move the keyboard instructions to Help
  Goal:
  Keyboard instructions belong in Help instead of the primary Horizon workspace.
  Requirements:
  - Add a Help rubric to the Settings dialog.
  - Add one Help panel for the Horizon keyboard instructions.
  - List the pan, scale, marker selection, and calendar visibility operations.
  - Remove the visible keyboard instruction paragraph from the Horizon workspace.
  - Remove the obsolete instruction style when no element uses it.
  - Preserve all current keyboard operations.
  - Support `#settings/help` as the direct Help location.
  - Keep the Help rubric in the Settings keyboard navigation order.
  Deliverables:
  - Update `templates/partials/settings-dialog.tmpl` and `templates/horizon.tmpl`.
  - Update the applicable Horizon and Settings styles.
  - Add Settings rubric, direct location, and keyboard instruction browser coverage.
  Validation:
  - Confirm that no keyboard instruction paragraph appears above the timeline.
  - Open Help and confirm that it lists each current Horizon keyboard operation.
  - Open `#settings/help` and confirm that the Help rubric is active.
  - Navigate to Help with the Settings arrow keys.
  - Use each documented keyboard operation in Horizon.
  - Run `make ci` and `make browser-test`.

- [x] [I012] (P1) {F002,F003,I006} Remove calendar symbols from the current contract
  Goal:
  Calendar names and colors identify calendar groups without a separate letter or symbol.
  Requirements:
  - Remove `symbol` from the canonical Calendar model and database schema.
  - Remove `symbol` from calendar constructors, defaults, patches, and service inputs.
  - Remove `symbol` from calendar HTTP requests and representations.
  - Remove `symbol` from the Horizon projection and OpenAPI schema.
  - Remove provider calendar symbol generation from calendar import.
  - Remove each calendar symbol field from setup and Settings forms.
  - Remove symbols from calendar selection options.
  - Remove symbols from calendar visibility controls and lane labels.
  - Remove the obsolete calendar and lane symbol styles.
  - Keep calendar identifiers, names, color tokens, order, visibility, and memberships unchanged.
  - Keep the numeric `1` through `9` calendar visibility shortcuts.
  - Reject `symbol` as an unknown field after the API cutover.
  - Delete obsolete symbol data during the canonical schema cutover.
  - Keep no symbol alias, compatibility read, or migration bridge after the cutover.
  Deliverables:
  - Update the calendar model, database contract, services, handlers, and provider adapter.
  - Update the Horizon projection, browser fixtures, templates, styles, and scripts.
  - Update `api/horizon.openapi.json` and each affected technical document.
  - Replace symbol contract tests with name, color, and visibility contract tests.
  Validation:
  - Create and update a calendar without a `symbol` field.
  - Submit a `symbol` field and confirm that the API rejects it.
  - Read a calendar and confirm that the response has no `symbol` field.
  - Read the Horizon projection and confirm that it has no `symbol` field.
  - Import provider calendars and confirm that RSVP creates no symbol data.
  - Confirm that setup and Settings contain no Symbol field.
  - Confirm that calendar controls, options, and lane labels contain no calendar symbol.
  - Use shortcuts `1` through `9` and confirm calendar visibility changes.
  - Validate the canonical schema after obsolete symbol data is absent.
  - Run `make ci` and `make browser-test`.
  - Run the Governor check and the language checker for each changed technical document.

- [x] [I013] (P1) {I009,I011} Add direct Horizon time scale controls
  Goal:
  The Horizon view needs direct day, week, month, and year scale choices.
  The direct choices replace the two relative scale controls.
  Requirements:
  - Replace the scale out and scale in buttons with `D`, `W`, `M`, and `Y` buttons.
  - Keep the backward pan button before the four scale buttons.
  - Keep the forward pan button after the four scale buttons.
  - Use `D` for the day scale.
  - Use `W` for the week scale.
  - Use `M` for the month scale.
  - Use `Y` for the year scale.
  - Make the month scale the initial choice.
  - Mark the current scale choice for assistive technology.
  - Replace the plus and minus keyboard operations with the four letter operations.
  - Keep each control visible at the supported mobile width.
  Deliverables:
  - Update the Horizon template, script, and styles.
  - Update the Help instructions and the user documentation.
  - Update the Horizon HTML and browser contract tests.
  Validation:
  - Confirm that the control order is backward, day, week, month, year, and forward.
  - Confirm that each scale button selects its specified scale.
  - Confirm that only the current scale button has the active state.
  - Use each letter key and confirm the same scale result.
  - Confirm that the plus and minus keys do not change the scale.
  - Confirm that all six controls remain visible at the supported mobile width.
  - Run `make ci` and `make browser-test`.
  - Run the Governor check and the language checker for each changed technical document.

## Maintenance

- [ ] [M400R] (P2) Backlog hygiene and archive
  Goal:
  Keep the issue tracker reliable, readable, and focused on active work while preserving resolved history in the appropriate archive.

  Requirements:
  - Cadence: run weekly during active development and before each release cut.
  - Validate section names, identifier prefixes, recurrence suffixes, priority markers, dependencies, and duplicate IDs against the current `issues-md-format.md`.
  - Reconcile stale statuses, duplicate issues, broken references, obsolete instructions, and entries filed in the incorrect section.
  - Move completed non-recurring history to the repository issue archive or durable documentation when the active tracker becomes noisy.
  - Keep active, blocked, planning, and recurring entries visible in `ISSUES.md`.

  Deliverables:
  - Normalized `ISSUES.md` structure and statuses.
  - Updated issue archive or docs when completed entries are removed from the active tracker.
  - A short `Last run:` note summarizing the cleanup and any follow-up issues filed.

  Validation:
  - Read `ISSUES.md` after edits and confirm that each issue is in the correct section.
  - Confirm that each issue has a unique section-aware ID.
  - Confirm recurring entries remain open and keep the `R` suffix.
  - Confirm no active, blocked, recurring, or planning work was archived.

- [ ] [M401R] (P2) Polish open issues
  Goal:
  Keep unresolved work executable by making each open issue concrete, ordered, and testable.

  Requirements:
  - Cadence: run weekly during active development and before handing a repo to automated execution.
  - Review every unresolved non-recurring issue for missing context, dependencies, repro steps, acceptance criteria, and validation expectations.
  - Make priorities concrete and make sure that each open issue has actionable deliverables.
  - Merge duplicate open issues or add explicit dependency links when separate entries must remain.
  - Do not close or implement issues as part of this polish pass unless that work is separately requested.

  Deliverables:
  - Open issues with enough detail for a person or agent to execute without rediscovery.
  - New or updated dependency markers where ordering matters.
  - A short `Last run:` note listing the number of issues polished and any blockers found.

  Validation:
  - Sample the open entries after the pass and confirm each has clear next actions and validation expectations.
  - Confirm that no recurring runbook has a closed status.
  - Confirm duplicates were merged or explicitly cross-referenced.

- [ ] [M402R] (P2) Architecture and policy review
  Goal:
  Catch architecture, policy, and workflow drift before it becomes hidden maintenance debt.

  Requirements:
  - Cadence: run monthly, before large refactors, and after major framework or runtime changes.
  - Review the codebase, docs, and workflow against `AGENTS.md`, `POLICY.md`, stack guides, and the current architecture notes.
  - Look for drift from forward-only contracts, edge-validation boundaries, smart-constructor usage, testing policy, and module ownership.
  - Record findings as new Maintenance issues with concrete scope, priority, and validation.
  - Close the pass with a no-action note only when the review finds no actionable drift.

  Deliverables:
  - New Maintenance issues for each actionable architecture or policy drift finding.
  - Updated notes on areas reviewed and areas intentionally left unchanged.
  - A short `Last run:` note with the review scope and outcome.

  Validation:
  - Confirm every finding is represented as an issue with owner-readable context and validation criteria.
  - Confirm no implementation changes were mixed into the review runbook unless separately requested.
  - Confirm all recurring runbooks remain open.

- [ ] [M403R] (P1) Dependency and security audit
  Goal:
  Keep third-party dependencies, runtime versions, and security-sensitive configuration within the current supported contract.

  Requirements:
  - Cadence: run weekly for active apps and before each release cut.
  - Inspect package managers, lockfiles, language toolchains, container bases, and generated clients for known vulnerabilities or stale direct dependencies.
  - Review auth, secret, CORS, CSP, SQL, network, and permission-sensitive configuration for drift from the current contract.
  - Prefer current supported dependencies.
  - Do not add compatibility shims for obsolete dependency behavior.
  - File separate Maintenance or BugFix issues for each actionable vulnerability, unsupported runtime, or security-contract gap.

  Deliverables:
  - Documented audit commands or data sources used for the pass.
  - Updated issues for each actionable dependency or security finding.
  - A short `Last run:` note with clean result or follow-up issue IDs.

  Validation:
  - Rerun the repository-native audit, lint, or dependency checks used for the pass.
  - Confirm every finding is either filed, fixed under a separate issue, or explicitly marked not applicable with evidence.
  - Confirm no secrets or private payloads were written into the tracker.

- [ ] [M404R] (P1) CI, release, and artifact health
  Goal:
  Keep the repository's validation, release, publication, and generated artifact surfaces trustworthy.

  Requirements:
  - Cadence: run before every release, publish, or deploy, and weekly for critical services.
  - Verify repository-native CI, lint, format, coverage, release, publish, Docker image, Pages, and artifact workflows still match the documented contract.
  - Do a check of generated artifacts, release tags, published images, and Pages outputs for source-to-public drift.
  - File concrete follow-up issues for failing gates, stale artifacts, missing release prerequisites, or undocumented workflow changes.
  - Do a production deployment only when the operator explicitly requests it.

  Deliverables:
  - Recorded gate status and artifact surfaces inspected.
  - Follow-up issues for each reproducible CI, release, publish, or artifact drift problem.
  - A short `Last run:` note with commands run and any skipped surfaces.

  Validation:
  - Use repository-native `make` targets or documented release helpers for checks.
  - Confirm release and deployment ownership boundaries remain separate.
  - Confirm public or published artifacts match the intended source revision when that surface is inspected.

- [ ] [M405R] (P1) Code contract and static hygiene
  Goal:
  Keep source contracts explicit, current, and statically guarded against policy drift.

  Requirements:
  - Cadence: run monthly and before large refactors.
  - Scan for dead code, unused exports, duplicated literals, silent fallbacks, legacy aliases, compatibility reads, and zero-but-invalid domain states.
  - Do a check of static analysis, coverage, schema, and contract guards that prevent drift.
  - File focused Maintenance issues for each concrete violation instead of broad cleanup placeholders.
  - Keep only the current canonical contract.
  - Preserve obsolete behavior only when a current product requirement explicitly specifies it.

  Deliverables:
  - Issue entries for each actionable static hygiene or contract violation.
  - Notes on static tools, searches, and contract guards used during the pass.
  - A short `Last run:` note with clean result or follow-up issue IDs.

  Validation:
  - Rerun the relevant static checks, contract tests, or repository searches used to identify drift.
  - Confirm every finding has a narrow follow-up issue and does not duplicate existing backlog work.
  - Confirm no implementation changes were mixed into the audit unless separately requested.

- [ ] [M406R] (P1) Production drift and health
  Goal:
  Detect drift between runtime state and the intended repository contract.

  Requirements:
  - Cadence: run weekly for deployed services and after each publish or deploy.
  - Compare current source, runtime configuration, published images, public routes, scheduled jobs, and health checks for drift.
  - Inspect real operator-facing surfaces rather than assuming merged source is deployed.
  - File follow-up issues for stale images, stale Pages output, missing routes, failed monitors, invalid production config, or undocumented runtime differences.
  - Stop before production deploy or destructive operator actions unless the operator explicitly requests them.

  Deliverables:
  - Recorded source revision, public artifact, route, image, or health surfaces inspected.
  - Follow-up issues for each source-to-runtime drift finding.
  - A short `Last run:` note with evidence links or commands used.

  Validation:
  - Verify inspected production or public surfaces directly where access is available.
  - Confirm any deploy-required finding is filed with the exact publish/deploy boundary and owner.
  - Confirm no production state was changed by the audit unless explicitly requested.

- [ ] [M407R] (P2) Documentation and runbook hygiene
  Goal:
  Keep durable documentation and runbooks aligned with the current behavior users and operators actually rely on.

  Requirements:
  - Cadence: run before release cuts and after merge bursts that change user-facing or operator-facing behavior.
  - Review README, ARCHITECTURE, PRD, CHANGELOG, docs, runbooks, setup guides, and local workflow notes for stale behavior or missing new contracts.
  - Review changed English technical prose against `.mprlab/AGENTS.DOCS.md` and the official ASD-STE100 standard.
  - Add approved repository terms to `.mprlab/TERMINOLOGY.md`.
  - Update docs when closed issues changed durable behavior, public APIs, operator workflows, release semantics, or deployment expectations.
  - Remove or rewrite stale instructions instead of preserving obsolete alternatives.
  - File separate issues for documentation gaps that require product or implementation decisions.

  Deliverables:
  - Updated documentation or filed follow-up issues for each gap.
  - A short `Last run:` note listing docs inspected and changes made.
  - Cross-references from archived issue history to durable docs when useful.

  Validation:
  - Run the skill `prepare-ste-reference` script and use its verified official PDF.
  - Run the skill `check-ste` script on each English technical document that changed.
  - Review the changed text against Part 1 writing rules and the Part 2 dictionary.
  - Confirm that the producing agent completed the review without end-user work.
  - Do a check of links, command names, paths, and public contract descriptions changed by the pass.
  - Confirm docs describe the current canonical path only.
  - Confirm issue archive and active tracker references remain consistent.

## Features

- [x] [F012] (P1) Add LoopAware data collection
  Goal:
  RSVP sends page traffic and customer comments to its LoopAware site.
  Requirements:
  - Use LoopAware site identifier `50bc3012-27c7-4fbd-8307-81dee1da50f7`.
  - Load the feedback widget on the public landing page.
  - Load the traffic pixel on the public landing page.
  - Load the feedback widget on each page that uses the shared application layout.
  - Load the traffic pixel on each page that uses the shared application layout.
  - Start the LoopAware scripts after the page load event.
  - Keep one canonical site identifier in each rendered page.
  Deliverables:
  - Add the LoopAware scripts to `templates/landing.tmpl`.
  - Add the LoopAware scripts to `templates/layout.tmpl`.
  - Add a rendered HTML contract test for both page surfaces.
  Validation:
  - Render the public landing page and confirm both current LoopAware URLs.
  - Render an application page and confirm both current LoopAware URLs.
  - Confirm that each page contains no other LoopAware site identifier.
  - Run `make ci`.
  Resolved:
  RSVP loads the current LoopAware widget and pixel after page load on both rendered page surfaces.

- [x] [F001] (P0) {I002} Add the horizon projection
  Goal:
  This issue adds the authenticated read interface for calendars, lanes, events, and probes in one time window.

  Requirements:
  - Add an authenticated horizon route.
  - Accept validated window start and end parameters.
  - Apply the approved maximum window size.
  - Return calendars in their display order.
  - Return each finite lane that intersects the requested window.
  - Return each active open lane that starts before the window ends.
  - Include open lanes that have no marker in the window.
  - Return events and probes as typed markers.
  - Filter each marker with the approved half-open window rules.
  - Preserve one projection row for each lane.
  - Keep calendar membership separate from lane membership.
  - Enforce owner scope in each database query.
  - Supply HTML and JSON representations from one projection service.
  - Add the approved representation and cache headers.
  - Reject invalid windows with the canonical validation response.

  Deliverables:
  - Add the horizon projection service and response types.
  - Add the horizon route and handlers.
  - Add query tests for ownership, ordering, windows, and empty lanes.

  Validation:
  - Request a window that contains no events for an active open lane.
  - Confirm that the response contains the open lane.
  - Request data for a different owner.
  - Confirm that the response contains no resources from the other owner.
  - Request two independent events from one calendar.
  - Confirm that the response contains two lanes.
  - Run `make ci`.

- [x] [F002] (P0) {F001} Add the interactive horizon view
  Goal:
  This issue makes temporal lanes the primary organizer interface.

  Requirements:
  - Render each lane as a 12-to-16-pixel horizontal line.
  - Render each marker as a 20-to-24-pixel dot.
  - Give each interactive marker a minimum 44-pixel target.
  - Show a cap at each finite lane end.
  - Show an arrow at each open lane continuation.
  - When the window contains no markers, render active lanes.
  - Clip each lane line to its visible start and end.
  - Show sticky lane labels, a time scale, and a today line.
  - Add horizontal pan and scale controls.
  - Add keyboard operations for pan, scale, calendar visibility, and marker selection.
  - Add calendar controls that show or hide complete calendars.
  - Store each calendar visibility choice for the current user.
  - Use text and symbols with color to identify calendars.
  - Render one row for each independent event lane.
  - Render one row for each event series.
  - Render one row for each dependency chain.
  - Link each RSVP event marker to its event and RSVP controls.
  - Make the horizon view the authenticated home route.
  - Put horizon styles and scripts in dedicated static assets.

  Deliverables:
  - Add the server-rendered horizon template and view data.
  - Add the horizon style and JavaScript modules.
  - Add the static asset route.
  - Add real-browser tests for the specified interactions.

  Validation:
  - Confirm that an empty open lane spans the visible window.
  - Confirm that calendar controls hide and show all related lanes.
  - Confirm that finite and open lanes use different end symbols.
  - Confirm that three independent birthdays use three lanes.
  - Confirm that two independent holidays use two lanes.
  - Confirm that dependent travel events use one lane.
  - Confirm that marker targets meet the minimum target size.
  - Confirm that keyboard operations control the view without a pointer.
  - Confirm that the browser renders the view at the supported mobile width.
  - Run `make ci`.
  - Run the browser test target.

- [x] [F003] (P1) {F002} Add calendar and lane management
  Goal:
  This issue lets an organizer create and maintain the temporal structure shown in the horizon view.

  Requirements:
  - Add resource routes for calendars and lanes.
  - Add calendar create, update, reorder, visibility, and delete operations.
  - Add lane create, update, reorder, resolve, and delete operations.
  - Support finite and open lane creation.
  - When an organizer creates an independent event, create a new lane.
  - Start each new local lane at the request reference time.
  - When an organizer creates a dependent event, require an anchor event.
  - Put each dependent event on the anchor event lane.
  - Put each new event occurrence on the related event series lane.
  - Recalculate finite lane bounds in each related marker transaction.
  - Use calendar selection only for calendar membership.
  - Apply the approved deletion rules in one database transaction.
  - Validate all request data at the HTTP boundary.
  - After request validation, construct only valid domain types.
  - Enforce ownership for each read and write operation.
  - Keep RSVP event identifiers and public response routes unchanged.

  Deliverables:
  - Add calendar and lane handlers, forms, and domain services.
  - Add lane controls to the horizon view.
  - Add integration tests for each resource operation.

  Validation:
  - Create one finite lane and one open lane from the horizon view.
  - Create two independent events in one calendar.
  - Confirm that the independent events use different lanes.
  - Create one dependent event from an anchor event.
  - Confirm that the dependent event uses the anchor event lane.
  - Reorder calendars and lanes.
  - After a new session starts, confirm that the browser shows the saved order.
  - Resolve an open lane.
  - Confirm that the resolved lane has a finite end.
  - Confirm that one owner cannot change another owner's resources.
  - Run `make ci`.
  - Run the browser test target.

- [x] [F004] (P1) {F003} Add attention policies and probes
  Goal:
  This issue gives unresolved lanes a durable review and escalation workflow.

  Requirements:
  - Add attention policy create, update, and delete operations.
  - Store the next probe time and the review interval.
  - Store an optional escalation time.
  - Create no more than one pending probe for each policy occurrence.
  - Record completed and missed probes.
  - After probe completion, set the next probe time.
  - When the lane becomes resolved, stop future probes.
  - Render pending and missed probes as distinct markers.
  - Show the next attention time in the lane details.

  Deliverables:
  - Add the attention policy service and probe state transitions.
  - Add policy and probe controls to the horizon view.
  - Add deterministic cadence and escalation tests.

  Validation:
  - Complete a probe.
  - Confirm that the policy creates the next due time.
  - Move the test clock past the escalation time.
  - Confirm that the probe becomes missed.
  - Resolve the lane.
  - Confirm that the policy creates no new probe.
  - Refresh the page.
  - Confirm that the probe state does not change.
  - Run `make ci`.
  - Run the browser test target.

- [x] [F005] (P1) {I002,F003} Add a Google Calendar connection
  Goal:
  This issue lets an organizer authorize read-only access and select source calendars.

  Requirements:
  - Add a calendar connection resource.
  - Use a consent flow that is separate from Google sign-in.
  - Request only the `calendar.calendarlist.readonly` and `calendar.events.readonly` scopes.
  - Store one expiring authorization request with a state value hash.
  - Keep the authorization callback free of database changes.
  - Add no-store and no-referrer headers to the authorization callback.
  - Before database storage, encrypt refresh credentials.
  - Keep the credential encryption key in private deployment values.
  - Do not write credentials or authorization codes to logs.
  - After authorization, list the provider calendars.
  - Let the organizer select the source calendars.
  - Connect each selected source calendar to one RSVP calendar.
  - Require an idempotency key for connection creation.
  - Show the connection state and provider errors.
  - When the organizer deletes the connection, delete stored credentials.
  - Limit this issue to connection and source selection.

  Deliverables:
  - Add the Google Calendar adapter and authorization handlers.
  - Add encrypted credential storage and private value configuration.
  - Add the connection and source selection interface.
  - Add adapter tests with a deterministic test server.

  Validation:
  - Complete authorization with the deterministic test server.
  - Confirm that RSVP encrypts the stored credential data.
  - Confirm that logs contain no credential values.
  - Select two source calendars.
  - Confirm that the response contains two RSVP calendars.
  - Delete the connection.
  - Confirm that stored credentials for the connection are absent.
  - Run `make ci`.
  - Run the browser test target.

- [x] [F006] (P1) {F001,F005} Synchronize Google Calendar markers
  Goal:
  This issue imports selected Google Calendar events into the horizon view without duplicate markers.

  Requirements:
  - Do an initial synchronization for each selected source calendar.
  - Store one sync cursor for each source calendar.
  - Store a new sync cursor only after the last response page commits.
  - Use the sync cursor for each later incremental synchronization.
  - When the provider rejects a sync cursor, do a complete source reconciliation.
  - Store one external event link for each imported event.
  - Store one external event series link for each imported event series.
  - Make each synchronization idempotent.
  - Import timed, all-day, and recurring event instances.
  - Preserve the source event timezone.
  - Apply provider updates and deletions to imported markers.
  - Protect provider-owned marker fields from local updates.
  - Create one lane for each independent provider event.
  - Create one lane for each provider event series.
  - Start each source-owned lane at the earlier first synchronization time or earliest marker time.
  - Put each recurring event occurrence on the related event series lane.
  - Use the source calendar only for calendar membership.
  - Record the last successful synchronization time and the current error.
  - Show synchronization state in the calendar controls.
  - Require an idempotency key for synchronization creation.

  Deliverables:
  - Add initial and incremental synchronization services.
  - Add external event link persistence.
  - Add presentation for source-owned markers.
  - Add synchronization status controls.
  - Add synchronization tests for updates, deletions, recurrence, and timezones.

  Validation:
  - Synchronize the same initial data two times.
  - Confirm that the second operation creates no duplicate marker.
  - Apply an incremental update.
  - Confirm that the imported marker contains the update.
  - Confirm that unchanged markers retain their stored values.
  - Delete a provider event.
  - Confirm that the imported marker for the provider event is absent.
  - Confirm that the projection shows an imported all-day event on the correct local date.
  - Synchronize three independent birthdays from one source calendar.
  - Confirm that the birthdays use three lanes.
  - Synchronize two independent holidays from one source calendar.
  - Confirm that the holidays use two lanes.
  - Synchronize two occurrences from one event series.
  - Confirm that the occurrences use one lane.
  - Run `make ci`.
  - Run the adapter tests.

- [x] [F007] (P2) {F003} Add derived markers
  Goal:
  This issue creates markers at typed time offsets from anchor markers.

  Requirements:
  - Add a resource for each derived marker rule.
  - Support offsets before and after an anchor marker.
  - Support anchor start and anchor end as offset bases.
  - Reject an all-day anchor marker.
  - Recalculate derived marker times in the anchor update transaction.
  - Before persistence, reject relation cycles.
  - Prevent direct time changes to a derived marker.
  - Put each derived marker on the anchor marker lane.
  - When you delete a derived marker rule, remove the related derived marker.
  - Show the anchor relationship in marker details.
  - Show each derived marker on the assigned lane.

  Deliverables:
  - Add the derived marker domain service and resource handlers.
  - Add derived marker controls to event details.
  - Add transaction, offset, and cycle tests.

  Validation:
  - Create markers before departure and after arrival anchors.
  - Confirm that the anchor and derived markers use one lane.
  - Change each anchor time.
  - Confirm that the related marker time changes.
  - Attempt to create a cycle.
  - Confirm that validation rejects the cycle.
  - Delete a rule.
  - Confirm that the derived marker for the rule is absent.
  - Run `make ci`.
  - Run the browser test target.

- [x] [F008] (P1) {F003,F004} Add quick ingestion drafts
  Goal:
  This issue supplies a short, confirmable workflow for new events and unresolved lanes.

  Requirements:
  - Add an available quick-add control to the horizon view.
  - Accept a dated event mode and an open lane mode.
  - Accept calendar, title, date, time, end, and attention inputs.
  - Convert each valid input into an ingestion draft.
  - For an independent event draft, propose a new lane.
  - For a dependent event draft, require an anchor event.
  - For a dependent event draft, propose the anchor event lane.
  - Use calendar selection only for calendar membership.
  - Before confirmation, show the proposed calendar, lane, marker, and attention policy.
  - Let the organizer correct each proposed value.
  - Use the organizer timezone for date interpretation.
  - Use explicit confirmation as the persistence boundary.
  - Require an idempotency key for draft confirmation.
  - Keep draft creation separate from temporal resource creation.

  Deliverables:
  - Add the ingestion draft resource and validation service.
  - Add the quick-add and confirmation interfaces.
  - Add tests for both input modes and all confirmation outcomes.

  Validation:
  - Create an open waiting lane with a weekly attention policy.
  - Create an independent birthday in the Birthdays calendar.
  - Confirm that the birthday receives a new lane.
  - Create a dependent travel event from a flight anchor.
  - Confirm that the travel event uses the flight lane.
  - Cancel each draft.
  - Confirm that no temporal data changes.
  - Correct a draft timezone.
  - Confirm that the marker uses the corrected time.
  - Run `make ci`.
  - Run the browser test target.

- [x] [F009] (P2) {F007,F008} Add natural-language ingestion
  Goal:
  This issue converts natural-language temporal requests into confirmable ingestion drafts.

  Requirements:
  - Add one provider adapter for natural-language parsing.
  - Validate each provider response against the ingestion draft schema.
  - Parse independent events, dated anchors, open lanes, attention intervals, and relative marker rules.
  - Use one explicit reference time and the organizer timezone.
  - Identify each missing required value.
  - Before confirmation, let the organizer supply each missing value.
  - Show each inferred value in the confirmation interface.
  - Use explicit confirmation as the temporal resource persistence boundary.
  - Keep provider credentials in private deployment values.
  - Do not write input text or credentials to application logs.
  - Do not store the original input text.

  Deliverables:
  - Add the parser adapter and validated response schema.
  - Add natural-language input to the quick-add interface.
  - Add deterministic parser fixtures for waiting and travel requests.
  - Add error and incomplete-input tests.

  Validation:
  - Parse an unresolved appeal with a weekly check interval.
  - Confirm that the draft contains an open lane and an attention policy.
  - Parse a flight with relative departure and arrival markers.
  - Confirm that the draft contains the anchor and derived marker rules.
  - Confirm that the draft creates one dependency chain for the flight markers.
  - Reject an invalid provider response without a database change.
  - Run `make ci`.
  - Run the browser test target.

- [ ] [F010] (P1) {F001,F002,F003} Add state cycles to Horizon
  Goal:
  This feature represents a subject that changes between states in one repeating schedule.
  The horizon view shows the current state and the next state transition on one lane.

  Requirements:
  - Represent one state cycle on one dedicated lane.
  - Keep the state cycle as the only primary temporal subject on its lane.
  - Keep calendar membership separate from state cycle lane membership.
  - Require two or more cycle states.
  - Give each cycle state one label, color token, and display order.
  - Define one cycle template as an ordered sequence of cycle phases.
  - Give each cycle phase one cycle state and one positive whole-day duration.
  - Permit unequal durations for different cycle phases.
  - Permit one cycle state in multiple nonadjacent cycle phases.
  - Require adjacent cycle phases to use different cycle states.
  - Require the last and first cycle phases to use different cycle states.
  - Give each state cycle one local anchor date and one IANA timezone.
  - Calculate the cycle offset from the local date, anchor date, and complete cycle duration.
  - Keep each cycle offset from zero through the complete cycle duration minus one.
  - Calculate the next state transition from the active cycle phase boundary.
  - Keep the cycle template as the canonical persisted schedule.
  - Generate state intervals only for the requested horizon window.
  - Use half-open local date ranges for generated state intervals.
  - Return the current cycle state when the organizer's local date intersects the horizon window.
  - Return the next transition date and next cycle state.
  - Keep each state cycle lane active and open while the cycle exists.
  - Create and replace a complete cycle template in one transaction.
  - Reject each request that would create an incomplete cycle template.
  - Require an idempotency key for each state cycle creation request.
  - Require resource preconditions for each concurrent state cycle update.
  - Return an entity tag for each state cycle representation.
  - Delete the dedicated lane in the same transaction as its state cycle.
  - Render one segmented band for each state cycle lane.
  - Use segment width to show the duration of each generated state interval.
  - Use stable state labels and colors for all segments.
  - Mark the current date with one visible cursor.
  - Show the current state, next state, and time to the next transition.
  - Keep the full state and transition text available to assistive technology.
  - Support two-state and three-state cycle templates with the same resource contract.

  Deliverables:
  - Add `StateCycle`, `CycleState`, and `CyclePhase` domain types and database tables.
  - Give `StateCycle` the canonical `lane_id`, `anchor_date`, and `timezone` fields.
  - Give `CycleState` the canonical `state_cycle_id`, `label`, `color_token`, and `display_order` fields.
  - Give `CyclePhase` the canonical `state_cycle_id`, `cycle_state_id`, `duration_days`, and `display_order` fields.
  - Add database constraints for ownership, order, references, and positive phase durations.
  - Add smart constructors that enforce the complete cycle template invariants.
  - Add owner-scoped state cycle create, read, replace, and delete services.
  - Add `POST /state-cycles/` for atomic lane, state cycle, state, and phase creation.
  - Add `GET /state-cycles/{cycle_id}` and `DELETE /state-cycles/{cycle_id}`.
  - Add `GET` and `PUT` operations for `/state-cycles/{cycle_id}/template`.
  - Require `If-Match` for each state cycle template replacement and deletion.
  - Add the state cycle resources and typed errors to `api/horizon.openapi.json`.
  - Add a `state_cycle` object to each related horizon lane projection.
  - Return states, generated intervals, the current state, and the next transition in that object.
  - Add one bounded projection algorithm for cycle windows.
  - Add the segmented cycle band, current cursor, and transition summary to the horizon view.
  - Add state cycle create and edit controls to the organizer interface.
  - Add uneven binary and three-state cycle data to the browser fixture.
  - Update `ARCHITECTURE.md`, `TIME_HORIZON_ACCEPTANCE.md`, and `USER_GUIDE.md`.
  - Update each affected database, REST, service, and browser contract test.

  Open Decisions:
  - Select how RSVP converts imported provider intervals when the provider gives no state cycle identifier.
  - Define how RSVP preserves provider event identity after a confirmed conversion.
  - Select how RSVP replaces one generated state interval for a temporary schedule exception.

  Validation:
  - Create a binary cycle with phase durations of four, two, five, and one day.
  - Confirm the state and next transition at each phase boundary.
  - Confirm that the cycle repeats after the complete twelve-day duration.
  - Project a window that starts and ends inside generated state intervals.
  - Confirm that the projection clips the first and last state intervals to the window.
  - Project the cycle across a daylight time change.
  - Confirm that each local date keeps the correct cycle state.
  - Create a three-state cycle and confirm the same projection contract.
  - Reject a cycle with fewer than two cycle states.
  - Reject a phase with a zero or negative duration.
  - Reject adjacent phases with the same cycle state.
  - Reject a template whose last and first phases use the same cycle state.
  - Replace one complete cycle template and confirm that the horizon changes in one transaction.
  - Submit a stale resource precondition and confirm that the update fails.
  - Delete a state cycle and confirm that its dedicated lane is absent.
  - Confirm that calendar visibility hides and shows the complete state cycle lane.
  - Confirm that one binary cycle renders as one segmented lane.
  - Move the current date through each segment and confirm the transition summary.
  - Confirm the full state and transition text at narrow browser widths.
  - Run `make ci` and `make browser-test`.
  - Run the Governor check and the language checker for each changed technical document.

- [x] [F011] (P1) {F001,F002} Add organizer account settings
  Goal:
  The Account rubric gives the organizer direct control of account settings.

  Requirements:
  - Add an Account rubric to Settings.
  - Add the organizer timezone to the Account rubric.
  - Show the current organizer timezone.
  - Let the organizer store one valid IANA timezone.
  - Reject an invalid timezone without a database change.
  - Use the changed timezone for each later default Horizon window.
  - Keep the timezone of each stored marker.

  Deliverables:
  - Add the owner-scoped organizer read and update resource.
  - Add the account timezone form and browser-timezone helper.
  - Add the organizer resource to the OpenAPI contract.
  - Add API, HTML, and browser contract tests.
  - Update the architecture, acceptance matrix, user guide, and operator runbook.

  Validation:
  - Change the organizer timezone in Settings.
  - Refresh the page and confirm that the value persists.
  - Confirm that the next default Horizon window uses the changed timezone.
  - Submit an invalid timezone and confirm that the stored value does not change.
  - Confirm that an existing event keeps its marker timezone.
  - Run `make ci` and `make browser-test`.
  - Run the Governor check and the language checker for each changed technical document.

## Planning

- [x] [P001] (P0) Specify the time horizon contract
  Goal:
  The time horizon needs one approved contract for data, interfaces, schema initialization, and user behavior.
  This issue gives the required decisions and does not authorize implementation.

  Requirements:
  - Define calendars, lanes, markers, probes, attention policies, and RSVP relationships.
  - Define a calendar as a visibility family that contains independent lanes.
  - Define one lane for each independent event.
  - Define one lane for each event series.
  - Permit distinct events on one lane only through an explicit dependency chain.
  - Keep each dependent event on the anchor event lane.
  - Define calendar membership without control of lane membership.
  - Define open, finite, active, and resolved lane states.
  - Define point, interval, all-day, and timezone rules for markers.
  - Define ownership without duplicate owner fields.
  - Define client responsibility for timezone defaults.
  - Define the default horizon window and maximum window size.
  - Define calendar visibility, lane order, and empty-lane behavior.
  - Define resource routes and response schemas for new interfaces.
  - Define Google Calendar authorization, source mapping, and synchronization ownership.
  - Define derived marker recalculation rules.
  - Define ingestion draft confirmation rules.
  - Define the schema initialization and production cutover sequence.

  Deliverables:
  - Add `ARCHITECTURE.md` as the approved time horizon architecture document.
  - Add `TIME_HORIZON_ACCEPTANCE.md` as the contract acceptance matrix.
  - Record each unresolved product choice as a blocker.

  Validation:
  - Review each decision against the current Event, RSVP, Venue, and User contracts.
  - Confirm that independent birthdays in one calendar use different lanes.
  - Confirm that independent holidays in one calendar use different lanes.
  - Confirm that dependent travel events use the anchor event lane.
  - Confirm that each later issue uses the approved terms and relationships.
  - Confirm that the design has no dual data contract or compatibility path.
  - Run the documentation language checker on each changed technical document.
