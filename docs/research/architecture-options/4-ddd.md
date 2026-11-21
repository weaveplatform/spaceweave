# Option 4: Domain-driven design

The repository-architecture decision is pending. This candidate expresses the
same agreed logical owners as the other four: combined Identity; attendance
planning and claim commitment inside Booking; separate Exchange and Events.
Its package organisation and dependency rules remain specific to this style.
[Identity](../../adr/0026-combine-workforce-identity-and-access-ownership.md),
[Booking planning](../../adr/0027-keep-attendance-planning-inside-booking.md) and
[Exchange](../../adr/0028-separate-exchange-synchronisation-from-booking.md)
record those ownership decisions independently of repository style.

SpaceWeave's booking, workforce identity, workplace configuration, visitor and
event capabilities use different models and consistency rules. This candidate
uses domain-driven design (DDD) to make those boundaries explicit inside one Go
application release serving approximately 100,000 staff and 60,000 desks.

DDD bounded contexts establish where a model and language apply; aggregates
identify consistency boundaries within a model. Neither concept prescribes a
separate service deployment. The [architecture foundations](architecture-foundations.md)
explain these principles from primary sources. This document evaluates a
concrete context-first layout against the [common requirements](README.md#requirements-held-constant).

## Summary

Organise the application into bounded contexts, each with a domain model,
application handlers, published interface and external adapters. Domain models
contain the language and behaviour of staff eligibility, allocation, claims,
visits and admission; adapters translate PostgreSQL, SCIM, Workday, Graph and
HTTP representations. Contexts exchange published commands and committed facts.

This candidate uses uniform context layers and defaults to aggregate-local
transactions. Some SpaceWeave operations require coordinated local writes across
several roots: the transaction policy therefore names those exceptions explicitly.
Microsoft's [domain-event guidance](https://learn.microsoft.com/en-us/dotnet/architecture/microservices/microservice-ddd-cqrs-patterns/domain-events-design-implementation)
discusses both same-transaction and eventual-consistency strategies; the choice
must follow the invariant and its failure model. An allocation group, required desk/parking and an
office transfer cannot be partially confirmed while compensating later. An
event's issued-ticket limit cannot race a venue reduction.

The alternative of requiring every transaction to remain within one aggregate
is evaluated below, including the coordination and contention costs.
The aggregate pattern and the bounded-context pattern describe different scopes;
neither a context nor an aggregate implies a separate Kubernetes deployment.
Evans's reference and Vernon's aggregate-design papers provide the primary
modelling sources; the placements here are SpaceWeave design inferences.
[DDD reference][evans], [aggregate design][vernon].

### Dependency rules

1. A context imports its own subtree, the small shared kernel, technical shared
   facilities where appropriate, and another context's published `api` only.
2. `domain` imports its own domain packages, the kernel and standard library;
   it does not import SQL, HTTP, provider clients, another context's interface,
   its application handlers or technical shared infrastructure.
3. `application` orchestrates domain behaviour through repository/provider
   interfaces and published context contracts; it never imports its adapters.
4. `api` exposes deliberate command/query types, outcomes and published facts.
   It does not expose repository handles or application/adapter implementations.
5. `shared` contains technical mechanisms and imports no context. `kernel`
   contains only agreed identifiers/value semantics and imports no context or
   provider. A shared `Staff` or `Booking` entity is deliberately excluded.
6. Only composition imports context root factories and assembles more than one
   context's concrete adapters. HTTP adapters do not write repositories directly.
7. Each context owns its business tables/migrations; shared operational facilities
   separately own incident declarations and shared execution tables. Booking is the sole physical-space claim writer,
   including calls from allocation, Exchange observations and timers.
8. Default transactions change one aggregate and its outbox records. The named
   booking consistency operations, event-venue coordination and registration-created
   visit/ticket coordination are documented
   exceptions; a generic transaction handle does not authorise foreign writes.
9. Published context facts and wire contracts are versioned separately. Generated
   HTTP/SCIM/provider types stay outside domain models.

The compiler rejects import cycles, but root-level `internal` alone does not
prevent sibling-context imports. Depguard/import checks enforce these rules;
query/migration review enforces SQL ownership. [Go import rules][go-imports].

## Repository root

```text
spaceweave/
├── cmd/spaceweave/                      retained executable; Cobra drives app composition
├── internal/                            ten contexts, kernel, shared, composition
├── pkg/                                 reserved for deliberate public clients/helpers
├── api/
│   ├── src/app/                         OpenAPI fragments organised by owning context
│   ├── src/provisioning/                SCIM wire schemas/compatibility documentation
│   └── generated/                       reproducible bundled contracts, not domain models
├── web/
│   ├── staff/                           staff/mobile-web React entrypoint
│   ├── admin/                           separate React administrative entrypoint
│   ├── shared/                          UI, generated clients and renderer integration
│   └── floorplan-iso/                   retained renderer during incremental adoption
├── test/
│   ├── acceptance/                      retained harness; add booking/provider scenarios
│   ├── integration/                     proposed context assembly/transaction suites
│   └── fixtures/                        synthetic offices, staff, events and provider data
├── deploy/
│   ├── kubernetes/                      proposed HTTP/worker role definitions, same image
│   └── operations/                      proposed migration, fencing and recovery procedures
├── docs/research/architecture-options/
├── Makefile                             generation, existing gates and acceptance entrypoints
├── .golangci.yml                        existing linter plus context import restrictions
├── .testcoverage.yml                    existing coverage requirements retained
└── go.mod                               one module; builds retain GOWORK=off
```

Existing Go/Chi/Cobra/configuration and quality conventions remain from
[CLAUDE.md][conventions]. The new paths above are candidate destinations, not
evidence that a database, booking platform or deployment already exists.

## Full package tree

Every context has `api`, `domain`, `application` and `adapters` in this candidate.
Uniform layering is a local organisation choice. Filenames name representative responsibilities,
not a commitment that every operation deserves its own Go package. Co-located
tests are omitted here and described in the verification section.

```text
internal/
├── app/
│   ├── app.go                                lifecycle and shutdown
│   ├── contexts.go                           constructors and published interfaces
│   ├── routes.go                             Chi assembly and generated-handler dispatch
│   ├── incidents.go                          wire authorised commands to the operational facility
│   ├── roles.go                              HTTP/allocation/Exchange/tracing/maintenance roles
│   ├── subscriptions.go                      published-fact consumers and version routing
│   └── migrations.go                         controlled context migration manifest
├── kernel/
│   ├── ids/
│   │   └── ids.go                            StaffID, SpaceID, OfficeID, BookingID, EventID
│   ├── principal/
│   │   └── principal.go                      actor identity envelope, not entitlement evaluation
│   ├── timewindow/
│   │   └── window.go                         interval value semantics, not an office calendar
│   └── clock/
│       └── clock.go                          clock abstraction for time-dependent decisions
├── shared/
│   ├── postgres/
│   │   ├── database.go                       pool, health and scoped transaction execution
│   │   ├── retry.go                          bounded whole-transaction retry support
│   │   └── migrations.go                     ordered migration runner, owner records
│   ├── incident/
│   │   ├── incident.go                       confirmed scope/times/revision; no business expiry policy
│   │   ├── store.go                          operational declaration writer and guarded reads
│   │   └── migrations/
│   │       └── 0001_incidents.sql
│   ├── outbox/
│   │   ├── append.go                         append within owning transaction
│   │   └── dispatch.go                       leased delivery and consumer progress
│   ├── jobs/
│   │   ├── queue.go                          durable job records and versioned payload envelope
│   │   └── lease.go                          claim, renew, fence, retry and park
│   ├── httpkit/
│   │   ├── router.go                         Chi middleware and transport error mapping
│   │   └── csrf.go                           browser session request protection
│   ├── telemetry/
│   │   └── telemetry.go                      traces, metrics and structured logs
│   ├── config/
│   │   └── config.go                         strict deployment YAML, flag > env > file
│   └── generated/
│       ├── app/
│       │   └── app.gen.go                    generated HTTP wire types/handler contracts
│       └── provisioning/
│           └── scim.gen.go                   generated wire shapes where tooling supports them
├── identity/
│   ├── identity.go                           one context factory; distinct People/Access/Delegation
│   ├── api/
│   │   ├── people.go                         verified source/eligibility and dated workforce views
│   │   ├── access.go                         actor/owner/action/office and session authority
│   │   ├── delegation.go                     local grants and fresh native delegation checks
│   │   └── events.go                         source, eligibility, access and grant facts
│   ├── domain/
│   │   ├── staff/
│   │   │   ├── staff.go                      internal identity and verified source links
│   │   │   └── eligibility.go                independent account/provisioning/workforce inputs
│   │   ├── workforce/
│   │   │   └── assignment.go                 effective office/structure/team facts
│   │   ├── team/
│   │   │   └── team.go                       external membership model
│   │   ├── scope/
│   │   │   └── scope.go                      office/global authority semantics
│   │   ├── delegation/
│   │   │   └── grant.go                      provenance, owner, actions and verification age
│   │   ├── session/
│   │   │   └── session.go                    staff/visitor session lifecycle
│   │   ├── repository.go                     Identity-owned people/session/grant repositories
│   │   └── events.go                         internal domain facts
│   ├── application/
│   │   ├── provision.go                      SCIM lifecycle and mapped groups
│   │   ├── import_workday.go                 source-owned effective workforce facts
│   │   ├── correlate.go                      verified matching and ambiguity review
│   │   ├── reconcile.go                      complete-source reconciliation
│   │   ├── queries.go                        current eligibility and dated facts
│   │   ├── login.go                          SSO and restricted visitor sessions
│   │   ├── decide.go                         current own eligibility/action/scope checks
│   │   ├── delegation.go                     app-managed grants and native evidence evaluation
│   │   ├── native_delegation.go              freshness refresh through authority adapter
│   │   └── revoke.go                         observed disablement plus session/access invalidation
│   └── adapters/
│       ├── http/
│       │   ├── scim.go                       provisioning resources
│       │   ├── people.go                     correlation queries/actions
│       │   ├── login.go
│       │   ├── middleware.go
│       │   └── delegation.go
│       ├── workday/
│       │   ├── client.go
│       │   └── translate.go
│       ├── entra/
│       │   └── oidc.go
│       ├── exchangeauthority/
│       │   └── permissions.go                fresh explicit native delegation evidence
│       └── postgres/
│           ├── staff.go
│           ├── teams.go
│           ├── access.go
│           └── migrations/
│               ├── 0001_identity_people.sql
│               └── 0002_identity_access.sql
├── workplace/
│   ├── workplace.go
│   ├── api/
│   │   └── workplace.go                      versioned definitions and impact-aware changes
│   ├── domain/
│   │   ├── office/
│   │   │   └── office.go                     location/building/floor hierarchy and calendar
│   │   ├── space/
│   │   │   └── space.go                      kind, features, active definition and capacity
│   │   ├── neighbourhood/
│   │   │   └── bundle.go                     one-building baseline desks/optional rooms
│   │   ├── mapping/
│   │   │   └── mapping.go                    versioned geometry-to-stable-space associations
│   │   └── repository.go
│   ├── application/
│   │   ├── publish_mapping.go                activate approved mapping after Booking impact validation
│   │   ├── configure.go                      validation and committed-booking impact checks
│   │   ├── queries.go                        versioned workplace snapshots
│   │   └── retirement.go                     scheduled retirement/reactivation and deletion dependencies
│   └── adapters/
│       ├── http/
│       │   └── workplace.go
│       └── postgres/
│           ├── workplace.go
│           └── migrations/
│               └── 0001_workplace.sql
├── policy/
│   ├── policy.go
│   ├── api/
│   │   └── policy.go                         resolve and publish policy versions
│   ├── domain/
│   │   ├── policy/
│   │   │   ├── version.go                    immutable published values/effective scope
│   │   │   └── validate.go                   individual and cross-field validation
│   │   └── repository.go
│   ├── application/
│   │   ├── publish.go                        global/office authority, publication and migration requests
│   │   └── resolve.go                        pinned booking/cycle rules and permitted overrides
│   └── adapters/
│       ├── http/
│       │   └── policy.go
│       └── postgres/
│           ├── policy.go
│           └── migrations/
│               └── 0001_policy.sql
├── booking/
│   ├── booking.go
│   ├── api/
│   │   ├── booking.go                        claim commands, scoped cancellation and room recovery
│   │   ├── allocation.go                     snapshot, proposal and completed-plan publication interface
│   │   ├── venue.go                          restricted event-managed venue operations
│   │   ├── evidence.go                       validate/store operational observations and publish facts
│   │   └── events.go                         committed booking/check-in/release facts
│   ├── domain/
│   │   ├── intent/
│   │   │   └── intent.go                     chosen office/date, requirements and collaboration
│   │   ├── allocation/
│   │   │   ├── cycle.go                      date cohort, cutoff and processing version
│   │   │   ├── group.go                      date-specific allocation group, distinct from team
│   │   │   ├── proposal.go                   candidate placements and dependency versions
│   │   │   └── planner.go                    pure grouping/packing objective, no persistence
│   │   ├── claim/
│   │   │   ├── claim.go                      booking lifecycle and owned resource claims
│   │   │   ├── requestset.go                 pair identity, required kind and cancellation scope
│   │   │   ├── transition.go                 desk use/expiry; native room outcomes and cancellation
│   │   │   ├── series.go                     native recurrence/intent and cancellation fence
│   │   │   └── occurrence.go                 one-off/series instance and native outcome references
│   │   ├── waitlist/
│   │   │   ├── request.go                    eligible unmet demand and priority
│   │   │   └── offer.go                      timed capacity offer and acceptance state
│   │   ├── inventory/
│   │   │   └── dated.go                      dated capacity and unallocated-desk bundle movements
│   │   ├── evidence/
│   │   │   └── evidence.go                   typed operational observations, provenance and freshness
│   │   ├── repository.go                     Booking-owned repository interfaces
│   │   └── events.go
│   ├── application/
│   │   ├── submit_intent.go                  app-owned preferences and dated travel choices
│   │   ├── plan_cycle.go                     assemble immutable snapshot and run planner
│   │   ├── solver.go                         consumed solver interface; calculation outside tx
│   │   ├── publish_plan.go                   completed-plan publication across affected aggregates
│   │   ├── direct.go                         direct requests after plan publication; waitlist priority
│   │   ├── transfer.go                       named acquire-replacement/release-old transaction
│   │   ├── checkin.go                        owner-only personal transition
│   │   ├── protection.go                     apply shared incidents to Booking-owned deadlines
│   │   ├── ingest.go                         validate scope/provenance, deduplicate and commit evidence
│   │   ├── release.go                        explicit selected/pair cancellation; timed desk-only release
│   │   ├── promote.go                        reserve/accept/expire eligible waitlist offers
│   │   ├── room.go                           room intentions, acceptance and retained-meeting recovery
│   │   ├── room_series.go                    native-policy series requests and cancellation
│   │   ├── room_use.go                       native check-in orchestration and observed room release
│   │   ├── lifecycle.go                      owner-based desk/parking/room leaver cancellation
│   │   └── venue.go                          event-governed capacity participant
│   └── adapters/
│       ├── occupancy/
│       │   └── input.go                      concrete source translation into Booking commands
│       ├── solver/
│       │   └── solver.go                     replaceable search using domain constraints/objective
│       ├── http/
│       │   ├── booking.go
│       │   ├── intents.go
│       │   └── checkin.go
│       └── postgres/
│           ├── claims.go                     overlap protection and consistent resource lock protocol
│           ├── allocation.go                 snapshot versions, runs and provisional/publication progress
│           ├── waitlist.go                   competing offers and acceptance
│           ├── inventory.go                  dated neighbourhood/capacity rows
│           ├── evidence.go                   Booking-owned desk evidence; native room outcome refs
│           ├── rooms.go                      series/occurrence state and cancellation fences
│           └── migrations/
│               └── 0001_booking.sql
├── exchange/
│   ├── exchange.go
│   ├── api/
│   │   └── exchange.go                       provider desired/observed state and reconciliation
│   ├── domain/
│   │   ├── conversation/
│   │   │   └── reconciliation.go             durable desired/base/observed event intentions
│   │   ├── subscription/
│   │   │   └── cursor.go                     per-mailbox/window progress, not all-history delta
│   │   └── repository.go
│   ├── application/
│   │   ├── on_room_intent.go                 committed app intention to provider work
│   │   ├── reconcile.go                      compatible edits, stale work and irreconcilable cases
│   │   └── observe.go                        retain provider facts and call Booking
│   └── adapters/
│       ├── graph/
│       │   ├── calendar.go                   create/edit/cancel with supported provider semantics
│       │   └── delta.go                      subscriptions plus durable window reconciliation
│       ├── http/
│       │   └── notifications.go              provider callback validation and enqueue
│       └── postgres/
│           ├── exchange.go
│           └── migrations/
│               └── 0001_exchange.sql
├── plans/
│   ├── plans.go
│   ├── api/
│   │   └── plans.go                          upload, process, review and read revision
│   ├── domain/
│   │   ├── revision/
│   │   │   └── revision.go                   immutable drawing/render revision and current pointer
│   │   ├── publication/
│   │   │   └── publication.go                staged/recovery-verified/durable/publication states
│   │   ├── blob.go                           provider-neutral opaque object token contract
│   │   └── repository.go
│   ├── application/
│   │   ├── upload.go                         accept staged source data
│   │   ├── trace.go                          retained pipeline behind a context-owned interface
│   │   ├── review.go                         approve/reject immutable assets; no inventory activation
│   │   ├── publish.go                        verify recovery copies before durability acknowledgement
│   │   └── cleanup.go                        referenced history and abandoned staging reconciliation
│   └── adapters/
│       ├── tracing/
│       │   └── pipeline.go                   autotrace/convert/floorgraph wrapper, not a rewrite
│       ├── blob/
│       │   ├── azure/
│       │   │   └── blob.go                   Azure conditional operations and private access
│       │   └── gcs/
│       │       └── blob.go                   GCS generation preconditions and private access
│       ├── http/
│       │   └── plans.go                      existing upload/review transport adapted incrementally
│       └── postgres/
│           ├── plans.go                      metadata/revision ownership, not object bytes
│           └── migrations/
│               └── 0001_plans.sql
├── visitors/
│   ├── visitors.go
│   ├── api/
│   │   └── visitors.go                       hosted visit and restricted visitor identity views
│   ├── domain/
│   │   ├── visit/
│   │   │   └── visit.go                      host/window/reception arrival/departure
│   │   └── repository.go
│   ├── application/
│   │   ├── invite.go
│   │   ├── reception.go
│   │   └── retention.go                      remove aged visits and identifying copies
│   └── adapters/
│       ├── http/
│       │   └── visitors.go
│       └── postgres/
│           ├── visitors.go
│           └── migrations/
│               └── 0001_visitors.sql
├── events/
│   ├── events.go
│   ├── api/
│   │   └── events.go                         audience, registration, venue and admission operations
│   ├── domain/
│   │   ├── event/
│   │   │   └── event.go                      audience/access, capacity and secured-venue reference
│   │   ├── ticket/
│   │   │   └── ticket.go                     named entitlement and single-use admission
│   │   └── repository.go
│   ├── application/
│   │   ├── venue_coordinator.go              named local Events/Booking transaction exception
│   │   ├── register.go                       ticket capacity plus required visitor visit link
│   │   ├── protection.go                     apply shared incidents to Events-owned deadlines
│   │   ├── admit.go                          duplicate-scan-safe admission
│   │   └── handover.go                       leaver Event ownership review
│   └── adapters/
│       ├── http/
│       │   ├── events.go
│       │   └── tickets.go
│       └── postgres/
│           ├── events.go
│           ├── tickets.go
│           └── migrations/
│               └── 0001_events.sql
├── attendance/
│   ├── attendance.go
│   ├── api/
│   │   └── attendance.go                     derived attendance projections and scoped reports
│   ├── domain/
│   │   ├── report/
│   │   │   └── report.go                     booking/check-in/occupancy dimensions and freshness
│   │   └── repository.go
│   ├── application/
│   │   ├── project.go                        committed-fact attendance projections
│   │   ├── report.go                         own history or authorised office reporting
│   │   └── retention.go                      configured reporting projection/copy retention
│   └── adapters/
│       ├── http/
│       │   └── attendance.go
│       └── postgres/
│           ├── attendance.go
│           └── migrations/
│               └── 0001_attendance.sql
├── notifications/
│   ├── notifications.go
│   ├── api/
│   │   └── notifications.go                  recipient preferences and delivery outcome queries
│   ├── domain/
│   │   ├── message/
│   │   │   └── message.go                    committed outcome and private intended recipient
│   │   ├── delivery/
│   │   │   └── delivery.go                   channel attempt, retry state and message-specific opt-in
│   │   └── repository.go
│   ├── application/
│   │   ├── on_booking_raised.go              individual opted-in Teams message, not a broadcast
│   │   ├── on_outcome.go                     in-app/email booking/visit/ticket/handover messages
│   │   └── deliver.go                        consent recheck, retry classification and outcomes
│   └── adapters/
│       ├── email/
│       │   └── sender.go                     organisation-approved email transport
│       ├── teams/
│       │   └── personal.go                   proactive personal bot message and conversation reference
│       ├── http/
│       │   └── preferences.go
│       └── postgres/
│           ├── notifications.go
│           └── migrations/
│               └── 0001_notifications.sql
├── autotrace/                                retained geometry packages; accessed through Plans
├── cad/
├── convert/
├── dwg/
├── dxf/
├── floorgraph/
├── raster/
├── svgdraw/
├── vector/
├── plan/                                     retained trace value/pipeline packages during migration
├── trace/
└── buildinfo/                                retained build metadata
```

The ten context roots are Go factories rather than global registries. The
tree does not multiply the current CAD package inventory into ten contexts.
Source import/package claims above are checked against the current
[server][server], [catalogue][catalogue], [worker][worker] and [drawing pipeline][autotrace].

## API surfaces and consumers

The [shared catalogue](../api-surfaces/README.md) now records the public REST
consumers as UI, MCP servers, SDKs, scripts, contract tests and Terraform. The
contexts' published `api` packages are still private to application code. Those
two meanings of publication are deliberately distinct.

| Consumer/surface | Kind and authority | Context/wire binding |
| --- | --- | --- |
| UI, MCP, SDK, scripts, contract tests and Terraform | Shared supported public REST with capability-specific fields/actions. | Owning contexts' HTTP adapters and app routing call application handlers; no aggregate or complete context API is serialized wholesale. |
| Human users and enrolled applications | Internet-accessible public REST with applicable Conditional Access plus current application grants. | Credential/profile checks at the edge; Identity and the command owner preserve real actor/client/owner identity. Machine grants are separate from human delegation. |
| Internal-event visitors, such as guest speakers | External invitation/verification and limited visitor event/visit/ticket operations. | Visitors/Events projections preserve the verified-email visitor scope; no general public event catalogue is established. |
| Entra, Graph, Teams and source clients | Native authenticated provisioning/callback/ingestion contracts. | Identity, Exchange, Notifications and Booking adapters retain protocol-specific semantics and caller validation. |
| Platform workers and application contexts | Private Go interfaces and versioned committed facts/jobs. | Workers call context application handlers directly; context API and repository/provider ports do not become remote endpoints. |
| Operators and platform dependencies | Private probes/telemetry; authorised public repair commands where supported. | Restricted operational routing plus owning commands; public repair capability does not expose SQL, repositories or transaction participation. |

Context-owned OpenAPI fragments can build one supported public contract family
with appropriate consumer/security views. The UI is a public REST consumer;
its release no longer defines the entire compatibility lifetime. A public SDK
is generated from the wire contract, not context `api` Go types.

The named Events/Booking coordinator remains behind authorised commands. A public
client cannot inject a transaction handle, native delegate ACL or manufactured
domain event. Private context/fact contracts can evolve with the application,
subject to persisted-version/replay compatibility. Human/application authority is
distinct; Terraform initially manages long-lived configuration; first-party
MCP/provider delivery follows the core platform/SDKs. Public versions evolve
independently of the UI. Identity owns configurable RBAC; MCP uses user context
and machine-identity scripts use a different channel. UI drift is permitted and
initial retirement notice is 180 days. Global administration, SCIM-backed role
mappings, grant-only scoped composition and self-permission discovery are agreed.
The [agreed request contract](../api-surfaces/request-behaviour.md) places command
receipts within the owning command/coordinator transaction, preserving the stated
shared-transaction exceptions, previous-version configuration edits, two request
classes, actor budgets and public operation status. Wire and retention
details remain open. Counts above remain tied
to the printed context tree rather than these explanatory surface bindings.

## Deep dives

The [two-phase allocation decision](../../adr/0037-form-collaborator-groups-before-packing-desk-allocations.md)
and [supporting research](../two-phase-desk-allocation.md) apply here: phase one
forms collaborator groups, including singletons; phase two packs those groups
into existing seats with member-specific requirements and required parking.
Sitting together precedes minimum walking distance within the agreed proximity
objectives. Quality takes precedence over compute cost; unequal placement quality
is expected. Completed-plan publication preserves previously published bookings;
intermediate persistence creates no protected confirmations. Algorithms, handling
of groups that cannot fit, and the publication mechanism remain unselected.

### Domain model and context map

Identity's `Staff` is a correlated identity with effective source facts.
Its distinct Access/People/Delegation interfaces use the same owned security
state; Booking consumes a staff identifier and dated team/office snapshot. An allocation group is created for an attendance
date, not imported from Workday and not synonymous with a team. None of these
models owns Entra tokens. Exchange's observed organiser, booking owner and actual
booker have separate fields and meanings.

Workplace owns stable space identity and baseline neighbourhoods. Booking's
`DatedInventory` represents availability and dated membership adjustments; it
does not mutate Workplace's baseline definitions during a run. Plans owns
geometry revisions, not desk claims or authoritative space identity. An amended
drawing can be reviewed before becoming the current plan without moving an
existing booking to a different inferred polygon.

Context relationships are Identity → eligibility/authority/workforce consumers;
Workplace/Policy → Booking's internal planning; Booking → Exchange, reporting
and notifications through committed facts; Events → Booking/Visitors through
published commands. Source status and session/access invalidation share Identity's
transaction; separate Directory and Access contexts are not retained. Composition
supplies a narrow impact-check interface where Workplace configuration must
consult committed Booking state, avoiding a cycle between concrete packages.

### Aggregates and the transaction exception

Candidate roots include StaffIdentity, PolicyVersion, AttendanceIntent,
AllocationCycle, RequestSet, BookingClaim, WaitlistOffer, RoomConversation,
PlanRevision, Visit and EventRegistration. Their boundaries are hypotheses to
validate against real concurrency, not a one-to-one translation of SQL tables.
Aggregate repositories do not load an entire office's annual booking history
just to change one desk.

| Invariant | Aggregate-local transaction challenge | Proposed coordination |
| --- | --- | --- |
| The completed allocation plan publishes together, preserving group and required-parking consistency | Independent claims/resource capacity belong to several roots. | Booking's `publish_plan.go` validates the complete proposal and coordinates publication across affected roots; final-transaction versus guarded-staging mechanics require proof. |
| Required desk plus parking both confirm or neither does | A request-set root alone cannot prevent another request-set claiming the same desk/pool capacity. | Shared Booking transaction coordinates request-set and competing resource/staff guards. |
| Before either starts, both-required pair cancellation ends both bookings or neither | The pair contains separate Booking claim roots. | Booking's release handler resolves the pair from either ID and revalidates link/start state, authority and resource/pool guards; both claims, retry outcome and facts commit together. |
| A person has no overlapping desks across offices | Independent office/date aggregates cannot alone enforce the global interval invariant. | A canonical Booking claim protocol and PostgreSQL staff-overlap protection apply to every writer. |
| Venue capacity remains at least active issued tickets | Venue claim is Booking-owned; registration/tickets are Events-owned. | Events' named venue coordinator uses both published interfaces in one local transaction and serialises ticket issuance against venue changes. |
| A visit created for registration commits with its ticket or neither does | Events and Visitors own separate records. | Events' registration coordinator joins Visitors' published participant in one local transaction; failed issuance preserves any reused existing valid visit. |

A strict alternative could use a huge aggregate containing office/date capacity,
all affected people and related tickets. Cross-office transfers and staff overlap
then expand its ownership further; it risks serialisation and poor locality.
Another strict alternative stages tentative claims through a process manager,
then confirms them together only after durable preparation. That needs a
coordination protocol covering shared capacity and the final visibility decision;
ordinary compensating events alone do not satisfy the agreed atomic commit.

The candidate chooses explicit exceptions instead of pretending those problems
vanish. If one-aggregate-per-transaction is non-negotiable, this concrete candidate
needs redesign and cannot simply be adopted as printed. It remains recognisably
DDD in language, context mapping, aggregate behaviour and provider translation;
its transaction policy makes the consistency/coupling tradeoff explicit.

### Booking write path and persistence

Booking's application layer owns the local transaction decision. A handler
loads policy/definition versions and relevant current state, validates the
domain command, acquires capacity/staff guards in a consistent order, rechecks
affected versions, saves Booking-owned records and appends committed facts.
No context saves a `BookingClaim` by calling a shared generic repository.

Before either linked booking starts, explicit cancellation of either component
of a both-required desk/parking pair cancels both in one Booking transaction.
Optional links allow independent cancellation. The command resolves and
revalidates the persisted pair, start times, current authority and claim state
under the normal guards; its receipt, affected claims and follow-up facts commit
together. Automatic desk no-show or vacancy release instead ends the desk claim
only. Parking remains booked until its own end or explicit release, including
in bay/pool capacity calculations. Desk-use evidence cannot establish that a car
has left. The [linked-booking contract](../api-surfaces/linked-bookings.md)
defines the public outcomes and concurrency validation. Once either booking
starts, explicit cancellation affects only the selected booking unless the caller
explicitly chooses both. Retain the other until its own end or explicit
cancellation. Preserve the requested scope in the command/retry identity;
revalidation at the first-start boundary cannot silently change affected claims.

PostgreSQL range exclusion can protect overlapping intervals for a specific
resource/person. Capacity pools need additional coordinated count/guard logic;
locking only existing bookings does not lock a currently empty slot. Use real
transaction tests for the chosen protocol. Serialization failures require
retrying the whole decision, not blindly retrying its final statement.
[Range constraints][pg-ranges], [row locks][pg-locks], [transaction retries][pg-retry].

Each context owns its PostgreSQL queries and migration sequence. Shared
infrastructure opens a pool and runs a manifest; it does not own every business
table. An explicit coordinator passes a bounded transaction participation
capability to Events/Booking adapters through their owning interfaces. It cannot
request arbitrary SQL from another context. Foreign keys on stable IDs are a
separate migration choice; they do not make context internals public.

### Allocation and neighbourhood flexing

`plan_cycle.go` assembles a versioned view of dated capacity, features, office
calendar, eligible intents, team memberships and committed claims. The pure
planner first satisfies hard constraints and accommodates demand, then applies
explicit collaboration → general team proximity → previously unfulfilled
demand among equal-demand placements. It forms per-date groups and packs them
into building-bound neighbourhood/floor capacity. Teams may split across floors.

Only specific unallocated desks may move between dated bundles. Every proposed
movement is versioned with the group placement that relies on it, and Booking
revalidates that the desks remain unallocated. A changed proposal does not move
another person's confirmed booking or silently alter a staff-chosen attendance
day. After the main plan is published, direct spare-desk booking uses the same claim interface
and waitlist priority. Late intents are incremental work, not a second writer.

### Check-in, occupancy and timed work

`claim.Transition` distinguishes allocated and taken. The command's actor must
be the owner for desk check-in, including QR-driven requests. Reports receive
identified check-in facts separately from anonymous occupancy. Raw signals retain
their actual evidence granularity: an office-presence observation is not proof
of use of a particular desk.

`release.go` and `promote.go` call the same Booking transition/claim interface
as HTTP commands. A valid allocation is atomically checked against no-show
expiry. A released desk returns to the pool even if a sensor says occupied;
after valid check-in, missing hardware evidence does not establish vacancy.
`shared/incident` owns neutral incident declarations and its migration, importing
kernel/standard-library values without any context model. Composition wires
authorised declaration commands; Booking and Events consume private incident
reads and apply protection through their own application handlers. Confirmed
incidents protect relevant check-in/offer/reconfirmation deadlines with configured
recovery grace. An old timer cannot reclaim a desk from its newly confirmed owner.

### Exchange and delegated booking

Exchange is a context for provider conversations, reconciliation and progress,
not another availability authority. `Booking.RoomRequested` produces durable
work after the local hold commits. The provider call creates/updates the owner's
calendar event; the PA remains the recorded app actor. Room acceptance is a
separate observation before Booking confirms local capacity. Preserve external
observations that conflict with local claims instead of dropping them on a SQL
constraint failure.

Persist base/desired/observed state and retry identity. Reconcile compatible
field changes, discard superseded work and surface incompatible intentions with
enough context for resolution. Cancellation must survive stale updates. Graph's
event `changeKey` is not assumed to be an ordered number or an undocumented CAS
contract, and a room replacement is not assumed atomic. Notifications plus
window-scoped delta reconcile at the agreed five-minute healthy cadence.
[Event semantics][graph-event], [delta scope][graph-delta],
[room acceptance][room-policy].


If an ordinary meeting's replacement request fails after the previous room is
lost, retain the meeting and attendees and expose no secured room. Booking owns
the room intention/claim outcome; Exchange retains provider observations and
reconciliation progress. Notify the organiser and offer available replacements
or explicit cancellation. A selected replacement remains pending until current
room acceptance and Booking capacity validation succeed. The
[room recovery contract](../api-surfaces/room-recovery.md) distinguishes this
outcome from uncertain provider state and preserves Events-managed venue rules.

Identity records source-labelled native grants and app-managed grants independently.
New native-derived actions need fresh verification within the configurable
fifteen-minute limit. Revocation of one source does not remove an independent
grant. App-managed authority permits app-mediated outcomes but does not write
native Outlook ACLs. Application mailbox access and the human actor's authority
remain separate checks. [Application permission scope][exchange-rbac].

### SCIM, Workday and staff lifecycle

The Identity context's provider adapters form an anti-corruption layer: SCIM
`active`, Entra account status and Workday workforce status remain distinct inputs
with explicit authority. Workday owns effective workforce structure/usual office;
the app owns preferences and dated travel. Only mapped groups become allocation
teams. Resolve source records through an approved identifier; quarantine ambiguous
links, never silently merge people by email.

On an observed effective disable/leaver, current eligibility denies further
staff access, including existing sessions. An asynchronous session-cleanup event
cannot be the only guard against known ineligibility. During feed failure, retain
last verified state; absence from a partial response is not a delete. Effective
office/team changes update unconfirmed derived defaults, preserving explicit
travel and confirmed bookings. A confirmed effective workforce departure
separately triggers personal desk/parking cleanup and cancellation of all
organiser-owned room meetings/series through Booking and Exchange. A disable is
not a departure; other-owner meetings created by the departing PA remain.
Events retains its own handover.

SCIM resource deletion hides it from that provisioning interface; it does not
imply erasing internal booking/audit history outside retention policy. Workday
entry/effective dates and periodic related-entity reconciliation need explicit
mapping. Tenant schemas and identifiers require verification, not a hard-coded
claim about WorkerID immutability. [SCIM deletion][scim-delete],
[Entra provisioning][entra-scim], [Workday integration][workday].

### Events and visitors

`EventRegistration` owns audience, invitation/public access rules, ticket limit
and named entitlements. A venue is a Booking-owned claim referenced by stable ID.
Generic booking edits reject an uncoordinated change to an event-governed claim.
The venue coordinator revalidates secured venue/time/capacity and serialises with
ticket issuance before either context commits. An observed external invalidation
is retained. Existing tickets keep their entitlements pending venue recovery;
admission and further issuance pause. The organiser explicitly secures a suitable
replacement or cancels, with confirmed changes communicated to attendees. The
model does not invent capacity to keep old tickets looking valid.

A visitor ticket requires a valid visit link before publication. Events'
registration handler coordinates Visitors' published participant in one named
local transaction: a visit created specifically for registration and its ticket
commit together or neither does. A reused existing valid visit survives failed
issuance. This agreed exception rejects staged orphan-visit cleanup; each context
still writes only its own state. Admission also rechecks the venue-recovery pause.
Reception arrival/departure and single-use event admission are separate
transitions. Retention removes aged visit data and identifying projection copies;
the retained event view cannot quietly restore deleted visitor details.

### Plans, blobs and the existing renderer

Plans wraps current autotrace/convert/floorgraph and renderer behaviour. Domain
objects describe revisions and publication, not DXF entities or DOM state.
The retained deterministic algorithms and fixtures remain in their current
packages until a justified change; this option does not require a geometry rewrite.

Blob adapters use provider-specific conditional operations behind an opaque
token contract. PostgreSQL metadata and object bytes do not share a transaction.
Stage an upload, verify recovery copies, record durability/publication, then
update a Plans-owned asset pointer with concurrency control. Workplace owns
geometry-to-stable-space mappings through its mapping domain and publication
command; Booking validates dated-inventory and commitment impact before
activation. One admin workflow may approve then activate, exposing each actual
outcome. Asset approval alone cannot change operational definitions. Orphan staging and
missing publication work are reconciled. Keep referenced historical revisions.
Default blob geo-replication alone does not establish the five-minute RPO.
[Azure concurrency][azure-blob], [GCS preconditions][gcs-blob].

### Policies, reporting and notifications

Policy versions are domain records with publication/effective dates, valid
office overrides and explicit migration behaviour. They are separate from
shared deployment configuration. Booking pins the applicable rules; publishing
a new no-show duration does not silently reinterpret existing confirmations.
The accepted numerical starting values remain configurable in
[policy defaults][defaults].

Booking owns validation, deduplication and custody of operational occupancy
evidence through its evidence domain, ingestion command, source adapter and
PostgreSQL evidence repository. Committed facts feed Attendance, which owns
reporting projections, queries and separately configured reporting retention.
Consumer progress/freshness is visible; a lagging projection cannot authorise a claim.
Staff see their own history; reporting-capable administrators see assigned
offices. Raw occupancy, attendance and visitor retention are separate and apply
to identifying copies. Neither attendance reports nor app policies enforce a
minimum number of office days.

Notifications consumes committed outcomes. Email/in-app cover the agreed
workflows; a private Teams booking-raised message requires the individual's
opt-in for that message. Teams installation/conversation access is a technical
prerequisite, not consent. A proactive personal bot adapter sends the message
asynchronously; blocked/uninstalled delivery is a visible permanent outcome,
transient failures retry, and no Teams failure rolls back the booking or creates
duplicate fallback emails. [Teams proactive messaging][teams].

### Durable workers, events and runtime health

Publish domain facts only after invariant-preserving commits, through the local
outbox. Facts contain event ID, producer/version, subject and ordering/revision
context; minimise personal payloads. Subscribers persist processing identity
with their local changes. Delivery can repeat; retries must not reissue tickets,
double-release claims or send an unconsented Teams message. Side-effect delivery
still needs provider reconciliation where the provider lacks idempotency.

One image supports separate HTTP, allocation, Exchange, tracing and maintenance
roles. Each opens the context subset/adapters it needs, with independently scoped
runtime identities. Jobs are PostgreSQL-backed with leases/fencing and observable
lag/failure states. A Graph outage degrades room synchronisation, not otherwise
healthy desk check-in. A failed notification consumer cannot stall allocation.

Track core success/latency, allocation publication conflicts, unmet demand, timer lag,
room-pending age, Exchange freshness and notification backlog. Export
OpenTelemetry traces/metrics and JSON logs through the Collector. Probe liveness,
readiness and dependency degradation deliberately rather than restarting healthy
API pods for a provider outage. [Kubernetes probes][probes].

Single-region writers, multi-zone HA and warm regional recovery are unchanged
by the context map. Fence previous database and external-effect workers before
promotion; exercise database/blob/identity/routing/worker recovery together.
RPO/RTO and one-second p95/99.95% targets require measured validation. If regional
replication exceeds the five-minute budget with a healthy primary, keep serving,
alert and require explicit operator choice for failover outside that budget.

### Composition root

`app/contexts.go` constructs Identity once with distinct People/Access/Delegation
interfaces, then Workplace/Policy/Plans and Booking with its private solver.
Events receives Booking's venue participant and Visitors' visit interface.
Exchange receives Booking's room interface and owns provider reconciliation;
Identity's native-authority adapter independently verifies delegation evidence.
Notifications/reporting subscribe to committed outcomes.

Potential runtime cycles are addressed by consumer-owned impact-check contracts
or composition-owned coordinators, not pervasive late setters. Wiring exposes
where a real dependency exists. The shared transaction coordinator is explicitly
named and limited; it is not placed in a hidden service locator or carried as an
unrestricted database handle inside every command.

### Generated code and API specifications

```text
spaceweave/
├── api/src/app/
│   ├── identity/                            workplace/ policy/ booking/ exchange/ plans/
│   └── visitors/                            events/ attendance/ notifications/
├── api/src/provisioning/                    SCIM Users/Groups wire contract and Entra profile
├── api/
│   └── generated/
│       └── app.openapi.yaml                 bundled application transport contract
├── internal/shared/generated/app/           generated Go transport contracts
├── internal/shared/generated/provisioning/
├── web/shared/generated/                    generated TypeScript transport types/clients
└── web/shared/renderer/                     lifecycle adapter around current D3/SVG renderer
```

Each context's HTTP adapter implements its operations. Composition dispatches
the generated route contracts without importing generated types into `domain`.
The exact generator/version is an implementation choice to validate against
Chi and required schema features. SCIM remains its standard protocol, not an
ad hoc app endpoint merely renamed as SCIM. Frontend mobile-web reuses staff
capabilities with a suitable layout; no native-mobile backend is implied.

### Tests and enforcement

Domain tests exercise meaningful transitions and planner properties. Application
and adapter acceptance tests use real PostgreSQL via the existing Docker/
testcontainers convention for overlap, pool capacity, leases and transactional
outbox semantics. In-memory objects can explain pure decisions but are not proof
of concurrency correctness. Retain the existing 95% total/90% package gates.

Context repository tests cover their own tables and migrations; named coordinator
tests assemble both context implementations in one transaction. End-to-end tests
cross published interfaces, including source deactivation with an existing
session, venue reduction racing ticket issuance, check-in racing release, and
group allocation with linked parking under changed inventory. Provider contract
tests and authorised environment trials cover Exchange and personal Teams
behaviour that a local fake cannot establish.

Dependency checks reject foreign context implementation imports and adapter
imports in domain/application. SQL ownership checks review both migrations and
queries. Generation is reproducible. Rolling releases use one controlled
migration step, expand/contract schemas and compatible job/fact versions; older
workers must drain before compatibility is removed.

### Workplace retirement, reactivation and deletion

`internal/workplace/application/retirement.go` handles explicit retirement and dependency-checked removal
through the existing owning interfaces. Retirement is immediate or scheduled at an
office-local date/time. Publishing the schedule immediately excludes new bookings
for use on/after that point, including descendant spaces; confirmed commitments
remain readable and manageable. Booking compares requested use with the published
lifecycle revision at claim commit. A stale snapshot cannot bypass the schedule.
Authorised reactivation revalidates definitions, calendars and dated inventory,
preserving the same identity, commitments and history. Pre-existing unconfirmed
holds/offers revalidate at confirmation and cannot confirm excluded use. Return
a clear unavailable outcome and reconcile late Exchange side effects; existing
confirmations remain protected. Deletion reports blocking bookings,
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
choose a repository architecture.

Shared operational facilities retain confirmed outage declarations, including
affected offices/actions/times and revisions. Booking and Events consume narrow
private incident reads and own their respective protected deadline state; Policy
owns configurable rules. Declaration publication is ordered against expiry
through the guarded read/transition protocol, rather than relying on delayed
notifications. Job execution supplies mechanics without choosing business expiry.
The [incident contract](../api-surfaces/operational-incidents.md) also preserves
incident-scoped request-recovery extensions. These owners apply independently of
repository style.

### Exchange-led room policy, check-in and leavers

Booking claim series/occurrence values and application room handlers retain local intention/consistency. The Exchange context owns native policy/check-in integration and correspondence through its published interfaces; domain models consume neutral outcomes, not provider SDK objects.

Exchange's supported resource policies govern room acceptance, recurrence,
principal/delegate admission, native check-in and autorelease for both Outlook
and SpaceWeave requests. App RBAC protects application operations; it does not
replace native resource admission with a stricter actual-author-only or
recurring-only rule. Unsupported requested granularity remains a capability gap.
Native recurrence/conflict outcomes must remain representable; a local conflict
is not a native decline. Any room-specific neighbourhood priority must also be
expressible through supported native policy rather than an app-only gate.

Room occurrences include one-off bookings and series instances/exceptions.
SpaceWeave can adapt its check-in surface to the documented native create/read
contracts using the required delegated user context and native eligibility.
An app-only worker cannot impersonate that context. Keep request acceptance,
native result and local observation progress distinct; a missing claim read is
not proof of non-use, and native claims do not identify a named person.

The effective native grace governs autorelease. Apply observed resource outcomes
through Booking and promise availability only from current native/local state;
do not add a second expiry timer or pause Exchange using an app-local incident.
Exact invitation/future-series effects must match validated native behaviour.
The previously proposed consecutive-miss series cancellation is unverified as a
native feature; no independent application counter/canceller is selected.

A confirmed effective leaver still triggers supported Exchange cancellation of
organiser-owned room meetings/series, alongside personal desk/parking cleanup.
Other-owner meetings created by a departing PA remain. Disable, SCIM deletion
and incomplete imports do not establish departure; Events retains handover.
Cancellation fences and bounded progress prevent recurrence expansion or stale
provider observations from recreating cancelled local commitments.

HTTP roles accept requests; Exchange roles reconcile supported provider work and
Booking handlers apply committed outcomes. Native check-in reads require the
supported user context rather than an assumed app-only polling permission.
Observe native-policy freshness, pending outcomes and cleanup completion.

The [native-policy research](../api-surfaces/outlook-room-policy-enforcement.md)
and [room-use/leaver contract](../api-surfaces/recurring-rooms-and-leavers.md)
record source facts, permission boundaries and delivery gaps. Physical
unavailability inputs/declarations remain unselected.

## Walkthroughs

### Publish the completed allocation plan while capacity changes

1. A durable cycle job calls `booking/application/plan_cycle.go` for the relevant
   office/date cohort and records its input snapshot and generation.
2. Identity/Workplace/Policy views and Booking-owned inventory supply versioned
   planning facts. No database transaction spans grouping or packing.
3. `domain/allocation/planner.go` first forms collaborator groups, including
   singletons, then packs those groups into existing desks while retaining
   member-specific requirements, relationships and required parking.
4. Proposals remain movable. `publish_plan.go` validates the complete plan against
   current authority, intent, policy, definitions and dated inventory dependencies.
5. Booking guards arbitrate concurrent changes and cancellation; stale provisional
   work requires repair before publication. Older published bookings remain fixed.
6. Publish the completed plan, required parking and durable outcome/follow-up work
   consistently. No early group becomes confirmed; publication/cancellation have
   a guarded ordering and retries recover the same published result.
7. Separate consumers notify and report published outcomes and unmet demand.

### Race check-in, release and waitlist promotion

1. The owner's QR or portal request reaches Identity and Booking's check-in handler.
2. A no-show timer concurrently calls `release.go` for that allocation/version.
3. Both use the current Booking transition and protected-incident policy. The
   committed state decides whether the check-in is valid or the desk was released.
4. An anonymous occupancy observation never marks the allocation taken or blocks
   return to the pool. A confirmed service incident pauses affected expiry.
5. Automatic desk release leaves linked parking booked until its own end or
   explicit release. Append desk-capacity work; waitlist promotion uses only
   actually released capacity before exposing it to fresh direct requests.
6. A stale timer or late check-in cannot take a desk from its new confirmed owner.

### A PA books a room and Outlook changes it

1. `identity/application/decide.go` authorises actor, owner, action and office using
   a sufficiently fresh source-labelled grant or current app grant.
2. Booking commits a pending room hold and desired intention with owner/actor.
3. Exchange's worker performs the provider action in the owner's calendar; the
   event response alone is not room acceptance.
4. Acceptance and an external Outlook edit arrive through callbacks/delta. Store
   observations and compare with durable base/desired state.
5. Merge compatible edits; revalidate capacity/time changes through Booking.
   Late/stale updates cannot resurrect a cancelled or expired local claim.
   Failed replacement after old-room loss retains meeting/attendees with no
   secured room; notify the organiser and offer replacements or cancellation.
   A selected replacement still requires current acceptance/capacity validation.
6. A later native-grant revocation removes only that source; a separate app grant
   can still authorise future app actions. Irreconcilable intent becomes visible
   resolution work, not a blanket error on every edit.

### Reduce an event venue while a visitor registers

1. The venue change and registration enter Events through independently
   authorised commands. Registration obtains/reuses the visitor's hosted visit.
2. `venue_coordinator.go` reads the active issued-ticket count and asks Booking's
   restricted venue interface to validate secured replacement time/capacity.
3. Serialize the event registration/venue guard with ticket issuance. If the
   smaller venue cannot hold issued tickets, reject/route the proposed change
   instead of committing an impossible event capacity.
4. The accepted venue claim and event capacity commit in the named cross-context
   transaction; registration issues a named ticket only with its valid visit link.
5. A scan records admission once. Repeated scans return the existing admission
   outcome; reception arrival remains a separate visit action.

### Apply lifecycle updates during a source outage

1. Authenticated SCIM disable reaches Identity; retain its source meaning and
   commit effective ineligibility and session/access invalidation together in Identity.
2. An existing staff session fails the next eligibility check; asynchronous
   cleanup handles retained sessions without being the sole enforcement.
3. Workday supplies an effective office/team change. Apply only its owned fields
   at the relevant date, preserving explicit travel and confirmed bookings.
4. A failing/partial source response does not delete known staff or reactivate
   another source's disabled eligibility input.
5. If the effective event is a confirmed leaver, Booking releases future personal
   desk/parking and cancels organiser-owned room meetings/series through Exchange.
   A departing PA's other-owner meetings remain. Events gets its own handover.
   Cancellation fences block later occurrence expansion or stale acceptance.

### Add a private booking-raised Teams message

1. Define the narrow notification purpose and individual message opt-in in
   Notifications; do not alter Booking's confirmation rule.
2. Add `notifications/adapters/teams/personal.go` and installation/conversation
   handling for the supported proactive personal messaging interface.
3. `on_booking_raised.go` derives the intended recipient and checks opt-in for
   that message; installation never creates consent.
4. Enqueue delivery after commit. Recheck opt-in before sending and record the
   outcome. Blocked/uninstalled errors are visible; transient failures retry.
5. Keep in-app/email behaviour according to their preferences and avoid duplicate
   fallback emails. No shared Teams channel or unrelated Teams alerts are added.

## Package mapping

The source repository currently implements floor-plan processing rather than
the proposed business contexts. New booking/access/directory/event tables and
provider integrations are new work, not moves of existing implementations.

| Today's path | Proposed destination or treatment | Reason |
| --- | --- | --- |
| `cmd/spaceweave` | Retain; delegate to `internal/app`. | Keep the existing executable and CLI semantics. |
| `internal/cli` | Retain during adoption; composition belongs in `app`. | Cobra owns flags/commands, not context construction rules. |
| `internal/config` | Retain initially, then technical `shared/config` if useful. | Deployment YAML is distinct from Policy's runtime records. |
| `internal/server/server.go` | `shared/httpkit`, `app/routes.go` and `plans/adapters/http`. | Separate HTTP assembly from existing plan operations. |
| `internal/server/profiles.go` | `plans/adapters/http` plus Plans application. | Layer-profile operations remain part of floor-plan review. |
| `internal/server/files.go` | Plans multipart attachment/detach HTTP adapter and application operations. | Preserve attachment validation; generated-directory static serving is in `server.go`. |
| `internal/catalog` | Legacy adapter during transition; durable metadata in `plans/adapters/postgres`. | Local catalogue locks/renames do not establish distributed booking correctness. |
| `internal/inbox` | Plans upload/intake adapter. | Intake stages work, not authoritative booking capacity. |
| `internal/worker` | Plans application/tracing adapter plus shared durable execution. | Keep trace behaviour; replace local queue assumptions for replicas. |
| `internal/autotrace` including profiles/evaluation | Retain behind Plans tracing adapter. | Existing deterministic algorithms and quality fixtures remain valuable. |
| `internal/cad`, `internal/dwg`, `internal/dxf` | Retain geometry/readers. | Context organisation does not require rewriting format readers. |
| `internal/convert`, `internal/raster`, `internal/svgdraw`, `internal/vector` | Retain rendering/conversion packages behind Plans. | Their actual variation is technical format/geometry behaviour. |
| `internal/floorgraph`, `internal/plan`, `internal/trace` | Retain behind Plans while separating revision metadata from geometry. | Do not turn a traced room polygon into an authoritative Booking entity. |
| `internal/fakeclaude` | Retain existing test support for the optional tracer. | No new cloud/provider assumption for deterministic tracing. |
| `internal/buildinfo` | Retain or technical shared location. | Build metadata has no domain owner. |
| `web/floorplan-iso` | Retain; add lifecycle adapter from `web/shared/renderer`. | React adopts the renderer incrementally. |
| `test/acceptance` | Retain and extend with full application assembly. | Preserve existing regression evidence. |
| `pkg/README.md` | Retain the deliberate public/private convention. | Publish deliberate consumer contracts while keeping domain internals private. |

## Numbers

These are properties of the printed candidate, not forecasts of code size,
staffing or runtime. Counts describe the representative tree in this document.

| Measure | Candidate value |
| --- | --- |
| Business contexts | 10, named in the full tree. |
| Uniform context layer directories | 40: `api`, `domain`, `application`, `adapters` for each context, before their internal subdivisions. |
| Explicit Go-bearing package paths | 112: one per parent directory with a named `.go` file in the full tree. |
| Explicit representative Go filenames | 205, including generated examples; co-located tests and unprinted retained engine files are excluded. |
| Shared-kernel packages shown | 4: IDs, principal envelope, interval values and clock. |
| Explicit booking multi-aggregate operations | Group placement, required linked claim, pre-start both-required pair cancellation, explicitly scoped post-start cancellation and office transfer. |
| Explicit cross-context transaction families | Event venue/capacity through Events and Booking; registration-created visit/ticket through Events and Visitors. |
| Runtime role families | 5: HTTP, allocation, Exchange, tracing and maintenance. |
| Blob provider implementations | 2: Azure and GCS, selected per deployment. |
| Notification purposes allowed in Teams | 1: private booking-raised message with individual opt-in for that message. |

The explicit counts are mechanically derived from filenames and their parent
paths in the Full package tree, excluding later repeated examples. They describe
this printed destination, not today's code or the number of packages to create
upfront. Retained CAD packages have existing files/subpackages not expanded in
the tree. Uniform layering still has a structural cost before implementations
become large. Count actual packages with `go list` after a layout is implemented.

## Placement choices and friction

- **Booking is one bounded context across space kinds.** This keeps local
  availability under one writer and required desk/parking close. A desk context
  and parking context with ordinary compensating messages would change the
  agreed atomic confirmation behaviour.
- **Aggregate semantics are explicit, but not a magic concurrency solution.**
  A request-set root cannot exclude overlapping claims in another root by itself.
  SQL capacity/staff guards and named transaction exceptions remain essential.
- **Events and Booking retain distinct language.** Admission/registration is
  different from venue occupancy. Their shared transaction is real coupling;
  extracting either context to a separate database would require a new protocol.
- **Provider conversations have their own context.** Graph payloads, acceptance
  and cursor recovery do not contaminate the Booking model. The price is durable
  translation/mapping and two models to inspect when diagnosing room changes.
- **Uniform layering adds thin packages.** Small Visitors/Policy/Notifications
  implementations still receive four layers. This differs from option 5, where
  only substantial implementations need an inner architecture.
- **The shared kernel can grow into a hidden global model.** Keep it to stable
  value semantics; Staff, Team, Booking and Event behaviour belong to contexts.
- **Published facts cost operational work.** Schema evolution, replay,
  idempotency, retention and stuck-consumer recovery are implementation obligations,
  even while all contexts ship together.
- **Context ownership helps later extraction but does not promise it.** Shared
  transactions, PostgreSQL guards, identity checks and the release train are
  intentional constraints, not independent services waiting to be deployed.
- **Multi-aggregate transactions create deliberate coupling.** The named
  exceptions require shared locking, migration and release compatibility. Hiding
  them behind an aggregate called Organisation would weaken the comparison.

This option is strongest when a team wants explicit modelling/context practices
throughout the application and will maintain that vocabulary and discipline.
Its extra ceremony earns value only where models/behaviour are substantial.
It has no proven throughput advantage over the other four options; the same
contention, provider and whole-system recovery trials are required.

## Sources

- [Evans's DDD reference][evans] and [Vernon's aggregate-design paper][vernon]
  establish the modelling concepts; proposed SpaceWeave contexts/aggregates and
  the transaction exceptions are this document's design choices.
- [Current server][server], [catalogue][catalogue], [workers][worker] and
  [tracing pipeline][autotrace] establish existing code responsibilities.
- The [shared requirements](README.md), [glossary][glossary],
  [defaults][defaults] and [verified constraints][architecture] supply agreed
  product behaviour and the fuller provider-source trail.

[evans]: https://www.domainlanguage.com/wp-content/uploads/2016/05/DDD_Reference_2015-03.pdf
[vernon]: https://www.dddcommunity.org/wp-content/uploads/files/pdf_articles/Vernon_2011_1.pdf
[go-imports]: https://pkg.go.dev/cmd/go#hdr-Internal_Directories
[pg-ranges]: https://www.postgresql.org/docs/current/rangetypes.html#RANGETYPES-CONSTRAINT
[pg-locks]: https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS
[pg-retry]: https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html
[graph-event]: https://learn.microsoft.com/en-us/graph/api/resources/event?view=graph-rest-1.0
[graph-delta]: https://learn.microsoft.com/en-us/graph/delta-query-events
[room-policy]: https://learn.microsoft.com/en-us/powershell/module/exchangepowershell/set-calendarprocessing?view=exchange-ps
[exchange-rbac]: https://learn.microsoft.com/en-us/exchange/permissions-exo/application-rbac
[scim-delete]: https://www.rfc-editor.org/rfc/rfc7644.html#section-3.6
[entra-scim]: https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups
[workday]: https://learn.microsoft.com/en-us/entra/identity/app-provisioning/workday-integration-reference
[azure-blob]: https://learn.microsoft.com/en-us/azure/storage/blobs/concurrency-manage
[gcs-blob]: https://docs.cloud.google.com/storage/docs/request-preconditions
[teams]: https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/conversations/send-proactive-messages
[probes]: https://kubernetes.io/docs/concepts/configuration/liveness-readiness-startup-probes/
[conventions]: ../../../CLAUDE.md
[server]: ../../../internal/server/server.go
[catalogue]: ../../../internal/catalog/catalog.go
[worker]: ../../../internal/worker/worker.go
[autotrace]: ../../../internal/autotrace/autotrace.go
[glossary]: ../../../GLOSSARY.md
[defaults]: ../../policy-defaults.md
[architecture]: ../../architecture.md
