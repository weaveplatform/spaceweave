# A common workspace-booking domain model

This model merges the data models of the eight vendors and five open-source projects in
this survey. It is analysis, not sourced fact. Each vendor mapping below points to the
§12 "Data model" section of that vendor's profile, where every field is cited. Eptura's
mapping uses its profile, which was validated last.

![common-domain-model data model](../architecture/diagrams/common-domain-model.drawio.svg)

Source: [`../architecture/diagrams/common-domain-model.drawio`](../architecture/diagrams/common-domain-model.drawio). The per-vendor diagrams are in [`diagrams/`](diagrams/).

## What the survey shows

1. **No product exposes the building as geometry.** The richest are:
   - Officely Enterprise: a JSON `floor-manifest/1` with desk rectangles and room or zone polygons ([officely](officely.md#12-data-model-enterprise-from-api-reference)).
   - Seatsurfing: a per-space box, rotation and shape, plus `map_scale` ([open-source](open-source.md)).
   - Archibus, via Eptura: a floor returned as base64 SVG ([eptura](eptura.md)).

   Envoy exposes only normalised desk x/y, and only when a desk is created ([envoy](envoy.md#12-data-model-documented)). Skedda, deskbird and Robin expose no geometry at all, and Clearooms has no public API reference.
2. **Every floor plan starts as a picture or as hand-layered CAD.** No surveyed product documents finding walls, rooms or desks in an unprepared DWG or PDF. Each instead uses one of these:
   - an image upload with hand-placed pins. PDFs count here: Eptura SpaceIQ takes DWG, PDF or PNG and publishes a PNG, and Teem takes PDFs through a submission flow.
   - a staff tracing service: Skedda, Robin, Clearooms
   - CAD with fixed layer conventions: Eptura Workplace, Serraview, Archibus, and the Eptura AutoCAD/Revit plugins. Robin's self-serve DXF import is walls-only.
3. **"Space kind" is modelled inconsistently.**
   - An enum on the bookable thing:
     - Officely: `desk | room | parking_bay | locker | equipment | pass | session | other`
     - Archie: Area `category`
     - deskbird: `flexDesk | parking | other`
     - Envoy: `DESK | ROOM`
   - A venue-defined label over structural categories: Skedda, with `bookable | assigned | addOn` plus a free-text Type.
   - Admin-defined types: LibreBooking.
   - Typed attributes: Seatsurfing.
4. **Bookable and structural are separate concerns.** Officely makes the split explicit: non-bookable `Space` (building, level, zone, room, car park, kitchen) and bookable `Resource`. The others merge the two into one "space" record. That merge is lossy once a meeting room is both a physical room and something people book.
5. **Bookings come as a series plus occurrences.** Skedda has Booking with BookingOccurrence. Robin has `series_id` and `recurrence_id` with RRULE. LibreBooking has `reservation_series` and `reservation_instances`. deskbird's room bookings have type `occurrence | exception | single`.
6. **Integrations are similar everywhere:**
   - calendar rooms paired one-to-one with M365 or Google resource calendars
   - SCIM groups mapped to tags or teams
   - signed webhooks: Skedda and Archie document the signing, and Officely Enterprise has signed webhook endpoints
   - presence from Wi-Fi, badges or sensors as an inbound signal

## The nine pillars

The model is booking first, and is organised as nine pillars, each a column in the diagram. Cards point to
Person, Group and Role in their text (`→ Person`) instead of with drawn lines, because
nearly every card refers to people. Drawing all of those links would hide the structure.
Grey dashed cards are things spaceweave should own because no surveyed vendor offers them.

| # | Pillar | Question it answers |
|---|---|---|
| 1 | **Organisation & branding** | Which tenant is this, and how does spaceweave look to its people? |
| 2 | **Places & floor plans** | Which venue, building and level? Which named spaces are on each floor, and how does a revised drawing carry over? |
| 3 | **Bookable** | What can be used, how it is grouped, and what it can do |
| 4 | **Space Allocation** | Who is given what, in which cycle, on what terms, and by which plan? |
| 5 | **Bookings** | Who uses what, and when: linked bookings, visits, holds, queues, approvals and check-in |
| 6 | **Space Rules & Availability** | What is allowed, how much, and when places are open |
| 7 | **Users & RBAC** | Who people are, who may act for whom, and what they may administer, and where |
| 8 | **Integration** | What crosses the boundary: connected systems and their credentials, sync runs and issues, devices and their signals, API clients |
| 9 | **System Events & History** | What happened, who is told about it, and how long it is kept |

### Why allocation is its own pillar

Assigning people to space is not an attribute of a desk. It is an *event*: a team is
sorted, sized and packed onto zones and desks, a plan is drafted and published, and the
result becomes bookings. The result is a set of owned or shared resources. Those resources
later release days back to the pool, generate bookings automatically, and are re-planned
when teams change or utilisation shifts. Folding this into `BookableResource.assignedTo`
loses the plan, the history and the rules that produce bookings.

The vendors already treat it as a process:
- **Robin:** assigned seats and neighbourhoods are managed in *assignment drafts*. Shared assigned desks are weekly RRULE ownership with exclusions that open `reverse_hoteled` sub-bookings ([robin](robin.md)).
- **Clearooms:** "advanced allocation" goes to a user, a team or guests. Part-time allocation releases days to the hot-desk pool, and priority allocations auto-release ([clearooms](clearooms.md)).
- **deskbird:** dedicated access carries `autoBook` and `allowOthersToBook`. Zones are assigned to groups and users, and per-day scheduling statuses record intent ([deskbird](deskbird.md)).
- **Officely Enterprise:** separate `Assignment`, `AssignmentRelease` and `Delegation` objects ([officely](officely.md)).
- **Envoy:** neighbourhoods are limited to named employees or SCIM groups, with neighbourhood captains, alongside WorkSchedules ([envoy](envoy.md)).

### How allocation works

Allocation works like booking a hotel room. People ask for a *kind* of space, never a
specific one, and a weekly run decides who sits where. The engine is generic, and desks are
its first use.

**Two booking modes side by side.** Each kind has a `booking_mode` on its `KindProfile`,
set per venue or resource group: `self_service` or `allocated`. One venue can run both at
once, for example allocated team floors alongside a self-service hot-desk zone. Rooms, event
spaces, lockers and bike racks are self-service. Parking can be allocated later without any
change to the model.

![Space allocation lifecycle](../architecture/diagrams/space-allocation-lifecycle.drawio.svg)

Source: [`../architecture/diagrams/space-allocation-lifecycle.drawio`](../architecture/diagrams/space-allocation-lifecycle.drawio)

1. **Request.** A `SpaceRequest` names the kind (a desk, for now), the dates, part of day
   and venue, and what the space must have as labels: must-have (`height-adjustable`,
   `accessible`) and nice-to-have (`quiet`, `window`). It can also say "sit near my collaborators" or
   name people, and list companions it also needs (a parking bay, a locker, a bike rack).
   Each request is timestamped, because shortfalls are settled first come, first served.
2. **Cut-off.** The venue's `AllocationCycle` closes intake for next week at its cutoff. Late
   requests wait for a top-up run.
3. **Group.** For each date, requests are clustered into `SeatingGroup`s of collaborators:
   the org team (an Entra ID group), a time-bound delivery team, declared collaborators,
   and, only if the organisation opts in, collaboration inferred from shared meetings
   (`CollaborationInferenceSignal`). The policy sets precedence. For example, while a
   delivery team runs, its members sit with it rather than with their org team.
4. **Pack.** Fixed `Assignment`s and ring-fenced `CollaboratorAllocation` seats come out of supply
   first; released days go back in. Groups are then placed onto levels, zones and desks:
   - **hard:** capacity, must-have labels, `ProximityConstraint`s of type keep-apart, rules and closures, and *required* companions: a request is placed only if they are secured too;
   - **soft:** groups kept together (zone, then floor), home zones, nice-to-have labels, the same desk as last time, fairness, and fewer floors open;
   - **shortfall:** first come, first served; the rest join the waitlist.
5. **Review and publish.** The run writes a draft `AllocationPlan` of `Placement`s, with a
   score and the preferences it could not meet. Publishing turns each placement into an
   *allocated* `BookingOccurrence`, linked to its companions, and tells the person where they
   sit, and why. A published placement is frozen unless the person cancels.
6. **The week.** Daily top-up runs place late requests and the waitlist into freed space.
   Check-in turns *allocated* into *taken*; a no-show releases the desk.
7. **Learn.** `Utilisation` (booked against occupied against no-show) feeds the next cycle's
   policy, for example its overbooking factor.

**The business logic is configuration, not code.** Grouping basis, weights, tie-breaks,
proximity constraints and collaborator ring-fences live in `AllocationPolicy`,
`ProximityConstraint` and `CollaboratorAllocation`. Each is versioned and effective-dated, and every
plan records the policy version that produced it, so an organisation can change its mind
and still explain last month's seating.

Most separation is soft. The hard case is an information barrier: for example, two trading
desks whose client accounts must not be overheard are kept apart by a hard keep-apart
constraint, at zone, floor or venue distance.

**Companions: "I only come if I can park".** A companion is either *optional* or *required*:
- **optional:** the desk is placed regardless, and the bay is a bonus if one is free;
- **required:** the person only travels if they get it, so the request is all or nothing.
  When the request is made, a `Hold` is taken on a bay if one is free, and the bay joins the
  waitlist if none is. The desk is placed only once the bay is secured. On publish, desk and
  bay are tied by a `required` `BookingLink`, so cancelling either cancels both before the
  start.

A self-service hot-desk booking uses the same rule. A desk and bay booked together with a
required link are confirmed together or not at all.

**One waitlist.** Every shortfall, whether a self-service room, a hot desk, a parking bay or
an allocation request the run could not place, is a `WaitlistEntry`. All of them sit in the
same first-come, first-served queue, which top-up runs and released bookings both draw from.

### What floor plans are for

These are booking maps, not architectural drawings. A floor plan matters for three things only:
1. **Turning the organisation's drawing into a bookable inventory:** spaces with their names, and desks, rooms and bays with their positions.
2. **Keeping identity when a revised drawing arrives,** so assignments, bookings, QR codes and external mappings carry over.
3. **Drawing a map people recognise.**

Walls, doors and windows serve only the third, so they are *render layers* of a `FloorPlanRevision`, not domain entities. Nothing books, allocates or reports against a wall.

### Why floor plans need stable identity

If a new drawing mints new ids, every assignment, booking and vendor mapping attached to a
desk breaks. No surveyed vendor documents matching desks and rooms between drawing
revisions. Where pins survive a new drawing, they stay where they were, and someone
corrects them by hand. To keep identity:
- every `Space` and `BookableResource` carries a **stable key**;
- each import is a `FloorPlanRevision` (draft → published → superseded, with an effective date);
- a `RevisionMapping` records what happened to each prior key: unchanged, moved, renamed, split, merged, removed or added, with a confidence and who reviewed it;
- publishing resolves through the mapping, so the resource, its assignments and its bookings carry over, and splits, merges and removals become review items rather than silent breakage.

Standards such as IMDF or IFC are not part of the domain. An indoor-map export may be offered later, and would need its own decision record.

## Proposed entities

### Organisation & branding
| Entity | Purpose | Closest vendor equivalents |
|---|---|---|
| **Organisation** | Tenant and data-residency boundary, with its brand | Skedda Venue; Officely Account/Organisation; Envoy Company; Seatsurfing organizations |
| **Brand** | The organisation's visual identity: display name, logo (light and dark), favicon and app icon, primary and accent colours (contrast checked to WCAG AA). Drafted and previewed, then published | Skedda's white-labelled PWA per venue; branding was not otherwise covered by the survey |

The brand appears wherever the organisation's people meet spaceweave:
- the web and mobile-web app;
- the Teams app and Outlook add-in;
- email and Teams notifications;
- room displays and kiosks;
- printed check-in QR signs.

People should see their organisation's booking app, not spaceweave. A brand is published
deliberately, after a preview, because a bad logo or unreadable colours reach every user at once.

### Places & floor plans
| Entity | Purpose | Closest vendor equivalents |
|---|---|---|
| **Venue / Site** | A site or campus, with an IANA timezone and working hours | Archie Campus/Space; Robin campus; Eptura Site; Officely Place; Envoy Location |
| **Building** | A physical building | Robin building; Eptura Building; Officely Space `type=building` |
| **Level** | One storey, which anchors floor plans | deskbird Floor; Envoy Floor; Robin level; Officely Space `type=level`; Archie Area `category=floor` |
| **FloorPlan** | The plan for one level, pointing at its current published revision | Officely FloorPlan; Seatsurfing `location_floor_plans` |
| **FloorPlanRevision** *(spaceweave)* | An imported drawing with scale, units and origin, and its render layers (walls, doors, windows): draft → published → superseded, with an effective date | Officely's draft, preview and publish, keeping the last 3 versions; Seatsurfing `map_scale`. Elsewhere it is only an image |
| **RevisionMapping** *(spaceweave)* | What happened to each space and resource key between two revisions | None documented |
| **Space** | A named room, zone, area, car park, bike store or locker bank from the drawing, with a stable key, outline and level. Not bookable itself | Officely Space (non-bookable); Clearooms neighbourhood polygons |

### Bookable
| Entity | Purpose | Closest vendor equivalents |
|---|---|---|
| **ResourceGroup** | A logical pool, desk group or neighbourhood, optionally drawn as a zone `Space` | Officely ResourceGroup and pools; Archie desk groups (`table`); Robin desk groups; LibreBooking resource_groups; Envoy Neighborhood; deskbird Zone |
| **BookableResource** | The thing people book: stable key, kind (desk, room, parking bay, bike rack, locker, event space, equipment, other), capacity mode (whole, per seat or pooled), position or footprint, containing `Space`, status | Skedda Asset; deskbird Resource/Room; Officely Resource; Envoy Space/Desk; Robin Desk (Seat); Archie Area; Eptura Engage Resource. Lockers: Robin (multi-day, optional check-in), Officely `type=locker`, Eptura personal-space type. Bike racks: no surveyed vendor documents them |
| **KindProfile** | Booking mode (`self_service` or `allocated`, per venue or resource group) and the capability flags behind each kind: calendar-backed, per-seat, check-in or approval required, vehicle required, accessible. Overridable per resource | Robin's scheduling config is separate from type; Envoy rooms must be calendar resources; Clearooms parking bays (EV, Blue Badge, permit) |
| **Label** | A tag people see and filter by: name, colour, icon, optional label set (Environment, Equipment). It replaces a separate attribute concept: with no value it is a plain tag (`quiet`, `standing`), and with a typed value it is a fact (`screens: 2`). Shown as badges on the floor-plan map, on the resource card, in filters and to assistants; rules can target labels | Skedda tags/type; deskbird equipment; Seatsurfing typed attributes; Robin seat_amenities |
| **ResourceLink** | Rooms that combine, depend on or contain one another | Archie `area_booking_dependencies` and event-space sub-rooms |
| **Inventory** | Counted add-on stock with a per-booking cap | Skedda add-on `quantity` / `maxQuantityPerBooking` |

### Space Allocation
| Entity | Purpose | Closest vendor equivalents |
|---|---|---|
| **AllocationCycle** | A venue's weekly allocation cycle for the kinds it allocates (desks first): horizon (next week), request cutoff, run time, daily top-up runs, and the freeze on published placements | Envoy WorkSchedules and deskbird scheduling statuses collect intent; no cycle entity documented |
| **SpaceRequest** | One person's request for a kind of space (desks first): dates, part of day, venue, must-have and nice-to-have labels, optional sit-near, companions (parking bay, locker, bike rack), each required or optional, and a timestamp for first come, first served. Never a specific resource | Envoy WorkSchedule; deskbird scheduling status (intent only, without desk kind) |
| **CollaborationInferenceSignal** *(opt-in)* | How strongly two people work together, inferred from shared meetings; off unless the organisation opts in. Declared collaborators are on the `SpaceRequest` (sit near), not here | Not covered by the survey |
| **SeatingGroup** | One date's cluster of people to seat together: basis (org team, delivery team, declared, inferred), size, combined label needs, target level or zone | Not covered by the survey |
| **AllocationPlan** | The output of one run (weekly or top-up) for a date: the policy version used, a score, unmet preferences; draft → published → superseded | Robin assignment drafts; Robin space and move planning |
| **Placement** | One line of a plan: person → resource, seating group, labels met and unmet, and an explanation. Placed only if required companions are secured. Publishing creates the booking and links its companions | Not covered by the survey |
| **CollaboratorAllocation** | A group of collaborators' claim on a resource group of any kind. The group is an org team or a time-bound delivery team, and the claim can end with it. Two modes: `ring_fenced` (hard seats the group always gets) or `home_zone` (a soft preference the packer weighs). Named for collaborators, not teams, because people often sit with the delivery team they work in, not their reporting line | Clearooms team neighbourhoods; deskbird zone `assignedGroups`; Envoy SCIM-group neighbourhoods |
| **Assignment** | A resource given to a person, group or guests: always or on an RRULE, with `autoBook` and `allowOthersToBook`. Works for any kind (a personal parking bay or locker, a team's project room); where the kind is allocated, it is removed from supply before packing | Skedda `assigned`; Officely Assignment; deskbird `DEDICATED`; Envoy `assignedTo`; Robin `shared` RRULE; Clearooms part-time and multi-owner |
| **AssignmentRelease** | Days given back: manually, as a part-time gap, or on a missed check-in. Released days go back into supply | Officely AssignmentRelease; Robin RRULE exclusions; Clearooms priority auto-release |
| **Utilisation** | A read model of booked against occupied against no-show, which feeds the next cycle | Skedda Insights; Robin and deskbird analytics |

### Bookings
| Entity | Purpose | Closest vendor equivalents |
|---|---|---|
| **Booking (series)** | Owner (organiser) and, when someone acts for them, the acting delegate and the `Delegation` it was made under; RRULE recurrence; visibility | Skedda Booking; Robin series; LibreBooking reservation_series; Archie Booking.recurrence |
| **BookingOccurrence** | One instance: offset times plus IANA zone, a state machine (held → pending → allocated → checked_in (taken) → released / no_show / cancelled → completed), source (web, assistant, API, allocation, or calendar for bookings made in Exchange or on a room panel). *Allocated* means the resource is yours, however you got it: a self-service booking, an assignment, or an allocation run. *Taken* means you checked in | Skedda BookingOccurrence; deskbird Booking; Envoy Reservation; Officely Booking |
| **BookingLink** | Two or more occurrences held together, such as a desk and a parking bay for one day: `required` (neither confirmed without the other) or `optional`; cancelled together before start | None documented; WARP holds a desk and parking spot in one slot, not as one unit |
| **Beneficiary** | Who a booking is for: a person, a guest or a vehicle | deskbird guest bookings; Clearooms Vehicle IDs |
| **Vehicle** | Plate, type (standard, EV, accessible), permit | Clearooms vehicles and bay types; Envoy parking amenities |
| **Visit** | A guest attached to a host | Envoy Entries/Invites; Robin Visit; Officely Visit; Eptura Engage Visitor |
| **Hold** | A short reservation that expires | Officely Hold |
| **WaitlistEntry** | A place in the one first-come, first-served queue: for a resource, a group, a kind with labels, or a `SpaceRequest` the run could not place. Self-service and allocation share it | Officely Waitlist Entry |
| **Approval** | Gates a request-only booking for an approver role | Officely Approval; Robin request-only spaces; Archie `requires_approval`; LibreBooking `requires_approval` |
| **CheckIn** | Method (QR, Wi-Fi, app, badge, sensor, kiosk, auto), time and source. Auto-release is an event | Robin Confirmation; Skedda and Clearooms check-in methods |

### Space Rules & Availability
| Entity | Purpose | Closest vendor equivalents |
|---|---|---|
| **Rule** | An ordered rule with an effect (allow, deny, limit, require approval) that applies to allocation, booking or check-in, and targets resources, groups, kinds or labels | Officely BookingRule (ordered); Skedda (strictest quota wins); Archie Area policy; deskbird `policyRule` |
| **AllocationPolicy** | The configurable business logic of desk allocation: grouping basis (org team, delivery team, declared collaborators, inferred opt-in) and its precedence, weights (proximity, floor consolidation, stability, fairness), tie-break and shortfall (first come, first served), overbooking factor. Versioned and effective-dated | Not covered by the survey |
| **ProximityConstraint** | Who must or should sit apart or together: between groups or people, `keep_apart` or `keep_near`, hard or soft, at zone, floor, venue or metres. Hard keep-apart covers information barriers | Not covered by the survey |
| **Quota** | A count per period | Skedda quotas; Archie Quota |
| **Entitlement** | Audience → targets, with booking horizon and priority | Robin priority booking for groups; Skedda booking windows |
| **OperatingHours** | Weekly hours for a venue, building, level or resource, which bound when occurrences can be booked | deskbird `openingHours`; Archie `available_times` |
| **Closure** | A dated closure of a scope, which blocks occurrences in that period | Officely Closure |
| **CapacityLimit** | Maximum occupancy or bookings for a scope, which limits how many occurrences it accepts | Envoy `capacityLimit` |

### Users & RBAC
| Entity | Purpose | Closest vendor equivalents |
|---|---|---|
| **Person / Group** | Directory identities and groups. A group is an org team (a SCIM group from Entra ID), a delivery team (time-bound, managed in spaceweave), or another group | Skedda VenueUser/VenueUserTag; Robin SCIM User; deskbird User; Clearooms User/Team |
| **Guest** | A non-directory person with a host | Robin and Envoy guests |
| **Delegation** | Authority for a delegate to act for a principal: book for them, manage their bookings, or manage their allocation. It comes from a calendar delegate grant or is set up in the app, and has validity dates. Bookings record the delegation they were made under, so the owner and the acting person stay distinct | Officely Delegation |
| **Role / Membership** | A role granted to a person or group, scoped to the organisation, a venue, building, level or group. Captains are a scoped role | Officely Role; Skedda tag types (bookingAdmin, owner …); Envoy captains |

### Integration
| Entity | Purpose | Closest vendor equivalents |
|---|---|---|
| **ApiClient** | A third party calling spaceweave: API key or OAuth app, scopes, rate-limit tier | Skedda per-venue static API keys; Officely Account API Tokens (`oat_v1_…`) and PKCE Person tokens; Envoy OAuth2 (24 h access, refresh) |
| **Connector** | A configured link to a calendar, directory (SCIM), vendor, sensor or import source, with direction and health state | Skedda two-way calendar sync; Clearooms Room ↔ calendar; SCIM provisioning at most vendors; Robin Universal Presence Connector |
| **Credential** | The secret a connector or API client uses: OAuth2, API key, SCIM bearer or HMAC secret, with scopes, expiry and rotation. Stored only as a vault reference | Skedda SCIM bearer per venue; Officely's Entra setup token that expires after 7 days; Officely webhook secret rotation |
| **ConnectorMapping** | Local entity and id ↔ external id and version, with state (linked, orphaned, conflicted) | Needed because only Officely Enterprise (floor-plan documents) and Archie (Area PUT) offer a geometry write path |
| **SyncRun** | One sync pass: trigger, cursor in and out, counts and status. Makes sync observable and resumable | None exposed by any surveyed vendor |
| **SyncIssue** | A conflict, validation failure, missing record or rate limit raised by a run, with how it was resolved | None exposed |
| **Device** | A sensor, room display, kiosk, badge reader or dock, installed in a `Space`, with its last-seen time | Robin room displays and Universal Presence Connector; Envoy Butlr sensors and the iPad kiosk; deskbird Dock; Clearooms Room Display; Skedda PWA tablet displays |
| **OccupancySignal** | An observation from a device, typed by a JSON schema, with its granularity (resource, space, level or building) | Officely Signal and SignalType; Robin presence (building or floor level only); deskbird presence networks |

### System Events & History
| Entity | Purpose | Closest vendor equivalents |
|---|---|---|
| **DomainEvent** | The single append-only stream of typed changes (`booking.created`, `assignment.applied` …), with aggregate version, sequence, actor, correlation id and idempotency key | Skedda booking and check-in events; Archie's 21 `.created/.updated/.deleted` events; Envoy entry and desk events; Eptura Engage event types 0–8 |
| **AuditEntry** | Who did what to which record, from where, with before and after | Officely AuditLogEntry (who, where from, outcome, what changed); deskbird booking `history[]` |
| **WebhookSubscription** | An outbound endpoint with an event filter, Standard Webhooks signing, and secret rotation with overlap | Officely WebhookEndpoint (event, `object.*` or `*`; Standard Webhooks; previous secret still signs during rotation); Skedda Standard Webhooks; Archie HMAC + AES-GCM payloads |
| **Delivery** | One attempt to deliver an event: status code, next retry with backoff, dead-lettered, redeliverable | Archie 5 attempts with backoff and in-flight, delivered, failed or not-sent statuses; Eptura Engage `Attempt` count; Officely resendable deliveries |
| **RetentionPolicy** | How long signals, audit entries, bookings and visits are kept, and whether they are then deleted or anonymised, with legal hold | deskbird anonymises personal booking data after 6 months by default; Envoy save or purge for visitor data; Robin keeps anonymised derived data |

### Out of scope: events and ticketing

Running events (organisers, audiences, attendee caps, tickets and admission) belongs to a
separate product. That product books its venue as an ordinary `event_space`
`BookableResource` through the public API, and links external attendees to `Visit`s. The
space-booking model only knows that the event space is booked, and who is visiting.

### Why integration and history are split

The previous single pillar mixed four flows that go in different directions:
- data coming in: calendars, directories, vendors and sensors;
- data going out: webhooks and vendor pushes;
- the record of what happened;
- how long that record is kept.

With the split, each relation reads in one direction:
- **Integration** is what crosses the boundary. Every connector run is a `SyncRun`, so a failing calendar or directory sync is visible and resumable instead of silent. Each problem is a `SyncIssue` with a resolution, not a log line.
- **System Events & History** is what happened. Every state change writes one `DomainEvent`. Audit entries, webhook deliveries and vendor-push connectors all read from that one stream, so they can never disagree.
- **Retention** applies to the data that matters most for employee privacy: occupancy signals and audit entries. It is a policy in the model, not a config file.
- **Devices** are first class and installed in a `Space`, so a signal can be tied to a room or zone rather than only to a floor, which is Robin's documented limitation. Sensors are optional: check-in works without them.

## Design recommendations for spaceweave

- **Treat identity across revisions as the core contract.** Give every space and bookable resource a stable key, and publish revisions through `RevisionMapping`. Make splits, merges and removals review items. This is what lets a revised drawing flow through without breaking bookings, and no vendor documents it.
- **Keep floor plans in their place.** The drawing yields `Space`s and positioned `BookableResource`s. Walls, doors and windows are render layers only. `ResourceGroup` is logical and may be drawn as a zone. One drawing can then feed any vendor's flatter model.
- **Drive behaviour from capabilities, not kind names.** Keep the kind enum for projection (Skedda category + Type, Envoy `DESK | ROOM`, Officely `type`), but let `KindProfile` flags decide check-in, approval, calendar backing and vehicles.
- **Allocate as a weekly batch, from requests for a kind of space, alongside self-service.** Group, then pack, under a versioned `AllocationPolicy`; publish placements as bookings and freeze them. Feed vendors that only have `assignedTo` (Envoy, Skedda) a flattened copy.
- **Evaluate rules in one place.** A single ordered rule set covers allocation, booking and check-in. Quotas and entitlements hang off rules, and availability (hours, closures, capacity) is evaluated in the scope's IANA timezone.
- **Model bookings as series plus occurrences with an explicit state machine.** A desk is allocated when booked and taken when checked in. Holds, waitlists, approvals and linked bookings are first-class, and check-in records its method.
- **Make everything observable.** Every state change writes a `DomainEvent` and an `AuditEntry` in the same transaction. Webhook deliveries are recorded per attempt and can be redelivered. Every connector pass is a `SyncRun` with its issues.
- **Keep secrets and personal data on a short leash.** Credentials live in a vault and the model holds only references, with expiry and rotation tracked. Occupancy signals and audit entries carry a retention policy that deletes or anonymises them on schedule.
- **Treat vendor sync as connectors.** Skedda, deskbird and Robin have no layout write API, and Envoy's desk positions are create-only. Each target is a `Connector` with `ConnectorMapping`s: push where the vendor allows it, and export or CSV elsewhere.
