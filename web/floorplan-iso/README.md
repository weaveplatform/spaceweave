# Isometric floor plan (trial)

A test of whether [D3](https://d3js.org/) can turn a flat 2D floor plan into an
interactive 3D isometric model. Four published plan images were traced by hand;
the Floor plan picker switches between them, and `?plan=` links to one directly.

| Plan | Source | Size | Calibration |
|---|---|---|---|
| Office | [RoomSketcher office sample](https://wpmedia.roomsketcher.com/content/uploads/2021/12/09160214/RoomSketcher-Office-Floor-Plan-PID3529710-2D-bw-with-Labels.jpg) | 19.3 × 10.7 m | Dimension chains, 34.3 px/m |
| Clinic | [SmartDraw physical therapy office](https://www.smartdraw.com/office-floor-plan/img/medical-office-layout.png) | 46.3 × 28 m | No dimensions; door leaves, treadmills and exam tables, 45 px/m |
| Suites | [New Urban Suites, third floor](https://i.pinimg.com/originals/45/48/50/4548508ef40e615e64ad3b9b6535654b.png) | 19.3 × 28.3 m | Graphic scale bar, 92.27 px/m |
| Corporate | [Corporate office test-fit](https://images.squarespace-cdn.com/content/v1/52f9dfd4e4b067fa9e2bb2cb/1394745812848-0EXC0B4JMUTENEE479E1/Picture3.jpg?format=2500w) | 54.8 × 29.2 m | Room-size labels, 38.6 px/m across and 44 px/m down |

The images are linked, not copied, because they belong to their publishers.
Each plan module's header comment records how its scale was measured.

New plans don't need tracing by hand: drop an image on the page (or into a
folder) and the `spaceweave` worker traces it and adds it to the picker as a
draft. See [Adding plans automatically](#adding-plans-automatically).

This is a static site with no build step and no npm dependencies. D3 7.9.0 loads
from jsDelivr, pinned with a subresource-integrity hash.

## Run it

```sh
cd web/floorplan-iso
python3 tools/serve.py           # then open http://localhost:8000 (no-cache dev server)
node --test 'test/*.test.js'     # projection, geometry, every plan's invariants, skin tokens
```

## Adding plans automatically

`spaceweave serve` serves this site with a small API added, and runs the
tracer alongside it:

```sh
# from the repository root; no API key needed
go run ./cmd/spaceweave serve         # then open http://127.0.0.1:8000
```

Under **Add a plan**, drop a plan on the page, choose one, or paste a
screenshot. The list below the drop zone follows each plan through
**Uploaded → Queued → Tracing → Review**. A drawing gets a **Converting** step first
(see [Drawings](#drawings)). When it finishes, the plan joins the
picker marked **Draft**, with its confidence and links to the overlay and the
report. Open it, then:

- **Approve** drops the Draft badge.
- **Send back…** asks what the next pass should fix. The automatic tracer
  gives the same trace again (it doesn't read notes, and says so in the
  caveats); with `tracer: claude` the agent starts from the rejected trace
  with your note as guidance.
- **Retry** appears on a plan that failed.

The server answers only to `localhost`-style host names, and the API refuses
requests that don't come from the page itself, because any page open in the
browser can reach a local server. To reach it from another machine, listen on that interface and name
it: `--addr 0.0.0.0:8000 --allow-host my-laptop.local`.

Under `tools/serve.py` the page has no API, so it shows how to start
`spaceweave serve` instead of the drop zone.

### Drawings

These formats are accepted:

| Format | Notes |
|---|---|
| PNG, JPEG, WebP | Traced as they are |
| DXF | ASCII or binary, any AutoCAD version |
| DWG | AutoCAD 2000, 2004, 2010, 2013 and 2018 formats |
| SVG | Exported from CAD or illustration tools |

The page checks each file before uploading it. The extension has to be one of
these, and the first bytes have to agree with it, so a renamed file or a format
we don't read is refused with a reason and costs nothing. The server checks
again. Common near-misses get specific advice:

- **DWG in another format** (AutoCAD 2007, or Release 14 and older): the
  refusal names the release; save it as DWG 2018 or 2013, or as DXF.
- **PDF**: export the page as PNG, or the drawing as DXF, DWG or SVG.
- **DWF**: export the drawing as DWG or DXF.
- **TIFF, GIF, BMP**: export as PNG.
- **HEIC**: export as JPEG.

PDF support is in progress.

A drawing is rendered to an image here, in Go, before it is traced. Nothing is
sent anywhere for this step. The rendering is black ink on white at up to
4096 px on the long side. Layer colours are dropped so a yellow or cyan layer
doesn't vanish when the tracer thresholds the image. A stray entity far from
the plan is left out of the picture.

**Scale from the drawing.** When a DXF or DWG states its units (`$INSUNITS`), the
rendering's scale is exact. The tracer is told it and holds every submission to
it, so there's no calibrating from dimension strings. When one doesn't state
its units, they're guessed from the drawing's size, and the tracer checks the
guess against a door or a dimension. An SVG measured in millimetres has a paper
scale only, so the tracer calibrates it as usual.

`spaceweave convert` runs the same conversion on its own, which is handy for
seeing what the tracer will see:

```sh
go run ./cmd/spaceweave convert office.dxf              # writes office.png and office.svg
go run ./cmd/spaceweave convert office.dwg              # the same, from a DWG
go run ./cmd/spaceweave convert plan.svg -o plan.png --max-px 2048
```

It prints the size, the scale, and anything it couldn't draw.

### Without the page

`spaceweave watch` runs the same tracer on `floorplans/inbox/` (which `serve`
watches too). Each image copied there is traced and listed in
`generated/catalog.json`; the site polls that file, so the plan appears in the
picker within seconds of the worker finishing. Both folders are git-ignored.

```sh
go run ./cmd/spaceweave watch                       # terminal 1
cd web/floorplan-iso && python3 tools/serve.py      # terminal 2
cp ~/Downloads/some-plan.png floorplans/inbox/      # terminal 3
```

### How plans are traced

By default (`tracer: auto`) plans are traced on the machine running
`spaceweave`, with no model and no API key, and the same file always gives the
same `plan.json`:

- **A DXF or DWG is read as CAD entities.** Layers are recognised by AIA/NCS
  names (`A-WALL`, `A-DOOR`, `A-GLAZ`, …), Uniclass 2015 codes (`EF_25_10`
  walls, `EF_25_30` doors and windows, `Ss_40_15` furniture, …) and common
  words (`WALL`, `MUR`, …), or every layer when none is named. Walls are pairs
  or bundles of parallel lines and polylines; long runs on window layers are
  curtain walling; circles on structural layers are columns. Doors are swing
  arcs (in blocks or loose, including elliptical ones), which give the hinge
  and swing; room names are the room-name text inside each room, not desk or
  room numbers; furniture blocks are matched to the site's furniture by name.
  The scale is exact when the drawing states its units, and areas written on
  the drawing ("23.3 sf", "12 m²") overrule units that contradict them. A
  space-planning drawing with room outlines but no walls is read as an area
  plan, its outlines becoming the walls.
- **An image is read from its pixels.** Walls are the thick strokes, found
  with a distance transform and traced as horizontal and vertical bands.
  Gaps between collinear walls become doors (when a swing arc is drawn),
  windows (thin lines across) or plain openings. The scale comes from the
  drawing when the image was rendered from one, and otherwise from door
  widths (taken as 0.86 m), marked as assumed; with enough doors each axis is
  measured on its own, which catches a squashed image. Walls drawn as two
  faces or hatched are filled in first; slanted walls are read by turning the
  image to their angle; thin partitions count as walls when both ends meet a
  wall.
- **Both** go through the same core: walls are snapped, joined at corners and
  T-junctions, and split into a planar graph; its faces are the rooms; walls
  on the outer face are exterior; rooms are named, zoned and floored from
  their labels. The result passes the same checks as a hand trace (the site's
  validator, `tools/validate.mjs`) before it is published, with a confidence
  computed from the evidence and caveats saying what was guessed.

Accuracy on plan images drawn from the four hand traces (`make eval`, which
also scores each plan drawn thin, bold, scanned, outlined and hatched). Walls
count within 3 px; a room counts when a traced room, or a group of them,
covers it at 90% overlap; doors found also have the right swing and hinge:

| Plan (clean drawing) | Wall recall / precision | Rooms | Openings | Scale error |
|---|---|---|---|---|
| Office | 1.00 / 0.96 | 13 of 13 | 22 of 22 | −1.7% (assumed from doors) |
| Clinic | 1.00 / 0.97 | 36 of 36 | 53 of 53 | −1.8% |
| Suites | 0.97 / 0.86 | 32 of 36 | 49 of 53 | +0.8% |
| Corporate | 0.84 / 0.85 | 25 of 58 | 74 of 95 | +11% |

What the image reader doesn't do yet: room names (images get "Room N" until
OCR lands), lift shafts and risers drawn as free-standing thin boxes, curved
walls, and windows inside walls drawn as outlines (they get filled in).

With `tracer: claude`, a Claude agent traces instead, the way the hand
tracing was done: it views crops with pixel rulers, measures wall faces, and
submits traces until the validator and the overlay are both clean. **Images
are then sent to the Claude API** (Anthropic), so don't drop in anything you
aren't allowed to share with a third-party service, and set
`ANTHROPIC_API_KEY` (or sign in with `ant auth login`).

The review buttons have command-line equivalents:

```sh
go run ./cmd/spaceweave plans list
go run ./cmd/spaceweave plans approve <id>                  # drops the Draft badge
go run ./cmd/spaceweave plans reject <id> --note "The kitchen is missing"
go run ./cmd/spaceweave plans retry <id>                    # re-run a failed plan
go run ./cmd/spaceweave trace plan.png                      # one file, no watcher
```

A rejected plan goes back in the queue. With `tracer: claude` the next run
gets the note and the rejected trace, and corrects that trace rather than
starting over.

Each `generated/<id>/` folder holds these files:

| File | Contents |
|---|---|
| `source.<ext>` | The uploaded image, or the PNG rendered from a drawing |
| `original.<ext>` | An uploaded drawing, as it was sent |
| `drawing.svg` | A drawing rendered as SVG in its layer colours, linked from the plan's blurb |
| `plan.json` | The plan in metres, in the same shape as a `js/plans/*.js` module |
| `trace.json` | The trace, in source pixels |
| `overlay.png` | The trace drawn over the image |
| `report.json` | The tracer, confidence, caveats, calibration evidence, remaining problems, and (Claude only) tokens and cost |
| `transcript.jsonl` | Claude only: one line per agent turn, with its text, tool calls and the results (no images) |

**Cost.** The automatic tracer costs nothing. With `tracer: claude` a trace
runs on Claude Opus 5.5 at high effort. Expect a few dollars
for a simple plan and up to about $15 for a dense floor. Each run stops at
whichever limit it hits first:

- `limits.maxUsd`, $15 by default, worked out from token usage and `prices`;
- 120 turns;
- 12 submissions;
- 45 minutes.

If the run already has a valid trace when it stops, that trace is still
published, with the reason in its caveats. Otherwise the plan is marked failed.

**Configuration.** Settings come from the defaults, then `spaceweave.yaml`, then
`SPACEWEAVE_*` environment variables, then flags. Later sources override earlier
ones. [`spaceweave.example.yaml`](../../spaceweave.example.yaml) lists every key.
Unknown keys are rejected, so a misspelt limit can't be silently ignored.
`tracer` is `auto` (the default) or `claude`; the model, effort, limits and
prices apply only to `claude`. The API key is never a setting: the Anthropic SDK reads `ANTHROPIC_API_KEY` or an
`ant auth login` profile. Node must be on the `PATH`, because every submission
runs `tools/validate.mjs`.

## What you can do

- Morph between the **Isometric**, **Bird's-eye** and **Plan** presets, and rotate in 90° steps.
- Drag the elevation and wall-height sliders. The default wall height is 1.5 m
  because full-height walls hide most of the furniture from an isometric camera.
- Use **Cut away near walls** to drop the exterior walls facing the camera, whichever way it points.
- Hover a room for its zone and area; click it to fly to it. Scroll or drag to zoom and pan.
- Toggle furniture, doors, glazing, room names and people.

## Styles

The Style picker reskins the same geometry. A skin is a block of CSS custom
properties on `.stage-wrap[data-skin="…"]` in `styles.css`, plus a catalogue entry
in `js/skins.js`. The properties cover colours, edge stroke, shading range, an
SVG filter, pencil hatching, corner rounding and facet count. The renderer reads
them with `getComputedStyle`, so adding a skin needs no JavaScript.
`test/skins.test.js` fails if a skin leaves out a token.

| Skin | Look |
|---|---|
| Model | Shaded materials; follows the system's light/dark mode |
| Clay | All-white architectural massing model, terracotta seating, soft ground shadow |
| Blueprint | Hidden-line white linework on blue paper, monospace labels |
| Sketch | Wobbly ink lines from an SVG turbulence filter, pencil hatching, watercolour fills, handwritten labels |
| Neon | Dark, glowing cyan edges, hot-pink wall caps |
| Pastel | Flat isometric illustration with no outlines, bold shading and rounded furniture |

`?skin=sketch` links to a style directly, and the viewer's last choice is
remembered. SVG filters are switched off while the camera moves and come back
when it settles; repainting them every frame would halve the frame rate.

## How it works

| File | Role |
|---|---|
| `js/plans/*.js` | One traced plan per module, in metres: walls with door and window openings, rooms, furniture, people. `index.js` is the picker's catalogue; plans load on demand |
| `js/plans/kit.js` | Shared tracing helpers (openings, furniture shorthands, pixel-to-metre conversion) and the contract a plan module must export |
| `js/projection.js` | An orbiting parallel camera. Azimuth θ and elevation φ; φ = 90° is the source plan, θ = −45° with φ = 35.26° is true isometric |
| `js/geometry.js` | Walls carved at openings into prisms, furniture built from prisms, visible faces, and draw ordering |
| `js/render.js` | D3: data joins of faces to SVG paths, shading, labels, hover, `d3.zoom` |
| `js/main.js` | State, controls and `d3.transition` tweens (the camera morph and the build-up animation) |

**Tracing.** Each plan is traced in source-image pixels and converted to metres
with its `SOURCE` calibration. On the Office plan, the bottom dimension chain
(4.08 + 6.56 + 4.18 + 2.30 + 2.22 m) lands on every wall it names to within one
pixel. The Corporate image turned out to be squashed about 13% horizontally, its
labelled room sizes disagreeing by axis, so `pxPerMetre` may be `[x, y]`.

`tools/overlay.mjs` draws a traced plan back over its source image (walls,
openings, rooms and furniture footprints, with an optional metre grid), which is
how tracing errors were found:

```sh
node tools/overlay.mjs js/plans/pt-clinic.js ~/plans/clinic.png /tmp/clinic.svg --grid
rsvg-convert /tmp/clinic.svg -o /tmp/clinic.png
```

`test/plans.test.js` checks the invariants every plan must meet: rooms inside
the outline, openings on their walls, furniture inside the building, and every
prism projecting to finite screen points.

**Depth.** SVG has no depth buffer, so faces must be painted far to near. Sorting
by a single depth value breaks when a long wall sits beside a small chair. Instead,
every pair of prisms whose screen boxes overlap is ordered by a separating axis
of their footprints, and the result is topologically sorted. Stacked parts, such as
a seat on its post or a lintel over a door, draw bottom up. Wall junctions, where
one wall tucks into another, are allowed a small overlap.

Typical cost: the Office plan is about 350 prisms, rebuilt in 3–20 ms a frame; the
Corporate floor is about 1,100 prisms at roughly 30 ms.

## Limits of the trial

- The four built-in plans were traced by hand. The `spaceweave` worker
  traces new plans automatically, but only as well as it can read them (see
  the accuracy table above), which is why its plans arrive as drafts for a
  person to approve.
- Furniture is made of blocks, so round items are 6- to 16-sided prisms.
- The painter's ordering is exact for disjoint convex footprints. Chairs tucked
  under desks fall back to a centre-depth guess, which can be visibly wrong at
  some angles.
- Curved walls are approximated by straight segments, and the Suites patio
  railing is drawn as full-height glass.
- SVG holds up at this size, though the Corporate floor is near its limit. A floor
  with ten times as much in it would want a canvas renderer: the geometry modules have no DOM dependency, so only `render.js` would change.
