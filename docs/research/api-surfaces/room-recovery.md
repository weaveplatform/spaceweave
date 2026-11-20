# Meeting-room changes and recovery

SpaceWeave synchronises ordinary meeting-room bookings with Exchange Online.
A meeting and its attendees can remain valid while its requested room is
unavailable. If a room replacement fails after the previous room has been lost,
retain the meeting and attendees, show a clear no-secured-room outcome, notify
the organiser, and offer available replacements or explicit cancellation.
This agreed product behaviour applies across all five repository candidates.

## Consumers and outcomes

| Consumer | Required contract |
| --- | --- |
| Staff/mobile-web organiser | See the retained meeting, the room outcome and any pending recovery. Select an available replacement or explicitly cancel the meeting under current authority. |
| Authorised delegate, SDK, script or user-context MCP | Use the same public room change/recovery commands, retaining actual actor and organiser identity and the ordinary retry/version contract. |
| Booking commands and workers | Own local room intentions, candidate holds, claims and business recovery outcomes; confirm a replacement only after current acceptance and capacity validation. |
| Exchange reconciliation workers | Own desired/observed provider state and progress, preserve observations and apply business outcomes through Booking; never write claims directly. |
| Notifications | Deliver committed recovery outcomes to the organiser through the existing message preferences; recovery notices do not expand Teams message opt-in. |

Read models distinguish a retained meeting, a secured room, a pending candidate
request and unresolved integration work. Display no secured room when loss is
established; when observations are uncertain, expose that uncertainty and
reconcile before claiming a definitive room outcome. Exact enum names and
endpoint spelling remain contract-engineering work.

## Change and recovery workflow

1. An authorised room change records the intended revision and local candidate
   hold through Booking. Retain the old local claim where valid and feasible;
   the provider's treatment of the old room must be observed separately.
2. Exchange sends the committed intention, retains correlated observations and
   reconciles ambiguous/out-of-order results. Successful provider HTTP delivery
   is distinct from room acceptance.
3. If the candidate fails and the previous room is known to be lost, Booking
   records the retained meeting's no-secured-room outcome. Retaining the meeting
   does not establish a physical room claim. Old/candidate claims and provider
   observations continue through their owning transition/reconciliation paths.
4. Notify the organiser and expose available replacement choices or explicit
   meeting cancellation. A recovery command rechecks current authority, intent
   revision and capacity. An availability listing is not a reservation.
5. A chosen replacement remains pending until its current room acceptance and
   Booking capacity validation succeed. A late acceptance cannot confirm a
   superseded request or overwrite a later claim. Public reads report the
   actual room and integration outcomes.

The retained meeting and attendee details survive the failed room change.
Compensating for a failed room request is not authority to cancel the organiser's
meeting. An explicit meeting cancellation uses the normal organiser/delegate
authority and provider reconciliation workflow.

Recovery rechecks the latest meeting intention and provider state. A genuine
organiser cancellation or superseding change remains effective; stale failure
or acceptance work cannot revive it.

Events-managed venue changes continue through the separate
[event recovery contract](event-workflows.md); generic room recovery cannot
bypass Events' ticket, admission or venue-capacity rules.

## Provider facts and validation limits

Microsoft documents event updates for attendee/location changes and separately
documents resource-mailbox acceptance, decline and delegate processing. These
contracts do not establish an atomic room-replacement or guaranteed restoration
operation. That is the boundary of this source review, not proof that a
particular tenant behaviour is impossible. [Graph event updates](https://learn.microsoft.com/en-us/graph/api/event-update?view=graph-rest-1.0),
[Exchange room booking](https://learn.microsoft.com/en-us/exchange/recipients-in-exchange-online/manage-resource-mailboxes#booking-tab).

Organiser cancellation and removing an event from a containing calendar have
different documented effects; do not assume deleting a room copy is equivalent
to cancelling the organiser's meeting. [Cancel event](https://learn.microsoft.com/en-us/graph/api/event-cancel?view=graph-rest-1.0),
[Delete event](https://learn.microsoft.com/en-us/graph/api/event-delete?view=graph-rest-1.0).

Validate old-room release timing, candidate decline/timeout, provider readback,
notifications and actual permission/series behaviour in an authorised tenant
before promising specific compensation mechanics. Exercise original-room loss
with meeting retention, pending/rejected replacements, retry after response loss,
concurrent Outlook edits, stale acceptance and organiser cancellation. Verify
local capacity and visible recovery independently of notification delivery.
The [ownership decision](../../adr/0033-retain-meetings-during-room-replacement-recovery.md)
and [request contract](request-behaviour.md) define the application obligations.
