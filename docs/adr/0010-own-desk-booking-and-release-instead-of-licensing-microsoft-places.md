# 0010: Own desk booking, check-in and release in spaceweave instead of licensing Microsoft Places

Status: Accepted

## Problem statement

Microsoft Places offers desk booking, check-in, auto-release, work location and occupancy analytics
inside Outlook and Teams. The features that matter most for spaceweave need a paid licence on every desk
or room. We need to decide which Places features spaceweave relies on, which it builds itself, and which
it leaves alone. A bad choice means paying for every desk, or running two booking systems that disagree
about who has which desk.

## Context

- spaceweave is deployed for one organisation on Microsoft 365. Entra ID handles sign-in, Exchange
  Online holds room mailboxes, and Teams and Outlook are where people already work (see the
  [integration architecture](../architecture/integration-architecture.md)).
- **Most of Places is no longer paid for.** Since 1 April 2026 these are included in ordinary Microsoft
  365 and Teams plans (E1/E3/E5, Business, F1/F3):
  - work plans and "who's in the office";
  - automatic work location from Wi-Fi or a desk peripheral;
  - Places Finder and Places Explorer;
  - booking rooms and desk pools;
  - desk assignment;
  - check-in;
  - managing the directory through PowerShell or Microsoft Graph.
- **What still costs money:**
  - **Per space:** booking an *individual* desk, auto-release, and occupancy reports. These need a Teams
    Shared Space licence on each desk, at $8 per month, which comes with three free single-space
    licences, so about **$2 per desk per month**.
    - For 1,000 desks that's about $24,000 a year before tax.
    - The free licences have to be ordered; they aren't added automatically.
    - Rooms that use Exchange auto-release need their own Teams Rooms or Shared Space licences.
  - **Per user:** quick book and Copilot room booking need a Copilot licence.
- **Places doesn't check anyone in from desk occupancy sensors.**
  - Its sensor partners (Crestron, Haltian, VergeSense) feed analytics only.
  - Desk check-in in Places means plugging into a mapped monitor (a Shared Space licence plus the Teams
    Rooms Pro portal, with no API), checking in by email, or, coming soon, checking in to the building.
  - spaceweave's signal ingest already treats sensor vacancy as evidence for release.
- **Places floor maps accept only IMDF.** DWG isn't accepted, and every update replaces the whole map.
  spaceweave produces its own floor maps (see the
  [floor-plan pipeline](../architecture/floor-plan-pipeline.md)).

### Places features and their licences

| Feature | What it is | Licence | Status |
|---|---|---|---|
| Building and floor directory | Building → floor → section → room, workspace or desk | Core | GA. Buildings are hidden until `Set-PlacesSettings -EnableBuildings` is turned on |
| Floor maps | Shown in Places Finder | Core | GA. IMDF only (GeoJSON, georeferenced, one building per package) |
| Room and desk-pool booking | Rooms and "workspaces" (desk pools) are Exchange mailboxes | Core | GA |
| Individual desk booking | Reservable or drop-in desks, each backed by a desk mailbox | **Teams Shared Space** (or the free single-space licence) per desk | GA. Teams Premium users kept access for a limited time |
| Desk assignment | A desk assigned to one person | Core | GA |
| Check-in | Building, room or desk | Core | GA |
| Auto-release | Frees an unclaimed booking (default 10 minutes) | **Space licence**: Teams Rooms or Shared Space for rooms; Shared Space for desks | GA. Reservable desks only |
| Auto-detected work location | From a mapped peripheral, or from Wi-Fi (BSSID mapped to a building) | Core | Peripherals GA. Wi-Fi is preview on Learn but "Launched" on the roadmap. Teams desktop on Windows or macOS only; not over Ethernet |
| Peripheral check-in to a desk | Plug into a mapped monitor or AV device | Shared Space per desk, set up in the Teams Rooms Pro portal | GA. Docks and webcams "coming soon". No API exposes it |
| Work plans and workplace presence | Planned and actual work location, visible within the organisation | Core | GA |
| Places Finder | Room and desk search with maps, in Outlook and Teams | Core | GA. Microsoft is "evaluating deprecation" of the older Room Finder |
| Places analytics | Building, room and desk-pool use | Core, but **occupancy reports need a space licence** | GA. Building occupancy comes from uploaded badge data only (kept 28 days). Desk pools under 10 desks aren't analysed |
| Quick book, Copilot room booking and rebooking | AI booking in Outlook | **Microsoft 365 Copilot** per user | GA |

Users whose mailbox is in on-premises Exchange get none of Places.

### How Places stores things

- **Every bookable space is an Exchange mailbox.**
  - Rooms are room mailboxes, desk pools are workspace mailboxes, and reservable or drop-in desks get a
    desk mailbox.
  - A booking is an ordinary calendar event, and booking rules are `Set-CalendarProcessing` settings.
- **The directory is separate from the mailboxes.** Names and capacity come from the Places directory.
  Changes can take up to 24 hours to appear.
- **Microsoft Graph Places API (v1.0):**
  - **Reading:** `building`, `floor`, `section`, `room`, `workspace` and `desk` need `Place.Read.All`.
  - **Writing:** needs `Place.ReadWrite.All`. An app also needs the Exchange RBAC roles
    `TenantPlacesManagement` and `MailRecipient`. Entra roles assigned through a group don't count.
  - **Check-in:** `POST /places/{id}/checkIns` is delegated only.
  - **Beta only:** bulk upsert, sensor devices and telemetry ingestion.
  - **Licence:** Microsoft lists directory management through Graph as core.

### Free surfaces spaceweave can build on

| Surface | What it gives spaceweave | Notes |
|---|---|---|
| Places directory (Graph) | Buildings, floors and sections that Outlook, Teams and work plans refer to by `placeId` | Rooms are already Exchange mailboxes |
| Work hours and locations (Graph v1.0) | Read planned office days (`occurrencesView`: `workLocationType` and `placeId`). Set today's location (`setCurrentLocation`) | Reading: `Calendars.Read.All` (application). Writing: delegated `Calendars.ReadWrite` |
| Presence work location (Graph v1.0) | Aggregated work location (manual > automatic > scheduled), including Teams' Wi-Fi detection | `Presence.Read.All`, up to 650 users per call |
| Teams app | Personal tab pinned for everyone by app setup policy; bot with Adaptive Cards; activity feed notifications; message extension; link unfurling; deep links | No licence found for custom apps. Single sign-on through nested app authentication. Activity notifications go to up to 100 recipients per call, and the app must be installed for each one |
| Universal Actions (`Action.Execute`) | One card that works in both Teams and Outlook, with a bot as the backend | Per-user refresh for up to 60 people |
| Outlook add-in (unified manifest) | Add a desk or room to a meeting; suggest a desk on office days | Web, new Outlook and classic Outlook. Not Mac or mobile with the unified manifest. No "location changed" event |
| Copilot declarative agent with spaceweave's MCP server | Booking from Copilot Chat | Microsoft's pages disagree on whether actions are metered for users without Copilot |

## Options

### Option A: License Places for desks and rooms

Make desks Places desks with desk mailboxes, buy Shared Space licences, and let Places handle booking,
check-in, auto-release and occupancy.

**Pros**
- Native booking in Outlook, Teams and Places Finder.
- Microsoft maintains the booking experience.

**Cons**
- About $2 per desk per month, plus a licence on every room that uses auto-release.
- Desks become Exchange mailboxes, so the allocation engine, neighbourhoods and waitlists have to work
  through calendar events they don't own.
- No sensor-based check-in or release.
- Floor maps must be converted to IMDF, and every update replaces the whole map.
- Places' licensing has already changed once; it could change again.

### Option B: spaceweave owns booking, and uses Places only where it's free

spaceweave owns desks, check-in, release and analytics. It puts its booking experience into Teams and
Outlook through its own app, and uses the free parts of Places as plumbing: the directory and work
location.

**Pros**
- No desk or room licences.
- One booking system, with allocation, waitlists and sensor evidence in one place.
- Outlook, Teams and work plans still name the same buildings and floors.

**Cons**
- spaceweave builds and maintains its own Teams and Outlook experience.
- Desks don't appear in Places Finder.
- The Outlook add-in doesn't run on Mac or mobile with the unified manifest.

### Option C: Ignore Places entirely

Integrate with nothing in Places.

**Pros**
- No dependency on Places at all.

**Cons**
- Work plans and Outlook's room finding name different buildings from spaceweave.
- Loses a free signal for planned office days and office-level presence.

## Recommendation

**Option B.** It removes the per-desk licence without losing the parts of Places that are free and
useful. The trade-off accepted is that spaceweave builds and maintains its own Teams and Outlook
experience, and desks don't appear in Places Finder.

## Decision

- **Desks belong to spaceweave.**
  - Desks aren't Exchange mailboxes and aren't licensed in Places.
  - Desks are kept out of the Places directory, or created only as `unavailable`, so nobody sees two
    booking systems.
- **Rooms stay Exchange room mailboxes.** Places Finder and Outlook keep booking them, and Calendar sync
  keeps spaceweave in step.
- **spaceweave writes the Places directory.** Each published floor-plan revision updates its buildings,
  floors and sections, so work plans and room finding use the same names.
- **Work location is a signal, not the source of truth.**
  - spaceweave reads planned office days to prefill attendance intent. spaceweave's own intent stays
    authoritative, because it drives allocation.
  - It reads the presence work location as office-level presence, never as desk-level.
  - On check-in it may set the person's current work location.
- **Check-in belongs to spaceweave:** a QR code, a Teams card, the web app, a spaceweave kiosk, or badge
  and sensor evidence through signal ingest.
- **spaceweave releases unclaimed bookings itself, for desks and rooms.** For rooms:
  - **When:** the room's check-in window, set in its kind profile, ends with no check-in.
  - **How:** the worker declines that one occurrence on the room's calendar through Graph, with a
    response sent. That frees the room in Exchange, and the organiser sees that the room declined. The
    rest of a recurring series is left alone.
  - **Told:** the organiser hears why, through Teams or email.
  - **Recorded:** the release is written as a `DomainEvent` and an `AuditEntry`.
  - **Permission:** application `Calendars.ReadWrite`, limited to room mailboxes through Exchange RBAC
    for applications.
  - **Panelled rooms:** a check-in on a Cisco or Crestron panel reaches spaceweave only if the panel
    changes the calendar. Until that's confirmed, these rooms either take check-in through spaceweave or
    have release turned off in their kind profile, so a meeting in use isn't cancelled.
- **The Teams and Outlook experience is spaceweave's own:**
  - a Teams app with a personal tab pinned by app setup policy;
  - Adaptive Cards with Universal Actions for check-in, waitlist offers and no-show warnings, in both
    Teams and Outlook;
  - activity feed notifications;
  - an Outlook add-in;
  - Copilot and other assistants through the MCP server.
- **Not used:**
  - Places desk booking, desk pools, auto-release, peripheral check-in, occupancy reports and analytics;
  - Places floor maps (IMDF): spaceweave's floor maps stay in spaceweave, and nothing is exported as
    IMDF.

## Consequences

- **Easier:**
  - no per-desk or per-room licence;
  - one owner for every desk booking, check-in and release;
  - sensor and badge evidence can drive release.
- **Harder:**
  - spaceweave must build and maintain the Teams app, the cards and the Outlook add-in;
  - room release cancels part of someone's meeting, so the organiser notice and the audit trail have to
    be clear.
- **Privacy:** reading work plans and presence location is personal data about where people are. It's
  added to the [personal data](../architecture/personal-data.md) DPIA, with sensor and badge
  data.
- **Follow-up:**
  - the tenant tests listed under Constraints;
  - the Calendar sync worker gains directory writes;
  - the Exchange RBAC scope for the Graph app.

## Constraints

- **Licensing can move.** Places features moved from per-user Teams Premium to per-space licences in
  April 2026. spaceweave depends on Places only for the directory and for work location signals, so a
  further change costs little.
- **Graph limits:** `setManualLocation` and the Places check-in endpoint are delegated only, and Places
  bulk upsert is beta.
- **Outlook add-in:** the unified manifest doesn't run on Mac or mobile, which would need a second,
  add-in-only manifest.
- **Still to confirm in a test tenant:**
  - **Declining as the room:** that declining an accepted occurrence as the room frees it, and what the
    organiser and attendees see.
  - **Panel check-ins:** whether a Cisco or Crestron panel check-in changes the calendar in a way
    spaceweave can see.
  - **Application permissions:** whether they work for `setCurrentLocation` and the
    `workHoursAndLocations` read. Microsoft's pages contradict themselves.
  - **Copilot metering:** whether declarative-agent actions are metered for users without Copilot.
  - **Adding a room from Outlook:** whether `enhancedLocation` in the Outlook add-in also adds the room as
    a resource attendee.

## Verification

Planned:
- acceptance tests for room release against a test tenant;
- a check that no desk appears in the Places directory as reservable or drop-in;
- a check that each published floor-plan revision matches the Places directory.

## References

- Places licensing:
  [licence requirements](https://learn.microsoft.com/en-us/microsoft-365/places/what-is-places/license-requirements);
  [Teams Shared Space licences for desks](https://learn.microsoft.com/en-us/microsoft-365/places/what-is-places/buy-set-up-teams-shared-space-licenses-desks);
  [FAQ](https://learn.microsoft.com/en-us/microsoft-365/places/frequently-asked-questions);
  [licensing update announcement](https://techcommunity.microsoft.com/blog/microsoftteamsblog/licensing-updates-extend-access-to-advanced-capabilities-in-microsoft-teams-and-/4488312);
  [Shared Space licence](https://learn.microsoft.com/en-us/microsoftteams/teams-add-on-licensing/teams-shared-device-license)
- Places features:
  - [desks](https://learn.microsoft.com/en-us/microsoft-365/places/room-desk-booking/desks);
  - [desk check-in](https://learn.microsoft.com/en-us/microsoft-365/places/room-desk-booking/enable-checkin-desks);
  - [peripheral check-in](https://learn.microsoft.com/en-us/microsoft-365/places/room-desk-booking/configure-auto-checkin-desk-peripherals);
  - [floor plans](https://learn.microsoft.com/en-us/microsoft-365/places/end-user/configure-floorplans-maps);
  - [Places Finder](https://learn.microsoft.com/en-us/microsoft-365/places/enable-places-finder);
  - [work plans](https://learn.microsoft.com/en-us/microsoft-365/places/configuring-places/enable-work-plans);
  - analytics: [building](https://learn.microsoft.com/en-us/microsoft-365/places/enable-places-analytics/building-analytics),
    [room](https://learn.microsoft.com/en-us/microsoft-365/places/enable-places-analytics/room-analytics-overview),
    [desk pool](https://learn.microsoft.com/en-us/microsoft-365/places/enable-places-analytics/desk-pool-analytics-signals);
  - [integration partners](https://learn.microsoft.com/en-us/microsoft-365/places/places-integration-partners);
  - [New-Place](https://learn.microsoft.com/en-us/microsoft-365/places/powershell/new-place);
  - [bookable desks in Teams Rooms](https://learn.microsoft.com/en-us/microsoftteams/rooms/bookable-desks)
- Microsoft Graph:
  - Places:
    [API overview](https://learn.microsoft.com/en-us/graph/api/resources/places-api-overview?view=graph-rest-1.0);
    [desk](https://learn.microsoft.com/en-us/graph/api/resources/desk?view=graph-rest-1.0);
    [check-ins](https://learn.microsoft.com/en-us/graph/api/place-post-checkins?view=graph-rest-1.0);
    [upsert (beta)](https://learn.microsoft.com/en-us/graph/api/place-patch-places?view=graph-rest-beta)
  - Work location:
    [work hours and locations](https://learn.microsoft.com/en-us/graph/api/workhoursandlocationssetting-get?view=graph-rest-1.0);
    [occurrences view](https://learn.microsoft.com/en-us/graph/api/workhoursandlocationssetting-occurrencesview?view=graph-rest-1.0);
    [set current location](https://learn.microsoft.com/en-us/graph/api/workplanoccurrence-setcurrentlocation?view=graph-rest-1.0)
  - Presence:
    [presence](https://learn.microsoft.com/en-us/graph/api/resources/presence?view=graph-rest-1.0);
    [get presences](https://learn.microsoft.com/en-us/graph/api/cloudcommunications-getpresencesbyuserid?view=graph-rest-1.0)
  - [Activity feed notifications](https://learn.microsoft.com/en-us/graph/teams-send-activityfeednotifications)
- Teams and Outlook:
  - Teams apps:
    [app setup policies](https://learn.microsoft.com/en-us/microsoftteams/teams-app-setup-policies);
    [Universal Actions](https://learn.microsoft.com/en-us/microsoftteams/platform/task-modules-and-cards/cards/universal-actions-for-adaptive-cards/overview);
    [message extensions](https://learn.microsoft.com/en-us/microsoftteams/platform/messaging-extensions/what-are-messaging-extensions);
    [nested app authentication](https://learn.microsoft.com/en-us/microsoftteams/platform/concepts/authentication/nested-authentication)
  - Outlook add-ins:
    [unified manifest](https://learn.microsoft.com/en-us/office/dev/add-ins/develop/unified-manifest-overview);
    [event-based activation](https://learn.microsoft.com/en-us/office/dev/add-ins/develop/event-based-activation)
- Copilot:
  [extensibility prerequisites](https://learn.microsoft.com/en-us/microsoft-365/copilot/extensibility/prerequisites);
  [MCP plugins](https://learn.microsoft.com/en-us/microsoft-365/copilot/extensibility/build-mcp-plugins)
- spaceweave research:
  [integration architecture](../architecture/integration-architecture.md#room-panels);
  [identity and Conditional Access](../architecture/identity.md);
  [personal data](../architecture/personal-data.md)
