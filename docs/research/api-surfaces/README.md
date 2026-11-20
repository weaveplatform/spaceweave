# API architecture: surfaces, consumers and contracts

SpaceWeave is a workplace booking platform for one organisation with approximately
100,000 staff and 60,000 desks across multiple offices, buildings and floors.
It combines advance desk allocation, direct space booking, meeting-room calendar
synchronisation, parking, hosted visits, event registration and attendance reporting.
Staff use desktop/mobile web applications; administrators use a separate portal.

The [API surface map](../../diagrams/README.md#proposed-api-surfaces) shows consumers,
application surfaces and contract boundaries. The complementary
[component diagram](../../diagrams/README.md#api-component-architecture) shows API
adapters, owning application capabilities, workers and provider dependencies.
Both are available as native draw.io source and SVGs with editable diagram data.

This document defines the external and internal API boundaries for technical
architects and engineers. It explains consumers, identity and authority, data
exposure, consistency, long-running operations and compatibility. It requires no
knowledge of the design discussions. The detailed [architecture candidates](../architecture-options/README.md)
show alternative implementation layouts for the same API requirements.

The supported business REST API serves the UI, Model Context Protocol (MCP)
servers, software development kits (SDKs), scripts, tests and Terraform providers.
Public means a documented consumer contract with a compatibility commitment;
every operation still requires its appropriate identity and permissions. Worker
commands, queue payloads and persistence interfaces remain private. Staff/admin
access is Internet accessible under the applicable Microsoft Entra Conditional
Access policy and application authorisation.

The requirements in this catalogue are product constraints. Illustrative paths,
wire schemas and component names are design proposals. Protocol claims are
supported by the linked standards/provider research, and implementation claims
are confined to the [current-system inventory](current-surfaces.md).

## Reading guide

| Topic | Purpose |
| --- | --- |
| [Request contract](request-behaviour.md) | Booking retries, configuration concurrency, actor budgets and operation lifecycle. |
| [Event workflows](event-workflows.md) | Registration, venue recovery, exclusive offers, reconfirmation and admission. |
| [Workplace lifecycle](workplace-lifecycle.md) | Scheduled retirement, identity-preserving reactivation and dependency-checked deletion. |
| [Plan publication](plan-publication.md) | Asset approval versus operational mapping activation and dated-inventory validation. |
| [Operational incidents](operational-incidents.md) | Shared outage declarations and owner-controlled deadline protection. |
| [Linked bookings](linked-bookings.md) | Pre-start paired cancellation, explicit post-start scope and automatic desk-only release. |
| [Room recovery](room-recovery.md) | Retain meeting/attendees after failed replacement and expose actual room outcomes. |
| [Idempotency and concurrency](idempotency-and-concurrency.md) | HTTP semantics, durable acceptance, replay safety and retention tradeoffs. |
| [API contract engineering](api-contract-engineering.md) | Primary guidance for specifications, generation, admission, backpressure and clients. |
| [Protocol contracts](protocol-contracts.md) | HTTP, OAuth, errors, pagination and compatibility facts. |
| [Client authentication](client-authentication.md) | Consumer/actor distinctions and supported authentication mechanisms. |
| [Access channels and RBAC](access-channels-and-rbac.md) | Configurable roles, scoped grants, delegates and machine authority. |
| [Authorisation groups](authorization-group-facts.md) | Entra membership, SCIM correlation and freshness constraints. |
| [Conditional Access](conditional-access.md) | Identity-policy enforcement boundaries for staff, applications and visitors. |
| [Automation consumers](automation-consumers.md) | MCP, Terraform, SDK and script-specific contract requirements. |
| [Provider protocols](provider-contracts.md) | SCIM, Graph, Teams, Workday and signed storage transfers. |
| [Current-system inventory](current-surfaces.md) | Existing floor-plan HTTP/file/CLI consumers and migration implications. |

## What public and private mean

Record four independent attributes for every surface:

| Attribute | Values/examples | Question it answers |
| --- | --- | --- |
| Contract audience | Supported public REST for UI and programmatic clients; provider protocol; private application/operations. | Who may build a consumer and rely on compatibility? |
| Network exposure | Internet edge governed by the applicable access policy; cluster/private listener; in-process only; outbound only. | From where can the caller reach it? |
| Caller identity | Staff session; restricted visitor session; registered application; provisioning client; verified provider; workload/process identity. | Who is making this request? |
| Data/action authority | Self, explicitly delegated owner, permitted offices, event/visit scope, global capability, provider/source scope. | What may that caller see or change? |

The agreed **public API** is the supported REST contract family consumed by the
UI as well as MCP servers, SDKs, scripts, test clients and Terraform. The UI is
a first-party consumer of that contract, not a separate private business API.
Public means supported consumer contract; authenticated capabilities and record
scope still govern every operation.

**Private APIs/interfaces** serve platform workers and other application/runtime
collaborators. In the monolithic candidates these are primarily Go calls and
versioned job/fact contracts. A separate private HTTP/RPC interface is warranted
only if a real remote platform consumer requires it; it is not implied here.
Authentication callbacks/session handling remain protocol plumbing rather than
an alternate UI-only business API.

An exported Go name or `internal/<module>/api` directory is not a public network
API. An Internet-reachable callback can be provider-restricted. The staff and
admin public REST access model is Internet access governed by Conditional Access
and application authorisation, rather than a required VPN/private admin network.
These distinctions follow the [protocol and visibility research](protocol-contracts.md#vocabulary-for-comparing-the-five-options).

## Users and consumers

| Consumer | Needs from SpaceWeave | Authority and data limits |
| --- | --- | --- |
| Staff member | Find offices/spaces, declare attendance intent/travel, manage own bookings, personally check in, register for events, invite visitors, read own notifications/history. | Own resources plus deliberately shared workplace/team information; no general access to colleagues' attendance evidence. |
| Personal assistant/delegate | Find/book/change/cancel for an authorised owner and understand pending outcomes. | Real actor and owner remain separate; current app grants or fresh native-derived authority; no checking in another person's desk. |
| Office/facilities administrator | Configure assigned offices, buildings, spaces, neighbourhoods, calendars and permitted policy overrides; review allocation exceptions. | Capabilities combined with one-or-more office scopes. An office ID in a request is not authority. |
| Reception | Find expected visitors, record arrival/departure and perform authorised admission scans. | Assigned offices/events and necessary identity details; no global directory/reporting privilege by default. |
| Event organiser | Manage an authorised event, audience/access rules, venue, free tickets and attendance/admission work. | Authorised events and secured venue capacity; organiser authority is checked per operation. |
| Reporting administrator | Read individual/aggregate attendance within scope, with evidence categories and freshness. | Reporting capability and assigned offices; separate retention categories remain in force. |
| Global administrator | Identity/source mappings, organisation policies, integration configuration and global grants. | Explicit global capabilities; office administration does not imply them. Secrets stay in the selected secret-management mechanism. |
| External visitor/attendee | Verify email, access their invitation/visit/ticket, register where permitted. | Their verified session, visit/event/audience/window; never a staff principal. |
| Internal-event visitor, such as a guest speaker | Follow an event invitation/access flow, verify email and see permitted event/visit/ticket information. | Visitor scope for that internal event; the answer does not approve an openly browsable public event catalogue. |
| UI clients | Implement staff/mobile-web and admin experiences through the supported public REST operations. | Session-derived human identity, owner/delegation/capability checks; hiding a button is not authorisation. |
| MCP server | Translate a user's tool/resource request into an approved public REST operation. | Runs in that user's context and current authority; client identity is retained separately. No machine-authority fallback. |
| SDK or script | Call the public REST contract with typed/manual transport, paging, versions, retries and async status. | SDK/language is not an identity or permission; use the actual human/application credentials. |
| Contract/acceptance test client | Exercise supported REST behaviour as a real consumer. | Test identities and environments; no production authentication/worker-state bypass. Private implementation tests still use private interfaces where appropriate. |
| Terraform provider | Read/refresh/import and reconcile long-lived workplace and permitted policy/configuration resources through public REST, with useful read-only data sources. | Personal bookings/intents/check-in/tickets are outside initial managed-resource scope. First-party provider implementation follows the core platform. |
| Registered organisation application | Query/automate explicitly approved capabilities through the same public REST family. | App registration and exact grants/actions remain to be defined. A human delegate grant is not automatically a machine grant. |
| Entra provisioning client | Create/update/deactivate/reactivate/delete SCIM users and mapped groups. | Separate provisioning authority; no general booking permission. |
| Graph/Teams provider | Deliver protocol-specific notification/installation/conversation activities. | Validate the particular provider protocol; not a browser session or generic staff mutation token. |
| Occupancy source | Optionally submit or supply provenance-labelled sensor evidence. | Registered source, office/device scope and stated evidence type; cannot mark a desk owner checked in. |
| API/worker/maintenance code | Call business interfaces, execute durable jobs and project committed facts. | Owning interfaces and scoped runtime credentials; no extra network service is implied. |
| Platform/SRE tooling | Probe health, collect telemetry, diagnose authorised workflow failures and operate recovery. | Private operational access; business repair actions additionally need explicit application authority and audit. |

Human and application actors have separate authority. The
[glossary](../../../GLOSSARY.md#spaces-and-locations) distinguishes Actor,
Application actor, Booker, human delegation and an application-initiated booking.
The staff booking owner remains distinct from the initiator. Accepting an
arbitrary `staffId` cannot turn an application into that human or confer their
personal check-in, delegation or administrative authority.

## Surface catalogue

Surfaces are named for the capability or protocol they provide. Suggested
paths are deliberately separate from both Go package paths and final hostnames.
User-facing APIs are Internet accessible under Conditional Access for their
REST; private workers/operations and provider-specific identity rules remain
distinct. Individual hostnames, listeners and connectivity profiles are detailed
deployment choices.

| Surface | Intended consumers | Kind and proposed base | Public/private contract | Exposure/authentication proposal |
| --- | --- | --- | --- | --- |
| Browser authentication and sessions | Staff/admin/visitor browsers and Entra. | OIDC redirects/callbacks; session HTTP under `/auth/`. | First-party plus standards-specific identity exchange. | Browser-reachable origins; Go handles provider exchange; secure cookies, redirect-state/nonce/PKCE as applicable and CSRF controls. |
| Staff and delegate booking | Staff/mobile-web, PA and permitted programmatic consumers. | Public REST/JSON `/api/v1/` business capabilities. | Supported public consumer contract. | Internet-accessible; Conditional Access for applicable identities, approved session/token profile and current owner/action checks. |
| Office administration and reporting | Office admin, reception, event administration and reporting clients. | Public REST operations, illustratively `/api/v1/admin/`. | Supported public contract with restricted capabilities. | Internet-accessible under Conditional Access; capability/office/event checks still apply, regardless of UI or automation client. |
| Organisation administration | Global policy, identity and integration administration clients. | Public REST organisation capabilities, illustratively `/api/v1/organisation/`. | Supported public contract, highly restricted authority. | Internet-accessible with applicable Conditional Access and explicit global capabilities. |
| Visitor event and ticket access | Verified visitor invitation, registration and ticket clients. | Scoped REST under `/api/v1/visitor/`. | Supported public visitor contract with restricted data/actions. | Internet edge; existing verified-email visitor identity and event/visit scope, not an invented staff Entra identity. |
| Visitor invitations and verification | Entry/verification for visitors attending internal events, including speakers. | Invitation landing and verification protocol, illustratively `/invitations/{opaqueReference}`. | Limited external entry surface; no general event-discovery catalogue established. | Minimal safe landing before proof; personal event/visit/ticket detail follows the agreed verified visitor flow. |
| Programmatic REST access | MCP servers, SDKs, scripts, tests, Terraform and other enrolled applications. | Consumer views of the same public REST family. | Supported public contract, not a second parallel business API. | Human/session or OAuth/application profile as approved; token audience and application grants remain explicit. |
| User and group provisioning | Entra user/group provisioning. | SCIM 2.0 `/scim/v2/Users`, `/Groups` and supported discovery. | Published provider/provisioning protocol. | Provisioning-only authentication; direct Entra connectivity or a supported private-agent route. |
| Exchange change notifications | Exchange change/lifecycle delivery from Microsoft Graph. | Provider callback `/integrations/graph/notifications` and lifecycle route. | Provider-specific ingress, not a general booking API. | Public HTTPS edge with Graph challenge/notification validation; enqueue or persist promptly. |
| Teams activities | Teams installation/conversation/activity handling. | Teams activity protocol `/integrations/teams/activities`. | Provider-specific ingress supporting optional personal delivery. | Provider-authenticated messaging edge; registration/SDK choice remains open. Installation never grants booking authority or message opt-in. |
| Occupancy evidence ingestion | Optional occupancy connectors calling Booking, which owns validated operational evidence and commits facts for Attendance projections. | Source-specific push HTTP/batch or outbound pull adapter. | Published source-restricted ingestion where push is selected; private connector execution. | Restricted Internet edge for configured push sources, or outbound pull; source identity and office/device/evidence scope, never staff check-in authority. |
| Protected asset transfer | Floor-plan manifests, source uploads, approved renders, ticket/export bytes. | Authorised metadata/capability HTTP plus bounded binary transfer. | Access-controlled asset contract; public frontend bundles are separate. | App authorises before stream/signing; storage capability limits object, method and lifetime. |
| Workflow status and recovery | Administrative workflow status and permitted repairs. | Public REST status/command operations with restricted operator capabilities. | Supported public contract for authorised operators/clients. | Internet-facing access follows applicable Conditional Access; private execution stays behind owning interfaces. No generic SQL/event injection/state setter. |
| Application health and observability | Liveness/readiness, metrics, telemetry export and diagnostics. | Private probe HTTP, optional metrics scrape, OTLP and structured logs. | Private operational contracts. | Dedicated listener/routing and platform identity controls; no general portal access to raw metrics/debug endpoints. |
| Internal application interfaces | Module/application calls by handlers, CLI and workers. | Synchronous Go interfaces with explicit context/actor/outcome. | Project-private in-process interfaces. | No URL, CORS, HTTP token, OpenAPI resource or independent service deployment implied. |
| Background jobs and committed events | Durable work and committed-fact consumers. | Versioned PostgreSQL jobs/outbox envelopes and owning handler contracts. | Private asynchronous application contracts. | Leased workers with registered payload versions; at-least-once processing and retry/reconciliation semantics. |
| Outbound provider integrations | SpaceWeave as a client of providers. | Graph REST/OAuth, Workday selected HR protocol, Azure/GCS SDK/API, email/personal Teams delivery, telemetry export. | Provider-owned outbound contracts. | Scoped workload/provider credentials. There is no matching inbound SpaceWeave HTTP endpoint for every provider method. |
| Local tracing workbench | Existing local tracing CLI/workbench during migration. | Current Cobra commands, handwritten HTTP and generated-directory file reads. | Existing local/developer contracts. | Preserve explicitly during adoption; current host/custom-header protections are not production staff authentication. |

See [provider contracts](provider-contracts.md) for the exact native media types,
challenge responses, authentication alternatives and lifecycle requirements.
SCIM/provider wire formats are not wrapped in our general REST success/error
envelope. Probe responses are not portal business resources.

## Proposed operations and representation limits

The following endpoint families are sketches for contract review. Each needs a
defined request/result/error schema and owner; these are not implemented routes.
Paths below are relative to the shared public REST family, with capability-based
grouping where helpful. The UI and approved programmatic clients call the same
business operations under their actual authority and permitted projections.

### Staff and delegate operations

| Example operation | Consumer purpose | Important contract particulars |
| --- | --- | --- |
| `GET /me`, `/me/delegations`, `/me/preferences` | Establish own identity, permitted owner choices and preferences. | Server derives the actor; grants expose permitted actions/source/freshness without provider credentials. |
| `GET /offices`, `/offices/{id}/floors`, `/spaces` | Find an office/floor and suitable space. | Bounded filters for space kind, requirements, capacity, time and office. Read permission and book permission remain separate decisions. |
| `GET /availability?officeId=...&from=...&to=...` | Find dated/timed suitable capacity. | A snapshot with generation time and relevant versions, not a promise or lock; bounded time range, features and result size. |
| `GET /people?...` | Find intended collaborators. | Minimal directory plus agreed relevant teammate/collaborator planned-day/desk projection; no source identifiers, employment details or general raw attendance payloads. |
| `POST /attendance-intents`, `PATCH /attendance-intents/{id}` | Declare chosen dates, office/travel, required features and explicit collaboration. | Explicit owner, authorised actor, expected revision and retry semantics. Keep imported teams distinct from internal allocation groups. |
| `GET /me/bookings`, `/bookings/{id}`, `/waitlist-offers/{id}` | Understand allocation, room acceptance and outstanding choices. | Kind-specific business state, including no-secured-room recovery, deadlines, office timezone, policy version and permitted next actions; no inference that a booked desk proves attendance. |
| `POST /desk-bookings`, `/linked-booking-requests` | Book spare capacity after the main run, including both-required parking. | One Booking transaction; waitlist precedence; no premature direct claim before the cohort run; never split a both-required request into client-coordinated calls. |
| `POST /bookings/{id}/transfers`, `/cancellations` | Secure a replacement or cancel an authorised booking. | Transfer is atomic including linked parking. Before either starts, explicit cancellation of either booking in a both-required pair cancels both atomically; optional links cancel independently. Once either starts, cancel the selected booking unless both are explicitly chosen. Room cancellation records Exchange reconciliation state. |
| `POST /bookings/{id}/check-ins` | Personally take an allocated desk. | Actor must be the assigned owner. QR supplies a desk reference, not authority or proof of physical identity/location. |
| `POST /waitlist-offers/{id}/acceptances` or `/declines` | Respond to a still-valid offer. | Server checks current offer/version/deadline, protected outages and required linked capacity. |
| `POST /room-requests` | Request a suitable Exchange-connected room. | Creates a pending request/hold; owner is organiser, actor is retained; response distinguishes acceptance and sync state. |
| Room change/recovery commands under `/room-requests/{id}/...` | Choose an available replacement for an existing meeting or explicitly cancel under current authority. | Failed replacement after old-room loss retains meeting/attendees with no secured room and organiser notification. Candidate acceptance and Booking capacity validation precede confirmation; exact command paths remain engineering work. |
| `POST /visits`, `/events/{id}/registrations` | Invite a hosted visitor or obtain a free ticket. | Host/event authority and event audience rules; issue no usable visitor ticket before its required visit link exists. |
| `GET /me/attendance`, `/me/notifications` | Read own evidence/history and messages. | Distinct bookings/check-ins/occupancy evidence, freshness and retention; no minimum-attendance enforcement. |
| Message-specific Teams delivery preference | Opt in to the private booking-raised message. | Explicit individual/message consent, rechecked at dispatch; no shared channel or implied permission for other Teams messages. |

GET reads never perform check-in, consume an offer, issue a ticket or cancel a
booking. Do not expose a general `PATCH booking.state = taken/confirmed` escape
hatch. The application validates transitions regardless of which surface calls.

### Administrator and operator operations

| Family | Consumer/action | Required distinction |
| --- | --- | --- |
| Workplace definitions | Office-scoped facilities staff configure buildings/floors/spaces/features/baseline bundles and calendars. | Definitions versus dated inventory and protected confirmed claims; impactful changes return explicit impact/workflow results. |
| Allocation control | Authorised admins inspect cycles, demand, group proposals, placement progress and unfulfilled demand; trigger permitted work. | Proposal versus committed assignment; duplicate triggers do not create duplicate runs/claims. |
| Plan management | Upload sources, inspect trace/review and approve immutable assets through Plans; publish operational geometry-to-stable-space mappings through Workplace after Booking validates impact. | Approval versus activation have separate outcomes, including in a combined admin workflow; source/report/draft access differs from staff-facing manifests. |
| Reception/admission | Expected visits, arrival/departure and event-ticket scans. | Office/event capabilities; reception arrival and ticket admission are different records. Duplicate admission is explicit. |
| Event management | Authorised organisers manage event audience, access, venue and ticket cap. | Current secured venue and active issued count are checked atomically; no arbitrary cap write through a generic admin DTO. |
| Reporting | Scoped individual/aggregate reports and, if selected, asynchronous exports. | Subject/office/date filters authorised server-side; output fields, download access and retention are part of the contract. |
| Policy publication | Publish permitted office overrides or organisation policies. | Global versus office authority; validation, effective version, impact and explicit migration of old confirmations. |
| Configurable RBAC | Existing authorised global administration configures roles and scoped assignments; SCIM supplies configured group mappings. | No added office-level role-delegation hierarchy. Grant-only composition, complete scope pairings and current domain checks remain enforced. |
| Identity/integration configuration | Global admins manage correlation review, mappings, client/source enrolment and integration status. | Source-owned membership is not locally editable; secret references/status are not secret values. |
| Failure repair | Operators inspect redacted job/operation status, retry an eligible failure or resolve incompatible calendar intent. | Narrow authorised action, current version, reason and audit; replay is not permission to bypass owner checks. |

Confirmed event venue loss retains tickets pending recovery and pauses both
admission and new issuance. The organiser explicitly secures a suitable
replacement or cancels; confirmed changes are communicated to attendees.
Office/time changes require attendee reconfirmation and linked visit validation.
A later invalid visit blocks admission while retaining the ticket and seat
pending authorised repair or explicit cancellation. Attendees can cancel their
own unused tickets; optional event waitlists receive released capacity before
fresh registrations through time-limited, eligibility-checked offers. Event
cancellation also cancels visits whose only remaining valid purpose was that
event. The [event workflow contract](event-workflows.md) defines
exclusive FIFO offers, decline/expiry/rejoin, the registration cutoff and
published reconfirmation deadlines releasing unconfirmed tickets. Unused-ticket
cancellation also cancels an associated future visit with no other valid purpose,
preserving other-purpose visits and reception history. Confirmed outages pause
affected reconfirmation deadlines; venue recovery preserves offers and pauses
expiry within registration limits. Temporary queue ineligibility preserves
position without capacity; office/time changes withdraw old offers without
ending requests, preserving position for updated offers. Initial numeric
offer/notice/recovery-grace defaults remain unselected.
When registration creates a visit solely for a ticket, Events and Visitors
commit both or neither; an existing valid reused visit survives failed issuance.

An admin portal remains a separate user experience. Its route prefix alone is
not an authorisation mechanism, and broad Kubernetes access is not automatically
application reporting or booking-repair authority.

### Internal-event visitors

External visitors attend internal events, for example as invited guest speakers.
An externally reachable invitation/verification flow grants access to the relevant
event, visit and ticket. A general anonymous event catalogue is outside the scope. Existing event audience/registration rules remain explicit; an
invitation grants neither staff identity nor arbitrary space-booking authority.

A safe landing may initiate verification without exposing the host's directory
record, attendee list, other office bookings, plan sources or attendance history.
Detailed personal access uses the existing verified-email visitor profile.

Email verification/session initiation is a separate operation from granting a
ticket. Exchange a verified, limited token for an appropriately scoped visitor
session; avoid treating an invitation URL as indefinite staff-like access.
Safe landing requests must not accidentally consume an entitlement or record
admission when a mail scanner follows a link. Define redemption, expiry, replay
and session scope explicitly.

Visitor reads expose only the permitted invitation/visit/event/ticket fields.
An opaque ID is not authorisation. Ticket admission is an authenticated
reception/organiser operation, not an anonymous GET performed by scanning a URL.
Rate limits and verification responses account for email enumeration and bulk
registration without exposing internal staff or capacity administration.

### Public REST consumers

The UI, MCP servers, SDKs, scripts, tests and Terraform use the public business
contract. Each consumer authenticates with the appropriate identity profile;
a consumer label is separate from a permission role. Reuse one supported
business contract family, with capability/audience projections and approved
authentication profiles rather than a private UI business-contract fork.
Actual enrolled applications/actions still need explicit grants; public API
publication is not employee-wide export or unrestricted service-account authority.

For token-based clients, use an explicit API token audience and enrolled application
identity with separately reviewed delegated scopes/application roles. Staff
browser ID tokens and Graph tokens are not SpaceWeave API access tokens.
`roles` being present does not alone identify an app-only token; validate the
complete intended caller profile. See [identity facts](protocol-contracts.md#entra-identities-and-api-permissions).

Proposed contract particulars are:

- Shared UI/programmatic capabilities and projections published through the
  supported contract, with stable operation identifiers and compatibility policy.
- Read-only and modifying authority distinguished. A service acting for a
  person requires an explicit service grant/actor model; a supplied owner ID or
  a human PA's unrelated grant is insufficient.
- Exact office/owner/event/source filters evaluated in the application. OAuth
  permission alone is not a grant to every record returned by the query.
- Explicit cost/admission classes and budgets, bounded pagination, retry conventions
  and asynchronous result access, with an owned support/deprecation process.
  The [agreed request contract](request-behaviour.md) uses actor/class
  budgets; client provenance is separate and does not itself create another quota.

Keep Go-managed browser sessions: a browser can use an approved session/CSRF
profile on the public REST operations while other clients use a relevant API
access token. Credential transport differs without forking business behaviour.
No Graph/refresh token moves into browser JavaScript because the API is public.

MCP servers are REST consumers here; the first-party MCP server and Terraform
provider are agreed follow-on deliverables after the core platform and
Go/TypeScript SDKs. Terraform manages long-lived workplace and permitted
policy/configuration initially, not every mutable resource. SDKs derive contracts from
OpenAPI; scripts and public contract tests need the same documented behaviour.
Private module/domain tests remain valid and platform worker operations remain
private. See [automation consumer research](automation-consumers.md).

### Identity profiles and audit meaning

The API supports delegated human authority and independently granted
application authority. They use the same public business operations where
permitted, but remain different actors.

| Example | Initiating actor | Requesting client | Booking owner / authority |
| --- | --- | --- | --- |
| Staff member uses the UI | Staff person. | Staff/admin web application. | Self or an owner they may act for; current human capabilities/delegation. |
| Staff/PA uses MCP | The current authenticated user. | The MCP client/server chain, retained separately. | Self or a named staff owner under that user's current roles/scopes and applicable human delegation. |
| Script uses machine identity | The application actor. | The authenticated application/client identity. | Configuration scope or explicitly permitted staff owners under independent, configurable application RBAC. |
| Worker continues accepted asynchronous work | Preserve the original initiating actor/client when one exists. | Record the executing workload separately. | The committed operation and current owning workflow rules; a worker is not a substitute human booker. |

Actor/identity class and requesting-client provenance come from validated
authentication/session context, not writable request-body assertions. The
booking owner remains a staff member. OAuth consent/API permission, an
application's local grant and record-level authority are distinct checks.

Application grants do not inherit a human's administrative role or PA grants.
App-only credentials cannot perform personal desk check-in. A human using a
client still makes the assigned person's explicit check-in action under their
own identity; the client label supplies no proof of physical presence. Accepted
machine authority also does not override staff-chosen attendance days or the
normal allocation, capacity, waitlist and linked-parking invariants.

Global administration approves applications, capabilities and permitted
office/owner populations, with default denial and current grant checks.
Application role-based access control (RBAC) is configurable: named roles map to
capabilities and scoped assignments grant authority to the appropriate subject.
SCIM-backed group mappings supply scoped grants. Role composition is grant-only,
and clients can discover their own effective permissions. Authorised global
administration manages role definitions and assignments.
Entra application registration/consent and Conditional Access administration
remain the organisation's identity-management responsibilities; an in-app
global capability does not confer an Entra administrator role.

### Terraform and client delivery scope

Initial Terraform management covers long-lived workplace and permitted
policy/configuration resources. Candidate resource families are offices,
buildings, floors, space definitions/features, baseline neighbourhoods, office
calendars and allowed policy configuration. Exact resource names/schemas remain
to be selected. Read-only data sources can query suitably authorised data.

Personal attendance intent/travel, transient desk/room/parking bookings,
check-in, visitor visits and event tickets are not initial declarative managed
resources. Imported Entra/Workday memberships remain owned by those sources.
Terraform cannot bypass definition-change impact checks, pinned policy versions
or protected confirmed bookings. Publishing desired policy configuration may
create a new policy version rather than mutate an existing published version.

First-party MCP server and Terraform provider implementations follow the core
platform and Go/TypeScript SDKs. Their consumer needs shape the
public API now; follow-on implementations remain public REST clients. This does
not add MCP protocol hosting or provider execution to platform-worker APIs.

Authorised UI/API edits to Terraform-managed configuration are permitted; later
Terraform plans expose drift rather than an automatic application ownership
lock. Normal permissions, expected versions and domain impact checks apply to
all writers. This does not make imported Entra/Workday data locally editable or
grant Terraform a force override.

Bulk exports, outbound organisation webhooks and streaming subscriptions are
candidate downstream contracts once actual consumers are known. They are not
silently added to the agreed initial product by this catalogue. No consumer gets
direct PostgreSQL access merely because reporting data exists there.

### Consumer-specific contract requirements

| Consumer | Public REST requirements | Private execution distinction |
| --- | --- | --- |
| UI | Same supported operations, pagination, errors, async state and permissions as other clients; staff/admin remain separate experiences. | Go retains provider tokens and browser session handling; no UI-only claim writer or unsupported business API. |
| MCP server | Tools/resources map to approved REST operations in the initiating user's context; preserve user/client provenance and honour retries/status. | No substitution of application authority, token passthrough or private worker commit calls. |
| SDK | Generate Go/TypeScript transport clients/models from the supported OpenAPI source; implement documented paging, conditional writes, safe retries and operation polling. | SDK conveniences do not contain an alternative allocation/authorisation engine. |
| Script/CLI client | Standard authentication and request/response contracts, deterministic machine errors, stable identifiers and documented limits. | Runtime-host commands and offline tracing tools are separate local/private tooling; a script's language does not grant worker authority. |
| Testing | Public contract/acceptance suites exercise the same HTTP operations and grants as real consumers. | Unit/module/transaction tests still exercise private interfaces; test fixtures do not imply a production bypass API. |
| Terraform | Stable resource IDs, authoritative read/refresh/import, drift and ownership semantics, expected versions, deletion impact and bounded async polling. | Long-lived workplace/permitted policy configuration plus useful read-only queries; no private worker calls or initial management of personal bookings/check-ins/tickets. |

For managed-resource refresh/import, define actual absence separately from denied
access and temporary failure. A Terraform client must not drop state after a
`401`, `403`, throttling or dependency error. Resource contracts that interpret
`404` as deletion cannot mask an authorisation failure as that same result;
scope checks before lookup can return a uniform denial without exposing whether
a particular object exists. Published retirement, immediate or scheduled, excludes
new bookings for use on/after its office-local effective time while keeping
existing confirmations manageable. Pre-existing unconfirmed holds/offers cannot
confirm excluded use; revalidate at confirmation and reconcile late Exchange
effects. Authorised reactivation revalidates definitions,
calendars and inventory while retaining identity/history. Separate deletion reports unresolved bookings,
children or other dependencies and refuses removal without cancelling/cascading.
Retired state remains readable/importable and is not absence. The
[workplace lifecycle contract](workplace-lifecycle.md) supplies these semantics;
an asynchronous status reference cannot hide a dependency conflict.

Internet-facing human access uses Entra Conditional Access rules and policies
alongside API authentication, capability, office, owner and delegation checks.
MCP uses a user-context channel. A machine-identity script uses a separate
application channel. Authentication journeys, protocol mechanics and authority
subjects must be modelled separately; no blanket system-browser/PKCE journey is
selected for SDKs, scripts and MCP. SDK and Terraform credentials follow their
actual host and execution context. See the [corrected matrix](access-channels-and-rbac.md).

Conditional Access is not a single proxy check that automatically covers every
credential. User policies target the configured resource/application; workload
policies have different supported identities/controls, and managed identities
are not covered by service-principal Conditional Access. Entra also cannot
directly invalidate a cookie session owned by SpaceWeave. Configure resource
targeting, session re-evaluation and supported claims/revocation behaviour, while
retaining the agreed denial of locally known effective staff ineligibility.
[Verified Conditional Access limits](conditional-access.md).

Visitors keep the agreed verified-email event/visit profile. Graph and Teams
callbacks keep their provider validation. Neither becomes a staff Entra sign-in
solely because the route is Internet-accessible. Private workers use scoped
workload credentials and private contracts. Authentication profiles must state
their actual enforcement mechanism without promising user MFA/Conditional
Access behaviour for credentials to which those controls do not apply.

## State, concurrency, retry and completion

### Different states need different fields

Keep domain outcome separate from transport and provider progress. A room
request resource may exist while the room is awaiting acceptance. A desk can be
allocated without being personally taken. A provider write can be queued after
the local transaction commits. An operation monitor can finish a local stage
without proving the provider accepted it.

Illustrative room response, not an adopted schema:

```json
{
  "id": "booking-123",
  "kind": "meeting_room",
  "ownerId": "staff-owner",
  "actor": {"kind": "staff", "id": "staff-pa"},
  "bookingState": "pending_acceptance",
  "roomAcceptance": "awaiting_response",
  "synchronisation": {"state": "queued"},
  "holdExpiresAt": "2026-11-19T10:15:00Z",
  "revision": "opaque-revision"
}
```

There is no provider token, delegate ACL or Workday payload in this projection.
State is returned through owning interfaces; the gateway does not infer it from
stale reporting tables. Late acceptance/expired holds remain reconciliation work.

### Three different controls

| Control | Purpose | Does not replace |
| --- | --- | --- |
| Expected version/representation validator | Reject an edit based on an obsolete resource/policy/workflow state. | Capacity locking and retry identity. |
| Command retry identity | Recognise another attempt at the same logical action after timeout/restart. | Current authorisation or state validation for new actions. |
| Transaction and capacity protocol | Enforce one desk per person, competing claims, required parking and ticket limits. | External-provider reconciliation or transport compatibility. |

Shared configuration edits require the previously read version.
Proposed wire form: strong ETags/`If-Match` for those edits, with `412`
for a failed supplied precondition and `428` only where the contract explicitly
requires one. A capacity/state conflict can be `409` with a distinct machine
problem code. Do not apply this convention automatically to SCIM's optional
ETags or assume Graph offers the same conditional-write contract.

Booking creation, ticket issuance and work submissions require retry keys and
atomic acceptance receipts. Document the key header, caller/operation scope, payload equivalence, retention and in-progress/replay
results. Persist acceptance and the key/result atomically. Re-authorise attempts
and redact previously stored outcomes if current access has changed. The
`Idempotency-Key` header remains draft work at research time, so any use here is
our documented contract, not a universal HTTP-server guarantee.

### HTTP outcomes and operation monitors

| Outcome | Proposed HTTP use | Domain meaning |
| --- | --- | --- |
| Resource created locally | `201` plus `Location`. | Resource exists with its explicit state; a pending room request is not a confirmed room. |
| Durable asynchronous work accepted | `202` with operation/status location. | Accepted work can still fail; no implied allocation or provider completion. |
| Read/update/command result available | `200`, or documented `204` for an empty result. | Return the defined current result, not an invented blanket success. |
| Stale supplied validator | `412`. | Client must obtain/merge against the current version. |
| State/capacity conflict | `409` with typed code/current permitted next step. | Capacity gone, offer expired, conflicting intention or invalid current transition. |
| Invalid instructions | Defined `400`/`422` distinction with field issues. | Syntax versus semantically unprocessable request, consistently documented. |
| Invalid/missing identity | `401`. | Authentication is required/invalid for that surface. |
| Authenticated but not permitted | `403`, or policy-defined `404` to avoid disclosing an inaccessible object. | No permission to that resource/action; no extra data in the error. |
| Throttled or temporarily unavailable | `429`/`503`, with retry guidance where applicable. | Not proof that a previous timed-out command failed to commit. |

`GET /operations/{id}` belongs to the public REST contract with access restricted
to the authorised initiator/owner/operator scope. Define execution states and the
required allocation/plan-processing/export cancellation separately from the business resource.
Return bounded error/result links and progress/freshness, not raw provider
payloads, SQL or secrets. Expiry of operation history does not delete the booking.

Use RFC 9457 problems for the general application REST contract, with stable
problem identifiers/codes and safe detail. SCIM errors, Graph validation
responses and Teams protocol acknowledgements retain their native contracts.
The [HTTP research](protocol-contracts.md#verified-http-semantics) provides the
normative distinctions. The product consistency requirements are fixed; exact
wire schemas and proposed status-code mappings still require contract validation.

## Reads, identity disclosure and performance

Propose opaque cursor pagination for bounded app lists with deterministic
ordering, explicit filters and maximum page/window/cost limits. State whether a
list is a live view or a consistent snapshot; a cursor does not automatically
freeze a changing waitlist or availability map. SCIM keeps its own pagination.

Time-based requests carry real instants and the relevant office calendar/date
semantics. Define half-open intervals and DST/invalid-local-time handling once
in the contract; do not let browser, worker and Exchange independently guess.
Use immutable asset revisions and representation validators where useful.

Staff directory search and named planned desk/day sharing are different from
identified check-in/occupancy reporting. Relevant teammates and explicit
collaborators may see planned days/desks within the shared office context.
If named desk allocations and live taken states are both returned, their
combination can reveal presence; simply labelling the latter anonymous does
not prevent that inference. Default general availability projections can use
coarse unavailable/available states, with detailed desk-use evidence reserved
for the owner and authorised reporting/operational consumers.

Cache policy is per representation. Personal bookings, invitations, tickets,
attendance and signed capabilities need explicit private/no-store treatment as
appropriate; frontend bundles and immutable public-safe assets have different
rules. A `304` path still authenticates/authorises the caller. Never serve one
user's cached projection to another user because an ID or office filter matches.

Batch queries should express bounded dates/spaces/features instead of causing a
request per desk. The 100,000-staff/60,000-desk target needs representative peak,
hot-office and queue-drain tests. Proposed initial status updates can use bounded
polling/conditional reads. SSE/WebSocket, externally supported streams and bulk
BI delivery require a consumer/use case and their own replay/auth/backpressure
contract; none follows merely from choosing React or Kubernetes.

## Assets and native integrations

Approved floor geometry is a consumer contract even if delivered as JSON files
rather than under `/api`. Plans approves immutable assets; Workplace separately
activates their geometry-to-stable-space mappings after Booking validates dated
inventory/commitment impact. One admin workflow may do both, exposing each actual
outcome. The [plan publication contract](plan-publication.md) applies across all
five architecture candidates. Current `generated/catalog.json`, plan/report/original
reads therefore belong in the migration inventory. Do not transfer broad
generated-directory access into the production staff/visitor surface.

Authorise a source upload or revision download before issuing a signed storage
capability. Bind uploads to staging/object identity, accepted type/size/checksum
rules and completion verification; a signed upload does not mean a reviewed,
durable or published plan. Restrict access to source drawings, trace reports and
optional transcripts separately from a staff-facing render manifest.

An issued storage capability can remain usable until expiry according to its
provider contract. For data requiring immediate permission re-evaluation, choose
an authenticated streaming/proxy contract rather than promising that revoking
an app grant instantly revokes every previously issued URL. Details and provider
limits are in [storage research](provider-contracts.md#private-azuregcs-assets-and-signed-transfer-capabilities).

SCIM's native media type, PATCH/filter/list/errors and deletion semantics are
distinct from app REST. Graph's ownership challenge, basic notification secret
validation and rich encrypted/JWT notification modes are different checks.
Teams needs its own authenticated activity/installation/conversation handling.
Outbound Workday is not an invented inbound SCIM feed or necessarily Graph-like
REST: verified WWS operations are SOAP/WSDL; select actual tenant operations and
credentials. Keep these protocol adapters separately registered and tested.

## Private interfaces and operational contracts

Handlers and workers call small business interfaces directly. A command carries
the actor, authorised owner/context, desired action, expected state and retry
identity where required. Results include business outcomes and allowed next
steps. Private repository/provider ports and transaction participation remain
implementation contracts rather than public HTTP resources.

Durable jobs/facts have producer/version, subject, operation/revision identity,
minimal payload and defined handler/retry/fencing rules. They do not grant
authority to invent a confirmed booking, and an external application cannot
enqueue an arbitrary internal event to bypass the command interface. One image
running several roles does not require private gRPC/HTTP between those roles.

Separate private process health, dependency readiness and business workflow
health. A healthy desk API with delayed Exchange sync reports that degradation;
it does not advertise every room as confirmed or restart endlessly. Restrict raw
metrics, profiles/debug endpoints, callback secrets and detailed job payloads to
their operational audiences. Application trace/metric export uses OTLP to the
Collector; structured logs remain a distinct collection contract.

## Specifications, ownership and compatibility

Use a shared supported public REST contract family for UI and other consumers.
Capability/security projections may produce contract views without forking
business semantics or creating a private UI API. Suggested fragment names below
remain illustrative and independent of the repository architecture choice.

```text
api/src/v1/workplace/       public REST office/floor/space definitions and queries
api/src/v1/booking/         public REST intents, claims, offers and check-in commands
api/src/v1/events-visits/   public REST event/visitor/ticket capabilities and scoped views
api/src/v1/administration/  public REST restricted policy/configuration/report operations
api/src/v1/common/          public transport errors, pagination and operation status
api/clients/               generation settings for supported UI/SDK consumers
api/profiles/scim/          SCIM standard and Entra profile/compatibility tests
api/profiles/providers/     Graph/Teams callback contract profiles and validation tests
api/profiles/operations/    private probes/telemetry/control documentation
```

These are logical bundles, not a mandate to replace every candidate's existing
spec file layout. An alternative may assemble one OpenAPI source into several
audience views. Document security per operation and keep incompatible native
protocols in their own profiles; OpenAPI generation does not replace protocol
implementation. Select generator/spec versions against actual required features.

The public REST family requires a version, deprecation and support policy
across UI, MCP, SDK, script, testing and Terraform clients. A UI deployment does
not imply those other clients have upgraded. Compatible changes stay within a
major version; breaking changes require a new major and migration/deprecation
policy. Cached/open browser sessions and rolling API releases also remain
supported within that policy. The initial retirement notice is at least
180 days once replacement/migration guidance is available. Configure future
notices organisation-wide and honour already published support dates.
Private Go contracts evolve with the repository; private persisted job/fact
versions still need mixed-version compatibility and recovery/replay support.

Compatibility review considers supported client behaviour, not just whether a
schema diff looks additive. New required request fields, narrower validation,
changed state/error/paging semantics and enum additions that break exhaustive
clients need explicit assessment. Proposed validation uses prior supported
OpenAPI/SDK/client fixtures and meaningful consumer journeys during implementation.

For deprecation signalling, RFC 9745 defines `Deprecation` and its link relation;
RFC 8594 defines `Sunset` for expected withdrawal. Those mechanisms communicate
lifecycle information; they do not choose a support period or itself retire a
version. Their use is a proposed implementation of the agreed version policy.
[Deprecation](https://www.rfc-editor.org/rfc/rfc9745.html),
[Sunset](https://www.rfc-editor.org/rfc/rfc8594.html).

Go/TypeScript clients target the shared supported REST contract. Private Go
module/worker contracts are separate from SDK contracts. First-party MCP/Terraform
implementations follow the core platform and SDKs, with their exact packaging
and release ownership still to be designed.

Each operation has one business owner, a wire-contract owner, authorised caller
types, field projection, rate/cost bounds, failure/retry behaviour and validation
evidence. A gateway may compose published queries but does not create a second
capacity writer. Ownership differs across the five layouts; the same external
surface must continue to enforce the agreed invariants.

## Design status and remaining decisions

The API requirements include one public business contract across supported clients,
independent human/application authority, configurable scoped RBAC, SCIM-backed role
mappings, and version compatibility independent of portal releases. Terraform
initially manages long-lived workplace and permitted configuration resources.
Authorised UI/API edits remain valid and appear as drift during reconciliation.
First-party MCP/provider delivery follows the core platform and Go/TypeScript SDKs.

Booking creation, ticket issuance and background-work submissions require retry
keys with atomic acceptance receipts. Shared configuration edits require the
previously read version. Standard/expensive admission budgets belong to actors
across their sessions/clients. Allocation, plan processing and exports expose
public operation references with authorised progress and result reads.

Definitive business refusals replay the same outcome for a retained command identity.
Allocation results distinguish unfulfilled demand from technical interruption.
Authorised cancellation stops allocation before another group commit, prevents
unfinished plan publication and cleans incomplete exports; completed effects and
export results retain their normal rules. Ambiguous outcomes are recoverable
across reloads/client restarts and a return the next working day. Equivalent
JSON requests ignore property order/whitespace, preserving omitted/null values
and original preconditions. The recovery guarantee covers the end of that day
using the relevant office calendar, including weekends/closures, with a published
deadline. After expiry, reconcile authorised resources before offering a new
submission; explain inconclusive recovery and require an explicit new attempt.
An outage spanning the deadline extends affected recovery through the end of
the next office working day after restoration, retaining evidence and publishing
the extended deadline.
Cancellation is requested until execution stops, ordered against commits and
publication; earlier commits survive. These behaviours are detailed in the
[request contract](request-behaviour.md).

Remaining design work includes receipt duration and retained expiry evidence,
concrete operation/resource schemas, pagination/input bounds, measured
rates and worker capacity, provider deployment permissions and compatibility-tested
code generation. The [Events–Booking boundary](../../adr/0023-separate-events-from-venue-booking.md)
is agreed; the remaining repository/module layout is an independent architecture
choice. Each unresolved item should identify a requirement, alternatives, tradeoff
and validation criterion before becoming an implementation contract.
