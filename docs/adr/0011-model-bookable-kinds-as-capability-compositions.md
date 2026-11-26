# 0011: Model bookable kinds as registered compositions of capabilities over one base resource

Status: Accepted

## Problem statement

spaceweave books desks, rooms, parking bays, lockers, bike racks, event spaces and equipment, and
more kinds will follow, such as phone booths, focus pods or EV chargers. Each kind has facts that the
code behaves on, and facts people search by. We need a model where every kind shares one foundation,
adds only what is specific to it, and can be added over time without reworking the database or every
module. A bad choice either scatters kind-specific logic through the code, or forces a migration and
code changes in several modules for every new kind.

## Context

- **What the model has today** (see the [domain model](../research/market/domain-model.md#bookable)):
  - `BookableResource` has a `kind` enum and a capacity mode.
  - `KindProfile` holds booking mode and capability flags per kind: calendar-backed, per-seat, check-in,
    approval, vehicle required, accessible.
  - `Label` holds tags with optional typed values, which people filter by.
  - `Inventory` holds counted add-on stock.
- **Facts specific to a kind have no typed home.** A parking bay's vehicle types or EV charger, a
  locker's lock type, and a room's Exchange mailbox live in flags, in labels, or nowhere.
- **Search is already a Selector.** Scope, kind and labels are what a `BookingIntent`, a waitlist entry
  and a rule each use to pick resources.
- **Settings already inherit along one physical path:** Venue → Building → Level → Space → resource.
- **The market:**
  - Vendors either hard-code a short list of types (Envoy's `DESK | ROOM`, Officely's `type`) or attach
    free-form attributes (Seatsurfing's typed attributes, Skedda's tags).
  - None documents typed, extensible kinds.
  - The [market research](../research/market/domain-model.md) recommends driving behaviour from
    capabilities, not kind names.
- **Who adds kinds:** spaceweave ships them, in releases. Organisations extend through labels, not new
  typed fields.

## Options

### Option A: One table per kind (class-table inheritance)

A base `bookable_resource` table, plus a table per kind keyed by the resource id.

**Pros**
- PostgreSQL enforces every kind's fields.
- A typed Go struct for each kind.

**Cons**
- Every new kind needs a migration, a new table and new joins in every query that reads it.
- Behaviour shared by several kinds, such as seats or calendar backing, is repeated in each kind's
  table and code.

### Option B: A kind enum with flags (keep the current approach)

Keep `kind` as an enum, with `KindProfile` flags, and put everything else in labels.

**Pros**
- Already in the model; simple.

**Cons**
- Kind-specific facts the code depends on have no typed home, so they end up in labels that admins can
  change.
- A new kind means an `ALTER TYPE` migration, plus flags that grow for every kind.

### Option C: Kinds composed from capabilities, registered in code, with versioned attributes

Behaviour lives in **capabilities**, each written once with its own fields and logic. A **kind** is a
definition, registered in code, that picks capabilities and adds any fields of its own. The resource
stores its kind's fields in a versioned `attributes` document, which is checked against the kind's
schema.

**Pros**
- A new kind that reuses capabilities is one definition and its tests, with no migration.
- Shared behaviour is written once.
- Search covers kind fields through derived system labels, so there is one search language.

**Cons**
- PostgreSQL doesn't enforce kind fields; the Go layer does.
- Changing a kind's schema needs a version and an upgrade function.

## Recommendation

**Option C.** It makes the common case, a new kind built from existing behaviour, a small change, and
it gives every kind-specific fact a typed home. The trade-off accepted is that kind fields are checked
in Go rather than by PostgreSQL. The base fields that every booking depends on keep real columns and
constraints.

## Decision

### Three layers

| Layer | Holds | Who defines it |
|---|---|---|
| **BookableResource** (base) | Key, name, `Space` and position, status, `accessible`, `kind`, `attributes` (versioned). Settings resolved by the cascade below | spaceweave |
| **Kind** | Its capabilities, its own fields, its default settings, its system labels, its icon | spaceweave, in a release |
| **Label** | Descriptive tags people search and filter by, with optional typed values | The organisation |

**The test that separates the layers:** if code branches on a fact, it belongs to a capability or a kind
field. If people only search by it, it's a label.

### Capabilities

| Capability | Fields | Behaviour |
|---|---|---|
| `seated` | seats, per-seat booking | capacity checks; per-seat occupancy |
| `calendar_backed` | Exchange room mailbox | Calendar sync; room release through Graph |
| `vehicle_bound` | vehicle types, height limit, EV charger kW | a vehicle must be on the booking |
| `timed_access` | lock type, code issuing, day or long-term | access codes; end-of-day clearing |
| `combinable` | the parts it combines, or contains | booking the whole blocks its parts, and booking a part blocks the whole (replaces `ResourceLink`) |
| `quantity` | stock, maximum per booking | counted inventory (replaces `Inventory`) |
| `check_in` | window, allowed methods | no-show release |

A resource is booked whole unless a capability says otherwise: `seated` with per-seat booking, or
`quantity`. This replaces the separate capacity mode.

### Built-in kinds

| Kind | Capabilities | Own fields |
|---|---|---|
| Desk | `check_in` | none |
| Room | `seated`, `calendar_backed`, `check_in` | none |
| ParkingBay | `vehicle_bound`, `check_in` | none |
| Locker | `timed_access` | size |
| BikeRack | none | e-bike charging, cargo bikes |
| EventSpace | `seated`, `calendar_backed`, `combinable` | layouts |
| Equipment | `quantity` | none |
| Other | none | none: labels only |

### Kind definitions

- **Each kind is one definition, registered in code.** It declares:
  - name, icon and version;
  - capabilities;
  - its own fields, as a JSON Schema generated from a Go struct;
  - **default settings**, which replace `KindProfile`;
  - **system labels** derived from its fields, such as `seats`, `ev-charging` and `step-free`.
- **Kinds are rows in a reference table,** not a PostgreSQL enum, so a new kind needs no `ALTER TYPE`.
- **A resource's kind is fixed.**
  - A desk never becomes a room.
  - If a new drawing turns desks into a room, `RevisionMapping` records the desks as removed and the room
    as added, and their bookings go to review.

### Settings cascade

A setting such as `booking_mode`, `check_in_required` or `approval_required` resolves from the most
specific value down:

1. a resource override;
2. a selector scope: Space, then Level, then Building, then Venue;
3. the kind's default;
4. the platform default.

Type inheritance (which fields exist) and settings inheritance (what values they have) are separate.
A Neighbourhood takes no part in either.

### Storage

- **`bookable_resource`** keeps real columns and constraints for the base fields, plus an `attributes`
  JSONB column holding the kind's fields and its schema version.
- **The Go layer checks `attributes`** against the kind's schema before every write.
- **When a kind's schema changes,** its version goes up, and an upgrade function rewrites older
  attributes.
- **A kind field that becomes a hot query** gets an expression index or a generated column, in a
  deliberate migration.
- **A capability may own a table** when its fields reference other resources. `combinable` keeps its
  parts in a small table of its own, not in `attributes`, for three reasons:
  - foreign keys to the parts;
  - a fast reverse lookup when a part is booked;
  - a place for `RevisionMapping` to update the keys when a part is renamed, split or removed.
  It's internal to the capability, not a domain entity.

### Search

- A search is a `Selector` over scope, kind and label predicates, plus a time window.
- System labels put kind fields into the same search language. "At least 8 seats" is an int predicate on
  the `seats` system label.
- System labels are read-only, so they can't drift from the field they come from.
- Organisations create their own labels and label sets, but not typed fields or kinds.

### Adding a kind

1. If it needs new behaviour, add a capability, with its fields, logic and tests.
2. Add the kind definition: capabilities, own fields, defaults, system labels and icon.
3. Add a row to the kind reference table, through a data migration.

No table changes are needed unless a field becomes a hot query.

## Consequences

- **Easier:**
  - new kinds, especially ones that reuse capabilities;
  - one place for each behaviour;
  - typed facts for integrations (Exchange mailboxes, vehicles, lock codes);
  - one search language.
- **Harder:**
  - kind fields depend on the Go layer and its tests, not on database constraints;
  - schema versions need upgrade functions;
  - reports that read kind fields go through `attributes` or system labels.
- **Follow-up:**
  - the domain model replaces `KindProfile` with `KindDefinition`, `Inventory` with the `quantity`
    capability, `ResourceLink` with the `combinable` capability, and the capacity mode with capabilities;
  - the floor-plan pipeline tags each imported resource with a kind;
  - the integration architecture's calendar-backed rooms follow `calendar_backed`.

## Constraints

- No PostgreSQL extension is needed: JSON Schema validation runs in Go. Managed PostgreSQL on Azure and
  Google Cloud may not offer a JSON Schema extension.
- The capability list above is a starting set. Whether `accessible` stays a base field or becomes a
  capability with its own entitlement rule is still to decide.
- Exactly which fields each capability carries will be settled as each one is built.

## Verification

Planned:
- **A conformance test for every kind definition:**
  - its JSON Schema matches its Go struct;
  - every system label is derived from a field;
  - its defaults resolve through the cascade.
- **Upgrade tests** from every earlier schema version to the current one.
- **An acceptance test that adds a sample kind** built only from existing capabilities, with no schema
  migration.

## References

- [Domain model](../research/market/domain-model.md), including vendor equivalents for kinds, labels and
  capabilities
- [Floor-plan pipeline](../architecture/floor-plan-pipeline.md)
- [Integration architecture](../architecture/integration-architecture.md)
- [ADR 0010](0010-own-desk-booking-and-release-instead-of-licensing-microsoft-places.md): room release
  through Graph, which `calendar_backed` provides
- [JSON Schema](https://json-schema.org/specification)
- [PostgreSQL JSON types and indexing](https://www.postgresql.org/docs/current/datatype-json.html)
