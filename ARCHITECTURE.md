# Time Horizon Architecture

Status: Approved contract for P001.

This document defines the target time horizon architecture for RSVP.
The current runtime uses the canonical calendar and lane schema.
The [acceptance matrix](TIME_HORIZON_ACCEPTANCE.md) assigns each contract behavior to an implementation issue.

## Product Contract

RSVP shows temporal subjects as lanes on a time axis.
A lane stays visible when its visible time window contains no marker.
Calendars group lanes into user-controlled visibility families.
Markers show events, probes, and times that RSVP derives from anchor markers.

The horizon view is the primary authenticated organizer interface.
The view uses thick lanes and large markers for clear selection.
An open lane continues beyond the visible time window.
A finite lane stops at its known end time.

## Protected RSVP Contract

The time horizon keeps the current RSVP invitation contract.

- Preserve each event identifier.
- Preserve each RSVP public code.
- Preserve each RSVP response and extra guest count.
- Preserve each venue relationship.
- Preserve the public route `/response/?rsvp_id={rsvp_id}`.
- Preserve the QR payload for the public response route.
- Keep the public response route available without organizer authentication.
- Keep organizer operations behind authentication and owner authorization.

I001 supplies tests for these protected behaviors.
I003 does the protected behavior checks again after the time horizon work.

## Resource Graph

An organizer owns each calendar directly.
Each temporal resource below a calendar gets ownership through its relationship chain.
RSVP does not store a duplicate organizer identifier on these temporal resources.

| Resource | Required relationship | Ownership source |
|---|---|---|
| Calendar | One organizer | Organizer identifier |
| Lane | One calendar | Calendar |
| Event series | One lane | Lane and calendar |
| Event | One lane | Lane and calendar |
| RSVP | One event | Event, lane, and calendar |
| Attention policy | One lane | Lane and calendar |
| Probe | One attention policy and one lane | Lane and calendar |
| Derived marker rule | One anchor marker and one lane | Lane and calendar |
| Derived marker | One derived marker rule and one lane | Lane and calendar |
| Calendar authorization request | One organizer | Organizer identifier |
| Calendar connection | One organizer | Organizer identifier |
| Source calendar mapping | One connection and one calendar | Connection organizer |
| External event series link | One mapping and one event series | Event series relationship |
| External event link | One mapping and one RSVP event | RSVP event relationship |
| Ingestion draft | One organizer and one calendar | Organizer identifier |
| Draft derived marker rule | One ingestion draft | Ingestion draft organizer |
| Draft confirmation | One ingestion draft and created resources | Ingestion draft organizer |

A venue keeps its direct organizer relationship.
An event keeps its optional venue relationship.

## Canonical Persistence

Each persisted resource uses an opaque identifier and standard audit timestamps.
Conditional fields use database constraints and validated domain constructors.
An organizer can exist before timezone confirmation.
The first temporal write must have a client-supplied timezone.
RSVP stores that value as the organizer timezone.

| Resource | Canonical fields |
|---|---|
| Organizer | `id`, `email`, `name`, `picture`, `timezone` |
| Calendar | `id`, `organizer_id`, `name`, `symbol`, `color_token`, `display_order`, `visible` |
| Lane | `id`, `calendar_id`, `title`, `status`, `starts_at`, `ends_at`, `resolved_at`, `display_order` |
| Event series | `id`, `lane_id`, `timezone`, `source_kind`, optional `recurrence_rule` |
| Event | `id`, `lane_id`, optional `event_series_id`, optional `anchor_event_id`, `relation_type`, `time_shape`, typed time fields, `timezone`, `title`, `description`, optional `venue_id` |
| RSVP | `id`, `event_id`, `name`, `response`, `extra_guests` |
| Attention policy | `id`, `lane_id`, `review_interval_seconds`, `next_probe_at`, optional `escalation_interval_seconds` |
| Probe | `id`, `policy_id`, `lane_id`, `due_at`, optional `escalates_at`, `state`, optional `completed_at` |
| Derived marker rule | `id`, `lane_id`, `anchor_type`, `anchor_id`, `anchor_edge`, `offset_seconds` |
| Derived marker | `id`, `rule_id`, `lane_id`, `at`, `timezone` |
| Calendar authorization request | `id`, `organizer_id`, `provider`, `state_hash`, `redirect_uri`, `expires_at`, optional `used_at` |
| Calendar connection | `id`, `organizer_id`, `provider`, encrypted credential fields, `status` |
| Source calendar mapping | `id`, `connection_id`, `calendar_id`, `provider_calendar_id`, optional `sync_cursor` |
| External event series link | `id`, `mapping_id`, `event_series_id`, `provider_series_id` |
| External event link | `id`, `mapping_id`, `event_id`, `provider_event_id`, optional `provider_series_id` |
| Calendar sync | `id`, `mapping_id`, `state`, `started_at`, optional `finished_at`, optional `error_code` |
| Ingestion draft | `id`, `organizer_id`, `status`, `mode`, `source`, `calendar_id`, typed proposals, `reference_time`, `timezone`, `missing_fields_json` |
| Draft derived marker rule | `id`, `draft_id`, `anchor_edge`, `offset_seconds` |
| Draft confirmation | `id`, `draft_id`, created resource identifiers |
| Idempotency record | `id`, `organizer_id`, `operation`, `key_hash`, `request_hash`, `response_status`, `resource_type`, `resource_id`, `expires_at` |

An event `relation_type` is `independent`, `series_occurrence`, or `dependent`.
An independent event has no event series or anchor relationship.
A series occurrence has one `event_series_id` value.
A dependent event has one `anchor_event_id` value.

An event time shape is `point`, `interval`, or `all_day`.
Database constraints permit only the fields for the selected time shape.
The service constructs one valid domain type before persistence.

One organizer cannot have two calendars with the same display order.
One calendar cannot have two lanes with the same display order.
One provider calendar cannot have two source mappings in one connection.
One RSVP calendar cannot have two source calendar mappings.
One provider series cannot have two external event series links in one mapping.
One provider event cannot have two external event links in one mapping.
One ingestion draft cannot have two draft confirmations.
One organizer, operation, and key hash cannot have two idempotency records.

### Closed Values

Lane `status` accepts `active` or `resolved`.
Event `relation_type` accepts `independent`, `series_occurrence`, or `dependent`.
Event `time_shape` accepts `point`, `interval`, or `all_day`.
Event series `source_kind` accepts `local` or `google`.

Probe `state` accepts `pending`, `completed`, `missed`, or `canceled`.
Calendar connection `status` accepts `authorization_pending`, `connected`, or `error`.
Calendar sync `state` accepts `pending`, `running`, `succeeded`, or `failed`.
Ingestion draft `status` accepts `incomplete`, `ready`, `confirmed`, or `canceled`.
Ingestion draft `source` accepts `quick` or `natural_language`.

Derived marker rule `anchor_type` accepts `event`, `probe`, or `derived`.
Derived marker rule `anchor_edge` accepts `start` or `end`.

## Calendar Contract

A calendar is a visibility family for lanes.
Each calendar has a name, symbol, color token, display order, and calendar visibility.
Calendar visibility does not change lane membership or persisted marker data.

Each lane belongs to exactly one calendar.
When the organizer moves a lane to another calendar, RSVP moves the complete lane.
The operation does not change the events, probes, or derived markers on that lane.

Calendar display order is unique for one organizer.
Lane display order is unique within one calendar.
RSVP normalizes each order in the same transaction as an order change.

## Lane Membership

Each independent event owns one lane.
Each event series owns one lane for all its event occurrences.
Each dependency chain uses the lane of its anchor event.

An independent event has no event series and no anchor event.
An event occurrence references exactly one event series.
A dependent event references the anchor event of its dependency chain.

Each probe uses the lane of its attention policy.
Each derived marker uses the lane of its anchor marker.
RSVP rejects a relationship that assigns either resource to a different lane.

Calendar selection controls only the calendar relationship of a lane.
Lane membership controls the timeline row for a temporal resource.
These two relationships stay separate.

## Lane Bounds

A lane starts when RSVP starts to track its temporal subject.
The lane start does not have to equal its first marker time.
Thus, a future event has a visible lane before its event marker.

For a local lane, `starts_at` uses the request reference time.
For a source-owned lane, `starts_at` uses the earlier first synchronization time or earliest imported marker time.

Each marker must be at or after the lane start.
Each marker on a finite lane must be at or before the lane end.
A marker transaction recalculates the related finite lane bounds from its current markers.
The transaction keeps the original tracking start when all current markers occur later.

An independent event lane ends at the last boundary of its event marker.
A finite event series lane ends at its recurrence limit.
A dependency chain lane ends at the last boundary of its anchor, dependent, or derived markers.
An active open lane has no calculated end.

## Lane States

Each lane has one canonical state.
The lane state controls its end shape and its future probes.

| State | `status` | `ends_at` | `resolved_at` |
|---|---|---|---|
| Active open lane | `active` | `null` | `null` |
| Active finite lane | `active` | Required | `null` |
| Resolved lane | `resolved` | Required | Equal to `ends_at` |

Each lane has a required `starts_at` value.
An active finite lane has an `ends_at` value after `starts_at`.
Only an active open lane can use the resolve transition.
Time passage does not change a lane state.

The resolve transition sets `ends_at` and `resolved_at` to one resolution time.
The same transaction cancels each pending probe for that lane.
The transition prevents each later probe for the lane.

An open event series can have no known end time.
An unresolved subject can also have no known end time.
An open event series and an unresolved subject do not require a marker in the current time window.

## Marker Contract

A marker is a projection type and not one general write resource.
Events, probes, and derived markers supply the marker variants.
Each marker has one lane and one time shape.

### Timed Shapes

A point marker has one UTC timestamp and one IANA timezone.
An interval marker has UTC start and end timestamps and one IANA timezone.
The interval end must be after the interval start.

An all-day marker has a local start date and an exclusive local end date.
It also has one IANA timezone for date interpretation.
An all-day marker does not store a time of day as its canonical value.

RSVP serializes each timestamp with RFC 3339 in UTC.
RSVP serializes each local date with the `YYYY-MM-DD` format.
The IANA timezone stays with the marker after UTC normalization.

### Marker Variants

Each marker response has `id`, `type`, `title`, `lane_id`, and `time`.
The `type` value is `event`, `probe`, or `derived`.
The `time` value uses one closed shape.

```json
{"shape":"point","at":"2026-08-23T14:00:00Z","timezone":"America/Los_Angeles"}
```

```json
{"shape":"interval","start":"2026-08-23T14:00:00Z","end":"2026-08-23T16:00:00Z","timezone":"America/Los_Angeles"}
```

```json
{"shape":"all_day","start_date":"2026-08-23","end_date":"2026-08-24","timezone":"America/Los_Angeles"}
```

An event marker adds `event_id` and its event relationship type.
A probe marker adds `probe_id`, `due_at`, and its probe state.
A derived marker adds `rule_id` and `anchor_marker_id`.

## Timezone Contract

Each organizer has one required organizer timezone.
RSVP stores the organizer timezone as an IANA timezone name.
RSVP does not use a fixed UTC offset as a timezone.
RSVP applies no server-side timezone default.

The organizer timezone controls default windows and local quick-add input.
A marker timezone controls that marker's local display and all-day dates.
A provider event keeps the timezone from its calendar provider.

The client must supply a timezone for each temporal write.
The browser client can supply its IANA timezone.
RSVP stores that value as the organizer timezone with the first temporal write.
RSVP rejects a temporal write when the supplied timezone is absent or invalid.
Before the first temporal write, the HTML Horizon shows setup for the first calendar.
The setup sends the browser IANA timezone with calendar creation.
The JSON Horizon returns `organizer_timezone_required` until that write completes.

## Horizon Window

The horizon API uses a half-open time window.
The window includes its start and does not include its end.

When both window parameters are absent, RSVP uses the default window.
The default starts at local midnight for the current organizer date.
The default ends at local midnight 90 calendar days later.

When one window parameter is present, the request requires both parameters.
Each parameter must be an RFC 3339 timestamp with an offset.
The end must be after the start.
The maximum request window is 366 calendar days.

## Horizon Projection

`GET /horizon/` returns one horizon projection.
The request accepts `start` and `end` query parameters.

The projection contains `window` and `calendars` fields.
The `window` value contains `start`, `end`, and `timezone`.
The `calendars` value contains calendars in display order.

Each calendar projection has these fields:

- `id`
- `name`
- `symbol`
- `color_token`
- `display_order`
- `visible`
- `lanes`

Each lane projection has these fields:

- `id`
- `title`
- `status`
- `starts_at`
- `ends_at`
- `display_order`
- `markers`

The `ends_at` field is `null` for an active open lane.
The `markers` field contains the typed marker variants.

The projection includes each finite lane that intersects the window.
The projection includes each active open lane that starts before the window end.
The projection includes an active open lane when the window contains no marker for that lane.

A finite lane intersects the window when `starts_at < window.end` and `ends_at >= window.start`.
A point marker is in the window when `window.start <= at < window.end`.
An interval marker is in the window when `start < window.end` and `end > window.start`.
The projection applies the same half-open rule to all-day local dates.

The projection includes all owner calendars and their calendar visibility values.
The browser uses calendar visibility for the initial lane presentation.
The organizer can show a hidden calendar without a new data import.

The HTML and JSON representations use one projection service.
The server selects a representation from the `Accept` header.
The response includes `Vary: Accept` and `Cache-Control: private, no-store`.
The JSON representation uses `application/json`.
The HTML representation uses `text/html`.

An invalid horizon request returns this error shape:

```json
{"error":{"code":"invalid_time_window","message":"The time window is invalid.","details":{},"request_id":"opaque-request-id"}}
```

## Horizon Presentation

The horizon view renders one row for each lane.
Each lane line is from `12px` through `16px` thick.
Each visible marker has a diameter from `20px` through `24px`.
Each interactive marker has a minimum `44px` target.

The browser clips each lane line to the visible time window.
A future event lane starts at its tracking start and stops at its known end.
The browser renders the lane when the visible section has no marker.

A finite lane has a visible end cap.
An active open lane has a visible continuation arrow.
The presentation uses text and symbols with calendar colors.

The view has sticky lane labels, a time scale, and a today line.
The organizer can pan and change the time scale.
Calendar controls show or hide all lanes for one calendar.

Keyboard operations control pan, scale, calendar visibility, and marker selection.
The same operations stay available at the supported mobile width.

## Resource Routes

New routes use resource nouns and opaque identifiers.
Each organizer route requires authentication and owner authorization.
The machine-readable contract is `api/horizon.openapi.json`.

| Method | Path | Result |
|---|---|---|
| `GET` | `/horizon/` | Read the HTML or JSON Horizon projection. |
| `HEAD` | `/horizon/` | Read the Horizon projection metadata without a body. |
| `POST` | `/calendars/` | Create a calendar. |
| `GET` | `/calendars/{calendar_id}` | Read one calendar. |
| `PATCH` | `/calendars/{calendar_id}` | Change calendar fields, order, or visibility. |
| `DELETE` | `/calendars/{calendar_id}` | Delete an eligible calendar. |
| `POST` | `/lanes/` | Create an open or finite lane. |
| `GET` | `/lanes/{lane_id}` | Read one lane and its relationships. |
| `PATCH` | `/lanes/{lane_id}` | Change, move, reorder, or resolve one lane. |
| `DELETE` | `/lanes/{lane_id}` | Delete an eligible lane. |
| `POST` | `/attention-policies/` | Create an attention policy. |
| `PATCH` | `/attention-policies/{policy_id}` | Change one attention policy. |
| `DELETE` | `/attention-policies/{policy_id}` | Delete one attention policy. |
| `PATCH` | `/probes/{probe_id}` | Record one probe state transition. |
| `POST` | `/calendar-authorization-requests/` | Create a Google Calendar authorization request. |
| `GET` | `/calendar-connection-callbacks/google/` | Validate the Google consent result without a database change. |
| `POST` | `/calendar-connections/` | Exchange the approved code and create a connection. |
| `DELETE` | `/calendar-connections/{connection_id}` | Delete one connection and its credentials. |
| `PUT` | `/calendar-connections/{connection_id}/source-calendars/` | Replace and synchronize the selected source calendars. |
| `POST` | `/derived-marker-rules/` | Create one derived marker rule. |
| `PATCH` | `/derived-marker-rules/{rule_id}` | Change one derived marker rule. |
| `DELETE` | `/derived-marker-rules/{rule_id}` | Delete one derived marker rule. |
| `POST` | `/ingestion-drafts/` | Create one quick or natural-language ingestion draft. |
| `GET` | `/ingestion-drafts/{draft_id}` | Read one ingestion draft. |
| `PATCH` | `/ingestion-drafts/{draft_id}` | Correct one ingestion draft. |
| `DELETE` | `/ingestion-drafts/{draft_id}` | Cancel one ingestion draft. |
| `POST` | `/ingestion-drafts/{draft_id}/confirmations/` | Confirm one draft and create temporal resources. |

Horizon resources accept only their declared standard HTTP methods.
They do not accept an `_method` override.
Draft creation requires a `source` value of `quick` or `natural_language`.
Both representations create the resource at `/ingestion-drafts/{draft_id}`.

An authenticated request to `/` redirects to `/horizon/`.
An unauthenticated request to `/` keeps the public landing page.
The current event, RSVP, venue, QR, and public response routes stay current contracts.

Create operations return `201 Created` with a `Location` header.
Successful delete operations return `204 No Content`.

The server synchronizes each selected source calendar immediately.
The server repeats the reconciliation every five minutes.
The browser does not provide synchronization controls.

A malformed request returns `400 Bad Request`.
A semantically invalid time window returns `422 Unprocessable Content`.
An unsupported representation returns `406 Not Acceptable`.
An unsupported request media type returns `415 Unsupported Media Type`.
Each Horizon API error uses the typed error representation.

The connection, synchronization, and draft confirmation routes require an `Idempotency-Key` header.
RSVP keeps the related idempotency record for 24 hours.
A repeated key with the same request returns the recorded result.
A repeated key with a different request returns `409 Conflict`.

## Deletion Rules

RSVP deletes a calendar only when the calendar has no lane and no source calendar mapping.
Otherwise, RSVP returns `409 Conflict`.

RSVP rejects direct deletion of a source-owned lane.
Calendar synchronization controls the removal of a source-owned lane.

RSVP deletes a local lane only when none of its events has an RSVP.
One transaction deletes its events, probes, policies, derived marker rules, derived markers, and lane.
RSVP returns `409 Conflict` when an RSVP prevents lane deletion.

Connection deletion removes credentials, sync cursors, source mappings, external event links, and eligible source-owned resources.
RSVP returns `409 Conflict` when an RSVP or local dependency uses an imported event.

## Authorization Contract

Each temporal query starts with the authenticated organizer.
The query follows the calendar relationship before it reads or changes a child resource.

RSVP returns `401 Unauthorized` when authentication is absent or invalid.
RSVP returns `403 Forbidden` when another organizer owns the addressed resource.
RSVP returns `404 Not Found` when the addressed resource does not exist.

The public response route uses an opaque RSVP code instead of organizer authentication.
The public response route returns only the invitation fields that the public response page requires.

## Attention Contract

An attention policy belongs to one active open lane.
The attention policy stores a review interval, next probe time, and optional escalation interval.

A pending probe copies the policy next probe time into `due_at`.
When an escalation interval exists, the probe sets `escalates_at` from `due_at`.

One policy occurrence has no more than one pending probe.
Probe completion records the completion time.
The policy sets its next probe time from the completion time and review interval.
The application clock sets a pending probe to missed at `escalates_at`.

When the organizer resolves the lane, RSVP stops future probes.
The resolve transaction cancels the current pending probe.
Completed and missed probes stay as historical markers.

## Google Calendar Contract

Google Calendar consent stays separate from Google sign-in.
The connection requests only these scopes:

- `https://www.googleapis.com/auth/calendar.calendarlist.readonly`
- `https://www.googleapis.com/auth/calendar.events.readonly`

RSVP stores each refresh credential with authenticated encryption.
The encryption key stays in private deployment values.

The authorization request creates the Google authorization URL.
The authorization request stores only a hash of the state value.
The authorization request expires and can create only one calendar connection.
The callback validates the state and returns a connection confirmation form.
The callback does not change the database.
The callback response uses `Cache-Control: no-store` and `Referrer-Policy: no-referrer`.
The confirmation request exchanges the code and stores the encrypted credential.
RSVP does not store the authorization code.

A calendar connection belongs to one organizer.
A source calendar mapping connects one selected source calendar to one RSVP calendar.
RSVP stores one sync cursor for each source calendar mapping.

The first calendar synchronization imports the selected source data.
RSVP stores the first sync cursor only after the last response page commits.
Each later synchronization uses the stored sync cursor and the same query parameters.
RSVP stores a replacement sync cursor only after the last response page commits.
The external event links make each imported occurrence idempotent.

When Google rejects a sync cursor, RSVP clears the cursor and starts a complete synchronization.
The complete synchronization uses external links to reconcile source-owned resources.
The source reconciliation keeps each RSVP identifier and each permitted local relationship.

The calendar provider owns imported event titles, descriptions, times, recurrence data, and deletion state.
RSVP owns calendar visibility, symbols, color tokens, and display order.
RSVP rejects a local change to a source-owned marker field.

Each independent provider event gets one lane.
Each provider event series gets one lane for all imported occurrences.
The external event series link identifies the stable lane for a provider series.
Provider deletions remove the related imported markers and external event links.

RSVP does not write authorization codes, refresh credentials, or input text to logs.
When the organizer deletes a calendar connection, RSVP deletes its stored credentials and sync cursors.

### External Contract References

Use the [Google Calendar API scopes](https://developers.google.com/workspace/calendar/api/auth) for the scope identifiers.
Use the [Google OAuth web server flow](https://developers.google.com/identity/protocols/oauth2/web-server) for state and code exchange.
Use the [Google Calendar synchronization guide](https://developers.google.com/workspace/calendar/api/guides/sync) for page and sync cursor behavior.

## Derived Marker Contract

A derived marker rule uses one timed anchor marker.
The rule selects the anchor start or anchor end.
The rule stores one signed offset in seconds.

RSVP calculates the derived timestamp from the UTC anchor timestamp.
The derived marker keeps the anchor marker timezone for display.
The derived marker uses the anchor marker lane.

The anchor update transaction recalculates each related derived marker.
RSVP rejects a derived marker relationship cycle before persistence.
RSVP rejects direct time changes to a derived marker.

When the organizer deletes a derived marker rule, RSVP deletes its derived marker in one transaction.
The first derived marker contract does not accept an all-day anchor marker.

## Ingestion Draft Contract

An ingestion draft is the only output of quick input and the natural-language parser.
Draft creation does not create a calendar, lane, event, probe, or derived marker.

A draft has `incomplete`, `ready`, `confirmed`, or `canceled` status.
Only a ready draft can receive a confirmation resource.
The organizer can correct each proposed value before confirmation.

An independent event draft proposes a new lane.
A dependent event draft requires an anchor event and proposes the anchor lane.
An open lane draft can propose one attention policy.

Draft confirmation uses one database transaction.
The transaction creates the approved temporal resources and sets the draft status to `confirmed`.
One draft can create no more than one confirmation resource.
Repeated confirmation returns the existing confirmation result.

A canceled draft changes no temporal resource.
An invalid parser response changes no temporal resource.
The natural-language parser uses one explicit reference time and the organizer timezone.
RSVP does not store the original natural-language input.

The parser adapter sends the input text, reference time, and organizer timezone.
The adapter authenticates with a key from private deployment values.
The adapter accepts only the current JSON response schema.
RSVP calculates the missing required fields from the validated response.

A natural-language draft stores its source and missing field names.
A dated draft can store proposed relative marker rules.
Draft confirmation creates each approved rule from the new event anchor.
The event and its derived markers use one lane.

## Fresh Schema Contract

RSVP initializes an empty SQLite database with the complete canonical schema.
Schema initialization occurs in one database transaction.

RSVP opens an existing database only when each canonical table and required column is present.
The event table must use lane ownership and the closed event constraints.
An event-only database is outside the current runtime contract.

The runtime contains no schema conversion operation.
Startup contains no data repair or schema update path.

## Cutover Sequence

1. Complete I001 contract coverage.
2. Approve this P001 contract and its acceptance matrix.
3. Start with an empty database or a complete canonical database.
4. Initialize the canonical schema in one transaction when the database is empty.
5. Use the client timezone for the first temporal write.
6. Make sure that calendar, lane, event, RSVP, and venue relationships are correct.
7. Make sure that the protected RSVP response and QR flows operate correctly.
8. Make sure that the horizon projection and owner boundaries are correct.

## Product Decisions

This contract has no unresolved product choice.
The client supplies each timezone default and sends one explicit IANA timezone.

Any later contract change requires an issue and an atomic update to this document.
