# Workplace retirement and deletion

SpaceWeave manages durable office, building, floor and space definitions used by staff,
administrators, bookings and automation. Retirement stops new use while allowing
existing commitments to be served; deletion is a separate removal operation.
This is the agreed application contract. Endpoint spelling, enum names and
dependency response schemas remain implementation-contract work.

## Consumers and visible outcomes

| Consumer | Required behaviour |
| --- | --- |
| Staff booking portal | Exclude retired resources from new-booking eligibility while keeping existing confirmed commitments readable and manageable under normal permissions. |
| Administrator portal | Explicitly retire an authorised resource; inspect permitted blocking dependencies and resolve them before requesting deletion. |
| SDK, script and MCP callers | Use the same supported REST retirement, impact and deletion semantics under their actual human/application authority. |
| Terraform | Read retirement as an existing resource state during refresh/import. A dependency conflict is a failed deletion, not absence or completed destruction. |
| Allocation and private workers | Revalidate effective definitions before committing claims; a stale snapshot cannot create new bookings after retirement takes effect. |

## Retirement contract

An authorised retirement stops new bookings for the selected office, floor or
space. Retiring a containing location also stops new-booking eligibility for its
descendant spaces without deleting children or silently rewriting their own
lifecycle state. Existing confirmed bookings remain visible and manageable.

Workplace owns the definition/lifecycle change. Booking remains the only writer
of dated claims and enforces the same effective definition for direct requests,
allocation and other capacity commands. Retirement and claim commitment need a
defined ordering using the existing version/revalidation protocol; an unlocked
precheck is insufficient.

Retirement preserves service of existing commitments. It does not by itself
mean that an event has lost its secured venue; an actual loss continues to use
the [event recovery contract](event-workflows.md). Retirement also does not
silently migrate, cancel or conceal existing bookings.

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

Exercise retirement racing a direct booking or allocation commit, retirement of
a parent containing active spaces, continued management of confirmed bookings,
dependency-scoped impact reads, concurrent child/dependency creation, Terraform
refresh/import of retired resources, and refusal without cancellation/cascade.

Retirement scheduling/reactivation and treatment of unconfirmed holds/offers
remain separate policy refinements; they are not supplied by the protected
confirmed-booking decision. Exact resource schemas, status mappings and import
identity conventions remain engineering work. The
[retirement decision](../../adr/0025-retire-workplaces-before-removing-protected-resources.md)
and [request contract](request-behaviour.md) govern all supported clients.
