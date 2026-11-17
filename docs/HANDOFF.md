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

Implemented on `feat/tracing-quality-trials` (not merged when updated). Read
[the research and results](research/tracing-quality.md) before further tuning.
`make quality` runs report-only CAD and raster ablations. Private CAD inputs
still need `SPACEWEAVE_DWG_DIR`; output artifacts use `SPACEWEAVE_EVAL_OUT`.

Registered spaces now precede inferred residual floor, keep their kind and
number, and are clipped to the outline and voids. Smaller source spaces precede
large enclosing zones. Source boundaries are preserved by default: both tested
wall-snapping policies reduced strict polygon matches. Crossings and substantial
clipping are reported for review. Partial packages with one or two outlines
are accepted. Semantic name/number layers exclude generic metadata from names.
Tracer version is 7; the five existing golden hashes stay unchanged, and full
and partial package hashes were added.

The 99% goal is still open. All-five-floor source-preserving outputs pass the
site validator; room/zone recall is 76.2–98.2% against raw supplied polygons at
IoU 0.9, with additional inferred service spaces reducing precision. These are
one-to-one all-space scores without the old wall exclusion mask, and must not
be compared directly with the room-only table above. No independent gold review
or human correction-time trial has been completed. Raster face recovery remains
opt-in for experiments because its precision regresses on several plans.

Validation: complete local `make gate` passed on Go 1.27.4 (97.7% merged
coverage, all six cross-compilation targets, no vulnerabilities), plus 140 site
tests and Chrome Plan/3D checks for all five private floors. The Go toolchain
selection fixes the pre-existing standard-library advisory GO-2026-6859.
Draft PR: #19. Cross-platform execution remains CI's responsibility.

Next priorities: independently adjudicate overlapping source outlines and
missing room identities; diagnose wall-recall losses and unplaced openings;
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
