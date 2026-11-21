# Architecture foundations for SpaceWeave

Research date: 19 November 2026. Audience: technical architects and engineers.
Status: foundation for five candidate layouts; the repository-architecture choice
is pending. Agreed logical owners apply across them; a modular monolith remains
the preferred candidate, subject to the
consistency, ownership and operational checks described here.

## The system being designed

SpaceWeave coordinates workplace use for one enterprise with approximately
100,000 staff and 60,000 desks across offices, buildings and floors. Staff and
administrators use separate React web entrypoints, including mobile web.
The backend uses Go and Chi, PostgreSQL, durable jobs and an outbox. Azure Blob
Storage or Google Cloud Storage holds versioned plan assets. Kubernetes runs
HTTP handlers and background processing roles from one application release.

The business scope includes desks, meeting rooms, parking, internal events and
hosted visitors. Advance attendance intents are planned together under hard
constraints, with collaboration, team proximity and packing objectives.
Planning first forms collaborator groups, including singletons, then packs them
into existing desks. Publish the completed, validated plan together, preserving
group and required-parking consistency; intermediate work stays provisional. Direct
requests, cancellations and waitlist promotions can compete for that inventory.

Room bookings synchronise bidirectionally with Exchange Online through
asynchronous reconciliation. Entra SCIM provisioning and Workday workforce
updates supply different identity and employment facts. These integrations
introduce delayed observations, retries and conflicting changes; provider
records cannot silently become an alternative local capacity writer.

These are **SpaceWeave requirements and technology choices**. The sources below
explain architectural principles; they do not independently establish the
product's scale, booking rules, provider choices or preferred package layout.

## Five approaches answer different questions

| Approach | Main design question | What a candidate must still decide |
| --- | --- | --- |
| Layered | Which technical responsibility belongs at each level? | Feature ownership, permitted layer skipping and transaction coordination. |
| Hexagonal | How does application behaviour communicate with external actors and technologies? | How the application interior is divided into business capabilities. |
| Clean | Which policies should be independent of lower-level mechanisms? | Package organisation, use-case granularity and data ownership. |
| Domain-driven design (DDD) | Where does each model and business invariant apply? | Deployment, adapters and the implementation of consistency boundaries. |
| Modular monolith | Which business capabilities are encapsulated inside a shared application release? | Exact module boundaries, internal styles and permitted coordination. |

The sections below establish those distinctions from primary sources. The
approaches overlap: a module can contain a domain model, use cases and ports
with database adapters. Comparing five layouts evaluates their concrete
organisation and enforcement; it does not require treating them as mutually
exclusive frameworks.

## Layered architecture

**Source principle.** Layers separate responsibilities and constrain dependency
direction. Microsoft's guidance distinguishes logical layers from physical
tiers: several layers can run in one tier. It also distinguishes closed layers,
which call the adjacent lower layer, from open layers, which may skip layers.
Neither a separate process per layer nor one universal folder vocabulary
follows from layering. [Microsoft: N-tier architecture](https://learn.microsoft.com/en-us/azure/architecture/guide/architecture-styles/n-tier).

A familiar arrangement separates presentation, application/business behaviour
and persistence. A dependency on a lower persistence layer differs from
dependency inversion, where persistence implements an interface defined by the
policy that consumes it. Layering alone does not select where every interface
must live; that requires an explicit candidate dependency policy.
[Martin: dependency inversion at boundaries](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html).

**SpaceWeave candidate.** Transport, application, model and persistence roots
make technical responsibilities easy to find. Booking ownership would span
those roots, so storage interfaces, SQL and migrations require a shared
ownership rule. Application-owned transactions and persistence-owned
interfaces are candidate decisions, rather than universal laws of layering.
The main risk is that a shared model or store gradually becomes everyone's
write surface. Evaluate [the layered layout](1-layered.md) against that risk.

## Hexagonal architecture: ports and adapters

**Source principle.** Cockburn's original design separates an application's
inside from external actors and technologies. A port describes a purposeful
conversation; adapters translate between that conversation and a particular
technology. Automated tests and batch drivers can drive the same application
behaviour as an interactive interface. The six sides are a drawing convention,
not a prescribed number of ports. [Cockburn: original hexagonal architecture](https://alistair.cockburn.us/hexagonal-architecture).

Driving ports initiate application work; driven ports express what the
application needs from its environment. The inside/outside boundary protects
business behaviour from HTTP, database and provider details. It does not
prescribe one domain model, one package for all ports, or one SQL adapter for
the entire enterprise application. Those are ways to instantiate the pattern.
[Cockburn: intent, structure and application notes](https://alistair.cockburn.us/hexagonal-architecture).

**SpaceWeave candidate.** HTTP, allocation jobs and Exchange observations can
enter through application contracts; PostgreSQL and provider APIs implement
outbound contracts. A central core is a possible comparison layout, while
separate domain modules can each use the same pattern. The risk of a central
core is shared-contract growth without business encapsulation. Inspect
[the hexagonal layout](2-hexagonal.md) for both kinds of boundary.

## Clean architecture

**Source principle.** Martin separates general business rules, application
use cases, interface adapters, and frameworks/drivers. Source dependencies
point toward higher-level policies. When execution calls outward, an inner
interface implemented outside preserves that dependency direction. Data
crossing a boundary uses simple representations suitable for the inner policy,
rather than importing a database framework's row type. [Martin: The Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html).

The four circles are schematic; Martin explicitly permits additional circles.
The article does not prescribe one global entity package, a package per
operation family, or a universal prohibition on use cases collaborating.
Such restrictions must be justified as local rules. [Martin: dependency rule and four circles](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html).

**SpaceWeave candidate.** Booking use cases can coordinate pure eligibility and
capacity rules through narrow storage and integration contracts. The structure
makes framework independence explicit, but its rings do not themselves assign
table ownership. Concentrating atomic booking changes in one interactor family
is a consistency choice; its interface and responsibilities still need review.
Inspect [the Clean layout](3-clean.md) for useful seams versus repeated wiring.

## Domain-driven design: model and consistency boundaries

**Source principle.** DDD starts with the business domain and a shared language.
A bounded context defines where a particular domain model applies; the same
word may legitimately mean different things in another context. An aggregate
is a smaller consistency boundary around entities and values, with one root
controlling access and transactional invariants. A context can contain several
aggregates. [Microsoft: domain analysis](https://learn.microsoft.com/en-us/azure/architecture/microservices/model/domain-analysis),
[Microsoft: tactical DDD](https://learn.microsoft.com/en-us/azure/architecture/microservices/model/tactical-domain-driven-design).

Microsoft's tactical guidance recommends small aggregates and eventual
consistency between them. Its domain-event guidance also discusses the tradeoff
between a single transaction across aggregates and eventual consistency,
including synchronously handled events before commit. A strict aggregate rule
is a modelling position with consequences, rather than a reason to discard a
required business invariant. [Microsoft: aggregate transaction choices](https://learn.microsoft.com/en-us/dotnet/architecture/microservices/microservice-ddd-cqrs-patterns/domain-events-design-implementation#single-transaction-across-aggregates-versus-eventual-consistency-across-aggregates).

**SpaceWeave candidate.** Workplace definitions, dated capacity, planning,
identity, visitors and events are modelling subjects to investigate. Their
names do not establish final contexts. A required desk-and-parking commitment
must either fit a suitable consistency model or use an explicit local
transaction involving its owners. A context is neither a Kubernetes Deployment
nor automatically a database, and DDD does not require identical four-layer
trees for every context. Inspect [the DDD layout](4-ddd.md) for models,
aggregate scope and clearly stated coordination costs.

## Modular monolith

**Source principle.** A shared deployment can contain several internal
components. Microsoft's monolith guidance explicitly separates external
deployment shape from internal libraries, components and layers. Spring
Modulith provides a concrete first-party example of logical application modules
with provided interfaces, private implementation and required interfaces to
other modules. [Microsoft: monolithic applications and containers](https://learn.microsoft.com/en-us/dotnet/architecture/modern-web-apps-azure/common-web-application-architectures#monolithic-applications-and-containers),
[Spring Modulith: fundamentals](https://docs.spring.io/spring-modulith/reference/fundamentals.html).

Spring's verifier checks acyclic module dependencies, access through module
APIs, and optional dependency allowlists. These are evidence of enforceable
modularity; its Java package rules are not Go language features. Translating
those principles into Go requires Go visibility and architecture checks.
[Spring Modulith: structural verification](https://docs.spring.io/spring-modulith/reference/verification.html).

**SpaceWeave candidate.** Encapsulate business capabilities behind narrow APIs,
give them explicit persistence ownership, and use ordinary Go calls inside a
runtime. Use internal ports/adapters where complexity warrants them. A Booking
module may contain many aggregates; it should not become one enterprise-sized
aggregate or lock. SpaceWeave has selected a separate Events owner for
registration, tickets and admission, coordinating venue changes with Booking
through owning interfaces in a narrow local transaction. Inspect
[the modular-monolith layout](5-modular-monolith.md) and
[boundary decision](../../adr/0023-separate-events-from-venue-booking.md) for the
tradeoff; the rest of the module map remains proposed.

## Go boundaries: release units, packages and visibility

A Go **module** is a collection of packages released and versioned together,
identified by `go.mod`. A **package** contains source files compiled together
from one directory. An architectural business module may span several packages
inside one Go module. Splitting `go.mod` files is therefore a dependency/release
decision, not a prerequisite for domain encapsulation. [Go: modules and packages](https://go.dev/ref/mod#modules-packages-and-versions).

Go prohibits direct and indirect package import cycles. Its `internal` rule
restricts imports to the directory/import-path tree above `internal`.
Consequently, a repository-level `internal` protects against outside importers
but usually permits imports among sibling business areas. Nested `internal`
packages can strengthen a module's implementation boundary, with a deliberate
public construction/API surface. [Go: imports](https://go.dev/ref/spec#Import_declarations),
[Go: internal directories](https://pkg.go.dev/cmd/go#hdr-Internal_Directories).

**SpaceWeave design recommendation.** For every candidate, name the public
contract, permitted callers, owned tables/migrations, and authorised writers.
Check imports and construction paths; review SQL against ownership. A directory
label does not stop a caller receiving an overpowered repository interface, or
a database connection writing a foreign table. Compiler-enforced package
visibility, architecture checks and database access rules provide different
protections and should be described separately.

## Transactions determine which boundaries are practical

PostgreSQL transactions make several database changes commit or roll back
together. Atomicity alone does not establish every concurrent invariant:
default Read Committed queries use statement-level snapshots, and successive
queries may see different committed state. Row locks coordinate conflicting
access to existing rows; serializable transactions can require whole-transaction
retries. [PostgreSQL: transactions](https://www.postgresql.org/docs/current/tutorial-transactions.html),
[PostgreSQL: isolation](https://www.postgresql.org/docs/current/transaction-iso.html),
[PostgreSQL: explicit locking](https://www.postgresql.org/docs/current/explicit-locking.html).

**SpaceWeave implications, regardless of layout:**

- Form groups then pack them outside long database transactions. Revalidate the
  complete proposal's relevant inputs before publishing it, preserving group and
  required-parking consistency. Staging does not confer claim authority; see the
  [decision](../../adr/0037-form-collaborator-groups-before-packing-desk-allocations.md)
  and [publication research](../two-phase-desk-allocation.md).
- Allocation, direct requests and waitlist promotion must use one authoritative
  claim protocol. A planner's earlier snapshot cannot reserve capacity by itself.
- Protect against concurrent claims using an explicit combination of database
  constraints, locking/conditional writes or serializable retry handling. The
  choice must cover absence as well as existing rows; locking one existing
  booking does not automatically protect an otherwise unrepresented free slot.
- Identify who opens the transaction, which owners participate, the required
  invariants, and retry/idempotency behaviour. Passing an unrestricted database
  transaction across every module weakens the ownership being claimed.
- If venue capacity and ticket issuance must remain consistent immediately,
  asynchronous messages alone cannot provide that guarantee. Keep the invariant
  inside a suitable owner or document the local transaction across owners.

The outbox records business changes and intended messages in the same local
transaction, followed by delivery after commit. Consumers must tolerate
duplicates. This resolves a local database/message dual-write problem; it does
not atomically commit an Exchange operation with PostgreSQL. [AWS: transactional outbox](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html).
For SpaceWeave, provider acceptance, reconciliation and notification delivery
therefore remain explicit asynchronous state transitions.

## Kubernetes roles are operational boundaries

Kubernetes Deployments manage replicated application Pods; Jobs run tasks to
completion and CronJobs schedule Jobs. Container command/argument configuration
can select different behaviours from the same image. [Kubernetes: Deployments](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/),
[Kubernetes: Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/),
[Kubernetes: container commands](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/).

**SpaceWeave design choice.** HTTP serving, allocation, Exchange reconciliation,
tracing and maintenance can have separate runtime roles, concurrency limits,
identities and replica counts while sharing a release and domain contracts.
A continuously polling worker differs from a finite Kubernetes Job. A durable
PostgreSQL job is an application work record, not a Kubernetes Job object.
Neither one process nor several role processes determines how many business
modules the code contains; a role can execute work owned by several modules.

## Criteria for choosing the detailed layout

The preferred modular-monolith direction combines local transactions with
business ownership and leaves room for ports/adapters and DDD modelling inside
modules. This is a SpaceWeave recommendation based on the requirements above,
rather than a claim that another style cannot implement them.

Review the five candidates against the same evidence:

1. Can an engineer locate each invariant, transaction owner and authorised
   writer without following a general-purpose shared store throughout the app?
2. Can concurrent completed-plan publication, direct booking and waitlist work preserve
   capacity and retry safely when inventory changes during planning?
3. Do framework/provider types stop at meaningful boundaries, and can delayed
   integration observations be reconciled without bypassing booking authority?
4. Are public APIs and private implementation enforced in practice, including
   dependency construction, SQL and migrations, as well as import direction?
5. Do the package and role boundaries justify their maintenance cost through
   clear ownership, useful tests, understandable changes and operational control?

Staff and desk counts do not select a package map or prove performance. Measure
representative concurrency, allocation completion, database contention, provider
backlogs and recovery behaviour before accepting operational claims. The exact
module split should follow those findings and the business consistency model.
