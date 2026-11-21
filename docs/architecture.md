# Architecture and policy design

SpaceWeave is a workplace booking platform for one enterprise across many offices.
This design record distinguishes product/runtime requirements, existing floor-plan
capabilities and proposed implementation boundaries. The [scope](../README.md#product-scope),
[glossary](../GLOSSARY.md) and [decision records](README.md) define the requirements.

The [architecture blueprint](architecture-blueprint.md) records agreed logical owners;
the [five-option comparison](research/architecture-options/README.md) evaluates
alternative layouts against the same workflows. The repository-architecture choice remains pending; concrete package placement
and publication-guard mechanics remain under review. [Architecture foundations](research/architecture-options/architecture-foundations.md)
and the [API research](research/api-surfaces/README.md) provide primary-source
technical guidance and design tradeoffs.

## Existing application

- One Go executable exposes the CLI, Chi HTTP routes, tracing workers and static
  floor-plan microsite. See [CLI](../internal/cli/cli.go) and
  [HTTP routing](../internal/server/server.go).
- Plans, uploads and the catalogue use local files. Catalogue locking and job
  claiming are implemented in [catalogue](../internal/catalog/catalog.go), with
  polling workers in [worker](../internal/worker/worker.go).
- The floor-plan frontend uses native JavaScript modules, SVG and D3, with no
  frontend build step. See its [README](../web/floorplan-iso/README.md).
- Staff authentication, office-scoped authorisation, booking persistence, distributed
  job execution and deployment infrastructure are new design work.
- Repository conventions retain Chi, Cobra, strict YAML configuration, the
  existing package layout and correctness gates. See [CLAUDE.md](../CLAUDE.md).

## Design tree

- Delivery and capacity
  - Established: one organisation across many offices/sites; target 100,000 staff
    and 60,000 desks. The earlier multi-organisation delivery proposal is replaced.
  - Established: containerised deployment on organisation-controlled Kubernetes,
    with application-health observability required.
  - Established: 99.95% monthly availability target for core booking and check-in;
    Exchange outages use pending requests and delayed synchronisation.
  - Established: one-second p95 initial responses for core availability, booking
    and check-in requests at the agreed capacity-test workload, measured
    separately from asynchronous completion and Exchange room acceptance.
  - Established: observability covers request health and business workflow
    health, including allocation lag, Exchange freshness, pending-room age and
    stalled desk-release/waitlist processing, with dashboards and alerts.
  - Established: region-disaster RPO target of five minutes and RTO target of
    fifteen minutes. Routine pod/node failures preserve acknowledged bookings.
    Provider features and whole-system recovery need validation against these
    targets; blob replication alone does not establish the five-minute bound.
  - Established: protect committed business records and assets acknowledged as
    durable/published, verifying asset recovery copies before durability
    acknowledgement. Unfinished uploads and processing remain retryable.
  - Established: one active write region with multi-zone HA and a preprovisioned
    warm recovery region. Fence the old writer before promotion and exercise
    database, asset, identity, routing and worker recovery together.
  - Established: if the primary is healthy but regional recovery replication
    exceeds the five-minute loss budget, keep serving and raise a critical alert.
    Failover beyond that budget requires an explicit operator decision; this
    accepts temporary exposure beyond the regional RPO target.
  - Established: runtime roles have separately scoped identities, workload
    federation where supported and organisational secret management for remaining
    credentials. OpenTelemetry traces/metrics and structured JSON logs flow
    through a Collector to the organisation's chosen observability backend.
  - Then: deployment/provider sizing and measured failure/recovery behaviour
    against representative peak workloads.
- Application structure
  - Established: retain Golang.
  - Research compares the five reference layouts: layered, hexagonal, clean,
    DDD and modular monolith, applied to SpaceWeave.
  - Pending: choose among layered, hexagonal, Clean, DDD and modular-monolith
    repository architectures. A domain-first modular monolith is the preferred
    candidate; the
    [research comparison](research/architecture-options-spaceweave.md) and
    [proposed architecture decision](adr/0009-use-a-domain-first-modular-monolith.md)
    record the reasoning.
  - Established: Booking owns local space availability and booking transitions;
    other modules use its interface.
  - Established: Workplace owns location/space definitions and baseline
    neighbourhoods; Booking owns dated capacity and temporary neighbourhood
    adjustments. Booking's private planner uses a versioned snapshot outside long transactions;
    Booking revalidates completed-plan publication and protects previously
    published allocations; staged groups remain provisional.
  - Established: Identity combines source/workforce facts, sessions, permissions
    and delegation through distinct interfaces. Booking owns intents, cycles,
    groups and planning; Exchange separately owns provider state/progress and
    applies transitions through Booking. These owners apply across all five layouts.
  - Established: Booking owns validated operational occupancy evidence used for
    release; Attendance projects committed facts with separate reporting retention.
    Plans approves immutable assets; Workplace publishes geometry-to-stable-space
    mappings after Booking validates dated inventory and commitment impact.
  - Established: shared operational facilities own confirmed outage declarations
    with affected offices/actions/times. Booking and Events own deadline effects;
    Policy owns configurable rules. Expiry checks authoritative declared scope
    with guarded ordering rather than waiting for an asynchronous projection.
  - Established: retirement is immediate or scheduled at an office-local time.
    Publication excludes new bookings for use on/after that point while protected
    confirmations remain manageable. Reactivation validates definitions, calendars
    and inventory, retaining identity/history. Dependency-blocked deletion never
    silently retires, cancels bookings or cascades through child resources.
    Unconfirmed holds/offers revalidate at confirmation and cannot confirm
    excluded use; late Exchange effects require reconciliation.
  - Established: one application image/release supports independently scaled
    HTTP, allocation, Exchange sync, tracing and maintenance roles.
  - Established: separate React/TypeScript staff/mobile-web and admin entrypoints
    share UI and the existing renderer. Go manages sessions with provider tokens
    server-side, secure HttpOnly cookies, backend authorisation and CSRF controls.
  - Established: contract-first REST APIs described with OpenAPI and generated
    Go/TypeScript transport types and clients. Domain interfaces remain independent
    of transport contracts.
  - Established: UI, MCP servers, SDKs, scripts, tests and Terraform consume the
    supported public REST contract family; platform workers use private APIs/
    interfaces. Browser sessions remain Go-managed. Client type is not authority.
  - Established: Internet-accessible user-facing APIs governed by Conditional
    Access rules/policies plus application permissions. Human, machine,
    visitor/provider and private-worker identity profiles remain distinct.
  - Established: delegated human identities and independently granted application
    identities, with actor/client/owner provenance. App-only credentials cannot
    perform personal desk check-in.
  - Established: configurable SpaceWeave RBAC. MCP operates in the initiating
    user's context; scripts using machine identity use a separate channel.
    Consumer, authentication journey and authority subject are distinct; no
    universal native/browser login journey is selected for those client kinds.
  - Established: Terraform initially manages long-lived workplace/permitted
    policy configuration, with useful read-only queries. Public REST compatibility
    is independent of UI releases; breaking changes need a new major and explicit
    migration/deprecation policy. First-party MCP server/provider implementations
    follow the core platform and Go/TypeScript SDKs.
  - Established: one overlapping desk allocation per person, transactional office
    transfers including required parking, and personal desk check-in by the owner.
  - Established: required retry keys for booking creation, ticket issuance and
    background-work submissions, with receipts committed atomically with acceptance.
    Shared configuration edits require the previously read version.
  - Established: standard/expensive request classes, configurable rate/burst limits
    and one token per admitted attempt, including replays. Budgets follow the
    person/application across sessions and clients.
  - Established: durable public operation references with authorised progress,
    failure and result reads for allocation, floor-plan processing and exports.
  - Established: recover ambiguous outcomes across reloads/client restarts and
    a return the next working day before inviting another submission. Resource
    visibility is independent of retry-receipt retention.
  - Established: equivalent retries ignore JSON property order and whitespace,
    preserve omitted/null distinctions and include owner/target/preconditions.
  - Established: definitive refusal replay, completed-plan unmet-demand results
    distinct from interrupted provisional processing, and cancellation ordered
    against allocation publication. Cancellation winning prevents publication;
    publication winning preserves the completed plan. Keep floor-plan and export
    cancellation separate, including cleanup and completed-result access/expiry.
  - Established: Events owns registration, tickets and admission; Booking owns
    event-space claims. Capacity-coupled changes coordinate through owning
    interfaces in one local transaction where required.
  - Established: booking/ticket recovery covers the end of the next working day
    in the relevant office calendar, including weekends/closures; publish the
    deadline. After expiry, reconcile authorised resources before offering a new
    submission; explain inconclusive recovery and require an explicit new attempt.
  - Established: cancellation stays requested until execution stops, serialized
    against commits/publication. Earlier commits survive; cancellation winning
    the ordering prevents the next commit/publication.
  - Established: a registration-created visit and event ticket commit together;
    failed issuance leaves an existing valid reused visit intact. Confirmed venue
    loss retains tickets pending recovery, pauses admission/issuance and requires
    explicit organiser replacement/cancellation with confirmed changes communicated.
  - Established: an outage spanning the recovery deadline extends affected
    requests through the end of the next office working day after restoration;
    preserve evidence and publish the extension without extending booked periods.
  - Established: event office/time changes require attendee reconfirmation and
    linked visit validation; later visit ineligibility blocks admission while
    retaining the ticket/seat pending authorised repair or cancellation.
  - Established: attendees can cancel unused tickets. Optional event waitlists
    receive released seats before fresh registration through time-limited offers
    with current eligibility checked on acceptance. Event cancellation cancels
    visits whose only remaining valid purpose was that event, preserving others.
  - Established: event offers follow eligible join order and hold seats exclusively;
    decline/expiry ends the request and rejoining creates a new request. Acceptance
    ends at registration closing time unless the organiser explicitly extends it.
  - Established: published reconfirmation deadlines release unconfirmed tickets
    under the enabled waitlist policy. Unused-ticket cancellation also cancels
    an associated future visit with no other valid purpose, preserving reception history.
  - Established: confirmed outages pause affected event reconfirmation deadlines;
    preserve seats and publish usable recovery grace. Venue recovery preserves
    offers and pauses expiry clocks, with acceptance bounded by registration close.
  - Established: temporary queue ineligibility preserves position without holding
    capacity. Office/time changes administratively withdraw old offers without
    ending their requests, preserving positions for offers under updated details.
  - Then: receipt duration/retained expiry evidence, exact wire details and
    load-validated numeric limits.
  - Then: shared review of the detailed module map and interface contracts in
    the blueprint, followed by implementation validation.
- Persistence and background work
  - Established: PostgreSQL holds durable application state, with module-owned
    tables and migrations.
  - Established: blob storage supports Azure Blob Storage and Google Cloud
    Storage for drawings and generated assets; PostgreSQL holds their metadata
    and identifiers.
  - Established: one blob interface, two provider adapters, with the active
    provider selected by deployment configuration. Drawing/render revisions are
    immutable, with current-version pointers and historical reference retention.
  - Established: booking changes and background-work records commit together;
    external calls occur after commit and retry-safe workers resume durable work.
  - Established: PostgreSQL-backed durable jobs/outbox processing, with leased
    claims, retry backoff and operator-visible failures. Production PostgreSQL is
    a managed platform service operated separately from application workloads.
  - Established: first form collaborator groups, including singletons, then pack
    groups into existing desks. Revalidate and publish the completed plan together,
    including required parking; no progressive group confirmations. Sitting
    together precedes walking distance, with allocation quality taking precedence
    over computation cost. The [decision](adr/0037-form-collaborator-groups-before-packing-desk-allocations.md)
    and [research](research/two-phase-desk-allocation.md) record the contract.
  - Then: concrete database constraints, job lease/fencing contracts and
    validation of asset publication, retention and recovery workflows.
- Identity and administration
  - Established: Entra ID staff SSO, verified-email visitor access, capabilities
    combined with one-or-more office scopes; on-behalf-of bookings are required.
  - Established: the staff member being booked for owns the booking and is its
    Exchange organiser; record the acting booker separately.
  - Established: reflect Exchange delegation for rooms, infer that authority for
    other space kinds and allow additional app-managed delegates.
  - Established: app-managed grants authorise app-mediated room bookings while
    leaving native Outlook permissions unchanged; resulting booking changes
    still synchronise with Exchange.
  - Established: initially infer explicit Exchange calendar-delegate grants;
    map other authority deliberately and retain source, owner and actions.
  - Established: revoking an Exchange-derived grant removes that grant;
    separately approved app-managed grants retain their own lifecycle.
  - Established: a configurable fifteen-minute maximum verification age for new
    actions relying on Exchange-derived delegation. Refresh stale authority;
    app-managed authority is checked against current local state.
  - Established: directory sources include Entra and Workday, with explicit staff
    and Team mappings and stable internal staff identifiers linked to source
    identities. Entra owns sign-in/account security; Workday owns workforce
    lifecycle, reporting structure and usual office. SpaceWeave owns booking
    preferences and dated travel choices.
  - Established: correlate using a unique organisation-approved identifier,
    retain both source identities and hold ambiguous matches for review. Email
    alone is not a permanent linking key.
  - Established: confirmed, effective leaver/disable events revoke staff access
    when observed, including existing sessions. Source outages preserve the last
    verified state of known staff; missing records do not imply deactivation.
    Deactivation and deletion are distinct lifecycle operations, as in SCIM.
  - Established: expose authenticated SCIM 2.0 user and explicitly mapped group
    provisioning for Entra, including creation, updates, deactivation,
    reactivation and deletion. Workday integration remains separate and source
    field ownership is preserved.
  - Established: effective-dated office/team changes update unconfirmed derived
    planning defaults while preserving explicit travel and confirmed bookings.
    Notify owners when existing arrangements need attention.
  - Established: before either booking starts, explicit cancellation of either
    component of a both-required desk/parking pair cancels both atomically;
    optional pairs permit independent cancellation. Once either starts, cancel
    the selected booking unless both are explicitly chosen. Automatic desk no-show or
    vacancy release retains parking until its own end or explicit release.
    All five layouts extend their existing Booking transition/store paths.
  - Established: automatically release a confirmed effective leaver's future
    personal desk/parking and cancel all organiser-owned room meetings/series,
    reconciling Exchange and notifying attendees. A departing PA's other-owner
    bookings remain. Events retains its own handover; disable or missing source
    data alone does not establish departure.
  - Established: native Exchange room policy, check-in eligibility and autorelease
    are authoritative across Outlook and SpaceWeave. The app adapts supported
    user-context check-in and observes native outcomes without a second timer.
    Recurring-only per-person admission, exact invitation effects and consecutive-
    miss cancellation remain unverified native capability gaps, not custom rules.
  - Established: organisation-wide policies, identity mappings and integration
    administration require explicit global capabilities beyond office scope.
  - Then: tenant-specific integration mappings/permissions and protocol validation.
- Policy configuration
  - Established: organisation defaults with permitted office overrides for
    operational policies; identity and retention stay organisation-wide.
  - Established: explicit office allocation cycles define attendance dates,
    intent cutoff and processing time; late changes are incremental.
  - Established: among placements accommodating the same number of people, order
    objectives as explicit collaboration, general team proximity, then previously
    unfulfilled demand. Direct desk booking opens after the date's main plan is published;
    ordinary requests before it enter intent collection.
  - Established: policies are versioned database records editable through
    administration, separate from deployment configuration. Apply changes to new
    cycles/bookings by default; preserve existing confirmations' versions unless
    explicitly migrated.
  - Established: [the initial defaults](policy-defaults.md) are accepted starting
    values, all configurable, including weekday, local time and duration values.
  - Established: a confirmed application outage preventing desk check-in or offer
    acceptance pauses affected no-show releases and waitlist-offer expiries,
    followed by a configurable recovery grace period.
  - Then: typed policy validation, effective dates, publication and migrations.
- Notifications
  - Established: email and in-app notifications for booking outcomes, waitlist
    offers, visitor invitations, event tickets and handover tasks. Address owners
    and relevant acting delegates with configurable delivery preferences.
  - Established: optionally send an individual a private Microsoft Teams message
    that a booking was raised, with that individual's opt-in for that message.
    This is not permission for other Teams messages or shared-channel broadcasts.
  - Then: personal Teams delivery integration and tenant installation prerequisites;
    technical installation never substitutes for the individual's message opt-in.
- Exchange synchronisation
  - Established: Exchange Online; bookings, changes and cancellations flow both
    ways; app room requests use an expiring local hold while pending room
    acceptance, then confirm, decline or expire.
  - Established: merge compatible edits automatically, discard stale updates and
    revalidate room/time changes; people handle genuinely incompatible intentions.
  - Established: a dedicated backend Exchange identity, scoped mailbox access,
    notifications and durable delta reconciliation every five minutes while
    healthy; persist progress and measure actual freshness.
  - Then: synchronisation execution, event mapping, retries, reconciliation,
    cancellation semantics and operational recovery.

## Design status and remaining decisions

The platform requirements include one public business REST contract for UI and
automation, private worker interfaces, Internet access under applicable identity
policies, independent human/application authority and configurable scoped RBAC.
MCP uses user context; machine scripts use separately approved application authority.
SCIM-provisioned mappings, grant-only role composition and permission discovery
support application access control. Global administration owns role configuration.

Public API compatibility is independent of portal releases, with an initial
180-day major-version retirement notice and migration guidance. Terraform initially
manages long-lived workplace and permitted configuration; authorised UI/API edits
appear as drift during reconciliation. SDKs precede first-party MCP/provider delivery.

The [request contract](research/api-surfaces/request-behaviour.md) requires atomic
retry receipts, previous-version configuration edits, two actor-owned admission
classes and durable public operation status. Definitive refusals replay their
retained outcome. Allocation results distinguish unfulfilled demand from technical
interruption; cancellation stops future work at safe boundaries while preserving
committed effects. JSON formatting does not change a retry identity; omitted
fields and explicit null remain distinct. Ambiguous outcomes must be recoverable
after reloads/client restarts through the end of the next working day in the
relevant office calendar, including weekends/closures. Publish that deadline;
after expiry, reconcile authorised resources before offering an explicit new
attempt when uncertainty remains. Cancellation stays requested until work stops
and is ordered against commits/publication. Receipt duration and retained expiry
evidence remain engineering decisions; no numeric default is selected. An outage
spanning the deadline extends recovery through the end of the next office working
day after restoration, preserving evidence and publishing the extension.

The [Events–Booking decision](adr/0023-separate-events-from-venue-booking.md)
settles that ownership boundary and its narrow capacity transaction. The design
also preserves [atomic visitor registration](adr/0024-create-event-visitor-visits-with-tickets-atomically.md)
and the agreed organiser-controlled venue recovery, retaining tickets while
admission and issuance are paused. Remaining architectural work includes the
pending repository-architecture choice, concrete incident-ordering protocols,
concrete evidence/activation protocols, operation schemas, provider deployment
permissions, load-validated limits and recovery
validation. The [event workflow contract](research/api-surfaces/event-workflows.md)
also records reconfirmation, retained seats during visitor ineligibility,
attendee cancellation, optional event waitlists and purpose-based visit cleanup.
Numerical proposals become defaults only with an explicit requirement
and validation rationale. Private Teams delivery is limited to an individually
opted-in booking-raised message.

## Verified personal Teams messaging constraints

Microsoft supports proactive personal messages through a Teams app with a
bot/agent. Personal-scope installation is required for messages addressed using
an Entra object ID. Entra login alone does not establish installation or a usable
conversation. Installation can be performed administratively, but it is distinct
from the person's opt-in to the SpaceWeave booking-raised message.
[Proactive messages](https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/conversations/send-proactive-messages),
[proactive app installation](https://learn.microsoft.com/en-us/microsoftteams/platform/graph-api/proactive-bots-and-messages/graph-proactive-bots-and-messages).

Ordinary Graph chat-message creation does not provide general application-only
permission for background notifications; its application permission is for
migration. An activity-feed alert is a separate surface from a personal chat
message. Neither shared-channel posting nor an activity-feed substitution is
part of the agreed requirement.
[Chat-message permissions](https://learn.microsoft.com/en-us/graph/api/chat-post-messages?view=graph-rest-1.0),
[activity-feed notifications](https://learn.microsoft.com/en-us/graph/teams-send-activityfeednotifications).

Teams integration must handle blocked/uninstalled delivery and throttling.
The blueprint proposes asynchronous, supplementary delivery with bounded retry;
the underlying booking outcome remains authoritative in the application.
[Delivery errors](https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/conversations/send-proactive-messages),
[Graph throttling](https://learn.microsoft.com/en-us/graph/throttling).

## Verified observability tooling

OpenTelemetry Go supports stable traces and metrics with OTLP export; its exporter
guidance recommends a Collector for production. A Collector can receive, process
and export telemetry to compatible organisational backends. Backend compatibility
still needs confirmation.
[Go signal status](https://opentelemetry.io/docs/languages/go/#status-and-releases),
[Go exporters](https://opentelemetry.io/docs/languages/go/exporters/),
[Collector](https://opentelemetry.io/docs/collector/).

Go's standard `slog.JSONHandler` emits structured JSON. The selected log path
uses structured stdout/file logs forwarded by the telemetry pipeline, while
traces and metrics use OpenTelemetry instrumentation.
[JSON logging](https://pkg.go.dev/log/slog#JSONHandler),
[logging workflows](https://opentelemetry.io/docs/languages/go/instrumentation/#logs).

## Verified browser and identity constraints

React supports incremental adoption and can coexist with a renderer that owns
the children of a dedicated DOM container. This supports evaluating reuse of the
current D3 renderer rather than assuming it needs replacement.
[Incremental adoption](https://react.dev/learn/add-react-to-an-existing-project),
[DOM ownership](https://react.dev/learn/manipulating-the-dom-with-refs).

Entra supports server web applications as confidential clients. Keeping provider
tokens in Go with application session cookies is the selected application design;
it still requires CSRF, session and authorisation controls. `oid` and `tid` are
stable identity-linking inputs; email and username are mutable contact data.
[Authorisation code flow](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-auth-code-flow),
[identity claims](https://learn.microsoft.com/en-us/entra/identity-platform/id-token-claims-reference).

User/group delta can support directory synchronisation, but user delta does not
support arbitrary staff-population filters. Configure population/team mappings
explicitly and do not treat filtering as a narrower permission grant.
[User delta](https://learn.microsoft.com/en-us/graph/api/user-delta?view=graph-rest-1.0),
[group delta](https://learn.microsoft.com/en-us/graph/api/group-delta?view=graph-rest-1.0).

## Verified Workday and Entra source constraints

Workday exposes employees, contingent workers, organisation, site/location and
management data. That does not define which records are SpaceWeave Staff or
Teams. Field ownership and domain mappings are application decisions.
[Workday service directory](https://community.workday.com/sites/default/files/file-hosting/productionapi/index.html),
[Get Workers](https://community.workday.com/sites/default/files/file-hosting/productionapi/Human_Resources/v47.1/Get_Workers.html).

Workday-to-Entra matching attributes, precedence, transformations and population
scope are configurable. WorkerID-to-employeeId is a typical mapping, not a
guarantee about this organisation. Preserve source identifiers and a verified
correlation; do not assume email is a stable linking key.
[Microsoft mapping guidance](https://learn.microsoft.com/en-us/entra/identity/saas-apps/workday-inbound-cloud-only-tutorial#part-2-configure-workday-and-microsoft-entra-attribute-mappings).

Workday entry/update time and effective time are distinct. Its worker response
is an as-of snapshot, not complete history. Future hires can appear active in
provisioning before their hire date, and worker incrementals do not establish
that related organisation/location records are all current. Effective-date rules
and periodic reconciliation need explicit treatment.
[Workday snapshots](https://community.workday.com/sites/default/files/file-hosting/productionapi/Human_Resources/v47.1/Get_Workers.html),
[Microsoft integration reference](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/workday-integration-reference),
[related-entity update limitations](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/hr-user-update-issues#some-workday-attribute-updates-are-missing).

Source import does not guarantee instantaneous offboarding. Microsoft documents
termination timing/lookahead considerations; tenant-specific mappings and
security visibility remain unverified.
[Termination processing](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/hr-user-update-issues#workday-termination-processing-delay),
[termination lookahead](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/configure-workday-termination-lookahead).

## Verified SCIM lifecycle constraints

SCIM 2.0 is an HTTP provisioning protocol. SpaceWeave has selected Entra-compatible
`/Users` and `/Groups` provisioning, although group support is optional in the
Entra integration profile. Provisioning clients need authentication and their
own access controls, separate from staff SSO.
[SCIM protocol](https://www.rfc-editor.org/rfc/rfc7644.html#section-1),
[SCIM authentication](https://www.rfc-editor.org/rfc/rfc7644.html#section-2),
[Entra provisioning requirements](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups#understand-the-microsoft-entra-scim-implementation).

SCIM `active` represents administrative status; Entra supports disabling and
reactivating an existing user. A SCIM deletion must remove that resource from
the provisioning interface, even if internal history remains under retention
rules. Entra's disable/delete actions depend on scope, mappings and tenant
configuration; a missing feed response is not an instruction to delete staff.
[User status](https://www.rfc-editor.org/rfc/rfc7643.html#section-4.1.1),
[Deletion semantics](https://www.rfc-editor.org/rfc/rfc7644.html#section-3.6),
[Entra deprovisioning](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/how-provisioning-works#deprovisioning).

SCIM's server-issued `id` is stable and non-reassignable. Its client-supplied
`externalId` is scoped to the provisioning domain and is not a universal,
server-enforced unique identity key. Mutable usernames/email therefore do not
replace the agreed verified staff correlation. These sources establish no
Workday-to-SpaceWeave SCIM transport.
[Common identity attributes](https://www.rfc-editor.org/rfc/rfc7643.html#section-3.1).

## Verified recovery limits

PostgreSQL asynchronous replication can lose acknowledged changes at promotion;
synchronous replication trades latency/availability for stronger durability.
Failover requires fencing the previous primary to prevent competing writers.
[Replication](https://www.postgresql.org/docs/current/warm-standby.html#SYNCHRONOUS-REPLICATION),
[failover](https://www.postgresql.org/docs/current/warm-standby-failover.html).

Azure PostgreSQL and Cloud SQL multi-zone HA operate within a region. Their
regional disaster-recovery replicas are asynchronous; promotion can lose changes
according to actual lag. Database features alone do not establish whole-app
RPO/RTO across routing, identity, blobs, workers and recovery coordination.
[Azure HA](https://learn.microsoft.com/en-us/azure/postgresql/high-availability/concepts-high-availability),
[Azure regional DR](https://learn.microsoft.com/en-us/azure/postgresql/backup-restore/concepts-geo-disaster-recovery),
[Cloud SQL HA](https://docs.cloud.google.com/sql/docs/postgres/high-availability),
[Cloud SQL DR](https://docs.cloud.google.com/sql/docs/postgres/intro-to-cloud-sql-disaster-recovery).

Ordinary blob geo-replication does not establish a five-minute RPO. GCS turbo
replication targets fifteen minutes; Azure Geo Priority Replication provides a
qualified fifteen-minute SLA. Critical-asset publication and recovery readiness
need an explicit design beyond enabling default replication.
[GCS durability](https://docs.cloud.google.com/storage/docs/availability-durability),
[Azure priority replication](https://learn.microsoft.com/en-us/azure/storage/common/storage-redundancy-priority-replication).

## Verified persistence constraints

PostgreSQL is the selected database. Transactions can commit
related state and work records together. The transactional outbox pattern still
requires idempotent consumers because delivery can repeat.
[PostgreSQL transactions](https://www.postgresql.org/docs/current/tutorial-transactions.html),
[transactional outbox](https://microservices.io/patterns/data/transactional-outbox.html).

Range exclusion constraints can reject overlapping reservations for one resource.
A parking pool with capacity greater than one needs additional coordinated
capacity logic. Row locks can coordinate competing writers when every claim and
release follows the same protocol; locking existing reservations alone does not
lock an absent future reservation.
[Range constraints](https://www.postgresql.org/docs/current/rangetypes.html#RANGETYPES-CONSTRAINT),
[row locks](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS).

Serializable/repeatable-read failures can require retrying the entire transaction,
including its decision logic. Exclusion conflicts may instead represent genuine
unavailability. `SKIP LOCKED` supports competing queue consumers but intentionally
returns an inconsistent view, so it is not a booking-availability query strategy.
[Serialization retry handling](https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html),
[queue locking caveat](https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE).

Pod-scoped ephemeral volumes disappear with their Pod; persistent volumes have
independent lifetimes. Blob storage is the selected durable asset design.
These storage mechanisms do not establish
throughput at the target staff and desk counts without workload measurement.
[Volume lifetimes](https://kubernetes.io/docs/concepts/storage/volumes/#how-volumes-work),
[persistent volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#introduction).

## Verified Azure and GCP blob constraints

Azure Blob Storage supports ETag-based conditional writes; Google Cloud Storage
uses object generations and, for metadata-sensitive changes, metagenerations.
A portable concurrency token must stay opaque; it is not a universal content
checksum.
[Azure concurrency](https://learn.microsoft.com/en-us/azure/storage/blobs/concurrency-manage),
[GCS metadata](https://docs.cloud.google.com/storage/docs/metadata),
[GCS preconditions](https://docs.cloud.google.com/storage/docs/request-preconditions).

Both providers support private objects and scoped, expiring access. Signed access
requires signing/delegation authority in addition to ordinary object access.
[Azure private access](https://learn.microsoft.com/en-us/azure/storage/blobs/anonymous-read-access-configure),
[Azure user-delegation SAS](https://learn.microsoft.com/en-us/rest/api/storageservices/create-user-delegation-sas),
[GCS signed URLs](https://docs.cloud.google.com/storage/docs/access-control/signed-urls),
[GCS signing permissions](https://docs.cloud.google.com/storage/docs/access-control/signing-urls-with-helpers).

Federated Kubernetes workload identity is supported beyond AKS/GKE, with provider
trust and permissions configured for the chosen cluster. The application need
not assume a stored account key or a particular Kubernetes vendor.
[Entra workload federation](https://learn.microsoft.com/en-us/entra/workload-id/workload-identity-federation),
[Google Kubernetes federation](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-kubernetes).

Neither provider gives a transaction across PostgreSQL and blobs, or across
providers. Publish and cleanup workflows need explicit reconciliation. Retaining
historical revisions requires a deliberate lifecycle contract, not just an ETag
or generation token.
[Azure transaction limits](https://learn.microsoft.com/en-us/azure/architecture/guide/multitenant/service/blob-storage),
[GCS object atomicity](https://docs.cloud.google.com/storage/docs/consistency).

## Verified Exchange constraints

Creating an event sends invitations. Room acceptance or rejection is a separate
response, and room policy can require delegate approval.
[Create event](https://learn.microsoft.com/en-us/graph/api/user-post-events?view=graph-rest-1.0),
[room calendar processing](https://learn.microsoft.com/en-us/powershell/module/exchangepowershell/set-calendarprocessing?view=exchange-ps).

An event's `changeKey` identifies its version. The event update reference does
not establish an event-specific conditional-write guarantee; the conflict
implementation needs validation against the supported provider behaviour.
[Event properties](https://learn.microsoft.com/en-us/graph/api/resources/event?view=graph-rest-1.0),
[update event](https://learn.microsoft.com/en-us/graph/api/event-update?view=graph-rest-1.0).

Webhook delivery can fail or be missed, so notifications need a recovery and
resynchronisation path. Calendar-view delta is scoped to a calendar and a fixed
date range; it is not an all-history change log.
[Notification recovery](https://learn.microsoft.com/en-us/graph/change-notifications-lifecycle-events),
[calendar-view delta](https://learn.microsoft.com/en-us/graph/delta-query-events).

Immutable event IDs remain stable within a mailbox, with documented exceptions.
Creation can include a client `transactionId` to avoid redundant events on retry;
this does not establish an exactly-once synchronisation guarantee.
[Immutable IDs](https://learn.microsoft.com/en-us/graph/outlook-immutable-id),
[event transaction IDs](https://learn.microsoft.com/en-us/graph/api/resources/event?view=graph-rest-1.0).

Organiser cancellation and deletion of a room's calendar copy have different
semantics. The cancellation design must preserve that distinction.
[Cancel event](https://learn.microsoft.com/en-us/graph/api/event-cancel?view=graph-rest-1.0),
[delete event](https://learn.microsoft.com/en-us/graph/api/event-delete?view=graph-rest-1.0).

Delegated creation can keep the calendar owner as organiser while a PA performs
the action. Microsoft's example does not preserve the delegate's identity in
the event itself, so the application's actor record is a separate concern.
[Delegated calendar creation](https://learn.microsoft.com/en-us/graph/outlook-create-event-in-shared-delegated-calendar).

Delegated calendar permissions and application calendar permissions confer
different authority. An application's mailbox access does not by itself
establish whether a particular PA is authorised to act for a particular person.
[Calendar permissions](https://learn.microsoft.com/en-us/graph/permissions-reference#calendarsreadwriteshared).

No verified generic room-swap contract guarantees retention of the old room
until a replacement accepts. The agreed [room recovery contract](research/api-surfaces/room-recovery.md)
retains the meeting and attendees with no secured room when replacement fails
after old-room loss, notifies the organiser, and offers replacement choices or
explicit cancellation. Replacement confirmation requires acceptance/capacity
validation; exact provider compensation mechanics still require tenant validation.
[Event updates](https://learn.microsoft.com/en-us/graph/api/event-update?view=graph-rest-1.0).

Room request approval, calendar delegation and the application's mailbox access
are distinct permissions. `BookInPolicy` and resource approval delegates govern
room admission; neither establishes general authority over another staff member's
calendar.
[Room calendar policy](https://learn.microsoft.com/en-us/powershell/module/exchangepowershell/set-calendarprocessing?view=exchange-ps).

The owner calendar's permission collection exposes sharing and delegate roles,
but it is not documented as a complete enumeration of every mailbox authority
source. Mailbox-level rights require separate consideration.
[Calendar permissions](https://learn.microsoft.com/en-us/graph/api/calendar-list-calendarpermissions?view=graph-rest-1.0),
[Exchange permission distinctions](https://learn.microsoft.com/en-us/exchange/collaboration-exo/shared-calendars/understand-calendar-delegate-permissions).

App-only calls use the application's authority. SpaceWeave must enforce the
chosen human delegation rule itself; a successful provider write does not prove
that the initiating PA has native calendar-delegate rights. Application RBAC and
Entra application grants are additive, so a scoped RBAC grant does not narrow an
existing unscoped Entra grant.
[Permission types](https://learn.microsoft.com/en-us/graph/permissions-overview),
[Application RBAC](https://learn.microsoft.com/en-us/exchange/permissions-exo/application-rbac).

App-managed delegation does not automatically change native Outlook access.
Native permission changes are separate, authorised calendar-permission
operations. The selected design keeps app-managed grants local to SpaceWeave.
[Create calendar permission](https://learn.microsoft.com/en-us/graph/api/calendar-post-calendarpermissions?view=graph-rest-1.0),
[update calendar permission](https://learn.microsoft.com/en-us/graph/api/calendarpermission-update?view=graph-rest-1.0).
