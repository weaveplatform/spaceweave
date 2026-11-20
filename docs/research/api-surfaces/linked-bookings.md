# Linked desk and parking booking lifecycle

Staff can associate desk and parking bookings for an attendance day. A request
marked both required confirms both or neither; an optional link permits each
resource to succeed independently. Booking owns both kinds of claims, their
availability and their transitions in every repository architecture candidate.
The accepted cancellation rule distinguishes an explicit instruction from an
automatic desk release.

## Consumers and outcomes

| Consumer | Required contract |
| --- | --- |
| Staff/mobile-web portal | Show the linked bookings and the effect of a cancellation. Before either booking starts, cancelling either part of a both-required pair cancels both atomically. |
| Authorised delegates, SDKs, scripts and user-context MCP | Use the same public cancellation command and actual actor/owner authority; do not implement pair cancellation as two client-coordinated requests. |
| Booking workers | Automatic desk no-show or vacancy release returns desk capacity only; linked parking remains booked until its own end or explicit release. |
| Waitlist and availability consumers | Treat capacity as available only after its owning claim actually ends. A released desk does not make its linked bay or parking-pool capacity available. |
| Attendance/reporting | Consume committed outcomes for each resource without inferring a car's departure from desk-use evidence. |

## Explicit cancellation before either booking starts

The server resolves the pair from either referenced booking, checks current
authority for the full proposed outcome, and revalidates both claims, the link
kind and their start times. Before either starts, a both-required cancellation
ends both claims in one Booking transaction, including its retry result and
committed follow-up facts. Failure cannot leave a half-cancelled pair.

For an optional link, cancelling one booking does not implicitly cancel the
other. The response identifies the affected claims and the remaining booking
state so that all clients can explain the actual outcome.

Pair cancellation, check-in, transfers, expiry and waitlist promotion use the
same current-state and resource/pool guard protocol. A stale page or queued
command cannot establish that both bookings are still before their start times.
This pre-start agreement does not determine the explicit cancellation policy
after either booking has started; that remains a separate product decision.

## Automatic desk release

Desk no-show and post-check-in vacancy are distinct automatic triggers. Both
release only the desk, even for a pair originally marked both required. Parking
continues until its own booking end or an explicit release. The original
both-required acquisition rule does not establish a permanent automatic
cancellation cascade.

A person may still have a car in the parking area after losing or leaving a
desk. Desk check-in, anonymous occupancy, missing sensor data and sustained desk
vacancy therefore cannot establish that parking capacity is free. Retain the
parking claim in bay/pool availability calculations and offer only genuinely
released capacity to waitlisted demand. Normal parking-end and explicit-release
commands continue to enforce current authority and claim state.

## Placement and validation

Extend each candidate's existing Booking cancellation/release code, request-set
model, claim interfaces and owned store facets. This requirement does not add a
new owner, service or repository package. Durable outcomes distinguish explicit
pair cancellation from automatic desk-only release; notifications follow commit
and cannot decide which capacity is returned.

Validate cancellation initiated from either booking, rollback of a failed paired
transition, optional independent cancellation, retry after response loss,
cancellation racing a start/check-in or transfer, specific-bay and pooled
parking, and both automatic desk-release paths retaining parking capacity.
Allocation, direct booking and waitlist callers must observe the same results.
See the [decision](../../adr/0032-separate-linked-cancellation-from-automatic-desk-release.md),
[request contract](request-behaviour.md) and [policy record](../../policy-defaults.md#linked-desk-and-parking-release).
