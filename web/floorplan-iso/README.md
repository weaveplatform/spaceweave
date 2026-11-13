# Isometric floor plan (trial)

A test of whether [D3](https://d3js.org/) can turn a flat 2D floor plan into an
interactive 3D isometric model. The input is RoomSketcher's
[office floor plan sample](https://wpmedia.roomsketcher.com/content/uploads/2021/12/09160214/RoomSketcher-Office-Floor-Plan-PID3529710-2D-bw-with-Labels.jpg),
an 800×460 black-and-white image. The image is linked, not copied, because it is
RoomSketcher's asset.

This is a static site with no build step and no npm dependencies. D3 7.9.0 loads
from jsDelivr, pinned with a subresource-integrity hash.

## Run it

```sh
cd web/floorplan-iso
python3 -m http.server 8000      # then open http://localhost:8000
node --test 'test/*.test.js'     # 21 tests on the projection, geometry and traced data
```

## What you can do

- Morph between the **Isometric**, **Bird's-eye** and **Plan** presets, and rotate in 90° steps.
- Drag the elevation and wall-height sliders. The default wall height is 1.5 m
  because full-height walls hide most of the furniture from an isometric camera.
- Use **Cut away near walls** to drop the exterior walls facing the camera, whichever way it points.
- Hover a room for its zone and area; click it to fly to it. Scroll or drag to zoom and pan.
- Toggle furniture, doors, glazing, room names and people.

## How it works

| File | Role |
|---|---|
| `js/plan.js` | The traced plan in metres: walls with door and window openings, rooms, furniture, people |
| `js/projection.js` | An orbiting parallel camera. Azimuth θ and elevation φ; φ = 90° is the source plan, θ = −45° with φ = 35.26° is true isometric |
| `js/geometry.js` | Walls carved at openings into prisms, furniture built from prisms, visible faces, and draw ordering |
| `js/render.js` | D3: data joins of faces to SVG paths, shading, labels, hover, `d3.zoom` |
| `js/main.js` | State, controls and `d3.transition` tweens (the camera morph and the build-up animation) |

**Tracing.** The plan's dimension chains calibrate the image at 34.3 px/m: the
bottom chain (4.08 + 6.56 + 4.18 + 2.30 + 2.22 m) lands on every wall it names to
within one pixel. Structural walls use the labelled dimensions. Furniture was
measured off a 4× enlargement with a metre grid overlaid.

**Depth.** SVG has no depth buffer, so faces must be painted far to near. Sorting
by a single depth value breaks when a long wall sits beside a small chair. Instead,
every pair of prisms whose screen boxes overlap is ordered by a separating axis
of their footprints, and the result is topologically sorted. Stacked parts, such as
a seat on its post or a lintel over a door, draw bottom up. Wall junctions, where
one wall tucks into another, are allowed a small overlap.

Typical cost: about 350 prisms and 1,200 faces, rebuilt in roughly 3–20 ms per frame.

## Limits of the trial

- The plan was traced by hand. Turning arbitrary plan images into this data model
  automatically (vectorising, then detecting walls, doors and symbols) is the real
  problem behind this trial, and it isn't attempted here.
- Furniture is made of blocks, so round items are 6- to 16-sided prisms.
- The painter's ordering is exact for disjoint convex footprints. Chairs tucked
  under desks fall back to a centre-depth guess, which can be visibly wrong at
  some angles.
- SVG holds up at this size. A floor with ten times as much in it would want a canvas
  renderer: the geometry modules have no DOM dependency, so only `render.js` would change.
