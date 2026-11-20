# Option 2: Hexagonal architecture: ports and adapters

The repository-architecture decision is pending. This candidate expresses the
same agreed logical owners as the other four: combined Identity; attendance
planning and claim commitment inside Booking; separate Exchange and Events.
Its package organisation and dependency rules remain specific to this style.
[Identity](../../adr/0026-combine-workforce-identity-and-access-ownership.md),
[Booking planning](../../adr/0027-keep-attendance-planning-inside-booking.md) and
[Exchange](../../adr/0028-separate-exchange-synchronisation-from-booking.md)
record those ownership decisions independently of repository style.

SpaceWeave coordinates workplace bookings and advance desk allocation across a
large organisation's offices. Its application must accept requests from web
portals, automation and workers while integrating with PostgreSQL, Exchange
Online, Entra, Workday and cloud asset storage. This candidate isolates those
technologies behind application-owned ports.

[Cockburn's original description](https://alistair.cockburn.us/hexagonal-architecture)
defines the inside/outside boundary and adapters for purposeful conversations.
It leaves the organisation of the application interior open. The central core,
shared port packages and single PostgreSQL adapter below are concrete comparison
choices, with their costs examined explicitly. See [architecture foundations](architecture-foundations.md)
for the source principles and the [common requirements](README.md#requirements-held-constant)
for product constraints. Paths are proposed destinations for a detailed design.

## Summary

One application core owns the whole system's business behaviour. Driving ports
describe requests from HTTP, provisioning clients, timers, inboxes and durable
workers. Driven ports describe dependencies on PostgreSQL, Exchange, Entra,
Workday, blob storage, tracing engines and delivery providers. Adapters translate
between those contracts and technology-specific representations.
The organising boundary is inside versus outside, consistent with
[Cockburn's original description](https://alistair.cockburn.us/hexagonal-architecture).

The proposal deliberately uses one `internal/core`, one driving-port package,
one driven-port package and one PostgreSQL adapter. It does not recreate the
blueprint's independently encapsulated domain modules under an `adapter` prefix.
Services share core models and may collaborate synchronously through explicitly
wired interfaces. Technology can be replaced at the perimeter; business-package
privacy needs additional rules because the hexagon alone does not provide it.

Booking is the single local authority for dated inventory, claims and releases.
Exchange observations, event tickets, sensor signals, reports and the allocation
planner cannot write physical-space claims directly. Events owns its ticket
capacity and admission. A shared database adapter is an
implementation of several restricted store facets, not permission for every
service to receive every facet. Logical table owners and migration reviewers
remain explicit; [ownership](#storage-ownership-and-transactions) explains the
compromise of placing their SQL in one package.

Dependency rules proposed for this candidate:

1. `internal/core/...` imports no adapter, composition, configuration, contract,
   generated transport, command, SQL driver or provider SDK package.
2. `core/port/driving` and `core/port/driven` import only core models and the
   standard library. They do not import services or each other.
3. `core/model`, `core/geometry` and `core/rules` import no service or port.
   Rules consume values; they neither fetch data nor persist decisions.
4. Driving adapters import core contracts/models, never concrete services,
   driven adapters or provider SDKs. HTTP handlers hold driving interfaces.
5. Driven adapters import core models/driven contracts, never driving adapters
   or application composition. A provider response becomes a core observation.
6. Core services receive only the driven facets their ownership permits.
   Booking alone receives `CapacityWriter`; Exchange receives no claim writer.
7. Services do not import concrete sibling services. A small consumer-owned
   interface or a driving contract carries collaboration; `app` supplies it.
8. `internal/app` alone constructs concrete services and adapters. The CLI is a
   declared exception to rule 4: its role commands call composition to start a
   runtime, while its business commands use driving ports.
9. Runtime packages import no `contract` or `tooling` package. `pkg` imports no
   project-private package; generated public clients remain transport clients.
10. Generated Go/TypeScript types stop at adapters and portal client wrappers.
    They never become persistence, planning or domain contracts.

Rules 6 and 7 are SpaceWeave ownership additions, not universal hexagonal laws.
An interface can hide a forbidden writer, so import checks need constructor,
SQL-ownership and real-transaction checks as well.

## Repository root

```text
spaceweave/
├── cmd/spaceweave/     main.go: executable and process exit only
├── internal/
│   ├── app/            composition and independently scaled runtime roles
│   ├── buildinfo/      existing version information
│   ├── config/         strict YAML deployment settings, Cobra overrides
│   ├── core/           model, pure rules/geometry, ports and services
│   ├── adapter/        driving/ and driven/ technology boundaries
│   ├── contract/       reusable driven-port test suites
│   ├── layout/         dependency and SQL-ownership checks
│   └── tooling/        OpenAPI bundling/generation checks
├── pkg/client/         generated Go transport clients, when consumers need them
├── api/                proposed OpenAPI sources, bundles and generator configs
├── web/
│   ├── staff/          React/TypeScript staff and mobile-web entrypoint
│   ├── admin/          separate React/TypeScript administration entrypoint
│   ├── shared/         UI, client wrappers, renderer integration
│   └── floorplan-iso/  existing D3 renderer and its geometry/render tests
├── test/               real-dependency acceptance, concurrency and recovery suites
├── deploy/             proposed container/Kubernetes/Collector configuration
├── docs/               scope, decisions, policies, research and runbooks
├── tools/              existing drawing/render checks plus generation scripts
├── .github/            existing quality/release workflows and ownership
├── CLAUDE.md
├── GLOSSARY.md         existing conventions and product language
├── Makefile            existing checks; proposed generation/acceptance targets
├── .golangci.yml       existing blocking linter, adds dependency restrictions
├── .testcoverage.yml   existing total/per-package coverage gates
├── go.mod
└── go.sum              existing module; dependencies added with implementation
```

Today's implementation is the floor-plan tool, not a booking
platform. `internal/server` holds a Chi upload/review server;
`internal/catalog` stores a filesystem catalogue; `internal/worker` runs local
conversion/tracing. PostgreSQL, SCIM and booking packages in this proposal do not
exist today. See the actual [server](../../../internal/server/server.go),
[catalogue](../../../internal/catalog/catalog.go),
[worker](../../../internal/worker/worker.go) and
[current module](../../../go.mod).

## Full package tree

Every directory containing a named `.go` file below is a proposed Go package.
Files are representative placements, not an exhaustive inventory.
Existing algorithms move only when their dependency seams are
made explicit. SQL migrations are one ordered stream with logical ownership.
The structural counts later use this block only, without counting examples twice.

```text
spaceweave/
├── cmd/
│   └── spaceweave/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   ├── runtime.go
│   │   ├── services.go
│   │   ├── adapters.go
│   │   ├── http.go
│   │   ├── roles.go
│   │   ├── subscriptions.go
│   │   └── shutdown.go
│   ├── buildinfo/
│   │   └── buildinfo.go
│   ├── config/
│   │   ├── config.go
│   │   └── roles.go
│   ├── layout/
│   │   ├── layout_test.go
│   │   └── ownership_test.go
│   ├── core/
│   │   ├── model/
│   │   │   ├── identity.go
│   │   │   ├── access.go
│   │   │   ├── workplace.go
│   │   │   ├── policy.go
│   │   │   ├── intent.go
│   │   │   ├── allocation.go
│   │   │   ├── booking.go
│   │   │   ├── room.go
│   │   │   ├── delegation.go
│   │   │   ├── drawing.go
│   │   │   ├── visit.go
│   │   │   ├── event.go
│   │   │   ├── attendance.go
│   │   │   ├── notification.go
│   │   │   ├── work.go
│   │   │   ├── incident.go
│   │   │   └── errors.go
│   │   ├── geometry/
│   │   │   ├── cad/
│   │   │   │   ├── cad.go
│   │   │   │   └── walk.go
│   │   │   ├── dwg/
│   │   │   │   ├── dwg.go
│   │   │   │   └── entities.go
│   │   │   ├── dxf/
│   │   │   │   ├── dxf.go
│   │   │   │   └── entities.go
│   │   │   ├── floorgraph/
│   │   │   │   ├── graph.go
│   │   │   │   └── build.go
│   │   │   ├── plan/
│   │   │   │   ├── trace.go
│   │   │   │   └── convert.go
│   │   │   ├── raster/
│   │   │   │   ├── raster.go
│   │   │   │   └── measure.go
│   │   │   ├── svgdraw/
│   │   │   │   ├── svgdraw.go
│   │   │   │   └── path.go
│   │   │   ├── vector/
│   │   │   │   ├── vector.go
│   │   │   │   └── raster.go
│   │   │   └── profile/
│   │   │       ├── profile.go
│   │   │       └── layers.go
│   │   ├── rules/
│   │   │   ├── access/
│   │   │   │   ├── capabilities.go
│   │   │   │   └── delegation.go
│   │   │   ├── allocation/
│   │   │   │   ├── constraints.go
│   │   │   │   ├── groups.go
│   │   │   │   ├── packing.go
│   │   │   │   ├── objective.go
│   │   │   │   └── repair.go
│   │   │   ├── attendance/
│   │   │   │   ├── evidence.go
│   │   │   │   └── deadlines.go
│   │   │   ├── calendar/
│   │   │   │   ├── merge.go
│   │   │   │   └── occurrences.go
│   │   │   ├── identity/
│   │   │   │   ├── correlation.go
│   │   │   │   └── effective.go
│   │   │   └── privacy/
│   │   │       └── retention.go
│   │   ├── port/
│   │   │   ├── driving/
│   │   │   │   ├── identity_access.go
│   │   │   │   ├── identity_people.go
│   │   │   │   ├── provisioning.go
│   │   │   │   ├── workplace.go
│   │   │   │   ├── policy.go
│   │   │   │   ├── booking.go
│   │   │   │   ├── allocation.go
│   │   │   │   ├── rooms.go
│   │   │   │   ├── exchange.go
│   │   │   │   ├── plans.go
│   │   │   │   ├── visitors.go
│   │   │   │   ├── events.go
│   │   │   │   ├── reporting.go
│   │   │   │   ├── notifications.go
│   │   │   │   ├── work.go
│   │   │   │   └── incidents.go
│   │   │   └── driven/
│   │   │       ├── unitofwork.go
│   │   │       ├── identity_people_store.go
│   │   │       ├── identity_access_store.go
│   │   │       ├── workplace_store.go
│   │   │       ├── policy_store.go
│   │   │       ├── capacity_store.go
│   │   │       ├── exchange_store.go
│   │   │       ├── plan_store.go
│   │   │       ├── visitor_store.go
│   │   │       ├── event_store.go
│   │   │       ├── reporting_store.go
│   │   │       ├── notification_store.go
│   │   │       ├── work_store.go
│   │   │       ├── incident_store.go
│   │   │       ├── blob.go
│   │   │       ├── exchange.go
│   │   │       ├── identity.go
│   │   │       ├── tracing.go
│   │   │       ├── delivery.go
│   │   │       ├── clock.go
│   │   │       └── solver.go
│   │   └── service/
│   │       ├── identity/
│   │       │   ├── identity.go
│   │       │   ├── access.go
│   │       │   ├── sessions.go
│   │       │   ├── delegation.go
│   │       │   ├── scim.go
│   │       │   ├── workday.go
│   │       │   ├── eligibility.go
│   │       │   └── native_delegation.go
│   │       ├── workplace/
│   │       │   ├── workplace.go
│   │       │   ├── changes.go
│   │       │   ├── retirement.go
│   │       │   └── mapping.go
│   │       ├── policy/
│   │       │   ├── policy.go
│   │       │   └── publish.go
│   │       ├── booking/
│   │       │   ├── booking.go
│   │       │   ├── inventory.go
│   │       │   ├── intents.go
│   │       │   ├── allocation.go
│   │       │   ├── commit_group.go
│   │       │   ├── transfer.go
│   │       │   ├── waitlist.go
│   │       │   ├── checkin.go
│   │       │   ├── release.go          explicit selected/pair cancellation; timed desk-only release
│   │       │   ├── evidence.go
│   │       │   ├── rooms.go            room intentions/holds and no-secured-room recovery
│   │       │   ├── venue.go
│   │       │   └── outages.go
│   │       ├── exchange/
│   │       │   ├── exchange.go
│   │       │   ├── reconcile.go
│   │       │   ├── outbound.go
│   │       │   ├── acceptance.go
│   │       │   └── subscriptions.go
│   │       ├── plans/
│   │       │   ├── plans.go
│   │       │   ├── processing.go
│   │       │   ├── review.go
│   │       │   └── publish.go
│   │       ├── visitors/
│   │       │   ├── visitors.go
│   │       │   ├── verify.go
│   │       │   └── retention.go
│   │       ├── events/
│   │       │   ├── events.go
│   │       │   ├── venue.go
│   │       │   ├── tickets.go
│   │       │   ├── admission.go
│   │       │   └── protection.go
│   │       ├── reporting/
│   │       │   ├── reporting.go
│   │       │   ├── project.go
│   │       │   └── retention.go
│   │       ├── notifications/
│   │       │   ├── notifications.go
│   │       │   ├── preferences.go
│   │       │   └── deliver.go
│   │       ├── operations/
│   │       │   └── incidents.go
│   │       └── work/
│   │           ├── work.go
│   │           ├── outbox.go
│   │           └── leases.go
│   ├── adapter/
│   │   ├── driving/
│   │   │   ├── cli/
│   │   │   │   ├── cli.go
│   │   │   │   ├── serve.go
│   │   │   │   ├── worker.go
│   │   │   │   ├── plans.go
│   │   │   │   └── migrate.go
│   │   │   ├── http/
│   │   │   │   ├── server.go
│   │   │   │   ├── middleware.go
│   │   │   │   ├── errors.go
│   │   │   │   ├── incidents.go
│   │   │   │   ├── gen/
│   │   │   │   │   ├── staff/
│   │   │   │   │   │   └── server.gen.go
│   │   │   │   │   ├── admin/
│   │   │   │   │   │   └── server.gen.go
│   │   │   │   │   └── scim/
│   │   │   │   │       └── server.gen.go
│   │   │   │   ├── session/
│   │   │   │   │   ├── login.go
│   │   │   │   │   ├── cookies.go
│   │   │   │   │   └── csrf.go
│   │   │   │   ├── booking/
│   │   │   │   │   ├── booking.go
│   │   │   │   │   ├── checkin.go
│   │   │   │   │   └── rooms.go
│   │   │   │   ├── workplace/
│   │   │   │   │   └── workplace.go
│   │   │   │   ├── plans/
│   │   │   │   │   ├── plans.go
│   │   │   │   │   └── uploads.go
│   │   │   │   ├── people/
│   │   │   │   │   ├── staff.go
│   │   │   │   │   └── delegation.go
│   │   │   │   ├── scim/
│   │   │   │   │   ├── users.go
│   │   │   │   │   ├── groups.go
│   │   │   │   │   └── protocol.go
│   │   │   │   ├── visitors/
│   │   │   │   │   ├── visits.go
│   │   │   │   │   └── verification.go
│   │   │   │   ├── events/
│   │   │   │   │   ├── events.go
│   │   │   │   │   ├── tickets.go
│   │   │   │   │   └── admission.go
│   │   │   │   ├── reports/
│   │   │   │   │   └── attendance.go
│   │   │   │   └── preferences/
│   │   │   │       └── notifications.go
│   │   │   ├── scheduler/
│   │   │   │   └── scheduler.go
│   │   │   ├── jobs/
│   │   │   │   ├── runner.go
│   │   │   │   └── handlers.go
│   │   │   ├── occupancy/
│   │   │   │   └── receiver.go
│   │   │   ├── exchangehook/
│   │   │   │   ├── notifications.go
│   │   │   │   └── lifecycle.go
│   │   │   ├── inbox/
│   │   │   │   └── watcher.go
│   │   │   └── web/
│   │   │       └── assets.go
│   │   └── driven/
│   │       ├── solver/
│   │       │   └── solver.go
│   │       ├── postgres/
│   │       │   ├── postgres.go
│   │       │   ├── unitofwork.go
│   │       │   ├── retry.go
│   │       │   ├── identity_people.go
│   │       │   ├── identity_access.go
│   │       │   ├── workplace.go
│   │       │   ├── policy.go
│   │       │   ├── capacity.go
│   │       │   ├── allocation.go
│   │       │   ├── evidence.go
│   │       │   ├── waitlist.go
│   │       │   ├── rooms.go
│   │       │   ├── exchange.go
│   │       │   ├── plans.go
│   │       │   ├── visitors.go
│   │       │   ├── events.go
│   │       │   ├── reporting.go
│   │       │   ├── notifications.go
│   │       │   ├── jobs.go
│   │       │   ├── outbox.go
│   │       │   ├── incidents.go
│   │       │   ├── migrations.go
│   │       │   ├── ownership.yaml
│   │       │   └── migrations/
│   │       │       ├── 0001_identity.sql
│   │       │       ├── 0002_workplace_policy.sql
│   │       │       ├── 0003_booking_capacity.sql
│   │       │       ├── 0004_work_outbox.sql
│   │       │       ├── 0005_exchange.sql
│   │       │       ├── 0006_plans.sql
│   │       │       ├── 0007_visitors_events.sql
│   │       │       ├── 0008_reporting_notifications.sql
│   │       │       └── 0009_incidents.sql
│   │       ├── blob/
│   │       │   ├── azure/
│   │       │   │   └── azure.go
│   │       │   ├── gcs/
│   │       │   │   └── gcs.go
│   │       │   └── verified/
│   │       │       ├── verified.go
│   │       │       └── reconcile.go
│   │       ├── graphcalendar/
│   │       │   ├── client.go
│   │       │   ├── events.go
│   │       │   ├── delta.go
│   │       │   └── permissions.go
│   │       ├── entra/
│   │       │   ├── oidc.go
│   │       │   └── tokens.go
│   │       ├── workday/
│   │       │   ├── client.go
│   │       │   ├── workers.go
│   │       │   └── mappings.go
│   │       ├── teams/
│   │       │   ├── personal.go
│   │       │   └── conversations.go
│   │       ├── smtp/
│   │       │   └── smtp.go
│   │       ├── tracing/
│   │       │   ├── automatic/
│   │       │   │   ├── automatic.go
│   │       │   │   ├── cadfront/
│   │       │   │   │   ├── build.go
│   │       │   │   │   └── spaces.go
│   │       │   │   ├── rasterfront/
│   │       │   │   │   └── build.go
│   │       │   │   ├── score/
│   │       │   │   │   └── score.go
│   │       │   │   └── synth/
│   │       │   │       └── synth.go
│   │       │   └── claude/
│   │       │       ├── agent.go
│   │       │       └── tools.go
│   │       ├── drawingconvert/
│   │       │   └── convert.go
│   │       ├── nodevalidator/
│   │       │   └── validator.go
│   │       ├── profilefiles/
│   │       │   └── library.go
│   │       ├── telemetry/
│   │       │   ├── telemetry.go
│   │       │   └── logging.go
│   │       └── fakeclaude/
│   │           └── fakeclaude.go
│   ├── contract/
│   │   ├── storetest/
│   │   │   ├── storetest.go
│   │   │   ├── capacity.go
│   │   │   └── leases.go
│   │   ├── blobtest/
│   │   │   └── blobtest.go
│   │   ├── exchangetest/
│   │   │   └── exchangetest.go
│   │   ├── scimtest/
│   │   │   └── scimtest.go
│   │   ├── tracingtest/
│   │   │   └── tracingtest.go
│   │   └── deliverytest/
│   │       └── deliverytest.go
│   └── tooling/
│       └── openapi/
│           ├── bundle.go
│           └── check.go
└── pkg/
    └── client/
        ├── staff/
        │   └── client.gen.go
        └── admin/
            └── client.gen.go
```

The frontend and runtime files are deliberately outside that Go-package count:

```text
spaceweave/
├── api/
│   ├── src/
│   │   ├── staff.yaml
│   │   ├── admin.yaml
│   │   └── scim.yaml
│   ├── staff.openapi.yaml
│   ├── admin.openapi.yaml
│   ├── scim.openapi.yaml
│   ├── oapi-server-staff.yaml
│   ├── oapi-server-admin.yaml
│   ├── oapi-server-scim.yaml
│   └── typescript-client.yaml
├── web/
│   ├── staff/
│   │   └── src/
│   │       ├── main.tsx
│   │       ├── routes.tsx
│   │       ├── booking/
│   │       │   ├── AttendanceDates.tsx
│   │       │   └── DeskMap.tsx
│   │       ├── rooms/
│   │       │   └── RoomRequest.tsx
│   │       └── events/
│   │           └── Ticket.tsx
│   ├── admin/
│   │   └── src/
│   │       ├── main.tsx
│   │       ├── routes.tsx
│   │       ├── allocation/
│   │       │   └── RunReview.tsx
│   │       ├── workplace/
│   │       │   └── FloorEditor.tsx
│   │       ├── reception/
│   │       │   └── VisitDesk.tsx
│   │       ├── events/
│   │       │   └── EventEditor.tsx
│   │       └── operations/
│   │           └── RecoveryStatus.tsx
│   ├── shared/
│   │   └── src/
│   │       ├── api/
│   │       │   ├── generated/
│   │       │   │   ├── staff.ts
│   │       │   │   └── admin.ts
│   │       │   └── client.ts
│   │       ├── session/
│   │       │   └── SessionProvider.tsx
│   │       ├── renderer/
│   │       │   └── FloorPlan.tsx
│   │       └── ui/
│   │           └── OfficePicker.tsx
│   └── floorplan-iso/
│       ├── js/
│       │   └── render.js
│       └── test/
│           └── invariants.test.js
├── deploy/
│   ├── Dockerfile
│   ├── kubernetes/
│   │   ├── http.yaml
│   │   ├── allocation.yaml
│   │   ├── exchange.yaml
│   │   ├── tracing.yaml
│   │   ├── maintenance.yaml
│   │   ├── migrate-job.yaml
│   │   └── serviceaccounts.yaml
│   └── collector/
│       └── config.yaml
└── test/
    ├── acceptance/
    │   ├── booking_test.go
    │   └── scim_test.go
    ├── concurrency/
    │   ├── capacity_test.go
    │   ├── checkin_test.go
    │   └── venue_test.go
    ├── integration/
    │   └── exchange_test.go
    ├── recovery/
    │   └── regional_test.go
    └── performance/
        └── booking_test.go
```

## API surfaces and consumers

The [shared catalogue](../api-surfaces/README.md) records UI, MCP servers, SDKs,
scripts, testing and Terraform as consumers of the supported public REST family.
The driving ports remain private application contracts. Internet accessibility
under Conditional Access does not publish every core method or grant every
client every action.

| Consumer/surface | Contract and access | Binding to this layout |
| --- | --- | --- |
| UI, MCP, SDK, script, contract tests and Terraform | Supported public REST, capability-specific views and common business semantics. | HTTP driving adapters translate the public schemas into `core/port/driving`; clients do not call concrete services or database adapters. |
| Staff/admin humans and enrolled applications | Approved session/CSRF or token profiles; Conditional Access and current record authority. | Credential validation at the driving edge; Identity/command owners distinguish actual actor, client, owner and machine grants. |
| Visitors to internal events | Internet invitation/verification and restricted visitor event/visit/ticket contract. | Visitor/session projection adapters; a guest speaker is not made a staff principal. No general anonymous catalogue is established. |
| Provisioning/provider/source clients | Native SCIM, Graph/Teams callbacks and selected source-ingestion protocols. | Specialised driving adapters validate the real caller/profile. Their Graph/Teams/Workday SDK clients are outbound driven adapters. |
| Platform workers, CLI runtime and schedulers | Private Go driving contracts and durable work envelopes. | Run direct core calls with scoped runtime credentials; not a second public REST or a remote HTTP copy of the whole hexagon. |
| Operational tooling and storage/provider implementations | Private probes/telemetry and driven dependency interfaces. | Restricted listeners/export and `core/port/driven`; no raw store/UoW interface is externally published. |

The single core lets all public consumers share behaviour. It also concentrates
model/port changes, so public wire schemas stay at the perimeter instead of
serializing central Staff/Booking models. Expected versions, retry identities
and actual permissions are carried into the core; an SDK or MCP adapter cannot
replace them with client-side decisions.

OpenAPI supplies the common UI/SDK transport contracts. Private core ports have
no external version/support promise, while the public contract must account for
Terraform/scripts that upgrade independently of a UI deployment. Distinct
human/application authority, initial configuration-only Terraform management,
independent public version compatibility and follow-on MCP/provider delivery are
agreed. Configurable RBAC belongs with Identity: MCP retains user context,
machine-identity scripts have a separate channel, UI drift is allowed and initial
version-retirement notice is 180 days. No common login journey is selected by
client label. Global administration, SCIM-backed role mappings, grant-only scoped
composition and self-permission discovery are agreed. The [agreed request contract](../api-surfaces/request-behaviour.md)
carries retry identity into the owning core command/transaction port, with shared
atomic acceptance receipts, previous-version configuration edits, two request
classes, actor budgets and public operation status. Wire and retention
details remain open; worker execution is bounded separately.
These bindings refine the proposal, not its printed counts.

## Deep dives

Booking consumes the driven solver interface, implemented by its solver adapter
using pure allocation rules. Demand, planning progress and claims remain one
Booking owner, with calculation outside claim transactions.

### Allocation, packing and one capacity authority

`driving.Allocation` offers cycle/snapshot/planning/commit operations implemented
by `core/service/booking`. `rules/allocation` receives a complete value snapshot;
it has no repository, SQL or worker dependency. Snapshot data identifies the
office/date/calendar, effective policy, workplace definition, dated bundle
membership, teams, eligible intents, explicit collaboration, claim versions and
parking capacity. Keep source versions and a stable input digest for diagnosis.

`groups.go` forms per-date allocation groups; a directory team is an input, not
the group itself. `constraints.go` filters mandatory features, availability,
building boundaries and linked parking. `packing.go` fits groups to dated
neighbourhoods, including bundles spanning floors of one building. The objective
first accommodates demand; among plans accommodating the same demand it ranks
explicitly prioritised collaboration, team proximity and previous unmet demand.
Deterministic tie breaking makes the same snapshot explainable. Heuristics,
fairness windows and acceptable solve budgets require workload validation.

Planning runs outside a write transaction. `commit_group.go` rechecks each
group's current versions, eligibility, policy and capacity in a short transaction.
All desks for that placement and every both-required parking claim succeed
together. A conflicted group remains uncommitted; repair affected uncommitted
groups without moving protected confirmations. A run is not one enormous atomic
transaction, and a proposal never counts as a booking.

Ordinary direct desk requests enter intent collection before the main run and
open afterward. Later requests, cancellations, waitlist promotion and dated
neighbourhood adjustments all enter Booking. Adjustments can move unallocated
desks between bundles, never silently revoke a confirmed claim. A desk transfer
locks old and replacement resources in a canonical order, obtains the replacement
desk/required parking and releases the former claims in the same transaction.
The owner cannot acquire overlapping desk claims in different offices.

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
These are baseline requirements from
[ADR 0014](../../adr/0014-use-explicit-allocation-cycles-and-atomic-group-placement.md)
and the [allocation policies](../../policy-defaults.md#agreed-allocation-rules).

### Storage ownership and transactions

`driven.UnitOfWork` supplies a callback-scoped transaction. Inside it, services
receive restricted transaction facets, never a raw `*sql.Tx` or a universal
`Store.Tables()` accessor. The PostgreSQL adapter implements all facets against
one connection/transaction; `app` gives each service only its permitted factory.
This is a shared system store with capability-shaped views, not separate stores
pretending to coordinate distributed transactions.

| Logical owner | Owned rows and migration responsibility | Service writer |
| --- | --- | --- |
| Identity | staff/source facts, teams, SCIM, sessions, capabilities/scopes and delegation evidence | Identity-owned people/access facets inside the shared adapter |
| Workplace | office/building/floor/space definitions, calendars, baseline bundles, lifecycle schedules and active geometry mappings | `workplace` |
| Policy | revisioned policies, office overrides, publication/effective records | `policy` |
| Booking | intents/cycles/groups/proposals, dated inventory, claims/holds, waitlist/offers, check-ins, validated operational occupancy evidence and own protected deadline state | `booking` only |
| Exchange | mailbox/event mappings, desired/observed revisions, acceptance facts, subscriptions/cursors | `exchange` |
| Plans | drawing packages, processing attempts, immutable revisions, blob/recovery/publication metadata | `plans` |
| Visitors | visitor identities, hosted visits, verified-link grants, reception records | `visitors` |
| Events | event/audience/capacity, named tickets, admission records, venue association | `events` |
| Reporting | derived attendance/report projections and separately configured reporting retention | `reporting` |
| Notifications | preferences, Teams message consent, in-app messages, delivery attempts | `notifications` |
| Operational facilities | confirmed incident declarations, scope/times/revisions and ordering guards | `operations` through its incident-store facet; no business expiry writes |
| Work | job/outbox rows, leases, consumer checkpoints, retry/failure records | `work` through transactional facets |

`postgres/ownership.yaml` maps each table and migration section to that owner.
SQL in `capacity.go`, `allocation.go`, `waitlist.go`, `evidence.go` and `rooms.go` implements
Booking's writer. Other SQL files may read approved views but cannot update those
tables. Layout checks inspect query ownership, constructors do not leak writer
facets, and review covers dynamic SQL which static prefix checks cannot prove.
The adapter is one Go package: these are enforced conventions, not Go's nested
`internal` protection. Shared database credentials likewise do not isolate owners.

For concrete concurrency control, propose lockable resource/date and owner/date
guard rows, materialised before claims, plus exclusion constraints for overlapping
specific resources and owner desk intervals. Claims/releases follow one ordered
guard protocol. Pooled parking additionally locks a pool/time-capacity guard and
validates the sum; a uniqueness constraint on a bay cannot implement a pool.
Locking only existing bookings misses the empty-resource race. PostgreSQL
documents [row locking](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS)
and [range exclusions](https://www.postgresql.org/docs/current/rangetypes.html#RANGETYPES-CONSTRAINT);
the guard layout and interval strategy here are proposed, requiring real tests.

Every command carries an actor, owner where relevant, expected version and retry
key. Persist the result and request digest in the same transaction; identical
retries return it and incompatible reuse fails. Retry a serialization failure
from the decision's beginning with bounded attempts; distinguish genuine capacity
loss from a transient database failure. PostgreSQL explicitly requires
[whole-transaction retry](https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html).

Event venue changes use a named `events.ChangeVenue` coordinator, constructed
with Events' writer, Booking's transaction participant and one UoW. Both take the
event guard before ticket count or venue decisions. Booking's participant alone
mutates claims; Events alone mutates tickets/capacity. Ordinary room/space editing
rejects event-owned venues and routes to this operation. This shared transaction
is natural in one hexagon, but it is an explicit exception to independent owner
transactions. No arbitrary service receives a cross-owner mutation bag.

### Check-in, occupancy, waitlists and outage protection

`booking/checkin.go` validates the current allocation, owner eligibility, booking
window and actor identity, then changes allocated to taken under the same claim
lock used by `release.go`. A PA's delegation cannot satisfy personal check-in.
No-show expiry checks that current state again; whichever valid transition
commits first determines the outcome. It cannot release a now-taken desk using
the stale state captured when its job was enqueued.

Booking validates, deduplicates and stores operational occupancy evidence through
its capacity-store facet. A driving occupancy receiver calls Booking's ingestion
port; outbound source polling, if needed, is a driven integration. Reporting
projects committed facts with separately configured retention. Anonymous occupancy
is distinct from personal check-in and cannot establish the owner's attendance
or withhold a no-show desk from the pool. Post-check-in vacancy may release a taken desk only using fresh sustained
vacancy evidence over the configured window. A missing/stale sensor is unknown.
Office presence is another evidence category and never proves desk use.

Release atomically marks the claim ended and advances eligible waitlisted demand
or records its next promotion job before fresh requests can win. Promotion and
direct requests share the same guards and eligibility checks. If an offer holds
capacity, it is a versioned Booking hold with a persisted expiry, not a browser
timer. Acceptance rechecks the current offer, linked parking and owner state.

`core/service/operations/incidents.go` owns declarations through the driving
incident contract and driven incident-store facet. Booking
`outages.go` applies relevant declarations and configured grace to its own deadline
state; Events `protection.go` does the same for event offers/reconfirmation. Expiry
workers use narrow guarded incident reads; health observations alone do not
declare an incident. Registering an incident and expiring protected claims share
a scope/date guard to give a defined ordering.
An already committed reassignment remains valid: recovery never resurrects a
former claim over its replacement. Grace ends at the booking's usable boundary.
The [policy record](../../policy-defaults.md#outage-protection) has no agreed
numeric recovery grace; configure it before enabling automatic expiry.

### Rooms, Exchange reconciliation and delegation

`booking/rooms.go` owns local pending holds and confirmed room claims.
`exchange` owns desired versus observed provider state. `graphcalendar` owns
Graph HTTP/authentication, payloads, paging and token handling. Neither Graph nor
an Exchange service can bypass Booking to insert an availability claim.

A room command commits its expiring local hold, desired meeting revision and
outbox work together. The outbound worker creates the event in the owner's
calendar, preserving the PA as the recorded actor. Creation is not acceptance:
the room can decline or await resource-delegate approval. The configured hold
deadline follows room workflow; approval-based rooms need an explicit longer
deadline. Microsoft distinguishes
[event creation](https://learn.microsoft.com/en-us/graph/api/user-post-events?view=graph-rest-1.0)
from [room processing policy](https://learn.microsoft.com/en-us/powershell/module/exchangepowershell/set-calendarprocessing?view=exchange-ps).

Webhooks persist observations or enqueue retrieval and acknowledge promptly.
Durable five-minute healthy reconciliation repairs missed notifications and
renews subscriptions. Track actual freshness per mailbox/window; a timer firing
does not prove successful synchronisation. Graph's
[calendar-view delta](https://learn.microsoft.com/en-us/graph/delta-query-events)
is window-scoped, and
[lifecycle notifications](https://learn.microsoft.com/en-us/graph/change-notifications-lifecycle-events)
include recovery cases. Persist cursors after durable application of pages and
support an explicit full-window rebuild after invalidation.

`calendar/merge.go` compares last agreed state, current desired revision and
current observed state. Merge compatible independent edits automatically, ignore
duplicate already-applied observations and refetch ambiguous out-of-order hints.
Provider version strings are opaque: do not sort `changeKey` values to infer
order. Room/time changes require fresh capacity validation. A genuinely competing
time or room intention creates visible conflict/recovery work, while preserving
the observed provider fact rather than discarding it as invalid local data.

Acceptance after hold expiry cannot blindly confirm: Booking reacquires capacity
and validates the current desired revision. If capacity was reassigned, retain
the accepted external event observation, mark divergence and apply the supported
reconciliation workflow. A delayed acceptance for a superseded room request
does not approve its replacement. Pending room changes explicitly expose which
claim is committed and what is awaiting acceptance.


If an ordinary meeting's replacement request fails after the previous room is
lost, retain the meeting and attendees and expose no secured room. Booking owns
the room intention/claim outcome; Exchange retains provider observations and
reconciliation progress. Notify the organiser and offer available replacements
or explicit cancellation. A selected replacement remains pending until current
room acceptance and Booking capacity validation succeed. The
[room recovery contract](../api-surfaces/room-recovery.md) distinguishes this
outcome from uncertain provider state and preserves Events-managed venue rules.

Graph documents an event
[version and transaction identifier](https://learn.microsoft.com/en-us/graph/api/resources/event?view=graph-rest-1.0),
but its [update reference](https://learn.microsoft.com/en-us/graph/api/event-update?view=graph-rest-1.0)
does not establish the event CAS or atomic room-swap contract this application
would need. Validate supported conditional behaviour in the authorised test
tenant. Until then use serialized local intentions, narrow updates, provider
readback and reconciliation; a local lock cannot exclude an Outlook user.
No proposal promises retention of the old external room until a replacement
accepts. Organiser cancellation and deleting a room copy also need distinct
[cancel](https://learn.microsoft.com/en-us/graph/api/event-cancel?view=graph-rest-1.0)
and [delete](https://learn.microsoft.com/en-us/graph/api/event-delete?view=graph-rest-1.0)
handling.

`identity/delegation.go` evaluates source-labelled grants for actor, owner, action
and office. Native-derived authority must be verified within the configurable
fifteen-minute limit for a new action. Room admission, human calendar delegation
and backend mailbox access are separate. App-managed grants authorise app actions
without creating native Outlook permissions; revoking a native source removes
that source grant without deleting an independent app grant.
Microsoft's [delegate example](https://learn.microsoft.com/en-us/graph/outlook-create-event-in-shared-delegated-calendar)
keeps the calendar owner as organiser, but the application's actor audit is still
required. App-only success does not prove the PA had native authority.
[Application RBAC](https://learn.microsoft.com/en-us/exchange/permissions-exo/application-rbac)
also does not narrow additive unscoped Entra grants automatically.

Reauthorise unsent human-requested writes when work begins: an expired/revoked
grant prevents new acting-on-behalf actions. Reconciliation of already committed
provider truth and necessary compensation use separately scoped system authority
and preserve the initiating actor. Revocation does not make the system ignore an
accepted event or secretly erase existing bookings.

### Entra, actual SCIM and Workday source authority

`http/scim` implements authenticated `/Users` and `/Groups`, filters, paging,
PATCH/error semantics and Entra compatibility through `driving.Provisioning`.
It is an inbound driving adapter, not an Entra polling client renamed SCIM.
Provisioning credentials are separate from staff SSO. `identity/scim.go` owns
resource creation/update/disable/reactivation/deletion and explicitly mapped
group membership; not every Entra group becomes a SpaceWeave team.
[RFC 7644](https://www.rfc-editor.org/rfc/rfc7644.html) and
[Entra's profile](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups)
define the external contract to test.

Internal staff IDs remain stable across contact changes. Retain Entra `(tid,oid)`,
Workday identifiers and an organisation-approved verified correlation. Ambiguous
matches wait for review. SCIM server `id` is stable; client `externalId` is scoped
to its provisioning domain, not a universal identity key. Email/username remain
mutable contact fields, as distinguished by the
[SCIM schema](https://www.rfc-editor.org/rfc/rfc7643.html#section-3.1).

Store source-qualified facts independently. Entra account/provisioning security
and effective Workday workforce eligibility can each deny access; a positive
update from one source cannot clear another's disable. Workday owns workforce
lifecycle, structure and usual office; SpaceWeave owns attendance preferences and
explicit travel. Map workforce organisation/team facts deliberately against
mapped Entra memberships rather than letting the most recent feed overwrite a
shared field. [ADR 0018](../../adr/0018-correlate-staff-with-explicit-source-authority.md)
and [ADR 0019](../../adr/0019-provision-entra-users-and-groups-through-scim.md)
establish those responsibilities.

Effective time differs from receipt time. Keep import checkpoints, as-of dates,
source freshness and future-effective changes; periodic full reconciliation
repairs increments that do not include related organisation/location updates.
An absent field or failed feed does not deactivate staff. Microsoft's
[Workday reference](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/workday-integration-reference)
and [configurable mappings](https://learn.microsoft.com/en-us/entra/identity/saas-apps/workday-inbound-cloud-only-tutorial)
do not establish this organisation's identifiers, population or latency, nor a
Workday-to-SpaceWeave SCIM transport.

Observed effective deactivation invalidates existing staff sessions and new
access, not only the next login. Identity commits source/eligibility changes,
access-epoch advancement and session invalidation in one owned transaction.
Requests check its current epoch; there is no cross-owner Access participant. Durable
follow-up releases future personal desk/parking claims through Booking and opens
room/event handover review. Recheck eligibility at claim commit during cleanup
lag. A SCIM delete removes the provisioning resource while retained booking
history remains subject to policy; it does not automatically erase staff history.

### Visitors, events and reporting

Visitors authenticate by verified email links restricted to visit/event purposes,
never by impersonating staff. Tokens are bounded by audience, expiry and allowed
operations. Visitors owns host/window/reception state. Events owns event audience,
registration rules, named tickets and admission; reception arrival and ticket
admission are separate records.

Venue changes, ticket issuance and admission use Events' event guard. A venue's
capacity is an upper bound on event capacity; issued tickets cannot be silently
invalidated by a smaller replacement venue. The coordinator either validates the
new venue/capacity against current commitments and changes both local owners
together, or rejects the move. Confirmed venue loss retains tickets pending
recovery and pauses admission/further issuance. The organiser explicitly secures
a suitable replacement or cancels, communicating confirmed changes to attendees.
Admission checks the venue-recovery pause and uses a unique ticket/admission key
so concurrent scans return the recorded admission without granting two entries.
A unique event/attendee identity also prevents duplicate active named tickets.

A visitor ticket is publishable only after a valid visit link exists. Use a
named shared local issuance transaction through Visitors and Events facets:
a registration-created visit and ticket commit together or neither does. A
reused existing valid visit survives failed issuance; retries recover the
same linked outcome. If verification or external email
is pending, the ticket remains unpublished and no email is sent inside that
transaction. Host deactivation raises review rather than inventing a new host.

Reporting consumes committed facts or published snapshot queries, not unrestricted
foreign-table joins in request handlers. Attendance reports distinguish booking,
identified check-in, anonymous desk occupancy and office-presence evidence with
freshness and provenance. Staff choose attendance; no minimum or anchor-day
compliance engine is introduced. Office-scoped reporting uses capabilities plus
allowed offices, while global settings need explicit global capabilities.

Retention is independently configured for visits, raw occupancy and attendance.
Jobs remove identifiable projections, extracts, delivery payloads and debugging
copies as well as owning rows; keep minimal non-identifying deduplication markers
where necessary to prevent replay recreating erased personal data. Rebuilding a
projection obeys original retention cutoffs. Backups have a declared expiry and
restore reapplies deletion/retention checkpoints before serving identifiable data.

### Floor-plan pipeline and revisioned assets

The current workflow already distinguishes ingest, conversion, tracing, review
and publication. Preserve its drawing fixtures, fuzz cases and renderer checks
while splitting mixed I/O and rule responsibilities. `core/geometry` contains
readers/geometry/value transforms; file opening, subprocess validation, provider
SDKs and catalogue persistence move behind driven ports. The existing
[converter](../../../internal/convert/convert.go) opens files and writes outputs;
the [Claude agent](../../../internal/trace/agent.go) embeds a provider client.
Neither can be copied wholesale into an infrastructure-independent core.

`plans/processing.go` coordinates conversion/tracing as durable leased work;
`drawingconvert` uses bounded scratch storage and `tracing/automatic` or
`tracing/claude` implements the selected engine. Automatic subpackages retain
their tested algorithms; moving them is a proposed extraction, not a requirement
to rewrite them. `nodevalidator` preserves the existing JavaScript validation
seam. Late progress/completion uses attempt and lease revisions so an expired
worker cannot overwrite a newer draft.

`driven.Blobs` stores immutable originals, drawings, approved geometry and renders
using one configured active Azure or GCS provider. Its contract exposes opaque
revision/concurrency tokens, checksums as separate data, private read access and
bounded signed access. Azure supports
[ETag conditions](https://learn.microsoft.com/en-us/azure/storage/blobs/concurrency-manage);
GCS supports [generation preconditions](https://docs.cloud.google.com/storage/docs/request-preconditions).
A token is not a portable checksum or recovery acknowledgement.

Upload first, verify its required recovery copy, then commit durable metadata or
publication acknowledgement. Interrupted steps remain explicit pending work;
reconcile orphan uploads and missing copies. There is no transaction across
PostgreSQL and either blob provider. `blob/verified` wraps the active adapter and
a deployment-selected recovery destination; selecting Azure versus GCS does not
require writing every asset to both providers. Published revisions never mutate
in place. Plans approves immutable assets; Workplace publishes their stable-space
mappings through `core/service/workplace/mapping.go` and its own driving/store
ports. Booking validates dated inventory and commitment impact before activation.
An admin operation may coordinate approval then activation, reporting distinct
outcomes; an approved asset alone cannot change bookable definitions.

Local scratch is disposable and cannot be the acknowledged source of truth.
Kubernetes documents [Pod volume lifetimes](https://kubernetes.io/docs/concepts/storage/volumes/).
The existing filesystem catalogue can remain a development/import adapter during
migration; its lock/rename protocol is not the production cross-Pod job queue.

### Durable work, events and private Teams delivery

Core services append versioned facts and jobs through a transaction-bound outbox
facet. `jobs/runner.go` claims leased work, calls driving ports and acknowledges
with lease/version fencing. Polling and scheduler nudges are interchangeable
triggers; the durable row determines work. `SKIP LOCKED` is suitable for queue
claiming, never the availability decision. Use bounded retry/backoff, dead-letter
or operator-visible failed state, cancellation checks and per-handler idempotency.
A crash after external success but before acknowledgement can repeat a call.

Booking events carry IDs, owner/actor references, office, committed revision and
minimal payload; they do not carry provider tokens or visitor email by default.
Each projection/delivery consumer has its own checkpoint or deduplication key.
Projection delay never grants capacity. Retention constrains replay and payload
lifetimes. Local atomic outbox creation cannot guarantee exactly-once external
effects; external retry identities and readback/reconciliation remain necessary.

Notifications stores email/in-app preferences and the separate opt-in for a
private Teams message that a booking was raised. It selects recipients using
committed owner/actor semantics and checks the recipient's message-specific
preference again immediately before dispatch. Installation is technical access,
not consent. Private conversation IDs are mapped to the intended staff identity;
never infer them from email or use a shared channel.

`teams/personal.go` is a driven adapter behind a narrow delivery contract.
Proactive personal delivery needs an installed app and usable conversation;
handle blocked/uninstalled recipients and throttling as visible outcomes.
Microsoft's [proactive messaging guidance](https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/conversations/send-proactive-messages)
supports this mechanism. Ordinary
[Graph chat posting](https://learn.microsoft.com/en-us/graph/api/chat-post-messages?view=graph-rest-1.0)
does not supply general app-only background messaging. Do not substitute an
activity-feed alert or expand opt-in to other notifications. Delivery failure
does not undo a booking or create an extra fallback email automatically.

### Composition root, generated transport and React portals

`app/adapters.go` opens PostgreSQL, identity clients, provider-specific blobs and
telemetry from strict deployment configuration. `services.go` constructs one
instance of each core service with restricted store facets and wires permitted
sibling interfaces. `subscriptions.go` registers typed job/event handlers.
`roles.go` starts only the drivers selected for HTTP, allocation, Exchange, tracing
or maintenance. One image contains the shared implementation; process role is
not a separate service architecture or owner of a second capacity store.

For example, `booking.New` receives capacity/UoW access, current eligibility,
policy/workplace queries, clock and outbox. `exchange.New` receives Exchange
state, the provider port and Booking's reconciliation interface. `identity.New`
receives distinct people/access storage facets and the native-authority port.
`events.New` receives event state plus a narrow Booking venue
participant. Constructors never receive the entire runtime. Avoid a service
locator or context that lets a core service discover arbitrary SQL/adapters.

The CLI exception lets `serve --role exchange`, migrations and local tools start
composition. Core operations remain available through driving ports for tests
and HTTP without Cobra. `cmd/spaceweave/main.go` stays small. Deployment config
uses Cobra without Viper and flag > environment > strict YAML; versioned runtime
policies are database records managed through Policy, not config-file edits.
Publication validates values, cross-field consistency and permitted office
overrides. New bookings/cycles use effective versions; existing confirmations
retain their referenced policy unless explicitly migrated.
These retain the repository's [conventions](../../../CLAUDE.md).

OpenAPI sources generate adapter-local Go server interfaces and optional public
Go clients, plus `web/shared/src/api/generated` TypeScript clients. Handwritten
HTTP adapters convert request/response values, errors, dates and paging into
driving DTOs. Core contracts never import generated or Graph/SCIM wire types.
SCIM wire compatibility remains a separate external contract even if its schema
uses the same generation tool. Generation checks detect drift, not domain truth.

Staff/mobile-web and admin have separate React entrypoints with shared UI and
client wrappers. Go retains Entra tokens and owns secure sessions; browser code
uses session cookies with backend CSRF and authorisation controls. Hiding an
admin route grants no authority. Visitor links use their restricted audience.
Wrap the current renderer with one dedicated container: React owns its shell,
the renderer owns that container's children, following React's
[DOM ownership guidance](https://react.dev/learn/manipulating-the-dom-with-refs).
The D3 renderer does not mutate React's sibling controls or decide availability.

### Tests and enforcement

| Evidence | Proposed home and boundary exercised |
| --- | --- |
| Pure rules | Adjacent core tests: deterministic packing, hard constraints, merge decisions, source authority, deadline arithmetic |
| Service behaviour | Adjacent service tests using restricted port fakes; no claim that a fake establishes SQL isolation |
| PostgreSQL contract | `contract/storetest`, run against real PostgreSQL/Testcontainers for constraints, UoW rollback, retries, leases and deduplication |
| Capacity races | `test/concurrency`: allocation/direct/cancel, owner overlap, pooled parking, transfer and waitlist precedence |
| Check-in races | Personal check-in/no-show/offer/incident registration, stale sensor evidence and expired leases |
| Venue/visitor safety | Smaller venue versus ticket issue, duplicate scan, visitor-link rollback/retry, invalid external venue |
| Provider adapters | Chi-based Graph/SCIM/Workday/Teams test servers for wire/failure contracts; authorised tenant tests for real Exchange semantics |
| Blob contract | Same suite for Azure/GCS and recovery wrapper: conditional writes, immutable revisions, copy verification, orphan recovery |
| Portal/plan regression | Existing renderer/geometry/fuzz fixtures plus session/CSRF/office-scope and upload/review acceptance |
| Whole system | Queue/process/zone failure, regional promotion, routing/identity/blob recovery and representative load |

Testcontainers and real dependencies are required by
[CLAUDE.md](../../../CLAUDE.md); preserve the ≥95% total and ≥90% per-package
coverage gate in [.testcoverage.yml](../../../.testcoverage.yml). Port-only files
have no executable statements; do not invent tests merely to inflate coverage.
Real provider semantics need tenant evidence, not a simulator that simply encodes
the desired conditional-update or room-swap behaviour.

`layout_test.go` checks the ten dependency rules using the actual Go package
graph, with the CLI composition exception named explicitly. An allow-list checks
which constructor receives `CapacityWriter`, and SQL-ownership checks map known
queries/migrations to owner files. Compile assertions verify port implementations.
CI generates contracts and checks for drift, runs golangci-lint, builds with
`GOWORK=off`, and retains existing OS/latest-runner and pinned-action conventions.
Review remains necessary for semantic ownership hidden inside a broad interface.

### Kubernetes, observability and regional recovery

HTTP, allocation, Exchange, tracing and maintenance use separate Deployments from
one image/release. Each has bounded concurrency, readiness/liveness appropriate
to its role, graceful lease draining and a scoped workload identity. Federation
and organisational secret management avoid baking provider credentials into the
image. Run one controlled migration Job per release with expand/contract schemas
and versioned jobs/events compatible across rolling API/worker releases.

The core target remains 99.95% monthly availability and one-second p95 initial
responses at the agreed representative workload for 100,000 staff/60,000 desks.
Measure queue completion and room acceptance separately. Allocation workers use
bounded batches; tracing CPU/memory and external-provider latency must not exhaust
HTTP/database pools. Package organisation does not establish these targets.

Instrument driving operations, UoW outcomes and provider attempts with trace
correlation. Export traces/metrics through OpenTelemetry and a Collector, using
JSON structured logs forwarded to the organisation's backend; the
[Go exporter guidance](https://opentelemetry.io/docs/languages/go/exporters/)
recommends production Collector use. Expose lock waits, retries, allocation
unmet demand, oldest job/outbox age, lease loss, Exchange actual freshness,
recovery-copy lag, active outage intervals and notification failures. Avoid staff
IDs, visitor emails and ticket tokens as metric labels or routine log fields.

One active write region uses multi-zone database HA and a warm recovery region.
Five-minute RPO/fifteen-minute RTO are whole-system targets, including published
assets, jobs, provider identities, keys, routing and workers. Verification of
recovery copies precedes durable/publication acknowledgement. Asynchronous
replication is not a zero-loss promise; PostgreSQL explains
[failover coordination](https://www.postgresql.org/docs/current/warm-standby-failover.html)
and [replication tradeoffs](https://www.postgresql.org/docs/current/warm-standby.html#SYNCHRONOUS-REPLICATION).

If recovery lag exceeds budget, keep a healthy primary serving and alert; an
operator explicitly chooses any failover beyond the loss budget. Fence former
database writers and external-write worker identities before enabling recovery
writers. A database lease restored from an older replica is not sufficient
cross-region fencing. Reconcile uncertain Graph/Teams effects and resume jobs
using durable revisions; apply confirmed check-in/offer outage protection without
overwriting existing reassignments. Rehearse complete recovery and retention
reapplication; a successful database promotion alone is not the RTO test.

### Workplace retirement, reactivation and deletion

`internal/core/service/workplace/retirement.go` handles explicit retirement and dependency-checked removal
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

### 1. Commit a group while direct requests and cancellations change capacity

1. `scheduler` calls `driving.Allocation.Start`; Booking creates a cycle/run job
   using its effective office policy. A duplicate trigger returns the same run.
2. An allocation worker calls `Snapshot`; Booking reads consistent dated
   inventory/intents/team/workplace inputs and records their revisions.
3. `rules/allocation` forms groups and packs a proposal outside a write
   transaction, including every both-required parking dependency.
4. `driving.Booking.Cancel` resolves the pair under Booking guards and creates
   committed waitlist work.
   Before either starts, a both-required cancellation ends both atomically; an
   optional link cancels only the selected booking.
   A premature ordinary direct request becomes intent; after the main run it
   still cannot jump eligible waitlisted demand.
5. `CommitGroup` locks resource/owner/pool guards in canonical order, reloads
   changed inputs and validates all members. Its one UoW either writes all desk
   and required parking claims plus outcomes/outbox, or writes none.
6. A competing claim makes that group stale/unavailable. Booking replans affected
   uncommitted groups; groups already confirmed remain protected. The response
   distinguishes committed, pending repair and unmet demand.

### 2. Check-in races no-show release, promotion and an outage

1. `http/booking/checkin.go` sends the authenticated actor and booking ID through
   `driving.Booking.CheckIn`; Identity requires that actor to be the owner.
2. A due no-show job calls `driving.Booking.ReleaseNoShow` with the expected
   claim revision. Both commands lock the same claim and scope/date guards.
3. If check-in commits while valid, the expiry observes taken state and does
   nothing. If release/reassignment committed first, check-in returns its current
   unavailable outcome; it cannot steal the replacement's booking.
4. Automatic desk release retains linked parking until its own end or explicit
   release. Promote eligible waitlist demand only against actually released
   capacity, atomically or through a guarded next action. Fresh direct requests
   use the same protocol.
5. The driving occupancy adapter calls Booking, which validates/deduplicates
   operational evidence, persists it and commits facts for Reporting. Anonymous
   occupancy is not owner check-in and cannot stop a no-show release. A missing
   sensor feed cannot trigger a taken-desk vacancy release.
6. An operator-confirmed affected outage interval pauses uncommitted no-show and
   offer expiry decisions. On recovery, workers recalculate deadlines with the
   configured grace and usable period. Already committed reassignments survive;
   declare uncertain incident ordering visibly rather than rewriting history.

### 3. A PA requests a room; late acceptance and Outlook edits arrive

1. `http/booking/rooms.go` calls `driving.Rooms.Request(actor=PA, owner=staff)`.
   Identity checks source-labelled authority/freshness for that action and office.
2. Booking commits a pending local hold, retry result, desired Exchange revision
   and work. The HTTP response says pending; it never invents room acceptance.
3. The Exchange worker rechecks authority for unsent work and calls
   `driven.Exchange` through `graphcalendar`. The owner is organiser; the PA
   remains audit actor. Retry uses the same operation identity and reconciliation.
4. A webhook indicates an external edit and an older acceptance arrives later.
   Exchange persists both observations, fetches current state where ordering is
   uncertain and merges compatible field changes against the last agreed state.
5. Acceptance reaches Booking with occurrence/request revision. It confirms only
   if the relevant hold/current capacity is still valid. An expired/reassigned
   hold produces divergence and recovery, not a duplicate capacity claim.
6. A native delegation revocation removes that source grant; independent local
   grants remain. New PA edits fail if no current sufficient grant exists.
   System reconciliation retains accepted provider truth and handles compensation.
7. A conflicting room/time edit requires an explicit validated transition. If a
   replacement fails after old-room loss, retain meeting/attendees with no secured
   room, notify the organiser and offer replacement choices or cancellation.
   A replacement remains pending until current acceptance and capacity validation.

### 4. Move an event while issuing a visitor ticket and scanning admission

1. Admin's venue move enters `driving.Events.ChangeVenue`; a concurrent attendee
   registration enters `IssueTicket`. Both acquire the same event guard.
2. The coordinator asks Booking's venue participant to validate/secure the new
   venue inside the shared UoW. Booking alone can change its claim rows.
3. Events compares proposed capacity with current issued commitments. If the
   smaller venue cannot contain them, reject the move and keep the old local
   venue; otherwise update venue association and capacity atomically.
4. Visitor issuance verifies permitted audience/registration and a visit link
   through Visitors' participant. Store the named ticket/link/retry result
   together; send the usable ticket only after commit.
5. Replayed issue requests reuse that pair. Concurrent scanners insert one
   admission record; the second sees already admitted and cannot grant another
   entry. Reception arrival is recorded separately.
6. Confirmed venue loss retains tickets pending recovery and pauses admission/new
   issuance until an explicit organiser replacement or cancellation; communicate
   confirmed changes. No ordinary booking edit bypasses these checks.

### 5. SCIM deactivation and effective workforce change during an outage

1. Entra's authenticated SCIM PATCH maps to `driving.Provisioning.ApplyUser`.
   Identity records source status and advances eligibility/access epoch in its
   transaction; existing sessions now fail the current-eligibility check.
2. Workday supplies a future-effective usual-office/team update through its
   driven adapter. Identity preserves its source IDs and effective time;
   verified correlation finds the same internal staff ID without email matching.
3. Another source or related-entity page fails. Persist successful progress only
   at its safe boundary, preserve last known facts for failed parts and surface
   freshness. Absence is not deletion; the Workday change cannot clear Entra's
   disable or rewrite explicit travel preferences.
4. When effective, the workforce update refreshes unconfirmed defaults and future
   snapshots. Confirmed bookings retain their history and explicit choices.
5. Idempotent lifecycle work releases future personal desk/parking via Booking
   and opens room/event handover review. Booking commit checks deny new claims
   even if cleanup is still queued. Reconciliation later repairs the partial feed.

### 6. Add the private booking-raised Teams adapter

1. `driven.Delivery` gains a narrow private booking-raised capability only if
   the existing contract cannot express it safely. Core consent stays in
   `notifications/preferences.go`; transport installation is separate evidence.
2. Implement `adapter/driven/teams`, its delivery contract suite and composition
   configuration. No booking capacity or PostgreSQL booking query changes.
3. A committed booking-raised fact creates a notification intent. Email/in-app
   keep their own preference/deduplication rules; Teams is an additional opted-in
   private delivery for that specific message and intended recipient.
4. Immediately before dispatch, recheck opt-in, identity and usable conversation.
   Opt-out suppresses queued delivery; administrative app installation cannot
   turn it back on.
5. Retry transient throttling within bounds; record blocked/uninstalled failure
   visibly. A crash after sending can leave uncertainty, so do not promise
   exactly-once chat delivery. Booking remains confirmed and no extra fallback
   email is invented.

## Package mapping

Current packages map to proposed destinations below. Tests and fixtures move
with their behaviour; introduce working seams before a bulk rename.

| Current package/path | Proposed location | Required split or retained responsibility |
| --- | --- | --- |
| `cmd/spaceweave` | same | Delegate command tree to driving CLI; main handles exit. |
| `internal/cli` | `adapter/driving/cli` plus `app` | Cobra parsing stays outside core; provider construction/runtime startup moves to composition. |
| `internal/buildinfo` | same | Version information stays outside core. |
| `internal/config` | same | Strict deployment config; runtime policy records are new core behaviour. |
| `internal/server` | `adapter/driving/http/{plans,session}` and root | Keep Chi/upload limits; replace catalogue access with Plans port and add production access/session controls. |
| `internal/catalog` | `core/model`, `service/plans`, driven persistence | Separate status/review rules from JSON/filesystem locking; replace production catalogue/job authority with PostgreSQL. |
| `internal/inbox` | `adapter/driving/inbox` | Watch/rescan/settling remain a driver; ingest through Plans port. |
| `internal/worker` | `adapter/driving/jobs`, `service/plans`, driven tracing/conversion | Separate process polling from processing decisions and file/provider I/O. |
| `internal/trace` | `adapter/driven/tracing/claude` plus core drawing values | Provider SDK/options/transcripts stay outside; no Anthropic type in core ports. |
| `internal/autotrace` | `adapter/driven/tracing/automatic` | Adapt its current shared trace contracts; retain automatic pipeline behaviour. |
| `internal/autotrace/{cadfront,rasterfront,score,synth}` | corresponding `automatic` subpackages | Preserve tested geometry algorithms and fixtures; inward references use core geometry. |
| `internal/autotrace/profile` | `core/geometry/profile` and `adapter/driven/profilefiles` | Pure layer/profile values split from filesystem library. |
| `internal/{cad,dwg,dxf,floorgraph,raster,svgdraw,vector}` | corresponding `core/geometry` packages | Keep parsers/transforms; peel off incidental file/provider access before moving. |
| `internal/plan` | `core/geometry/plan` and `adapter/driven/nodevalidator` | Plan values/conversion/validation rules versus external Node execution. |
| `internal/convert` | `adapter/driven/drawingconvert` | File/temp/output handling stays driven; reuse core CAD primitives. |
| `internal/fakeclaude` | `adapter/driven/fakeclaude` | Test-only adapter, no runtime import. |
| `internal/{dwg/dwgtest,dxf/dxftest}` | adjacent geometry test helpers | Retain fixture builders as needed; omitted from representative count. |
| `web/floorplan-iso` | retained, wrapped by shared renderer | Incremental React adoption, preserve DOM ownership and rendering evidence. |
| `test/acceptance` | retained and expanded | Existing drawing checks survive; real booking/provider/recovery suites are additions. |
| `pkg` | optional generated `pkg/client` only | Current placeholder does not imply published core models. |
| No current booking/identity persistence | proposed core/ports/services/PostgreSQL | New implementation guided by behaviour and real dependencies, not a directory-only migration. |

## Numbers

Counts cover the first full package tree only, excluding repeated examples.
A package is a directory containing a named Go file; generated and test-only
packages are included. They are not implementation-effort estimates.

| Measure | Printed candidate |
| --- | --- |
| Named Go package directories, including command/client/test-only packages | 86 |
| Core packages | 30 |
| Core service packages | 12 |
| Driving adapter packages | 21 |
| Driven adapter packages | 21 |
| Reusable contract packages | 6 |
| Driving-port source files | 16 |
| Driven-port source files | 21 |
| Named PostgreSQL migration files | 9 |
| Generated Go packages | 5: three server, two client |
| Named runtime role Deployment manifests | 5 |
| Explicit dependency rules | 10, including the CLI composition exception |

No line-count or throughput prediction follows from these counts. The shared
model/ports/SQL/composition packages are likely coordination hotspots; actual
size and contention need measurement. Adding a provider primarily touches a
driven adapter, contract tests and composition. Adding business behaviour often
touches model, both port sides, service, persistence, transport and tests.

## Placement choices and friction

- **One system-wide hexagon.** Provider/transport replacement is the principal
  seam. Splitting every business area into a private module with its own adapter
  namespace would change the comparison into the modular-monolith option.
- **Shared model and centralized port packages.** They make inward dependencies
  visible and reduce tiny interface packages. They also couple unrelated callers
  to a growing catalogue; keep files focused and avoid one universal request DTO.
- **One PostgreSQL implementation with logical owners.** Shared transactions
  make group placement, linked parking and venue/ticket invariants concrete.
  Ownership depends on facets, query checks and review because one package and
  credential cannot provide strong owner isolation by themselves.
- **Booking is the physical-space claim writer, including planner commits.** This preserves
  authority across direct booking, allocation, Exchange and events. The resulting
  Booking service is substantial and needs internal decomposition; distributing
  claim SQL among adapters would weaken correctness.
- **Explicit sibling collaboration.** Identity, Booking and Events use
  interfaces supplied by composition. This is real type-level inversion, but all
  implementations still live in one core, so the extra import rule matters.
- **Named multi-owner local transactions.** Venue changes and visitor issuance
  use restricted participants. This avoids overselling owner independence;
  transaction scope and lock ordering are shared design responsibilities.
- **Geometry inside, conversion/tracing mechanisms outside.** Existing code mixes
  pure algorithms, files and provider options. Extraction costs real work; the
  architecture does not justify rewriting proven parsers or renderer algorithms.
- **Outbox polling is a driver; persistence is driven.** Core subscribers decide
  business effects. A handler registry must remain explicit so arbitrary event
  subscriptions do not quietly bypass owner interfaces or retention rules.
- **No in-memory production parity claim.** Small fakes help rules/handler tests,
  while PostgreSQL contracts prove constraints and races. A second general store
  implementation would add maintenance without reproducing database isolation.
- **Composition concentrates change.** New services, providers and roles meet in
  `app/services.go`, `adapters.go` and subscriptions. Split these by wiring concern
  without turning composition into a hidden business-workflow layer.
- **A feature spans the perimeter.** Room work appears in driving HTTP, core
  contracts/services/rules, PostgreSQL and Graph adapters. Review locality is
  weaker than a domain-owned subtree, even when each file has a clear role.
- **External consistency remains visible.** Graph acceptance, authority freshness,
  Teams delivery and recovery copies are asynchronous facts. Ports express their
  uncertainty; neither a hexagon nor one local transaction removes it.

This option offers a clear, testable perimeter around the entire application.
Its main review question is whether that perimeter is sufficient when business
ownership relies on enforced facets and shared-package discipline rather than
language-protected module interiors.
