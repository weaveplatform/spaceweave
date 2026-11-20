# Confirmed application outages and deadline protection

A confirmed outage can prevent staff from checking in, accepting offers or
responding to an event reconfirmation. SpaceWeave records the affected offices,
actions and time interval once through its shared operational facilities.
Booking and Events apply that declaration to their own deadlines; Policy retains
the configured protection and recovery-grace rules. This is the agreed logical
ownership across all five repository architecture candidates.

## Consumers and authority

| Consumer | Required contract |
| --- | --- |
| Authorised operator/admin | Declare or update a confirmed interruption and restoration through supported public operational commands with current capability and office scope. |
| Staff/delegate clients | Read the applicable pause, revised deadline and recovery outcome through the relevant booking/event resource; an operational declaration grants no booking authority. |
| Booking workers and commands | Read authoritative incident scope/revision and apply relevant protection to no-show, desk-offer and other agreed booking deadlines. |
| Events workers and commands | Read the same incident declarations and apply relevant event-offer/reconfirmation protection using Events-owned state. |
| Platform health and job facilities | Supply observations and execute owning handlers; health readings and queue delay alone do not confirm an outage or choose expiry policy. |

Operational commands use the supported REST contract, including retry recovery,
concurrency and restricted impact/status reads. Private worker interfaces do not
become a second public incident or booking API. Exact endpoint names, capability
names and response schemas remain contract-engineering work.

## State custody

The operational facility alone writes the declaration's identity, affected
offices/actions, start/restoration times and revision. It exposes narrow read and
ordering interfaces to the business owners and does not write claims, offers,
tickets or their effective deadlines. Job/outbox facilities retain execution
mechanics; Policy retains configurable rules rather than incident declarations.

Booking and Events own the state needed to apply protection, preserve applicable
policy versions, and publish their actual paused/resumed outcomes. They do not
independently recreate the shared declaration. Consumers can explain which
incident and policy affected a deadline without giving every caller access to
the underlying operational record.

## Ordering and recovery

Native Exchange room check-in and autorelease follow provider policy. A local
application incident does not establish a pause or extension of that native
deadline; observe the actual outcome and use supported native check-in channels.
The [room contract](recurring-rooms-and-leavers.md) records this boundary.

Declaration publication and an affected expiry must have a defined ordering.
The expiry command checks current incident scope/revision through its private
read contract alongside its own state/guards; a lagging asynchronous projection
or an old job payload is insufficient. A proposed implementation uses an ordered
scope guard/revision checked within the owning transition transaction. Each
candidate must validate that protocol with concurrent declaration/expiry tests.

If publication wins, the affected workflow observes protection before expiring.
Already committed reassignments remain valid; a declaration recorded later must
not resurrect an earlier claim over its replacement. For booking/event
transitions, recovery applies configured grace and publishes revised deadlines
within the actual usable booking/event
period and the organiser-controlled registration cutoff. A late declaration
exposes affected earlier outcomes for authorised review rather than silently
rewriting them.

Outcome recovery has a separate guarantee: when a declared outage spans a
published recovery deadline, preserve the affected retry evidence and extend
recovery through the end of the next office working day after restoration.
The owning recovery facility publishes the extended deadline using the office
calendar. This preserves outcome recovery without extending booked periods,
physical capacity or event registration cutoffs; the
[request contract](request-behaviour.md#retention-and-ambiguous-outcomes)
defines the underlying promise.

Test duplicate declarations, scope-limited actions, restoration, retries after a
lost response, declaration/expiry races, worker delay and restoration from backup.
Verify that Booking and Events read one declaration while retaining separate
transition authority. See the [outage policy](../../policy-defaults.md#outage-protection),
[event contract](event-workflows.md), [request contract](request-behaviour.md) and
[ownership decision](../../adr/0031-share-incident-declarations-with-owning-deadline-workflows.md).
