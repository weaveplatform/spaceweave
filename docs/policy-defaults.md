# Operational policy defaults

Status: agreed initial defaults, 19 November 2026. Every value below is
configurable, including weekday, local time, period length and duration values.
Organisation defaults and permitted office overrides apply. Office dates and
times use the office calendar and timezone.

| Policy | Initial default | Meaning |
| --- | --- | --- |
| Intent lookahead | 28 calendar days | Future attendance dates staff can request. |
| Allocation cycle | 7 days, starting Monday | Attendance dates planned as one cycle; closed dates remain unavailable. |
| Intent cutoff | 14 calendar days before the cycle start date, at 17:00 local | Deadline for that cycle's main cohort. |
| Main allocation run | 30 minutes after cutoff | Plan and commit the cohort; handle later changes incrementally. |
| Desk no-show grace | 60 minutes after booking start | Assignee must personally check in before expiry. |
| Post-check-in vacancy | 30 minutes | Require fresh, sustained vacancy evidence; missing signals mean unknown. |
| Same-day desk waitlist offer | 10 minutes | Acceptance deadline before offering desk capacity to the next eligible person. |
| Application room-request provisional hold | 15 minutes | Local coordination bound, distinct from native acceptance/check-in policy. Native pending/acceptance outcomes remain authoritative; approval-based workflows require explicit supported coordination. |

The delegation verification limit of fifteen minutes is already agreed and
configurable. Retention periods remain explicitly configured by the organisation
for attendance, occupancy evidence and visitor visits.

Operational policies are versioned database records editable through
administration, separate from deployment configuration. Apply changes to new
cycles/bookings by default; existing confirmations retain their policy version
unless explicitly migrated. Validate individual values and cross-field
consistency before publishing a policy.

## Public API version retirement

The agreed initial notice is at least 180 days before withdrawing a supported
public major version, once replacement and migration guidance are available.
This is an organisation-wide API compatibility policy, not an office override.
Configure future notice periods explicitly and honour already published support
dates. Public version compatibility is independent of UI releases.

## REST admission and retry policy

Configure rate/burst limits for standard and expensive
request classes, with one token per admitted attempt, including replays.
Authenticated balances follow the person/application across sessions and
clients. Bound inputs and worker execution separately; establish numerical
limits through representative load validation.

No numerical initial rate/burst or retry-retention value is adopted yet.
Booking/ticket recovery covers the end of the next working day in the relevant
office calendar, including weekends/closures, with a published deadline.
Post-expiry reconciliation precedes another submission; inconclusive recovery
requires explained uncertainty and an explicit new attempt. An outage spanning
the published deadline extends affected recovery through the end of the next
office working day after restoration; preserve the evidence and publish the
extended deadline. Booking periods and capacity keep their normal rules. The
[request contract](research/api-surfaces/request-behaviour.md) records that
behaviour and the remaining receipt-duration/expiry-evidence decisions.

## Event ticket policies

Events may enable an attendee waitlist. Released seats go to eligible waitlisted
attendees in join order before fresh registrations. Time-limited offers hold
their seats exclusively and recheck current eligibility on acceptance. Decline or
expiry ends that request; explicit rejoining creates a new one. Offers expire no
later than registration closes; only an explicit organiser extension permits
later acceptance. The initial numeric offer duration remains unselected; the
desk waitlist's ten-minute default does not define an event offer duration.

Office/time changes require attendee reconfirmation and linked visitor host/window
validation. The organiser publishes a reconfirmation deadline and notifies affected
attendees; unconfirmed tickets release seats at that deadline, following the
enabled event waitlist policy. Initial notice periods and reconfirmation
recovery-grace defaults remain unselected. A confirmed application outage pauses
affected reconfirmation deadlines and preserves seats; publish a recovery grace
period within the event's usable attendance window.

Venue recovery preserves existing exclusive offers and pauses their expiry
clocks. Resume acceptance when the venue is secured and registration remains
open, with revised deadlines bounded by its cutoff. Office/time changes instead
withdraw old unaccepted offers, preserving requests/queue positions for new
offers under the updated details. Temporary queue ineligibility preserves the
request's original position without holding capacity while it is skipped.
The [event workflow contract](research/api-surfaces/event-workflows.md) records
ticket cancellation, visitor eligibility and linked-visit lifecycle behaviour.

## Outage protection

Shared operational facilities retain confirmed outage declarations describing
affected offices, actions and times. Booking and Events own the deadline effects;
Policy owns the configured rules. The [incident contract](research/api-surfaces/operational-incidents.md)
defines declaration custody and ordering against expiry, including recovery
extensions for uncertain requests.

During a confirmed application outage preventing desk check-in or acceptance of
waitlist offers, pause the affected no-show releases and offer expiries. Resume
with a configurable recovery grace period. No numerical initial grace value has
been agreed; configure it explicitly before enabling automatic expiry. These
protections do not extend a booking beyond its actual usable period.
For event offers, outage grace also respects the registration cutoff; later
acceptance requires an explicit organiser extension of registration.

## Linked desk and parking release

Before either booking starts, explicit cancellation of either component of a
both-required desk/parking pair cancels both atomically. Optional links permit
independent cancellation. Revalidate the pair, current state, authority and start
times when the command commits; a stale page cannot establish eligibility.

Automatic desk no-show or vacancy release returns desk capacity only. Linked
parking remains booked until its own end or explicit release, including in
specific-bay and pooled-capacity calculations. Desk evidence cannot establish
that the vehicle has left. Acquisition/transfer atomicity remains unchanged.
The [linked-booking contract](research/api-surfaces/linked-bookings.md) records
consumer outcomes and validation. Once either booking starts, cancellation
affects only the selected booking unless both are explicitly chosen; the other
remains until its own end or explicit cancellation. Revalidation across the
first-start boundary must preserve the requested cancellation scope.

## Meeting-room recovery

When an ordinary meeting's replacement request fails after the previous room is
lost, retain the meeting and attendees with an explicit no-secured-room outcome.
Notify the organiser and offer available replacements or explicit cancellation.
A replacement remains pending until current room acceptance and Booking capacity
validation succeed. Events-managed venues retain their own recovery workflow.
The [room recovery contract](research/api-surfaces/room-recovery.md) separates
product outcomes from provider compensation mechanics requiring validation.

## Native room policy and leavers

Exchange's supported room policy, check-in eligibility and autorelease govern
both Outlook and SpaceWeave room requests, including one-offs and series.
Read/manage supported native configuration through authorised provider paths;
show desired, pending and effective values truthfully. There is no separate
SpaceWeave room-expiry timer, grace default or incident-based native deadline
extension. Desk and Event policy defaults remain application-owned.

Recurring-only per-person admission and cancellation after consecutive misses
remain unverified native capability gaps; do not enable custom enforcement to
supply them. Exact native invitation/series release effects and feature
availability require validation. The [provider research](research/api-surfaces/outlook-room-policy-enforcement.md)
records supported settings/APIs and boundaries.

A confirmed effective workforce departure cancels all remaining room meetings and
series owned by that organiser. Other-owner meetings created by a departing PA
remain. Disable, SCIM deletion or incomplete imports do not invent departure.
Events retains its own handover. The [recurring-room and leaver contract](research/api-surfaces/recurring-rooms-and-leavers.md)
defines durable cancellation, provider scope and remaining policy choices.

## Agreed allocation rules

Among plans accommodating the same number of people, prioritise explicit
collaboration, then general team proximity, then previously unfulfilled demand.
Ordinary direct desk booking opens after the date's main allocation run; before
then, requests enter intent collection. Existing waitlist priority still applies
when capacity becomes available.
