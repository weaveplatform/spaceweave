# Hand-off notes

For an agent (or person) picking spaceweave up cold. Written 2026-11-17, after
#17 merged. It records what isn't in the code: where the work is going, why,
what's been measured, and the rules the owner has set. Update it when a step
lands.

## What spaceweave is

A Go server and a static microsite (`web/floorplan-iso/`) that turn a floor
plan (image, PDF render, DXF, DWG or SVG) into an interactive isometric model.
Tracing is deterministic and local by default (`internal/autotrace`): the same
file always gives the same plan. A Claude agent tracer exists behind
`tracer: claude` but is optional and not where the work is. Uploads become
drafts that a person approves or sends back. See the [README](../README.md)
for targets and layout.

## First ten minutes

```sh
git switch main && git pull
make gate                                   # vet lint test eval acceptance cover vuln build
(cd web/floorplan-iso && npm test)          # site tests, no dependencies
go run ./cmd/spaceweave serve --addr 127.0.0.1:8010 \
  --output "$SCRATCH/generated" --inbox "$SCRATCH/inbox"
```

- **Use port 8010 for your own server.** The owner often runs one on
  `:8000`. Never stop it, restart it or write to its folders.
- `make gate` takes a while: the race build of `internal/autotrace` alone is
  several minutes. Run the package you're changing first.
- `make build` cross-compiles six platforms and needs a few GB of disk.

## Direction: read meaning first, infer only where it's missing

The tracer used to rebuild a floor in four stages: line work, then walls,
then faces of the wall graph as rooms, then labels. Walls, doors and scale
came out well this way, but rooms stalled at about 60%, and each fix added
another tolerance hack. The owner agreed on 2026-11-17 to change approach.
The reasons:

1. **Physical construction and named space are different things.** A space
   plan's areas include circulation and desk neighbourhoods with no walls
   between them, so no wall graph can produce them.
2. **The client has already drawn the answer.** Room outlines, names and
   areas are in their space plan, and wall and door meanings are on named
   layers. Use those authoritative sources first and infer only the gaps.
3. **Region growing is more robust than face topology.** A face needs every
   wall end to meet, so a 5 cm gap merges two rooms. A flood fill over a
   barrier grid tolerates gaps that small.
4. **The finish line is 99% after a short review, not full automation.**
   Layer roles are detected and then confirmed. Corrections are deterministic
   edits that are re-applied on every trace.

The owner chose four things:
- a **floor package** (several files per floor);
- **walled rooms plus wall-less zones**;
- **small in-page review edits**;
- **layer roles that are detected, then confirmable**.

**Evidence ladder for spaces:** the first source that's available wins.

1. Explicit space data: a registered space plan, area-layer polylines, or room hatches.
2. Label-seeded flood fill over a barrier grid.
3. Wall-graph faces, as the fallback for images, which have no labels without OCR.

### Status

| Step | What | State |
|---|---|---|
| 1 | Rooms can be `kind: zone` (wall-less), with overlap-based site invariants | #13, merged |
| 4 | Layer profiles (built-in plus per-client YAML), a layers panel, `GET/PUT /api/profiles/{name}` | #14, merged |
| 2 | Label-seeded spaces for CAD (`floorgraph/spaces.go`) | #15, merged |
| 3a | Floor packages: several files per plan, space plan registered to the drawing | #16, merged |
| **3b** | **The tracer builds rooms from the registered space plan** | **implemented on `feat/tracing-quality-trials`; measured below** |
| 5 | Review edits (`edits.json`) | to do |
| 6 | Gold-standard evaluation of spaces | to do |

Steps 4, 2 and 3a landed in that order because each later step needed the
earlier one.

## Where the numbers stand

These come from the five real floors: the owner wants these right, and the
synthetic set already scores well. The drawings are private (see
[Private drawings](#private-drawings)).

- **Walls:** 94–98% precision. **Doors:** 81–96% found, with hinge and swing
  about 100%. **Scale:** exact.
- **Rooms** against the space plan's area polygons, after #15:

  | Floor | Recall at IoU 0.9 (at 0.75) | Precision | Rooms | Zones |
  |---|---|---|---|---|
  | 00 | 33.9 (37.5) | 23.2 | 82 | 7 |
  | 01 | 65.1 (72.1) | 50.0 | 56 | 1 |
  | 02 | 66.0 (74.5) | 51.7 | 60 | 2 |
  | 03 | 65.1 (72.1) | 49.1 | 57 | 2 |
  | 04 | 61.0 (68.3) | 41.0 | 61 | 9 |

  - Floor 00 is low because most of its rooms have no label in the drawing.
  - Precision is low because the tracer outputs walled cupboards, risers and
    plant rooms (as "Room N") that the space plan doesn't list.
- **Registration** of each space plan to its drawing (#16):
  - all five floors share coordinates;
  - 84–93% of corners land on walls;
  - the residual is 0.016–0.043 m;
  - when the space plans were deliberately moved 40 m × 25 m, every one was
    fitted back to within 0.046 m.

**The target is 99% or better on walls, openings, rooms, room names and
scale.** The owner's words: it's "not a ci gate … the quality levels need to
be higher". Report every metric per floor and honestly. Don't argue the
target down by citing the literature; find the route that reaches it, and
state plainly what gap remains.

## Step 3b implementation and measured trials

The first implementation merged in PR #19 and tracer-8 refinements in PR #20.
Tracer-9 rendering work merged in PR #21. Tracer-10 wall-recall work is on
`fix/wall-evidence-recall`. Read
[the research and results](research/tracing-quality.md) before further tuning.
`make quality` runs report-only CAD and raster ablations. Private CAD inputs
still need `SPACEWEAVE_DWG_DIR`; output artifacts use `SPACEWEAVE_EVAL_OUT`.

Registered spaces now precede inferred residual floor, keep their kind and
number, and are clipped to the image bounds and voids. They can supply floor
beyond an incomplete wall-derived outline, including when no walls close.
Inferred clipping remainders retain the existing 0.4 m minimum equivalent width;
explicit narrow spaces remain. Smaller source spaces precede
large enclosing zones. Source boundaries are preserved by default: both tested
wall-snapping policies reduced strict polygon matches. Crossings and substantial
clipping are reported for review. Partial packages with one or two outlines
are accepted. Semantic name/number layers exclude generic metadata from names.
Tracer version is now 10. Tracer 9 refreshed the full and partial package
hashes for the optional slab footprint; tracer 10 changes the two raster
hashes and preserves all five CAD/package hashes.

Door leaves now follow their host wall's perpendicular, with correct hinge
selection when wall endpoints run backwards. Registered packages carry a
separate slab footprint with polygons and holes, so a dropped validation outline
does not force a rectangular slab. Footprints preserve declared voids and use
the existing half-exterior-wall allowance in metres. Read the
[rendering results](research/rendering-precision.md) before further tuning.
The new wall-continuation trial improves wall recall but adds facade spaces;
it stays off by default. The later wall-recall measurements are below.

The 99% goal is still open. All-five-floor source-preserving outputs pass the
site validator; room/zone recall is 98.0–98.4% against raw supplied polygons at
IoU 0.9, with precision 84.9–90.0%. Floor 00 improved from 48/63 to 62/63 matches;
44 narrow inferred fragments were removed across the five floors without losing
a match. Each floor retains one unmatched enclosing source outline overlapping
its smaller spaces. Trials expose those conflicts without altering references.
Additional inferred service spaces also reduce precision. These are
one-to-one all-space scores without the old wall exclusion mask, and must not
be compared directly with the room-only table above. No independent gold review
or human correction-time trial has been completed. Raster face recovery remains
opt-in for experiments because its precision regresses on several plans.

Validation: tracer 9's 137 trial configurations pass the Node validator; all
112 raster score rows and the five floors' selected wall/door/space scores remain
unchanged. All 148 site tests and Chrome Plan/3D/reverse-angle checks passed.
The complete final tracer-9 `make gate` passed on Go 1.27.4 (97.7% merged coverage,
every package ≥90%, all six cross-compilation targets, no vulnerabilities).
Semantic diagnostics also pass. The Go toolchain
selection fixes the pre-existing standard-library advisory GO-2026-6859.
The tracer-9 follow-up needs its own cross-platform CI; local cross-compilation
does not replace it. The review server uses port 8010 and the private scratch
catalogue; keep it running when the user is reviewing.

### Wall-recall follow-up: tracer 10

The graph merged adjoining walls with different measured widths and averaged
their thicknesses, losing source faces. It now retains thickness junctions
using the existing 25% similarity rule, and emits every measured width that
differs from its default at millimetre precision. The grid façade filter now
uses clear width before centreline snapping, with one actual grid cell of
uncertainty. See [the full results and limitations](research/wall-recall.md).

| Floor | Wall recall, tracer 9 → 10 | Wall precision, tracer 9 → 10 |
|---|---|---|
| 00 | 93.53% → 93.45% | 96.83% → 96.90% |
| 01 | 71.53% → 75.64% | 97.79% → 98.03% |
| 02 | 70.19% → 74.77% | 97.53% → 97.73% |
| 03 | 70.65% → 76.67% | 97.85% → 98.02% |
| 04 | 98.56% → 98.68% | 93.96% → 94.23% |

All 271/276 source-space matches remain; predictions change 307 → 306.
Floor 01 loses four unmatched inferred spaces; floor 03 gains three façade
spaces still needing review. Door recall improves by one match on each of
floors 00 and 03, but loses one on floor 04 (about 156 mm displacement on a
425 mm host wall, beyond the fixed 150 mm hinge tolerance). Do not hide that
regression or widen the tolerance to remove it.

All 137 configurations validate. Raster wall recall never decreases, but two
assisted corporate face variants each lose two strict space matches. Much of
the remaining CAD reference miss is a dense structural-layer façade band;
its physical meaning needs independent review. The original all-ink reference
and tolerances remain unchanged. The 99% target has not been reached.

Tracer 10's full local `make gate` passed: 97.7% merged coverage, every package
at least 90%, no vulnerabilities and all six platform builds. All 147 site
tests, semantic diagnostics and five-floor Chrome Plan/3D checks pass. The
candidate on port 8011 matches the scored plans; port 8010 retains tracer 9
for comparison. Cross-platform runtime CI remains required.

Next priorities: adjudicate physical walls versus façade detail while retaining
the original score; review the floor-04 hinge and three floor-03 façade spaces;
independently adjudicate overlapping source outlines and missing identities;
implement persistent review edits. The original step 3b checklist follows for
context; its snapping recommendation was superseded by the measured trials.

### Original step 3b checklist

**Where it plugs in.** `autotrace.readSpaces` (`internal/autotrace/autotrace.go`):
- calls `cadfront.ReadSpaces`;
- drops the result when `reg.Poor`;
- otherwise sets `floorgraph.Input.Spaces` (`internal/floorgraph/input.go`,
  type in `space_input.go`: `Polygon`, `Name`, `Number`, `Area`, `Kind`).

**`floorgraph` ignores `in.Spaces` today.** `builder.rooms` (`build.go`) goes
straight to `labelSpaces`, then falls back to `faceRooms`.

**To do:**
1. In `builder.rooms`, use `in.Spaces` first, when present.
   - Snap each edge onto the centreline of the wall it follows. Reuse
     `snapRing` and `alongWall` from `spaces.go`. The plan was to snap within
     300 mm; #15 snaps within half the wall's thickness plus 0.12 m and 12°,
     so measure both.
   - Keep each space's `Kind` (room or zone, which `ReadSpaces` decided from
     how much of its outline runs along walls or openings), and its name and
     number.
   - If snapping makes a ring cross itself (`crosses`), keep the unsnapped
     ring, as #15 does.
2. Fill floor the space plan doesn't cover with label-grown spaces
   (`labelSpaces`), clipped so they don't overlap the explicit ones.
   - `spaceGrid.separate` resolves overlaps in #15.
   - The site invariants from #13 have to hold: spaces don't overlap, stay
     inside the outline, and stay out of voids, each within a slack of
     max(0.25 m², 0.5%).
3. Bump `autotrace.Version` to 7, and regenerate the golden hashes if they
   move: `go test ./internal/autotrace -run Golden -update`, run **without**
   `-race`.
4. Extend `test/acceptance/package_test.go` (`TestFloorPackage`): after
   attaching the space plan, assert that the rooms and names come from it.
   Add synthetic `dxftest` fixtures for:
   - a space plan with rooms the drawing doesn't label;
   - a zone;
   - a space plan that covers only part of the floor.
5. Measure on all five floors with `TestRealCAD`, then put the before and
   after table in the PR.
   - Target with a package: spaces at least 99% at IoU 0.9, and names 100%.
   - Check that walls, doors and scale are unchanged.

## After that

### Step 5: review edits
- **Storage:** `generated/<id>/edits.json`, an ordered list of `{op, target, args}`.
- **Ops:** `merge`, `split` (by a drawn line), `rename`, `zone`, `void`,
  `unvoid`, `kind`.
- **Targets** are geometry anchors (a point inside the room), not ids, so
  they survive a retrace.
- **Applying:** a new `internal/edits.Apply(trace, edits)` runs after the
  deterministic trace and before `trace.Finalize`.
  - An edit whose anchor no longer hits anything is reported in the caveats
    and kept. It is never dropped silently.
- **Page:** in Plan view on a draft, click a room to rename it, change its
  kind or merge it; draw a line to split; toggle a void.
  - Each edit goes to `POST /api/plans/{id}/edits` and causes a retrace.
- **Report:** `report.json` records `edits: n`, and the site credits
  "traced automatically, N corrections".
- **Route conflict:** `server.go` registers `POST /plans/{id}/{action}` for
  review actions, so the new route must either be added before it or be
  handled as a new action.
- **Claude reviewer:** it may propose edits, but it is never required.

### Step 6: evaluation that measures the right thing
- **Gold standard:** hand-verified spaces for floor 02. Start from the
  space-plan polygons and correct them against the architecture and the PDF.
  - Keep it private, outside the repo, next to the drawings.
- **Scoring:** `TestRealCAD` scores spaces against it by IoU, both per kind
  and per name. It also reports:
  - zone coverage;
  - desks against desk-number labels;
  - "edits needed to reach gold".
- **Targets (report-only):**
  - with a package: spaces at least 99% at IoU 0.9, names 100%;
  - after review: gold reached in at most 10 edits per floor.

## Code map

| Path | What |
|---|---|
| `internal/autotrace` | Entry point: picks the CAD or raster front end, reads the space plan, runs `floorgraph`, writes the report. `Version` is stamped into every trace |
| `internal/autotrace/cadfront` | DXF/DWG entities to `floorgraph.Input`: walls (`walls.go`), openings and doorways, voids, furniture, room text, units (`areas.go`), space-plan reading and registration (`spaces.go`) |
| `internal/autotrace/rasterfront` | Pixels to `floorgraph.Input` for images |
| `internal/autotrace/profile` | Layer-role profiles, as embedded YAML plus client files, decoded strictly |
| `internal/autotrace/{score,synth}` | The synthetic eval: renders plans from the hand traces and scores walls and openings |
| `internal/floorgraph` | Wall graph, openings, façade, voids, scale, rooms (`rooms.go` faces, `spaces.go` label-seeded) |
| `internal/plan` | The plan format (`trace.go`, where `Room.Kind` is `room` or `zone`), checks, and conversion to site JSON |
| `internal/trace` | The `Job` and `Spaces` report types, `Finalize`, and the Claude agent tracer |
| `internal/catalog` | Plan store and status machine (`catalog.go`: `Ingest`, `Claim`, `Review`), and floor-package files (`package.go`: `Attach`, `Detach`) |
| `internal/inbox` | Watched folder; groups `Floor 02.dwg` with `Floor 02 space.dwg` |
| `internal/worker` | Claims, converts, traces and publishes. `attachments` passes the package's files to the job |
| `internal/server` | chi routes under `/api`, with origin, host and `X-Spaceweave` checks |
| `internal/{convert,dxf,dwg,svgdraw,cad,vector,raster}` | In-Go readers and renderers. There's no external converter |
| `web/floorplan-iso` | The site: `js/render.js` (isometric), `workbench.js` (drafts and review), `layers.js`, `api.js`. `test/invariants.mjs` holds the site's geometric rules, with a Go copy in `floorgraph` tests |
| `test/acceptance` | Builds the binary with coverage and drives it end to end |

**Retrace path:** a reject, retry or retrace action, or attaching or
detaching a file, goes to `Store.Review` or `Attach`/`Detach`. That re-queues
the plan, `Claim` hands it out again, and `Worker.Process` traces it again
**without converting the drawing again**.

## Conventions

- **[CLAUDE.md](../CLAUDE.md)** applies throughout: the coverage gate (at
  least 95% total and 90% per package, merged across OSes and acceptance),
  golangci-lint as the only linter, chi, cobra without viper, Conventional
  Commits, and comments that explain *why*.
- **When trace output changes:** bump `autotrace.Version` and update the
  golden hashes. When it doesn't, say so in the PR and show that the hashes
  match.
- **Fixtures:** every new rule gets a committed synthetic fixture
  (`internal/dxf/dxftest`, `internal/dwg/dwgtest`). Real drawings never go in
  the repo.
- **Work on a feature branch.** Open a PR into `main` whose body is plain
  English with sections, bullets and result tables, as in #13–#16:
  - what changed and why;
  - results per floor;
  - what was tested;
  - what wasn't run locally, and why.
  - End it with the "Generated with Claude Code" line.
- **After a PR that changes behaviour,** check it in Chrome against your
  `:8010` server: load the five floors, use the new feature, and look at the
  3D view, the overlay and `report.json`. Don't share renders of the real
  floors outside the session.

## Private drawings

The real evaluation set is five floors of one office building, in the private
GitHub repo `deploymenttheory/floorplans`. It holds three zips:
- architectural DWG (AC1032);
- the same floors as PDF;
- CAFM space-plan DWG (AC1015).

**Setup:** clone it into a scratch folder **outside** this repo, then unzip
it so that it has this layout:

```
$DWG/arex/   one architectural DWG per floor
$DWG/space/  one space-plan DWG per floor
```

Floors pair up by the two-digit number at the end of each file name.

**Running the evaluation:**

```sh
SPACEWEAVE_DWG_DIR=$DWG SPACEWEAVE_EVAL_OUT=$SCRATCH/eval \
  go test -count=1 -run 'RealCAD|RealSpaces' -v ./internal/autotrace/cadfront
```

- `SPACEWEAVE_EVAL_FLOORS=02,03` limits the run to the floors listed.
- `SPACEWEAVE_EVAL_OUT` writes a picture of each floor's trace over its
  truth. These images stay local.
- The other `SPACEWEAVE_EVAL_*` switches are debugging dumps. Read
  `realcad_test.go` before using them.

**What's in the drawings:**
- The architectural drawings use Uniclass 2015 / BS 1192 layer names. Most
  doors are loose arcs and polylines, not blocks.
- The space plans have an area layer (about 55 polygons per floor, which are
  the room truth) and a desk layer (about 470).
- The space plans' header says inches, but the coordinates are millimetres.
  `ReadSpaces` fixes the units from the areas written on the plan.

**Never** commit these drawings, copy them into fixtures, publish renders of
them, or name the client or building in commits, PRs or docs.

## Gotchas

- **Disk:** this machine's disk has filled before. Go's build cache (about
  12 GB) is shared with other sessions, so ask the owner before running
  `go clean -cache`.
  - Agent worktrees live under `.claude/worktrees/`. Remove them once their
    PR is open.
- **Windows CI:** under race and coverage, `internal/autotrace` takes 5–8
  minutes on Windows CI, and those runners' speed varies about twofold. The
  `-timeout 25m` in `make test` is deliberate; don't remove it.
- **Race builds:** under `-race`, `TestAccuracy` runs only a subset (every
  plan clean, and every style on the office plan), and `-update` refuses to
  run. The full table runs in `make eval`.
- **Site invariants** exist twice, in `web/floorplan-iso/test/invariants.mjs`
  and in the Go copy in `floorgraph`'s tests. Change both together.

## Known loose ends

- **Poor-fit caveat:** fixed to match `goodMatch = 0.7` (70%).
- **Floor 00:** its rooms are mostly unlabelled. Step 3b should fix this
  floor most of all, since its names come from the space plan.
- **Extra numbered rooms:** walled cupboards and risers become "Room N" even
  when the space plan omits them.
  - Decide with the owner whether these should be kept or folded into a
    neighbouring space once the space plan is used.
- **CI on main:** the quality gate for the #16 merge commit (`1cfa263`) was
  still running when this was written. Check it before starting.

## Build-animation performance follow-up

The owner explicitly wants to keep the full build animation. Improve its frame
pacing; do not skip it or shorten its 2.8-second reveal to make loading appear
faster. Work on `perf/smooth-floor-animation` replaces the all-pairs draw-order
scan, caches stationary footprint comparisons and finished geometry, ranks
cycle-break choices once, and avoids redundant SVG writes. Pending transition
cancellation also releases motion state. Trace output and version do not change.

See [the performance trials](research/build-animation.md). All 150 site tests
and 80 fixed-state SVG comparisons pass, including skins, heights and toggles.
The selected paired run reduces frame-gap p95 by 27–67%, with the complete
animation retained. Heavy floors still miss a consistent 30/60 fps budget;
floor 02 remains a hotspot. An upfront hidden-node pool was rejected after
measurement. Use the new `tools/animation-trial.mjs` for future comparisons;
keep private trial output outside the repository.

The desk-specific follow-up replaces D3's remaining document-wide position
queries with `orderSiblings` in the complete solids layer. The owner reported
smooth walls but stuttering desks; the new trial reports progress before/after
0.42 separately. A dense-floor replay changes 47,746 native position queries
to zero, with exact SVG output preserved. All 152 site tests and 80 SVG checks
pass. See the furniture-phase table in the same performance report: slow
render-JavaScript time falls 49–64% in the recorded pair, while actual frame
gaps improve 17–42%. Heavy furniture frames still miss a reliable 30/60 fps
budget. Keep the complete animation and all furniture detail.

## Desk highlighting

`feat/highlight-desks` adds hover highlighting and single-desk selection to every
rendered `type: desk` furniture object. Top and legs share the original furniture
index as their scene identity; regenerated prism IDs are unsuitable because they
change with styles and builds. Click/tap toggles selection, background click or
Escape clears it, and Tab plus Enter/Space supports keyboard use. Selection
survives camera, style, build and furniture visibility changes; loading another
floor resets it. Desk numbers are display labels, not persisted asset IDs.

`js/desks.js` delegates events once per scene and repaints only affected desk
faces. Keep hover/selection outside the geometry rendering loop. Opaque solid
faces block picking obscured desks; glass and hatch overlays pass through.
Each desk top supplies one keyboard button. Tooltips use text nodes for source
labels. Scene disposal removes its document and pointer listeners.

Chrome trials cover all 1,801 desks (105 / 433 / 451 / 454 / 358 across floors
00–04), actual mouse/keyboard/touch events, drag suppression, room interaction,
six styles, camera changes and rebuilds. All 152 Node tests pass. The complete
2.8-second animation remains; 45 fixed-stage SVG comparisons preserve geometry
and materials, plus 35 comparisons across styles, heights and toggles, excluding
only added desk/accessibility metadata. Settled SVG
writes and native furniture position queries remain zero. The timing pair has
mixed small frame-pacing differences and does not establish a speed improvement.

Repeat the interaction trial against an isolated Chrome debugging port (default
9267), keeping its JSON output outside the repository:

```sh
node web/floorplan-iso/tools/desk-trial.mjs http://127.0.0.1:8011/ "$SCRATCH/desks.json"
```

This branch builds on the accepted animation work in PR #23; merge that first.
Tracing and generated plan data are unchanged by desk highlighting.

## Room wall cutaways

`feat/room-wall-cutaways` extends the existing exterior cutaway with room focus
and a separate cut height. Choose a room in the picker or click its floor;
occluding walls drop away as the camera rotates, while rear walls stay up.
Set the cut height to 0 m to hide those wall assemblies. Fit returns to the
whole-floor exterior policy. Turning Camera cutaways off restores every wall.
Room focus resets on a new floor. The main height slider remains independent.

`js/cutaway.js` tests the room against each wall's camera-projected floor shadow,
subtracting footprint overlap to preserve rear walls on source centrelines.
Results are cached across build frames. Host-wall IDs keep doors and glazing
at the same cut height; void guardrails and furniture are unaffected. Cuts act
on complete source wall segments, which can extend beyond the focused room.

Read [the implementation and measured trials](research/room-cutaways.md).
All 157 Node tests, 25 real-floor camera poses, all 306 room-picker entries and
the 1,801-desk interaction trial pass. Full animation and zero settled SVG
writes/native ordering queries remain. Eight default SVG fingerprints change
only because exterior door leaves now follow their host's cut; the other 37
match the accepted desk build. Tracing and generated data are unchanged.
This branch builds on PR #24 and its animation dependency #23; merge those first.

## Workspace consolidation

`feat/consolidate-workspace` supersedes the interaction and control descriptions
above. The owner approved a View/Review workspace with a separate Import dialog.
Imported floors and examples have separate header selectors; normal viewing no
longer includes upload jobs, layers or raw reports. Review includes actual source
and trace-overlay previews, caveats, approval, processing, source packages and
layer roles. Confidence is kept in Technical details and labelled as unmeasured.

Rooms and desks share selection details and explicit Focus/Clear actions. A room
click only selects. Focus frames the object and identifies its room for cutaways;
Off stays off. Fit changes the camera without clearing the cutaway target. Whole
floor clears the room context. Wall modes state their scope explicitly; absent
targets and irrelevant settings are handled. Keep selection, camera framing and
cutaway room context separate when extending this. Replay animation retains the
complete build; Retrace is a distinct processing action in Review.

Layer edits are staged per floor, with an unload guard and one Apply and retrace
action. Failed saves keep changes; a failed queue request after saving can retry
without another profile write. Pending edits block approval/separate retrace.
Request changes is restricted to the active Claude tracer; automatic tracing
explains that unchanged inputs repeat deterministically. The active model reloads
when a queued trace finishes, without rebuilding on an approval-only update.

Read [the consolidation notes](research/workspace-consolidation.md). All 161 Node
tests, 1,801-desk interactions, 25 cutaway poses and the workspace trial pass.
All 45 default SVG comparisons match `91eeffb`; full animation and cached render
checks remain. The workspace trial intercepts API writes and must never mutate
the private evaluation catalogue or profiles. Screenshots remain private.
This work follows merged PRs #23–#25.
