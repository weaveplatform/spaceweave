# Architecture options for SpaceWeave

SpaceWeave is a workplace booking application for one organisation with
approximately 100,000 staff and 60,000 desks across multiple offices. It combines
advance team-proximity allocation, direct space booking, parking, meeting rooms,
internal events, hosted visits and attendance reporting. Correctness depends on
atomic local capacity decisions and reliable asynchronous enterprise integration.

This comparison is for technical architects and engineers evaluating how to
organise and evolve the application. It explains five concrete repository
layouts, their dependency rules, state ownership, transaction boundaries,
operating model and change costs. The documents assume no prior design discussion.
Candidate paths illustrate possible implementations; the detailed module/package
map remains undecided. The preferred direction is a modular monolith, supported
by the [architecture decision](../../adr/0009-use-a-domain-first-modular-monolith.md)
and subject to validation of its actual boundaries.

Start with [architecture foundations](architecture-foundations.md), which explains
layering, hexagonal architecture, Clean Architecture, domain-driven design and
modular monoliths using primary sources. Then compare the same workflow in each
candidate. [Decision analysis](../architecture-options-spaceweave.md) summarises
the consistency and operational tradeoffs.

| Candidate | Primary organising rule |
| --- | --- |
| [Layered](1-layered.md) | Technical responsibilities: transport, application, model and persistence. |
| [Hexagonal](2-hexagonal.md) | Application-owned ports with driving/driven technology adapters. |
| [Clean architecture](3-clean.md) | Business policies and use cases with inward source dependencies. |
| [Domain-driven design](4-ddd.md) | Bounded contexts, domain models, aggregates and explicit coordination. |
| [Modular monolith](5-modular-monolith.md) | Business modules with published interfaces and owned data in one release. |

These approaches overlap. A domain module can contain use cases and ports/adapters;
a layered application can also use domain modelling. The comparison evaluates
concrete placement and enforcement choices rather than treating architecture
labels as mutually exclusive frameworks or guaranteed performance properties.

All candidates expose the same supported [public business APIs](../api-surfaces/README.md)
to UI, MCP, SDK, script, testing and Terraform consumers. Workers use private
application contracts. API compatibility, network exposure, identity and
resource-level authority remain separate from directory layout.

## Requirements held constant

Use the [product scope](../../../README.md#product-scope),
[glossary](../../../GLOSSARY.md), [architecture decisions](../../README.md) and
[policy defaults](../../policy-defaults.md) as the authoritative requirements.
The [Events–Booking boundary](../../adr/0023-separate-events-from-venue-booking.md)
is agreed: Events owns registration/tickets/admission and Booking owns venue
claims, coordinated through owning interfaces in one local transaction where
required. The rest of the blueprint's detailed module split remains proposed.

- One organisation, many offices, 100,000 staff and 60,000 desks. Offices contain
  buildings/floors/plans. Neighbourhoods belong to one building, can span its
  floors, and bundle desks plus optional meeting rooms; unallocated desks may
  move between dated bundles without disrupting confirmed claims. Assigned
  groups receive priority access to bundled rooms, with remaining availability
  governed by configurable policy.
- Desks, meeting rooms, specific-bay/pooled parking, event venues, hosted visits
  and free named event tickets. Separate staff/mobile-web and admin portals.
- Staff choose attendance dates and explicit travel exceptions. No attendance
  minimum or anchor-day configuration: attendance is reporting, and divisional
  anchor days remain organisational context.
- Collect advance desk intents in configurable office cycles. Plan per-date
  groups distinct from externally maintained teams; allocate within hard
  constraints, then prioritise explicit collaboration, team proximity and
  previously unfulfilled demand among plans accommodating the same demand.
- Plan outside long transactions; commit a group placement atomically,
  including both-required parking. Replan affected uncommitted groups. Protect
  confirmed bookings; ordinary direct desk booking opens after the main run.
- Enforce one overlapping desk per person. Required desk/parking claims confirm
  both or neither; office transfers acquire replacements and release old claims
  atomically. Eligible waitlisted demand precedes fresh direct requests.
- Allocation means allocated; the owner's personal check-in means taken. A PA
  cannot check in the owner's desk. Anonymous occupancy never establishes the
  owner's attendance or withholds a no-show desk from the pool. Optional sensors
  support post-check-in vacancy release; missing signals are not vacancy.
- Bidirectional Exchange Online room create/edit/cancel. App requests hold local
  capacity pending room acceptance; compatible changes reconcile automatically.
  Preserve actor versus owner/organiser, source-labelled delegation, revocation
  semantics and configurable native-authority freshness.
- Entra SSO and actual authenticated SCIM user/mapped-group provisioning;
  Workday workforce lifecycle, structure and usual office. Correlate verified
  source identities without email-only matching; apply effective dates, revoke
  access on observed effective disables/leavers and preserve known state during
  source outages. Review rooms/events for leaver handover.
- Capabilities plus configurable office scopes; explicit global capabilities
  for organisation-wide settings. Visitor access uses verified email with
  visit/event restrictions. Separately configured visitor, occupancy and
  attendance retention applies to identifiable copies as well as owning data.
- Email/in-app notifications with preferences. Microsoft Teams is narrowly a
  private message that a booking was raised, with the individual's opt-in for
  that message. No shared channel, default opt-in or automatic extension to other
  Teams message types.
- Go, Chi, Cobra without Viper, strict YAML deployment configuration; React/
  TypeScript portal entrypoints; Go-owned provider tokens and secure sessions
  with backend authorisation/CSRF controls. OpenAPI and generated transport
  types/clients stay separate from domain contracts.
- UI, MCP servers, SDKs, scripts, public contract tests and Terraform consume
  supported public REST; platform workers use private interfaces. User-facing
  Internet access is governed by Conditional Access and application permissions.
  Event visitors include invited guests/speakers attending internal events.
- Configurable SpaceWeave RBAC; MCP uses user context, while machine-identity
  scripts use a separate channel. No universal login journey follows from the
  consumer label. Authorised UI edits can create Terraform drift; initial public
  version-retirement notice is 180 days with published support dates honoured.
  Existing global administration manages RBAC, with SCIM-backed role mappings,
  grant-only scoped composition and self-permission discovery.
- Explicit REST idempotency, call-cost and retry behaviour. The shared
  [request contract](../api-surfaces/request-behaviour.md) applies across the five
  layouts. Required keys/atomic acceptance receipts, versioned
  configuration edits, two admission classes, actor budgets across clients and
  public operation status are application requirements. Definitive refusal replay,
  partial allocation reporting and safe cancellation preserve completed effects.
  Retention, wire conventions and numeric limits remain open; each layout
  preserves bounded inputs and worker execution.
- PostgreSQL durable state, logical ownership of tables/migrations and one
  authoritative local booking/availability write path. Durable PostgreSQL jobs
  and transactional outbox; external effects after commit, retry-safe handlers,
  leases, reconciliation and visible failures. Architecture alternatives must
  explain how ownership is enforced in their own organisation.
- Azure Blob and Google Cloud Storage behind one interface, active provider by
  deployment configuration; immutable plan/render revisions, PostgreSQL metadata
  and verified recovery copies before durable/publication acknowledgement.
- One image/release, independent Kubernetes HTTP/allocation/Exchange/tracing/
  maintenance roles, scoped/federated workload identities, organisational secret
  management and OpenTelemetry/JSON telemetry through a Collector.
- Core availability target 99.95% monthly and one-second p95 initial responses
  at an agreed representative workload; asynchronous completion is separate.
  Single active write region, multi-zone HA, warm regional recovery, five-minute
  RPO/fifteen-minute RTO targets. Healthy primary keeps serving during excess
  recovery lag; alert and require explicit operator choice for failover beyond
  the loss budget. Fence prior writers and test whole-system recovery.
- Confirmed app outages preventing check-in/offer acceptance pause affected
  no-show/offer timeouts and allow configurable recovery grace. Existing
  reassignments remain protected.

## Common comparison scenarios

Every document traces the same work so a reader can compare where it lives and
which interfaces/transactions carry it:

1. Commit an allocation group's desk placements and required parking from a
   snapshot while direct requests and cancellations change inventory.
2. Race an assignee's check-in against no-show release and waitlist promotion,
   including anonymous occupancy and a confirmed application outage.
3. A PA creates a room request; room acceptance and an external Outlook edit
   arrive late/out of order; source delegation is subsequently revoked.
4. Move an event to a smaller venue while another attendee requests a ticket;
   issue a visitor ticket with a visit link and prevent duplicate admission.
5. Receive SCIM deactivation and an effective Workday office/team change during
   a partial source outage; preserve correct source ownership and booking history.
6. Add a private booking-raised Teams adapter with message-specific opt-in,
   while retaining email/in-app delivery and the authoritative booking outcome.

## Comparison at a glance

These entries compare the concrete candidates in this folder, including their
SpaceWeave-specific safeguards and adaptations. They are design assessments,
not inherent guarantees or restrictions of the architectural labels.

| Decision | Layered | Hexagonal | Clean | DDD | Modular monolith |
| --- | --- | --- | --- | --- | --- |
| Main navigation | `transport`, `application`, `model`, `persistence`. | One `core` and driving/driven `adapter` roots. | `entity`, `usecase`, `adapter`, `infrastructure`. | Context first, then uniform `api/domain/application/adapters`. | Module first; small public interface, private implementation, inner ports/adapters where useful. |
| Booking authority | Booking application service alone receives dated-space claim mutation facets. | Core Booking service alone receives physical-capacity writer ports. | Booking mutation interactor implements injected consumer-owned physical-claim ports. | Booking context owns physical-capacity repositories and coordinated application handlers. | Booking module owns dated-space claims; Events owns ticket capacity. |
| Allocation placement | Sibling application planner calls Booking. | Core allocation service calls Booking through a driving contract. | Allocation use case receives a Booking command implementation through its own port. | Pure planner and allocation handlers inside Booking context. | Separate planning module computes proposals; Booking validates/commits them. |
| Event venue and ticket consistency | Events coordinates shared transaction; Booking retains its writer. | Core Events coordinates transaction-scoped owner facets. | Events' manageevent family coordinates its own ticket state with Booking's venue participant through consumer-owned ports and shared unit of work. | Events and Booking use an explicit shared local transaction; ordinary aggregate transactions remain the default. | Separate Events and Booking owners join the narrow venue transaction; neither receives foreign repositories. |
| Visitor ticket linkage | Proposed named Events/Visitors shared transaction. | Proposed named owner-participating transaction. | Proposed narrow Events/Visitors capabilities in one transaction. | Proposed staged/reused visit, then idempotently published linked ticket. | Proposed named Events/Visitors transaction exception. |
| Identity and access | Separate Directory/Access application subjects over shared models/persistence. | Separate core Directory/Access services over central contracts. | Staff entities plus provisioning/login/authorisation operation families. | Directory and Access contexts with published eligibility views. | One Identity module combines workforce projection, access and delegation. |
| Exchange state | Exchange application/persistence subject coordinates with Booking. | Core Exchange service and provider adapter call Booking. | Reconciliation use cases consume Booking commands through their own ports. | Exchange context translates provider observations for Booking. | Booking owns room/provider state behind its Exchange adapter. |
| SQL and migration custody | Logical owners in a common PostgreSQL package plus ownership manifest. | Logical owners in one PostgreSQL adapter plus restricted facets/manifest. | Owner-labelled repository groups and migration manifests, assembled through narrow handles. | Context-owned repositories and migration sets. | Module-private repositories and migration sets. |
| What enforces locality | Import/facet/query review across horizontal layers. | Core/adapter import checks plus writer/facet construction rules. | Ring checks, no use-case peer implementation imports, writer and SQL ownership checks. | Context API checks, domain purity and explicit transaction exception review. | Public-interface imports, private implementation packages and SQL ownership review. |
| Main structural cost | One feature touches several technical roots; shared model/store can grow. | Central model/ports/store/composition can become shared hotspots. | More ports and wiring; cross-owner venue coordination through consumer-owned interfaces. | Uniform layers and context/event contracts; shared transactions couple aggregate/context boundaries. | Module custody plus explicit Events/Booking venue coordination and proposed Events/Visitors issuance coupling. |

Every candidate still needs PostgreSQL capacity protection, retry-safe durable
work, source-aware authorisation, provider reconciliation and whole-system
recovery exercises. Those concerns are not solved by changing directory names.

## How to review the set

Start with each Summary/dependency rule set and Full package tree, then compare
the same walkthrough across all five documents. The walkthroughs name actual
proposed paths and transaction participants so ownership can be challenged.

The most consequential differences to review are:

- **Where changes concentrate.** A new booking rule crosses horizontal roots in
  options 1–3, a context's four layers in option 4, and the owning module's
  implementation in option 5. Integration/composition changes remain explicit.
- **How transaction coupling is expressed.** Options 1–2 use guarded owner
  facets, option 3 consumer-owned participant ports, option 4 context interfaces,
  and option 5 module APIs with registered scope participants. All retain the
  agreed Events/Booking venue boundary. Visitor-creation atomicity remains a
  separate product choice: the staged-visit and shared-transaction proposals
  have different failure/cleanup costs.
- **Whether an internal seam earns its cost.** The shared core, repeated use-case
  ports, uniform context layers and selectively deep modules expose different
  amounts of wiring. A printed folder is not automatically useful abstraction.
- **How enforceable ownership is.** Distinguish compiler/import protection,
  constructor restrictions, query manifests and reviewer discipline. None is a
  substitute for testing the claim protocol with real concurrent transactions.
- **How existing rendering work survives.** Each Package mapping distinguishes
  retained algorithms/fixtures from new booking/integration work. The comparison
  does not assume a wholesale rewrite or a fleet of separately deployed services.

The Numbers sections describe their own printed inventories with different
granularity. Use them to understand structure, not to rank runtime speed, cost,
test coverage or delivery dates. No weighted score or final architecture vote is
assigned here; selection depends on the documented engineering evidence.

## Evidence, proposals and validation

[Architecture foundations](architecture-foundations.md) cites the primary
framework descriptions and official Go, PostgreSQL and Kubernetes documentation.
Each candidate separates those source principles from SpaceWeave requirements
and proposed implementation choices. A no-peer-import policy, one central core,
uniform context layers or a particular module count is a local design choice.

The detailed trees and structural counts describe the candidate being reviewed.
They are navigation and change-impact tools, not estimates of delivery effort,
throughput or coverage. Existing-code references support migration analysis only;
they do not establish that a booking platform has already been implemented.

Select boundaries using invariant ownership, concurrent transaction tests,
provider recovery trials, representative load and operational failure exercises.
Document remaining uncertainties as engineering decisions with alternatives,
tradeoffs and a validation plan. The same architecture label can describe designs
with very different data ownership and failure behaviour.
