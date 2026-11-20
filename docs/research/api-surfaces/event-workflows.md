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
period remains unselected. A confirmed application outage pauses affected
reconfirmation deadlines and preserves those tickets/seats while response is
impossible. Publish a recovery grace period within the event's usable attendance
window; event times and organiser-controlled registration cutoffs remain in force.

Existing offers remain held during venue recovery, with their expiry clocks
paused while acceptance cannot produce a valid ticket. Resume acceptance only
when the venue is secured and registration is open. Publish revised deadlines
bounded by the registration cutoff; an organiser extension is required for later
acceptance.

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
or expiry, subject to the administrative withdrawal rule below, and counts
against available capacity. Acceptance rechecks current
audience, registration and visit eligibility before issuing the ticket and
consuming the hold in the same owning capacity transition. An offer itself is
not a ticket or admission entitlement.

Decline or expiry ends the waitlist request and releases the offered seat to the
next eligible person. An attendee can explicitly rejoin as a new request under
the event's ordering policy; the previous request's position is not restored.

Temporarily ineligible queued attendees retain their original position but are
skipped while ineligible; eligible people behind them can receive offers. A
queued request holds no capacity. This differs from a previously issued ticket
or an outstanding exclusive offer.

When an event's office or time changes, withdraw unaccepted offers made under
the old details and preserve each waitlist request and its position. Issue a
new offer under the updated details when ready. Administrative withdrawal is
distinct from attendee decline or expiry and does not end the waitlist request.
If acceptance committed before the change, the issued ticket follows the normal
material-change reconfirmation workflow instead.

Offer acceptance ends no later than the published registration closing time;
clip each offer's deadline to that cutoff. An organiser can explicitly extend
registration to permit later acceptance. Outage grace alone does not extend the
registration window. Expired offers and cancelled tickets cannot grant admission.

Event waitlists have their own policies. Desk allocation priorities and the
same-day desk offer duration do not define event ordering or expiry. Initial
numeric offer/notice/recovery-grace defaults remain unselected and require
explicit policy configuration; the organiser supplies the reconfirmation deadline.
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
Include reconfirmation during outage, paused offer clocks versus registration
cutoff, temporary queue ineligibility/restoration, administrative offer withdrawal
and acceptance racing a material event change.
Check all actions through their supported public contracts and
the real owning transaction boundaries. The
[request contract](request-behaviour.md) supplies recovery, authority and retry
requirements independently of the event's business state.
