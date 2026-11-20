# Option 1: Layered architecture

The repository-architecture decision is pending. This candidate expresses the
same agreed logical owners as the other four: combined Identity; attendance
planning and claim commitment inside Booking; separate Exchange and Events.
Its package organisation and dependency rules remain specific to this style.
[Identity](../../adr/0026-combine-workforce-identity-and-access-ownership.md),
[Booking planning](../../adr/0027-keep-attendance-planning-inside-booking.md) and
[Exchange](../../adr/0028-separate-exchange-synchronisation-from-booking.md)
record those ownership decisions independently of repository style.

SpaceWeave manages desks, meeting rooms, parking, events and hosted visitors for
one organisation with approximately 100,000 staff and 60,000 desks. This candidate
organises the Go backend by technical responsibility while preserving one local
booking authority, asynchronous provider reconciliation and separate web portals.

The architectural basis is separation into logical layers and a declared dependency
policy. Logical layers can share a process; physical tiers are a separate concern.
[Microsoft's layered/N-tier guidance](https://learn.microsoft.com/en-us/azure/architecture/guide/architecture-styles/n-tier)
and the [architecture foundations](architecture-foundations.md) establish the
principles. The persistence-owned interfaces and exact package names below are
SpaceWeave design choices.

Read the [common requirements](README.md#requirements-held-constant) and
[comparison scenarios](README.md#common-comparison-scenarios) alongside this
candidate. Paths are proposed destinations; `←` marks existing code to adapt.
The detailed trees support ownership and change-impact review.

## Summary

One organisation spans many offices, targeting 100,000 staff and 60,000 desks.
All common booking, authority, privacy and recovery requirements remain fixed.

Three horizontal layers sit over a shared model and pure support libraries.
**Transport** translates HTTP, CLI, scheduled triggers and claimed jobs into
application calls. **Application** makes business decisions, coordinates sibling
services and owns transaction boundaries. **Persistence** declares storage and
provider interfaces beside their implementations. Subjects repeat under these
technical roots: room handling lives in transport, application, model and
persistence rather than inside a self-contained Rooms module.

This candidate places persistence interfaces beside their implementations.
Consumer-owned ports are another valid dependency policy. The ownership and
sibling-call restrictions below are additional SpaceWeave safeguards.

Booking alone owns dated availability/transitions across application, model,
store facets, SQL and migrations. Rules/review enforce that logical ownership.
Booking owns demand, private planning and capacity commitment. A solver interface
isolates calculation outside long transactions while the allocation role scales
independently. Identity combines people/source facts, sessions and delegation;
Exchange separately owns calendar integration state and progress.

Dependency rules:

1. `model` and `support` import none of transport, application, persistence or app.
2. Persistence imports model/support, never application, transport or app.
3. Application imports model/support and persistence interface roots, never
   transport, generated API packages, concrete provider/storage packages or app.
4. Transport imports application/model/support, never storage/provider packages.
   Asset reads pass through Plans so authorisation has one application owner.
5. App constructs concrete dependencies and registers job handlers. Only cmd and
   app import app; cmd remains an entry point, not a second composition root.
6. Public `pkg` packages import neither internal nor cmd. Generated clients may
   depend on public transport schemas, never internal booking contracts.
7. Within application, calls follow a reviewed acyclic allowlist. Only Booking
   uses dated-space claim mutation facets; only Events can invoke its venue transaction
   entry point; only the named coordinators can open shared business transactions.
8. SQL and migration ownership follows a checked manifest, even though files sit
   in common technical packages. A shared database is not shared write authority.

Rules 1–3 express the horizontal direction. Rules 4–8 are concrete choices for
this candidate. Go's import-cycle check does not establish rules 7–8 or prevent a
new SQL statement from bypassing Booking.

## Repository root

```text
spaceweave/
├── cmd/spaceweave/main.go             thin Cobra execution entry point
├── internal/                         horizontal layers and composition below
├── pkg/client/                       generated Go API clients only
├── api/                              OpenAPI sources, generation settings and bundles
├── web/
│   ├── staff/                        React/TypeScript staff and mobile-web entrypoint
│   ├── admin/                        separate administrative entrypoint
│   ├── shared/                       UI primitives, generated clients, renderer bridge
│   └── floorplan-iso/                 retained renderer and its current fixtures
├── test/
│   ├── acceptance/                   existing tracing plus new booking journeys
│   ├── integration/                  real PostgreSQL/provider protocol contracts
│   ├── capacity/                     representative response/queue/contention workloads
│   └── recovery/                     database, assets, identity, routing and workers
├── deploy/
│   ├── kubernetes/                   one image, separate role manifests/identities
│   ├── telemetry/                    Collector pipelines and alert definitions
│   └── recovery/                     fencing and recovery exercises/runbooks
├── docs/                             scope, ADRs, research and operator guidance
├── Makefile                          generation, validation and existing gate targets
├── .golangci.yml                     blocking lint and import restrictions
├── .testcoverage.yml                 existing total/per-package coverage requirements
└── go.mod                            existing module path; GOWORK=off builds
```

`pkg/` currently contains guidance. Proposed public HTTP clients support callers
and acceptance harnesses; domain contracts and operational tooling stay internal.

## Full package tree

Every counted Go package below has a printed Go file. Directory-only nodes
organise packages; representative files do not enumerate every eventual file.

```text
internal/
├── app/
│   ├── app.go                        startup, drain and role selection
│   ├── runtime.go                    construct services and their approved siblings
│   ├── database.go                   open managed PostgreSQL; migration command wiring
│   ├── providers.go                  configured blob, Graph, Workday and delivery clients
│   ├── jobs.go                       bind durable job kinds to application methods
│   ├── http.go                       compose Chi handlers and portal assets
│   ├── telemetry.go                  SDK lifecycle and JSON logging wiring
│   └── recovery.go                   writer epoch and recovery-readiness startup checks
├── config/
│   └── config.go                     strict deployment YAML and precedence ← internal/config
├── buildinfo/
│   └── buildinfo.go                  stamped release identity ← internal/buildinfo
├── archtest/
│   ├── imports_test.go               layer and application-call allowlists
│   └── ownership_test.go             store facet/SQL/migration ownership checks
├── model/
│   ├── identity.go                   StaffID, source links, effective workforce facts
│   ├── access.go                     actor/owner, capability, scope, source-labelled grant
│   ├── workplace.go                  office/building/floor/space and baseline neighbourhood
│   ├── policy.go                     policy revision and pinned operational values
│   ├── intent.go                     attendance date, travel, requirements, collaboration
│   ├── allocation.go                 cycles, per-date groups, snapshots and proposals
│   ├── booking.go                    claims, linked parking, holds and booking versions
│   ├── deskuse.go                    allocated/taken/released states and personal check-in
│   ├── calendar.go                   desired/observed room changes and reconciliation state
│   ├── plans.go                      drawing package and immutable asset revision metadata
│   ├── visitor.go                    restricted visitor identity, hosted visit and reception
│   ├── event.go                      audience, registration, tickets and admissions
│   ├── reporting.go                  evidence kinds and projection freshness
│   ├── notification.go               message kinds, recipients and per-message preferences
│   ├── work.go                       durable task envelopes, outbox facts and retry identity
│   └── failure.go                    unavailable/stale/pending/forbidden/dependency results
├── support/
│   ├── clock/
│   │   └── clock.go                  injected wall clock and controlled test time
│   ├── officecalendar/
│   │   └── calendar.go               timezone, working-window and closure arithmetic
│   ├── plan/
│   │   ├── trace.go                  drawing/trace schema ← internal/plan
│   │   └── validate.go               geometric validation and conversion checks
│   ├── convert/
│   │   ├── convert.go                format dispatch ← internal/convert
│   │   └── formats.go                image/DXF/DWG/SVG content eligibility
│   ├── cad/
│   │   ├── cad.go                    common CAD representation ← internal/cad
│   │   └── draw.go                   drawing interpretation/render geometry
│   ├── dxf/
│   │   ├── dxf.go                    DXF reader ← internal/dxf
│   │   ├── entities.go               entity decoding and units
│   │   └── dxftest/
│   │       └── dxftest.go            synthetic drawing fixture builder
│   ├── dwg/
│   │   ├── dwg.go                    DWG reader ← internal/dwg
│   │   ├── entities.go               supported binary drawing entities
│   │   └── dwgtest/
│   │       └── dwg.go                binary drawing fixture builder
│   ├── svgdraw/
│   │   ├── svgdraw.go                SVG drawing interpretation ← internal/svgdraw
│   │   └── path.go                   vector paths and transforms
│   ├── vector/
│   │   ├── vector.go                 drawing representation ← internal/vector
│   │   └── svg.go                    SVG output
│   ├── raster/
│   │   ├── raster.go                 raster loading/analysis ← internal/raster
│   │   └── overlay.go                tracing/debug overlays
│   ├── floorgraph/
│   │   ├── graph.go                  wall graph ← internal/floorgraph
│   │   └── spaces.go                 rooms, openings and space topology
│   ├── autotrace/
│   │   ├── autotrace.go              deterministic tracing ← internal/autotrace
│   │   ├── cadfront/
│   │   │   ├── cadfront.go           structured drawing extraction
│   │   │   └── layers.go             role-aware layer interpretation
│   │   ├── rasterfront/
│   │   │   ├── rasterfront.go        image wall/opening extraction
│   │   │   └── walls.go              raster evidence processing
│   │   ├── profile/
│   │   │   ├── profile.go            profile values ← internal/autotrace/profile
│   │   │   └── words.go              pure built-in role/name matching
│   │   ├── score/
│   │   │   └── score.go              tracing evaluation helpers
│   │   └── synth/
│   │       └── synth.go              synthetic plan fixtures
│   └── testfault/
│       └── fault.go                  controlled failures used by contract suites
├── transport/
│   ├── http/
│   │   ├── server.go                 Chi routes and request limits ← internal/server
│   │   ├── middleware.go             request/trace IDs, recovery and security headers
│   │   ├── session.go                cookie/CSRF extraction; call Access
│   │   ├── gen/
│   │   │   ├── staff/
│   │   │   │   └── server.gen.go      generated staff transport contract
│   │   │   ├── admin/
│   │   │   │   └── server.gen.go      generated administrative transport contract
│   │   │   └── scim/
│   │   │       └── server.gen.go      generated provisioning wire types/server contract
│   │   ├── staff/
│   │   │   ├── booking.go            intents, direct requests, changes and own history
│   │   │   ├── checkin.go            booking/QR personal check-in and offer acceptance
│   │   │   └── events.go             registration, tickets and restricted visitor access
│   │   ├── admin/
│   │   │   ├── workplace.go          definitions and dated adjustment requests
│   │   │   ├── policy.go             versioned policy publication
│   │   │   ├── reception.go          host/visit arrival and departure
│   │   │   └── reporting.go          office-scoped reports and handover tasks
│   │   ├── plans/
│   │   │   ├── upload.go             bounded multipart validation ← server upload/files
│   │   │   ├── review.go             approve/reject/retry/retrace ← server review
│   │   │   └── profiles.go           layer-profile corrections ← server profiles
│   │   ├── scim/
│   │   │   ├── users.go              authenticated Users CRUD/PATCH/filter handling
│   │   │   ├── groups.go             mapped Groups and membership provisioning
│   │   │   └── discovery.go          service-provider/resource/schema discovery
│   │   ├── exchange/
│   │   │   └── notifications.go      validate callbacks; durably schedule reconciliation
│   │   ├── occupancy/
│   │   │   └── receiver.go           source-authenticated evidence command to Booking
│   │   ├── teams/
│   │   │   └── activities.go         authenticated personal app/conversation lifecycle
│   │   ├── health/
│   │   │   └── health.go             readiness, liveness and dependency status views
│   │   └── static/
│   │       └── portals.go            staff/admin builds and renderer assets
│   ├── cli/
│   │   ├── root.go                   Cobra commands ← internal/cli command half
│   │   ├── plans.go                  one-off conversion/tracing/review operations
│   │   └── operations.go             migrate, reconcile, retry and recovery inspection
│   ├── trigger/
│   │   ├── schedule.go               enqueue due cycles, delta rounds and retention
│   │   └── outage.go                 authenticated operational incident declarations
│   ├── inbox/
│   │   └── inbox.go                  optional local import watcher ← internal/inbox
│   └── jobs/
│       └── dispatch.go               decode/version job envelope; invoke registered handler
├── application/
│   ├── booking/
│   │   ├── booking.go                authoritative booking service and actor/owner checks
│   │   ├── intents.go                collect/amend attendance demand and travel exceptions
│   │   ├── cycles.go                 main-run cutoff, processing state and direct-open gate
│   │   ├── snapshot.go               consistent versioned allocation input query
│   │   ├── placement.go              revalidate and atomically commit one whole group
│   │   ├── direct.go                 spare capacity after run; waitlist precedence
│   │   ├── parking.go                bay/pool claims and required desk+parking linkage
│   │   ├── transfer.go               acquire replacements/release old claims atomically
│   │   ├── adjustment.go             dated movement of unallocated neighbourhood desks
│   │   ├── checkin.go                only assignee can mark live allocation taken
│   │   ├── evidence.go               validate/deduplicate operational evidence; commit facts
│   │   ├── release.go                cancellation/no-show/vacancy and linked claim policy
│   │   ├── waitlist.go               eligibility, ordering and offer creation
│   │   ├── offer.go                  versioned acceptance, expiry and next candidate
│   │   ├── rooms.go                  pending local holds and room acceptance transitions
│   │   ├── roomchange.go             revalidate externally observed room/time changes
│   │   ├── venue.go                  venue mutation inside Events' named transaction
│   │   ├── lifecycle.go              idempotent leaver personal-claim cleanup
│   │   ├── outage.go                 protected timeout evaluation/recovery grace
│   │   ├── record.go                 booking facts, audit and outbox inside transaction
│   │   ├── solver.go                 Booking-owned solver interface and planning orchestration
│   │   └── planning/                 private calculation; Booking owns demand/progress/claims
│   │       ├── run.go                    load snapshot; plan outside database transaction
│   │       ├── groups.go                 per-date grouping distinct from external teams
│   │       ├── packing.go                neighbourhood/floor placement and split candidates
│   │       ├── eligibility.go            mandatory features, calendars and required parking
│   │       ├── objective.go              demand, explicit collaboration, proximity, unmet demand
│   │       └── replan.go                 affected uncommitted groups after stale proposal
│   ├── workplace/
│   │   ├── workplace.go              office/building/floor/space definitions
│   │   ├── neighbourhood.go          baseline bundles within one building
│   │   ├── revision.go               prepare/publish immutable definition versions
│   │   ├── mapping.go                publish approved geometry bindings after Booking validation
│   │   └── retirement.go             scheduled retirement/reactivation; guarded deletion
│   ├── policy/
│   │   ├── policy.go                 resolve pinned versions and permitted office overrides
│   │   └── publish.go                validate effective dates and cross-field consistency
│   ├── identity/
│   │   ├── identity.go               People/Access/Delegation facades under one owner
│   │   ├── correlation.go            verified identifiers; ambiguous-match review
│   │   ├── scim.go                   provisioning lifecycle and mapped groups
│   │   ├── workforce.go              Workday source-owned effective facts
│   │   ├── eligibility.go            reduce independent disabling inputs
│   │   ├── reconcile.go              source progress and missing-record repair
│   │   ├── access.go                 current capability, office and global checks
│   │   ├── sessions.go               session and server-owned token lifecycle
│   │   ├── delegation.go             app-managed/source-labelled grant evaluation
│   │   ├── native_delegation.go      fresh native evidence through provider adapter
│   │   ├── visitor.go                restricted verified-email visitor access
│   │   └── decide.go                 current own eligibility/session denial
│   ├── exchange/
│   │   ├── exchange.go               send committed desired room operations after commit
│   │   ├── observe.go                retain provider observations and local correspondence
│   │   ├── reconcile.go              three-way compatible merge; incompatible review tasks
│   │   ├── acceptance.go             associate responses with current request/hold revision
│   │   ├── delta.go                  calendar/window cursor rounds and resynchronisation
│   │   └── subscriptions.go          renew subscriptions and process lifecycle recovery
│   ├── plans/
│   │   ├── ingest.go                 drawing package deduplication and revision identity
│   │   ├── process.go                conversion/tracing workflow ← internal/worker
│   │   ├── review.go                 approve/reject/retry/retrace application decisions
│   │   ├── profiles.go               version corrections and enqueue affected retraces
│   │   └── publish.go                recovery-verified immutable asset publication
│   ├── visitors/
│   │   ├── visitors.go               host, visit window and restricted visitor details
│   │   ├── reception.go              arrival/departure distinct from event admission
│   │   └── ticketvisit.go            retry-safe visit association within ticket transaction
│   ├── events/
│   │   ├── events.go                 event/audience/access and organiser management
│   │   ├── venue.go                  coordinate venue and issued-ticket capacity atomically
│   │   ├── registration.go           named staff/visitor ticket issue and duplicate requests
│   │   ├── admission.go              one successful entry scan per usable ticket
│   │   └── handover.go               assisted leaver ownership/venue recovery work
│   ├── reporting/
│   │   ├── projection.go             committed booking/check-in facts and source freshness
│   │   ├── reports.go                own history and scoped individual/aggregate queries
│   │   └── retention.go              reporting projection/copy retention and purge progress
│   ├── notifications/
│   │   ├── record.go                 intended recipients/message kind from committed outcome
│   │   ├── preferences.go            message-specific delivery choices/Teams opt-in evidence
│   │   ├── deliver.go                delivery attempts and preference check at dispatch
│   │   ├── templates.go              email/in-app/private booking-raised content
│   │   └── teams.go                  eligibility and private conversation prerequisites
│   ├── operations/
│   │   ├── workplace.go              coordinate asset approval, mapping and Booking validation
│   │   ├── incidents.go              persist affected outage windows and recovery grace
│   │   └── recovery.go               lag/readiness assessment and explicit failover evidence
│   └── jobs/
│       ├── jobs.go                   enqueue/claim/retry primitives and registered handlers
│       ├── outbox.go                 append committed facts; dispatch with durable progress
│       └── lease.go                  bounded lease renewal, lost-lease rejection and drain
└── persistence/
    ├── store/
    │   ├── store.go                  aggregate factory and narrow read/write facets
    │   ├── tx.go                     explicit transaction handle and bounded Tx callback
    │   ├── booking.go                Booking claims/evidence; allocation snapshot contracts
    │   ├── identity_people.go        Identity staff/source/effective-state facets
    │   ├── identity_access.go        Identity sessions, grants and delegation facets
    │   ├── workplace.go              versioned definitions and calendars
    │   ├── policy.go                 policy publication and pinned-version lookup
    │   ├── exchange.go               desired/observed mappings, subscriptions and cursors
    │   ├── plans.go                  package/revision/profile/recovery-copy metadata
    │   ├── events.go                 venue coordination state, tickets and admissions
    │   ├── visitors.go               visit links and reception/retention state
    │   ├── reporting.go              derived projections and scoped read queries
    │   ├── notifications.go          preferences, inbox and delivery identities
    │   ├── work.go                   jobs/outbox/leases/incidents/idempotency records
    │   ├── postgres/
    │   │   ├── postgres.go           pool lifecycle and facet implementation constructors
    │   │   ├── transaction.go        commit/rollback; shared handle validation and retries
    │   │   ├── booking.go            booking claims and transitions; no business bypass API
    │   │   ├── capacity.go           stable lock rows and overlap/capacity constraints
    │   │   ├── allocation.go         snapshot/cycle/group/proposal rows
    │   │   ├── evidence.go           Booking-owned operational occupancy records
    │   │   ├── identity_people.go    Identity source observations and effective facts
    │   │   ├── identity_access.go    Identity session/grant/token persistence
    │   │   ├── workplace.go          definition revisions and immutable references
    │   │   ├── policy.go             validated published revisions
    │   │   ├── exchange.go           durable calendar observations and reconciliation state
    │   │   ├── plans.go              asset revisions and publication state
    │   │   ├── events.go             event locks, ticket counters and unique admissions
    │   │   ├── visitors.go           visit association, arrival/departure and purge state
    │   │   ├── reporting.go          derived projections and paged scope-filtered reports
    │   │   ├── notifications.go      deliveries/preferences and attempt uniqueness
    │   │   ├── jobs.go               due claims, leases, backoff and failed-job inspection
    │   │   ├── outbox.go             same-transaction append and consumer checkpoints
    │   │   ├── migrate.go            one ordered migration runner and ownership manifest
    │   │   └── migrations/
    │   │       ├── owners.yaml       table/file owner and permitted cross-owner references
    │   │       ├── 0001_identity_people.sql Identity people/source constraints
    │   │       ├── 0002_identity_access.sql Identity sessions/capabilities/grants
    │   │       ├── 0003_workplace.sql Workplace definitions, calendars and active mappings
    │   │       ├── 0004_policy.sql    Policy revisions and publications
    │   │       ├── 0005_booking.sql   Booking inventory/intents/cycles/claims/evidence
    │   │       ├── 0006_exchange.sql  Exchange mappings/observations/cursors
    │   │       ├── 0007_plans.sql     Plans revisions and recovery metadata
    │   │       ├── 0008_visitors.sql  Visitors/visits and reception state
    │   │       ├── 0009_events.sql    Events/tickets/admissions
    │   │       ├── 0010_reporting.sql Reporting projections and purge progress
    │   │       ├── 0011_messages.sql  Notifications preferences/inbox/attempts
    │   │       └── 0012_work.sql      durable jobs/outbox/incidents and retry records
    │   └── storetest/
    │       ├── booking.go            concurrent claim/transfer/check-in contract suite
    │       └── workflows.go          event/visit/identity/shared-transaction contract suite
    ├── blob/
    │   ├── blob.go                   private immutable object interface; opaque tokens
    │   ├── azure/
    │   │   └── azure.go              Azure Blob implementation
    │   ├── gcs/
    │   │   └── gcs.go                Google Cloud Storage implementation
    │   ├── recovery/
    │   │   └── copy.go               write/verify recovery copy before durable acknowledgement
    │   └── blobtest/
    │       └── contract.go           shared provider immutability/read/retry contract
    ├── calendar/
    │   ├── calendar.go               provider-neutral calendar/delegation client interface
    │   └── graph/
    │       ├── events.go             Graph event mapping and create/edit/cancel transport
    │       └── sync.go               subscriptions/delta/permission reads and throttling
    ├── workforce/
    │   ├── workforce.go              source snapshot/incremental interface
    │   └── workday/
    │       └── workday.go            configured tenant/API mapping and progress transport
    ├── identity/
    │   ├── identity.go               provider sign-in/key/token exchange interface
    │   └── entra/
    │       └── entra.go              validate configured issuer/audience and provider exchange
    ├── delivery/
    │   ├── delivery.go               message transport interface and attempt result
    │   ├── email/
    │   │   └── email.go              organisational email provider adapter
    │   └── teams/
    │       └── teams.go              private proactive message transport and conversation IDs
    ├── tracing/
    │   ├── tracing.go                optional tracing-agent interface
    │   ├── claude/
    │   │   └── agent.go              external-agent loop ← internal/trace
    │   └── fakeclaude/
    │       └── fakeclaude.go          acceptance provider ← internal/fakeclaude
    ├── scratch/
    │   └── files.go                  bounded local processing workspace; never durable truth
    ├── profilefiles/
    │   └── library.go                optional local profile I/O ← profile/library.go
    └── telemetry/
        └── telemetry.go              OpenTelemetry exporters and JSON log configuration
```

PostgreSQL implementation is deliberately centralised: ownership spans roots
and requires file/SQL discipline within a large persistence package.

## API surfaces and consumers

The [surface catalogue](../api-surfaces/README.md) defines the UI,
MCP servers, SDKs, scripts, test clients and Terraform consume supported public
REST. Workers use private application contracts. User-facing REST is Internet
accessible under Conditional Access and application authority; invited internal-
event visitors retain their scoped verified-email flow.

| Surface/consumer | Kind and contract | Placement in this candidate |
| --- | --- | --- |
| UI, MCP, SDK, script, public contract tests and Terraform | Shared supported public REST business operations with capability/field views. | `transport/http` maps public wire schemas to the owning `application` facades. Staff/admin are separate experiences over those contracts, not private business APIs. |
| Human and application caller profiles | Session/CSRF or appropriate API token; actual actor, client and owner/grant checks. | Transport validates credentials; Identity/command owners enforce current capabilities, office and delegation scope. Conditional Access differs for humans and workloads. |
| Internal-event visitor, such as a guest speaker | Publicly reachable invitation/verification and limited visit/event/ticket REST. | Visitor transport mapping and scoped session; no full staff model or general public event catalogue follows. |
| Entra, Graph, Teams and optional sensor clients | Provider/source-restricted native protocols or selected ingestion REST. | Dedicated HTTP adapters preserve protocol authentication/media/errors and dispatch to the relevant application owner. |
| Platform workers, scheduling and runtime tools | Private Go and versioned job/fact interfaces. | `transport/{jobs,trigger}` invokes application facades directly. Worker commit/release/reconciliation internals are not public REST operations. |
| Platform probes/telemetry and implementation ports | Private operational contracts and outbound storage/provider interfaces. | Restricted listeners/export plus `persistence` interfaces; no public store facet, raw SQL or transaction handle. |

Wire publication and horizontal model sharing need explicit discipline. One
Booking model can support many clients, but generated public DTOs select fields
and operations deliberately; changing `model` must not leak fields or bypass
compatibility. The same command checks apply to every HTTP client. A script,
SDK or MCP tool name provides no special authority.

Generated Go/TypeScript clients target the shared public contract; a possible
`pkg/client` location is now a supported client deliverable rather than evidence
of a private UI-only surface. Human/application identities and configuration-only
Terraform scope are agreed; first-party MCP/provider implementations follow the
core platform and SDKs. Public version compatibility is independent of UI
releases. RBAC is configurable; MCP uses user context, machine-identity scripts
use their own channel, UI edits may create Terraform drift and initial version
retirement notice is 180 days. Identity holds role policy;
global administration, SCIM-backed mappings, grant-only scoped composition and
self-permission discovery are agreed. The [agreed request contract](../api-surfaces/request-behaviour.md)
places atomic command receipts in the owning application/persistence transaction,
with previous-version configuration edits, standard/expensive actor budgets and
public operation status. Wire and retention details remain open;
worker execution is bounded separately.
Private module/store contracts continue to evolve separately from the public
wire version. Counts above still describe the printed candidate tree only.

## Deep dives

### Desk demand, group planning and commitment

`booking/intents.go` records chosen dates, usual-office defaults, travel,
requirements/preferences and explicit collaboration priority. Imported teams,
including multiple memberships, inform per-date groups in `booking/planning/groups.go`;
group IDs never replace source team identities.

Cycles pin office calendars, cohorts and policy versions. Seven-day cycles and
fourteen-day cutoffs are configurable [defaults](../../policy-defaults.md).
Allocation snapshots include dated neighbourhoods, claims, required parking and
source-effective eligibility; computation holds no long transaction.

`packing.go` evaluates placements satisfying mandatory space features before
preferences. It can split teams across floors to fit more demand; neighbourhoods
remain within one building even when they span floors. `objective.go` compares
accommodated demand first, then explicit collaboration, general team proximity
and previously unfulfilled demand. It does not enforce anchor-day attendance.

Booking commits a whole group plus both-required parking in one bounded
transaction, revalidating affected snapshot dependencies, current eligibility,
policy and claim constraints. Replan affected uncommitted groups; preserve
confirmations. Bound group size without breaking atomicity.

`direct.go` opens ordinary direct booking only after the date's main run.
Earlier requests join intents. Eligible waitlist demand precedes fresh requests;
same-day offers hold capacity until acceptance/expiry through Booking's current
state transition. A stale offer cannot displace a later confirmation.

### One capacity writer, linked parking and transactions

Only composition receives aggregate `Store`; services receive narrow facets.
Exchange and Events call Booking; its planner is private for capacity. Booking's transaction
facet names supported transitions, avoiding generic `Save(any)` access.

Proposed pseudocode illustrates placement, not a promised Go API:

```go
// internal/application/booking/placement.go
func (s *Service) CommitGroup(ctx context.Context, req GroupPlacement) error {
    return s.DB.Tx(ctx, func(tx store.Tx) error {
        claims := s.Bookings.WithTx(tx)
        // Stable lock keys include people, resources and parking-pool windows.
        if err := claims.LockPlacement(ctx, req.LockKeys()); err != nil {
            return err
        }
        if err := s.revalidate(ctx, tx, req); err != nil {
            return err
        }
        if err := claims.CommitWholeGroup(ctx, req); err != nil {
            return err
        }
        return s.Work.WithTx(tx).Append(ctx, req.CommittedFacts())
    })
}
```

The transaction covers claims, required parking, waitlist state, audit/retry
results and follow-up facts. Planning/provider calls stay outside. Transfers
lock old/replacement keys in stable order and commit acquisition/release together;
failure retains the old allocation.

Propose half-open time ranges and database overlap constraints for individual
resources and one overlapping desk per person. PostgreSQL supports exclusion
constraints over ranges; that does not solve pooled capacity greater than one.
Pool usage needs coordinated checks under stable pool/window locks, shared by
all claim/release paths. Locking existing claims alone cannot protect an absent
future claim. These are proposed locking choices built on documented
[range constraints](https://www.postgresql.org/docs/current/rangetypes.html#RANGETYPES-CONSTRAINT)
and [row-lock semantics](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS).

The SQL design still needs a measured contention and deadlock strategy. Retriable
transaction failures rerun the entire decision; unavailable capacity is a normal
business result, not an instruction to retry forever. `SKIP LOCKED` belongs to
job claiming, not the inventory query deciding who can book.
[Transaction retry rules](https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html),
[queue-locking caveat](https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE).

### Logical ownership in horizontal persistence

| Logical owner | Application decisions | Persistence files/tables it alone mutates |
| --- | --- | --- |
| Booking | claims, dated inventory, intents/cycles, waitlists, check-in/release and validated operational occupancy evidence | booking/capacity/allocation/evidence SQL; Booking-owned claim/inventory/evidence tables |
| Workplace | baseline definitions, calendars, lifecycle schedules and active geometry mappings | workplace SQL and definition/mapping tables |
| Identity | canonical IDs/source facts, SCIM, sessions, capabilities and delegation | Identity-owned people/access facets and migration files under the shared SQL layer |
| Exchange | correspondence, provider observations, progress | exchange SQL and mapping/cursor/subscription tables |
| Plans | drawing packages, processing/review/publication | plans SQL and asset/revision/profile tables |
| Visitors / Events | visits/reception / event/tickets/admission | visitors SQL / events SQL, respectively |
| Reporting / Notifications | derived projections/report retention / preferences/delivery | reporting SQL / notifications SQL, respectively |
| Policy / Jobs | version publications / durable execution mechanics | policy SQL / jobs/outbox/incidents, respectively |

`owners.yaml` lists table/query/migration owners and approved references.
Architecture checks inspect imports, facet selectors and queries; schema review
approves changes. Only `store/postgres` opens business-query SQL connections.

One migration sequence provides deployment order; every migration has a logical
owner. Cross-owner foreign keys require coordinated upgrade/deletion review.
Repairs also pass through Booking. Shared credentials/package access provide no
adversarial isolation; checks target accidental bypass.

Two named shared transactions are permitted: event venue/capacity changes and
visitor ticket issuance. Events owns their transaction boundary and passes an
explicit store transaction handle to Booking/Visitors operations. Those services
retain their checks and repository access; Events never receives their writers.
A nested call joins that handle, never starts an independent transaction that
could commit half the workflow. This is ordinary layered service coordination,
with an explicit cross-owner consistency contract.

### Check-in, no-show, occupancy and incident protection

`checkin.go` requires the authenticated actor to own a live allocation: allocated
→ taken. A PA cannot check in for the owner. QR identifies the desk, not its user.

Booking validates and stores operational evidence, distinguishing anonymous
occupancy, identified desk evidence and office presence. Reporting projects
committed facts under separately configured reporting retention. Anonymous
occupancy cannot establish attendance or prevent no-show release. After personal
check-in, fresh sustained vacancy may release capacity; missing signals mean unknown.

Expiry workers read current booking version, policy and protected outage
intervals under the same transition locks as check-in. An operator-declared
incident records affected actions/offices/times durably; a worker-local health
flag cannot decide everybody's expiry policy. Confirmed app outages preventing
check-in/offer acceptance pause those deadlines, followed by configured recovery
grace. They do not extend actual space use past the booking period. An old
allocation is never resurrected over a committed reassignment.

### Room synchronisation and delegation

Booking owns local room holds and confirmation; Exchange owns provider mapping,
desired/observed state and progress. A PA request records the owner as organiser
and PA as actor. Identity checks current app grants or native-derived evidence
within the configurable fifteen-minute freshness limit before a new delegated
action. Application mailbox access is not evidence of that PA's personal rights.

App grants remain local. Native revocation removes only that source grant.
Failed refresh blocks actions needing stale native evidence, preserving other
valid grants. Committed work retains actor/authority evidence; a worker's service
identity never grants a human new authority.

A local request first commits an expiring hold and desired operation. A worker
creates the organiser's provider event after commit. A successful create is not
room acceptance. The acceptance observation must correspond to the current
request/hold revision; Booking confirms only after current capacity validation.
Late acceptance after expiry creates reconciliation work rather than silently
reclaiming capacity from another booking.

The application retains its actor audit because Microsoft's delegated-calendar
example establishes the owner/organiser but does not supply a durable PA audit
for SpaceWeave. Graph exposes event version information and a creation
`transactionId` for redundant-create prevention. Neither supplies exactly-once
synchronisation. [Delegated calendar creation](https://learn.microsoft.com/en-us/graph/outlook-create-event-in-shared-delegated-calendar),
[event resource](https://learn.microsoft.com/en-us/graph/api/resources/event?view=graph-rest-1.0).

`reconcile.go` compares last agreed state, current desired local state and latest
provider observation. Compatible field changes merge; room/time changes pass
Booking capacity checks; incompatible intentions open a visible review task.
Keep a conflicting provider observation rather than pretending the local model
can overwrite provider truth. Outbound retry checks the latest desired revision
before sending and checks again when applying its result.

Do not infer Graph compare-and-swap from `changeKey`: the update reference does
not establish that event-specific conditional-write contract. Do not promise an
atomic provider room swap retaining the old room until replacement acceptance.
Replacement, organiser handover and compensating cancellation need validated
workflows. Organiser cancellation and deleting a room calendar copy also remain
distinct operations. [Event update contract](https://learn.microsoft.com/en-us/graph/api/event-update?view=graph-rest-1.0).

Notifications accelerate discovery; durable reconciliation runs on the agreed
five-minute healthy cadence and measures actual freshness. Each calendar/window
has its own cursor; complete a page round before advancing the durable delta
checkpoint. Graph's calendar-view delta is bounded by calendar and date range,
and tokens are opaque. Rollover/reinitialisation is an explicit workflow, not
an assumption that one token represents all history.
[Calendar-view delta](https://learn.microsoft.com/en-us/graph/delta-query-events).

### Identity, SSO and actual SCIM provisioning

Identity keeps canonical StaffID, verified source links and separate source
inputs. Entra owns sign-in/account security and supplies authenticated SCIM Users
and mapped Groups provisioning. Workday owns workforce lifecycle, structure and
usual office. SpaceWeave owns preferences and explicit dated travel. Identity's
field-authority table and effective-date logic prevent one import overwriting
another source's responsibility.

SCIM implements create, read/filter, update/PATCH, deactivate/reactivate, delete,
mapped memberships, discovery and errors, with credentials separate from SSO.
Deletion removes the provisioning representation; retained booking history uses
stable internal identity. This follows
[SCIM authentication/deletion semantics](https://www.rfc-editor.org/rfc/rfc7644.html)
and the [Entra provisioning profile](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups).

Correlate an organisation-approved unique identifier with verified source IDs;
email changes do not merge people. Ambiguous links wait for review. Workday
imports distinguish observation time and effective time; a partial response or
unavailable feed cannot establish a leaver. Microsoft's integration describes
as-of data and effective/update processing, supporting the need for explicit
mapping and reconciliation rather than an assumed instantaneous complete feed.
[Workday integration reference](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/workday-integration-reference).

An observed effective disable/leaver immediately denies existing/new sessions;
another source cannot clear its authoritative denial. Durable cleanup releases
future personal desk/parking through Booking and raises room/event handover.
Effective office/team changes update unconfirmed defaults, preserving travel and
confirmations. Cleanup lag never postpones denial.

### Events, visits and venue consistency

Events manages audience and registration access separately: a staff-only or
visitor-admitting audience can have invitation-only or permitted public
registration. Tickets are free, named and capacity limited. Booking secures the
venue; Events owns the ticket limit, which cannot exceed that venue's capacity.

A venue edit and ticket issue lock the same event coordination row. Events opens
the shared transaction, checks issued tickets against prospective limit/capacity,
asks Booking to acquire/release venue claims, updates event state and appends
facts. A move to capacity below issued tickets fails rather than revoking
entitlements incidentally. Generic Booking venue edits route through this
workflow. Confirmed venue loss retains tickets pending recovery and pauses
admission and new issuance; the organiser explicitly secures a suitable
replacement or cancels, and confirmed changes are communicated to attendees.

Events and Visitors commit a registration-created visit and its ticket together;
failed issuance leaves no new visit. A reused existing valid visit remains intact.
Publish only a committed linked ticket. Retry identities prevent duplicates;
conditional admission checks the venue-recovery pause and allows one successful scan. Reception remains separate;
verified-email access reveals only relevant visit/event actions.

### Plans: evolve the current pipeline without losing it

The current [server](../../../internal/server/server.go),
[catalogue](../../../internal/catalog/catalog.go) and
[worker](../../../internal/worker/worker.go) implement upload, queue claim,
conversion, tracing and approval using local files. Catalogue lock files and
startup recovery are useful local-tool behaviour; they are not distributed
Kubernetes leases or the production booking store.

Move catalogue decisions into model/application Plans and production claims into
PostgreSQL/jobs, preserving hashes, attachments, profiles, review states, progress
and retry/retrace. Inbox, HTTP and CLI share one ingest operation.

Conversion and deterministic geometry remain support libraries. The external
Claude loop becomes a persistence adapter; provider-free job/trace values move
out of its current package so the deterministic tracer does not import the
provider adapter. Profile values/name matching remain support; profile file I/O
moves to persistence/profilefiles. This is a real dependency refactor, not only
a directory rename.

Tracing stages bytes in bounded scratch space, checks lease/revision on progress,
writes immutable objects and verifies recovery copies before durable/publication
acknowledgement. PostgreSQL holds IDs, hashes, status and current pointers.
Plans approval advances asset pointers without overwriting referenced history.
Workplace publishes the reviewed geometry-to-stable-space mapping through
`application/workplace/mapping.go`, after Booking validates dated-inventory and
commitment impact. `application/operations/workplace.go` can coordinate approval
and activation in one administrator workflow, returning each actual outcome;
detection or asset approval alone creates no authoritative bookable desks.

Azure uses ETags for conditional operations; GCS uses generation preconditions.
The interface carries opaque provider tokens separately from content hashes.
This supplies provider-safe object operations, not a PostgreSQL/object-store
transaction; staged/orphaned work requires reconciliation.
[Azure concurrency](https://learn.microsoft.com/en-us/azure/storage/blobs/concurrency-manage),
[GCS preconditions](https://docs.cloud.google.com/storage/docs/request-preconditions).

### Notifications, reporting and privacy

Booking records committed facts and intended owner/actor recipients in its
transaction. A registered outbox dispatcher invokes Notifications and Reporting
after commit, with independent consumer progress. In-app rows, email attempts
and reporting projections can lag without changing a booking outcome. Delivery
identities cover message kind + originating fact + recipient + channel; the
system handles duplicate dispatch without claiming exactly-once remote effects.

Teams is one narrow delivery option: a private message that a booking was
raised, with that individual's opt-in for that message. Installation is a
technical prerequisite, not consent. Recheck current message-specific preference
and recipient/conversation binding at dispatch. No shared-channel destination
exists in this candidate, and this opt-in cannot authorise other Teams messages.
Microsoft documents personal installation and conversation requirements for
proactive delivery; blocked/uninstalled responses become visible delivery state.
[Teams proactive messages](https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/conversations/send-proactive-messages).

Reporting consumes committed facts and evidence, exposing bookings, personal
check-ins and occupancy as separate measures. Read queries apply office scope
and capability in the backend; a React route is not authorisation. Projection
freshness is returned with results, and no projection can grant capacity.

Visitor, occupancy and attendance retention are independent organisation settings.
Purge identifiable projections, messages, exports and failed-job data as well as
owning rows. Prefer minimal retry references to repeated names/email. Retain only
policy-permitted identifiers needed for safe workflows; metrics exclude person
IDs, ticket secrets and unbounded labels.

### Composition, Kubernetes roles, telemetry and recovery

App constructs concrete PostgreSQL/cloud/provider clients and injects facets and
approved sibling services. It registers job handlers without making the generic
Jobs service import every business service. For example, Booking appends a room
sync task; composition binds that kind to Exchange. Exchange calls Booking to
apply observations, so Booking never imports Exchange and forms no cycle.

One versioned image runs independently scaled HTTP, allocation, Exchange,
tracing and maintenance roles. HTTP serves portals/SCIM/callbacks and short
commands; allocation does CPU planning; Exchange owns provider reconciliation;
tracing isolates expensive conversions; maintenance handles expiry, outbox,
retention, delivery and repair queues. Separate role identities restrict database,
blob and provider privileges; sharing an image does not require sharing tokens.
Use workload federation where supported and organisational secret management for
remaining credentials.

Jobs use leases, bounded backoff and visible failures. Commits verify lease/writer
epoch, revision and expected state. External calls may finish after lease loss:
reconcile correspondence before retry. Managed PostgreSQL owns durability;
pod files are scratch. Controlled migrations and compatible schema/job envelopes
support rolling upgrades until old workers drain.

Telemetry records HTTP initial-response latency, booking conflicts/lock waits,
allocation completion, oldest due job, Exchange actual freshness, notification
attempts, protected timeout incidents and recovery-copy lag. Traces connect
request, committed task and worker attempt without recording sensitive payloads.
OpenTelemetry exports traces/metrics and structured JSON logs through the
organisation's Collector pipeline; the Collector receives/processes/exports
telemetry, not booking state.
[Collector purpose](https://opentelemetry.io/docs/collector/).

Measure 99.95% monthly core availability and one-second p95 initial responses
separately from allocation/acceptance. Readiness distinguishes core write failure
from provider degradation. Layout/replicas prove no SLO; office-skew contention
requires measurement.

Use one active write region, multi-zone HA and a warm recovery region. Recovery
fences the previous database and worker/external writers before resuming work;
PostgreSQL explicitly requires preventing the former primary from continuing as
a competing writer. [PostgreSQL failover](https://www.postgresql.org/docs/current/warm-standby-failover.html).

The five-minute RPO/fifteen-minute RTO targets cover database, acknowledged assets,
identity, routing and workers. Verify recovery copies before asset durability or
publication acknowledgement. When lag exceeds five minutes and primary remains
healthy, continue serving with a critical alert; promotion outside the loss
budget requires explicit operator choice. Exercise regional recovery, ambiguous
external writes, stale leases, restored cursors and check-in/offer protection.
A database promotion alone is not proof of whole-application recovery.

## Generated code and API specifications

```text
api/
├── src/staff/                       availability, intents, bookings, check-in and tickets
├── src/admin/                       definitions, policies, reception, reports and operations
├── src/scim/                        SCIM resources/filter/error/discovery transport schemas
├── staff.openapi.yaml               bundled staff REST contract
├── admin.openapi.yaml               bundled administrative REST contract
├── scim.openapi.yaml                provisioning contract with SCIM semantics
└── generation/                      pinned generation settings and output paths
internal/transport/http/gen/staff/   generated Go strict server/types
internal/transport/http/gen/admin/   generated Go strict server/types
internal/transport/http/gen/scim/    generated provisioning transport/types
pkg/client/staff/                   generated Go client
pkg/client/admin/                   generated Go client
web/shared/api/generated/           generated TypeScript transport clients/types
web/shared/renderer/                bridge around retained floorplan-iso code
web/staff/src/                      staff/mobile-web React composition and journeys
web/admin/src/                      administrative React composition and journeys
```

Handwritten handlers translate generated wire values into application arguments
and map explicit results back to transport status/envelopes. Model/Application
imports cannot reference generated types, Graph events or SCIM wire resources.
SCIM generation handles shapes, not provisioning semantics or authentication.
Separate server contracts avoid a single giant generated handler embedding every
portal operation.

Go handles authentication redirects/token exchange and retains provider tokens.
Secure HttpOnly session cookies and server-side CSRF/access checks protect
mutations; the browser receives a session, not a Graph token. Staff/admin builds
share UI and renderer seams while the backend checks actual capabilities and
scope on every operation. The current renderer remains incrementally usable;
React adoption does not require replacing its geometry/render algorithms.

## Tests and enforcement

No executable tests are added/run. Implementation retains [engineering gates](../../../CLAUDE.md):
95% merged total/90% per-package coverage, blocking lint and real-dependency
acceptance. Required validation includes:

| Location | Meaningful evidence |
| --- | --- |
| model/support package tests | deterministic eligibility/geometry/calendar cases and existing renderer fixtures |
| application/booking tests | policy/state results with controlled time; no fake-only proof of atomicity |
| store/storetest against PostgreSQL | competing claims, pooled parking, transfer rollback, check-in/release, event-ticket races |
| test/integration | authenticated SCIM, source-effective changes, provider mappings and supported retry behaviour |
| test/acceptance | HTTP/CLI journeys with real PostgreSQL, blob contract fixtures and retained plan validation |
| authorised provider environment | PA/owner semantics, room acceptance, stale changes, subscription recovery and private Teams delivery |
| test/capacity | site-skew contention, p95 initial response, group completion and backlog recovery |
| test/recovery | whole-region fencing, database/assets/routing restore and outage timeout protection |
| archtest | import direction, approved sibling edges, store-facet access and migration/query ownership |

Import/facet/SQL checks enforce placement, not every invariant. Real concurrent
transactions must prove locking, idempotency and shared-transaction rollback.
Thin transport/provider packages retain coverage obligations: contract-test
mapping, authority and failures; exclude only explicit generated outputs.

### Workplace retirement, reactivation and deletion

`internal/application/workplace/retirement.go` handles explicit retirement and dependency-checked removal
through the existing owning interfaces. Retirement is immediate or scheduled at an
office-local date/time. Publishing the schedule immediately excludes new bookings
for use on/after that point, including descendant spaces; confirmed commitments
remain readable and manageable. Booking compares requested use with the published
lifecycle revision at claim commit. A stale snapshot cannot bypass the schedule.
Authorised reactivation revalidates definitions, calendars and dated inventory,
preserving the same identity, commitments and history. Deletion reports blocking bookings,
children or other dependencies and never silently retires, cancels or cascades.
Retired resources remain present for authorised read/refresh/import, including
Terraform. The [workplace lifecycle contract](../api-surfaces/workplace-lifecycle.md)
applies independently of this candidate's technical organisation.

### Evidence custody and operational plan activation

Booking owns validated operational occupancy evidence used for desk-release
commands. Source adapters authenticate and translate observations; Booking checks
scope, provenance, granularity and freshness, deduplicates and commits evidence
facts. Attendance/Reporting consumes committed facts into projections with
separately configured reporting retention. A report or delayed projection never
authorises release. Personal check-in remains the only transition to taken;
anonymous occupancy cannot prevent a no-show returning to the pool.

Plans approval identifies an immutable, durable asset. Workplace owns publication
of geometry-to-stable-space mappings; Booking validates dated inventory and
confirmed-claim impact before operational activation. A combined administrator
workflow may complete both steps, but reports approval separately when activation
cannot succeed. The [mapping publication contract](../api-surfaces/plan-publication.md)
and [operational evidence decision](../../adr/0029-keep-operational-occupancy-evidence-in-booking.md)
apply in this candidate's directory style. Version guards, atomic visibility and
retry outcomes need transaction-level validation; this owner agreement does not
choose a repository architecture. Confirmed outage-record custody is a separate
[shared ownership question](README.md#remaining-shared-design-questions); existing
incident placement below remains a proposal until that decision is resolved.

## Walkthroughs

### 1. Commit a group while inventory changes

1. Trigger enqueues a due office cycle; Booking's allocation worker loads its versioned
   snapshot, source-effective teams/eligibility and pinned policy/calendar.
2. Private `booking/planning/groups.go` and `packing.go` form/date-place groups through
   Booking's consumed solver interface outside a long transaction;
   required parking and mandatory features constrain candidates.
3. A direct request or cancellation reaches Booking through the same service.
   Its transaction changes capacity/version and commits associated outbox facts.
4. `booking/placement.go` locks stable keys for the proposed group and revalidates
   affected snapshot dependencies, current claims, waitlist precedence and people.
5. If required parking is unavailable, the whole group transaction rolls back;
   no desk in that proposed group is partly confirmed.
6. Booking replans affected uncommitted groups. Successful groups atomically
   create claims and work records; confirmed earlier groups remain protected.

### 2. Race check-in, release and waitlist promotion

1. Staff POST/QR check-in identifies the authenticated actor and target booking;
   Identity and Booking require the assigned owner, never a delegate substitute.
2. Maintenance claims a due no-show job. Both paths lock/check the same current
   booking transition and protected incident interval.
3. If valid check-in commits first, the later no-show job observes taken state
   and does nothing. If release commits first, stale check-in cannot restore it.
4. The source receiver calls Booking to validate/deduplicate/store operational
   evidence and commit facts. Reporting projects those facts; anonymous occupancy
   cannot stop the no-show capacity returning to the pool.
5. Booking promotes eligible waitlist demand and holds capacity for an offer.
   Accept/expiry compete on current offer/claim state through Booking.
6. A confirmed preventing app outage pauses affected deadlines and adds configured
   recovery grace. Existing reassignments remain valid; sensor silence alone
   neither declares an app outage nor supplies post-check-in vacancy evidence.

### 3. PA room request, late updates and revoked authority

1. Staff transport passes PA actor + owner organiser. Identity verifies app grant
   or sufficiently fresh native-derived grant, preserving the authority source.
2. Booking commits pending hold, owner/actor audit and desired room-sync work.
   Exchange calls Graph after commit and records correspondence/results.
3. Delayed acceptance refers to an older request revision. Exchange stores it;
   Booking cannot confirm it over a newer/expired hold or another confirmed claim.
4. An Outlook edit is read through notification-driven/delta reconciliation.
   Exchange compares agreed/local/observed fields; compatible changes merge and
   room/time changes are revalidated through Booking.
5. Incompatible changes become visible review work without discarding the
   provider observation or claiming a generic atomic room-swap facility.
6. Subsequent native-grant revocation removes that grant. New actions refresh
   stale evidence or use an independently valid app grant. Existing booking
   ownership and PA audit survive; app-managed authority does not change Outlook
   permissions or allow PA check-in.

### 4. Smaller venue, competing ticket and visitor admission

1. Events starts a venue-edit transaction and locks the event coordination row;
   a competing registration waits on the same row.
2. Events compares issued tickets and requested ticket limit to replacement
   capacity, then calls Booking's transaction-aware venue operation.
3. Too-small capacity rejects the edit with all old venue/ticket state intact.
   An accepted edit atomically secures/releases venue claims and updates limits.
4. Registration rechecks current audience/access/capacity. For a visitor,
   Visitors ensures the visit link inside the same transaction before ticket
   publication; retries reuse the same outcome.
5. Ticket delivery happens after commit. A unique admission transition allows
   one successful scan; reception arrival remains a separate visit operation.
6. Confirmed venue loss is recorded, retains tickets pending recovery and pauses
   admission/new issuance. The organiser explicitly replaces the venue or cancels;
   confirmed changes are communicated to attendees.

### 5. SCIM disable plus effective workforce change during outage

1. Authenticated SCIM PATCH reaches Identity's provisioning interface.
   Identity updates source status, effective eligibility and session/access
   invalidation in one owned transaction.
2. Identity's Access facade rejects existing sessions and new actions against
   current state; queued booking cleanup is not the access-control boundary.
3. A verified Workday office/team change is retained with its effective date.
   At that date, unconfirmed derived planning defaults update; travel choices
   and confirmed bookings retain their meaning/history.
4. A partial Workday outage preserves known data and its freshness marker.
   Missing responses neither invent a leaver nor undo SCIM disable.
5. Committed lifecycle work releases future personal desk/parking claims through
   Booking and creates room/event handover tasks. An ambiguous correlation waits
   for review rather than matching mutable email.
6. SCIM deletion later removes that provisioning resource; internal history and
   identifiable reporting copies follow their respective retention policies.

### 6. Add the private booking-raised Teams adapter

1. Add `persistence/delivery/teams` implementation and scoped composition config;
   authenticate personal app lifecycle callbacks under transport/http/teams.
2. Notifications adds only the `booking-raised` private delivery capability and
   its individual's explicit preference evidence. Email/in-app remain existing
   channels for their configured message kinds.
3. Existing Booking fact recording is reused; no provider call enters a booking
   transaction. App registration connects the durable delivery handler.
4. At dispatch, Notifications checks current recipient-specific opt-in and valid
   personal conversation binding before calling the Teams adapter.
5. Retry throttling/transient failures; expose permanent blocked/uninstalled
   status. No fallback path creates duplicate email attempts or changes the
   authoritative booking outcome.
6. Verify correct private recipient and opt-out-before-dispatch. This capability
   grants no shared-channel delivery and no opt-in for other message types.

## Package mapping

These are adaptations of the code present today, not a claim that a booking
platform already exists. The current inventory contains 26 Go packages under
`internal`; the horizontal tree includes planned capabilities as well as moves.

| Current package/path | Candidate destination | Split or preservation |
| --- | --- | --- |
| cmd/spaceweave | cmd/spaceweave | thin main preserved |
| internal/cli | transport/cli + app | Cobra presentation versus current inline composition |
| internal/config | config | strict YAML; move profile values away from I/O dependency |
| internal/buildinfo | buildinfo | unchanged responsibility |
| internal/server | transport/http, plans, static | upload/review/profile routing; application decisions move to Plans |
| internal/catalog | model/plans + application/plans + store/postgres | local filesystem catalogue/locks replaced for production durability |
| internal/worker | application/plans + transport/jobs | workflow versus polling/dispatch; replace claiming with durable leases |
| internal/inbox | transport/inbox | watcher invokes shared ingest; local input remains optional |
| internal/trace | persistence/tracing/claude + provider-free model/support types | isolate SDK loop; retain trace/prompt/tool behaviour |
| internal/fakeclaude | persistence/tracing/fakeclaude | keep acceptance fixture capability |
| internal/plan | support/plan | schema/geometry validation; remove provider/workflow dependencies |
| internal/convert | support/convert | conversion algorithms retained |
| internal/cad | support/cad | retained |
| internal/dxf, internal/dxf/dxftest | support/dxf, support/dxf/dxftest | readers and fixtures retained |
| internal/dwg, internal/dwg/dwgtest | support/dwg, support/dwg/dwgtest | readers and fixtures retained |
| internal/svgdraw | support/svgdraw | retained |
| internal/vector | support/vector | retained |
| internal/raster | support/raster | retained |
| internal/floorgraph | support/floorgraph | retained |
| internal/autotrace | support/autotrace | deterministic orchestration; invert current trace-package value dependency |
| internal/autotrace/cadfront | support/autotrace/cadfront | retained extraction |
| internal/autotrace/rasterfront | support/autotrace/rasterfront | retained extraction |
| internal/autotrace/profile | support/autotrace/profile + persistence/profilefiles + application/plans | values/matching versus filesystem library versus correction workflow |
| internal/autotrace/score | support/autotrace/score | retained evaluation helpers |
| internal/autotrace/synth | support/autotrace/synth | retained fixtures |
| web/floorplan-iso | retained renderer + web/shared/renderer bridge | native JS/D3 used incrementally from React portals |
| test/acceptance | test/acceptance | preserve deterministic/fake-provider plan journeys; add durable booking slices |
| pkg/README.md | pkg/client plus existing guidance | no current public domain API to relocate |

## Numbers

Counts refer only to the full package tree printed above, including existing
support/test-helper packages and proposed generated server packages. They are
not estimates of final file size, effort or throughput. The repository-root/API
sketches and walkthrough paths are not counted again.

| Measure | Printed candidate |
| --- | --- |
| Internal Go packages | 78 |
| Application packages | 13 |
| Persistence packages | 23 |
| Representative Go files, including generated servers and architecture tests | 226 |
| Initial named SQL migrations | 12; one ordered sequence, individually owned |
| Dependency/ownership rules | 8; application edges and SQL owner manifest expand their checks |
| Independent runtime roles from one release | 5 |
| Explicit shared business transaction workflows | 2: venue/ticket capacity and visitor ticket/visit link |

No line-count estimate for the eventual PostgreSQL package is defensible from
this design. Its breadth is visible in the named files; query complexity and
schema contention need implementation evidence. Thin packages and new provider
adapters increase coverage obligations even when they contain few lines.

## Placement choices and friction

- **Persistence owns interfaces.** Application depends downward on interface
  roots beside implementations. Provider-specific SDK
  types cannot escape those interfaces. Moving consumer-owned ports upward
  would change this candidate toward the hexagonal/clean alternatives.
- **Model is shared horizontally.** Stable IDs and value types avoid transport
  coupling, but identity/booking/event changes can still affect many packages.
  Shared types need careful ownership; `model` is not a miscellaneous DTO dump.
- **Planning is private Booking behaviour.** Technical layering is retained,
  but demand, group/proposal progress and capacity commitment share one owner.
  Booking consumes a solver interface implemented by its private planning package;
  calculation runs outside short claim transactions. The broader service reduces
  cross-owner validation at the cost of more operation-specific private code.

- **Booking ownership is logical, not directory-enclosed.** Facets, manifests,
  allowed calls and real transaction tests compensate. A future developer can
  still bypass it by weakening checks; nested module boundaries would give a
  stronger structural obstacle with the same shared database.
- **One PostgreSQL package and migration order.** Coordination and transaction
  reuse are straightforward, while query ownership and review become demanding.
  Splitting storage into per-domain packages would reduce collision but begin
  rebuilding domain modules below a horizontal facade.
- **Application calls remain visible.** Events calls Booking/Visitors;
  Booking calls Identity/Policy/Workplace/Notifications; Identity keeps People,
  Access and Delegation as distinct internal/published facets.
  Every new edge must remain acyclic and authorised. The generic job registry
  removes import cycles but cannot become a way to hide synchronous invariants.
- **Shared transactions are explicit.** Events coordinates two narrow workflows
  across owners. Broadening that into arbitrary cross-service repository access
  would erase ownership. Forbidding it entirely would require a redesigned
  consistency boundary or compensating workflows that cannot promise the same
  local atomic behaviour without additional coordination.
- **Support libraries are substantial.** The existing CAD pipeline stays useful,
  but removing provider/config/I/O dependencies is real work. Pure geometry and
  local scratch processing must not regain catalogue or booking authority.
- **Transport has no persistence exception.** Plans mediates signed/private asset
  retrieval and authorisation. This centralises access rules at the cost of
  additional application methods for streaming and asset retrieval.
- **Provider truth stays outside local transactions.** Local constraints cannot
  make Exchange or Teams transactional. Durable desired/observed state, retries
  and visible recovery are necessary regardless of how tidy the layers look.
- **A vertical change crosses the tree.** Adding an event registration rule can
  touch generated transport, model, Events, Visitors, Booking, SQL/migration,
  React journeys and acceptance coverage. Layering makes technical placement
  predictable while scattering the complete business story.

This option is credible when the team values a familiar technical split and is
willing to enforce ownership through explicit rules. Its main comparison cost
is the distance between a logical capability's decisions, data and entry points;
the [other candidates](README.md) change that organisation while retaining the
same capacity, authority, privacy and recovery requirements.
