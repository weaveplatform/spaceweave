# Option 3: Clean architecture

The repository-architecture decision is pending. This candidate expresses the
same agreed logical owners as the other four: combined Identity; attendance
planning and claim commitment inside Booking; separate Exchange and Events.
Its package organisation and dependency rules remain specific to this style.
[Identity](../../adr/0026-combine-workforce-identity-and-access-ownership.md),
[Booking planning](../../adr/0027-keep-attendance-planning-inside-booking.md) and
[Exchange](../../adr/0028-separate-exchange-synchronisation-from-booking.md)
record those ownership decisions independently of repository style.

SpaceWeave coordinates workplace bookings, advance allocation and enterprise
integrations for approximately 100,000 staff and 60,000 desks. This candidate
organises the backend around dependency direction: business policies remain
independent of HTTP, persistence, cloud storage and provider SDKs.

[Martin's original Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
defines the inward dependency rule and simple data crossing boundaries. The four
rings, operation-family packages and strict peer-import policy in this document
are candidate implementation choices. Their purpose and costs are evaluated
against the [common requirements](README.md#requirements-held-constant) and
[architecture foundations](architecture-foundations.md).

The package trees describe a proposed destination and identify current code where
relevant. The comparison includes ownership, transactions, generation, operations,
workflow walkthroughs and migration implications.

## Summary

The primary organising rule is dependency direction. Pure entities express
booking, eligibility, evidence and capacity rules. Operation-oriented use cases
coordinate those rules through ports they declare. Adapters translate HTTP,
SQL, provider APIs and drawing engines. Infrastructure opens drivers, configures
frameworks and runs processes. `internal/app` constructs the graph.
This follows the inward dependency rule and simple data crossing boundaries in
[Martin's original account](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html).

This candidate is not a set of self-contained domain modules with their own
adapters. Booking entities, booking orchestration, booking SQL and booking HTTP
are four separate locations. The rings alone do not establish data ownership;
an explicit repository ownership manifest and composition restrictions do that.
Narrowly constructed repository handles preserve one local booking authority.
A concrete SQL adapter may satisfy several interfaces, but constructor wiring
must prevent use cases from receiving a general unrestricted store.

One `usecase/booking` interactor family owns every dated physical-space claim mutation.
Identity lifecycle and Exchange workflows invoke it through their own consumed
ports; allocation is private Booking behaviour.
Pure `entity/claim` rules are shared without importing booking orchestration.
Events' `manageevent` family separately owns registration, tickets and admission.
Its venue workflow coordinates Booking's published participant in one local
transaction through consumer-owned ports; neither family receives foreign SQL.
It avoids copying claim logic into twenty small interactors while preserving
this candidate's prohibition on use-case implementation imports.

The result is one Go release/image with independently scaled HTTP, allocation,
Exchange, tracing and maintenance roles. PostgreSQL remains authoritative;
React staff/mobile-web and admin portals are separate entrypoints.
There is one organisation across offices, targeting 100,000 staff and 60,000
desks. Core booking/check-in targets are 99.95% monthly availability and a
one-second p95 initial response at the agreed representative workload.
Asynchronous allocation completion and room acceptance are measured separately.
Attendance remains reporting. Neither this layout nor its package count proves
those targets or the regional recovery targets.

## Dependency and ownership rules

1. `internal/entity/...` imports no other ring and no database, HTTP, filesystem,
   SDK or telemetry package. `time`, pure collections and deterministic geometry
   are acceptable. Clocks and source observations arrive as values.
2. `internal/usecase/...` imports entities and only three allowlisted peer
   contract packages: `unitofwork`, `event` and `job`. No operation package
   imports another operation package, even for constructors or input structs.
3. `internal/adapter/...` imports entities and use cases, never infrastructure
   or app. Providers, SQL, inbound protocols and generated transport live here.
4. `internal/infrastructure/...` may import inward rings, never app.
   Router assembly knows controller constructors; controllers do not know the
   concrete router, server lifecycle or deployment configuration.
5. `internal/app` is the sole internal composition root. `cmd/spaceweave` calls
   it and Cobra wiring; it does not assemble a second production object graph.
6. `pkg/...`, if introduced for generated clients, imports no internal package.
   Domain entities and use-case contracts remain private to this application.
7. Each operation owns its consumed interfaces in `port.go`. Structurally
   identical interfaces may repeat; common transaction/event/job contracts are
   the only ring-2 exceptions. Their packages contain contracts, not workflows.
8. Only the booking interactor receives booking mutation repositories. No SQL
   adapter, controller or event consumer makes its own claim decision.
   Reporting may read projections; a projection never confirms availability.
9. SQL groups own tables and migrations through an explicit owner manifest.
   Imports and constructor injection are checked in addition to ring direction;
   the rings alone would permit another use case to declare a booking writer.

The strict peer-import rule is a local dependency policy. Clean Architecture
requires inward dependencies without prescribing this exact package graph.
Passing a Booking room interface into Exchange's consumer-owned observation port
is permitted: the source dependency still points to
entities/contracts and app performs the assignment. Importing `usecase/booking`
from `usecase/exchangesync` would violate the candidate even if described as reuse.

## Repository root

```text
spaceweave/
├── cmd/
│   └── spaceweave/
│       └── main.go       executable; selects Cobra command/runtime role
├── internal/
│   ├── entity/           ring 1: pure models, decisions and invariants
│   ├── usecase/          ring 2: operation families plus three contracts
│   ├── adapter/          ring 3: controllers, repositories, providers, engines
│   ├── infrastructure/   ring 4: frameworks, drivers, process lifecycle
│   ├── app/              explicit construction and event/job registrations
│   ├── layout/           dependency, ownership and wiring checks
│   └── devtool/          OpenAPI generation/compatibility tooling
├── api/
│   ├── src/
│   │   ├── staff.yaml    staff/mobile-web transport contract
│   │   ├── admin.yaml    office/global administration transport contract
│   │   └── visitor.yaml  visit/event restricted-access contract
│   └── config/           pinned generation configurations
├── web/
│   ├── staff/            React/TypeScript staff/mobile-web entrypoint
│   ├── admin/            React/TypeScript admin entrypoint
│   ├── packages/         generated API client, UI and renderer bridge
│   └── floorplan-iso/    existing renderer/fixtures during migration
├── test/                 contracts, integration, acceptance, platform, load
├── deploy/               one image; role deployments and recovery overlays
├── tools/                reproducible generation and quality commands
├── docs/                 decisions, policy, ownership and operator guides
├── Makefile              GOWORK=off, generation, existing quality gate
├── .golangci.yml         existing linter; depguard mirrors ring rules
├── .testcoverage.yml     existing 95% total/90% package coverage gate
└── go.mod                current module path; dependencies evolve deliberately
```

`api/`, production portals, PostgreSQL adapters and Kubernetes manifests are new
work. Their appearance here is not a claim that they exist. Today's
[server](../../../internal/server/server.go),
[catalogue](../../../internal/catalog/catalog.go) and
[worker](../../../internal/worker/worker.go) implement a drawing microsite using
local files and polling, not the future distributed booking application.

## Full package tree

These four trees are the counted Go inventory. Each directory with named `.go`
files is one package; parent grouping directories are not packages. File names
show responsibilities rather than a mandatory one-type-per-file convention.
Only explicitly named files count; existing drawing-engine files are represented
by named package facades here, not silently counted as their complete inventory.
Later deep dives expand selected packages and are not counted a second time.

### Ring 1: entities

```text
internal/entity/
├── calendar/             office dates/timezones, working windows, protected deadlines
│   ├── calendar.go
│   ├── interval.go
│   └── deadline.go
├── staff/                stable staff IDs, verified links, source/effective observations
│   ├── staff.go
│   ├── source.go
│   └── lifecycle.go
├── authority/            capability + office/global decisions; visitor restrictions
│   ├── principal.go
│   ├── capability.go
│   └── scope.go
├── workplace/            office/building/floor definitions and baseline bundles
│   ├── location.go
│   ├── space.go
│   └── neighbourhood.go
├── claim/                local claims, required links, legal transitions, capacity rules
│   ├── claim.go
│   ├── eligibility.go
│   ├── transition.go
│   └── capacity.go
├── intent/               staff choices, explicit travel, cutoff and collaboration priority
│   ├── intent.go
│   ├── cycle.go
│   └── collaboration.go
├── allocation/           per-date groups, snapshot versions, lexicographic objectives
│   ├── proposal.go
│   ├── group.go
│   └── objective.go
├── attendance/           personal check-in, anonymous occupancy and office evidence
│   ├── evidence.go
│   ├── checkin.go
│   └── vacancy.go
├── delegation/           native/app grant sources, actions and freshness decisions
│   ├── grant.go
│   ├── provenance.go
│   └── freshness.go
├── exchange/             provider-neutral sync identity, baseline and compatible changes
│   ├── mapping.go
│   ├── observation.go
│   └── merge.go
├── visit/                visitor, named host, visit window and restricted access
│   ├── visit.go
│   └── access.go
├── event/                audience, registration, named entitlement and admission rules
│   ├── event.go
│   ├── ticket.go
│   └── admission.go
├── drawing/              immutable drawing/render manifests and publication state
│   ├── revision.go
│   ├── asset.go
│   └── review.go
├── policy/               published operational versions, allowed office overrides
│   ├── version.go
│   └── validate.go
├── notification/         message purposes, recipients and purpose/channel preferences
│   ├── message.go
│   └── preference.go
├── retention/            independent data categories, expiry and deletion evidence
│   ├── policy.go
│   └── record.go
├── incident/             confirmed operational scope/times/revision; no deadline policy
│   └── incident.go
└── work/                 durable job/event identities, attempt and fencing values
    ├── job.go
    ├── envelope.go
    └── lease.go
```

### Ring 2: operation families and shared contracts

```text
internal/usecase/
├── unitofwork/           transaction scope contract; no SQL transaction type
│   └── unitofwork.go
├── event/                append an entity/work event in the active transaction
│   └── appender.go
├── job/                  enqueue durable work in the active transaction
│   └── enqueuer.go
├── booking/              demand/planning/claim owner; solver through own consumed port
│   ├── port.go
│   ├── command.go
│   ├── commit.go
│   ├── direct.go
│   ├── transfer.go
│   ├── release.go          pre-start pair cancellation; timed desk-only release
│   ├── checkin.go
│   ├── offer.go
│   ├── room.go
│   ├── venue.go
│   ├── adjustment.go
│   ├── intent.go
│   ├── cycle.go
│   ├── snapshot.go
│   ├── plan.go
│   ├── run.go
│   ├── repair.go
│   ├── evidence.go
│   └── protection.go
├── availability/         eligible search and authoritative reads; no promises from cache
│   ├── port.go
│   └── query.go
├── manageworkplace/      definitions/baseline bundles; dated inventory via booking port
│   ├── port.go
│   ├── location.go
│   ├── space.go
│   ├── neighbourhood.go
│   ├── retirement.go
│   └── mapping.go
├── managepolicy/         validate and publish versions; explicit migration of old claims
│   ├── port.go
│   ├── publish.go
│   └── migrate.go
├── recordoccupancy/      translate source input; Booking validates, deduplicates and stores evidence
│   ├── port.go
│   ├── ingest.go
│   └── evidence.go
├── outage/               operational declaration lifecycle; owners apply deadline consequences
│   ├── port.go
│   ├── confirm.go
│   └── resume.go
├── roomrequest/          staff/PA authorisation; local pending request via booking port
│   ├── port.go
│   ├── request.go
│   ├── change.go
│   └── cancel.go
├── exchangesync/         provider commands, observations, compatible merge and conflicts
│   ├── port.go
│   ├── outbound.go
│   ├── inbound.go
│   ├── reconcile.go
│   └── subscription.go
├── identity/             source identity, sessions, access/delegation under one owner
│   ├── port.go
│   ├── access.go
│   ├── grant.go
│   ├── refresh.go
│   ├── revoke_grants.go
│   ├── callback.go
│   ├── session.go
│   ├── visitor.go
│   ├── user.go
│   ├── group.go
│   ├── delete.go
│   ├── import.go
│   ├── effective.go
│   ├── reconcile.go
│   ├── apply.go
│   ├── revoke_access.go
│   └── handover.go
├── managevisit/          host/window checks and reception operations
│   ├── port.go
│   ├── invite.go
│   ├── arrival.go
│   └── departure.go
├── manageevent/          Events owns registration/tickets/admission; Booking venue participant
│   ├── port.go
│   ├── publish.go
│   ├── edit.go
│   ├── register.go
│   ├── venue.go
│   ├── issue.go
│   ├── revoke.go
│   ├── admit.go
│   └── protection.go
├── notify/               recipients, message-specific consent and bounded delivery
│   ├── port.go
│   ├── compose.go
│   ├── deliver.go
│   └── preference.go
├── drawingupload/        accept source packages, checksums and recoverable intake
│   ├── port.go
│   ├── upload.go
│   └── attach.go
├── drawingprocess/       engine orchestration, validation, immutable draft artifacts
│   ├── port.go
│   ├── convert.go
│   ├── trace.go
│   └── publishdraft.go
├── drawingreview/        reject/retrace, approve immutable assets and advance asset pointers
│   ├── port.go
│   ├── review.go
│   └── approve.go
├── report/               scoped evidence-labelled reports; retention-aware exports
│   ├── port.go
│   ├── attendance.go
│   ├── utilisation.go
│   └── export.go
├── retain/               category policies and copy-aware deletion workflows
│   ├── port.go
│   ├── schedule.go
│   ├── erase.go
│   └── reconcile.go
└── operatejobs/          privileged retry/park/cancel; never alter booking tables directly
    ├── port.go
    ├── inspect.go
    └── retry.go
```

`booking` is deliberately an operation family, not a forwarding package for
separate `desk`, `parking` and `event` implementations. Its private functions
share one mutation protocol. Other use cases have different reasons to change:
external synchronisation, source lifecycle and messaging.
Their files contain orchestration, not interchangeable layers of the same service. Identity combines
source lifecycle, access/session denial and delegation in one operation family;
Booking combines intent/planning and claim work. Exchange remains a separate
operation owner that applies provider observations through its consumed Booking port.

### Ring 3: adapters

```text
internal/adapter/
├── http/
│   ├── controller/
│   │   ├── staff/
│   │   │   ├── controller.go
│   │   │   ├── booking.go
│   │   │   ├── intent.go
│   │   │   ├── rooms.go
│   │   │   └── events.go
│   │   ├── admin/
│   │   │   ├── controller.go
│   │   │   ├── workplace.go
│   │   │   ├── policy.go
│   │   │   ├── visitors.go
│   │   │   ├── reports.go
│   │   │   └── operations.go
│   │   └── visitor/
│   │       ├── controller.go
│   │       ├── visit.go
│   │       ├── registration.go
│   │       └── ticket.go
│   ├── gen/
│   │   ├── staff/
│   │   │   └── server.gen.go
│   │   ├── admin/
│   │   │   └── server.gen.go
│   │   └── visitor/
│   │       └── server.gen.go
│   ├── scim/
│   │   ├── routes.go
│   │   ├── users.go
│   │   ├── groups.go
│   │   ├── filter.go
│   │   └── presenter.go
│   ├── occupancy/
│   │   └── routes.go
│   ├── exchangehook/
│   │   ├── validation.go
│   │   ├── notification.go
│   │   └── lifecycle.go
│   ├── teamshook/
│   │   ├── activity.go
│   │   └── installation.go
│   ├── session/
│   │   ├── cookie.go
│   │   ├── csrf.go
│   │   └── redirect.go
│   ├── middleware/
│   │   ├── authenticate.go
│   │   ├── authorise.go
│   │   └── request.go
│   └── presenter/
│       ├── error.go
│       ├── booking.go
│       └── evidence.go
├── cli/
│   ├── root.go
│   ├── serve.go
│   ├── workers.go
│   ├── migrate.go
│   └── plans.go
├── jobrunner/
│   ├── runner.go
│   ├── claim.go
│   ├── dispatch.go
│   └── heartbeat.go
├── eventloop/
│   ├── relay.go
│   ├── registry.go
│   └── delivery.go
├── repository/postgres/
│   ├── tx/
│   │   ├── scope.go
│   │   ├── transaction.go
│   │   └── retry.go
│   ├── booking/
│   │   ├── repository.go
│   │   ├── locks.go
│   │   ├── claims.go
│   │   ├── capacity.go
│   │   ├── transitions.go
│   │   └── evidence.go
│   ├── workplace/
│   │   ├── repository.go
│   │   ├── definitions.go
│   │   ├── snapshots.go
│   │   └── mappings.go
│   ├── identity/
│   │   ├── repository.go
│   │   ├── sources.go
│   │   ├── membership.go
│   │   ├── sessions.go
│   │   ├── grants.go
│   │   └── verification.go
│   ├── exchange/
│   │   ├── repository.go
│   │   ├── mappings.go
│   │   ├── cursors.go
│   │   └── conflicts.go
│   ├── visits/
│   │   ├── repository.go
│   │   ├── visits.go
│   │   └── access.go
│   ├── events/
│   │   ├── repository.go
│   │   ├── events.go
│   │   ├── tickets.go
│   │   └── admission.go
│   ├── drawings/
│   │   ├── repository.go
│   │   ├── revisions.go
│   │   └── manifests.go
│   ├── policies/
│   │   ├── repository.go
│   │   └── versions.go
│   ├── incidents/
│   │   └── incidents.go
│   ├── notifications/
│   │   ├── repository.go
│   │   ├── preferences.go
│   │   └── deliveries.go
│   ├── reports/
│   │   ├── repository.go
│   │   ├── projections.go
│   │   └── exports.go
│   ├── retention/
│   │   ├── repository.go
│   │   ├── copies.go
│   │   └── tombstones.go
│   └── queue/
│       ├── repository.go
│       ├── jobs.go
│       ├── outbox.go
│       └── consumers.go
├── gateway/
│   ├── solver/
│   │   └── solver.go
│   ├── blob/
│   │   ├── azure/
│   │   │   ├── store.go
│   │   │   ├── access.go
│   │   │   └── recovery.go
│   │   └── gcs/
│   │       ├── store.go
│   │       ├── access.go
│   │       └── recovery.go
│   ├── recovery/
│   │   ├── copy.go
│   │   ├── verify.go
│   │   └── manifest.go
│   ├── entra/
│   │   ├── oidc.go
│   │   ├── tokens.go
│   │   └── keys.go
│   ├── exchange/
│   │   ├── calendar.go
│   │   ├── delegation.go
│   │   ├── delta.go
│   │   └── subscription.go
│   ├── workday/
│   │   ├── workers.go
│   │   ├── organisations.go
│   │   └── mapping.go
│   ├── teams/
│   │   ├── personal.go
│   │   ├── conversation.go
│   │   └── installation.go
│   ├── email/
│   │   ├── sender.go
│   │   └── verification.go
│   ├── conversion/
│   │   ├── convert.go
│   │   └── sniff.go
│   ├── claude/
│   │   ├── agent.go
│   │   ├── tools.go
│   │   └── limits.go
│   ├── validator/
│   │   └── node.go
│   └── inbox/
│       └── watcher.go
└── drawingengine/
    ├── autotrace/
    │   ├── autotrace.go
    │   ├── cadfront/
    │   │   └── cadfront.go
    │   ├── rasterfront/
    │   │   └── rasterfront.go
    │   ├── profile/
    │   │   └── profile.go
    │   ├── score/
    │   │   └── score.go
    │   └── synth/
    │       └── synth.go
    ├── cad/
    │   └── cad.go
    ├── dxf/
    │   ├── dxf.go
    │   └── dxftest/
    │       └── dxftest.go
    ├── dwg/
    │   ├── dwg.go
    │   └── dwgtest/
    │       └── dwgtest.go
    ├── floorgraph/
    │   └── graph.go
    ├── plan/
    │   └── plan.go
    ├── raster/
    │   └── raster.go
    ├── svgdraw/
    │   └── svgdraw.go
    └── vector/
        └── vector.go
```

Most drawing-engine packages already exist under `internal/` with more files;
their facades above stand for retained implementations, not rewrites into
business entities. CAD parsing, raster processing and pure geometry may depend
on each other inside this adapter subtree. They cannot import booking SQL or
generated staff API types. `gateway/conversion` handles temporary files and
normalises their output for the drawing use cases.

### Ring 4, composition and enforcement

```text
internal/
├── infrastructure/
│   ├── router/
│   │   ├── router.go
│   │   ├── middleware.go
│   │   └── mounts.go
│   ├── httpserver/
│   │   ├── server.go
│   │   └── drain.go
│   ├── database/
│   │   ├── open.go
│   │   ├── migrate.go
│   │   └── ownership.go
│   ├── config/
│   │   ├── config.go
│   │   ├── environment.go
│   │   └── validate.go
│   ├── telemetry/
│   │   ├── otel.go
│   │   ├── metrics.go
│   │   └── logging.go
│   ├── buildinfo/
│   │   └── buildinfo.go
│   └── process/
│       ├── role.go
│       ├── signals.go
│       └── readiness.go
├── app/
│   ├── app.go
│   ├── runtime.go
│   ├── repositories.go
│   ├── usecases.go
│   ├── controllers.go
│   ├── events.go
│   ├── jobs.go
│   ├── roles.go
│   ├── providers.go
│   └── shutdown.go
├── layout/
│   ├── layout_test.go
│   ├── ownership_test.go
│   └── wiring_test.go
└── devtool/
    ├── openapi.go
    └── compatibility.go
```

Migrations sit beside their owning repository in `migrations/*.sql`; the driver
runner orders them using `database/ownership.go`. The inventory names migration
directories by rule rather than inventing a complete SQL sequence before the
schema is designed. Test files accompany each implemented package; only the
three enforcement test files are explicitly inventoried here.

## API surfaces and consumers

The [surface catalogue](../api-surfaces/README.md) defines UI,
MCP servers, SDKs, scripts, contract tests and Terraform consume supported public
REST; workers use private APIs/interfaces. The rings organise implementation,
not external consumer privilege or publication.

| Consumer/surface | Contract kind | Binding to this layout |
| --- | --- | --- |
| UI, MCP, SDK, scripts, contract tests and Terraform | Shared public REST business capabilities and explicit projections. | HTTP controllers/presenters map public transport to operation inputs; staff/admin controllers do not define a private UI business contract. |
| Human and non-interactive clients | Distinct human/application identities with session/CSRF or API tokens, Conditional Access and scoped authority. | Edge validates credentials; interactors receive the real actor/client/owner. Application grants do not inherit human authority; app-only identities cannot check in desks. |
| Internal-event visitors including speakers | Internet invitation/verification and restricted visitor REST. | Visitor controller/session profile; no staff privileges or generally discoverable public event catalogue are implied. |
| Provisioning and provider/source callbacks | Native SCIM or provider/source-specific ingress. | `adapter/http/scim` and protocol adapters retain their actual authentication and acknowledgement contracts before use-case calls. |
| Platform workers and private collaborators | Private operation ports and persisted job/fact versions. | App wires workers/runners directly to use cases. The single Booking mutation family and its owner-specific repository capabilities are not public wire resources. |
| Operational and outbound dependencies | Private probes, telemetry, repository/provider gateways. | Infrastructure/private operational routing and adapter gateways; not general public controller methods. |

Consumer-owned use-case ports remain project-private interfaces; no operation
package imports another implementation merely because both are exposed by
public REST. Controllers/composition can reuse the same authorised interactor
for all client kinds. Generated transport stays outside entities and use cases.

Booking enforces physical-space claims; Events' manageevent interactor owns
ticket capacity and coordinates venue consistency through owning interfaces.
Publishing supported business commands does not expose the
transaction handle or every internal transition. SDKs derive from the public
OpenAPI source; private entity/port changes do not automatically change that
schema. Compatibility must cover independently upgraded clients as well as the
UI. Terraform initially manages long-lived configuration; first-party MCP and
provider implementations follow the core platform/SDKs. RBAC is configurable in
the authorisation/role-management use cases. MCP uses user context; machine
scripts use another channel. Authorised UI drift is allowed and initial version
retirement notice is 180 days. Global administration, SCIM-backed role mappings,
grant-only scoped composition and self-permission discovery are agreed, without
selecting a universal authentication journey. The [agreed request contract](../api-surfaces/request-behaviour.md)
puts command receipts in the owning interactor's unit of work, with public
configuration version checks, two request classes, actor budgets and
operation status. Wire and retention details remain open; worker
execution is bounded separately.
Counts remain properties of the printed four-ring trees.

## Deep dives

### Shared claim rules and the single local writer

The critical distinction is between a pure rule and an I/O workflow.
`entity/claim` decides whether a proposed transition is legal from supplied
claims, policy, authority and capacity. It does not load a person, lock a row,
query waitlists, or publish a notification. `booking` loads fresh state, calls
those rules and commits the resulting mutation. Allocation may use the same
eligibility functions to avoid producing obviously invalid proposals, but its
snapshot result is never final authority.

```text
internal/usecase/booking/
├── port.go                Claims, Inventory, OperationalEvidence, IncidentReader, Waitlist, Solver, Authority, Policies, unitofwork.Runner, event.Appender, job.Enqueuer
├── command.go             entity-valued commands/results and operation dispatch
├── commit.go              whole allocation-group validation and one transaction
├── direct.go              post-main-run direct desk/parking requests; waitlist priority
├── transfer.go            acquire replacements and release old claims together
├── release.go             explicit pair/optional cancellation versus automatic desk-only release
├── checkin.go             personal assignee check-in under current claim lock
├── evidence.go            validate/deduplicate/store operational observations; commit facts
├── protection.go          read shared incidents; apply Booking-owned deadline protection
├── offer.go               capacity holds, acceptance, decline and protected expiry
├── room.go                pending holds, acceptance/decline and external observation apply
├── venue.go               Booking-owned venue validation/claim participant in shared scope
├── adjustment.go          dated unallocated-desk movement; preserve confirmed claims
├── intent.go              collect chosen dates, travel and requirements
├── cycle.go               cycle cutoff and execution state
├── snapshot.go            versioned planning inputs
├── plan.go                solver port and pure policy orchestration outside transactions
├── run.go                 Booking-owned group/proposal progress
└── repair.go              replan affected uncommitted groups
```

The file split does not create new importable packages. This candidate accepts
a substantial booking interactor for physical claims without copying their
rules into unrelated use cases. Events retains its own eligibility/ticket rules.
Pure rule tests sit beside `entity/claim` and `entity/event`;
transactional race tests exercise the interactor against PostgreSQL.

Consumer-owned composition uses entity request/result types, not another use
case's `Input` struct:

| Consumer package | Interface it declares | Implementation assigned by app |
| --- | --- | --- |
| `roomrequest` | `RoomCommands.Request/Change/Cancel(context, claim.RoomCommand)` | booking interactor |
| `exchangesync` | `RoomObserver.Apply(context, exchange.Observation)` | booking interactor |
| `manageevent` | `VenueParticipant.Validate/Replace(context, unitofwork.Scope, claim.VenueCommand)` | Booking's owning venue participant; Events keeps its ticket/admission writer. |
| `identity` | `PersonalClaims.ReleaseFuture(context, staff.Departure)` | booking interactor |
| `recordoccupancy` | `OperationalEvidence.Record(context, attendance.Observation)` | Booking validates, deduplicates, persists and commits evidence facts; the edge use case has no evidence-store writer. |
| `manageworkplace` | `DatedInventory.ValidateAndApply(context, workplace.Change)` | Booking validates dated-inventory/commitment impact under the definition-version protocol, including mapping activation. |

These are illustrative signatures, not generated contracts. Booking's method
set uses entity types and satisfies them structurally without importing the
consumer packages. App can add a narrow wrapper for argument adaptation, but
that wrapper cannot decide eligibility or directly write SQL.
The shared `unitofwork` package supplies a transaction-bound scope and join
semantics; SQL types remain in `repository/postgres/tx`.

One place validates actor authority for every mutation, including queued work.
Middleware supplies identity, not a permanent authorisation exemption.
Actor, owner, office scope, delegated source, verification time and applicable
policy version travel with the command. System jobs use explicit operation
authority and recheck current staff/claim state; they do not impersonate a PA.
If source-derived authority needs refresh, booking obtains it through its own
authority port before opening the transaction, then rechecks the locally stored
grant/version/freshness inside the mutation. A revocation between refresh and
commit therefore blocks the action. It never holds inventory locks over Graph.
An administrator's one-or-more office scope is combined with the requested
capability. Organisation-wide policy, retention, identity mappings and integration
settings require explicit global capabilities; office administration alone does
not grant them.

### PostgreSQL ownership and consistency

The reference leaves all tables owned by one repository ring. SpaceWeave's
agreed logical ownership is stronger. This alternative retains owner-labelled
repository groups and migrations while organising code by rings. The owner
labels describe data responsibility, not self-contained domain packages.

| SQL owner | Durable records | Mutation authority and enforcement |
| --- | --- | --- |
| booking | claims, person guards, dated inventory, parking pools, waitlist/offer state, dated neighbourhood adjustments and validated operational occupancy evidence | injected only into booking; evidence validation/deduplication and exclusion/capacity constraints |
| workplace | office calendars, buildings, floors, space definitions, baseline bundles, lifecycle schedules and active geometry mappings | manageworkplace; Booking validates dated impact under versioned activation |
| identity | source links/facts, membership, sessions, capabilities and source-labelled grants | Identity operation family with distinct People, Access and Delegation facets |
| exchange | external mappings, common baseline, cursors, subscriptions, conflicts | exchangesync; no direct mutation of booking claims |
| visits | visitors, host/window, arrival/departure and verified access | managevisit; constrained ticket-link capability consumed by manageevent |
| events | descriptions, audience/access, venue reference, limit, tickets, admission | manageevent owns all event/ticket/admission mutations; Booking receives no Events repository. |
| drawings | source/render revisions, manifests, review/asset pointers | upload/process/review, each with limited operations; no operational mapping writer |
| policies | versions and publication | managepolicy; owners read applicable frozen/current rules |
| operational incidents | confirmed declarations, affected offices/actions/times and revisions | outage confirm/resume family alone writes incident records; Booking and manageevent consume their own guarded read ports |
| notifications | preferences, recipient-purpose deliveries, provider attempts | notify; no authority to change booking outcome |
| reports/retention | projections, export/copy registry, erasure/tombstones | report/retain; source owners apply category deletion ports |
| queue | jobs, outbox, consumer receipts, leases and fences | tx-bound producer handles plus leased runner handles |

Repository constructors return distinct read/write handles, not an exported
`Store` with every method. A manifest maps each table/migration to an owner and
each mutation capability to permitted app constructors. Architecture tests scan
imports, SQL references and wiring; review checks semantic changes to that
manifest. A restricted database role per runtime narrows production access, but
cannot prove per-interactor ownership inside a single process. That limit is
explicit: Go's compiler and the ring diagram do not enforce table ownership.

Design inference: materialise durable guard rows before claims can race.
Acquire locks in a documented total order: event/visit guard where relevant,
staff guards sorted by stable ID, inventory/pool date guards sorted by key,
then claim/offer rows. Operations lock the union of old and replacement keys.
Locking only existing booking rows leaves absent capacity unprotected.
PostgreSQL documents row locks and the need to avoid inconsistent lock order;
the exact guard protocol remains an application design to validate.
[Row locks](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS).

Use half-open UTC intervals with an explicit office-date interpretation.
Exclusion constraints reject overlapping active claims for a specific resource
and overlapping desk claims for one staff member. Pending local room holds and
waitlist offers count as capacity-consuming states until transitioned.
PostgreSQL range exclusions support resource overlap constraints; pooled parking
requires additional coordinated capacity checks rather than a scalar exclusion.
[Range constraints](https://www.postgresql.org/docs/current/rangetypes.html#RANGETYPES-CONSTRAINT).

For pooled parking, lock the area's affected date/interval guards and validate
the proposed overlap count against capacity for every segment. All inserts,
releases and transfers follow that protocol. A required desk/parking group
commits all rows or none. An office transfer validates the replacement against
the prior claims being replaced, then updates them in one transaction; it does
not temporarily create two overlapping personal desks or release the original
first. Serializable/deadlock retries rerun decisions with bounded backoff;
genuine exclusion conflicts return unavailability/replan, not endless retries.
[Transaction retry guidance](https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html).

Specific parking bays use ordinary resource claims; capacity-only parking uses
the pooled path. A linked request without the both-required flag may succeed
independently for each resource, with separate availability and explicit
outcomes. Requirements such as EV charging still determine eligibility.

Before either linked booking starts, explicit cancellation of either component
of a both-required desk/parking pair cancels both in one Booking transaction.
Optional links allow independent cancellation. The command resolves and
revalidates the persisted pair, start times, current authority and claim state
under the normal guards; its receipt, affected claims and follow-up facts commit
together. Automatic desk no-show or vacancy release instead ends the desk claim
only. Parking remains booked until its own end or explicit release, including
in bay/pool capacity calculations. Desk-use evidence cannot establish that a car
has left. The [linked-booking contract](../api-surfaces/linked-bookings.md)
defines the public outcomes and concurrency validation; explicit cancellation
after either booking starts remains a separate product decision.

Versioned workplace changes create dated inventory changes through booking.
Removing a space or lowering capacity with affected confirmations returns an
impact/review result; it does not erase claims. A temporary neighbourhood
adjustment locks the dated inventory and moves only unallocated desks within
the same building. Neighbourhoods may span that building's floors.

### Allocation, waitlist, check-in and outage deadlines

Booking's intent operation persists explicit attendance choices and travel separately from
source-derived defaults. Policies determine office cycle dates, cutoff and run
time. Initial defaults remain the agreed 28-day lookahead, seven-day cycle,
14-day cutoff and run 30 minutes later; values are published data, not constants
inside the scheduler. Closed dates and local daylight-saving transitions are
resolved through `entity/calendar`.

Booking's planning operation reads a versioned snapshot of eligible demand, capacity, teams,
features, policy and neighbourhood membership. It forms allocation groups per
attendance date, not permanent copies of externally maintained teams.
Maximise accommodated demand subject to hard requirements. Among equal-demand
plans, compare explicit collaboration, team proximity and previously unfulfilled
demand in that order. A feature requirement is never softened to improve score;
splitting teams across floors is permitted where it fits more people.

Each proposal carries its input versions, intent IDs and group membership.
Booking revalidates all members and required parking inside a short transaction.
A conflicting member invalidates that group's atomic commit; already committed
groups survive. The allocator refreshes the affected uncommitted groups rather
than starting a long transaction around search or undoing all confirmations.
Direct booking opens after the main run for the date, and newly available
capacity is first matched to eligible waitlisted demand under the same guards.
Assigned groups receive policy-controlled priority access to their
neighbourhood's optional meeting rooms; remaining room availability opens to
other staff under the configured policy. That priority does not bypass room
capacity, delegation checks or Exchange acceptance.

An allocated desk becomes taken only through its owner's personal check-in.
The actor must equal the assignee; delegate authority cannot satisfy this check.
Booking-page and desk-QR entrypoints invoke the same backend command. The initial
no-show grace is 60 minutes and post-check-in vacancy period is 30 minutes,
both versioned/configurable. Full-day intents cover working hours; a released
desk can be booked for the remaining usable period.
Anonymous occupancy and office-presence evidence are separate types and cannot
take a desk or prevent no-show release. After check-in, only fresh sustained
vacancy evidence can trigger sensor-assisted release; an absent or stale sensor
sample means unknown. Without sensors, the taken desk lasts until its normal
end, owner release or another authorised cancellation.

Design inference: persist protected deadlines rather than extend timestamps
from worker execution delay. Outage records identify affected office/service
windows and an epoch. `outage` owns the declaration lifecycle through its incident
repository; Booking and `manageevent` consume their own read ports, never import
the `outage` implementation, and retain their own protected deadline state.
Check-in/no-show/offer transitions read that epoch while
holding the relevant guard, pause elapsed eligible time during confirmed
interruptions and apply configured recovery grace on resumption. A publication
of an outage coordinates with those guards so expiry cannot pass an already
confirmed interruption. A late incident declaration may require review of prior
releases; it cannot silently displace a valid new assignee.

No-show and offer jobs re-evaluate state and protected deadlines when claimed.
A job retry cannot take capacity from a new owner or resurrect an expired usable
booking period. Outage protection never extends a desk past working hours or
overwrites an already committed reassignment. The recovery grace has no agreed
numeric default and must be configured before automatic expiry is enabled.
[Agreed outage policy](../../policy-defaults.md#outage-protection).

### Exchange rooms and source-labelled delegation

`roomrequest` owns the interactive request workflow; `booking` owns local room
holds and transitions; `exchangesync` owns the external reconciliation workflow.
`gateway/exchange` translates Graph/Exchange payloads and errors only.
The dedicated backend identity has scoped mailbox access, but that technical
access does not establish the initiating PA's authority.
Exchange Application RBAC scopes and existing Entra grants are independent;
deployment must check their combined permissions.
[Application RBAC](https://learn.microsoft.com/en-us/exchange/permissions-exo/application-rbac).

An app room request records actor and owner/organiser, takes an expiring local
hold, and atomically enqueues provider creation. Confirmation awaits the room's
accepted response, not merely successful event creation. Room-policy approval
can require a longer explicitly configured workflow than the default 15-minute
automatic acceptance hold.
[Create event](https://learn.microsoft.com/en-us/graph/api/user-post-events?view=graph-rest-1.0),
[room processing policy](https://learn.microsoft.com/en-us/powershell/module/exchangepowershell/set-calendarprocessing?view=exchange-ps).

Persist mailbox-scoped provider identities, client operation IDs, local command
versions, observations and the last common baseline. Webhooks enqueue recovery
work; they are not applied as an ordered authoritative stream. The healthy
five-minute reconciliation cadence persists calendar/date-window delta cursors,
advances only after durable page application and handles invalid cursors with
a bounded resync. Missed/lifecycle notifications trigger recovery.
[Calendar-view delta](https://learn.microsoft.com/en-us/graph/delta-query-events),
[notification recovery](https://learn.microsoft.com/en-us/graph/change-notifications-lifecycle-events).

Design inference: compare local intent and fresh external state to the common
baseline, auto-merge compatible fields, suppress self echoes, and revalidate all
room/time changes through booking. Stale payloads trigger reread or are ignored
when safely identified; arrival order does not decide the winner. Genuinely
incompatible changes become visible reconciliation work with the latest state.
An external overlapping room booking may reveal a real divergence: record it,
block misleading availability and require a resolution rather than violate the
local exclusion constraint or claim a successful confirmation.

Graph's event `changeKey` is a version marker and `transactionId` helps avoid
redundant creates. Neither is treated here as a verified event compare-and-swap
contract or an exactly-once promise. Ambiguous provider timeouts require lookup
and reconciliation before retry. Never infer success solely from a sent request.
[Event resource](https://learn.microsoft.com/en-us/graph/api/resources/event?view=graph-rest-1.0),
[update event](https://learn.microsoft.com/en-us/graph/api/event-update?view=graph-rest-1.0).

For room/time edits, retain the old local claim while obtaining a candidate hold
where feasible and expose pending change state. Exchange may not retain the old
room until the new one accepts; this is a compensating/reconciliation workflow,
not an atomic room swap. If provider behaviour releases the old room, report the
actual state and recover deliberately. Organiser cancellation and deletion of
a room calendar copy are distinct operations.
[Cancel event](https://learn.microsoft.com/en-us/graph/api/event-cancel?view=graph-rest-1.0),
[delete event](https://learn.microsoft.com/en-us/graph/api/event-delete?view=graph-rest-1.0).

Identity's delegation operation imports explicit native calendar-delegate grants with source, owner,
actions and verification time. A new action relying on native-derived authority
must refresh observations older than the configured 15-minute limit; failure to
refresh does not grant permission. Independently approved app-managed grants
are checked against local current state and remain valid after an unrelated
native-derived grant is revoked. Native revocation stops new actions through
that grant; it does not silently cancel already authorised bookings.
Inference for desk/parking/event actions uses an explicit source/action mapping;
it does not treat every observed Exchange permission as unrestricted delegation.

App-managed grants authorise app-mediated rooms through backend authority while
leaving Outlook permissions unchanged. They are not evidence of native PA
access. Provider event data may omit the PA, so the app audit retains the actor
separately from the owner/organiser. Native permission changes would be distinct
authorised operations, not a hidden side effect of granting app delegation.
[Delegated event creation](https://learn.microsoft.com/en-us/graph/outlook-create-event-in-shared-delegated-calendar),
[calendar permissions](https://learn.microsoft.com/en-us/graph/api/calendar-list-calendarpermissions?view=graph-rest-1.0).

### Entra SCIM, Workday and observed lifecycle

Entra SSO, SCIM provisioning and Workday importing are separate protocols and
credentials. `http/scim` implements authenticated `/Users` and mapped `/Groups`
operations, filtering/pagination, PATCH and protocol errors; Identity decides
allowed source mutations. It is not a Graph polling adapter renamed SCIM.
Protocol discovery and supported operations follow the chosen Entra profile.
[Entra SCIM integration](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups),
[SCIM authentication](https://www.rfc-editor.org/rfc/rfc7644.html#section-2).

Maintain internal StaffID, server-issued non-reassignable SCIM ID, provisioning
domain/external ID, Entra object ID and Workday worker identity distinctly.
An organisation-approved unique correlation links verified identities; ambiguous
or conflicting matches enter review. Email-only matching never links accounts.
SCIM `externalId` is client-scoped, not a universal server uniqueness guarantee.
[SCIM common attributes](https://www.rfc-editor.org/rfc/rfc7643.html#section-3.1).

Workday owns workforce lifecycle, reporting structure and usual office; Entra
owns authentication/account security. App preferences and explicit dated travel
stay local. Identity stores effective and observation times independently and
reconciles related organisation/location data, not just worker incrementals.
The organisation's worker/team/population mapping must be validated; a vendor
default mapping is not proof of a correct correlation.
[Workday integration reference](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/workday-integration-reference),
[Workday service directory](https://community.workday.com/sites/default/files/file-hosting/productionapi/index.html).

Identity reduces source observations into access and planning consequences.
A confirmed effective disable/leaver revokes access when observed, increments
a local security epoch and invalidates existing sessions in the same durable
state change. New requests recheck that state; an otherwise valid cookie does
not keep a disabled account active. Effective future changes are scheduled;
source outages preserve known verified state and freshness flags. A missing page
or failed run cannot fabricate a leaver.

The same transaction enqueues cleanup of future personal desks/parking through
the booking command port and room/event handover review. This makes revocation
immediate locally and cleanup retryable; it does not claim instantaneous source
offboarding or atomic updates across Workday, Entra and Exchange.
Workforce office/team changes update unconfirmed derived defaults on the
effective date, preserving explicit travel and confirmed arrangements.
Rooms/events need assisted continuation decisions; organiser transfer is not
assumed to be an in-place provider operation.

SCIM deactivation, reactivation and deletion remain distinct. A deletion removes
the resource from SCIM responses even when retained booking/audit history still
references its internal ID. Reactivation never silently revives cancelled
bookings. Group mappings are explicit and read-only in the app for externally
owned memberships; corrections occur in their source system.
[SCIM active](https://www.rfc-editor.org/rfc/rfc7643.html#section-4.1.1),
[SCIM deletion](https://www.rfc-editor.org/rfc/rfc7644.html#section-3.6).

### Visits, event venues and tickets

`managevisit` checks a named active staff host, office and visit window, issues
restricted verified-email access and records arrival/departure separately from
event admission. A visitor principal can act only for the associated visit or
eligible event; it never receives a general staff session.
Reception requires the visitor capability and an office in scope.

`manageevent` is Events' operation family: it owns title, audience, registration,
ticket limits, named entitlements and admission. `booking` owns venue claims.
The [agreed boundary](../../adr/0023-separate-events-from-venue-booking.md) is
represented across rings rather than by a self-contained domain directory.
App injects Events repositories only into manageevent and supplies Booking's
venue participant through the consumer-owned interface; no Events SQL handle
is handed to Booking.

Event/ticket business rules remain pure functions in `entity/event`, including
audience eligibility, named-attendee uniqueness and admission transitions.
Tickets are free and capacity-limited. Event audience may be staff-only or admit
external visitors; registration access independently selects invitation-only
or public registration for eligible attendees.
Manageevent applies those rules to current Events state and validated
identity/visit facts. Events owns its repositories and migrations. Its venue
coordinator joins Booking's owner-specific operation to the opaque unit of work;
Booking validates/writes only venue claims and Events validates/writes only
event/ticket state. Controllers, queue runners and Booking never receive an
Events writer. The peer-implementation import ban remains: app wires the
structurally compatible venue participant instead of importing Booking in manageevent.

Design inference: the coordinated venue workflow and ticket issuance take
compatible physical-resource guards followed by the Events-owned event guard.
Events validates issued tickets and its event limit; Booking validates the
new venue. Each owner updates its own state and appends follow-up notification
work in one shared transaction. It either preserves existing ticket entitlements or
rejects with an impact result; it cannot confirm a too-small venue and repair
overselling afterwards. Publish the limit no higher than the booked venue.

Create a registration-specific visit and ticket in one local transaction through
Events' narrowly injected Visitors capability, as agreed. Failed issuance rolls
back that new visit, while a reused existing valid visit remains intact. Ticket uniqueness and
capacity checks share the event guard. Admission validates audience, usable
ticket and event window, then records one admission under a unique ticket key.
A repeated scan returns already admitted with the prior record; a visit arrival
does not consume a ticket and a ticket scan does not fabricate a reception visit.

Confirmed venue loss retains tickets pending recovery and pauses admission and
further issuance. Events requires an explicit organiser decision to secure a
suitable replacement or cancel, communicating confirmed changes to attendees.
Admission rechecks that venue recovery has been resolved as well as ticket,
audience, visit and event-window validity.

### Durable jobs, outbox and visible failure

Every business state change and its required follow-up job/outbox record commit
together. Shared contracts describe append/enqueue within a unit of work;
repository handles reject a production write that lacks the active scope.
No HTTP/provider call runs inside a booking transaction. An after-commit wakeup
reduces delay, but polling durable work is the recovery path if it is lost.

`jobrunner` is a driving adapter. It claims ready jobs with leases and invokes
use-case handlers selected in `app/jobs.go`. `eventloop` fans out outbox records
with durable per-consumer receipts; event order is per relevant aggregate/version,
not an unsupported global order. PostgreSQL `SKIP LOCKED` is appropriate for
competing queue claims, and explicitly gives an inconsistent view; it is never
used to find supposedly free desks or skip a waitlisted claimant.
[Queue locking caveat](https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE).

Design inference: a claim returns a monotonically changing fence token. Heartbeat,
acknowledgement and state mutation compare that token; expired workers cannot
acknowledge a successor's lease. Use deterministic operation IDs, inbox/consumer
deduplication and provider-specific reconciliation. Retry with bounded backoff,
honour throttling, park permanent failures with an operator reason, and preserve
attempt/audit history. Leases do not prevent a remote call completing after its
worker has lost the lease; reconciliation must tolerate that race.

`operatejobs` may retry, cancel or unpark work with appropriate administrative
capability. It cannot change a booking from the job dashboard. Completion always
re-enters the owning use case and its current guards. Inspect backlog age,
attempt counts, missing receipts and stalled workflow state, not only queue depth.

### Notifications and private booking-raised Teams messages

`notify` consumes a committed booking-raised event, derives relevant owners and
acting delegates, and creates purpose/recipient/channel delivery intents.
Email and in-app delivery retain their configured preferences. Teams receives
only a private message that a booking was raised, and only for an individual who
opted in to that exact message purpose. A generic Teams-enabled flag is
insufficient. Invitation, waitlist, handover and cancellation messages do not
inherit that permission.

The `PersonalMessenger` port consumes an internal recipient and message; the
Teams adapter handles installed app identity, conversation reference and provider
requests. Installation is separate from message consent. Recheck current opt-in
before each send; a later retry after opt-out is suppressed and audited.
Missing installation, blocked/uninstalled users and throttling become visible
delivery states and bounded retries, never a rollback of the booking.
Microsoft documents proactive personal messaging and installation prerequisites.
[Proactive messages](https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/conversations/send-proactive-messages).

Design inference: keep the personal delivery interface narrow enough that it
cannot accept a channel ID. The composition root registers the Teams route only
for `booking.raised`. There is no shared-channel branch or default opt-in.
Ordinary Graph background chat writes are not assumed to support this: its
documented application permission on chat-message creation is for migration.
[Chat-message permissions](https://learn.microsoft.com/en-us/graph/api/chat-post-messages?view=graph-rest-1.0).

### Drawing conversion, tracing and publication

Today's deterministic CAD/raster pipeline is valuable implementation, not a new
booking domain. Preserve its algorithms and fixtures while separating business
revision workflow from local filesystem orchestration. Current
[conversion](../../../internal/convert/convert.go),
[trace types](../../../internal/plan/trace.go) and
[worker publishing](../../../internal/worker/worker.go) show those mixed concerns.

```text
upload source package -> verify immutable source/recovery objects -> metadata/job
  -> convert/sniff -> deterministic autotrace OR configured Claude gateway
  -> validate geometry/renderer schema -> immutable draft render manifest
  -> reviewer approve -> verify recovery artifacts -> approved asset pointer
  -> manageworkplace prepares stable-space mapping -> Booking validates dated impact
  -> activate Workplace mapping with definition-version guards
```

`drawingupload` preserves architecture, space-plan and reference roles, checks
format/content and records checksums. `drawingprocess` uses bounded temporary
scratch space, the existing CAD/raster engines and a validator port. The optional
Claude adapter keeps provider SDK, spend/turn/time limits and transcripts outside
entities; deterministic mode needs no model credentials or outbound model call.
Retries reuse source revision and attempt identity, never rewrite an approved
asset in place. Review rejection creates a new attempt/revision lineage.
`manageworkplace/mapping.go` owns operational mapping publication through its
consumed Booking inventory port and its own repository; `drawingreview` owns
asset approval only. A combined administrator workflow reports each outcome,
including approved assets awaiting activation after an impact conflict.

Azure and GCS implement the same use-case-consumed blob interface; deployment
configuration selects the active provider. Blob identity includes opaque provider
version/concurrency tokens plus an independently computed content checksum.
ETags and generations have different semantics and are not portable checksums.
[Azure conditional writes](https://learn.microsoft.com/en-us/azure/storage/blobs/concurrency-manage),
[GCS preconditions](https://docs.cloud.google.com/storage/docs/request-preconditions).

Design inference: upload staging, verified recovery copy and PostgreSQL manifest
publication form a resumable state machine. Database/blob writes are not one
transaction. Verify every referenced source/render asset in the recovery region
before acknowledging it durable or publishing the revision. A staged upload may
return a processing status; it is not described as durable publication yet.
Orphan cleanup uses manifests and grace periods; historical booking/plan
references protect retained revisions from normal replacement cleanup.

### Generated transport and the two React portals

The reference places generated strict servers in infrastructure and then allows
controllers to import that outward ring. This candidate deliberately chooses
`adapter/http/gen` instead. Generated Go request/response types and strict server
interfaces are transport adapters, so controllers can implement them without a
dependency exception. The router alone assembles Chi middleware/mounts.
`oapi-codegen` documents strict-server and Chi generation; tool selection and
version pinning remain implementation decisions.
[Generator documentation](https://github.com/oapi-codegen/oapi-codegen).

| Contract | Go generated transport | Handwritten mapping | TypeScript output |
| --- | --- | --- | --- |
| staff REST | `adapter/http/gen/staff` | `controller/staff` -> entity commands/use cases | `web/packages/api/staff.gen.ts` |
| admin REST | `adapter/http/gen/admin` | `controller/admin` -> scoped/global operations | `web/packages/api/admin.gen.ts` |
| restricted visitors | `adapter/http/gen/visitor` | `controller/visitor` -> visit/event access | `web/packages/api/visitor.gen.ts` |
| Entra SCIM | protocol adapter, not staff DTO aliases | `http/scim` -> Identity provisioning | no browser provisioning client |
| provider notifications | provider payloads inside adapters | exchangehook/teamshook | no provider tokens/types in browser |

OpenAPI fields do not become entity fields by import. Controllers validate
transport syntax and map to internal IDs, intervals and entity commands.
Use cases return domain outcomes; presenters map pending/conflict/error forms.
Generated schemas can evolve independently of SQL rows or allocator proposals.

```text
web/
├── staff/
│   ├── index.html
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── routes.tsx
│       └── screens/
│           ├── AttendanceIntent.tsx
│           ├── Availability.tsx
│           ├── MyBookings.tsx
│           ├── Rooms.tsx
│           ├── CheckIn.tsx
│           ├── Events.tsx
│           ├── MyAttendance.tsx
│           └── Preferences.tsx
├── admin/
│   ├── index.html
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── routes.tsx
│       └── screens/
│           ├── Workplace.tsx
│           ├── Cycles.tsx
│           ├── AllocationRuns.tsx
│           ├── Visitors.tsx
│           ├── EventCapacity.tsx
│           ├── Reports.tsx
│           ├── Policies.tsx
│           ├── Handover.tsx
│           ├── Integrations.tsx
│           ├── Jobs.tsx
│           └── DrawingReview.tsx
├── packages/
│   ├── api/
│   │   ├── staff.gen.ts
│   │   ├── admin.gen.ts
│   │   ├── visitor.gen.ts
│   │   └── transport.ts
│   ├── ui/
│   │   ├── Button.tsx
│   │   ├── Field.tsx
│   │   └── PendingState.tsx
│   ├── session/
│   │   ├── SessionProvider.tsx
│   │   └── csrf.ts
│   ├── floorplan/
│   │   ├── FloorPlan.tsx
│   │   ├── bridge.ts
│   │   └── selection.ts
│   └── renderer/
│       ├── geometry.js
│       ├── projection.js
│       ├── render.js
│       ├── cutaway.js
│       └── desks.js
└── floorplan-iso/                    retained development workbench/fixtures initially
```

Go owns OAuth/OIDC state, provider tokens, session rotation and server-side
session revocation. Cookies are secure/HttpOnly with CSRF controls on mutations.
Both entrypoints call backend-authorised APIs: hiding an admin route does not
confer an office or global capability. Browser clients carry no Graph/Workday
credentials. Their pending state represents asynchronous provider completion.

Design inference: wrap the existing D3/SVG renderer behind a ref-managed
container; React owns the wrapper and controls, the renderer owns that
container's children, and cleanup removes listeners and animation work.
React documents safe DOM management boundaries for empty containers; renderer
reuse still needs visual and interaction regression checks.
[React DOM refs](https://react.dev/learn/manipulating-the-dom-with-refs).

### Reporting and retention across copies

`report` queries permission-filtered views/projections labelled by evidence:
booked, personal check-in, desk occupancy and office-presence are different
measures. Staff see their own attendance. Reporting administrators see only
authorised offices; export creation and download both enforce current scope.
No report generates an attendance minimum or turns anchor-day context into an
attendance enforcement policy.

`retain` applies separately configured organisation-wide attendance, raw
occupancy and visitor retention. The copy registry includes identifiable
outbox payloads, delivery records, exports, projections, trace/log fields and
recovery material where applicable. Prefer IDs and minimal event envelopes,
avoid unnecessary personal data in telemetry, and erase/anonymise derived copies
through explicit owner ports. A projection refresh must not restore erased data.

Design inference: deletion records include subject/category, policy version,
last processed copy and a resumable tombstone. Restore runbooks reapply these
tombstones before serving retained snapshots, while documenting backup-expiry
limits. Retention is not complete when only the owning visit row disappears.
Protected historical references may retain non-identifying revision/booking
facts without retaining visitor contact data indefinitely.

### Composition, runtime roles, observability and recovery

```text
internal/app/
├── runtime.go       shared durable dependencies and lifecycle handles
├── repositories.go  least-capability SQL handles; enforce owner/writer assignment
├── providers.go     active Azure/GCS, recovery, Exchange/Entra/Workday/Teams
├── usecases.go      construct operation families; assign consumer-owned ports
├── controllers.go   construct transport adapters and generated implementations
├── events.go        event kind -> consumer -> durable receipt identity
├── jobs.go          job kind/version -> handler; retry/permanent failure policy
├── roles.go         HTTP/allocation/Exchange/tracing/maintenance role allowlists
└── shutdown.go      drain admission, release/expire leases and close resources
```

App constructs booking once per process and assigns its narrow methods to
Booking's private planning, room/event workflows and direct controllers. It
constructs one Identity family for source lifecycle, sessions and grants. It assigns
delegation lookup to roomrequest/booking without creating import edges between
the operation implementations. Tests inspect registrations: no missing consumer,
duplicate receipt identity or privileged repository handle in a notification
worker. Role-specific wiring avoids opening unnecessary provider credentials.

Each Kubernetes role uses the same immutable image but different command,
queue-kind allowlist, scaling policy, resource requests and service identity.
HTTP replicas are not an in-memory job queue; tracing scratch disks are not
durable assets. Managed PostgreSQL runs separately with its own HA and backups.
Migration is a controlled release job, not every Pod racing at startup.
Kubernetes Deployments manage replicated workloads; this candidate must still
validate application drain/lease behaviour during rollout.
[Deployment behaviour](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/).

Provider access uses scoped workload federation where supported and the
organisation's secret manager for remaining credentials. A tracing Pod need not
have mailbox access; an Exchange worker need not issue public blob links.
Health checks distinguish process liveness, ability to accept core requests,
and delayed optional integrations. An unavailable Exchange service creates
pending/retry work without taking desk booking/check-in down.

Infrastructure configures OpenTelemetry trace/metric export through a Collector
and structured JSON logging. Use-case decorators record command latency/outcome,
SQL adapters measure lock waits/retries, and workers attach operation/event IDs
across asynchronous hops. Avoid staff/booking IDs as unbounded metric labels.
OpenTelemetry recommends a Collector for production exporting.
[Go exporters](https://opentelemetry.io/docs/languages/go/exporters/).

Dashboards/alerts cover core SLO burn, initial-response p95, claim contention,
allocation completion/failed groups, oldest unprocessed outbox/job, Exchange
actual freshness and pending-room age, check-in/offer expiry lag, copy erasure
backlog and asset recovery readiness. A healthy scheduler heartbeat is not
evidence that bookings or Exchange deltas are progressing.

The deployment has one active write region, multi-zone HA and a warm recovery
region. Failover fences prior database/application writers before promoting and
resuming jobs. PostgreSQL explicitly warns against old/new primaries both acting
as writer; database recovery alone does not restore routing, identity or assets.
[Failover](https://www.postgresql.org/docs/current/warm-standby-failover.html).

Five-minute RPO and fifteen-minute RTO are measured targets. The recovery process
checks database commit position, verified asset manifests, routing, source
credentials, session/security state, outbox cursors and worker fencing together.
A healthy primary continues serving if recovery lag exceeds five minutes,
raises a critical alert, and requires an explicit operator decision for failover
beyond the loss budget. No automated rule silently converts lag into a primary
outage. Post-failover reconciliation detects effects sent before lost receipts.

GCS turbo replication and Azure Geo Priority Replication document qualified
15-minute replication objectives, not this application's five-minute guarantee.
Therefore verified recovery copies before asset publication are part of the
workflow, and actual end-to-end recovery must be exercised on the chosen platform.
[GCS durability](https://docs.cloud.google.com/storage/docs/availability-durability),
[Azure priority replication](https://learn.microsoft.com/en-us/azure/storage/common/storage-redundancy-priority-replication).

### Workplace retirement, reactivation and deletion

`internal/usecase/manageworkplace/retirement.go` handles explicit retirement and dependency-checked removal
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

## Walkthroughs

### 1. Commit a group while capacity changes

1. `jobrunner` invokes Booking's allocation operation for one office cycle/date from durable work.
2. `booking/snapshot.go` reads versions for intent, workplace, inventory,
   source teams and policy; planning holds no long database transaction.
3. `entity/allocation` forms date-specific groups and compares eligible plans.
4. Booking invokes its consumed solver interface, then its own group-commit
   operation; no peer allocation owner or forwarding interactor is introduced.
5. `booking/commit.go` starts a transaction, deduplicates the proposal command,
   locks all people and desk/parking guards in the documented order and reloads.
6. A concurrent direct request or cancellation either precedes the guard lock
   or waits. Booking checks current eligibility, waitlist priority and versions.
7. It confirms every group desk and both-required parking claim plus jobs/events,
   or commits none of that group. A conflict returns affected keys for replanning.
8. `booking/repair.go` refreshes affected uncommitted groups; existing confirmed
   placements remain protected. Notify runs after committed state exists.

### 2. Check-in races no-show and a waitlist offer

1. Staff controller resolves the personal session and calls booking check-in;
   a PA request fails actor-equals-owner even with valid booking delegation.
2. A no-show job targets the same claim/date guard. Both reread the claim,
   protected deadline and outage epoch under the lock.
3. If valid check-in wins, the claim is taken and stale no-show work is a no-op.
   If valid release wins, a late check-in cannot reclaim reassigned capacity.
4. Anonymous occupancy cannot block release. A confirmed affected app outage
   pauses eligible expiry time and adds configured recovery grace on resumption.
5. Automatic desk release preserves linked parking until its own end or explicit
   release. Write desk waitlist-match work atomically; evaluate eligible queued
   demand against actually released capacity before fresh direct bookings.
6. Same-day promotion creates a ten-minute default offer hold, not an automatic
   personal check-in. Acceptance/expiry share guards and outage protection.
7. After acceptance and check-in, a sensor outage preserves taken status;
   fresh sustained vacancy evidence is required for the optional vacancy rule.

### 3. PA room request, late acceptance and Outlook edit

1. `roomrequest` distinguishes PA actor and owner/organiser, asks its authority
   port for the grant source and refreshes stale native-derived evidence.
2. Booking rechecks current local authority and creates a pending room hold,
   command version and outbox/job in one transaction.
3. `exchangesync/outbound.go` invokes the scoped Exchange gateway after commit,
   retaining provider correlation/operation IDs across ambiguous retries.
4. A webhook arrives late after an Outlook edit; the handler durably schedules
   reconcile rather than trusting payload arrival order.
5. The sync use case fetches current state/delta, compares the common baseline,
   auto-merges compatible changes and asks booking to revalidate room/time.
6. Acceptance confirms only the still-current compatible request. An obsolete
   acceptance cannot revive an expired/replaced local hold; divergence is visible.
7. An incompatible change creates review work. Local/provider room swaps remain
   pending/compensating workflows without a Graph atomic-swap assumption.
8. Revocation removes the native grant for new actions. A separately granted
   app-managed authority remains independent; neither changes Outlook rights.

### 4. Smaller event venue races ticket issuance

1. Admin controller checks event-management capability and office scope.
   Events' `manageevent` starts the named venue-coordination unit of work.
2. Booking's participant locks/validates its old/new venue claims; Events locks
   its event guard and reads its own issued tickets and current limit.
3. A simultaneous registration reaches manageevent and takes compatible venue
   validation and event guards; it cannot use an old ticket count and oversell.
4. If issued entitlements exceed the new capacity, the move returns an impact
   result with the original venue intact. Otherwise venue claim, reference,
   event limit and follow-up work commit together.
5. A visitor registration verifies eligible audience/access, creates/links the
   named visit and issues the ticket in the same capacity transaction.
6. Entry scans create one unique admission. Duplicate scans return the prior
   admission; reception arrival/departure remains a separate visit record.

### 5. SCIM disable and an effective Workday change during outage

1. `http/scim` authenticates the provisioning client and maps the SCIM ID to a
   verified source link; email similarity cannot substitute for that link.
2. Identity records the effective source disable and access/session epoch
   invalidation in the same owned transaction; later cleanup cannot delay denial.
3. Identity imports an independently observed effective office/team change.
   Field ownership prevents that feed from re-enabling an Entra-disabled account.
4. A failed related Workday page preserves its previous state with stale-source
   evidence; it does not clear teams, infer deletion or manufacture a leaver.
5. Lifecycle jobs release future personal desks/parking through booking and
   create room/event handover tasks. History remains subject to retention.
6. Unconfirmed planning defaults adopt valid effective source changes; explicit
   travel and confirmations survive. Ambiguous source correlations enter review.
7. Recovery reconciles missed pages and related records. SCIM deletion later
   hides the provisioning resource while retained internal references remain.

### 6. Add private booking-raised Teams delivery

1. Add a purpose-specific preference and optional `PersonalMessenger` port to
   notify; booking rules and SQL claim code receive no Teams dependency.
2. Add gateway personal-conversation/installation handling and authenticated
   inbound activity mapping; pin any SDK/protocol choice during implementation.
3. App registers this channel only for committed `booking.raised` recipients.
4. Notify checks the individual's opt-in for that message and available personal
   installation, composes minimal content and records a delivery intent.
5. A leased job rechecks consent before sending. Blocked/uninstalled/throttled
   delivery records visible suppression/retry/failure, never booking failure.
6. Email/in-app outcomes remain governed by their own preferences. No shared
   channel is introduced and no other Teams purpose inherits consent.

## Package mapping

The current repository is a rendering/tracing application. The mappings below
describe origin and responsibility, not a mechanical directory rename.
Existing geometry, real-file fixtures and rendering regressions are assets to
preserve. Booking, identity and provider integrations require new behaviour.

| Current path | Candidate destination | Split or preservation |
| --- | --- | --- |
| `cmd/spaceweave/main.go` | same entrypoint -> `app`/adapter CLI | retain one executable; introduce explicit production roles |
| `internal/cli` | `adapter/cli`, `app`, infrastructure process | Cobra commands/presenters versus construction/listen/drain |
| `internal/config` | `infrastructure/config` | strict YAML/flag precedence; operational policy moves to DB |
| `internal/buildinfo` | `infrastructure/buildinfo` | retain version metadata |
| `internal/server/server.go` | infrastructure router + adapter HTTP controller | Chi mounts/lifecycle separate from upload/review orchestration |
| `internal/server/files.go` | drawing attachment controller/upload operations | Preserve multipart attachment/detach validation; static generated-directory serving comes from `server.go`, not this file. |
| `internal/server/profiles.go` | admin drawing controller + processing ports | preserve editable profile workflow; revision its effective input |
| `internal/catalog/catalog.go` | drawing entity, drawing repositories and queue | state machine separates from local lock/file catalogue |
| `internal/catalog/package.go` | drawingupload + revision metadata | preserve architecture/space-plan/reference package roles |
| `internal/inbox` | `adapter/gateway/inbox` | optional local development ingestion, not production durable queue |
| `internal/worker` | drawingprocess + jobrunner + blob publication | split workflow, lease runner and artifact persistence |
| `internal/convert` | gateway conversion | sniffing/conversion stays outside pure entities |
| `internal/autotrace` | drawingengine autotrace | retain deterministic algorithm and golden tests |
| `internal/autotrace/cadfront`, `rasterfront` | corresponding engine packages | preserve CAD/raster frontends and quality fixtures |
| `internal/autotrace/profile`, `score`, `synth` | corresponding engine packages | library I/O stays adapter; profile versions enter processing input |
| `internal/floorgraph` | drawingengine floorgraph | retain graph/topology algorithms and regression corpus |
| `internal/cad`, `dxf`, `dwg` | corresponding engine packages | preserve parsers, limits and real-file tests |
| `internal/dxf/dxftest`, `dwg/dwgtest` | corresponding engine fixture packages | keep test-data construction local to engine families |
| `internal/raster`, `svgdraw`, `vector` | corresponding engine packages | no automatic rewrite as booking entities |
| `internal/plan` | engine plan + entity drawing + validator gateway | geometry/trace format, revision rules and Node execution split |
| `internal/trace` | gateway Claude + drawingprocess | SDK/tool loop separate from durable revision orchestration |
| `internal/fakeclaude` | test fixture for Claude gateway | retain controlled provider behaviour; absent from production roles |
| `web/floorplan-iso/js` | shared renderer + React bridge/workbench | keep tested D3 geometry/rendering; migrate workflow UI separately |
| `web/floorplan-iso/tools/validate.mjs` | validator gateway tool | retain schema/geometry validation contract |
| `test/acceptance` | existing tracing acceptance plus booking suites | keep real dependency testing; add PostgreSQL/provider contracts |

Design inference: migrate at a working drawing seam first, with old/new output
comparison and immutable publication acceptance. Do not convert `catalog.Claim`
into distributed job leasing by changing its package name: its local lock-file
protocol is not PostgreSQL fencing. Likewise the current local custom-header
origin defence is not the complete authenticated session/CSRF design.
Those facts are visible in the current catalogue and server sources cited above.

## Test and enforcement plan

The existing engineering gates remain: `GOWORK=off`, golangci-lint, SHA-pinned
CI actions and merged coverage of at least 95% total/90% per package.
The candidate's narrow ports support pure orchestration tests but do not
prove transactional correctness. Establish real dependency acceptance early,
as required by [CLAUDE.md](../../../CLAUDE.md).

| Suite | Evidence required |
| --- | --- |
| entity tests | interval/DST boundaries, required features, claim transitions, evidence distinctions, objective order, consent scope |
| operation tests | own-port fakes; authorisation, source ownership, stale observation, pending and retry decisions |
| PostgreSQL contracts | migrations, exclusions, pool limits, lock protocol, rollback and durable job/outbox atomicity |
| concurrent integration | group/direct/transfer races, check-in/no-show/offer races, smaller venue/ticket race, lease fencing |
| provider contracts | Graph duplicates/out-of-order/missed deltas; SCIM PATCH/filter/delete; Workday effective/related changes; personal Teams failures |
| drawing regression | current deterministic golden/real CAD corpus, renderer validation, immutable manifest and recovery-copy failure |
| browser/acceptance | two portals, backend scope/CSRF, PA personal check-in denial, restricted visitor and duplicate ticket scan |
| failure/recovery | commit-before-crash, send-before-ack, lost lease, confirmed outage grace, source outage and regional failover |
| load/SLO | representative concurrent office bursts, hot parking pools/events, initial p95, allocation completion and DB contention |

`layout_test.go` checks the import DAG including test imports, entity I/O bans,
and the exact three shared-contract exceptions. Generated adapter imports do
not need an outward exception. `ownership_test.go` checks table/migration owner
and SQL write allowlists. `wiring_test.go` checks that only Booking receives
physical-claim handles and only Events receives event/ticket/admission handles,
every durable event/job has a registered versioned handler,
and each role gets only required capabilities.

Acceptance must prove adversarial cases, not mirror implementation: an anonymous
occupied desk still releases as no-show; a PA cannot check in; stale native
authority cannot authorise a new action; an app grant survives unrelated native
revocation; a visitor expiry removes identifiable report/export copies; a Teams
opt-out during retry suppresses that specific message. Recovery tests restore
database, assets, identity/routing and workers together and reapply erasure state.

Thin router/driver/generated packages may create pressure on package coverage.
Use narrow real assembly tests and justified generated-code exclusions, not fake
tests asserting constructor assignments solely to reach a number. Any exclusion
change requires review against the existing gate. This research adds no tests or
code; these are the candidate's required verification responsibilities.

## Numbers

Counts below describe only the explicitly printed canonical four Go trees.
They exclude root/portal examples, later deep-dive repetitions, implied test
files, SQL migrations and the unprinted files behind existing engine facades.
They are structural design counts, not estimates of final implementation size.

| Measure | Printed candidate |
| --- | --- |
| Entity packages | 18 |
| Operation-family packages | 18 |
| Shared ring-2 contract packages | 3 |
| Adapter packages, including generated transport and engine fixtures | 59 |
| Infrastructure packages | 7 |
| Composition/enforcement/devtool packages | 3 |
| Total explicitly inventoried Go packages | 108 |
| Explicit `.go` file entries, including generated/enforcement examples | 332 |
| Core dependency rules plus ownership/port rules | 9 |
| Common comparison scenarios traced | 6 |

The count includes the three generated transport packages and two engine
fixture-helper packages because their directories contain Go source.
No public Go client is counted: one may be generated later if an actual consumer
needs it. The largest proposed handwritten operation family is booking; the
largest retained algorithm inventory is the drawing engine, whose real files
are deliberately not estimated from these facades.

## Placement choices and friction

Choices made here:

- **Entities stay private and pure.** Shared claim/authority/evidence decisions
  are inward dependencies; CAD parsers and model SDK loops remain adapters.
- **Operation families organise ring 2.** There are no self-contained domain
  directories containing their own HTTP, SQL and application implementation.
- **Exactly three ring-2 shared contracts.** Unit of work, event append and job
  enqueue are explicitly allowlisted; they contain no booking convenience API.
- **One physical-claim interactor and a distinct Events owner.** Consumers declare
  command ports and app assigns Booking's implementation. Events owns ticket SQL
  and joins Booking's venue participant through a narrow shared transaction.
- **Owner-labelled SQL groups.** Limited repository handles express ownership
  within the shared persistence adapter.
- **Generated Go transport is an adapter.** Wire types remain outside entities
  and use cases, preserving the inward dependency rule.
- **Jobs are driving adapters and durable data.** Provider effects run after
  commit with reconciliation; queue workers do not own business transitions.

Friction and compromises:

- A feature crosses entities, operation packages, repositories, controllers,
  specifications and app. Reading a whole event workflow requires following
  those paths; the rings do not reveal the event/visit/booking collaboration.
- Consumer-owned ports can duplicate shapes and increase constructor wiring.
  Replacing them with a broad shared `BookingService` would weaken this
  candidate's operation/consumer boundary while reducing duplicated interfaces.
- Booking becomes a large operation family. This is the price of shared atomic
  capacity and the peer-import ban. If it becomes unmanageable, revisiting that
  rule or selecting a consistency-oriented module layout is more honest than
  creating peer implementations under an innocently named contract package.
- Event and venue writes have distinct owners. The coordinator must preserve
  atomicity and a common lock order through owning interfaces; tests must reject
  both stale cross-owner checks and injection of foreign repository handles.
- Ring tests do not stop a new operation declaring an illicit writer port.
  Construction/SQL ownership checks and review remain necessary; per-runtime
  database roles cannot fully enforce an intra-process boundary.
- Pure rules require carefully chosen input facts. Passing giant snapshots to
  every rule recreates a universal model; many tiny DTO conversions obscure the
  invariant. Use stable entity values and explicit operation-specific facts.
- Cross-source and provider uncertainty survives architectural purity. Graph
  lacks a verified atomic room-swap/CAS contract here; source offboarding is
  observed, and external sends may repeat. Pending state and reconciliation
  remain first-class user-visible workflows.
- The composition root carries meaningful policy about event/job wiring and
  capabilities. It grows with the number of workflows and must be reviewed as
  application structure rather than dismissed as boilerplate.
- Recovery-copy verification can delay durable drawing acknowledgement, and
  guard contention can affect latency. Benchmark those actual workflows; neither
  an inward arrow nor extra worker replicas establishes the SLOs.

This candidate is strongest when strict isolation of pure policy from SQL,
frameworks and provider payloads is the primary maintenance concern. Its cost is
the distance between all pieces of one business workflow and the extra ownership
enforcement needed beyond the rings.
