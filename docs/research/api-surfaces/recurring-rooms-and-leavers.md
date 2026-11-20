# Recurring rooms, usage and leaver cancellation

Recurring room bookings can outlive the need for the meeting and consume
capacity for months. SpaceWeave supports series and individual-occurrence
creation, changes and cancellation, restricts series authoring to explicitly
permitted people, and cancels abandoned series using verified occurrence-use
outcomes. Confirmed leavers' owned room bookings are cancelled rather than sent
for room-ownership handover. These requirements constrain all five repository
architecture candidates without selecting a repository style.

## Consumers and authority

| Consumer | Contract |
| --- | --- |
| Staff/mobile-web author | Create/manage series only with the configured recurring-room capability and applicable office/resource scope. Series permission is not granted to every staff member. |
| PA/delegate | Retain actual actor and organiser/booking owner. Recurring-room capability and current delegation are checked for the requested action; creating the series does not make the PA its owner. |
| Organiser or authorised attendee | Check in to the specific occurrence under current verified identity and meeting authority. Series-authoring permission is separate from eligibility to check in. |
| SDK, script and user-context MCP | Use the same supported REST capabilities and constraints. A consumer label or an owner ID cannot bypass the recurring-room restriction. |
| Booking workers | Evaluate committed occurrence-use outcomes, enforce the configured threshold, and accept owner-based leaver cleanup through private owning interfaces. |
| Exchange workers | Reconcile series/occurrence/exception identities, desired cancellation and observed provider state; report actual cancellation progress. |
| Administrator/operator | Configure permitted roles/assignments and policy, inspect scoped series/use/cleanup outcomes and repair visible failures through owning commands. |

Existing configurable RBAC supplies the recurring-room capability through scoped
assignments; no separate permission service or hard-coded list is required.
The capability is assigned to selected people; unattended applications do not
inherit it from general booking grants.
Commands revalidate current authority. Private policy/leaver workers execute
their explicit workflow authority rather than impersonating a PA or requiring
the departed organiser to log in. Existing independent machine-grant rules still
apply; exposing a REST operation grants no new application authority.

## Series and occurrence state

Booking owns series identity/revision, recurrence/timezone, organiser/owner,
creating actor, individual occurrence identity/revision, claims/holds, check-in,
evaluation outcome, consecutive-miss progress and cancellation reason/state.
An occurrence changed independently retains its relationship to the series.
Exchange owns provider series-master/occurrence/exception correspondence and
desired/observed progress; it cannot write Booking's claims or usage decisions.

Creation and edits distinguish whole-series versus individual-occurrence scope,
use the normal retry/version contract, and return actual acceptance/capacity
outcomes. Recurrence expansion is bounded work with durable progress and
configurable limits. A bounded expansion or calendar-view window is not the
entire lifetime of a series.

Usage evaluation covers managed-room series already present in Exchange as well
as app-created series. Activate the policy explicitly with a verified evidence
baseline; missing historical check-in data cannot be backfilled as non-use.

## Establishing use and cancelling abandoned series

The organiser or an authorised attendee personally checks in to an occurrence.
The PA who created it need not attend. Booking validates the occurrence, current
actor/attendee authority, version and configured check-in window before committing
use evidence. A QR reference identifies the occurrence/room and does not grant
authority. A repeated check-in must not count the occurrence twice.

Commit valid use, its consecutive-count reset, retry outcome and facts together.
Evaluation/cancellation revalidates current series and occurrence state under
the same version/guard protocol; stale evaluation cannot erase a committed
check-in, and late work cannot reopen a cancelled series. An application actor
cannot substitute its own credentials or a supplied owner ID for the required
person's check-in.

Usage of the room does not establish that every invitee or the organiser was
personally present. Attendance/reporting consumes committed facts with provenance;
its projections do not authorise the cancellation decision.
Series/use/cancellation views enforce existing owner, delegation and operational
data scopes. Routine availability does not expose general identified attendance
or raw check-in evidence.

For each eligible occurrence, Booking records the use-evaluation outcome and
applicable policy version. Consecutive unused occurrences increment the series
count; verified check-in resets it. Evaluation uses occurrence order and durable
outcomes, so scheduler retries or out-of-order delivery cannot invent extra
misses. Cancelled, unaccepted or otherwise unusable occurrences and confirmed
application-outage protection require explicit eligibility handling rather than
being counted as evidence of abandonment.

At the configured threshold, commit a series cancellation decision and durable
Exchange work, cancel the remaining meetings and notify attendees. This cancels
the remaining meeting series rather than merely removing room bookings.
Preserve historical outcomes and the reason for cancellation. The initial
threshold number and check-in/evaluation deadline remain unselected and must be
configured before automatic enforcement is enabled.

Automatic early release of an individual occurrence merely because nobody has
checked in has not yet been agreed. That is distinct from the consecutive-miss
series policy. Enforcement of recurring-room authoring restrictions for series
created directly in Outlook is also a separate provider-policy decision; an
app permission does not itself configure Exchange resource admission.

## Leaver cancellation

Identity supplies a confirmed, effective workforce departure for a stable staff
identity. Booking cancels that person's remaining room bookings, including
single meetings and recurring series, using organiser/booking-owner identity.
The target is meetings that reserve managed rooms; the cleanup is not a general
instruction to cancel unrelated roomless calendar meetings.
Meetings created by a departing PA for other owners remain with those owners.
Source disable, SCIM deletion, team changes or an incomplete import do not
invent a workforce departure. Preserve known state during source outages.

The room-cancellation workflow replaces assisted room handover. Events retains
its own handover and venue/ticket authority; ordinary room cleanup cannot bypass
those owning interfaces. Historical records and identifying copies continue to
follow their access and retention contracts.

Cleanup includes Exchange-originated room bookings and future series occurrences
outside the currently expanded local window. Record a durable cancellation fence
and progress so recurrence expansion, late acceptance or provider replay cannot
recreate cancelled commitments. The effective departure determines cleanup;
access revocation and cancellation progress remain separate observable outcomes.

## Cancellation and integration consistency

Booking commits the cancellation intention, reason, retry outcome and durable
follow-up work together. Provider calls occur after commit. Exchange applies the
provider operation and returns observations through Booking before checkpointing.
Public status distinguishes accepted cancellation, pending provider work and
reconciled outcome. Retained provider uncertainty must not be reported as a room
being free in Exchange. Failures remain retryable/repairable and visible to
authorised operators; automatic cleanup does not depend on a user's active session.

Meeting cancellations update Exchange and notify attendees through the applicable
provider workflow; app notices follow existing message preferences. They do not
expand the individually opted-in Teams booking-raised message permission.

## Provider facts and implementation validation

Graph distinguishes series masters, occurrences and exceptions; listing a
master's instances returns occurrences/exceptions within a requested date range.
Preserve those distinctions and validate series/occurrence changes against
current provider state. [Event model](https://learn.microsoft.com/en-us/graph/api/resources/event?view=graph-rest-1.0),
[List instances](https://learn.microsoft.com/en-us/graph/api/event-list-instances?view=graph-rest-1.0).

Exchange's documented administrative future-meeting cancellation targets the
organiser and requires its mailbox to remain able to send cancellation messages.
A matching recurring instance can cancel the whole series, and the query window
is bounded. This is a provider capability to evaluate, not a selected adapter or
a drop-in filter for only managed-room meetings; its documented scope includes
other meetings with attendees/resources. [Remove-CalendarEvents](https://learn.microsoft.com/en-us/powershell/module/exchangepowershell/remove-calendarevents?view=exchange-ps).

Validate provider scope, mailbox-deprovisioning order, series/exception coverage,
attendee notifications and uncertain-result recovery in an authorised tenant.
Exercise restricted authors, delegated creation, organiser/attendee check-in,
consecutive misses/reset, duplicate evaluation, threshold/cancellation races,
leaver-versus-disable, departing PAs, future-window expansion and replay after
cancellation. Tenant offboarding must preserve a supported cancellation route;
the precise provider strategy and completion checks remain engineering work.

See the [recurring-room decision](../../adr/0034-restrict-recurring-room-bookings-and-cancel-unused-series.md),
[leaver decision](../../adr/0035-cancel-leavers-owned-room-bookings.md),
[room recovery contract](room-recovery.md), [request contract](request-behaviour.md)
and [access model](access-channels-and-rbac.md).
