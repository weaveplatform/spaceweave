# SpaceWeave

SpaceWeave is a space booking application for one organisation's members across
its offices.

## Language

### People and organisation

**Organisation**:
The company whose staff, offices, spaces and bookings SpaceWeave manages.

**Staff**:
People who are members of an organisation using SpaceWeave.
_Avoid_: Employees (membership does not establish an employment relationship)

**Staff identity link**:
The verified association between a staff member and that person's records in
external identity and workforce sources.

**Staff deactivation**:
Withdrawal of a person's access to SpaceWeave while their existing records
remain subject to the organisation's retention rules.

**Leaver**:
A person whose membership of the organisation has ended on its effective date.

**Administrative scope**:
The configurable set of one or more offices an administrator is authorised to
administer within an organisation.

**Administrative capability**:
A function an administrator is authorised to perform within their administrative
scope, such as space configuration, visitor reception, event management or
reporting.

**Capability**:
A specific action or category of actions an actor may be authorised to perform.

**Role**:
A configurable, named set of capabilities.

**Role assignment**:
A grant of a role to a person or application within a defined scope.

**Organisation-wide administrative capability**:
Explicit authority to manage settings affecting the whole organisation, such as
global policies, identity mappings and integrations, beyond office administration.

**Notification preference**:
A person's choices about receiving particular messages and their delivery
methods. Opting in to a private Teams message about a booking being raised does
not opt the person in to other Teams messages.

**Division**:
An organisational unit whose anchor days provide context for in-person
collaboration.

**Team**:
An externally maintained organisational group whose membership informs
proximity-based desk allocation. Staff can belong to multiple teams.

**Visitor**:
A person who is not a member of the organisation they are visiting.
_Avoid_: Guest

**Event visitor**:
A visitor attending or contributing to an organisation's internal event,
including an invited guest speaker.

**Visit**:
A planned or actual occasion when a visitor attends an organisation's premises,
with a named staff host and visit window.

**Staff host**:
The staff member named as the host for a visitor's visit.

### Spaces and locations

**Space**:
A place that can be booked. In-scope kinds are desks, meeting rooms, car parking
and event spaces.

**Space requirement**:
A mandatory space feature that must be satisfied for a booking request to be
eligible for that space.

**Space preference**:
A desired space feature used to choose among spaces that satisfy the request's
requirements.

**Booking**:
An arrangement to use a space on specified dates or at specified times.
_Avoid_: Reservation (the same concept)

**Actor**:
The person or application initiating an action.

**Application actor**:
An organisation-authorised application acting under its own granted authority,
distinct from a person using that application.

**Application grant**:
Explicit authority for an application to perform specified actions within a
defined organisational scope.

**Booker**:
The actor performing a booking action, including an authorised application or a
person acting on another staff member's behalf.

**Booking owner**:
The staff member for whom a booking is made. For an on-behalf-of room booking,
this person is the meeting organiser while the delegate is the booker.

**Booking handover review**:
An assisted review of a leaver's room bookings or events to determine their
continuation and responsible owner.

**Delegated booking**:
A booking made by an authorised human delegate on another staff member's behalf.

**Application-initiated booking**:
A booking initiated by an application under its own grant for a named staff
booking owner.

**Delegate**:
A person authorised to manage bookings on another person's behalf.

**Exchange-derived delegation**:
A delegation relationship inferred from Exchange authority and recognised by
SpaceWeave for bookings.

**App-managed delegation**:
A delegation relationship explicitly granted for use inside SpaceWeave in
addition to Exchange-derived delegation.

**Neighbourhood**:
A configurable bundle of desks and optional meeting rooms within one building,
to which groups are assigned. Its size can flex with demand, and it may span
floors within that building.

**Office**:
A workplace site containing one or more buildings.

**Office calendar**:
An office's local timezone, working hours and closure dates.

**Usual office**:
The work office a staff member normally attends.
_Avoid_: Home office (which can also mean a workspace at home)

**Building**:
A physical building containing floors.

**Floor**:
A level within a building, with its own floor plan.

**Floor plan**:
A representation of a floor's layout.

**Plan revision**:
An immutable version of a drawing or approved floor-plan representation.

**Parking area**:
A location offering bookable parking through specific bays or unassigned
capacity.

**Parking bay**:
An individually bookable vehicle parking space within a parking area.

**Meeting room**:
A space for meetings with a stated capacity and facilities, booked for a start
and end time.

### Desk attendance and allocation

**Anchor days**:
A division's two recurring office days for in-person collaboration.

**Attendance intent**:
A staff member's declaration of the future days they choose to attend the office
and want a desk.

**Allocation cycle**:
An office's defined range of attendance dates planned together, with an intent
cutoff and processing time.

**Intent cutoff**:
The deadline for including attendance intent in a cycle's main allocation run.

**Linked desk and parking request**:
A request associating desk and parking bookings for the same attendance day.
It may require both bookings to be secured together before either is confirmed.

**Allocation group**:
A group of staff formed within SpaceWeave for assignment to a neighbourhood's
desks, separate from externally defined teams.

**Allocation run**:
A scheduled round that considers attendance intents together and forms groups for
each attendance date using team proximity. It assigns those groups to
neighbourhoods and confirms desk bookings.

**Allocation proposal**:
A set of planned desk placements for attendance intents, awaiting validation and
commitment as bookings.

**Allocated desk**:
A desk assigned by the system to a staff member for a specified period.

**Taken desk**:
An allocated desk whose assigned staff member has checked in.

**Desk transfer**:
A booking change securing replacement desk capacity and releasing the previous
allocation together, including required linked parking.

**Neighbourhood adjustment**:
A temporary, dated change moving unallocated desks between neighbourhood
bundles.

**Collaboration request**:
A staff member's request to attend another neighbourhood on specified days to sit
near named collaborators. The requester can explicitly give it priority over
general team proximity.

**Desk waitlist**:
Unfulfilled desk requests awaiting suitable capacity.

**Waitlist offer**:
A time-limited invitation to accept newly available desk capacity.

### Attendance and desk use

**Attendance report**:
A view of staff attendance at work offices, informed by check-ins and occupancy
evidence. Bookings, identified check-ins and occupancy detections are distinct
forms of evidence.

**Desk check-in**:
The assigned staff member's personal confirmation that they are using their desk.

**Occupancy signal**:
Evidence that a desk is physically occupied, separate from its assignee's
check-in.

**Office-presence signal**:
Evidence that a person is present somewhere in an office, without establishing
use of a particular desk.

**Desk release**:
Ending a desk allocation so its capacity returns to the bookable pool.

**No-show**:
A desk allocation whose assigned staff member has not checked in within its
check-in grace period.

**Desk vacancy**:
A taken desk becoming physically unused.

### Events

**Event**:
A scheduled gathering with a venue booking securing its event space and separate
tickets controlling attendance capacity.

**Venue booking**:
The booking securing an event space for an event.

**Event venue recovery**:
The period after an event loses its secured venue while its organiser arranges
a suitable replacement or cancels the event. Existing tickets retain their
entitlements, with admission and further ticket issuance paused until resolution.

**Event audience**:
The people eligible to attend an event, selected by its organiser. It can be
staff-only or include external visitors.

**Registration access**:
An event's rule determining whether eligible attendees need an invitation or may
register publicly.

**Event capacity**:
The maximum number of attendees admitted to an event, no greater than its booked
venue's capacity.

**Event ticket**:
An entitlement for one named attendee to attend a free, capacity-limited event
held in an event space. A visitor's ticket is linked to their visit record.

**Event check-in**:
The admission record created when an attendee's ticket is scanned at event entry.
