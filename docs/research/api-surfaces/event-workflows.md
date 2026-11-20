# Event registration, recovery and ticket lifecycle

SpaceWeave supports free, named tickets for capacity-limited internal events,
including staff and verified external visitors. An event has a secured venue
booking and its own audience, registration access and ticket limit. This contract
records agreed application behaviour for architects and engineers; exact REST
routes, schemas and implementation classes remain design work.

## Users, consumers and ownership

| Actor or consumer | Supported purpose and boundary |
| --- | --- |
| Staff attendee | Register, inspect and cancel their own unused ticket; join an enabled waitlist and reconfirm material changes. |
| Verified visitor | The same attendee actions within their restricted event/visit/ticket scope; no general staff authority. |
| Organiser or authorised administrator | Manage the event and secured venue, resolve recovery, cancel the event and communicate confirmed changes under current capabilities and office scope. |
| Reception or admission staff | Validate the current event, ticket and visit conditions and record admission; reception arrival remains a separate visit action. |
| UI, MCP, SDKs and scripts | Use supported public REST with their actual human/application authority; consumer labels do not grant attendee or organiser powers. |
| Platform workers | Deliver committed changes and process eligible expiry/recovery work through private owning interfaces. |

Events owns registration, ticket capacity, entitlements, reconfirmation, event
waitlists and admission. Booking owns venue claims; Visitors owns hosted visits,
their validity and reception history. The
[Events boundary](../../adr/0023-separate-events-from-venue-booking.md) and
[visitor registration transaction](../../adr/0024-create-event-visitor-visits-with-tickets-atomically.md)
keep each owner responsible for its writes. Notification delivery follows commit
and does not change ticket success. Event messages use the approved delivery
channels; the existing private Teams opt-in concerns booking-raised messages.

## Registration and admission invariants

- New tickets and exclusive offer holds respect the event limit and secured
  venue capacity. Active offers count against available seats before they become
  tickets. Venue/capacity changes account for issued tickets and active offer
  holds through the owning interfaces in one local transaction where required.
- A visit created solely for registration commits with its ticket or neither
  does. A failed issue leaves a reused existing valid visit intact. Retained
  definitive refusals do not create a new visit or ticket.
- Admission checks current event availability, audience, ticket status, any
  required reconfirmation, and visitor host/window restrictions. A prior page,
  message or rendered ticket does not establish current eligibility.
- One ticket has at most one recorded admission. Cancellation and admission
  revalidate current state so a cancelled entitlement cannot also admit someone.
  Completed admission records remain history.

## Venue recovery and material changes

Confirmed loss of the secured venue retains existing ticket entitlements pending
recovery and pauses admission and further issuance. The organiser explicitly
secures a suitable replacement or cancels, and confirmed changes are communicated
to attendees.

A suitable replacement within the same office and time retains tickets with
notification. A change of office or time requires attendee reconfirmation before
admission, plus revalidation of linked visitor hosts and visit windows. A venue
replacement alone does not establish that those visit conditions remain valid.
The organiser publishes a reconfirmation deadline and notifies affected attendees.
At that deadline, unconfirmed tickets release their seats under the event's
enabled waitlist policy; without a waitlist, normal registration rules apply.
Public resource views show both the deadline and the outcome. The initial notice
period and reconfirmation protection during an application outage remain open.

## Visitor eligibility after issuance

If a visitor's host leaves or their visit becomes invalid after successful ticket
issuance, block admission while retaining the ticket and its seat pending
authorised repair or explicit ticket cancellation. Existing visitor restrictions
still apply. Preserve the distinction between an invalid visit, a retained
entitlement and an admitted attendee in the public resource views.

## Attendee and event cancellation

Staff and verified visitors may cancel their own unused tickets before admission,
releasing those seats. Current scope and ticket state govern the action;
completed admissions remain historical records.
If an associated future visit has no other valid purpose, also cancel that visit.
Preserve visits supporting other purposes and existing reception history.
Visitors checks current purposes before applying the cancellation; a stale event
link cannot cancel a visit that has acquired another valid purpose.

When an event is cancelled, also cancel visits whose only remaining valid purpose
was that event. Preserve visits supporting other valid purposes, including reused
visits. Cancellation changes lifecycle state; record retention follows the
organisation's existing visitor and historical-data rules.

## Optional event waitlists

An event may enable a waitlist for eligible attendees. Released seats are offered
to eligible waitlisted attendees in join order before fresh registrations.
An offer holds one seat exclusively for its recipient until acceptance, decline
or expiry, counting against available capacity. Acceptance rechecks current
audience, registration and visit eligibility before issuing the ticket and
consuming the hold in the same owning capacity transition. An offer itself is
not a ticket or admission entitlement.

Decline or expiry ends the waitlist request and releases the offered seat to the
next eligible person. An attendee can explicitly rejoin as a new request under
the event's ordering policy; the previous request's position is not restored.

Offer acceptance ends no later than the published registration closing time;
clip each offer's deadline to that cutoff. An organiser can explicitly extend
registration to permit later acceptance. Outage grace alone does not extend the
registration window. Expired offers and cancelled tickets cannot grant admission.

Event waitlists have their own policies. Desk allocation priorities and the
same-day desk offer duration do not define event ordering or expiry. Initial
numeric offer durations, temporarily ineligible queued requests and existing
offers during venue recovery or material event changes remain open decisions.
Retry and capacity protection must cover simultaneous offer
acceptance, fresh registration, cancellation and venue changes.

## Validation evidence required

Exercise registration losing the last seat, reused-visit preservation, material
changes racing admission, visitor ineligibility after issuance, ticket cancellation
racing admission, shared-purpose visit preservation, waitlist priority over fresh
registration, exclusive hold accounting, FIFO selection, decline/expiry/rejoin,
acceptance at the registration cutoff, reconfirmation deadline/release, purpose-based
future-visit cleanup racing a new valid purpose, duplicate retries and venue
recovery with existing tickets.
Check all actions through their supported public contracts and
the real owning transaction boundaries. The
[request contract](request-behaviour.md) supplies recovery, authority and retry
requirements independently of the event's business state.
