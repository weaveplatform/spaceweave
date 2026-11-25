# Diagrams

Each diagram is a draw.io source (`.drawio`) with its SVG export (`.drawio.svg`). The SVG
embeds the diagram, so it also opens in draw.io for editing. They use the dark-tile style of
the infraweave diagrams. Each column groups entities:

- blue: places and hierarchy (in the common model, Places & floor plans)
- purple: bookables
- teal: allocation (common model only)
- pink: bookings and policy (in the common model, Bookings)
- amber container: rules and availability (common model only)
- slate, dashed: people and integration (in the common model, People & access)
- orange: integration, and coral: events & history (common model only)

Amber cards are enums. Grey dashed cards and lines are inferred rather than documented. In
the common model, grey dashed cards are spaceweave-owned, because no surveyed vendor offers
them. Each vendor's profile §12 cites every field.

Relations in the vendor diagrams are amber, or slate dashed when inferred. In the common
model, each relation takes the colour of the pillar it starts from, and the legend lists
the colours. Each relation has a lane of its own, and its label sits on the short segment
leaving its source, so labels are never stacked on a shared line.

## Synthesis

- [common-domain-model](../../architecture/diagrams/common-domain-model.drawio.svg): the merged, booking-first model in eight pillars. It now lives in `docs/research/architecture/diagrams/`.

## Vendors

- [skedda](skedda.drawio.svg): three read-only API resources, with Type as a venue-defined label.
- [deskbird](deskbird.drawio.svg): Office → Floor → Zone → Resource, with rooms separate. Layout is read-only.
- [robin](robin.drawio.svg): the narrow current REST API next to the legacy SDK hierarchy.
- [clearooms](clearooms.drawio.svg): rebuilt from the Zapier app and knowledge base, because no API reference is public.
- [officely](officely.drawio.svg): Officely Enterprise's split between Space and Resource, and its floor-manifest documents.
- [envoy](envoy.drawio.svg): `DESK | ROOM` spaces with normalised, create-only desk positions.
- [eptura](eptura.drawio.svg): the platform portfolio, Engage (Condeco), Workplace (iOFFICE), SpaceIQ and Archibus.
- [archie](archie.drawio.svg): category-typed Areas that carry map fields.

## Open source

- [open-source-seatsurfing](open-source-seatsurfing.drawio.svg): spaces as box, rotation and shape, with a map scale.
- [open-source-librebooking](open-source-librebooking.drawio.svg): resources, types, groups, and reservation series with their instances.
- [open-source-warp](open-source-warp.drawio.svg): an image plan with seats placed at x/y.
- [open-source-mrbs](open-source-mrbs.drawio.svg): areas, rooms and entries.
- [open-source-mensam](open-source-mensam.drawio.svg): desks with position, direction and size.

## Editing

Open the `.drawio` file in draw.io (desktop, diagrams.net, or the VS Code extension) and
change it. Then export the SVG to `<name>.drawio.svg` with "Include a copy of my diagram"
turned on. The SVGs here were exported with draw.io's own renderer, running in the
`jgraph/drawio` container.
