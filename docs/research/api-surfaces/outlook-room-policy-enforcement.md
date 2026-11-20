# Exchange-led room admission and check-in release

Exchange Online and its supported Microsoft Places check-in and autorelease
capabilities govern room admission and release. SpaceWeave must follow those
provider semantics across application-created and Outlook-created bookings.
Application state, RBAC or a worker timer cannot independently redefine the
provider's decision. This note records native facts and requirement gaps; it
selects no tenant configuration or custom enforcement algorithm. Sources checked:
20 November 2026.

## Requested outcomes and compatibility boundary

The requested outcomes are restricted recurring-room authoring across both
application and Outlook entry paths, configurable no-check-in room release for
ordinary one-off meetings and series occurrences while retaining the meeting,
and cancellation of remaining future meetings after a configured number of
consecutive unused occurrences.

Existing configurable application RBAC grants selected people recurring-room
authoring authority in scope. Actual author and organiser/owner remain distinct,
including when a PA acts for another person. That application model does not
prove an equivalent Exchange admission rule. Likewise, the requested personal
organiser or authorised-attendee check-in does not establish that native
check-in has the same eligibility rules.

These outcomes must be reconciled with supported Exchange behaviour before
being stated as delivered. A gap is not permission to add an application-owned
admission policy, grace timer or consecutive-miss cancellation worker. Native
release remains authoritative. Owner-leaver cancellation is a separate workflow
whose provider scope and mailbox lifecycle also require validation.

## Native booking admission

Exchange documents `AllowRecurringMeetings` as a resource-wide Boolean.
`AllBookInPolicy` and `BookInPolicy` govern automatic approval of in-policy
requests. `RequestInPolicy` identifies users whose in-policy requests require
resource-delegate approval; `RequestOutOfPolicy` permits named users to request
policy exceptions requiring approval. `ResourceDelegates` identifies approvers.
For delegated requests, `BookInPolicy` allows automatic approval if either
delegate or principal is listed. These controls do not establish an
actual-author-only, per-person recurring entitlement.
Native conflict settings also govern whole-series versus per-occurrence outcomes.
[Calendar processing][calendar-processing]

**Unproven configuration candidate:** ordinary in-policy one-offs remain
accepted while `AllowRecurringMeetings=false` routes selected users' recurring
requests through `RequestOutOfPolicy` and delegates. The documentation does not
confirm that combination. Its exception channel is broader than recurrence.
Neither this combination nor a broad all-bookings gate is selected.
[Calendar processing][calendar-processing]

The admission gap therefore concerns both policy granularity and actual author
identity. Application RBAC may describe authority in SpaceWeave; it must not be
reported as Exchange enforcement. Tenant testing must distinguish refusal or
approval before resource acceptance from a later corrective change. Observing
an accepted Outlook booking and subsequently removing it would not demonstrate
prevention at admission.

## Delegated Outlook provenance

Microsoft's delegated-calendar example identifies the delegate as
`eventMessage.sender`, the principal as `eventMessage.from`, and the principal
as `event.organizer`. The delegate identity is in the message rather than the
associated event. The organiser event alone therefore cannot identify the actual
author for that delegated path. [Delegated calendar creation][delegated-calendar]

Provider request messages are a candidate source of provenance for observation
and supported integration. Their access, retention, trustworthy correlation and
coverage across Outlook and delegation paths remain tenant proof requirements.
An absent message does not justify inventing an author or treating the
organiser as the person who created the request. Provenance visibility alone
does not establish a supported hook that can replace Exchange's initial
admission decision.

## Native grace, release and check-in scope

Microsoft documents `EnableAutoRelease` with a
`PostReservationMaxClaimTimeInMinutes` interval from meeting start; the documented
default is ten minutes. Native methods include Teams panels, joining through a
Teams Rooms console, Teams chat and documented email fallback. Applicable
licensing and device configuration matter; policy changes can take 48 hours to
apply. Room participants receive a room-release message when nobody checks in.
[Room autorelease][room-autorelease]

The Places FAQ describes cancellation of the room reservation when nobody checks
in within the configured period. It does not require the organiser: anyone
physically in the room or with access to the Teams meeting chat can check in,
and one person suffices. This is broader than a custom SpaceWeave
authorised-attendee rule. [Places FAQ][places-faq]

The required alignment is to native eligibility and timing. The application must
not promise that only its authorised-attendee set can prevent release, or
override a native check-in because its own record has no matching actor.

Neither reviewed page explicitly specifies preserved organiser/attendee event
copies or future-series changes. Validate these, room/location presentation and
exact notification recipients in the tenant. Do not infer remaining-meeting
cancellation from room-reservation release.
[Room autorelease][room-autorelease], [Places FAQ][places-faq]

## Supported application-to-native check-in surface

Graph v1.0 documents `POST /places/{placesId}/checkIns` to create a
`checkInClaim` for a calendar reservation and prevent configured autorelease.
The request binds the claim through `calendarEventId`, referring to the event's
`iCalUId`, and supplies a `checkInMethod`. Its permission table supports
work/school delegated `Calendars.ReadWrite`; application permissions and
personal-account delegated access are unsupported.
[Create native check-in claim][create-checkin]

Graph also documents
`GET /places/{placesId}/checkIns/{calendarEventId}` for reservation check-in
status. It has the same work/school delegated `Calendars.ReadWrite` requirement
and no application permission. The page does not establish bulk/list access or
POST idempotency. [Read native check-in claim][get-checkin]

The claim model exposes calendar event ID, method and creation time, with no
named-person field. Its `verified` method means check-in through a device bound
to the place; it is not a label for arbitrary application identity validation.
The documented methods are create and get. [Check-in model][checkin-model]

These establish supported API surfaces, not an automatic bridge from local
check-in records. Tenant integration still needs to prove user eligibility,
reservation and occurrence correlation, applicable permissions, timing,
duplicate/late submissions, error handling and the effect on native release.
It does not justify an app-only check-in worker or a second expiry authority.

A SpaceWeave check-in flow can act as a delegated adapter to the native decision,
using native eligibility rather than imposing an organiser-only rule. Until the
native result is known, local success must not be presented as confirmed
prevention of release. Validate reading check-ins from every native channel.
After an uncertain submission, the supported read operation is a recovery
candidate; its observed reservation status does not identify who checked in or
prove POST replay semantics. An access failure or missing response is not
evidence that the room is unused.

## Consecutive-unused-series cancellation gap

The reviewed official room-autorelease documentation establishes a
reservation-level grace and release mechanism. It does not establish a native
policy that counts N consecutive unused occurrences and cancels the remaining
meeting invitations. The documented calendar-processing controls also do not
establish that threshold. This is an unverified requirement gap, not a claim
that Microsoft can never support it.
[Room autorelease][room-autorelease],
[Calendar processing][calendar-processing]

The desired threshold behaviour must therefore remain unresolved until a
supported provider capability and its exact semantics are established. Do not
infer it from per-reservation release or introduce a local counter and automatic
cancellation algorithm. If supported future-series cancellation is later
verified, validate its cutoff, modified exceptions, attendee notifications and
coverage beyond the locally expanded occurrence window.

The desired preservation of the current occurrence's invitation must not be
accidentally overridden by a proposed future-only cancellation scope; its actual
native release behaviour still requires validation.
A blanket series-cancellation operation or administrative cmdlet must not be
assumed to meet that requirement. If Exchange's supported scope differs, record
and reconcile the requirement difference rather than implementing an independent
replacement. No custom organiser-event attendee/location rewrite is selected as
an autorelease mechanism.

## Minimum tenant proof and delivery status

| Area | Evidence required before claiming compatibility |
| --- | --- |
| Effective native policy | Room processing settings, applicable licences and devices, propagation completion, supported environment and integration permissions. Configuration intent alone is insufficient. |
| Outlook admission | Direct one-off and recurring requests by permitted/restricted people, changes that add rooms or recurrence, actual initial resource decisions and delegate approval behaviour. Test any candidate combination without assuming it is a per-person recurrence control. |
| Principal and actual author | PA/delegate entitlement permutations, message sender/from/organiser correlation, missing-message handling and supported provenance on each entry path. |
| Native check-in | Panel, Rooms console, Teams chat and email behaviour, including a person who qualifies natively but falls outside the requested custom app attendee rule. |
| Graph check-in integration | Delegated create/read user context, correct room/reservation/occurrence identity, native prevention of release, retries, late submission and visible failures. Validate reading native-channel check-ins and recovery from uncertain results without inventing a person or an app-only bulk reader. |
| Release outcome | Both one-off and recurring bookings, including exceptions: actual room availability, retained meeting invitation, online/time data, recipient notices and remaining future reservations. |
| Threshold and leaver scope | Authoritative provider documentation and tenant evidence for any proposed cancellation operation, current/future cutoff, exceptions, organiser ownership, mailbox lifecycle and departing-PA behaviour. No automatic threshold policy is established by this note. |

Until the relevant proof exists, report each capability as documented native
behaviour, validated tenant behaviour or an unresolved requirement gap. A
SpaceWeave intention, queued provider request or calendar update response alone
does not prove Exchange considers the room free. Retain provider uncertainty in
the user/operator status; do not create a conflicting local room commitment.

[calendar-processing]: https://learn.microsoft.com/en-us/powershell/module/exchangepowershell/set-calendarprocessing?view=exchange-ps
[delegated-calendar]: https://learn.microsoft.com/en-us/graph/outlook-create-event-in-shared-delegated-calendar
[room-autorelease]: https://learn.microsoft.com/en-us/microsoft-365/places/room-desk-booking/enable-checkin-auto-rooms
[places-faq]: https://learn.microsoft.com/en-us/microsoft-365/places/frequently-asked-questions
[create-checkin]: https://learn.microsoft.com/en-us/graph/api/place-post-checkins?view=graph-rest-1.0
[get-checkin]: https://learn.microsoft.com/en-us/graph/api/checkinclaim-get?view=graph-rest-1.0
[checkin-model]: https://learn.microsoft.com/en-us/graph/api/resources/checkinclaim?view=graph-rest-1.0
