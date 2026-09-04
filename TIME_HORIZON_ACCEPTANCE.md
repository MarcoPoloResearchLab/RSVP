# Time Horizon Acceptance Matrix

This matrix connects the approved [time horizon architecture](ARCHITECTURE.md) to implementation evidence.
The issue identifiers refer to [.mprlab/ISSUES.md](.mprlab/ISSUES.md).

## Data And Schema

| ID | Contract behavior | Required evidence | Issue |
|---|---|---|---|
| TH-001 | Each organizer owns calendars. | Model constraints and owner query tests. | I002 |
| TH-002 | Each lane belongs to one calendar. | Model constraint and schema tests. | I002 |
| TH-003 | Each independent event owns one lane. | Relationship and creation tests. | I002, F003 |
| TH-004 | One event series uses one lane. | Series occurrence tests. | I002, F006 |
| TH-005 | One dependency chain uses its anchor lane. | Relationship and cycle tests. | I002, F003 |
| TH-006 | Each probe uses its policy lane. | Model constraint and cadence tests. | I002, F004 |
| TH-007 | Child resources use calendar ownership. | Cross-owner query and handler tests. | I002, F003 |
| TH-008 | Events have no duplicate organizer field. | Schema inspection and owner query tests. | I002 |
| TH-009 | Open and finite lanes reject invalid states. | Constructor and database constraint tests. | I002 |
| TH-010 | Lane resolution creates one finite end. | Transaction and state transition tests. | F003, F004 |
| TH-011 | Timed markers keep an IANA timezone. | Constructor, storage, and serialization tests. | I002 |
| TH-012 | All-day markers keep local date bounds. | Date and daylight time tests. | I002, F006 |
| TH-013 | Each organizer has a stored timezone after its first temporal write. | Setup and invalid timezone tests. | I002 |
| TH-014 | Each temporal write must have a client-supplied timezone. | HTTP rejection and daylight time tests. | I002 |
| TH-015 | Canonical fixtures keep RSVP relationships. | Fixture relationship and public route tests. | I002, I003 |
| TH-016 | Temporal creation rollback is atomic. | Forced event failure test. | I002 |
| TH-017 | The runtime uses only canonical schema initialization. | Source scan and clean startup test. | I002, I003 |
| TH-018 | A lane starts when RSVP starts to track its subject. | Creation, import, and projection tests. | I002, F003, F006 |
| TH-019 | A finite event lane ends at its event marker end. | Creation and update tests. | I002 |

## Projection And View

| ID | Contract behavior | Required evidence | Issue |
|---|---|---|---|
| TH-020 | The default window has 90 local calendar days. | HTTP projection test with a fixed clock. | F001 |
| TH-021 | The maximum request window is 366 calendar days. | Boundary and rejection tests. | F001 |
| TH-022 | The projection includes each finite lane that intersects the window. | Projection query test. | F001 |
| TH-023 | The projection includes an active open lane without markers. | Empty-lane projection test. | F001 |
| TH-024 | Projection data stays within owner scope. | Cross-owner HTTP integration test. | F001 |
| TH-025 | HTML and JSON use one projection service. | Representation, header, and cache tests. | F001 |
| TH-026 | Invalid windows use the typed error shape. | HTTP status and body tests. | F001 |
| TH-027 | A new session keeps calendar visibility. | Update and new-session browser tests. | F002, F003 |
| TH-028 | A new session keeps calendar and lane order. | Reorder and new-session tests. | F003 |
| TH-029 | Hidden calendars stay available to controls. | Projection and browser interaction tests. | F001, F002 |
| TH-030 | Each lane is 12 to 16 pixels thick. | Browser geometry test. | F002 |
| TH-031 | Each marker is 20 to 24 pixels wide. | Browser geometry test. | F002 |
| TH-032 | Each marker target is at least 44 pixels. | Browser target geometry test. | F002 |
| TH-033 | Finite and open lanes have different ends. | Browser visual state test. | F002 |
| TH-034 | Keyboard operations control the horizon view. | Real-browser keyboard test. | F002 |
| TH-035 | The browser renders the horizon view at mobile width. | Real-browser responsive test. | F002 |
| TH-036 | Authenticated home opens the horizon view. | Registered-route browser test. | F002 |
| TH-037 | A future marker has a visible lane before its marker time. | Empty-section browser geometry test. | F001, F002 |
| TH-102 | The heading has no tagline. | HTML and browser presentation tests. | I008 |
| TH-103 | The time window row contains the range and view controls. | Desktop and mobile browser geometry tests. | I009 |
| TH-104 | The time window row is directly above the timeline. | HTML order and browser position tests. | I010 |
| TH-105 | Help contains the keyboard instructions. | Settings location and keyboard navigation tests. | I011 |
| TH-106 | Calendar resources and views contain no symbol. | Schema, HTTP, projection, import, and browser tests. | I012 |
| TH-107 | Direct controls select the day, week, month, and year scales. | HTML and browser interaction tests. | I013 |
| TH-108 | Each scale controls the visible window and stays selected after a later load. | HTTP and browser window tests. | B051 |

## Management And Attention

| ID | Contract behavior | Required evidence | Issue |
|---|---|---|---|
| TH-040 | Calendar operations use resource routes. | HTTP method and status tests. | F003 |
| TH-041 | Lane operations use resource routes. | HTTP method and status tests. | F003 |
| TH-042 | Calendar changes do not change lane membership. | Relationship test after calendar move. | F003 |
| TH-043 | Another organizer cannot change a resource. | Cross-owner HTTP tests. | F003, F004 |
| TH-044 | One policy has one pending probe occurrence. | Unique constraint and cadence tests. | F004 |
| TH-045 | Probe completion sets the next policy time. | Fixed-clock state transition test. | F004 |
| TH-046 | Escalation marks an overdue probe missed. | Fixed-clock escalation test. | F004 |
| TH-047 | Lane resolution stops future probes. | Transaction and fixed-clock tests. | F004 |
| TH-048 | Historical probes stay visible markers. | Projection state test. | F001, F004 |
| TH-049 | Calendar and lane deletion obey dependency rules. | Conflict, commit, and rollback tests. | F003 |

## Google Calendar

| ID | Contract behavior | Required evidence | Issue |
|---|---|---|---|
| TH-050 | Calendar consent is separate from sign-in. | Authorization flow and callback database snapshot tests. | F005 |
| TH-051 | The connection requests the two approved read-only scopes. | Authorization request contract test. | F005 |
| TH-052 | RSVP encrypts stored refresh credentials. | Storage and key failure tests. | F005 |
| TH-053 | Logs contain no authorization secrets. | Captured log test. | F005 |
| TH-054 | One provider calendar semantic group connects to one RSVP calendar. | Mapping constraint and automatic reconciliation tests. | F005, I006, B045, B047 |
| TH-055 | Each successful provider calendar synchronization stores its last-page sync cursor. | Pagination and transaction tests. | F006, B047 |
| TH-056 | Initial synchronization creates no duplicate marker. | Repeated initial synchronization test. | F006 |
| TH-057 | Incremental synchronization uses the sync cursor. | Deterministic provider update test. | F006 |
| TH-058 | Provider updates change source-owned fields. | Imported marker update test. | F006 |
| TH-059 | Local changes cannot change source-owned fields. | Rejected update test. | F006 |
| TH-060 | Provider deletions remove imported markers. | Deterministic provider deletion test. | F006 |
| TH-061 | All-day imports keep the correct local date. | Timezone and daylight time tests. | F006 |
| TH-062 | Imported event occurrences use one event series lane. | Multiple occurrence projection test. | F006 |
| TH-063 | Connection deletion removes eligible provider resources. | Database deletion, conflict, and log tests. | F005, F006 |
| TH-064 | A rejected sync cursor starts one complete source reconciliation. | Provider error and stable-identifier tests. | F006 |
| TH-065 | Authorization callbacks are safe and have no stored code. Connection creation confirms the browser timezone or uses `UTC`. | Database snapshot, header, timezone, and storage tests. | F005, B047 |
| TH-066 | Connection completion imports all readable CalendarList entries, including hidden entries. | HTTP and browser connection tests with Holidays and Family calendars. | I006 |
| TH-067 | Each successful CalendarList reconciliation stores its last-page cursor after all mapping changes commit. | Adapter pagination and transaction tests. | I006 |
| TH-068 | A source rename keeps the mapping identifier and changes only the RSVP calendar name. | Incremental reconciliation and stable-identifier tests. | I006 |
| TH-069 | Google selection sets the initial visibility. Later reconciliation keeps local presentation changes. | Service and browser visibility tests. | I006, B045 |
| TH-070 | A new source creates one mapping and calendar. An eligible deleted source removes both resources. | Incremental add and delete tests. | I006 |
| TH-071 | Local use stops source deletion, records a failed synchronization, and keeps the CalendarList cursor. | Conflict, synchronization record, and cursor tests. | I006 |
| TH-072 | A rejected CalendarList cursor starts one complete CalendarList reconciliation. | Provider rejection and full reconciliation tests. | I006 |
| TH-073 | The first complete import removes prior unmapped calendar groupings. | Service and browser cutover tests. | B045 |
| TH-074 | Primary-calendar birthday events appear only in the `Birthdays` calendar. | Provider normalization, service, and browser event tests. | B045 |
| TH-075 | At most eight calendars are visible. Visibility and calendar additions do not change an assigned presentation color. | High-cardinality browser presentation and stability tests. | B045, B047 |
| TH-076 | Raw and unknown Google event types do not create RSVP group names. | Provider normalization test with a future event type. | B045 |
| TH-077 | An explicit birthday title maps to `Birthdays` when Google supplies no birthday metadata. | Provider and browser event tests. | B045 |
| TH-078 | A provider event moves between semantic groups when its meaning changes. | Incremental provider and browser event tests. | B045 |
| TH-079 | The immediate predecessor schema migrates provider identity to the sync state and clears both cursor types. | Database migration test. | B047 |
| TH-080 | One provider calendar synchronization requests one unfiltered event feed. | Adapter and synchronization request-count tests. | B047 |
| TH-081 | The Contacts semantic source creates no visible source calendar. | Reconciliation and browser tests. | B047 |
| TH-082 | Birthday metadata and complete birthday title words map only to `Birthdays`. | Classification matrix and browser tests. | B047 |
| TH-083 | Anniversary, general, known, and unknown event types stay in the source calendar. | Classification matrix and browser tests. | B047 |
| TH-084 | A semantic group change keeps one RSVP event and one external event link. A recurring exception keeps the provider series on one lane. | Forward, reverse, and mixed-series move tests. | B047 |
| TH-085 | A sparse cancellation deletes the event from all semantic group mappings. | Cancellation transaction test. | B047 |
| TH-086 | A rejected event cursor starts one complete provider calendar reconciliation. | Cursor rejection and absent-event tests. | B047 |
| TH-087 | Each local startup deletes and creates the `rsvp-data` volume before service startup. | Local startup reset and health validation. | B047 |
| TH-088 | Connection creation returns an active task before source or event import completes. | HTTP task response and separate scheduler-cycle tests. | B048 |
| TH-089 | The callback and Integrations rubric show the initial import task without an operating-system wait cursor. | HTML and browser task-state tests. | B048 |

## Derived Markers And Ingestion

| ID | Contract behavior | Required evidence | Issue |
|---|---|---|---|
| TH-070 | A derived marker uses its anchor lane. | Relationship constraint test. | F007 |
| TH-071 | A rule supports timed anchor start and end. | Offset and all-day rejection tests. | F007 |
| TH-072 | Anchor updates recalculate derived markers. | Forced rollback and commit tests. | F007 |
| TH-073 | RSVP rejects derived relationship cycles. | Cycle constructor and handler tests. | F007 |
| TH-074 | Direct derived time changes fail. | HTTP validation test. | F007 |
| TH-075 | Rule deletion removes its marker. | Transaction test. | F007 |
| TH-076 | Draft creation changes no temporal resource. | Database snapshot test. | F008 |
| TH-077 | An independent event draft proposes a new lane. | Draft schema test. | F008 |
| TH-078 | A dependent event draft proposes its anchor lane. | Draft relationship test. | F008 |
| TH-079 | An open lane draft can include attention. | Draft schema and browser tests. | F008 |
| TH-080 | Draft confirmation creates resources atomically. | Forced rollback and commit tests. | F008 |
| TH-081 | One draft creates one confirmation. | Repeated confirmation test. | F008 |
| TH-082 | Draft cancellation changes no temporal resource. | Database snapshot test. | F008 |
| TH-083 | The parser uses one reference time and timezone. | Deterministic parser fixture. | F009 |
| TH-084 | Invalid provider data changes no resource. | Invalid response and database snapshot test. | F009 |
| TH-085 | RSVP does not store or log natural-language input. | Database snapshot and captured log tests. | F009 |
| TH-086 | Retry-sensitive creation has one result for each idempotency key. | Repeated and conflicting request tests. | F005, F006, F008 |

## Protected RSVP And Final Acceptance

| ID | Contract behavior | Required evidence | Issue |
|---|---|---|---|
| TH-090 | Event identifiers stay stable in canonical operations. | Canonical fixture comparison. | I002, I003 |
| TH-091 | RSVP public codes stay stable in canonical operations. | Canonical fixture comparison. | I002, I003 |
| TH-092 | The public response route stays available. | Registered public HTTP test. | I001, I003 |
| TH-093 | Public response updates stay available. | Registered public HTTP update test. | I001, I003 |
| TH-094 | QR images contain the public response URL. | Decoded QR payload test. | I001, I003 |
| TH-095 | Venue relationships stay stable in canonical operations. | Canonical fixture comparison. | I002, I003 |
| TH-096 | No obsolete temporal contract exists. | Source, schema, route, and asset scan. | I003 |
| TH-097 | Desktop and mobile browsers render the complete horizon. | Deterministic end-to-end suite. | I003 |
| TH-098 | A new organizer gets Horizon setup before the default window. | HTML setup, typed JSON error, and browser tests. | B043 |
| TH-099 | Horizon uses one REST interface for methods, errors, and draft creation. | Registered HTTP and browser contract tests. | B044 |
| TH-100 | Account settings change the organizer timezone without a change to stored marker timezones. | Organizer resource, HTML, and browser tests. | F011 |
| TH-101 | A finite lane uses the same height for its body and circular end. | Browser geometry test. | B049 |

## Completion Rule

An implementation issue closes only after its assigned evidence is valid.
I003 closes only after each row has valid evidence or an approved exclusion.
Each exclusion requires a new planning issue and an architecture update.

## I003 Evidence

The deterministic browser suite is `tests/browser/horizon.spec.js`.
It uses the production-like fixture in `internal/browserfixture/main.go`.

The suite verifies desktop and mobile presentation.
It verifies the time window row position and the Help keyboard instructions.
It also verifies keyboard access, labels, focus order, and color-independent meaning.

The suite verifies initial and incremental Google Calendar synchronization.
It verifies attention, derived markers, quick drafts, and natural-language drafts.
It verifies the QR page and the public RSVP response flow.

The schema record is [TIME_HORIZON_SCHEMA_RECORD.md](TIME_HORIZON_SCHEMA_RECORD.md).
The user procedure is [USER_GUIDE.md](USER_GUIDE.md).
The operator procedure is [OPERATOR_RUNBOOK.md](OPERATOR_RUNBOOK.md).
