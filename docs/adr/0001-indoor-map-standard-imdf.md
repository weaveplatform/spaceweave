# 0001: Align the spatial model with IMDF and use it as the interchange format

Status: Proposed

## Problem statement

spaceweave turns floor-plan drawings (DWG and PDF) into walls, openings, rooms, desks and
other spaces. That geometry then has to reach booking products, map viewers and wayfinding
tools. We need to decide which standard, if any, the spatial model should follow:

- for the names and structure of our own entities;
- for the format other systems receive.

The wrong choice either locks spaceweave into a vocabulary nobody else reads, or forces
booking concepts into a format that was never designed for them.

## Context

- **No vendor exchanges geometry in a standard format.** The [market survey](../research/market/README.md) covered eight workspace-booking vendors and five open-source projects ([domain model](../research/market/domain-model.md)):
  - None imports or exports IMDF or IndoorGML.
  - Only Archibus (via Eptura) touches IFC.
  - The richest geometry APIs are proprietary: Officely's `floor-manifest/1` JSON, Seatsurfing's box, rotation and map scale, and Archibus floors as SVG.
  - Envoy exposes only normalised desk x/y, and only when a desk is created.
  - A standard export would therefore be a differentiator, not table stakes.
- **The input is CAD in local coordinates.** Drawings use their own origin, units and rotation. They are not georeferenced.
- **Drawings are revised.** The [common domain model](../research/market/domain-model.md) keeps a stable key on each feature across `FloorPlanRevision`s, so assignments and bookings survive a re-import. Any external format has to carry those identifiers rather than mint new ones on each export.
- **The domain needs more than geometry.** Bookable resources, allocation, rules and bookings are spaceweave concepts that no indoor-map standard models.
- **Accuracy target.** Walls, openings, rooms, names and scale should be at least 99% accurate. Whatever format we choose must not lose geometry that we extracted correctly.

### The candidate standards

| Standard | Owner and status | What it models | Encoding |
|---|---|---|---|
| **IMDF** (Indoor Mapping Data Format) | Created by Apple. Published as an OGC Community Standard (OGC 20-094) | Indoor maps for display and wayfinding: `venue`, `building`, `footprint`, `level`, `unit`, `opening`, `fixture`, `section`, `anchor`, `amenity`, `occupant`, `detail`, `kiosk`, `geofence`, `relationship`, `address` | One GeoJSON file per feature type. It conforms to RFC 7946, so coordinates are WGS84 longitude/latitude. Feature ids are UUIDs |
| **IFC** (Industry Foundation Classes) | buildingSMART, ISO 16739-1 | The full building information model (BIM): `IfcSite`, `IfcBuilding`, `IfcBuildingStorey`, `IfcSpace`, `IfcWall`, `IfcDoor`, `IfcWindow`, `IfcFurnishingElement`, and so on | STEP (`.ifc`) or ifcXML / ifcJSON. Solid 3D geometry |
| **IndoorGML** | OGC standard | Indoor space as a navigation network: cells, boundaries, and the dual graph used for routing | GML (XML) |

## Options

### Option A: Our own model, no standard

The model and its export format are defined entirely by spaceweave.

**Pros**
- Complete freedom of naming and structure, with no translation layer.
- Nothing extra to learn, validate or keep up with.

**Cons**
- Every consumer needs a bespoke adapter. This repeats the problem we found in every vendor.
- The names are invented, so new contributors and integrators have nothing to anchor on.
- spaceweave gets no differentiator from a standards-based export.

### Option B: IMDF as the canonical internal model

Store IMDF features directly and treat the GeoJSON as the source of truth.

**Pros**
- Export is free, because the stored form is the exchange form.
- Indoor-map tooling and viewers can read it directly.

**Cons**
- IMDF geometry is in WGS84 longitude and latitude. Storing it that way would georeference every drawing at import time, before anyone knows where the building is. It would also add floating-point drift to geometry measured in millimetres.
- IMDF has no wall solids. Walls exist only as unit boundaries, or as `detail` lines for display. Our extracted walls would lose their meaning.
- IMDF has no concept of bookable resources, revisions, allocation or bookings. These would end up as untyped `properties` or a parallel model anyway.
- IMDF's category lists are closed enumerations. Tenant-specific kinds would have to be squeezed into them.

### Option C: Our own model with IMDF names, and IMDF as the export (recommended)

spaceweave's places and geometry entities take IMDF's names and hierarchy: Venue → Building → Level → Unit, plus Opening, Fixture and Anchor. They stay in drawing coordinates, using the `FloorPlanRevision` coordinate reference (origin, units, scale, rotation). An exporter projects a published revision into an IMDF archive. Booking-domain entities stay spaceweave-only. IFC is a documented mapping for each entity, and a later import path.

**Pros**
- One standard vocabulary for the spatial pillar, and no lossy storage. Geometry keeps millimetre precision and our wall model.
- Stable feature keys become stable IMDF UUIDs, so consumers see the same feature id across revisions. This is the identity contract the domain model already requires.
- Georeferencing happens once at export: one control point plus a bearing per venue. It is not a precondition for importing a drawing.
- The booking domain is free to evolve without breaking the export.
- The IFC mapping (`IfcSpace`, `IfcDoor` and so on) leaves room for a BIM import later, without committing to one now.

**Cons**
- We have to build and test an exporter, with category mapping, coordinate transform and validation.
- Two vocabularies must be kept aligned. That means a mapping table and conformance tests, not just convention.
- Some IMDF features, such as `occupant`, `kiosk` and `geofence`, have no source in a floor-plan drawing. They are left empty or filled from other data.

### Option D: IFC as the canonical model

**Pros**
- The richest standard. It covers walls, doors, windows, spaces and furniture as real building elements.
- Architects and facilities teams already produce IFC from Revit and ArchiCAD.

**Cons**
- Our input is 2D DWG or PDF, not BIM. Producing valid IFC means synthesising 3D solids and a large schema we would barely use.
- Heavyweight to parse and store. Go tooling for it is limited.
- No workspace-booking product consumes it, and only Archibus touches it. It would not help integration.

### Option E: IndoorGML

**Pros**
- Models navigation explicitly. Useful if routing and wayfinding become a product goal.

**Cons**
- Little adoption outside research and niche GIS. No surveyed vendor and no mainstream map platform consumes it.
- XML/GML encoding and a cell-and-graph abstraction that does not fit booking.
- A routing graph can be derived from units and openings later, if it is ever needed.

## Recommendation

**Option C.** It gives spaceweave a standard, recognisable spatial vocabulary and a standard
export, which no vendor in the survey offers, without bending the internal model to fit a
display format:

- IMDF is the only candidate that is lightweight (GeoJSON), openly specified, and consumed by mainstream indoor-map platforms.
- Using it as the export rather than the store avoids its two real problems for us: WGS84 storage and the lack of walls.
- IFC is too heavy to be the core for 2D input, but worth mapping to now so a BIM import stays cheap.
- IndoorGML solves a problem (routing) we do not have yet.

## Decision

Adopt Option C:

1. **Naming.** The places and geometry pillars use IMDF feature names and hierarchy: `Venue`, `Building`, `Level`, `Unit`, `Opening`, `Fixture`, `Anchor`.
   - Each entity also records its IFC counterpart in the model documentation.
   - Two deliberate departures:
     - Resource features are called `Attribute`, because IMDF `amenity` means a point of interest.
     - `Wall` is a spaceweave extension. IMDF has no wall solids.
2. **Storage.** Geometry is stored in the drawing's coordinate reference: origin, units, scale and rotation on `FloorPlanRevision`. It is not stored in WGS84.
3. **Identity.** Each feature's stable key is a UUID that is kept across revisions through `RevisionMapping`. The same UUID is the IMDF feature `id` in every export.
4. **Export.** A published revision can be exported as an IMDF archive:
   - The venue's georeference is set at export time: one control point (longitude, latitude) plus a bearing and scale.
   - spaceweave kinds map onto IMDF categories through an explicit table.
   - Walls are emitted as `detail` lines so viewers can draw them.
   - Bookable resources, allocation and bookings are not part of the export.
5. **IFC.** The IFC mapping is documented now. IFC import or export is future work and needs its own decision record.
6. **IndoorGML.** Not adopted. A navigation graph, if needed, will be derived from `Unit` and `Opening`.

## Constraints

- The IMDF category enumerations and per-feature required properties must be pinned to a specific version of the spec in the exporter. They must be revalidated when we move to a newer version.
- Exports for real buildings need a georeference that a person supplies or confirms. Without one, the exporter must refuse, not guess.
- Test fixtures must be synthetic or licensed drawings. The real floor plans used for accuracy work are private and must never be committed or used as test data in this repository.
- Not yet established, to confirm before implementation:
  - whether the spec's published Validations rules can be checked in CI by an existing tool, or need a validator of our own;
  - how IMDF expects stacked or mezzanine levels (non-integer ordinals) to be handled.

## Verification

Planned. Nothing is implemented yet:
- Exporter unit tests cover the coordinate transform (local → WGS84 round-trip within a stated tolerance), category mapping, and UUID stability across two revisions of the same drawing.
- An acceptance test exports a synthetic multi-level drawing and validates it against the pinned IMDF spec.
- A conformance test fails if a places or geometry entity is added without an IMDF/IFC mapping entry.

## References

- Market survey and common domain model: [`docs/research/market/README.md`](../research/market/README.md), [`docs/research/market/domain-model.md`](../research/market/domain-model.md)
- OGC Indoor Mapping Data Format (IMDF) 1.0.0 Community Standard, OGC 20-094: https://docs.ogc.org/cs/20-094/index.html (validation rules: https://docs.ogc.org/cs/20-094/Validations/index.html)
- RFC 7946, The GeoJSON Format: https://www.rfc-editor.org/rfc/rfc7946
- Apple IMDF documentation: https://register.apple.com/resources/imdf/
- buildingSMART IFC: https://technical.buildingsmart.org/standards/ifc/
- OGC IndoorGML: https://www.ogc.org/standards/indoorgml/
