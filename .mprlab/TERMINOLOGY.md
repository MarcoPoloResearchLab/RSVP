# Repository Terminology

This file contains the approved technical nouns and technical verbs for repository documentation.

Use this file with `.mprlab/AGENTS.DOCS.md` and ASD-STE100 Simplified Technical English, Issue 9.

Do not add a general dictionary word to this file. Use the ASD-STE100 dictionary for general words.

Give each term one meaning. Use the same term for the same concept in all documents.

## MPR Lab Technical Nouns

- `acceptance criteria`: Conditions that show that a change has the necessary behavior.
- `active issue tracker`: The canonical file that contains current work.
- `ADR`: An architecture decision record.
- `adapter`: A code unit that connects a core module to an external system.
- `agent guide`: A file that gives binding instructions to an agent.
- `API`: A repository-owned application programming interface.
- `API contract`: The canonical schema and behavior of an API.
- `ASD-STE100`: The Simplified Technical English standard for technical documentation.
- `architecture`: The structure, boundaries, and ownership of a software system.
- `App Store Connect`: The Apple service that receives and manages iOS store artifacts.
- `artifact`: A file or image that a build, release, or generator creates.
- `backlog`: The set of unresolved issues in the active issue tracker.
- `backend client`: A code unit that sends requests to a backend.
- `browser frontend`: A user interface that operates in a web browser.
- `build`: A process or output that converts source code into an artifact.
- `changelog`: A file that records completed changes for releases.
- `CI`: The repository continuous-integration system.
- `CLI`: A command-line interface.
- `code path`: A sequence of operations in source code.
- `config`: Source-controlled configuration data.
- `container`: An isolated runtime package with an application and its dependencies.
- `contract`: A binding definition of behavior, data, or ownership.
- `credential`: A private value that an external service uses to authenticate an identity.
- `documentation`: Technical information in repository documents.
- `coverage`: Evidence that tests exercise specified behavior.
- `dependency`: An external or internal component that a system requires.
- `deployment`: An operation that changes a runtime environment.
- `domain type`: A type that represents validated domain data.
- `EAS`: Expo Application Services for hosted build, submission, and update operations.
- `endpoint`: One HTTP API address and its operation.
- `end user`: The person who requests or receives the agent work.
- `environment file`: A private file that contains environment variable assignments.
- `environment variable`: A named process input.
- `Expo`: A framework and source config system for React Native mobile clients.
- `Expo CLI`: The Expo command-line tool for local development and native project generation.
- `Google Play`: The Google service that receives and manages Android store artifacts.
- `issue`: One tracked unit of work.
- `issue tracker`: A file or system that contains issues.
- `language checker`: A tool that finds specified language errors.
- `language review`: An agent-owned examination of text against language rules and terminology.
- `manifest`: A source-controlled file that declares resources or configuration.
- `mobile client`: An application for a mobile platform.
- `mobile store artifact`: A signed `.ipa` or `.aab` file for store publication.
- `native toolchain`: The platform tools that build and sign a mobile store artifact.
- `payload`: Structured data that crosses a system boundary.
- `PDF`: A file that uses the Portable Document Format.
- `PRD`: A product requirement document.
- `private input channel`: A documented process environment, anonymous pipe, or private file input.
- `producing agent`: The agent that creates or changes technical prose.
- `pull request`: A proposed Git change for review and merge.
- `repository`: A source-controlled project and its files.
- `reference cache`: A private local directory that stores a verified official reference.
- `route`: An API or user-interface address and its handler.
- `runbook`: A technical procedure for an operator or agent.
- `runtime`: An operating instance of a service or application.
- `schema`: A machine-readable definition of structured data.
- `SHA-256`: A cryptographic digest that identifies the verified official reference.
- `source code`: Human-readable instructions that define software behavior.
- `source blocker`: A failure that prevents access to a necessary official source.
- `stack guide`: An agent guide for one language, framework, or runtime.
- `STE reference`: The verified official ASD-STE100 PDF that controls a language review.
- `store publisher`: A repository-owned tool that submits a mobile store artifact directly to its store.
- `technical document`: A repository document that contains technical information or instructions.
- `technical noun`: A subject-field noun that the repository approves.
- `technical prose`: English technical text outside code and source-controlled literals.
- `technical verb`: A subject-field verb that the repository approves.
- `validation`: Evidence that a change obeys its current contract.
- `worktree`: A Git checkout that has its own working directory.

## Repository Technical Nouns

Add repository-specific technical nouns below this line.

```text
- `term`: Definition with one meaning.
```

- `anchor marker`: A marker that supplies the base time for a derived marker.
- `active lane`: A lane that can receive state changes and future markers.
- `all-day marker`: A marker that uses a local date range without a time of day.
- `all-day event`: An event that uses a local date without a time of day.
- `attention policy`: Rules that specify the next probe and its escalation time for a lane.
- `browser test`: An automated test that operates the browser frontend.
- `calendar`: A user-owned visibility family that groups lanes without control of lane membership.
- `calendar authorization request`: A resource that starts Google Calendar consent without a calendar connection.
- `calendar connection`: An authorized link between RSVP and an external calendar provider.
- `calendar provider`: An external service that supplies source calendars and provider events.
- `calendar synchronization`: An operation that imports changes from one source calendar.
- `calendar visibility`: A saved choice that shows or hides one calendar in the horizon view.
- `daylight time`: A local clock change that an IANA timezone defines.
- `dependency chain`: An anchor event and related dependent events on one lane.
- `dependent event`: An event with an explicit dependency that shares a lane with its anchor event.
- `draft confirmation`: A resource that creates temporal resources from one approved ingestion draft.
- `derived marker`: A marker whose time uses an offset from an anchor marker.
- `derived marker rule`: A stored offset relationship between an anchor marker and a derived marker.
- `event occurrence`: One dated instance of an event series.
- `event series`: One recurring event with all event occurrences on one lane.
- `external event link`: A relationship between one RSVP event and one provider event.
- `finite lane`: A lane with an end time.
- `horizon projection`: Structured calendar, lane, and marker data for one time window.
- `horizon view`: The browser frontend that shows calendars, lanes, and markers on a time axis.
- `idempotency record`: A temporary record that connects one request key to one operation result.
- `ingestion draft`: Validated proposed temporal data that has not changed persisted data.
- `independent event`: An event without an explicit dependency that owns one lane.
- `interval marker`: A timed marker with a start time and an end time.
- `lane`: A timeline row for one independent event, one event series, or one dependency chain.
- `lane membership`: The relationship of one independent event, event series, or dependency chain to one lane.
- `local wall time`: A local clock date and time that does not contain timezone rules.
- `marker`: A visible point or interval for one event occurrence, probe, or derived marker.
- `natural-language parser`: An adapter that changes text into an ingestion draft.
- `open lane`: A lane without an end time.
- `opaque identifier`: A resource identifier whose value has no client-visible meaning.
- `organizer`: An authenticated RSVP user who owns temporal resources.
- `organizer timezone`: The IANA timezone that RSVP uses for an organizer's local dates and default time window.
- `point marker`: A timed marker with a start time and no end time.
- `probe`: A dated review action for a lane.
- `provider event`: An event that a calendar provider owns.
- `resolved lane`: A lane with a recorded resolution time and no future probe.
- `source calendar`: An external calendar that an organizer selects for import.
- `source calendar mapping`: A relationship between one source calendar and one RSVP calendar.
- `source reconciliation`: A complete synchronization that compares provider data with source-owned RSVP resources.
- `source-owned marker`: An imported marker with fields that a calendar provider owns.
- `sync cursor`: Provider data that identifies the next incremental calendar synchronization position.
- `temporal resource`: A calendar, lane, event, probe, attention policy, or derived marker.
- `time horizon`: The product contract that represents dated events and unresolved temporal subjects.
- `time shape`: One closed marker time representation for a point, interval, or all-day date range.
- `time window`: A bounded interval for one horizon projection.
- `timezone`: A named IANA rule set for local time interpretation.

## MPR Lab Technical Verbs

- `archive`: Move completed history from the active issue tracker to durable storage.
- `authenticate`: Confirm the identity of a client or user.
- `authorize`: Confirm that an identity can do an operation on a resource.
- `build`: Convert source code into an executable or generated artifact.
- `cache`: Store a verified reference outside a target repository for repeated use.
- `commit`: Record a Git change in repository history.
- `configure`: Set source-controlled values that control system behavior.
- `deploy`: Change a runtime environment to use a specified artifact and configuration.
- `file`: Add an issue to the active issue tracker.
- `generate`: Create an artifact from its canonical source.
- `lint`: Use static rules to find source or document errors.
- `merge`: Add the changes from a pull request to its target branch.
- `normalize`: Change a file to obey one canonical format or contract.
- `parse`: Convert input data into a typed internal value.
- `publish`: Make an artifact available outside the source repository.
- `refactor`: Change code structure without a change to public behavior.
- `regenerate`: Create a generated artifact again from its canonical source.
- `redistribute`: Provide a third-party reference outside its approved distribution method.
- `render`: Convert source data into a visible or machine-readable output.
- `retrieve`: Get an official reference from its approved source.
- `review`: Examine an artifact against its requirements and record the result.
- `scan`: Use an automated process to find specified source patterns.
- `serialize`: Convert a typed value into a transport or storage representation.
- `validate`: Confirm that an input or artifact obeys its contract.
- `verify`: Confirm a result at its public or runtime boundary.

Use the simple present, simple past, simple future, imperative, or infinitive form of these verbs.

## Repository Technical Verbs

Add repository-specific technical verbs below this line.

```text
- `term`: Definition with one meaning and the approved verb forms.
```

- `approve`: Accept a contract or draft as the current choice. Approved forms: `approve`, `approves`, `approved`.
- `assign`: Connect one resource to its required owner or parent resource. Approved forms: `assign`, `assigns`, `assigned`.
- `belong`: Have one required parent resource. Approved forms: `belong`, `belongs`, `belonged`.
- `cancel`: Stop a pending probe or ingestion draft. Approved forms: `cancel`, `cancels`, `canceled`.
- `confirm`: Accept an ingestion draft and create its temporal resources. Approved forms: `confirm`, `confirms`, `confirmed`.
- `construct`: Create a valid domain type. Approved forms: `construct`, `constructs`, `constructed`.
- `create`: Add one new persisted resource. Approved forms: `create`, `creates`, `created`.
- `define`: Specify one canonical meaning or contract. Approved forms: `define`, `defines`, `defined`.
- `delete`: Make one persisted resource absent. Approved forms: `delete`, `deletes`, `deleted`.
- `derive`: Calculate a marker time from an anchor marker. Approved forms: `derive`, `derives`, `derived`.
- `encrypt`: Protect a credential with authenticated encryption. Approved forms: `encrypt`, `encrypts`, `encrypted`.
- `enforce`: Reject a state or operation that does not obey a contract. Approved forms: `enforce`, `enforces`, `enforced`.
- `exchange`: Send an authorization code and receive provider credentials. Approved forms: `exchange`, `exchanges`, `exchanged`.
- `fail`: End an operation without its required result. Approved forms: `fail`, `fails`, `failed`.
- `group`: Put related lanes in one calendar. Approved forms: `group`, `groups`, `grouped`.
- `hide`: Remove a calendar from the current horizon presentation. Approved forms: `hide`, `hides`, `hidden`.
- `implement`: Add source code for an approved contract. Approved forms: `implement`, `implements`, `implemented`.
- `import`: Create RSVP temporal resources from provider events. Approved forms: `import`, `imports`, `imported`.
- `interpret`: Apply timezone rules to a local wall time. Approved forms: `interpret`, `interprets`, `interpreted`.
- `intersect`: Share one or more instants with a time window. Approved forms: `intersect`, `intersects`, `intersected`.
- `migrate`: Change persisted data to the canonical schema. Approved forms: `migrate`, `migrates`, `migrated`.
- `own`: Have authorization control of a resource. Approved forms: `own`, `owns`, `owned`.
- `permit`: Accept a specified state or operation. Approved forms: `permit`, `permits`, `permitted`.
- `preserve`: Keep a value or relationship unchanged during migration. Approved forms: `preserve`, `preserves`, `preserved`.
- `propose`: Put a candidate value in an ingestion draft. Approved forms: `propose`, `proposes`, `proposed`.
- `recalculate`: Calculate a derived marker again after an anchor change. Approved forms: `recalculate`, `recalculates`, `recalculated`.
- `reconcile`: Compare provider data with source-owned RSVP resources and apply the differences. Approved forms: `reconcile`, `reconciles`, `reconciled`.
- `reference`: Store an explicit relationship to another resource. Approved forms: `reference`, `references`, `referenced`.
- `represent`: Supply a specified model or visible form for a resource. Approved forms: `represent`, `represents`, `represented`.
- `redirect`: Send an HTTP client to another route. Approved forms: `redirect`, `redirects`, `redirected`.
- `reorder`: Change the display order of resources. Approved forms: `reorder`, `reorders`, `reordered`.
- `require`: Make a field, input, or relationship necessary. Approved forms: `require`, `requires`, `required`.
- `resolve`: Close an active open lane at one resolution time. Approved forms: `resolve`, `resolves`, `resolved`.
- `return`: Send an HTTP response to a client. Approved forms: `return`, `returns`, `returned`.
- `store`: Write a resource or value to the database. Approved forms: `store`, `stores`, `stored`.
- `support`: Accept an approved resource shape or operation. Approved forms: `support`, `supports`, `supported`.
- `synchronize`: Apply source calendar changes to RSVP temporal resources. Approved forms: `synchronize`, `synchronizes`, `synchronized`.
- `update`: Change one persisted resource. Approved forms: `update`, `updates`, `updated`.
