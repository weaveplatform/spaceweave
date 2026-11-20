# Exchange-led room use, recurring bookings and leavers

Exchange Online and its supported Microsoft Places room capabilities govern
managed-room admission, check-in and autorelease across SpaceWeave and Outlook.
SpaceWeave records booking intentions, maintains local consistency and reflects
provider decisions through supported integrations. It does not run a second
room-admission policy or expiry authority. This requirement applies to all five
repository architecture candidates; the repository-style decision remains pending.

## Consumers and authority

| Consumer | Contract |
| --- | --- |
| Staff/mobile-web client | Request one-off or recurring rooms through supported application commands; distinguish request acceptance from native resource acceptance. |
| PA/delegate | Retain actual application actor and organiser/booking owner; pass supported provider context and follow native principal/delegate admission semantics. |
| Room check-in client | Use the supported native check-in flow with the required user context. Native eligibility and the native result govern whether the room is claimed. |
| SDK, script and user-context MCP | Use the same public contracts under actual identity and app permissions; a client label grants no additional native authority. |
| Booking workers | Apply observed native room outcomes to owned local state; preserve current versions and cancellation fences. They do not invent room no-shows or native check-in success. |
| Exchange workers | Own native policy/correspondence/progress and reconcile provider observations through Booking; use only the permissions supported by each provider operation. |
| Administrator/operator | Manage application RBAC separately from supported native room configuration; expose desired, effective and pending provider configuration truthfully. |

Application RBAC controls access to SpaceWeave operations and scoped delegation.
It does not substitute for Exchange room admission or create an equivalent
Outlook restriction. A recurring-only, actual-author-only entitlement must not
be presented as natively enforceable unless that exact behaviour is supported
and validated. Existing app-managed delegation authorises an application request;
it does not guarantee native room acceptance.

## One-off and recurring state

Booking owns local room intentions, series/occurrence business identities,
versions, capacity commitments and references to observed provider outcomes.
A room occurrence may be a one-off booking or a series instance/exception.
Exchange owns native series/master/occurrence correspondence, resource policy
observations, Places/reservation mappings, native check-in claims and
reconciliation progress. Native claim data is not a Booking-owned replacement
for provider policy.

Native accepted/conflicting outcomes must remain representable. A local
consistency conflict is not a native decline, and an application constraint
cannot discard provider truth or silently replace a configured native rule.
Room-specific neighbourhood priority also needs native expressibility; an
app-only preference must not be presented as admission enforced in Outlook.

Series creation/changes distinguish whole-series and individual-occurrence
scope, use the normal retry/version contract and expose native acceptance or
decline outcomes. Expansion is bounded durable work; a local calendar window is
not the complete lifetime of a series. SpaceWeave reflects the effective native
recurrence, conflict and booking-horizon settings rather than choosing different
application admission semantics.

## Native check-in and autorelease

The supported native check-in/autorelease policy applies to ordinary one-off
rooms and recurring occurrences where the provider feature is enabled and
applicable. The effective native grace and eligibility govern release. Native
channels may establish a valid check-in outside an app-specific attendee rule;
SpaceWeave must observe that accepted native claim rather than override it.

Graph documents native check-in create/read operations requiring work/school
user-delegated permission, without application permission. A SpaceWeave action
can adapt to those operations using supported user context and provider mapping;
an app-only worker is not a substitute for that context. Map the application
occurrence to the correct native room/reservation identity, including its
provider calendar-event identifier. The [provider research](outlook-room-policy-enforcement.md)
records the exact API and permission contracts.

Record request acceptance and retry recovery separately from the native result.
Provider calls occur outside application database transactions. After an
uncertain submission, use the documented native read contract where authorised
to establish the reservation's current claimed state; this is not proof of which
caller created it or a documented POST replay guarantee. Access failures or an
absent response do not establish non-use.

Native claim data does not identify a named person. Keep any application request
audit separate, and do not derive individual attendance or attendee presence
from a room claim. Native check-in status and resource availability require
appropriate source freshness and permitted data projections.

When Exchange autoreleases a room, apply the authoritative resource outcome
through Booking before promising usable local capacity. Public status separates
pending observation, known native release and uncertainty. A stale check-in or
callback cannot reclaim a room already reassigned. Waitlist/direct requests use
only capacity actually available under current native and local constraints.
Observe supported resource availability as well as organiser-event state; a
retained invitation alone establishes neither a held nor an available room.

The desired outcome is room release while retaining the meeting invitation.
The reviewed native documentation does not specify every organiser/attendee-copy
or future-series effect, so validate those exact outcomes in the tenant; do not
force the desired result with an independent calendar rewrite. Native release
is not evidence that remaining recurring meetings have been cancelled.

## Outlook admission and capability gaps

The same supported Exchange admission policy governs Outlook-originated and
SpaceWeave-originated room requests. Exchange integration supplies observed
policy and supported requester/principal evidence. An organiser field alone
must not be treated as proof of the actual author; where message provenance is
available, preserve and correlate it according to its documented semantics.
Never invent a native actor or a permission result when evidence is missing.

The [native-policy research](outlook-room-policy-enforcement.md) distinguishes
resource-wide settings, principal/delegate admission, native check-in APIs and
unverified combinations. Application-only role checks or cancellation after an
already accepted native request do not prove equivalent native admission.
No tenant configuration or custom admission algorithm is selected by this note.

The requested policy that counts consecutive unused occurrences and cancels the
remaining meetings has not been verified as a native capability. It remains a
requirement gap; SpaceWeave will not implement an independent counter or automatic
series-cancellation rule to fill it. Likewise, a recurring-only per-person native
entitlement requires proof of expressibility. A missing documented capability is
not proof that Microsoft can never support it.

## Leaver cancellation

Identity supplies a confirmed, effective workforce departure for a stable staff
identity. The leaver workflow cancels remaining managed-room meetings and series
owned by that organiser through supported Exchange cancellation operations.
Other-owner meetings created by a departing PA remain. Account disable, SCIM
deletion, team changes or incomplete imports do not establish departure.

This is an explicit offboarding workflow using supported provider cancellation,
not a replacement admission or no-show policy. Booking records cancellation
intent/fences and Exchange performs provider work after commit, exposing pending,
failed and reconciled outcomes. Cover future series beyond the locally expanded
window without assuming that a broad native cancellation command has exactly
the required room-only/current/future scope.

Preserve historical records under retention rules and a supported cancellation
route through mailbox offboarding. Events retains its separate handover and
venue/ticket authority. Neither source reactivation nor stale native observations
can resurrect cancelled commitments.

## Incidents, configuration and validation

Room grace and check-in eligibility come from effective native configuration.
The native default is not an independently selected SpaceWeave default; expose
configuration propagation and feature availability. Do not enable independent
app/native room timers or assume that a local incident declaration pauses
Exchange autorelease. Existing app-owned desk, event and request-recovery
protection contracts remain separately applicable.

Validate native permission/licensing/device prerequisites, delegated app
check-in create/read, other native check-in channels, one-off/series/exception
release effects, actual room availability, principal/delegate admission,
reservation correlation, uncertain results and leaver scope. The
[provider research](outlook-room-policy-enforcement.md) records delivery gates.
Observe policy freshness, native outcome lag and cleanup progress; a queued
operation or accepted HTTP mutation alone does not prove room availability.

See the [current decision](../../adr/0036-follow-exchange-room-policy-and-native-check-in.md),
[superseded app-specific proposal](../../adr/0034-restrict-recurring-room-bookings-and-cancel-unused-series.md),
[leaver decision](../../adr/0035-cancel-leavers-owned-room-bookings.md),
[room recovery](room-recovery.md), [request contract](request-behaviour.md) and
[access model](access-channels-and-rbac.md).
