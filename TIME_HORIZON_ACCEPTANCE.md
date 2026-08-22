# Time Horizon Acceptance Matrix

This matrix connects the approved [time horizon architecture](ARCHITECTURE.md) to implementation evidence.
The issue identifiers refer to [.mprlab/ISSUES.md](.mprlab/ISSUES.md).

## Data And Migration

| ID | Contract behavior | Required evidence | Issue |
|---|---|---|---|
| TH-001 | Each organizer owns calendars. | Model constraints and owner query tests. | I002 |
| TH-002 | Each lane belongs to one calendar. | Model constraint and migration tests. | I002 |
| TH-003 | Each independent event owns one lane. | Migration and creation tests. | I002, F003 |
| TH-004 | One event series uses one lane. | Series occurrence tests. | I002, F006 |
| TH-005 | One dependency chain uses its anchor lane. | Relationship and cycle tests. | I002, F003 |
| TH-006 | Each probe uses its policy lane. | Model constraint and cadence tests. | I002, F004 |
| TH-007 | Child resources use calendar ownership. | Cross-owner query and handler tests. | I002, F003 |
| TH-008 | Events have no duplicate organizer field. | Schema inspection and migration tests. | I002 |
| TH-009 | Open and finite lanes reject invalid states. | Constructor and database constraint tests. | I002 |
| TH-010 | Lane resolution creates one finite end. | Transaction and state transition tests. | F003, F004 |
| TH-011 | Timed markers keep an IANA timezone. | Constructor, storage, and serialization tests. | I002 |
| TH-012 | All-day markers keep local date bounds. | Date and daylight time tests. | I002, F006 |
| TH-013 | Each organizer has a confirmed timezone. | Setup and invalid timezone tests. | I002 |
| TH-014 | Migration requires one timezone and preserves local wall times. | Command, daylight time, and fixture tests. | I002 |
| TH-015 | Migration preserves RSVP relationships. | Fixture row and relationship comparison. | I002, I003 |
| TH-016 | Migration rollback is atomic. | Forced failure migration test. | I002 |
| TH-017 | The canonical runtime has no migration bridge. | Source scan and clean startup test. | I002, I003 |
| TH-018 | A lane starts when RSVP starts to track its subject. | Creation, import, and projection tests. | I002, F003, F006 |
| TH-019 | Migrated lane bounds preserve the event history and end. | Migration fixture comparison. | I002 |

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
| TH-054 | One source calendar connects to one RSVP calendar. | Mapping constraint and interface tests. | F005 |
| TH-055 | Each successful mapping synchronization stores its last-page sync cursor. | Pagination and transaction tests. | F006 |
| TH-056 | Initial synchronization creates no duplicate marker. | Repeated initial synchronization test. | F006 |
| TH-057 | Incremental synchronization uses the sync cursor. | Deterministic provider update test. | F006 |
| TH-058 | Provider updates change source-owned fields. | Imported marker update test. | F006 |
| TH-059 | Local changes cannot change source-owned fields. | Rejected update test. | F006 |
| TH-060 | Provider deletions remove imported markers. | Deterministic provider deletion test. | F006 |
| TH-061 | All-day imports keep the correct local date. | Timezone and daylight time tests. | F006 |
| TH-062 | Imported event occurrences use one event series lane. | Multiple occurrence projection test. | F006 |
| TH-063 | Connection deletion removes eligible provider resources. | Database deletion, conflict, and log tests. | F005, F006 |
| TH-064 | A rejected sync cursor starts one complete source reconciliation. | Provider error and stable-identifier tests. | F006 |
| TH-065 | Authorization callbacks are safe and have no stored code. | Database snapshot, header, and storage tests. | F005 |

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
| TH-090 | Event identifiers stay unchanged. | Migration fixture comparison. | I002, I003 |
| TH-091 | RSVP public codes stay unchanged. | Migration fixture comparison. | I002, I003 |
| TH-092 | The public response route stays available. | Registered public HTTP test. | I001, I003 |
| TH-093 | Public response updates stay available. | Registered public HTTP update test. | I001, I003 |
| TH-094 | QR images contain the public response URL. | Decoded QR payload test. | I001, I003 |
| TH-095 | Venue relationships stay unchanged. | Migration fixture comparison. | I002, I003 |
| TH-096 | No obsolete temporal contract exists. | Source, schema, route, and asset scan. | I003 |
| TH-097 | Desktop and mobile browsers render the complete horizon. | Deterministic end-to-end suite. | I003 |

## Completion Rule

An implementation issue closes only after its assigned evidence is valid.
I003 closes only after each row has valid evidence or an approved exclusion.
Each exclusion requires a new planning issue and an architecture update.
