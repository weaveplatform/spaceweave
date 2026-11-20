# Workplace retirement, reactivation and deletion

SpaceWeave manages durable office, building, floor and space definitions used by staff,
administrators, bookings and automation. Retirement stops new use while allowing
existing commitments to be served; deletion is a separate removal operation.
This is the agreed application contract. Endpoint spelling, enum names and
dependency response schemas remain implementation-contract work.

## Consumers and visible outcomes

| Consumer | Required behaviour |
| --- | --- |
| Staff booking portal | Exclude resources from new bookings for use on/after their published retirement time while keeping existing confirmed commitments readable and manageable under normal permissions. |
| Administrator portal | Retire immediately or schedule retirement, inspect commitment impact, and explicitly reactivate an authorised resource after validation; resolve blocking dependencies before deletion. |
| SDK, script and MCP callers | Use the same supported REST retirement, impact and deletion semantics under their actual human/application authority. |
| Terraform | Read retirement as an existing resource state during refresh/import. A dependency conflict is a failed deletion, not absence or completed destruction. |
| Allocation and private workers | Compare requested use with the published lifecycle revision before committing claims; a stale snapshot cannot book a use period excluded by a retirement schedule. |

## Retirement contract

An authorised retirement can take effect immediately or at a scheduled
office-local date and time. Publishing the schedule immediately prevents new
bookings for use on or after that effective point, including requests submitted
before the retirement date. This is a restriction on the requested use period,
not merely the time at which a command reaches the server.

Retirement applies to the selected office, building, floor or space. Retiring a
containing location also stops new-booking eligibility for its
descendant spaces without deleting children or silently rewriting their own
lifecycle state. Existing confirmed bookings remain visible and manageable.

Workplace owns the definition/lifecycle change. Booking remains the only writer
of dated claims and enforces the same effective definition for direct requests,
allocation and other capacity commands. Retirement and claim commitment need a
defined ordering using the existing version/revalidation protocol. Commit checks
compare the requested use period with the published lifecycle revision; an
unlocked precheck or a worker that only changes state at the scheduled time is
insufficient. Impact reads expose affected existing commitments without silently
changing them. Responses and automation reads expose both current state and any
published effective retirement time.

Retirement preserves service of existing commitments. It does not by itself
mean that an event has lost its secured venue; an actual loss continues to use
the [event recovery contract](event-workflows.md). Retirement also does not
silently migrate, cancel or conceal existing bookings.

## Unconfirmed holds and offers

A hold or offer created before retirement was published cannot subsequently
confirm use on or after the effective retirement point. Confirmation and offer
acceptance revalidate the requested use against the current published lifecycle
revision, even when an earlier proposal or pending request was eligible.
Return an explicit unavailable/conflict outcome explaining retirement; an old
hold is not a protected confirmed commitment.

Late Exchange acceptance is retained as a provider observation. Booking refuses
an invalid local confirmation and Exchange reconciles the resulting side effects
through its owning workflow. Neither the callback nor an expiry worker bypasses
the lifecycle check, and the app must not report successful synchronisation while
the external and local outcomes still require reconciliation. Existing confirmed
bookings retain the protection described above.

## Reactivation contract

An authorised administrator can explicitly reactivate a retired resource.
Workplace revalidates its current definitions, containing hierarchy and office
calendar; Booking validates the dated inventory before new eligibility becomes
visible. Reactivation retains the same resource identity, existing commitments
and history. It does not recreate the resource or replay old booking commands.

Reactivation and claim commitment use the same definition-version protocol as
retirement. A still-retired parent continues to exclude descendants even if a
child's own lifecycle state is active. The administrator receives a truthful
validation/conflict outcome when the requested activation cannot take effect.

## Deletion and blocking dependencies

Deletion is a distinct, authorised operation. Unresolved protected bookings,
child resources or other actual blocking dependencies return an explicit
dependency/conflict result. Report impact only within the caller's permitted
resource and data scope. The caller resolves dependencies through their owning
workflows rather than receiving an unreviewed cascade.

Deletion does not implicitly retire the resource, cancel commitments or delete
children. An operation reference cannot disguise an unresolved conflict as
inevitable eventual success. If removal is asynchronous after acceptance, public
status must report actual completion/failure using the normal operation contract.

Historic references, tombstones and physical erasure follow their own access,
consistency and retention requirements. The deletion rule does not mean every
historical reference prevents removal forever. Retirement alone is never proof
of resource absence.

## Validation and remaining policy details

Exercise scheduled retirement published before an affected future booking,
use periods crossing the effective point, pre-existing hold/offer acceptance,
late Exchange acceptance and reconciliation, office-local time conversion,
retirement/reactivation racing a direct booking or allocation commit, retirement of
a parent containing active spaces, continued management of confirmed bookings,
dependency-scoped impact reads, concurrent child/dependency creation, Terraform
refresh/import of retired resources, identity-preserving reactivation, validation
failure on reactivation, and refusal without cancellation/cascade.

Exact resource schemas, status mappings and import identity conventions remain
engineering work. The
[retirement decision](../../adr/0025-retire-workplaces-before-removing-protected-resources.md)
and [request contract](request-behaviour.md) govern all supported clients.
