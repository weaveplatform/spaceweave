# Build-animation frame pacing

Measured 18 November 2026. The owner wants to keep the complete construction
animation and improve its smoothness. Its 2.8-second duration, entrance-based
stagger, growth curve, furniture and geometric detail are preserved.

## Diagnosis and selected changes

The original renderer from `3da7863` took 100–200+ ms on heavy frames. A local
CPU profile across the five floors attributed about 47% of sampled rendering
JavaScript time to draw ordering and 23% to DOM-order checks. Plan downloads
were only 1–4 ms locally. Removing the animation was explicitly rejected.

[geometry.js](../../web/floorplan-iso/js/geometry.js) now finds overlapping
screen boxes with an x sweep, retaining the original pair enumeration order.
It memoises immutable footprint comparisons while the camera is stationary;
height and fallback-depth comparisons remain dynamic during growth. Moving
the camera bypasses initial cache population. New polygon objects and changes
to the wall-junction tolerance invalidate the relevant comparisons.

Dense scenes also required hundreds of cycle breaks in the dependency graph.
The old algorithm scanned the whole floor for the next unfinished item each
time. The new fallback ranks items once, then skips finished entries. Input
indices preserve the original choice when depths tie. Differential tests use
the old pair scan and heap as their oracle, including ties, growth, camera
changes, subsets, stacked parts and wall/furniture tolerance changes.

[render.js](../../web/floorplan-iso/js/render.js) reuses projected prisms and
formatted faces after their heights stop changing. Projection changes discard
that cache; palette and hatching changes rebuild the painted faces. Floors,
slab, tile paths and labels update only when their inputs change. Solid paths
receive only changed attributes and are reordered only when the face sequence
changes. D3's ordinary `join` already performs ordering, so the previous
explicit `order` duplicated that work. See the
[official join documentation](https://d3js.org/d3-selection/joining#selection_join).

[main.js](../../web/floorplan-iso/js/main.js) also releases motion state when a
pending transition is cancelled. Previously it handled only completion and
interruption, leaving the page marked as moving after an early cancellation.

Upfront SVG path preparation and a hidden-node pool were tried and rejected:
setup rose from roughly 35 ms to 134–295 ms without consistently better frame
pacing. They are not included in the selected implementation.

## Measured result

The frozen original and candidate served identical tracer-10 plan JSON in
isolated local catalogues. These are a representative paired run in headless
Chrome 157, 1440 × 1000, Model skin, device scale 1, GPU disabled. Timing is
machine- and load-dependent; repeated trials varied. Lower frame gaps are
better. The 95th percentile describes the slower end of the animation, rather
than the cheap, nearly empty frames at its beginning.

| Floor | Frame-gap p95 before → after | Render-JavaScript p95 before → after | Observed animation frames before → after |
|---|---|---|---|
| 00 | 150.0 → 66.6 ms | 95.8 → 29.0 ms | 56 → 93 |
| 01 | 233.3 → 100.0 ms | 198.8 → 119.8 ms | 47 → 62 |
| 02 | 250.0 → 183.3 ms | 191.5 → 160.8 ms | 50 → 62 |
| 03 | 249.9 → 83.4 ms | 189.3 → 56.5 ms | 49 → 70 |
| 04 | 250.0 → 133.3 ms | 191.5 → 82.0 ms | 55 → 68 |

Frame-gap p95 falls by 27–67% in this run. The nominal reveal still takes
2.8 seconds; long frames previously extended the observed completion time.
The candidate's loading/setup remains about 35–41 ms in this run.

This reduces stutter, but does not establish a consistent 60 fps, or even
30 fps on every heavy frame. Floor 02 remains the largest pacing problem.
Browser painting, path insertion and remaining per-frame work still need
attention if the owner finds the preview insufficiently smooth.

## Correctness and validation

All 150 site tests pass. The browser comparison checks 45 exact SVG hashes:
five floors × three camera poses × three build stages. A further 35 hashes
cover all six skins, three wall heights, visibility controls and returning
to the original style. Paths, draw order, fills, labels and positions match
the frozen renderer at those states. These are deterministic geometry/DOM
checks, not an independent photographic ground truth.

A settled rerender previously caused 12,562 SVG attribute mutations and 59
child-list mutations in the recorded check; the candidate causes zero of
each. Pending-animation cancellation now releases motion state. The browser
trial asserts the full reveal has not been skipped. Chrome Plan/3D checks
also cover all five floors, compound slab rings, overlays and reports.

Only frontend rendering, tests and diagnostics change. Trace data, validator
rules and tracer version do not change, so no extraction golden hashes need
refreshing. The Go gate and CAD/raster extraction trials were not repeated
locally for this frontend-only change. Cross-platform browser/runtime CI is
not replaced by this local Chrome run. Private plans, renders, CPU profiles
and trial output remain outside the repository.

## Reproduce

Start an isolated Chrome debugging session in one terminal:

```sh
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  --headless=new --disable-gpu --no-first-run --no-default-browser-check \
  --remote-debugging-port=9267 --user-data-dir="$SCRATCH/chrome" about:blank
```

Serve the frozen original and candidate against separate copies of the same
ready generated catalogue. In another terminal:

```sh
node web/floorplan-iso/tools/animation-trial.mjs http://127.0.0.1:8012/ "$SCRATCH/baseline.json"
node web/floorplan-iso/tools/animation-trial.mjs http://127.0.0.1:8011/ "$SCRATCH/candidate.json" "$SCRATCH/baseline.json"
cd web/floorplan-iso
npm test
```

The original intentionally fails the redundant-mutation/cancellation assertions
after writing its baseline JSON. Run the second command independently. The
candidate must pass both those checks and the SVG comparison. The tool creates
and closes only its own tab; `SPACEWEAVE_CHROME_DEBUG_PORT` selects another
isolated debugging port. Output contains timings and hashes, not coordinates.
