# Architecture decision analysis for SpaceWeave

SpaceWeave is a workplace booking application for one enterprise, targeting
approximately 100,000 staff and 60,000 desks across many offices. Staff select
attendance dates; the platform groups advance demand and packs groups onto
building/floor inventory. It also supports direct space booking, linked parking,
meeting-room synchronisation, hosted visitors, internal events and attendance
reporting through separate staff and administration web applications.

This analysis explains which architectural properties matter for that system.
It is written for technical architects and engineers evaluating consistency,
modularity, integration and operations. The [five detailed candidates](architecture-options/README.md)
provide full repository layouts, dependency rules, walkthroughs and migration
maps. [Architecture foundations](architecture-options/architecture-foundations.md)
explains the industry principles and primary sources behind each style.

## Requirements driving the choice

| Requirement | Architectural consequence |
| --- | --- |
| One overlapping desk per staff member | Every local capacity writer must enforce the same person/time constraint, including cross-office transfers. |
| Group placement and required parking commit atomically | Planning may be expensive and asynchronous, but acceptance needs a bounded shared consistency protocol. |
| Confirmed bookings survive replanning | A proposal and a committed claim must be different states with explicit authority. |
| Check-in races no-show release | The booking owner decides the transition against current state; anonymous occupancy is not staff identity. |
| Event capacity and ticket issue remain consistent | Venue changes and issuance must share a consistency boundary or an explicit coordination protocol. |
| Exchange Online changes both ways | Local desired state and observed provider state need durable correlation, progress and reconciliation. |
| Entra and Workday own different identity facts | Source authority, stable correlation and observed effective deactivation must be explicit. |
| UI and automation share supported REST | Wire DTOs, domain interfaces, identity profiles and compatibility have separate owners. |
| Multiple Kubernetes roles use one release | Concurrency, idempotency and failure recovery cannot depend on one process's memory. |

The platform uses Go/Chi, PostgreSQL durable state/jobs/outbox, React/TypeScript
portals and Azure/GCP blob adapters. Core availability, booking and check-in
have a 99.95% monthly availability target and one-second p95 initial-response
target at representative load. A single active write region uses multi-zone
availability and warm recovery, with five-minute RPO/fifteen-minute RTO targets.
These are product/operational requirements; staff and desk counts alone do not
establish request peaks, hot-office contention or allocation completion time.

## Comparing the approaches

The five approaches answer different questions. Layering organises technical
responsibilities. Hexagonal architecture protects the application boundary from
external technologies. Clean Architecture directs source dependencies toward
business policy. Domain-driven design establishes model and invariant boundaries.
A modular monolith encapsulates business capabilities inside one shared release.
They can be combined; the candidate layouts make different combinations concrete.
[Primary framework analysis](architecture-options/architecture-foundations.md).

| Candidate | Main benefit in this domain | Main engineering risk |
| --- | --- | --- |
| Layered | Familiar technical placement and straightforward application-owned transactions. | Feature ownership spans model/service/persistence roots; a shared store can become a second booking writer. |
| Hexagonal | Provider replacement and testing through application-owned ports. | One large core and shared port packages can obscure independent business ownership. |
| Clean | Explicit use-case inputs and inward dependency direction. | Many small interfaces and a broad mutation interactor can increase navigation/wiring cost without improving invariants. |
| DDD | Explicit language, context boundaries and aggregate behaviour. | Aggregate/context splits may conflict with atomic group, parking and venue/ticket requirements unless coordination is designed. |
| Modular monolith | Business ownership and local consistency align with package navigation and migration custody. | A large Booking module and permitted cross-module transactions create coupling that needs active control. |

The preferred direction is a domain-oriented modular monolith with ports and
adapters inside substantial modules. The reason is the ownership of local booking
invariants and integration state. The final module split remains open: placing
events inside Booking reduces shared transaction boundaries but increases that
module's breadth. Separating Events requires a precise venue/ticket protocol.
[Architecture decision](../adr/0009-use-a-domain-first-modular-monolith.md),
[modular-monolith candidate](architecture-options/5-modular-monolith.md).

## Transaction and workflow ownership

One owner arbitrates dated capacity. Allocation calculates against a versioned
snapshot outside long transactions; the owner revalidates and commits each group,
including required parking. Direct requests, waitlist promotion, check-in and
release use the same claim protocol. PostgreSQL constraints and locking protect
competing writers; transaction isolation choices require retry handling and
concurrency evidence. [PostgreSQL transaction isolation](https://www.postgresql.org/docs/current/transaction-iso.html),
[constraints](https://www.postgresql.org/docs/current/ddl-constraints.html).

Booking creation, ticket issue and work submission require retry identities.
Acceptance receipts, domain changes and durable follow-up work commit together.
A response cache written later leaves an ambiguity window after a successful
business commit. External calls happen after commit and need their own duplicate
handling/reconciliation. [Atomic acceptance decision](../adr/0022-commit-retry-receipts-with-command-acceptance.md),
[request contract](api-surfaces/request-behaviour.md).

Exchange acceptance remains external. A local room hold is pending until the
room accepts; duplicate/out-of-order observations are reconciled against desired
and observed revisions. Provider identifiers and tokens use documented semantics,
not assumptions that every version field is a portable compare-and-swap mechanism.
[Provider contract research](api-surfaces/provider-contracts.md).

Allocation results distinguish unfulfilled capacity demand from technical
interruption. Cancellation stops additional work at safe boundaries and preserves
committed effects. Public operations expose progress/results, while queue payloads
and worker repair controls remain private. These semantics must remain the same
under every repository layout.

## Module boundaries, deployment and enforcement

A business module, Go package, Go module, runtime role and Kubernetes Deployment
are different boundaries. Several private packages can implement one business
module; several independently scaled roles can use that same module. Go's
`internal` visibility protects an ancestor tree but does not prevent all imports
between sibling domain packages. Ownership requires deliberate package visibility,
import rules, constructor capabilities and database access conventions.
[Go internal directories](https://pkg.go.dev/cmd/go#hdr-Internal_Directories).

Kubernetes can scale HTTP, allocation, Exchange, conversion and maintenance roles
separately. Scaling does not remove shared-database contention or create domain
encapsulation. Startup/readiness/liveness probes have different semantics; failures
of a recoverable provider should not automatically restart healthy booking
processes. [Kubernetes workload model](https://kubernetes.io/docs/concepts/workloads/),
[probe guidance](https://kubernetes.io/docs/concepts/configuration/liveness-readiness-startup-probes/).

Versioned wire contracts and private application interfaces evolve differently.
The public API supports independently upgraded clients, with an initial 180-day
major-version retirement notice. Private durable jobs and committed facts still
need rolling-release compatibility and recovery/replay handling.
[API contract engineering](api-surfaces/api-contract-engineering.md).

## Migration and validation

The current application is a local floor-plan conversion/tracing tool. Existing
geometry, renderer fixtures, review feedback and conversion behaviours are assets
to preserve. The booking authority, staff identity model, durable distributed work
and enterprise integrations are new capabilities. The [current inventory](api-surfaces/current-surfaces.md)
distinguishes implemented surfaces from proposed ones.

Validate the preferred architecture with four focused bodies of evidence:

1. **Consistency:** concurrent allocation, direct booking, check-in/release,
   parking and venue/ticket transitions against real PostgreSQL transactions.
2. **Provider recovery:** duplicate/out-of-order Exchange changes, lost
   notifications, revoked delegation and source identity outages in authorised
   provider test environments.
3. **Capacity:** office/date skew, scarce features, peak arrivals, queue drain,
   receipt/admission contention and HTTP latency under varying worker concurrency.
4. **Boundary and recovery:** import/data-ownership checks, migration compatibility,
   restart/replay and fenced regional recovery across database, assets and workers.

Use the results to choose module boundaries, transaction granularity and resource
limits. Package counts and architecture labels explain organisation; measured
correctness, change impact and recovery determine whether the design is suitable.
