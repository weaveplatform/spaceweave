# Idempotency, concurrency and durable command outcomes

Research baseline: **19 November 2026**. Audience: architects and engineers
designing SpaceWeave's public REST API and PostgreSQL command processing.
This is a researched design note, not evidence of implemented endpoints.
**Source facts**, **SpaceWeave decisions** and **implementation proposals** are
distinguished below; a cited implementation pattern is not a universal rule.

SpaceWeave serves one organisation across multiple offices, with a planning
scale of approximately 100,000 staff and 60,000 desks. UI, SDKs, MCP, scripts and
Terraform consume shared public business semantics. PostgreSQL owns durable
booking state, jobs and outbox records; external provider effects follow commit.
These are application constraints from the [scale decision](../../adr/0005-design-for-one-organisation-across-many-offices.md),
the [REST contract decision](../../adr/0020-share-public-rest-contracts-across-ui-and-automation.md)
and [persistence decision](../../adr/0011-own-booking-state-and-durable-work-in-postgresql.md),
not throughput claims established by the sources below.

## What the primary sources establish

| Source | Established fact | Design implication |
| --- | --- | --- |
| [HTTP semantics, RFC 9110 §9.2.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2) | Idempotency concerns the intended effect of repeated requests; responses may differ. PUT, DELETE and safe methods are idempotent. | Method semantics alone do not recover a booking command's original receipt. |
| [RFC 9110 §8.8.3](https://www.rfc-editor.org/rfc/rfc9110.html#section-8.8.3), [§13.1.1](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.1) | ETags identify representations; If-Match uses strong comparison and prevents a false precondition from applying the change. | An expected version protects an edit against concurrent modification. |
| [RFC 9110 §15.3.2–3](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.2) | 201 reports resource creation; 202 reports accepted, unfinished processing and should describe or link a status monitor. | Creation and workflow completion need separate meanings. |
| [Problem Details, RFC 9457](https://www.rfc-editor.org/rfc/rfc9457.html#section-3) | Defines machine-readable error details, including problem types and extensions. | Clients can distinguish key reuse, stale state and business capacity conflicts without parsing prose. |
| [PostgreSQL transactions](https://www.postgresql.org/docs/current/tutorial-transactions.html) | A transaction groups changes atomically; uncommitted intermediate changes are not visible to concurrent transactions. | Store command acceptance and its recovery receipt together. |
| [PostgreSQL constraints](https://www.postgresql.org/docs/current/ddl-constraints.html#DDL-CONSTRAINTS-UNIQUE-CONSTRAINTS) | Unique constraints cover columns or combinations; exclusion constraints support more general conflicts. Cross-row CHECK constraints cannot guarantee consistency. | Use database-enforced identities and overlap rules where representable, with explicit coordination for remaining invariants. |
| [PostgreSQL isolation](https://www.postgresql.org/docs/current/transaction-iso.html) | Read Committed gives statement snapshots; Serializable can abort conflicting transactions and requires retrying the whole transaction. | A read-then-insert availability check needs a deliberate concurrency strategy. |

**Standards status:** the IETF's [Idempotency-Key document](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/)
is an **active Internet-Draft**, revision **08**, published 22 October 2026 and
expiring 25 April 2027. It is work in progress, not a published RFC. Its
[draft text](https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header-08#section-2)
defines a Structured Field string, payload consistency, optional fingerprints,
published expiry policy and example error responses. Adoption requires an
explicit API profile; the header's familiar name does not establish universal
syntax, storage, retention or response-replay guarantees.

## Three independent controls

**SpaceWeave decision:** booking creation, ticket issuance and background-work
submission require a retry key. The accepted outcome commits in the same
transaction as the business change or durable submission. See the
[atomic receipt decision](../../adr/0022-commit-retry-receipts-with-command-acceptance.md).

| Control | Question it answers | What it does not replace |
| --- | --- | --- |
| Keyed command receipt | Has this actor's same command already reached a durable outcome? | Capacity checks or current authorisation. |
| Expected resource version | Is this edit based on the configuration the caller read? | Deduplication of a lost-response retry. |
| Domain invariant | Can this booking, ticket or state transition exist alongside other committed state? | Recovery of the first command's identity and result. |

Two different keys can still request the same scarce desk. Database/domain
rules must prevent both from obtaining it. One person cannot obtain overlapping
desk allocations by changing key, client or office. These rules also survive
receipt expiry, worker redelivery and application restarts.

Natural idempotency uses the business identity and state transition: repeated
personal check-in for the same assigned booking reports that check-in already
occurred, without another attendance fact. Event admission and visit arrival
likewise need their own distinct transition identities. A keyed creation command
instead recovers the same booking, ticket or operation outcome. Neither property
implies byte-identical responses or exactly one execution attempt.

## Atomic acceptance and the crash gap

**Engineering inference:** saving an HTTP response after a separate business
commit leaves an unsafe interval: the booking can commit, the process can die,
and a retry can find no completed receipt. Middleware cannot close that interval
unless it participates in the owning command's database transaction.

**Implementation proposal:** keep the following work in one short transaction:

1. Establish exclusive processing for the actor/key identity and verify that
   the command matches any existing record.
2. Evaluate current authority, expected state and domain constraints under the
   selected concurrency strategy.
3. Commit the business change, or durable operation acceptance, together with
   its receipt and any required outbox/job records.
4. Respond only after commit. On an ambiguous commit response, reconnect and
   resolve the same identity before attempting new work.

The durable receipt can be small: command identity, request fingerprint,
outcome kind, created resource/operation identifiers and the accepted revision
or stable refusal. Raw response retention is a separate storage/privacy choice.
A durable operation record can also carry its acceptance receipt; a second
parallel record is not intrinsically necessary.

An uncommitted claim must not become a completed receipt. A rolled-back command
has no accepted booking or work submission. Concurrent same-key attempts must
converge on one durable outcome across API replicas; bounded waiting or an
explicit outstanding response are wire-level design choices. A request-local
memory cache cannot coordinate those replicas.

**SpaceWeave decision:** a definitive business refusal is an outcome of that
command. During the published receipt-retention window, the same key returns
the same refusal even if capacity subsequently becomes available. A deliberate
new attempt uses a new key and undergoes current domain checks.

Record a refusal only after the business decision has been established without
partial committed effects. Technical failures, dependency timeouts, database
deadlocks and serialization aborts are not permanent business refusals. They
may require retry or outcome reconciliation with the original key. An HTTP 5xx
or disconnected client alone does not establish whether acceptance committed.
The detailed transaction/savepoint strategy for retaining refusals remains open.

## Concurrent edits and retry ordering

**SpaceWeave decision:** shared workplace, neighbourhood, role and policy edits
must supply the version previously read. UI and automation obey the same rule;
a stale edit cannot overwrite intervening changes. Runtime policy publication
retains its [separate revision model](../../adr/0016-version-runtime-policies-separately-from-deployment-config.md).

**Implementation proposal:** expose a strong ETag on the relevant read and use
If-Match for edits. Check the supplied version in the write predicate or under
equivalent transactional protection; a prior comparison followed by an
unconditional update still races. If-Match `*` only establishes existence and
does not satisfy this application's requirement for a previously read version.
Use 412 for a stale supplied version; 428 for a missing mandatory precondition
is a proposed mapping supported by [RFC 6585 §3](https://www.rfc-editor.org/rfc/rfc6585.html#section-3).

For commands combining keys and expected versions, proposed processing is:

1. Authenticate and validate current authority to invoke/read this command.
2. Resolve the key and compare the complete command identity, including its
   expected version. A changed expected version is a changed command.
3. Recover an existing permitted outcome without applying the mutation again.
4. For a new command only, evaluate the version and other domain constraints
   atomically with the attempted change.

This recovers a successful edit after a lost response even when the resource
has since advanced. The receipt describes the accepted revision; an authorised
resource read supplies current state. The selected HTTP representation must
preserve that distinction, rather than presenting an old body/ETag as current.
[RFC 9110 §13.1.1](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.1)
permits successful recovery where a conditional change already succeeded; it
does not define SpaceWeave's key lookup or fingerprint protocol.

**SpaceWeave decision:** compare validated, client-supplied JSON values, ignoring
property order and insignificant whitespace. Preserve omitted versus explicit
`null`; changed values, target, owner or expected version change the command.
Later server-default or policy changes must not change the accepted fingerprint.
This selects equivalence behaviour, not a particular canonicalisation library.

**Open:** key namespace, length, randomness requirements, relevant headers and
versioned fingerprint encoding. The identity must distinguish actor, command,
authorised owner/context and relevant preconditions. Preserve the actor, client
and booking owner as separate values; a fingerprint is not an authorisation
mechanism.

## Domain transactions and external effects

**SpaceWeave decisions:** Booking owns local availability. A desk plus required
parking commits together; an office transfer acquires replacement claims and
releases previous claims atomically. A failed transfer preserves the existing
allocation. An allocation run first forms collaborator groups, including
singletons, then packs them into existing seats. Intermediate placements remain
proposals; publish the completed, validated plan together with required parking
and its durable outcome, preserving older published bookings. The
[two-phase decision](../../adr/0037-form-collaborator-groups-before-packing-desk-allocations.md)
refines the [allocation-cycle contract](../../adr/0014-use-explicit-allocation-cycles-and-atomic-group-placement.md).

**Implementation proposal:** combine uniqueness/exclusion constraints with
consistent lock ordering or serializable transactions where multi-row rules
require them. Select the strategy per invariant and verify contention at office
and allocation peaks. Retrying a transaction must rerun its decision logic;
it must not repeat provider calls inside the transaction.

PostgreSQL's [locking documentation](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-DEADLOCKS)
warns against keeping transactions open for long periods. A retained receipt is
stored data, not a database lock or a booking hold. Worker leases, room-hold
expiry and receipt retention are separate lifetimes. If workers use claim
takeover, completion must verify current claim ownership so a late predecessor
cannot overwrite a successor's state.

AWS's [transactional outbox guidance](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html)
explains atomic state/outbox writes and duplicate-message handling. Applied
here, durable outbox work closes the lost-follow-up gap, but workers still need
deduplication and reconciliation after external timeouts or crashes. This does
not select AWS messaging infrastructure or promise exactly-once provider calls.

Microsoft Graph documents an event [transactionId](https://learn.microsoft.com/en-us/graph/api/resources/event?view=graph-rest-1.0#properties)
for avoiding redundant create-event POSTs on client retry; it cannot be changed
after creation. **Proposal:** retain a stable provider correlation per outbound
intent and reconcile ambiguous outcomes. That property does not establish an
unlimited deduplication window or atomicity with PostgreSQL.

A room request remains pending until Exchange accepts it, per the
[room acceptance decision](../../adr/0006-keep-room-requests-pending-until-exchange-accepts.md).
Replaying local acceptance does not confirm the room, repeat the external
intent, or extend a hold. Provider acceptance, decline and timeout remain
explicit later domain transitions.

## Durable operations and cancellation

**SpaceWeave decision:** allocation, drawing processing and exports return a
durable public operation reference with authorised status, progress, failure
and result reads. Public operation identity is separate from private job claims
and execution attempts. A retry of submission recovers the same acceptance.

**Implementation proposal:** return 202 with an operation link for genuinely
deferred commands. Return 201 with the created resource link when creating a
local pending room request is itself complete. Neither response asserts later
provider confirmation or successful allocation. Replayed acceptance stays
stable; current operation/resource reads show subsequent progress.

An allocation run that completes planning but cannot place every group reports
a published completed plan with unfulfilled demand, placement counts and safe
unplaced reasons. A technical interruption before publication reports failure
and provisional progress without confirming any groups from that run. Older
published bookings remain protected. A capacity shortfall must not be disguised
as a transient server failure that invites blind resubmission of the whole run.

**SpaceWeave decision:** authorised cancellation stops additional work at safe
boundaries and preserves committed placements and other completed effects.
Allocation cancellation requires allocation-management authority. Report both
the cancellation outcome and effects already committed. Cancellation is a
durable request, not proof that an in-flight provider call was undone.
Allocation cancellation fences completed-plan publication: cancellation winning
prevents publication; publication winning remains completed. Staged placements
are not committed booking effects.
Floor-plan processing stops cooperatively and cannot publish an unfinished
revision. Exports stop generation and clean incomplete output; completed results
retain their normal access/expiry. These boundaries constrain the implementation.
A cancellation acknowledgement means requested until execution actually stops.
Serialize its durable decision against commit/publication: preserve earlier
commits and prevent the next one when cancellation wins. A completed export
remains completed. **Open:** cancellation endpoints, concrete state names and
progress/response schemas; they must preserve this ordering.

## Replay authorisation, privacy and retention

**SpaceWeave decision:** revalidate current authority on every attempt, including
replay. MCP acts only in its initiating user's context; machine scripts use an
independently authorised application. App-only actors cannot perform personal
check-in. See the [authority decision](../../adr/0021-separate-human-and-application-authority.md).
A key is neither a bearer credential nor a grant to replay another actor's data.

Stable command outcomes do not guarantee disclosure after access is revoked.
An unauthorised retry receives the current access refusal without executing
the command again. Authorised recovery should return a permitted receipt or
resource link; retained payloads must not bypass changed field/resource access.
Avoid storing credentials, reusable admission secrets or unnecessary personal
data in a general response cache. Reissue sensitive access material through
its own currently authorised protocol where needed.

**Recovery purpose is agreed; numerical retention is unselected.** Recover
ambiguous outcomes across reloads/client restarts and a return the next working
day before inviting another submission. Derive a published horizon from that
promise, bounded supported retries, outage recovery and maximum supported
nonterminal operation lifetime. For booking/ticket commands, use the relevant
office calendar through the end of its next working day, including weekends
and closures, and publish the guaranteed deadline for clients.
An outage spanning that deadline extends affected recovery through the end of
the next office working day after service returns. Preserve the evidence and
publish the extension; booking periods/capacity keep their normal rules.
Choose when terminal retention starts and ensure an accepted operation's
identity remains recoverable throughout its supported processing/retry horizon.
This does not require holding a transaction, physical lock or booking capacity
for that period.

Minimum outcome receipts can outlive bulky response bodies where justified,
subject to privacy/deletion rules and access controls. Resource history, audit,
operation results and deduplication receipts need explicit, potentially
different retention purposes; no unlimited raw-response archive is implied.
Booking and ticket visibility does not end when their original retry receipt
expires; the resources have their own access and retention contracts.

**SpaceWeave decision:** after replay expiry, reconcile authorised bookings,
tickets or operations before offering another submission. Never automatically
replace an unknown outcome with a new command key. If recovery is inconclusive,
explain that uncertainty and require an explicit new attempt.

**Open:** retained expiry evidence and exact wire representation of the recovery
deadline/expiry. After purge, the server cannot reconstruct an old refusal
from an absent record or distinguish it from every never-seen key without
additional information. Do not claim the stable-refusal guarantee beyond the
published window. Document how clients reconcile known resource/operation
identities before deliberately submitting new intent after that window.

## Implementation boundaries and evidence required

In Go/chi architecture candidates, transport validates wire metadata and passes
actor, command identity and preconditions to the owning application command.
That command controls the PostgreSQL acceptance transaction. Layered services,
hexagonal ports, clean use cases and domain modules can all implement this
boundary; shared receipt infrastructure does not move capacity ownership into
generic HTTP middleware. No repository layout is selected by this mechanism.

Proposed problem types distinguish missing/invalid keys, changed-command reuse,
outstanding work, stale version, capacity refusal and temporary technical
failure. Exact status codes, replay headers, operation paths and OpenAPI
extension names remain contract-design choices. Publish actual operation and
response declarations so SDKs preserve keys and preconditions consistently.

Meaningful verification uses real PostgreSQL transactions and multiple API or
worker instances where coordination is claimed:

- Lose the response immediately after acceptance commits; recover identical
  resource/operation identity without another business effect or job.
- Race same-key commands, mismatched commands and different keys competing for
  the same desk/person/parking; prove all applicable invariants.
- Replay a definitive refusal after capacity changes; require a new key for a
  new attempt. Retry technical transaction failures without retaining refusal.
- Race versioned edits; reject the stale one. Change only the expected version
  under an existing key and reject command mismatch.
- Revoke delegation or application grants before replay; deny disclosure while
  preserving the committed outcome. Repeat personal check-in without another
  attendance fact and reject an app-only actor.
- Crash workers around provider calls, duplicate delivery and lease takeover;
  reconcile without claiming exactly-once external execution.
- Race cancellation with allocation publication and provider work; preserve
  published effects, distinguish completed-plan unmet demand from provisional
  technical failure, and confirm no partial groups from interrupted planning.
- Lose publication responses, replace a stale worker and replay publication;
  recover the same result without duplicate claims or premature notifications.
- Exercise the chosen retention boundary, privacy deletion and recovery after
  expiry once their contract is specified; measure contention at expected peaks.
