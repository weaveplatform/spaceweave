# Engineering the public API contract and controlling request load

Research checked: 19 November 2026. Audience: technical architects and engineers
joining SpaceWeave. **Source facts** below describe published specifications and
maintainer documentation; **SpaceWeave decisions** describe the intended product;
**implementation proposals** and **unresolved values** are labelled explicitly.
This is documentation research: no generators, runtime tests or load benchmarks
were run, and no production admission settings were inspected.

## Scope and the contract boundary

**SpaceWeave context.** One enterprise, approximately 100,000 staff and 60,000
desks, uses React staff/admin portals and mobile web. Go/Chi serves the application;
PostgreSQL owns durable state, jobs and outbox records. Deployment uses one active
write region, multi-zone availability and warm regional recovery. Azure Blob or
Google Cloud Storage holds revisioned assets. These are project constraints, not
claims that any particular deployment has demonstrated the target capacity.
[Architecture context](../../architecture-blueprint.md).

**SpaceWeave decision.** UI, user-context MCP, Go/TypeScript SDKs, scripts, tests
and Terraform share the supported public REST contract. Public means supported
for those consumers; access remains authenticated and authorised. Private Go
interfaces and durable jobs serve internal consumers. A private HTTP/RPC protocol
requires an actual remote consumer and its own compatibility requirements.
[Shared REST boundary](../../adr/0020-share-public-rest-contracts-across-ui-and-automation.md).

Human MCP requests retain the person's authority and request budget. Machine
applications authenticate independently and use their application authority.
Record **actor**, **requesting client** and **business owner** separately: a
delegate may create a booking for another staff member without becoming its owner.
Neither an SDK nor a generated operation grants additional permissions.
[Authority model](../../adr/0021-separate-human-and-application-authority.md),
[owner and delegate](../../adr/0008-separate-booking-ownership-from-the-acting-delegate.md).

## OpenAPI version and generator compatibility

**Source facts.** The latest published OpenAPI specification checked is **3.2.1**,
dated 10 September 2026. The specification version describes the document format;
it does not establish the application's public API major version. A unique
`operationId`, when supplied, identifies an operation within the description.
[OpenAPI 3.2.1](https://spec.openapis.org/oas/v3.2.1.html#operation-object).

`oapi-codegen` v2.8.0 introduced initial OpenAPI 3.1 support; its release notes
require Go 1.25 and runtime v1.6.0 or newer for that release. The current release
listing identifies v2.8.2, including subsequent generation security fixes.
These release facts do not establish compatibility of every schema feature.
[3.1 release notes](https://github.com/oapi-codegen/oapi-codegen/releases/tag/v2.8.0),
[release register](https://github.com/oapi-codegen/oapi-codegen/releases).

The current generator documentation supports Chi and strict server generation,
Go clients and models. It describes OpenAPI 3.0/3.1 support and specific 3.2 HTTP
method features; this is not evidence of complete OpenAPI 3.2 compatibility.
Strict interfaces constrain handler request/response types but do not provide
complete incoming-request validation. Chi uses the separate `nethttp-middleware`.
[Generator capabilities](https://github.com/oapi-codegen/oapi-codegen),
[validation middleware](https://github.com/oapi-codegen/nethttp-middleware).

**Implementation proposal.** Evaluate a pinned OpenAPI 3.1.2 toolchain first,
with 3.0.4 as a compatibility fallback. Do not select 3.2.1 merely because it is
newest. Pin Go, generator, runtime, validator and TypeScript tooling together;
prove required features with small generated-and-compiled fixtures before choosing
the format. Nullability, unions, additional properties, headers, multipart input
and date/time representation belong in those fixtures. No pin is selected here.

OpenAPI Generator's `typescript-fetch` is a documented TypeScript client option.
Its feature table and configuration expose limitations, including composition
support and an additional-properties compatibility switch; generation must be
verified against SpaceWeave's actual schema. This research does not choose it
over another TypeScript generator. Browser sessions, application credentials,
retry policy and operation polling still need explicit client integration.
[TypeScript Fetch generator](https://openapi-generator.tech/docs/generators/typescript-fetch/).

## Publish one reviewable contract

**Source facts.** OpenAPI permits descriptions split across referenced documents,
and `x-` specification extensions at defined locations. Components have no effect
until referenced by the description. Extensions supply application metadata;
OpenAPI itself does not execute their authorisation or admission meaning.
[OpenAPI 3.1.2](https://spec.openapis.org/oas/v3.1.2.html#specification-extensions).

**Implementation proposal.** Maintain editable fragments, assemble a deterministic
consumer bundle, then generate server adapters and supported clients from that
bundle. Publish the supported bundle with its API release and migration material.
Keep internal paths, credentials, raw jobs and unpublished operations out of it.
Generated files should identify their inputs and pinned tool versions.

Give every supported operation a stable identifier, business owner and explicit
runtime contract. Renaming an identifier can change generated SDK methods even
when the HTTP path is unchanged; treat that as a compatibility review item.
Keep public resource and operation IDs stable across retries, process restarts
and generated-client releases; database or queue placement is not their identity.

| Proposed operation metadata | What the implementation must establish |
| --- | --- |
| Identity profile and audience | Correct authenticator for human, application, visitor or native integration. |
| Capability and resource resolver | Current role/grant checks, owner/delegation scope and authorised projection. |
| Retry mode and precondition | Required command identity, matching rules and mandatory read version where applicable. |
| `standard` or `expensive` | Exactly one declared request class; no silent class for an omitted annotation. |
| Input/output limits | Date/resource/page/batch/file bounds and documented refusal behaviour. |
| Outcome and completion model | Typed business errors, operation monitor and result semantics. |

Use a small validated metadata schema and one operation registry shared by
contract checks and runtime registration. Validate declared capabilities and
resource locations against their catalogues. An annotation alone cannot check
whether a staff member can access a particular booking, attendance report or
floor-plan revision. Native SCIM and provider callbacks keep their own profiles.
[Protocol boundaries](protocol-contracts.md), [authentication profiles](client-authentication.md).

## Verify source, generated code and served behaviour separately

**Implementation proposal.** Reject duplicate method/path pairs, operation IDs,
unresolved references, invalid metadata and examples that contradict schemas.
Regeneration should produce the same bundle and code; compare supported prior
contracts as well as the current source. Ensure client generation includes every
supported business operation, including any adapter implemented by hand.

Chi exposes `Walk` for traversing a router tree. Use that capability to compare
the assembled routes with the intended operation registry, with explicit native
protocol and probe exceptions. A route match proves registration, not correct
authorisation or business behaviour.
[Chi route traversal](https://pkg.go.dev/github.com/go-chi/chi/v5#Walk).

Track distinct evidence: described, generated, registered, implemented and
acceptance-tested. A generated placeholder returning `501` can satisfy an
interface while providing none of the advertised business behaviour. Production
publication needs a deliberate implementation check, not a count of generated
methods. This is a proposed verification rule, not a finding about current handlers.

Contract tests should exercise requests through the real router and assert
response status, media type, headers, schema, permissions and domain effects.
Include malformed/oversized input, unauthorised object access, stale updates,
simultaneous keyed commands, response loss after commit and queue saturation.
Static schema checks and generated Go types do not establish response-schema
parity; the generator documents no general response-validation middleware.
[Validation limits](https://github.com/oapi-codegen/oapi-codegen#requestresponse-validation-middleware).

The middleware documents preserving the validated body for strict-server binding.
Include a fixture proving the handler sees that same value: a second permissive
JSON decode can otherwise interpret member names differently from validation.
[Validated body handoff](https://github.com/oapi-codegen/nethttp-middleware).

## Compatibility and lifecycle

**SpaceWeave decision.** API support evolves independently of UI deployments.
The initial retirement policy provides at least **180 days' notice** once a
replacement and migration guidance are available, and honours published support
dates. This is a project support policy, not an HTTP or OpenAPI requirement.
[Public version policy](../../adr/0020-share-public-rest-contracts-across-ui-and-automation.md).

Review behaviour as well as shape: required inputs, tighter bounds, error types,
ordering/cursors, authorisation projections and asynchronous transitions affect
clients. Even an added enum member can break an exhaustive client. Compare prior
supported SDK fixtures and consumer journeys; do not assume that a rolling UI
release upgrades scripts, MCP hosts, Terraform or already-open browser sessions.
These are proposed compatibility checks for this shared-consumer model.

RFC 9745 defines `Deprecation` and its documentation link relation. RFC 8594
defines `Sunset`, an Informational RFC, to signal likely withdrawal time.
Neither chooses a minimum notice period. Signal lifecycle dates consistently
with the published SpaceWeave policy and migration guide.
[Deprecation](https://www.rfc-editor.org/rfc/rfc9745.html),
[Sunset](https://www.rfc-editor.org/rfc/rfc8594.html).

## Request tokens, identity and bounded work

**Source fact.** Go's `x/time/rate` implements an initially full token bucket
with refill rate and burst capacity. `Allow` consumes one token when possible
and otherwise refuses; `Wait` can block. Its limiter supports concurrent
goroutines, which does not make separate process instances share a balance.
[Go rate package](https://pkg.go.dev/golang.org/x/time/rate).

**SpaceWeave decision.** Publish `standard` and `expensive` request classes.
Each admitted HTTP attempt spends **one token in its selected class**, including
a completed replay or a later handler failure. `expensive` uses a separate
bucket; it is not a multiplier of standard tokens, CPU seconds or monetary cost.
Admission prevents excess attempts; keyed commands prevent duplicate effects.
[Request behaviour](request-behaviour.md).

The authenticated key is stable **actor kind + actor ID + request class**.
Sessions, SDKs and MCP do not create extra balances for the same person.
Machine applications have their own configurable balances. Preserve client
provenance for diagnosis without adding per-client quotas, billing tiers or a
customer tenancy hierarchy. Integration and unauthenticated entry profiles need
their own relevant admission rules; do not manufacture staff actors for them.

**Implementation proposal.** Set a small early body-size guard before decoding;
authenticate and perform applicable early authority checks before admission.
After admission, check retry receipts and validate/execute the command. Later
validation and handler refusals retain the spent token. Return bucket
state only when the admission store supplied it; early refusals do not imply an
empty balance.

OWASP identifies missing payload, batch, page and execution limits as resource
consumption risks. SpaceWeave therefore needs hard operation bounds alongside
its two request classes; one admitted call must not expand into unbounded work.
[OWASP API4](https://owasp.org/API-Security/editions/2023/en/0xa4-unrestricted-resource-consumption/).

| SpaceWeave workload | Proposed independent bounds |
| --- | --- |
| Availability/search | Date span, offices/resources, filters and result page. |
| Booking/ticket creation | Named owner/attendee scope, interval and supported recurrence/batch size. |
| Drawing processing | Upload bytes, pages, pixels/entities and generated output size. |
| Allocation | Admitted cohort scope, outstanding runs, worker concurrency and publication workload; no quality-reducing solver-time cutoff or invented group-size limit. |
| Reports/exports | Date/scope/volume, outstanding exports and authorised result size. |
| Operation reads | Response size, bounded polling and retention-aware lookup. |

No numeric rate, burst, page/window, queue or concurrency value is selected here.
Rate admission does not reserve worker execution. Bound backlog, active jobs,
per-kind concurrency, execution time and retries separately; decline unsafe new
acceptance with a typed overload outcome. Allocation interruptions remain
provisional; execution controls must not publish a lesser result as the completed
plan merely because a compute budget expired. Keep core availability, booking and
check-in responsive while allocation, tracing and export work runs.

## Share admission across replicas

**Implementation proposal.** All API replicas consult one logical admission
state. Independent full in-memory buckets would grant an actor another burst
on each replica; this follows from the Go limiter's instance-local state.
PostgreSQL is an existing implementation candidate, not a reason to add a
separate service or assume unlimited debit throughput.

PostgreSQL row locks coordinate conflicting updates, and `INSERT ... ON CONFLICT
DO UPDATE` provides an atomic insert-or-update outcome. These primitives can
support a single bounded refill-and-debit operation per actor/class. Design its
clock, concurrent initial creation, row cleanup, lock timeout and configuration
changes explicitly; do not use a read followed by an unlocked write.
[Row locks](https://www.postgresql.org/docs/18/explicit-locking.html),
[atomic upsert](https://www.postgresql.org/docs/18/sql-insert.html#SQL-ON-CONFLICT).

Proposed failure policy: when shared admission state is unavailable, return a
typed temporary `503`; do not invent a `429` balance or grant each replica a fresh
bucket. Select limiter-state durability/reset behaviour separately from durable
command receipts. Test database failover and recovery against that policy.

HPA scales replicas using resource, custom or external metrics; it does not
coordinate actor budgets or increase PostgreSQL capacity. Readiness removes
unready pods from Service traffic; liveness/startup failures can restart them.
Choose probes for those purposes, keeping job progress and provider lag visible
as workflow signals. Do not restart healthy API processes for every queued job.
[HPA](https://kubernetes.io/docs/concepts/workloads/autoscaling/horizontal-pod-autoscale/),
[probe behaviour](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/).

## Backpressure, problems and client retries

**Source facts.** RFC 6585 defines `429 Too Many Requests` and permits
`Retry-After`; it does not prescribe how to identify callers or count requests.
RFC 9110 permits an HTTP-date or non-negative delay in seconds for `Retry-After`.
[429](https://www.rfc-editor.org/rfc/rfc6585.html#section-4),
[retry delay](https://www.rfc-editor.org/rfc/rfc9110.html#section-10.2.3).

The checked `RateLimit-Policy`/`RateLimit` work remains an active Internet-Draft,
revision 11. It is not a published RFC contract. Any `X-RateLimit-*` fields below
are project-specific fields with precisely documented units, not standard HTTP
rate-limit headers. Recheck draft status before adopting its wire syntax.
[Rate-limit draft status](https://datatracker.ietf.org/doc/draft-ietf-httpapi-ratelimit-headers/).

| Proposed response field | Exact proposed meaning |
| --- | --- |
| `X-RateLimit-Limit` | Burst capacity of the selected class bucket. |
| `X-RateLimit-Remaining` | Whole tokens left after this attempt's decision. |
| `X-RateLimit-Reset` | Rounded-up seconds until full; not an epoch timestamp. |
| `Retry-After` on `429` | Rounded-up seconds until one token is expected to be available. |

Declare headers on applicable responses so SDKs expose them. Concurrent callers
may consume a token first; a delay is guidance, not a capacity reservation.
Use a distinct temporary-unavailable problem for admission-store/queue failure.
Exact custom names and successful-response exposure remain wire-design choices.

RFC 9457 defines `application/problem+json` and a problem-type URI as the primary
machine identifier. Human `detail` is not a parsing contract. Publish a stable
problem register and safe field issues; avoid raw SQL, provider payloads or secret
debugging data. Distinguish throttling, an in-flight command and a stable capacity
refusal even where two responses share the same HTTP status.
[Problem details](https://www.rfc-editor.org/rfc/rfc9457.html).

Amazon's published retry guidance describes load amplification, bounded backoff
and jitter to spread correlated retries. Apply that pattern with one explicit
retry owner, an attempt/elapsed-time budget and bounded operation polling.
Honour a valid server hint without retrying earlier; if it exceeds the client's
remaining deadline, stop or defer. Retry by operation/problem semantics, not
every `409` or `5xx`. Timeouts do not prove a command failed to commit.
[Retry guidance](https://aws.amazon.com/builders-library/timeouts-retries-and-backoff-with-jitter/).

## Durable acceptance and public operation status

**SpaceWeave decisions.** Booking, ticket and work creation require command keys
and receipts atomic with acceptance. Configuration edits require the previously
read version. Stable business refusals replay under the same key; a deliberate
new attempt has a new key. Transient technical failures are not permanent
business refusals. Receipt retention is configurable and **numerically unset**.
[Retry and concurrency contract](request-behaviour.md).

PostgreSQL transactions provide an all-or-nothing boundary. SpaceWeave applies
that boundary to the receipt, local mutation or operation record, and durable
job/outbox submission. Network/provider calls run after commit. Isolation and
capacity constraints still need explicit design; an idempotency key alone cannot
prevent two different commands claiming the same desk or last ticket.
[Transactions](https://www.postgresql.org/docs/18/tutorial-transactions.html),
[isolation](https://www.postgresql.org/docs/18/transaction-iso.html).

HTTP `202` means accepted processing has not completed. Microsoft's asynchronous
request-reply pattern illustrates a status URL and polling hints; it is a vendor
pattern rather than a mandatory status schema. SpaceWeave's durable public
operation references cover allocation, tracing and exports, with authorised
progress, failures and result links. Keep queue payloads, leases and repair
controls private. A room-request resource can exist while Exchange acceptance
is still pending.
[202 semantics](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.3),
[asynchronous pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/asynchronous-request-reply).

A completed allocation may contain unfulfilled demand without being a processing
failure. Report published placements and safe unplaced reasons separately from
technical failure before publication, which confirms no partial groups. Allocation
status separates grouping/packing progress from the completed published result.
Authorised cancellation fences publication; if publication wins, preserve its
completed result. Other workflows retain their own safe boundaries and effects.
Allocation cancellation requires allocation-management authority. These are
SpaceWeave outcomes, not guarantees supplied by a queue or HTTP status code.

## Proposed artifacts in every architecture candidate

```text
api/src/public/                 editable requests, responses and operation metadata
api/public.openapi.yaml         deterministic supported consumer bundle
api/generation/                 pinned server and Go/TypeScript client configuration
api/profiles/                   native integrations and private operational profiles
docs/data/api-operations.json   owner, support state and verification evidence
docs/data/problems.json         stable problem types and their retry meaning
```

These are logical artifacts, not a selected package tree or new service split.
Layered places adaptation in transport/application; Hexagonal in driving adapters
and core ports; Clean in interface adapters/use cases; DDD in context transports
and application commands; Modular Monolith in public transport and owning-module
interfaces. In all five, the owner controls atomic acceptance and private jobs.
Generated Go placement follows the selected layout; business policy stays out of
generated adapters and clients.
[Architecture alternatives](../architecture-options/README.md).

Before claiming capacity, measure representative peaks, hot offices, Terraform
refresh bursts, UI/MCP sharing, concurrent retries, failover, queue saturation
and drain time. Check one-second p95 initial responses separately from operation
completion and the 99.95% monthly core availability target. Rates, bursts,
input bounds, retry/receipt windows and worker limits remain implementation
values to establish from that evidence. This note supplies no measured capacity.
[Project targets](../../architecture-blueprint.md).
