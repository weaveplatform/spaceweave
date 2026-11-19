# spaceweave

SpaceWeave is a space booking application for organisation members, referred to
as staff. It helps staff secure suitable spaces and work near the people they
need to collaborate with, while administrators manage capacity, visitors,
events and attendance reporting across offices.

## Product scope

The intended product covers:

- Booking of desks, meeting rooms, car parking and event spaces; "reservation"
  means the same thing as "booking".
- Advance desk attendance intents, allocated together on a cadence to favour
  team proximity and fit groups onto floors.
- Staff-chosen attendance days, including travel from a usual office to another
  work office for workshops or other activities.
- Office attendance reporting, desk check-in, occupancy signals and release of
  booked desks that are unused.
- Configurable neighbourhoods bundling desks and optional meeting rooms, with
  groups assigned to neighbourhoods and sizes that can flex with demand.
- Cross-neighbourhood collaboration for staff with different assignments.
- Multiple offices, each containing buildings, with multiple floors per building
  and a floor plan for each floor.
- One organisation with many offices/sites, targeting 100,000 staff and 60,000
  desks across its locations.
- On-behalf-of booking, including personal assistants booking for other staff.
- Visitors (also called guests) with separate visit records; aged visit records
  are removed after a retention period configured by the organisation's admins.
- Free, capacity-limited event ticketing for staff-only events and events
  admitting external visitors.
- An admin portal separate from the end-user space-booking portal, with each
  administrator's scope configurable across one or more offices.
- Staff authentication through their organisation's Entra ID single sign-on.
- Meeting-room calendar integration with Microsoft Exchange Online.
- Web and mobile web applications.
- Supported public REST contracts for the UI, MCP servers, SDKs, scripts,
  testing and Terraform; private interfaces for platform workers.
- Internet-accessible user-facing APIs protected by Conditional Access rules
  and policies, with application capabilities, office scopes and delegation.
- Containerised hosting on the organisation's Kubernetes infrastructure, with
  application-health observability.
- A 99.95% monthly availability target for core booking and check-in.
- A one-second p95 initial-response target for core availability, booking and
  check-in requests at the agreed capacity-test workload.
- PostgreSQL for durable application state, with module-owned tables and
  migrations; blob storage for drawings and generated assets, supporting Azure
  Blob Storage and Google Cloud Storage.

Agreed domain terms are recorded in [GLOSSARY.md](GLOSSARY.md).

### Desk attendance and allocation

Staff choose the future days they intend to attend the office and want a desk,
within a configurable advance planning window. The initial lookahead is 28 days,
with a configurable cutoff 14 days before the cycle start date.
The advance planning window and allocation cadence are configured separately.
Attendance intents are collected and processed together on that cadence. The
allocation process uses staff-chosen attendance dates, considers proximity to
team members, forms groups separately for each attendance date, and assigns
groups to neighbourhoods across available floors. Teams may be split across
floors when that allows more people to receive desks. Other space kinds have
booking rules suited to each kind.

Requests may specify mandatory space features, such as desk setup requirements
or EV charging for parking. Required features determine eligibility; preferences
guide selection among eligible spaces.

Team membership will ultimately come from external data sources. Teams are
separate from the allocation groups formed within SpaceWeave to assign people
to desks. Externally supplied team membership is read-only in SpaceWeave, with
corrections made in the source system. Directory integration uses Entra and
Workday, with explicit staff/team mappings and stable internal staff identifiers
linked to source identities. Entra owns sign-in and account security; Workday
owns workforce lifecycle, reporting structure and usual office. SpaceWeave owns
booking preferences and dated travel choices. Correlate records through a unique,
organisation-approved identifier, retain both source identities and hold
ambiguous matches for review rather than matching by email alone.

Confirmed, effective disable or leaver events revoke staff access when observed;
temporary feed outages preserve the last verified state of known staff. Apply
office/team changes on their effective dates to unconfirmed planning defaults,
preserving explicit travel choices and confirmed bookings, and notify owners
where existing arrangements need attention. Release leavers' future personal
desk/parking bookings automatically; route rooms and events for assisted
ownership-handover review. Provisioning distinguishes deactivation from deletion;
SpaceWeave exposes an authenticated SCIM 2.0 interface for Entra user and mapped
group provisioning, including creation, updates, deactivation, reactivation and
deletion. Workday remains a separate integration with the agreed field ownership.

A successful allocation run automatically creates confirmed bookings; staff are
notified and can cancel. Staff may directly
book spare desks after the date's main allocation run, while existing confirmed
bookings are protected. Before that run, ordinary requests enter intent collection.
Unmet desk demand enters a waitlist, with affected staff notified and unmet
demand visible to administrators. Eligible waitlisted requests get newly
available desks before that capacity is offered for fresh direct bookings.
Same-day waitlist promotion uses a time-limited offer requiring acceptance; if
declined or expired, the desk is offered to the next eligible person.

Attendance intents request a desk for a full working day. Desks released during
the day can be booked for the remaining working hours. Each office has its own
timezone, working hours and closure calendar, used for bookings, check-in
deadlines and reporting.

Several groups can share a neighbourhood by taking their office turns on
different days. Divisions have two recurring anchor days for in-person
collaboration; those days are organisational context only. Staff choose their
attendance days. The product supports staff attending their usual work office
and travelling to other work offices for workshops or other activities. Intent
defaults to the usual office, with dated exceptions for travel.

Neighbourhoods belong to one building and may span floors within it. Their
configured bundles can be adjusted for particular dates by moving specific,
unallocated desks between
bundles, preserving existing bookings. Staff can request another neighbourhood
for particular days; their desk demand is counted in that neighbourhood for those
days. Collaboration requests identify the people the requester needs to sit near,
as well as the requested neighbourhood, to guide physical placement. Assigned
groups receive priority access to the neighbourhood's meeting rooms, with
remaining availability open to other staff through a configurable room-booking
policy. Explicit collaboration priority can override general team proximity;
the staff member deliberately sets that priority on their request.
Among placement plans accommodating the same number of people, prioritise
explicit collaboration, then general team proximity, then previously unfulfilled
demand.

### Attendance and desk use

Attendance is a reporting feature. Desk check-in and occupancy signals support
reporting office attendance and identifying booked desks that remain unused so
they can be released for others. Reports distinguish bookings, identified
check-ins and occupancy detections. Signals distinguish the person, particular
desk or office-level presence they establish; anonymous desk occupancy does not
identify the person attending.

A desk is **allocated** when the system assigns it and **taken** when the assigned
staff member checks in. Check-in is available through the booking or by scanning
a QR code at the desk. Occupancy evidence is separate from the check-in that
marks a desk taken.

Check-in works without occupancy hardware. The organisation may add occupancy
integrations, with sensor-assisted vacancy release used where signals are
available.

An assignee who does not check in within the configurable grace period loses
their allocation as a no-show, even if anonymous occupancy is detected. After
check-in, a sensor outage preserves the taken desk until the booking ends, the
member releases it, or fresh vacancy evidence satisfies the separately
configured vacancy release period, which accommodates temporary breaks.

A desk released as a no-show returns directly to the pool even if occupancy is
still detected. Unbooked physical occupancy cannot withhold bookable capacity.

Staff see their own attendance history. Administrators with reporting permission
see individual and aggregate reports within their assigned offices. The
organisation configures retention independently for attendance records, raw
occupancy evidence and visitor visit records.

Staff can see relevant teammates' and explicit collaborators' planned office
days and allocated desks within their shared office context. Identified check-in
and attendance evidence remains limited to self and authorised reporting or
operational access; public projections account for presence inference.

### Other spaces and events

Parking supports bookings for a specific bay and capacity-only bookings within
a parking area. Staff can link desk and parking bookings for the same attendance
day and manage them together, while each retains its own availability and rules.
They may mark a linked request as both required, so neither booking is confirmed
unless both can be secured. Otherwise, their availability is handled independently.

Meeting rooms use start/end times, with staff finding suitable rooms by capacity
and facilities. Their calendar integration is with Microsoft Exchange Online:
connected room bookings participate in conflict checks and SpaceWeave bookings
are reflected in Exchange Online. Synchronisation is bidirectional: bookings,
changes and cancellations made in either system update the other.
App-originated room requests remain pending with an expiring local hold until
the Exchange room accepts; they are then confirmed, declined or expired according
to the outcome.
Synchronisation automatically merges compatible edits, discards stale updates
and revalidates room/time changes. People resolve genuinely incompatible
intentions; ordinary reconciliation proceeds automatically.
Exchange processing uses a dedicated backend identity with scoped mailbox
access, notifications and durable delta reconciliation on a five-minute cadence
while healthy. Progress and actual synchronisation freshness are monitored.

An event has a venue booking securing its event space and separate free tickets
controlling attendee capacity. The organiser chooses whether its audience is
staff-only or also admits external visitors, and whether visitor registration
is invitation-only or public. The ticket limit cannot exceed the booked venue's
capacity; organisers may set a lower limit. An external attendee's ticket links
to their visitor visit record, with visit arrival and event admission recorded
separately. Each ticket identifies one attendee and is scanned at event entry to
record admission and prevent duplicate use.

### Visitors and administration

Staff invite external visitors with a named staff host and a visit window.
External event access supports visitors attending internal events, including
invited guest speakers, through the scoped invitation/verification flow.
Reception or administrators record arrival and departure. Each administrator's
administrative scope covers a configurable set of one or more offices within
their organisation. Access combines that scope with configurable capabilities
such as space configuration, visitor reception, event management and reporting.
Operational policies use organisation defaults with permitted office overrides;
identity and retention settings remain organisation-wide.

Organisation-wide policies, identity mappings and integration administration
require explicitly granted global capabilities beyond an administrator's office
scope.

Operational policies are versioned database records editable through
administration, separate from deployment configuration. Changes apply to new
cycles/bookings by default; existing confirmations retain their policy version
unless explicitly migrated. All values in the [initial defaults](docs/policy-defaults.md)
are configurable.
Staff authenticate through their organisation's Entra ID single sign-on.
External visitors access registration, invitations and tickets through verified
email links, with restricted access associated with their visits and events.

Booking outcomes, waitlist offers, visitor invitations, event tickets and
handover tasks use email and in-app notifications, addressing owners and relevant
acting delegates with configurable delivery preferences. An individual may opt
in to a private Microsoft Teams message that a booking was raised. This
permission applies to that message; it does not enable other Teams notifications
or shared-channel announcements.

Booking supports delegates acting on behalf of other staff, including personal
assistants. The staff member being booked for owns the booking and is its Exchange
organiser; the acting person is recorded separately. Exchange-derived delegation
informs room-booking authority and can be inferred for other space kinds. Users
can additionally name delegates within SpaceWeave. App-managed delegation
authorises app-mediated bookings, including rooms, while native Outlook delegate
permissions remain unchanged. The resulting room bookings, edits and cancellations
still synchronise bidirectionally with Exchange Online. Initial inference uses
explicit Exchange calendar-delegate grants; other permissions require deliberate
mappings that preserve their source, owner and authorised actions.
Revoking an Exchange-derived grant removes that grant; independently approved
app-managed grants retain their own lifecycle.

### Application state and background work

Staff/mobile-web and admin portals use separate React/TypeScript entrypoints
sharing UI components and the existing floor-plan renderer. Go manages
authentication sessions, keeping Entra tokens server-side and using secure,
HttpOnly session cookies with backend authorisation and CSRF protection.
The application REST APIs are described using OpenAPI, with generated
Go/TypeScript transport types and clients kept separate from domain interfaces.
The UI consumes the same supported public REST contracts as SDKs, scripts,
MCP servers, test clients and Terraform. Platform workers use private application
interfaces; public client type does not itself grant additional authority.
Delegated human identities and independently authorised application identities
remain distinct, with actor, client and staff booking owner retained separately.
App-only credentials cannot perform personal desk check-in.
SpaceWeave RBAC is configurable. MCP uses the initiating user's context;
machine-identity scripts use a separate application-authority channel. The
consumer type does not impose a common authentication journey.
Existing authorised global administration manages roles and scoped assignments,
including configured SCIM-backed group mappings. Roles compose grants within
their complete scopes; clients can discover their own effective capabilities,
and commands revalidate current authority.

Booking creation, ticket issuance and background-work submissions require retry
keys; accepted outcomes commit with the business change or work submission.
Shared configuration edits require the previously read version and reject stale
writes. Standard/expensive request classes have configurable rate/burst limits;
each admitted attempt spends one token, including replays. Budgets follow the
person/application across sessions and clients. Allocation, floor-plan processing
and exports expose durable operation references with authorised progress and
result reads. The [request contract](docs/research/api-surfaces/request-behaviour.md)
records these decisions; remaining wire details, lifecycle policies and numeric
limits are proposed for review.

Terraform initially manages long-lived workplace and permitted policy/configuration
resources, with useful read-only data sources. Personal bookings, attendance
intent, check-in and tickets are outside initial declarative management scope.
Public API compatibility is managed independently of UI releases; breaking
changes require a new major version and a migration/deprecation policy.
The initial retirement notice is 180 days once replacement/migration guidance
is available; future notice policies honour already published support dates.
Authorised UI/API edits to Terraform-managed configuration are allowed and
reported as drift; normal version, permission and domain checks still apply.
First-party MCP server and Terraform provider implementations follow the core
platform and agreed Go/TypeScript SDKs.

The Booking module owns local space availability and booking-state transitions.
Other modules use its interface. PostgreSQL holds durable application state;
each module owns its tables and migrations. Source drawings and generated assets
live in blob storage supporting Azure and GCP, with metadata and identifiers in
PostgreSQL. Both providers implement one blob interface; deployment configuration
selects the active provider. Drawings and approved renders use immutable revisions
with explicit current-version pointers, preserving referenced historical versions.

Workplace owns offices, buildings, floors, space definitions and baseline
neighbourhoods. Booking owns dated capacity and temporary neighbourhood
adjustments. Allocation calculates placements from a versioned snapshot outside
long transactions; Booking revalidates and commits bounded batches while
preserving confirmed allocations.
Each office defines allocation cycles with attendance dates, an intent cutoff
and processing time; late changes are handled incrementally. Each allocation
group placement commits atomically, including required linked parking. Changed
capacity triggers replanning of affected uncommitted groups.

A staff member has only one overlapping desk allocation. An office move secures
replacement capacity and releases the previous allocation transactionally,
including required linked parking. Desk check-in is personal to the assigned
member; delegates manage bookings.

Booking changes and their background-work records commit in the same transaction.
External calls run after commit; durable jobs and a transactional outbox let
retry-safe workers resume committed work after restarts. Interactive requests
have a one-second p95 initial-response target at the agreed test workload;
allocation completion and Exchange room acceptance have separate workflow targets.
The initial worker system uses PostgreSQL-backed durable jobs and outbox
processing, with leased claims, retry backoff and operator-visible failures.
Production PostgreSQL is a managed platform service operated separately from
application workloads, with its own availability, backup and restore plan.

One versioned application image supports independently scaled Kubernetes roles
for HTTP, allocation, Exchange synchronisation, tracing and maintenance. Runtime
roles use separately scoped identities, workload federation where supported and
the organisation's secret manager for remaining credentials. OpenTelemetry
traces/metrics and structured JSON logs flow through a Collector to the
organisation's observability backend.

Use one active write region with multi-zone high availability and a
preprovisioned warm recovery region. Fence the previous writer before failover
and exercise recovery of the database, assets, identity, routing and workers. Regional
disaster-recovery targets are at most five minutes of data loss and service
restoration within fifteen minutes; routine pod/node failures preserve
acknowledged bookings. End-to-end recovery must be validated against the platform.
If recovery replication exceeds the five-minute loss budget while the primary
remains healthy, continue serving and raise a critical alert. Failover with loss
beyond that budget requires an explicit operator decision; the exposure can
temporarily exceed the RPO target.
Recovery protection covers committed business records and assets acknowledged as
durable or published. Verify asset recovery copies before acknowledging durability;
unfinished uploads and processing remain retryable.

During a confirmed application outage preventing check-in or waitlist-offer
acceptance, pause affected no-show releases and offer expiries. Allow a
configurable recovery grace period when service returns.

Exchange-derived authority must have been verified within the configurable
fifteen-minute limit before authorising new delegated actions. Stale authority
is refreshed first; app-managed grants are checked against current app state.

This record describes product purpose and agreed functional scope.
[Architecture and policy design](docs/architecture.md) covers integration
adapters, synchronisation conflict handling, confirmation timing and operational
policy defaults. The agreed decisions cover
[desk allocation](docs/adr/0001-allocate-advance-desk-intents-together.md),
[neighbourhoods](docs/adr/0002-model-neighbourhoods-as-flexible-space-bundles.md),
[desk check-in](docs/adr/0003-require-assignee-check-in-to-mark-a-desk-taken.md)
and [Exchange synchronisation](docs/adr/0004-synchronise-room-bookings-bidirectionally-with-exchange-online.md).

## Floor rendering

Its floor-rendering component turns floor-plan images into interactive isometric
models. The microsite is in
[`web/floorplan-iso/`](web/floorplan-iso/README.md). `go run ./cmd/spaceweave serve`
serves it with a drop zone: upload a plan image, or a DXF, DWG or SVG drawing, and
it is traced into the site as a draft for you to approve or send back.

Tracing is deterministic and needs no API key: the same file gives the same
plan every time, and nothing leaves the machine. `internal/autotrace` reads a
DXF's or DWG's entities (wall layers, door blocks, room text, exact units) and an
image's pixels (wall bands, openings, door arcs), and `internal/floorgraph`
joins the walls into a graph and finds the rooms. Set `tracer: claude` to have
a Claude agent trace instead.

What each layer of a drawing holds comes from layer profiles
(`internal/autotrace/profile`): built-in ones for Uniclass 2015 codes, AIA/NCS
names, facilities-management exports and plain words, under a client's own
`profiles/<name>.yaml` in the output folder (`profile:` in the config). A
draft's Layers panel shows each layer's role and where it came from; changing
one saves it in the client profile and traces the plan again.

Drawings are rendered in Go by `internal/convert` (with `internal/dxf`,
`internal/dwg`, `internal/svgdraw`, `internal/cad` and `internal/vector`). No external
converter is used. [`NOTICE`](NOTICE) lists the third-party material that
ships in the binary, and what was consulted while writing the readers.

The rest of this README comes from the weaveplatform template.

## weaveplatform-template

The starting point for weaveplatform Go repositories: a blocking quality gate
(lint, unit tests on Linux, macOS and Windows, govulncheck, cross-compile, and a
≥95% merged coverage gate), release-please through the organisation's App,
dependencies that track their latest release from the day a repository is born,
and conventional-commit PR titles.

## After creating a repository from this template

1. **Rename the module.** `make init NAME=<repository name>` rewrites
   `weaveplatform-template` everywhere (module path, this README) and tidies.
2. **Set the owners** in `.github/CODEOWNERS`.
3. **Protect `main`.** Require the `Quality gate` and `✅ Validate PR Title` checks,
   and enable auto-merge so Dependabot updates merge once the gate passes.
4. **Releases** start at 0.1.0 and need nothing per repository: the organisation's release-please
   App is installed on all repositories and its credentials are organisation
   variables and secrets. The workflow warns in its log if it falls back to a
   `RELEASE_PLEASE_PAT`.
5. **Replace this README** with the project's own.

## Dependencies stay at latest

| Piece | What it moves | When |
|---|---|---|
| `deps-refresh.yml` | The `go` directive (latest stable Go), every module and every `go.mod` tool (`go get -u ./... tool`), and `.github/golangci-lint-version` | On the push that creates `main` (repository birth), daily, and on demand |
| Dependabot | Go modules and GitHub Actions pins, majors included, one grouped PR per ecosystem | Daily |
| `auto-merge.yml` | Merges either kind of PR once the quality gate has passed on that exact commit | When the gate finishes |

The gate is the only thing that holds an update back. A major that breaks the
build fails it, and the PR stays open for a person. Dependabot owns the action
pins because changing workflow files needs a permission the org App does not
have.

This repository itself refreshes the same way, so a repository created from it
starts from a current snapshot and updates again within minutes of creation.

## Layout

| Path | What |
|---|---|
| `cmd/<binary>/` | Entry points: thin cobra shims over `internal/`; excluded from the coverage gate |
| `internal/` | Project-only packages; `internal/buildinfo` carries the version stamped in by `make build` |
| `pkg/` | Public, importable packages |
| `docs/` | Design notes and decision records |
| `test/acceptance/` | Acceptance tests against real dependencies (excluded from `make test`) |

## Make targets

| Target | What |
|---|---|
| `make gate` | Everything CI runs: vet, lint, test, acceptance, cover, vuln, build |
| `make test` | Unit tests with `-race -shuffle=on`, coverage to `cover/unit` |
| `make acceptance` | Builds `spaceweave` with coverage and drives it, with no API key and against a fake Claude API, using the real node validator; coverage to `cover/acceptance` |
| `make eval` | Prints the automatic tracer's accuracy on plan images drawn from the hand traces |
| `make cover` | Merges every `cover/*` directory and enforces `.testcoverage.yml` |
| `make lint` / `make fmt` | golangci-lint with `.golangci.yml` |
| `make build` | Cross-compiles every `cmd/*` binary for six platforms, CGO disabled |

Acceptance tests write their coverage to another `cover/<name>` directory; the
coverage job merges whatever it downloads, so adding one is a new CI job that
uploads a `cover-<name>` artifact plus an entry in the gate's `needs`.
