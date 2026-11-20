# Option 5: Modular monolith

The repository-architecture decision is pending. This candidate expresses the
same agreed logical owners as the other four: combined Identity; attendance
planning and claim commitment inside Booking; separate Exchange and Events.
Its package organisation and dependency rules remain specific to this style.
[Identity](../../adr/0026-combine-workforce-identity-and-access-ownership.md),
[Booking planning](../../adr/0027-keep-attendance-planning-inside-booking.md) and
[Exchange](../../adr/0028-separate-exchange-synchronisation-from-booking.md)
record those ownership decisions independently of repository style.

SpaceWeave coordinates workplace use across offices, buildings and floors for
approximately 100,000 staff and 60,000 desks. Its booking and allocation rules
need strong local consistency, while identity providers, Exchange Online,
notifications and cloud assets introduce asynchronous integration work.

This candidate encapsulates business capabilities in modules with published
interfaces and owned data inside one application release. Modules use ports and
adapters internally where their complexity warrants them. The [architecture
foundations](architecture-foundations.md) distinguish this organisation from Go
packages, process roles and microservices. [Spring Modulith's module model](https://docs.spring.io/spring-modulith/reference/fundamentals.html)
provides a concrete framework example of module APIs and internal dependencies;
SpaceWeave's implementation remains Go.

The [common requirements](README.md#requirements-held-constant) and
[comparison scenarios](README.md#common-comparison-scenarios) define the evaluation
baseline. Module boundaries, package paths and transaction exceptions below are
proposed design choices for review.

## Summary

Organise the application by logical owners with explicit published interfaces
and private data. Identity combines workforce/source correlation, sessions,
permissions and delegation through distinct interfaces. Booking owns attendance
intents, cycles, groups, solver orchestration and dated physical-space claims.
Exchange separately owns provider mappings, desired/observed calendar state,
subscriptions and reconciliation progress; it applies capacity/state transitions
through Booking. Events owns registration, tickets, offers and admission.

The scale is one organisation across many offices, 100,000 staff and 60,000
desks. Bookable kinds include desks, meeting rooms, specific parking bays,
pooled parking and event venues; tickets are free and named.

The agreed logical boundaries apply to every architecture candidate. This option
encapsulates them as modules with inner ports/adapters where useful; the overall
repository-architecture choice remains pending. Compare the
[Identity decision](../../adr/0026-combine-workforce-identity-and-access-ownership.md),
[Booking planning decision](../../adr/0027-keep-attendance-planning-inside-booking.md),
[Exchange decision](../../adr/0028-separate-exchange-synchronisation-from-booking.md)
and [Events decision](../../adr/0023-separate-events-from-venue-booking.md).
Concrete file names, transaction-scope names and publication-guard mechanics
below are representative implementation proposals.

Dependency rules for this candidate:

1. A module imports its own packages, `kernel`, approved `platform` packages and
   another module's `api` packages. It never imports another module's SQL,
   implementation models, jobs or constructors. Cross-module calls are ordinary
   Go calls with explicit arguments, deadlines and errors.
2. Each module's `api` imports only the standard library and `kernel`. It
   publishes commands, results, query interfaces and versioned event payloads;
   it exposes no SQL handles, generated HTTP types or private models. Keeping
   API packages independent prevents package cycles, but does not prevent
   runtime call cycles; the composition and call graph are checked separately.
3. Large modules use `core` and `adapters`: core never imports adapters,
   provider SDKs, Chi or generated transport. Small modules may keep rules,
   stores and handlers in a few flat packages. Uniform layering inside every
   module would add ceremony without strengthening ownership.
4. Gateways call module APIs and own response composition, portal assets and
   transport mapping. They own no tables and cannot create transactions across
   modules. The gateway never decides availability or authorisation by joining
   another module's tables.
5. Platform and kernel import no business module. Platform supplies mechanisms
   such as PostgreSQL connections, named transaction scopes, outbox storage,
   jobs, HTTP support and telemetry. Kernel contains only stable identifiers,
   principal context, time ranges and paging; no Staff, Booking or global Store.
6. App constructs modules and adapters, mounts Chi routes and registers durable
   handlers. Only app and command entrypoints import module constructors.
   Provider adapters are registered explicitly; business services do not locate
   dependencies through a service registry.
7. A module alone writes its schema and migrations. Two narrow named write
   scopes represent agreed boundaries: `EventVenueChange` coordinates Events and Booking;
   `VisitTicketIssuance` coordinates Events and Visitors. Each participant joins
   one local PostgreSQL transaction and still writes only its own tables. Source-owned validation guards may enlist read-only participants in
   an explicitly registered operation scope. No ambient transaction may silently
   enlist arbitrary modules or turn a validation read into a cross-owner write.

Compared with [uniform-layer DDD](4-ddd.md), the primary review unit here is a
module's published contract and owned tables; a small module need not introduce
repositories, aggregates and domain services. Compared with the
[single-core hexagonal option](2-hexagonal.md), there is no application-wide
business core or global collection of ports: Booking and Plans have separate
internal hexagons, while Policy stays small. All options retain one release and
the same deployment roles; none implies microservices.

## Repository root

```text
spaceweave/
├── cmd/spaceweave/          one Cobra executable; HTTP and worker roles
├── internal/               app, kernel, platform, ten modules, gateways, layout
├── pkg/                    only reusable published clients when justified
├── api/                    OpenAPI source and generation configuration
├── web/                    staff, admin, shared UI and existing floor-plan renderer
├── test/                   module assembly, acceptance, contention, recovery
├── deploy/                 Kubernetes role configuration and operational runbooks
├── tools/                  contract generation and ownership validation
├── docs/                   existing decisions and this comparison
├── go.mod                  one Go module; GOWORK=off builds
├── Makefile                generation, quality gate and acceptance entrypoints
├── .golangci.yml           sole linter; import restrictions mirror layout rules
└── .testcoverage.yml        existing total/package coverage thresholds
```

The existing repository is a tracing application, not an implemented booking
platform. [CLI](../../../internal/cli/cli.go),
[Chi server](../../../internal/server/server.go),
[catalogue](../../../internal/catalog/catalog.go) and
[worker](../../../internal/worker/worker.go) already provide useful behaviour.
The proposed tree moves that behaviour into Plans and app without inventing
existing identity, booking or PostgreSQL packages. Existing local-file catalogue
locking is not the distributed booking design.

The renderer currently uses native JavaScript modules and D3 with no build step,
as its [README](../../../web/floorplan-iso/README.md) and
[package file](../../../web/floorplan-iso/package.json) show. React/TypeScript
portal entrypoints are new work; renderer reuse is an explicit migration task.
[Repository conventions](../../../CLAUDE.md) continue to require Chi, Cobra
without Viper, strict YAML, public packages under `pkg`, internal packages under
`internal`, and the established correctness gates.

## Full package tree

This is a representative final-capability tree, not an instruction to create
empty scaffolding. Every named Go file contributes to the structural count;
one directory containing named Go files counts as one package. SQL directories,
frontend folders and parent directories without Go files do not. Unlisted unit
tests and generated expansion are not included in the totals.

Ten owners are represented: Identity, Workplace, Booking, Events, Exchange,
Visitors, Plans, Notifications, Attendance and Policy. Combined Identity,
planning inside Booking, separate Exchange, and the Events boundary are agreed.
The candidate preserves these owners using module APIs; the other four options
express the same ownership through their own layers, ports, rings or contexts.

<!-- Counted candidate tree begins. -->
```text
spaceweave/
├── cmd/
│   └── spaceweave/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── run.go
│   │   ├── modules.go
│   │   ├── roles.go
│   │   ├── router.go
│   │   ├── migrations.go
│   │   ├── shutdown.go
│   │   ├── cli/
│   │   │   ├── cli.go
│   │   │   ├── serve.go
│   │   │   ├── plans.go
│   │   │   └── migrate.go
│   │   └── config/
│   │       ├── config.go
│   │       └── validate.go
│   ├── kernel/
│   │   ├── ids.go
│   │   ├── principal.go
│   │   ├── interval.go
│   │   └── page.go
│   ├── platform/
│   │   ├── postgres/
│   │   │   ├── pool.go
│   │   │   ├── roles.go
│   │   │   └── migrate.go
│   │   ├── tx/
│   │   │   ├── runner.go
│   │   │   └── scopes.go
│   │   ├── jobs/
│   │   │   ├── queue.go
│   │   │   ├── lease.go
│   │   │   └── retry.go
│   │   ├── outbox/
│   │   │   ├── append.go
│   │   │   ├── dispatch.go
│   │   │   └── receipts.go
│   │   ├── migrations/
│   │   │   ├── embed.go
│   │   │   └── 0001_jobs_outbox.sql
│   │   ├── httpkit/
│   │   │   ├── middleware.go
│   │   │   ├── errors.go
│   │   │   └── csrf.go
│   │   ├── telemetry/
│   │   │   ├── otel.go
│   │   │   └── logging.go
│   │   ├── secrets/
│   │   │   └── references.go
│   │   ├── buildinfo/
│   │   │   └── buildinfo.go
│   │   └── testkit/
│   │       ├── postgres.go
│   │       └── clock.go
│   ├── identity/
│   │   ├── module.go
│   │   ├── api/
│   │   │   ├── access.go
│   │   │   ├── people.go
│   │   │   ├── delegation.go
│   │   │   └── events.go
│   │   ├── core/
│   │   │   ├── people.go
│   │   │   ├── authority.go
│   │   │   ├── sessions.go
│   │   │   ├── delegation.go
│   │   │   ├── lifecycle.go
│   │   │   ├── correlation.go
│   │   │   ├── scim.go
│   │   │   └── port/
│   │   │       ├── store.go
│   │   │       └── sources.go
│   │   └── adapters/
│   │       ├── http/
│   │       │   ├── login.go
│   │       │   ├── scim.go
│   │       │   └── administration.go
│   │       ├── postgres/
│   │       │   ├── store.go
│   │       │   └── migrations/
│   │       │       └── 0001_identity.sql
│   │       ├── entra/
│   │       │   └── oidc.go
│   │       ├── workday/
│   │       │   └── changes.go
│   │       └── exchangeauthority/
│   │           └── delegates.go
│   ├── workplace/
│   │   ├── module.go
│   │   ├── api/
│   │   │   ├── definitions.go
│   │   │   └── events.go
│   │   ├── definitions.go
│   │   ├── neighbourhoods.go
│   │   ├── mapping.go               publish approved geometry bindings after Booking validation
│   │   ├── calendar.go
│   │   ├── store.go
│   │   ├── http/
│   │   │   └── routes.go
│   │   ├── postgres/
│   │   │   ├── store.go
│   │   │   └── migrations/
│   │   │       └── 0001_workplace.sql
│   │   └── retirement.go
│   ├── booking/
│   │   ├── module.go
│   │   ├── api/
│   │   │   ├── claims.go
│   │   │   ├── allocation.go
│   │   │   ├── desks.go
│   │   │   ├── rooms.go
│   │   │   ├── venues.go
│   │   │   ├── operations.go
│   │   │   └── planning.go
│   │   ├── core/
│   │   │   ├── model/
│   │   │   │   ├── claim.go
│   │   │   │   ├── versions.go
│   │   │   │   └── outcome.go
│   │   │   ├── port/
│   │   │   │   ├── stores.go
│   │   │   │   ├── occupancy.go
│   │   │   │   └── solver.go
│   │   │   ├── inventory/
│   │   │   │   ├── calendar.go
│   │   │   │   ├── neighbourhoods.go
│   │   │   │   └── capacity.go
│   │   │   ├── allocation/
│   │   │   │   ├── commit.go
│   │   │   │   └── validate.go
│   │   │   ├── desks/
│   │   │   │   ├── book.go
│   │   │   │   ├── transfer.go
│   │   │   │   ├── checkin.go
│   │   │   │   └── release.go
│   │   │   ├── waitlist/
│   │   │   │   ├── priority.go
│   │   │   │   ├── promote.go
│   │   │   │   └── accept.go
│   │   │   ├── rooms/
│   │   │   │   ├── request.go
│   │   │   │   ├── apply_observation.go
│   │   │   │   └── cancel.go
│   │   │   ├── venues/
│   │   │   │   └── claims.go
│   │   │   ├── protection/
│   │   │   │   ├── outages.go
│   │   │   │   └── deadlines.go
│   │   │   ├── lifecycle/
│   │   │   │   ├── leavers.go
│   │   │   │   └── handover.go
│   │   │   └── planning/
│   │   │       ├── cycle.go
│   │   │       ├── intents.go
│   │   │       ├── groups.go
│   │   │       ├── score.go
│   │   │       └── proposals.go
│   │   └── adapters/
│   │       ├── http/
│   │       │   ├── claims.go
│   │       │   ├── operations.go
│   │       │   └── planning.go
│   │       ├── postgres/
│   │       │   ├── claims.go
│   │       │   ├── capacity.go
│   │       │   ├── venues.go
│   │       │   ├── evidence.go
│   │       │   ├── migrations/
│   │       │   │   ├── 0001_inventory_claims.sql
│   │       │   │   ├── 0002_room_requests.sql
│   │       │   │   └── 0003_planning.sql
│   │       │   └── planning.go
│   │       ├── sensors/
│   │       │   └── ingest.go
│   │       ├── jobs/
│   │       │   ├── releases.go
│   │       │   ├── leavers.go
│   │       │   └── allocation.go
│   │       ├── storetest/
│   │       │   ├── claims.go
│   │       │   └── races.go
│   │       └── solver/
│   │           └── solver.go
│   ├── events/
│   │   ├── module.go
│   │   ├── api/
│   │   │   ├── events.go
│   │   │   └── tickets.go
│   │   ├── core/
│   │   │   ├── publish.go
│   │   │   ├── audience.go
│   │   │   ├── venue.go
│   │   │   ├── issue.go
│   │   │   ├── revoke.go
│   │   │   ├── visit_link.go
│   │   │   ├── admit.go
│   │   │   └── port/
│   │   │       └── store.go
│   │   └── adapters/
│   │       ├── http/
│   │       │   ├── events.go
│   │       │   └── tickets.go
│   │       └── postgres/
│   │           ├── store.go
│   │           └── migrations/
│   │               ├── 0001_events_tickets.sql
│   │               └── 0002_visit_ticket_link.sql
│   ├── exchange/
│   │   ├── module.go
│   │   ├── api/
│   │   │   ├── reconciliation.go
│   │   │   └── events.go
│   │   ├── core/
│   │   │   ├── reconcile.go
│   │   │   ├── state.go
│   │   │   └── port/
│   │   │       ├── calendar.go
│   │   │       ├── booking.go
│   │   │       └── store.go
│   │   └── adapters/
│   │       ├── graph/
│   │       │   ├── client.go
│   │       │   ├── subscriptions.go
│   │       │   └── delta.go
│   │       ├── http/
│   │       │   └── callbacks.go
│   │       ├── postgres/
│   │       │   ├── store.go
│   │       │   └── migrations/
│   │       │       └── 0001_exchange.sql
│   │       └── jobs/
│   │           └── reconcile.go
│   ├── visitors/
│   │   ├── module.go
│   │   ├── api/
│   │   │   ├── visits.go
│   │   │   └── ticket_visits.go
│   │   ├── visits.go
│   │   ├── verification.go
│   │   ├── restrictions.go
│   │   ├── reception.go
│   │   ├── retention.go
│   │   ├── http/
│   │   │   └── routes.go
│   │   └── postgres/
│   │       ├── store.go
│   │       └── migrations/
│   │           └── 0001_visitors.sql
│   ├── plans/
│   │   ├── module.go
│   │   ├── api/
│   │   │   └── revisions.go
│   │   ├── core/
│   │   │   ├── revisions.go
│   │   │   ├── publication.go
│   │   │   ├── recovery.go
│   │   │   ├── retention.go
│   │   │   └── port/
│   │   │       ├── blobs.go
│   │   │       └── store.go
│   │   ├── processing/
│   │   │   ├── convert/
│   │   │   │   └── convert.go
│   │   │   ├── cad/
│   │   │   │   └── cad.go
│   │   │   ├── dwg/
│   │   │   │   └── dwg.go
│   │   │   ├── dxf/
│   │   │   │   ├── dxf.go
│   │   │   │   └── dxftest/
│   │   │   │       └── office.go
│   │   │   ├── svgdraw/
│   │   │   │   └── svgdraw.go
│   │   │   ├── vector/
│   │   │   │   └── vector.go
│   │   │   ├── raster/
│   │   │   │   └── raster.go
│   │   │   ├── plan/
│   │   │   │   └── validate.go
│   │   │   ├── floorgraph/
│   │   │   │   └── build.go
│   │   │   ├── autotrace/
│   │   │   │   ├── autotrace.go
│   │   │   │   ├── profile/
│   │   │   │   │   └── profile.go
│   │   │   │   └── score/
│   │   │   │       └── score.go
│   │   │   └── trace/
│   │   │       └── agent.go
│   │   └── adapters/
│   │       ├── http/
│   │       │   └── revisions.go
│   │       ├── postgres/
│   │       │   ├── store.go
│   │       │   └── migrations/
│   │       │       └── 0001_plans.sql
│   │       ├── azure/
│   │       │   └── blobs.go
│   │       ├── gcs/
│   │       │   └── blobs.go
│   │       ├── jobs/
│   │       │   └── processing.go
│   │       ├── localimport/
│   │       │   └── catalogue.go
│   │       └── blobtest/
│   │           └── contract.go
│   ├── notifications/
│   │   ├── module.go
│   │   ├── api/
│   │   │   └── notifications.go
│   │   ├── preferences.go
│   │   ├── catalogue.go
│   │   ├── subscribe.go
│   │   ├── delivery.go
│   │   ├── retention.go
│   │   ├── http/
│   │   │   └── routes.go
│   │   ├── postgres/
│   │   │   ├── store.go
│   │   │   └── migrations/
│   │   │       └── 0001_notifications.sql
│   │   ├── email/
│   │   │   └── sender.go
│   │   └── teams/
│   │       ├── personal.go
│   │       └── installation.go
│   ├── attendance/
│   │   ├── module.go
│   │   ├── api/
│   │   │   └── reports.go
│   │   ├── projection.go
│   │   ├── provenance.go
│   │   ├── reports.go
│   │   ├── retention.go
│   │   ├── http/
│   │   │   └── routes.go
│   │   └── postgres/
│   │       ├── store.go
│   │       └── migrations/
│   │           └── 0001_attendance.sql
│   ├── policy/
│   │   ├── module.go
│   │   ├── api/
│   │   │   └── policies.go
│   │   ├── versions.go
│   │   ├── validate.go
│   │   ├── overrides.go
│   │   ├── http/
│   │   │   └── routes.go
│   │   └── postgres/
│   │       ├── store.go
│   │       └── migrations/
│   │           └── 0001_policy.sql
│   ├── gateway/
│   │   ├── portal/
│   │   │   ├── routes.go
│   │   │   ├── dashboard.go
│   │   │   └── assets.go
│   │   └── operations/
│   │       ├── queues.go
│   │       ├── retention.go
│   │       └── recovery.go
│   ├── transport/
│   │   └── gen/
│   │       └── rest/
│   │           └── server.gen.go
│   └── layout/
│       ├── imports_test.go
│       ├── sql_ownership_test.go
│       └── transactions_test.go
├── pkg/
│   └── client/
│       └── rest/
│           └── client.gen.go
├── api/
│   ├── openapi.yaml
│   ├── booking.yaml
│   ├── identity.yaml
│   ├── workplace.yaml
│   ├── plans.yaml
│   ├── visitors.yaml
│   ├── operations.yaml
│   └── generation.yaml
├── web/
│   ├── package.json
│   ├── staff/
│   │   ├── index.html
│   │   └── src/
│   │       ├── main.tsx
│   │       ├── AttendanceDates.tsx
│   │       └── BookingStatus.tsx
│   ├── admin/
│   │   ├── index.html
│   │   └── src/
│   │       ├── main.tsx
│   │       └── EventCapacity.tsx
│   ├── shared/
│   │   ├── FloorPlan.tsx
│   │   ├── api.ts
│   │   └── session.ts
│   ├── generated/
│   │   ├── transport.ts
│   │   └── client.ts
│   └── floorplan-iso/
│       ├── index.html
│       ├── js/
│       │   ├── render.js
│       │   ├── projection.js
│       │   ├── geometry.js
│       │   └── selection.js
│       └── test/
│           └── render.test.js
├── test/
│   ├── assembly/
│   │   └── modules_test.go
│   ├── acceptance/
│   │   ├── booking_test.go
│   │   ├── delegation_test.go
│   │   └── provisioning_test.go
│   ├── contention/
│   │   ├── allocations_test.go
│   │   ├── checkin_test.go
│   │   └── tickets_test.go
│   ├── recovery/
│   │   ├── restart_test.go
│   │   └── region_test.go
│   └── support/
│       └── application.go
├── deploy/
│   ├── kubernetes/
│   │   ├── http.yaml
│   │   ├── allocation.yaml
│   │   ├── exchange.yaml
│   │   ├── tracing.yaml
│   │   ├── maintenance.yaml
│   │   ├── identities.yaml
│   │   └── collector.yaml
│   └── runbooks/
│       ├── region-failover.md
│       └── retention.md
└── tools/
    ├── generate-contracts.sh
    └── check-ownership.sh
```
<!-- Counted candidate tree ends. -->

The processing directories retain their existing subsidiary files and tests as
they migrate; the representative entry per package shows the intended owner.
The current `dwg/dwgtest` helper would remain under Plans too, although it is not
expanded in this representative tree. Likewise, the printed generated files
stand for generator outputs, not handwritten domain models.

## API surfaces and consumers

The [shared surface catalogue](../api-surfaces/README.md) defines the consumers:
UI, MCP servers, SDKs, scripts, contract tests and Terraform consume supported
public REST. Workers consume private interfaces. Internet-facing user access is
protected by Conditional Access and application permissions; visitor and provider
profiles keep their appropriate identity mechanisms.

| Consumer/surface | Contract kind | Binding in this ten-module candidate |
| --- | --- | --- |
| UI, MCP, SDK, scripts, contract tests and Terraform | Shared supported public REST operations with scoped projections. | Module HTTP adapters/portal gateway call module `api` interfaces. Separate staff/admin experiences do not require private business APIs. |
| Human and application callers | Session/CSRF or selected API-token profile; real actor/client/owner and distinct grants. | Identity plus owning command authorisation; shared module ownership does not merge token audiences or grant machine impersonation. |
| Internal-event visitor, including guest speaker | Internet invitation/verification and restricted visitor/event/ticket REST. | Visitors handles visit scope; Events owns event audience/tickets/admission. No general anonymous event catalogue follows from guest access. |
| Entra SCIM and Graph callbacks | Native authenticated provisioning/provider protocols. | Identity owns SCIM; Exchange owns provider observations/reconciliation. Neither is a generic public mutation backdoor. |
| Teams and optional occupancy clients | Restricted provider/source protocols. | Notifications handles personal delivery support; Booking owns transition evidence and Attendance reports it. Installation/occupancy does not establish message consent or personal check-in. |
| Platform workers/private collaborators | Private Go APIs and versioned jobs/facts. | App composes direct module calls; private transaction/source-validation participants cannot be supplied by public clients. |
| Operational tooling | Private probes/metrics/telemetry plus explicitly authorised public status/repair commands. | Platform routing stays restricted; public repairs invoke owning module interfaces without exposing stores or raw event payloads. |

Booking serves staff availability and room workflow state; Events serves organiser
event/ticket management and scoped ticket access. Authority, field visibility and operation
contracts are independent of module depth and the caller's SDK/tool label.

Generated Go/TypeScript clients target the supported public OpenAPI contract;
module `api` packages remain project-private Go interfaces rather than SDK types.
The UI and automation share semantics while their credential profiles can differ.
Private worker operations, repositories and named write/read-validation
scopes are not public endpoints. Distinct human/application authority, initial
configuration-only Terraform management, independent public API versions and
follow-on MCP/provider delivery are agreed. Identity owns configurable RBAC;
MCP is user-context and machine-identity scripts use another channel. UI drift
is allowed and initial retirement notice is 180 days. Global administration,
SCIM-backed role mappings, grant-only scoped composition and self-permission
discovery are agreed. The [agreed request contract](../api-surfaces/request-behaviour.md)
keeps command receipts with Booking or the other owning module's commit, retaining
the named venue and visitor-link scopes. Previous-version configuration edits, two request
classes, actor budgets and public operation status are agreed; wire/retention
details remain open. Worker execution is bounded separately. Surface
bindings preserve the agreed owners; repository-architecture selection, concrete
package placement and concrete publication-guard mechanics stay under review.

## Deep dives

### Published interfaces and module construction

Each `module.go` constructs private services and exposes a small bundle of API
interfaces plus route/job registrations for app. Other modules receive the
interfaces they use, never a `*Module` that gives convenient access to every
internal service. Construction errors identify the module and unwind resources.
Module shutdown stops accepting work, drains bounded handlers and closes only
resources it owns; a shared PostgreSQL pool belongs to app.

| Published contract | Important operations and ownership |
| --- | --- |
| `identity/api.Access` | Resolve session; require capability with office scope; verify actor/owner delegation and source freshness. |
| `identity/api.People` | Read verified staff snapshots and effective team/office facts; receive lifecycle events. |
| `identity/api.Delegation` | Evaluate app-managed grants and fresh native delegation evidence through Identity-owned authority adapters. |
| `workplace/api.Definitions` | Read/version definitions, retire/reactivate resources and publish approved geometry mappings after Booking validates dated impact. |
| `booking/api.Claims` | Query authoritative availability; create/cancel claims; transfer desk and linked parking. |
| `booking/api.Allocations` | Read planning snapshot; commit one group with expected inputs; return precise stale/conflict causes. |
| `booking/api.Desks` | Personal check-in, eligible waitlist submission/acceptance, validated operational evidence ingestion and protected expiry; commit facts for Attendance projections. |
| `booking/api.Rooms` | Commit business room intentions/holds, expose desired revisions, and apply provider observations through capacity-safe booking transitions. |
| `exchange/api.Reconciliation` | Durably accept provider observations, reconcile committed room intentions and expose integration progress; no claim writes. |
| `booking/api.Venues` | Validate and mutate event-space claims as Booking's participant in the named venue workflow; no event/ticket table access. |
| `events/api.Events` and `Tickets` | Publish/edit event details, audience and registration; coordinate venue changes; issue/revoke tickets and admit attendees using Events-owned state. |
| `booking/api.Planning` | Manage attendance dates, travel exceptions, cycles, groups, proposals and run status. |
| `visitors/api.TicketVisits` | Ensure a verified restricted visit for a ticket operation; participates only in the named issuance transaction. |
| `plans/api.Revisions` | Upload, process, review, publish and retrieve immutable revision metadata and authorised asset access. |
| `notifications/api.Notifications` | Read in-app messages and update message-specific delivery preferences. |
| `attendance/api.Reports` | Read provenance-labelled attendance and occupancy reports under configured scope/retention. |
| `policy/api.Policies` | Read effective immutable policy versions; validate and publish permitted office overrides. |

Command inputs carry request ID, actor, owner where relevant, office and expected
version. Domain APIs use meaningful failure categories such as `Unavailable`,
`StaleProposal`, `AuthorityStale`, `OfferExpired` and `HandoverRequired`; they do
not return HTTP status objects. HTTP maps those outcomes into transport errors.
The API packages publish event payloads without importing the platform outbox;
app provides serializers and producer topic registration.

The kernel principal states the authenticated subject and session context. It
does not embed an unbounded list of offices or permanently cached permissions.
A module obtains current authorisation for each operation and passes the
decision into its private service. Administrative capability is evaluated
together with the operation's offices; global integration, retention and policy
administration require explicitly global capability. A staff token is not a
SCIM credential and a provider application token is not a human delegation.

### Booking's internal hexagon and transaction boundary

Booking privately owns intent collection, cycles/groups, planning and claims.
Its solver interface accepts versioned snapshots and returns proposals; the
implementation runs outside long transactions and may use a separate allocation
process. Core scoring expresses the agreed business priorities, while the solver
implementation remains replaceable. Package separation inside Booking does not
create a second owner or permission to bypass its capacity writer.

`CommitGroup` rechecks current local intent/group/cycle versions, hard requirements,
bundles, owner overlap, required parking and waitlist priority, then commits every
member or none. Confirmed groups remain protected; only affected uncommitted
groups are replanned. Group receipts, claim changes, essential run/proposal
progress and outbox facts share the Booking transaction; derived status views
can lag without creating a cross-module Planning authority.

`TransferDesk` acquires replacement desk/required parking and retires previous
claims in one transaction, checking the final one-desk-per-owner invariant.
Rollback preserves the previous arrangement when replacement capacity fails.
The [allocation](../../adr/0014-use-explicit-allocation-cycles-and-atomic-group-placement.md)
and [dated-inventory](../../adr/0013-separate-workplace-definitions-from-dated-booking-inventory.md)
decisions apply to direct and background callers alike.

External source validation still matters. Identity supplies current eligibility
and default versions; Workplace supplies effective definitions, including
retirement; Policy supplies applicable versions and activation state. In a
registered read-validation scope, their narrow published participants retain
compatible guards through Booking's commit without exposing repositories.
Intent/group/cycle guards are local Booking operations, not another owner's API.
An unlocked precheck cannot protect against retirement, disablement, intent
cancellation or policy activation racing the claim.

Source writers take compatible guards and never call back into Booking while
holding them. Use a measured global resource/key order; retain stable guards
when versions disappear. Ordinary remote/provider calls stay outside the short
claim transaction. Guard granularity and SQL-role mechanics remain engineering
validation, rather than guarantees supplied by a folder layout.

Cancellation stops before another group commit and preserves earlier placements.
A later change to an already committed intent uses the authorised Booking change
workflow; removing an intent alone cannot silently erase its claims.



Specific resources use non-overlap constraints over active claim intervals.
Transfers check owner overlap on the final transaction state, using a deferrable
exclusion constraint or an equivalent validated statement order. An immediate
owner constraint must not reject the replacement while the old row remains
active. Pooled parking serialises on pool/local-date guards and checks the
endpoint-based concurrent total across relevant intervals; a simple count of
overlapping rows miscounts staggered bookings. Every claim, cancel, transfer,
expiry and capacity edit follows the same stable guards and lock order. Locks
on existing claims alone cannot protect an empty interval. PostgreSQL documents
PostgreSQL documents [range exclusion](https://www.postgresql.org/docs/current/rangetypes.html#RANGETYPES-CONSTRAINT)
and [row-lock behaviour](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS).

The proposed order is office/date protection guard, sorted resource/pool guards,
sorted owner guards, then claim/offer/event rows. Operations spanning offices
sort office IDs too. Guard granularity is a measured choice: per-office locking
is simple but risks allocation bursts blocking check-in; per-resource guards
need a consistent way to cover pooled intervals and no-existing-row cases.
The implementation must establish one concrete order in database tests before
accepting the performance claim. It cannot combine incompatible orders merely
because the calling packages are separate.

Use bounded retries for serialization/deadlock failures, rerunning the whole
decision. An exclusion conflict can mean genuine unavailability rather than a
retryable infrastructure fault. A request idempotency key identifies the logical
operation, so an ambiguous client retry discovers its committed result rather
than allocating twice. These are design choices grounded in PostgreSQL's
[transaction retry guidance](https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html),
not an exactly-once claim.

### Owned schemas, migrations and SQL access

All tables below are proposed, not existing storage. A schema name is an
ownership label; sharing one database does not grant arbitrary cross-schema SQL.
Platform migrations install required shared extensions and mechanisms; module
migrations install their own tables, indexes and constraints.

| Owner/schema | Representative tables | Migration responsibility |
| --- | --- | --- |
| Platform | `jobs`, `job_attempts`, `leases`, `outbox`, `consumer_receipts`, `schema_versions` | Durable mechanisms, not booking state or staff models. |
| Identity | `staff`, `source_links`, `source_checkpoints`, `workforce_facts`, `groups`, `memberships`, `sessions`, `capabilities`, `delegation_grants`, `scim_resources` | Correlation, source authority, access and lifecycle in one owned transaction. |
| Workplace | `offices`, `buildings`, `floors`, `space_definitions`, `office_calendars`, `baseline_neighbourhoods`, `definition_versions`, `plan_mappings`, `lifecycle_schedules` | Definitions and operational mapping publication; no dated active claim writes. |
| Booking | `inventory_days`, `dated_bundles`, `resource_guards`, `owner_guards`, `claims`, `parking_segments`, `waitlist_entries`, `offers`, `checkins`, `occupancy_evidence`, `protection_windows` | All claim/timeout transitions and validated operational occupancy evidence; commit facts for Attendance. |
| Booking, continued | `room_requests`, `handover_reviews`, `cycles`, `attendance_intents`, `travel_exceptions`, `collaboration_requests`, `allocation_groups`, `runs`, `proposals` | Business room intentions plus planning, demand and group progress under the same claim owner. |
| Events | `events`, `event_audiences`, `tickets`, `admissions` | Event lifecycle, registration, ticket capacity and admission, coordinated with Booking's venue state. |
| Exchange | `exchange_links`, `provider_observations`, `provider_desired_state`, `sync_cursors`, `subscriptions`, `reconciliation_progress` | Provider correlation, desired/observed calendar state and reconciliation; apply business transitions through Booking. |
| Visitors | `visitors`, `email_proofs`, `visits`, `visit_restrictions`, `reception_entries` | Verified email, hosts, access windows and visitor-specific retention. |
| Plans | `uploads`, `plan_revisions`, `render_revisions`, `asset_copies`, `publication_pointers`, `processing_attempts` | Metadata and publication; blob objects remain outside PostgreSQL transactions. |
| Notifications | `preferences`, `messages`, `delivery_attempts`, `teams_installations` | Recipient-specific delivery and suppression; no change to booking success. |
| Attendance | `evidence_projection`, `daily_reports`, `projection_checkpoints`, `retention_runs` | Derived evidence with provenance; no write-back to claim truth. |
| Policy | `policy_versions`, `office_overrides`, `publication_audit` | Runtime policy publication, distinct from strict YAML deployment settings. |

Each migration stream has a module ID and applied version, and startup refuses
an incompatible schema. App runs the platform stream before module streams in
declared dependency order; it does not use alphabetical discovery. The migration
for Events' visit link explicitly depends on Visitors' first stream. Schema
changes that affect cross-owner contracts are reviewed together, with rollback
and mixed-version behaviour documented.

Cross-module foreign keys are restricted to explicit lifecycle cases. The
initial allowlist contains only `events.tickets.visit_id` referencing
`visitors.visits.id`, nullable for staff tickets, with restrictive deletion and
no cascade. It protects the selected shared issuance boundary. Other references
use stable IDs, versioned facts and controlled tombstones until a specific
foreign key has a documented lifecycle case. A restrictive FK is neither a
permission to query the other table nor a retention policy.

The restrictive FK cannot extend the configured visit retention period. Visitors
marks a retention-due visit under its guard and refuses new ticket linkage.
An owned staged cleanup tells Events to remove identifying ticket copies and
detach the optional link, then lets Visitors purge its details. Each step is
idempotent and commits in its owner's transaction, with progress, deadlines and
failure alerts; an indefinite referential hold is not acceptable. Any retained
pseudonymous key needs an explicit purpose and bounded lifetime; hash-shaped
identifiers are not automatically anonymous. Staff deactivation never means
cascading deletion of historical bookings.

Database roles strengthen the boundary: module SQL adapters receive module
connections able to write only that schema and approved platform append/job
functions. Exchange connections cannot `UPDATE booking.claims`. Allocation runtime grants
remain narrowly scoped even though its code belongs to Booking. Source-owned
locking reads use narrowly granted functions with fixed queries and controlled
execution/search paths, rather than granting Booking broad read/update rights
on source tables simply to acquire validation locks. Those functions run on
Booking's transaction connection through the source API's private adapter;
the API exposes no raw connection. A specially scoped
transaction connection permits only the registered owners: Events and Booking
for `EventVenueChange`, or Events and Visitors for `VisitTicketIssuance`.
Booking's venue-validation guard can join issuance read-only through its API.
Code can still misuse privilege, so CI checks call sites and live role tests
reject out-of-scope SQL. PostgreSQL grants are
real enforcement, while the operation name and allowed participants are
application enforcement; neither should be presented as sufficient alone.

### Events, venue consistency and visitor tickets

`events/core` owns title, organiser, audience, registration access/window,
selected event capacity, named tickets and admission. Booking owns physical
venue claims. Publication and ticket issuance lock the Events-owned event
version; the effective venue is validated through Booking's published API in
the same transaction. Capacity may never exceed the secured venue, and active
issued tickets may never exceed the selected event capacity.

`EventVenueChange` is a narrow proposed write scope: Events coordinates its
event reference/limit with Booking's old/new venue claims. Each owner's private
adapter performs only its own writes through the opaque scope. A too-small
replacement fails or enters an explicit organiser resolution workflow; it
never quietly invalidates tickets. Ticket issuance takes compatible Booking
venue guards and the Events event guard, so it sees either the old secured
venue or the committed replacement. Generic Booking edits cannot bypass this
workflow. The shared transaction has a documented global lock order and cannot
expose foreign repositories or become an all-module unit of work.

Events also checks current audience and registration rules inside its guarded
operation. Staff identity and visitor verification remain separately owned
facts, validated through published interfaces rather than browser-supplied flags.

A visitor ticket must reference a valid visit with verified email, host, office
and compatible time window. `VisitTicketIssuance` joins Events and Visitors in
one transaction: Events starts the named scope, validates secured venue and
event eligibility, asks `visitors/api.TicketVisits.Ensure` to validate/create
the restricted visit, then inserts its ticket. Both owners commit their writes
and outbox records together. A failed capacity or visit check rolls back a newly
created registration visit and its ticket, retaining the required definitive
refusal receipt without either new business record. An existing valid reused
visit survives failed issuance.

The runner permits only those registered writers; Booking's venue guard joins
read-only. Lock order covers sorted physical-resource guards, the Events event
guard and then visitor/visit guards. Visitors never calls back into Events or
Booking while holding visit locks. Email verification precedes the transaction;
invitation delivery follows commit. A repeated request recovers the original
linked ticket/visit through the shared retry identity.

Ticket scans lock Events' ticket and create one unique Events admission record.
A duplicate scan returns the prior outcome and only reception-authorised detail.
The venue-recovery pause, visit restrictions, ticket revocation and event time
are rechecked; a concurrent
restriction change uses Visitors' published locking validation in the registered
read-enlisted scope, retained through Events' admission commit. Identity
validation follows the same short current-fact protocol. These are explicit
read-consistency couplings, not permission to write another owner's tables.

Confirmed venue loss keeps ticket entitlements pending recovery and pauses
admission and further issuance. Events requires an explicit organiser decision
to secure a suitable replacement or cancel, and communicates confirmed changes
to attendees. Prior admission records are retained.

Events also owns attendee reconfirmation, unused-ticket cancellation and optional
event waitlists under the [event workflow contract](../api-surfaces/event-workflows.md).
Its admission guard checks required reconfirmation and visitor eligibility;
later visitor ineligibility retains the ticket/seat pending repair or explicit
cancellation. Office/time changes revalidate linked visit hosts/windows. Event
cancellation asks Visitors to cancel visits whose only remaining valid purpose
was that event, while preserving other-purpose visits and reception history.
Events selects eligible requests in join order and holds offered seats exclusively.
Decline/expiry ends that request; explicit rejoin creates a new one. Acceptance
ends at registration closing time unless the organiser explicitly extends it.
Published reconfirmation deadlines release unconfirmed tickets. Unused-ticket
cancellation also cancels a future visit with no other valid purpose. Confirmed
outages pause reconfirmation; venue recovery preserves offers and pauses expiry
within registration limits. Temporary queue ineligibility preserves position
without capacity, while office/time changes withdraw old offers without ending
requests. Initial numeric policy defaults remain unselected; the desk waitlist
policy is separate.

### Desk use, waitlists, outage protection and reporting

Booking distinguishes allocated, personally checked in, released and expired
claims. `identity/api.Access` verifies the current actor; `Desks.CheckIn`
requires actor equal to owner even when that actor holds a valid PA delegation.
Anonymous occupancy says a desk looks occupied, not that its owner attended.
Office-presence evidence likewise does not establish use of a particular desk.
These distinctions follow the [check-in decision](../../adr/0003-require-assignee-check-in-to-mark-a-desk-taken.md)
and [glossary](../../../GLOSSARY.md#attendance-and-desk-use).

The release worker checks current claim state, policy version, authoritative
time and protection windows inside the same transaction as release/promotion.
A personal check-in winning that lock prevents no-show release. A completed
release winning first makes a later check-in stale; a desk already assigned to
someone else cannot be reclaimed by a delayed action. Vacancy release applies
only after personal check-in, with fresh sustained vacancy signals; absent or
stale sensor data means unknown. Sensor ingestion is optional.

Eligible waitlisted requests precede fresh direct requests. Returning capacity
and creating its next offer occur under the same resource/pool guards; the
worker cannot release, notify asynchronously and let a fresh caller bypass
priority. Offer acceptance atomically consumes the active offer and claim,
including both-required parking. An offer without suitable linked capacity is
not eligible. Fairness decisions are recorded so operators can explain them.

Booking owns confirmed app-outage protection records because those records
change its timeout decisions. An authorised operator action can target affected
offices and operations, with start/end and recovery-grace policy. Expiry reads
that state under the protection guard. A telemetry gap alone does not infer a
confirmed outage, and an outage cannot extend a booking beyond its useful
window. Existing reassignments stay protected. See
[outage policy](../../policy-defaults.md#outage-protection).

Attendance consumes committed, versioned evidence events and keeps source,
confidence and time separately for bookings, personal check-ins and occupancy.
It reports attendance dates without minimum attendance or anchor-day compliance
settings. Booking owns explicit travel choices; division anchor days remain
context. Reporting lag is displayed and monitored. Attendance never writes a
derived "present" flag into Identity or uses a desk allocation as proof of
personal attendance.

Visitor, occupancy and attendance retention are independent configured rules.
They cover owning tables, report projections, identifiable outbox payloads,
notification copies, diagnostic exports and recovery procedures. Prefer events
with stable references and minimal personal data; purge/encrypt-and-expire
payloads where necessary. Retain only the minimal deduplication evidence needed
within the supported recovery horizon and applicable deletion rules. Backups
need a documented restore-and-reapply retention process.

### Exchange module, authority and graceful synchronisation

Booking owns business room intentions, pending holds, claim deadlines and booking
transitions. Exchange owns Graph requests, provider mappings, desired/observed
calendar state, merge bases, subscriptions, delta progress and integration retries.
Its handlers use Booking's published room API for every capacity/state transition.
Identity's separate Exchange-authority adapter reads fresh delegation facts,
so "may this PA act?" is not confused with "did this room accept?".

Raising a room request records owner/organiser, acting booker, authority source,
verification time and a local pending capacity hold, plus durable provider work.
App-managed grants permit app-mediated actions without granting native Outlook
access. Exchange-derived grants require verification within the configurable
fifteen-minute age limit. Stale source authority is refreshed before a new
dependent action; failure cannot silently become an app-managed grant. A valid
separate app grant can still authorise the action. These are
[selected delegation rules](../../adr/0010-keep-app-managed-delegation-local.md).

Provider application permission and human delegation are distinct. Delegated
creation can retain the owner's organiser identity, while the app separately
preserves the PA actor. Scoped Exchange application RBAC must be evaluated with
any independent Entra application grants, which are additive. A successful
app-only Graph call proves neither the PA's authority nor a room's approval.
[Delegated calendar creation](https://learn.microsoft.com/en-us/graph/outlook-create-event-in-shared-delegated-calendar),
[application RBAC](https://learn.microsoft.com/en-us/exchange/permissions-exo/application-rbac).

Booking commits a versioned desired-change fact with its room intention/hold.
Exchange consumes it idempotently in its own transaction and drives provider
work after local acceptance. Provider observations enter its owned
a durable inbox and are reconciled by reading the current event when required;
arrival order does not define truth. Keep local desired version, observed
provider version, last merged base, pending operation key and freshness. Merge
compatible independent edits; revalidate room/time changes; expose genuinely
incompatible intentions for review. Native changes can update local claims only
through Booking's same availability path. Booking applies each observation
idempotently; Exchange checkpoints after the owning result. A crash can leave
integration progress behind, but cannot create a second claim writer. Pending holds expire under configured
room workflow rules, including explicitly longer approval-based deadlines.

Graph `changeKey` identifies event versions; it is not evidence here of an
event-specific compare-and-swap guarantee. Creation `transactionId` helps avoid
redundant retry creation, but reconciliation still handles ambiguous outcomes.
No design promise says a room swap preserves the old booking until a new room
accepts. Provider cancellation and deleting a room's calendar copy are distinct
operations. [Event resource](https://learn.microsoft.com/en-us/graph/api/resources/event?view=graph-rest-1.0),
[event update](https://learn.microsoft.com/en-us/graph/api/event-update?view=graph-rest-1.0),
[organiser cancellation](https://learn.microsoft.com/en-us/graph/api/event-cancel?view=graph-rest-1.0).

Notifications are wake-up hints; durable delta reconciliation runs every five
minutes while healthy and records actual age. Calendar-view delta is scoped to
a calendar and fixed date window, so windows/checkpoints must be maintained and
renewed deliberately. Resubscription, missed notifications, token invalidation,
provider throttling and access loss each have visible retry/recovery states.
Exchange downtime leaves app requests pending and other space workflows serving.
[Calendar-view delta](https://learn.microsoft.com/en-us/graph/delta-query-events),
[notification recovery](https://learn.microsoft.com/en-us/graph/change-notifications-lifecycle-events).

### Identity, provisioning and effective lifecycle

Identity owns stable internal staff IDs, verified source links, source-specific
facts, sessions, capabilities and delegation grants. Entra owns account security
and provisioning state; Workday owns workforce lifecycle, structure and usual
office; SpaceWeave owns preferences and dated travel choices. The same module
can atomically record an observed effective disable and invalidate sessions,
avoiding an asynchronous projection that leaves known-disabled staff authorised.
Future-effective changes are scheduled and evaluated against current time.

The SCIM HTTP adapter implements authenticated `/Users` and explicitly mapped
`/Groups` operations, filtering/paging and the supported PATCH profile. Create,
update, deactivate, reactivate and delete are actual provisioning operations;
SSO login and Graph directory reads are not substitutes. A deleted SCIM resource
disappears from that interface while retained internal booking history remains
subject to its own purpose. RFCs distinguish `active`, stable server IDs and
provisioning-domain `externalId`; email alone is not a permanent correlation key.
[SCIM schema](https://www.rfc-editor.org/rfc/rfc7643.html#section-3.1),
[SCIM protocol](https://www.rfc-editor.org/rfc/rfc7644.html#section-3),
[Entra provisioning profile](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups).

A Workday adapter maps the tenant's approved workforce transport into source
facts and effective dates. This document does not assert that Workday sends
SCIM to SpaceWeave. Approved staff identifiers and explicit mappings correlate
records; ambiguous links are held for review. Entra tenant/object IDs identify
that source, while email remains mutable contact data. Workday integration
permissions and field visibility require tenant validation rather than assuming
a generic feed contains every organisational relationship.
[Workday integration reference](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/workday-integration-reference),
[selected source authority](../../adr/0018-correlate-staff-with-explicit-source-authority.md).

Each source keeps its own checkpoint, observed time and health. A partial source
outage preserves known facts and does not convert missing rows into leavers.
Security deactivation takes effect when a confirmed effective instruction is
observed; no architecture can promise instantaneous offboarding before the
source delivers it. Booking receives durable lifecycle facts. An effective
leaver instruction releases future personal desk/parking claims retry-safely and
opens room/event handover reviews. Account disable revokes access; it must not
invent a Workday leaver fact. Cleanup may lag access revocation; the lag is a
metric and task state. In-place Exchange organiser transfer is not assumed.

Office/team changes revise unconfirmed derived planning defaults. They preserve
explicit travel, current confirmed claims and historical organisational facts.
Booking's allocation validation checks relevant current eligibility; it does not
silently move every confirmation to the new usual office. Notification tasks
explain existing arrangements needing review. App-managed and Exchange-derived
delegation grants retain separate source and revocation lifecycles.

### Plans, blob recovery and the existing processing pipeline

Plans owns the full drawing-to-approved-revision lifecycle. Its processing
packages retain the existing CAD, vector, raster, floorgraph, automatic tracing
and optional agent tracing algorithms. `localimport` is an explicit migration
adapter for existing `catalog.json` packages, not the production source of
distributed job ownership. Scratch directories are per attempt and disposable.
The local [conversion path](../../../internal/convert/convert.go) and
[worker publication path](../../../internal/worker/worker.go) supply behaviour
to preserve, including reports, review feedback and draft status.

The blob port and Azure/GCS adapters stay inside Plans because Plans is the
only business owner of drawing/render assets. A later second independent asset
owner could justify extracting a neutral byte-store mechanism; it would not
justify sharing revision publication rules. Deployment configuration chooses
the active provider. The port offers immutable put/read, provider-opaque version
tokens, scoped access and verified-copy operations, without importing either
provider's object types into core.

Upload bytes and conversion happen outside the metadata transaction. New
revisions use unique immutable object names and content digests; provider tokens
are opaque concurrency values, not portable content checksums. Azure ETags and
GCS generation preconditions have different semantics. Publication verifies the
recovery copy and manifest, then commits revision metadata, the asset pointer and
outbox record. This approves/publishes an asset; Workplace separately owns
`mapping.go` and the active geometry-to-stable-space mapping. Booking validates
dated-inventory and commitment impact before Workplace activates that mapping.
One admin workflow can coordinate both steps when validation succeeds, exposing
an approved-but-not-active result if activation fails. Until verification succeeds, status remains processing or
awaiting recovery protection and no durable/published acknowledgement is issued.
[Azure concurrency](https://learn.microsoft.com/en-us/azure/storage/blobs/concurrency-manage),
[GCS preconditions](https://docs.cloud.google.com/storage/docs/request-preconditions).

No PostgreSQL transaction spans blob operations. Orphan object cleanup,
interrupted copy verification and pointer recovery are durable reconciliation
jobs with ownership and attempt keys. A stale processing attempt cannot publish
over a newer revision because its lease/version predicate is checked on the
metadata commit. Signed asset access follows backend office authorisation and
expires quickly; obtaining a session never gives unrestricted object access.
GCS documents atomic object operations, not a transaction with PostgreSQL.
[GCS consistency](https://docs.cloud.google.com/storage/docs/consistency),
[revisioned-storage decision](../../adr/0012-use-configurable-revisioned-blob-storage.md).

Verified recovery copies are required before acknowledging protected assets;
default asynchronous geo-replication is insufficient evidence for the five-minute
whole-system RPO. Recovery inventories include exact revision IDs, copy digests,
metadata checkpoints, policy versions and historical references. Retention never
deletes a historical revision still needed by an allowed reference merely
because it is no longer the current pointer. A complete regional exercise must
restore a metadata-consistent set of published bytes.

### Durable work, delivery and the private Teams adapter

Platform provides one durable PostgreSQL queue and outbox mechanism. Each module
registers its own job kinds and event handlers; the mechanism does not decide
room acceptance, visitor eligibility or notification preferences. Booking writes
claim transitions and their work/outbox records in the same transaction. A
process crash after commit cannot lose the instruction simply because an
in-memory signal was missed. Polling remains the durable fallback.

Jobs claim with leases and attempt/fencing tokens. A stale worker may have made
an external call, so fencing guards subsequent local writes while reconciliation
resolves the provider outcome. Use `SKIP LOCKED` for competing queue consumers,
not for answering booking availability: PostgreSQL explicitly warns that it
gives an inconsistent view unsuitable for general queries.
[Queue-lock caveat](https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE).
Retry uses bounded backoff and classified errors; poison work becomes visible
with safe replay controls. Consumer receipts deduplicate per consumer/event ID;
failed delivery does not advance successful completion for that consumer.

Outbox payloads carry producer ID, event ID, aggregate/version, causation and
minimal references. No global sequence is treated as business order across
concurrently committed transactions. Consumers ignore obsolete aggregate
versions, detect missing prerequisites and reread the owning API when needed.
A receipt and local consumer state commit together. External effects can repeat
after a crash between remote success and local recording; delivery is retryable,
not exactly once. Audit and retention cover both payloads and failure details.

Notifications has real business ownership: preference resolution, recipient
selection, templates, in-app inbox, retries and suppression. A booking-raised
event may create email/in-app delivery according to configured preferences and
one optional private Teams task. It never turns a failed optional message into
a failed booking or sends a message to a shared channel.

The Teams adapter uses supported proactive personal messaging through an
installed Teams app/bot or agent. Installation/conversation readiness and the
individual's opt-in to this particular booking-raised message are separate
checks. Default is not opted in, and consent for this message enables no other
Teams message type. Recheck the current preference immediately before an
attempt; withdrawal suppresses pending unsent work. A send already in flight
may finish, which the preference UI and audit must represent accurately.
[Proactive personal messages](https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/conversations/send-proactive-messages).

Ordinary Graph chat-message creation's application permission is migration
permission, so the adapter must not assume background `ChatMessage.Send` with
app-only credentials. An activity-feed notification is a separate product
surface and does not satisfy the requested private message. Blocked/uninstalled
recipients and throttling are handled with suppression/retry and visible status;
tenant installation approval does not count as individual message opt-in.
[Graph chat-message permissions](https://learn.microsoft.com/en-us/graph/api/chat-post-messages?view=graph-rest-1.0).

### Composition, generated transport and React portals

App constructs platform mechanisms, Policy, Identity, Workplace, Visitors,
Plans, Booking, Events, Exchange, Notifications and Attendance, then registers
subscriptions and gateways. Construction order follows injected dependencies;
event subscription registration follows construction, so consumers do not call
half-built modules. Events receives Booking's narrow venue participant and
Visitors' ticket-visit contract; it never receives their constructors or stores.
The proposed venue-change scope registers Events and Booking writers; the
visitor-issuance scope registers Events and Visitors writers plus Booking's
read-only venue validation. Neither scope authorises arbitrary owner enlistment.

The single OpenAPI source describes externally visible transport; fragments
follow capabilities and are bundled deterministically. Generated Go server
types live under `internal/transport/gen/rest`; generated public Go clients
under `pkg/client/rest`; TypeScript clients/types under `web/generated`.
Module HTTP adapters translate DTOs into module API commands. Generated nullability,
HTTP errors, cookies and JSON enums never become the domain model. Regeneration
and contract-diff checks detect stale output. OpenAPI describes HTTP contracts;
it does not define private module interfaces.
[OpenAPI specification](https://spec.openapis.org/oas/latest.html).

`app/router.go` mounts module handlers on Chi with shared recovery, request IDs,
tracing, body limits and session/CSRF middleware. Prefer explicit route ownership
and generated operation registration, with handlers assigned to their
owning module. A route-to-module manifest fails if
two handlers claim an operation or any implemented operation lacks an owner.
SCIM stays a separately authenticated provisioning surface. Provider webhooks
have their own validation and bounded bodies.

The portal gateway composes read responses, preserves source freshness and
mounts static assets. It cannot turn a dashboard cache into availability truth.
Staff/mobile-web and admin React entrypoints share accessible components,
generated clients and a renderer wrapper. `FloorPlan.tsx` supplies immutable
plan data and owns the container; the existing renderer owns its children and
is cleaned up when the wrapper unmounts. Bookable-space overlays refer to stable
space IDs and revision IDs, rather than trusting SVG geometry as inventory.
React supports incremental integration and controlled DOM ownership.
[Incremental React](https://react.dev/learn/add-react-to-an-existing-project),
[DOM refs](https://react.dev/learn/manipulating-the-dom-with-refs).

Go owns provider tokens and secure server sessions. Browser clients send
HttpOnly secure cookies and CSRF proof for mutations; backend handlers enforce
capabilities, office scope and actor/owner rules regardless of which portal
rendered the button. The existing loopback/custom-header tracing server is not
relabelled as production SSO. Entra authorisation-code integration and session
controls need their own tests and threat review.
[Entra server flow](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-auth-code-flow),
[portal/session decision](../../adr/0015-use-react-portals-with-go-managed-sessions.md).

### Enforcement and tests against real PostgreSQL

The import test enumerates the ten allowed module roots, reads the Go import
graph and rejects unknown roots or non-API cross-module imports. It also
rejects provider SDKs and generated REST packages in core and API packages.
Generated clients remain transport clients; a module cannot circumvent the API
boundary by making an HTTP call to the same application's private endpoint.
Static rules are mirrored in the existing sole linter's allowed import policy.

SQL enforcement scans migrations and query manifests for schema ownership,
checks the explicit cross-module FK allowlist, verifies migration dependencies
and rejects unregistered named transaction participants. This is supplemented by
live database role tests: Exchange's credentials must fail an attempted claim
write, and ordinary Visitors credentials must fail a Booking write. Text scans
alone miss dynamic SQL and do not prove privilege boundaries.

| Test surface | Evidence it must establish |
| --- | --- |
| Module/unit | Hard constraints, policy versions, source authority, typed outcomes and retry classification with controlled clocks. |
| Booking store contracts on real PostgreSQL | Resource/owner exclusion, pooled capacity, transaction rollback and outbox/job atomicity; fake stores cannot prove locking. |
| Contention suites | Concurrent group/direct/cancel, transfer/parking, check-in/release, outage/expiry, venue/ticket and duplicate admission with barriers. |
| Cross-module assembly | Named venue-change rollback/ticket race and new-visit/ticket rollback versus existing-visit preservation; audience/visit restriction validation; no partial entitlement. |
| Provider adapters | Graph subscriptions/delta, late observations and ambiguous sends against controlled Chi test servers; source/Teams errors and retries. |
| Provisioning | Authenticated SCIM profile, deactivation/reactivation/delete, verified identity correlation and effective dates during source outages. |
| Recovery/rolling versions | Crash after each durable boundary; old lease cannot commit; expanded schema works with old/new roles; restored database matches published asset copies. |
| Acceptance | Staff/admin/visitor workflows through assembled HTTP and real PostgreSQL; backend access rules survive client manipulation. |

Use Docker/testcontainers before implementing most behaviour, as
[conventions](../../../CLAUDE.md) require. Preserve the existing total 95% and
per-package 90% coverage gates; measure meaningful behaviour rather than padding
thin API packages with constructor-only tests. Any coverage exclusions follow
the established gate configuration and are explicit review decisions. The
printed tree's representative tests are not a promise that every package needs
its own elaborate fake persistence implementation.

### Kubernetes roles, telemetry, rolling changes and recovery

One image supports independently scaled HTTP, allocation, Exchange, tracing and
maintenance deployments. Modules are code boundaries, not one Deployment each.
HTTP can accept a pending request while Exchange workers are unavailable;
allocation CPU must not consume HTTP's latency budget. Tracing uses bounded
scratch resources and cannot inherit broad booking or identity credentials.
Role-selected startup registers only its permitted job kinds and adapters.

Give roles separate Kubernetes service accounts and scoped database/provider
identities. Prefer workload federation where supported; remaining credentials
come through organisational secret management. Exchange's mailbox scope,
SCIM provisioning authentication and Teams installation permissions remain
separate from Azure/GCS object access. Federation requires configured issuer,
audience, subject trust and provider permissions; Kubernetes membership alone
does not confer cloud access.
[Entra federation](https://learn.microsoft.com/en-us/entra/workload-id/workload-identity-federation),
[Google Kubernetes federation](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-kubernetes).

OpenTelemetry traces and metrics export through a Collector; structured JSON
logs follow the organisation's logging pipeline. Carry request/operation IDs
through jobs and outbox handlers without placing visitor details or source
tokens in logs. Measure p95 initial response separately from allocation/room
completion, plus lock wait, job age, lease loss, waitlist lag, Exchange freshness,
pending-room age, identity-source age, retention backlog and protected-asset lag.
Business queue degradation can be critical while HTTP remains green.
[Go exporters](https://opentelemetry.io/docs/languages/go/exporters/),
[Collector](https://opentelemetry.io/docs/collector/).

Rolling releases use expand/backfill/contract migrations, versioned event
payloads and job handlers compatible with queued older work. A module does not
drop a column because its new binary no longer reads it while an old HTTP or
worker Pod still runs. Cross-owner FK and named-scope changes need a combined
compatibility plan. Run migrations under a controlled deployment role rather
than every replica racing to apply DDL. Pause only affected job kinds where a
safe rolling interpretation cannot be established.

The agreed target remains one active write region, multi-zone HA, warm regional
recovery, five-minute RPO and fifteen-minute RTO. If the primary is healthy and
recovery lag exceeds the loss budget, continue serving, raise a critical alert
and require an explicit operator decision before failover beyond that budget.
Promotion fences prior HTTP writers, workers and provider-effect owners before
enabling replacement writers. PostgreSQL documents the need to prevent the old
primary from returning as another writer.
[PostgreSQL failover](https://www.postgresql.org/docs/current/warm-standby-failover.html),
[selected recovery policy](../../adr/0017-use-one-write-region-with-warm-regional-recovery.md).

The recovery exercise includes database promotion, verified plan bytes, routing,
identity/secret access, job leases, Exchange subscriptions and delta progress,
notification replay and timeout protection. A healthy database replica alone
does not prove the whole-system target. After promotion, reconcile uncertain
external effects before resending blindly. Target 99.95% monthly availability
and one-second p95 initial response at the agreed representative workload need
contention, load and recovery measurements; ten modules and five role manifests
do not establish those outcomes.

### Workplace retirement, reactivation and deletion

`internal/workplace/retirement.go` handles explicit retirement and dependency-checked removal
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

Each trace uses the same scenarios as the other four options. Package paths
refer to the candidate tree; no production behaviour is implemented by this note.

### 1. Commit a group while inventory changes

1. `booking/core/planning/intents.go` collects staff-selected attendance dates, hard space
   requirements, explicit travel and linked parking requirements in a configured
   office cycle. Before the main run, ordinary desk requests join this demand.
2. `booking/core/planning/groups.go` creates per-date groups distinct from source-maintained
   teams. Booking assembles versioned snapshots with Identity and Workplace facts
   and passes them to its private solver adapter outside a long transaction.
3. `booking/core/planning/score.go` first maximises accommodated demand under hard constraints, then
   compares explicit collaboration, team proximity and previously unfulfilled
   demand in that order among equally accommodating plans. No attendance
   minimum or compulsory anchor-day setting enters the score.
4. `booking/core/planning/proposals.go` persists group placements and relevant input versions. The
   allocation worker calls `booking/api.Allocations.CommitGroup` with one
   idempotent group operation, not one booking call per person.
5. `booking/core/allocation/commit.go` authorises the operation and acquires
   protection, resource/pool and owner guards in the common order. A direct
   caller or cancellation can win a conflicting guard first; a snapshot gives
   no entitlement to overwrite the resulting confirmed state.
6. `validate.go` reloads current capacity, bundle membership, requirements,
   eligibility and relevant policy versions. Required parking participates in
   the same validation and transaction. A pooled place is not treated as an
   unconstrained "parking available" flag.
7. If all members still fit, the PostgreSQL adapter writes all claims, group
   commit receipt and durable work/outbox records together. Any failed member
   rolls back the whole group, including its parking. Commit success names the
   canonical Booking claim IDs.
8. On conflict, Booking records precise reasons and replans affected
   uncommitted groups against a fresh snapshot. Confirmed groups remain fixed.
   Once the main run completes, direct booking opens while eligible waitlist
   demand retains priority over fresh requests.
9. A later office transfer uses Booking's own command to acquire replacements
   and retire old claims atomically. A planner cannot simulate a transfer with
   separate cancel and book calls.

The contention test must show that the group is never partly confirmed, no owner
gets overlapping desks, parking never exceeds its pool and retry returns the
same committed group. The tradeoff is Booking's broad transaction knowledge;
the benefit is no choreography across independent Desk and Parking modules.

### 2. Race check-in, release and waitlist promotion during an outage

1. The staff portal posts a check-in through the generated transport adapter.
   Identity resolves the live session; Booking rejects a delegated PA action
   even if the PA can otherwise manage the owner's booking.
2. `booking/core/desks/checkin.go` and the no-show worker both lock the same
   protection and claim/resource guards. They use the booking's preserved policy
   version and authoritative clock, not the browser timestamp or a stale job
   payload's old deadline.
3. If check-in commits first, `Taken` and its evidence/outbox record exist;
   `release.go` rechecks state and cannot release it as a no-show. If release
   commits first, the delayed check-in is stale and cannot displace a subsequent
   assignee.
4. A fresh anonymous occupancy signal is recorded separately. It does not
   establish the owner's presence or block no-show release. Missing sensors
   cannot be interpreted as vacancy; only fresh sustained vacancy after a valid
   personal check-in can trigger that different policy.
5. For a confirmed application outage, `protection/outages.go` records affected
   offices and check-in/offer operations under the protection guard. A release
   obtaining the guard afterward observes the pause and reschedules. An already
   committed reassignment is preserved, including when the incident's evidence
   later establishes an earlier start.
6. On recovery, `deadlines.go` applies explicitly configured recovery grace
   within the booking's usable period. Offer acceptance and expiry follow the
   same pause rules; Exchange downtime alone is not automatically a confirmed
   app check-in outage.
7. When release is permitted, `waitlist/promote.go` selects the next eligible
   request while the capacity guard remains locked, secures its required linked
   parking and creates an offer in the release transaction. A fresh caller
   cannot jump into the gap between release and asynchronous notification.
8. Notification delivery follows commit. A delivery retry never creates another
   offer, and a delayed old acceptance cannot override a newer offer generation.
   Attendance later consumes the personal check-in or occupancy as separately
   labelled evidence under its retention policy.

This race is a module store/assembly test with real PostgreSQL and barriers,
including outage publication competing with expiry. It is not proved by a unit
test of three independently correct service methods.

### 3. PA room request, late acceptance, Outlook edit and revoked delegation

1. Identity verifies actor PA, owner/organiser and an authorised source-labelled
   grant. If it relies on Exchange delegation older than the configured limit,
   `exchangeauthority/delegates.go` refreshes before approving this new action.
   An outage preserves old source knowledge but does not make stale authority
   sufficient for a new dependent operation.
2. `booking/core/rooms/request.go` validates local room/time capacity and commits
   a pending hold, owner/actor audit and versioned desired-change outbox fact. The response
   reports pending acceptance; it does not claim a confirmed room because Graph
   accepted an HTTP request.
3. Exchange consumes that committed intention idempotently, records its own
   desired state/progress, and its worker sends to the owner's calendar using the scoped
   application identity and a stable creation transaction ID. An ambiguous
   timeout is reconciled before indiscriminate recreation. App-managed authority
   remains local; no native Outlook delegate grant is created.
4. Room acceptance, a webhook about an Outlook edit and an older response arrive
   out of order. Exchange persists observations and `exchange/core/reconcile.go`
   fetches current state where necessary, checking correlation/version/base state.
   Booking's `rooms/apply_observation.go` applies an idempotent observation through
   its own capacity rules. Exchange checkpoints after that result; stale messages
   cannot bypass the owning transition or make arrival order authoritative.
5. A compatible subject edit merges automatically. A time/room edit reacquires
   local capacity through Booking. If the desired replacement conflicts or the
   provider workflow cannot guarantee a safe replacement, the application shows
   the real pending/conflict state and seeks resolution; it invents no atomic
   provider room swap.
6. Acceptance after a hold expired cannot recreate released local capacity
   without revalidation. The handler schedules the appropriate reconciliation
   and user-visible resolution rather than overwriting another confirmed claim.
7. Revoking the Exchange-derived grant removes that source's authority for new
   actions and updates its verification state. Existing bookings retain their
   original actor/owner/source audit. A separately granted app-managed authority
   retains its own lifecycle; it is neither silently revoked nor silently
   substituted for an operation already denied on another basis.
8. Organiser cancellation uses the provider cancellation workflow; deleting the
   room copy is not treated as equivalent. Provider downtime keeps pending work
   durable and freshness visible while desks and check-in continue.

Tests assert convergence under duplicate/late observations and actor/owner
preservation. They do not assert an undocumented Graph conditional-write
guarantee, in-place organiser reassignment or exactly-once synchronisation.

### 4. Smaller event venue, concurrent ticket and visitor admission

1. An organiser edits the venue through `events/api.Events`; an attendee calls
   `events/api.Tickets.Issue`. Both enter Events. Through owning interfaces,
   they take compatible Booking venue guards followed by Events' event guard;
   audience/window state remains authoritative in Events.
2. `events/core/venue.go` coordinates the named venue transaction. Booking
   validates/replaces its old/new venue claims; Events rechecks issued tickets
   and updates its venue reference/capacity. Both owners commit together. If
   existing tickets exceed the proposed venue, the change is refused or an
   explicit resolution workflow begins.
3. `events/core/issue.go` sees either the committed smaller venue or the prior
   valid state, depending on which transaction wins. It checks current audience,
   invitation/public-registration rule, registration window and named attendee
   uniqueness before issuing. Capacity never depends on an eventually updated
   display counter.
4. A visitor first completes email verification. During `VisitTicketIssuance`,
   Events locks event capacity, invokes Visitors' narrow API to validate/create
   the host-linked restricted visit, inserts a ticket with that visit ID and
   commits both owned writes, receipts and outbox records in one transaction.
5. If the ticket loses the last-capacity race, the newly created ticket visit
   rolls back too. If the visit window/host restrictions fail, ticket insertion
   rolls back; a reused existing valid visit remains intact. A definitive refusal
   retains its receipt without new visit/ticket records. Repeating a successful
   operation returns the same linked pair and never consumes another seat.
6. At reception, `events/core/admit.go` checks that venue recovery no longer
   pauses admission, then validates ticket, event time and current visit
   restriction version. It coordinates with Visitors' version guard before
   committing admission; the read coupling prevents a restriction changing
   between a stale eligibility read and a successful entry.
7. The first scan creates the unique admission record. A concurrent scan reports
   already admitted, with no second admission and no new ticket. Offline scan
   acceptance is not assumed by the agreed scope and cannot bypass this check.
8. Ticket and visit messages follow commit. Visitor retention later removes
   identifiable visit/ticket/message/report copies in the required order;
   Attendance does not turn visitor entry into staff desk attendance.

The agreed venue transaction crosses Events and Booking through owning interfaces;
registration-created visits and tickets also commit together through Events and
Visitors. The scopes have separate participant allowlists, assembly tests and
migration checks. Their exact names and the remaining module map stay proposed.

### 5. SCIM disable and Workday office/team change during a partial outage

1. An authenticated SCIM request reaches `identity/adapters/http/scim.go` using
   provisioning credentials and the configured Entra-compatible profile.
   Identity resolves its source-linked staff ID; it does not match solely on
   the supplied email or accept a normal staff cookie as provisioning authority.
2. `core/scim.go` records the effective Entra administrative disable, source
   version and provisioning response. `sessions.go` invalidates sessions and
   access in that same Identity transaction, together with lifecycle outbox work.
3. A separately received Workday change has its own checkpoint, effective date
   and source-owned office/team facts. It cannot reactivate the Entra account,
   overwrite app travel choices or infer a different person from an email change.
4. A Workday fetch then fails halfway through a page sequence. The adapter does
   not advance the durable checkpoint past unapplied data or treat absent staff
   as deleted. Known source facts survive, and health/freshness describes the
   incomplete import. Ambiguous identity correlation becomes a review task.
5. On a future effective date, Identity activates the relevant facts and emits
   a versioned change. Booking updates only derived unconfirmed defaults and
   replans affected uncommitted proposals; explicit travel and confirmed
   bookings retain their intended dates/places and historical context.
6. Booking distinguishes account disable from effective leaver status. An
   actual leaver instruction releases future personal desk/parking claims,
   promotes eligible waitlist demand and opens room/event handover review.
   Neither an office/team move nor a partial feed implies departure. Historical
   claims survive, and in-place Exchange organiser mutation is not assumed.
7. Notification work explains arrangements requiring attention. Consumer
   receipts prevent duplicate handover tasks after replay; lifecycle cleanup lag
   is visible independently of already-effective access revocation.
8. SCIM reactivation and deletion remain distinct operations with source
   precedence. Deletion removes the provisioning resource while retained
   internal records follow configured purposes; a resumed source connection is
   not itself an instruction to restore access.

Combining Identity access and workforce projection reduces one cross-module
security race, but increases that module's breadth and access to sensitive
facts. Source ownership, internal APIs and least-privilege adapters still matter.

### 6. Add the private booking-raised Teams message

1. Booking already commits a versioned `BookingRaised` event with owner, actor,
   booking ID and minimal routing facts. No Teams client enters its transaction
   or its core port set; the authoritative booking response is unchanged.
2. `notifications/catalogue.go` maps this event to the exact message category
   `booking_raised`. Recipient selection addresses the relevant individual and
   distinguishes owner from delegate; it does not choose a shared channel.
3. `preferences.go` resolves explicit opt-in for that individual's private Teams
   booking-raised message. An absent preference means no Teams task. Existing
   email/in-app choices remain independently evaluated.
4. Add `notifications/teams/personal.go` and installation support, inject the
   sender at module construction, and configure separate app/bot identity and
   tenant prerequisites. Technical installation does not set the opt-in value.
5. The durable delivery handler verifies current consent and personal-scope
   readiness immediately before attempting the supported proactive message.
   It uses a deduplicated local delivery ID and records provider response or
   uncertainty; provider success cannot be atomically committed with PostgreSQL.
6. Uninstalled/blocked recipients become suppressed or actionable failures;
   transient throttling retries with bounded backoff. Consent withdrawal skips
   pending unsent attempts. No fallback sends an unapproved activity-feed alert
   or shared-channel message.
7. A repeated event or worker restart reuses delivery state. A remote send whose
   response was lost may require reconciliation and can still duplicate; the
   adapter makes no unsupported exactly-once guarantee. The app's booking and
   in-app status remain authoritative.
8. Tests cover no preference, explicit opt-in, withdrawal before send, owner/PA
   selection, missing installation, blocked delivery, retry and redaction.
   Future Teams message categories require their own separately approved
   preference and product decision.

The change is local to Notifications, app wiring/configuration, deployment
identity and adapter/assembly tests. It does not require a Teams port in every
module, a new business module or a second booking write path.

## Package mapping

Current paths are evidence of existing tracing behaviour; proposed paths are
placements to review. New business modules have no current implementation to
move. Preserve algorithm tests and golden fixtures during any later migration.

| Current path or responsibility | Proposed owner/location | Migration implication |
| --- | --- | --- |
| `cmd/spaceweave/main.go` | Same entrypoint, delegates to `internal/app/cli` | One image supports named runtime roles and existing CLI operations. |
| `internal/cli` | `internal/app/cli` plus module APIs | Remove direct catalogue/provider construction; preserve flags/exit behaviour. |
| `internal/config` | `internal/app/config` | Strict deployment YAML; runtime policy becomes Policy records. |
| `internal/buildinfo` | `internal/platform/buildinfo` | Neutral build information, no business dependency. |
| `internal/server/server.go` | `plans/adapters/http`, `gateway/portal`, `app/router` | Split plan operations, static hosting and assembly; replace local-only security assumptions deliberately. |
| `internal/server/profiles.go` | Plans HTTP and processing profile ownership | Existing layer-profile behaviour remains with Plans, scoped by office/admin capability. |
| `internal/server/files.go` | Plans multipart attachment/detach transport and upload workflow | Preserve attachment validation; static asset mounts originate in `server.go`. |
| `internal/catalog` | Plans metadata/core and PostgreSQL adapter; `localimport` for old packages | Catalogue entities/review behaviour survive; file locks do not become HA leases. |
| `internal/worker` | Plans processing job adapter and core publication | PostgreSQL attempts/leases replace polling catalogue claims; scratch remains disposable. |
| `internal/inbox` | Plans upload/import edge, composed by CLI | Local watch workflow is explicit tooling, not production distributed ingress. |
| `internal/convert`, `cad`, `dwg`, `dxf`, `svgdraw`, `vector`, `raster` | `plans/processing/<same-name>` | Preserve format, scale, rendering and validation behaviour and tests. |
| `internal/plan`, `floorgraph` | `plans/processing/plan`, `floorgraph` | Keep plan document/schema concerns distinct from bookable-space inventory. |
| `internal/autotrace`, `profile`, `score` | Plans processing subpackages | Tracer algorithm and profile/score tests move with their owner. |
| `internal/trace`, `fakeclaude` | Plans trace processing and adapter test support | Optional external model use remains behind tracing role/configuration. |
| `web/floorplan-iso` | Retained renderer plus `web/shared/FloorPlan.tsx` | React owns a wrapper; renderer owns its DOM subtree; retain JS tests. |
| Existing tracing CLI/server/catalogue tests | Beside moved packages and new Plans assembly suites | Move incrementally with behaviour; do not rewrite merely to match directories. |
| Staff access, source provisioning, scope/delegation | New Identity | No existing complete auth package is claimed. |
| Location definitions, dates, resource claims, allocation, visits/events | New Workplace, Booking, Exchange, Events and Visitors | Events owns registration/tickets/admission; Booking retains venue claims and joins the narrow venue transaction. |
| Delivery, attendance, runtime policy | New Notifications, Attendance and Policy | Distinct state/retention owners; neither reports nor notifications become command truth. |
| PostgreSQL jobs/outbox, cloud blobs, Kubernetes roles | New platform/Plans adapters/deploy | Existing local files and goroutines do not supply these guarantees. |

The existing [code map](../../architecture.md#existing-application) and
[module declaration](../../../go.mod) support the current-state column.
The tree is a target placement map rather than a claim that every source file
has already been migrated or that Go package moves are behaviour-neutral.

## Numbers

The numbers below count only the marked candidate tree in this document.
Directories containing at least one printed `.go` filename count as Go
packages, including test-only packages. A generated file is counted once as
printed; its eventual generator expansion, existing unexpanded files and future
unit tests are excluded. Root overview and prose examples are not counted.

| Measure | Printed candidate |
| --- | --- |
| Business modules | 10 |
| Gateway packages | 2 |
| Go packages | 114 |
| Named Go files, including generated and test files | 239 |
| Named SQL migration files | 14 |
| Named non-Go frontend files | 19 |
| Named Kubernetes manifest files | 7 |
| Packages under Booking | 19 |
| Packages under Events | 6 |
| Packages under Exchange | 8 |
| Packages under Plans | 25 |
| Cross-module write transaction exceptions | 2, `EventVenueChange` and `VisitTicketIssuance` |
| Cross-module foreign keys proposed initially | 1, nullable ticket-to-visit link, restrictive deletion |

These structural counts describe the representative proposal, not lines of
code, development effort or production performance. Plans has many packages
because existing processing already does; Booking has many because consistency
and operation-specific complexity justify them.

Adding the Teams adapter changes a small sender/installation package, message
catalogue/preference logic, app/deployment configuration and meaningful tests.
Adding a future independent business module adds its public API, private rules,
owned persistence/migrations, adapter registration, ownership policy and
acceptance coverage. Neither has a defensible fixed file-count estimate before
the capability is known. Event registration/ticket/admission changes belong to
Events; venue-claim changes cross its explicit Booking contract.

## Placement choices and friction

The choices below identify the ownership, consistency and change-impact
tradeoffs that matter when selecting this candidate.

- **Events and Booking have distinct ownership.** Events owns catalogue,
  audience, registration, ticket limits and admission; Booking owns venue claims.
  This agreed boundary limits Booking's breadth and gives event work its own
  review/migration stream, at the cost of the named shared venue transaction,
  compatible lock ordering and cross-owner tests. Asynchronous events alone
  cannot protect the final ticket against a concurrent venue shrink.
- **Merged event ownership was considered.** Keeping event policy and tickets
  inside Booking would remove that transaction seam but broaden Booking to own
  the entire attendance lifecycle. The selected boundary instead keeps all
  eligibility/ticket rules together in Events and joins Booking only where
  secured venue state is needed.

- **Combined Identity is agreed.** It can revoke known
  effective access and sessions atomically, while source adapters retain explicit
  field ownership. It also concentrates sensitive staff facts and authority
  rules. A future split needs a current security-state protocol, not merely a
  "deactivated" event eventually consumed by an otherwise permissive gateway.
- **Planning stays inside Booking.** Demand, cycles/groups and claim commitment
  share an owner, removing separate progress and demand-validation contracts.
  The solver remains a private replaceable capability with independent worker
  scaling; Booking becomes broader and needs operation-level package discipline.
- **Exchange is separate.** Its provider protocol/state lifecycle has its own
  interfaces and migrations, at the cost of a durable retry-safe handoff of
  intentions and observations. Booking keeps all room claim transitions.
- **Visitors remains independent despite its shared issuance operation.**
  Hosted visits, verification, reception restrictions and visitor retention are
  real behaviour beyond tickets. `VisitTicketIssuance` coordinates Events and
  Visitors atomically while retaining those responsibilities. The exception couples migration order,
  locking and release compatibility; if exceptions multiply, reconsider the
  boundary rather than generalising an all-module unit of work.
- **Read consistency still matters across modules.** Visitor restriction
  validation during admission and current identity eligibility cannot be solved
  by passing a stale DTO. Explicit version guards and short locking reads may
  enlist owned stores without giving callers SQL access. Their transaction
  semantics and failure behaviour require assembly tests and documented lock
  order; "only published APIs" does not by itself establish correctness.
- **Asset approval and operational publication have distinct owners.** Plans
  approves immutable assets; Workplace owns active geometry mappings and baseline
  changes as versioned inputs. Booking validates impact and owns dated applicability and
  temporary moves of unallocated desks within a building. A new definition
  cannot silently shrink protected confirmed inventory. This creates a visible
  publication/materialisation boundary and demands a clear admin status model.
- **Policy is a small module, not a universal rules engine.** It validates
  versions and override authority. Booking enforces its
  operational meaning using pinned versions. Runtime policy migrations that
  affect existing confirmations are explicit operations, not an incidental
  side effect of editing YAML or publishing a new default.
- **Module privacy needs multiple enforcement layers.** Go `internal` only
  constrains imports outside an ancestor tree; it does not automatically stop
  one sibling module importing another's adapter. Import tests, SQL manifests,
  role grants and review conventions carry the intended restriction. One shared
  broad database credential would make ownership mainly conventional.
- **Cross-module foreign keys couple lifecycles.** The allowed ticket/visit FK
  strengthens this local invariant and makes retention ordering
  explicit. It also creates schema coupling. Adding more such keys requires
  their own lifecycle case; blanket freedom would quietly recreate one shared
  data model.
- **Blob storage stays inside Plans initially.** Revision ownership is clear
  and provider types stay behind a single port. Extraction to platform is
  justified only by another independent asset owner, while publication and
  verified recovery policy remain Plans behaviour. Azure/GCP portability does
  not make recovery semantics identical.
- **Events and jobs make operations more demanding.** Receipts, pending effects,
  freshness, retention, replay and lease recovery need tooling from the first
  durable workflows. Separate role scaling protects resources but does not
  remove database contention or create fault isolation between code modules.
- **Moving current processing code has its own risk.** Preserve fixtures,
  reviewer feedback and renderer assumptions while introducing revisioned cloud
  assets. A directory change is not evidence of an improved drawing conversion,
  and generating an empty module skeleton is not progress on booking invariants.

The decision to review is whether these concrete ownership and transaction
boundaries make the six scenarios easier to keep correct and change safely than
the other four layouts. Select the exact boundaries using consistency, change-impact
and operational evidence.
