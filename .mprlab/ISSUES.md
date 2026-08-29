# ISSUES

Entries record newly discovered requests or changes.

Read `AGENTS.md`, `.mprlab/POLICY.md`, `.mprlab/issues-md-format.md`, and relevant stack guides before implementing changes.

Format: `- [ ] [B042] (P1) {I007} Title`

## BugFixes

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
