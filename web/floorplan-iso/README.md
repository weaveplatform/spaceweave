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
folder) and the `spaceweave` worker traces it with Claude and adds it to the
picker as a draft. See [Adding plans automatically](#adding-plans-automatically).

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
# from the repository root
export ANTHROPIC_API_KEY=…            # or sign in once with `ant auth login`
go run ./cmd/spaceweave serve         # then open http://127.0.0.1:8000
```

Under **Add a plan**, drop a PNG, JPEG or WebP image on the page, choose one,
or paste a screenshot. The list below the drop zone follows each image through
**Uploaded → Queued → Tracing → Review**, with the agent's turn count and the
money spent so far while it traces. When it finishes, the plan joins the
picker marked **Draft**, with its confidence and links to the overlay and the
report. Open it, then:

- **Approve** drops the Draft badge.
- **Send back…** asks what the next pass should fix. The plan is traced again,
  starting from the rejected trace with your note as guidance.
- **Retry** appears on a plan that failed.

The server answers only to `localhost`-style host names, and the API refuses
requests that don't come from the page itself, because every upload costs
money. To reach it from another machine, listen on that interface and name
it: `--addr 0.0.0.0:8000 --allow-host my-laptop.local`.

Under `tools/serve.py` the page has no API, so it shows how to start
`spaceweave serve` instead of the drop zone.

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

**Uploaded images are sent to the Claude API** (Anthropic) to be traced. Don't
drop in anything you aren't allowed to share with a third-party service.

The agent works the way the hand tracing did. It views crops with pixel rulers,
measures wall faces with ink-run scans, and calibrates each axis separately.
It then submits a trace and reads back two checks: the site's own validator
(`tools/validate.mjs`, the same invariants as `test/plans.test.js`) and the trace
drawn over the image. It repeats until both are clean, then reports a
confidence and the caveats.

The review buttons have command-line equivalents:

```sh
go run ./cmd/spaceweave plans list
go run ./cmd/spaceweave plans approve <id>                  # drops the Draft badge
go run ./cmd/spaceweave plans reject <id> --note "The kitchen is missing"
go run ./cmd/spaceweave plans retry <id>                    # re-run a failed plan
go run ./cmd/spaceweave trace plan.png                      # one image, no watcher
```

A rejected plan goes back in the queue. The next run gets the note and the
rejected trace, and corrects that trace rather than starting over.

Each `generated/<id>/` folder holds these files:

| File | Contents |
|---|---|
| `source.<ext>` | The uploaded image |
| `plan.json` | The plan in metres, in the same shape as a `js/plans/*.js` module |
| `trace.json` | The agent's trace, in source pixels |
| `overlay.png` | The trace drawn over the image |
| `report.json` | Confidence, caveats, calibration evidence, remaining problems, tokens and cost |
| `transcript.jsonl` | One line per agent turn: its text, tool calls and the results (no images) |

**Cost.** A trace runs on Claude Opus 5.5 at high effort. Expect a few dollars
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
Unknown keys are rejected, so a misspelt limit can't be silently ignored. The
API key is never a setting: the Anthropic SDK reads `ANTHROPIC_API_KEY` or an
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

- The four built-in plans were traced by hand. The `spaceweave` worker automates
  the same process with a vision model rather than a vectoriser. It is only as
  good as the agent's reading of the image, which is why its plans arrive as
  drafts for a person to approve.
- Furniture is made of blocks, so round items are 6- to 16-sided prisms.
- The painter's ordering is exact for disjoint convex footprints. Chairs tucked
  under desks fall back to a centre-depth guess, which can be visibly wrong at
  some angles.
- Curved walls are approximated by straight segments, and the Suites patio
  railing is drawn as full-height glass.
- SVG holds up at this size, though the Corporate floor is near its limit. A floor
  with ten times as much in it would want a canvas renderer: the geometry modules have no DOM dependency, so only `render.js` would change.
