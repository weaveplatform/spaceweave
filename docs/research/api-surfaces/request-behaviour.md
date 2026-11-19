# REST request contract: retries, concurrency and cost

SpaceWeave coordinates desks, meeting rooms, parking, event tickets and workplace
allocation for one enterprise. Its public business API serves web/mobile-web
portals, MCP servers, SDKs, scripts, tests and Terraform. All consumers need the
same answer to a basic reliability problem: what happened when a request timed
out, overlapped another edit or submitted work that finishes later?

This document defines the application requirements and proposed wire contract
for technical architects and engineers. [Idempotency and concurrency](idempotency-and-concurrency.md)
establishes HTTP and PostgreSQL foundations; [API contract engineering](api-contract-engineering.md)
covers primary-source guidance for specifications, rate admission and client
behaviour. Protocol mechanisms and application policy are identified separately.
The detailed repository/module layout is an independent architecture choice.

## Required application behaviour

| Concern | Requirement | Engineering implication |
| --- | --- | --- |
| Lost responses | Booking creation, ticket issuance and work submission require a retry key. | Commit the acceptance receipt with the business change or durable work submission. |
| Competing configuration edits | Workplace, neighbourhood, role and policy edits require the version previously read. | Reject stale edits for every consumer, including UI and Terraform. |
| Request admission | Publish standard and expensive classes with configurable rate/burst limits. | Each admitted attempt spends one token from its class, including a replay. |
| Budget ownership | A person/application shares its budget across sessions and clients. | UI/MCP share the person's allowance; machine scripts use the application's allowance. |
| Deferred work | Allocation, floor-plan processing and exports expose durable operation references. | Authorised clients can inspect status, meaningful progress, failure and result links. |
| Definitive refusal | Repeating a retained command identity returns its original refusal. | New intentional attempts use new keys; transient technical failures are handled separately. |
| Partial allocation | Unfulfilled capacity demand differs from technical processing failure. | Report placed/unplaced groups and partial progress without erasing committed placements. |
| Cancellation | Authorised callers can request cancellation of long-running work. | Stop at safe boundaries, preserve completed effects and report what completed. |

These are SpaceWeave requirements. They are not guarantees supplied by HTTP,
OpenAPI, PostgreSQL or a client library. No numeric retry-retention value has
been selected. Exact extension names, response schemas and illustrative status
mappings below remain proposed wire conventions.

## Retry identity and atomic acceptance

Use one client-generated identity for one intended command and preserve it across
network retries. A new intentional action uses a new identity. Proposed wire
form: `Idempotency-Key`, scoped to the authenticated actor and operation, with a
fingerprint covering the target, owner, request body and relevant preconditions.
The header is described by an active IETF draft, not a universal HTTP guarantee.
See the [standards analysis](idempotency-and-concurrency.md#what-the-primary-sources-establish).

The fingerprint must include a supplied expected version. It must not change
because a server default or policy version changed after the command was accepted.
Exact-byte request equivalence is a possible initial convention: clients would
retain the original body serialisation. The public contract must state the chosen
scope, equivalence, size limits and mismatch behaviour.

Persist receipt uniqueness, the accepted domain outcome and follow-up job/outbox
records in one bounded PostgreSQL transaction. An office transfer includes its
replacement claims and old releases; required desk/parking claims confirm both
or neither. Commit success before acknowledging acceptance. A lost response then
leaves a recoverable result rather than an unrecorded business mutation.
[Atomic receipt decision](../../adr/0022-commit-retry-receipts-with-command-acceptance.md).

Heavy calculation, drawing conversion, blob transfer and provider calls run
outside that transaction. For deferred work, acceptance atomically creates the
operation/resource, its receipt and the durable job. A retry recovers the same
operation identity without enqueuing another logical operation. Worker delivery
remains duplicate-tolerant independently of the HTTP request key.

The caller, client application and staff booking owner remain separate fields.
MCP uses the person's authority; machine scripts use an approved application
actor. Reusing a token, changing sessions or supplying another staff ID cannot
transfer a retained outcome to a different actor.

## Replay, refusals and current authority

Authenticate and re-authorise each attempt, including access to the original
owner/resource and permitted response fields. A retained result cannot preserve
revoked authority or bypass visitor-data removal. Keep minimal outcome data and
stable resource links; exclude cookies, credentials and expiring signed URLs.

Replay the accepted outcome with safe response metadata. Proposed fields include
`Idempotency-Replayed`, `Location` and the validator for the returned representation.
Recompute request identifiers and rate state for the current attempt. The receipt
is historical: a room request originally returned as pending may now be accepted
or declined. Its resource read provides the current business state.

Check a completed matching receipt before applying a fresh resource-version
precondition. A successful edit's retry still supplies its original version;
its own committed update must not make that retry fail. Changing the supplied
version under the same identity is a different command fingerprint.

A definitive business refusal, such as insufficient capacity, remains that
command's outcome while its receipt is retained. The same key does not later
become a successful booking when capacity changes. A deliberate retry of the
business intent uses a new key. Timeouts, temporary dependencies and admission
failures must not be mistaken for permanent business refusals.

Domain constraints protect all commands independently of retry keys: one
overlapping desk per person, required parking, event capacity, one admission per
ticket and owner-only desk check-in. Repeated check-in/admission can use the
existing booking/ticket transition as its natural identity.

## Retention and ambiguous outcomes

Retention is a reliability/privacy tradeoff, not a duration prescribed by HTTP.
Specify the supported automatic retry period, delayed recovery after an ambiguous
response and the maximum lifetime of nonterminal operations. Derive a published
receipt-retention policy from those requirements and validate it against storage,
privacy and operational recovery needs. No initial duration is selected.

Retaining a receipt does not hold a transaction open, reserve capacity or extend
a booking/hold. The receipt is a small record of an outcome. Operation identities
must remain usable during nonterminal execution and the documented recovery
period. Personal-data deletion rules apply to receipts and derived copies.

The expiry contract remains open: clients must know when replay is guaranteed,
how to reconcile after expiry and when a new action is safe. After deleting all
identity evidence, a server cannot reliably distinguish an expired key from one
it never received. Do not claim that distinction without an explicit mechanism.

## Concurrent configuration edits

Require the previously read version for shared workplace, neighbourhood, role
and policy changes. A stale write must not overwrite another administrator's or
Terraform apply's change. This check is separate from the command's retry key.

Proposed wire form: a strong ETag on the read and `If-Match` on the edit, with
412 for mismatch and 428 where the precondition is required. Generated clients
and UI forms carry the version; command handlers enforce it atomically. Native
SCIM and provider preconditions retain their own documented semantics.
[HTTP validators and preconditions](https://www.rfc-editor.org/rfc/rfc9110.html#section-13).

## Cost classes, budgets and backpressure

A standard request is a bounded interactive read or ordinary mutation. An
expensive request initiates or performs broader computation, such as allocation,
conversion or export. Classification is explicit per operation; GET alone does
not establish that an operation is cheap. These classes control admission and
are unrelated to monetary charging or free event-ticket pricing.

Each admitted HTTP attempt spends one token in its class, including a replay or
a handler failure. Balances follow stable person/application identity across
sessions and clients. Record client provenance for diagnosis. All HTTP replicas
share admission state so replica scaling does not multiply a caller's allowance.
Provider/provisioning/visitor protocols need their own appropriate admission
profiles instead of inheriting a staff session identity.

Bound date spans, selected resources, pages, batches, file complexity and output
volume. Separately bound outstanding work, worker concurrency, attempts and
execution time. One expensive submission cannot imply unlimited queued work.
Protect core availability, booking and check-in from heavy-work load. Establish
numeric rates and bounds through representative contention and capacity tests.
[Admission and worker controls](api-contract-engineering.md).

Publish response budget state and retry guidance with precise units. Any
`X-RateLimit-*` fields are project-specific conventions; they must define whether
reset means a delay or timestamp. Do not present unavailable admission storage
as an empty budget. Exact header names remain a wire-design choice.

## Operation status and cancellation

A deferred command returns a stable operation reference after durable acceptance.
The proposed HTTP mapping is 202 plus a status location; a locally created room
request can return 201 with its explicit pending state. Neither response proves
that allocation succeeded or Exchange accepted the room.
[HTTP 201/202 semantics](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3).

Separate execution state, business result and provider synchronisation state:

| Situation | Required result meaning |
| --- | --- |
| Allocation completes with sufficient capacity for some groups | Completed planning with placed/unplaced counts and permitted reasons for unfulfilled demand. |
| Technical failure interrupts processing | Failure/interruption with accurate partial progress and preserved committed groups. |
| Cancellation arrives after some commits | Stop further work at safe boundaries and report completed effects and remaining work. |
| Room request is accepted locally | Pending local request/hold until Exchange's acceptance transition. |
| Export generation completes | Authorised result link, with download access and expiry evaluated separately. |

Allocation cancellation requires allocation-management authority. Other
cancellation calls require the owning workflow's permissions. Cancellation does
not silently undo bookings, tickets or external effects already committed;
normal authorised business commands handle later changes.

Public monitors return bounded status/progress, safe errors and result links.
Queue payloads, SQL failures, provider credentials and worker repair controls
remain private. Define terminal states, progress freshness and per-workflow
cancellation boundaries in the concrete operation schemas.

## Proposed error and retry profile

| Situation | Illustrative response | Client behaviour |
| --- | --- | --- |
| Required key missing/invalid | 400 problem | Correct the request. |
| Same key with changed intent | 422 key-reuse problem | Recover the original request or intentionally start a new command. |
| Matching command still executing | 409 in-progress problem plus retry guidance | Retry the same identity within the supported period. |
| Recorded definitive refusal | Original refusal | A new business attempt requires a new key. |
| Stale configuration version | 412 precondition problem | Read current state and decide the next edit. |
| Actor's class budget exhausted | 429 plus retry guidance | Wait and retain the command identity. |
| Temporary acceptance/dependency overload | 503 where applicable | Back off and reconcile ambiguous acceptance. |

Use distinct machine-readable problem types. SDKs must not retry every 409 as
though it means work is in progress. Bound automatic retries, honour server
hints and add jitter; preserve the original key/body/preconditions after a
transport failure. Apply native response rules to SCIM and provider callbacks.

## Implementation and validation across candidate layouts

| Candidate | Atomic acceptance owner |
| --- | --- |
| Layered | Owning application service and its persistence transaction. |
| Hexagonal | Core command and transaction port, implemented by a storage adapter. |
| Clean | Owning interactor and unit of work, including coupled Booking mutations. |
| DDD | Owning application command/coordinator with explicit local coordination. |
| Modular monolith | Owning module and named cross-module transaction participants where required. |

Every implementation must demonstrate simultaneous identical commands, response
loss after commit, fingerprint mismatch, stale edits, partial group placement,
refusal replay, revoked access, visitor-data removal, cancellation during commits,
shared budgets across replicas and saturation of heavy-work queues. Test clients
through supported public contracts as well as real PostgreSQL concurrency paths.

Remaining design items are retention/expiry policy, exact operation and response
schemas, fingerprint equivalence, rate-header conventions and load-validated
numeric limits. These are explicit engineering decisions supported by the
requirements and primary-source research, rather than unexplained defaults.
