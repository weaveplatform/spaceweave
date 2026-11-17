# Rendering precision: tracer 9

Measured 17 November 2026 against the five private CAD packages and the raster trial suite. Angled-door geometry and package slab footprints are enabled. Wall-continuation recovery remains an explicit experiment. The 99% overall accuracy target is not achieved.

## Selected changes

Door leaves previously extended along a global compass vector, making leaves on angled walls skewed parallelograms. Compass now chooses the side of the host wall; its perpendicular supplies the leaf direction. Reversed wall endpoints also inverted the meaning of opening hinge `a`/`b`: those refer to increasing absolute coordinates, while the renderer's local coordinates follow the wall direction. Hinge selection now accounts for both. Rotation tests cover 11 wall directions, both hinges and both compatible swing sides. Source: [door geometry](../../web/floorplan-iso/js/geometry.js), [regressions](../../web/floorplan-iso/test/geometry.test.js).

An optional `footprint` in trace pixels becomes `FOOTPRINT` in plan metres. It contains polygons, each outer ring followed by holes, so disconnected floor and recesses need no artificial connecting bridge. It is separate from `OUTLINE`, which also constrains walls, rooms and furniture. When a registered package is consumed, its accepted floor and the wall-enclosed floor supply the slab. The existing half-exterior-wall allowance is applied with a robust buffer in metres, then declared voids are subtracted without shrinking them. Unregistered plans keep the existing outline/rectangle fallback. Sources: [package footprint](../../internal/floorgraph/explicit.go), [conversion](../../internal/plan/convert.go).

The renderer draws compound top paths and oppositely oriented inner vertical faces. Camera fitting includes the footprint and wall endpoints. Structural validation covers finite rings; the Node validator also checks hole containment, component overlap, room coverage, open voids and finite projected faces. Sources: [renderer](../../web/floorplan-iso/js/render.js), [geometry](../../web/floorplan-iso/js/geometry.js), [site invariants](../../web/floorplan-iso/test/invariants.mjs).

This footprint is derived from available evidence and a wall-thickness allowance. It has not been independently reviewed as an architectural slab boundary. It does not recover absent walls or establish accurate floor heights.

## Renderer measurements on fixed inputs

The old geometry module from merged tracer 8 (`43a1b74`) and the new module receive the same final plan JSON. This isolates rendering from extraction changes. A leaf counts as non-perpendicular when its longest edge differs from the host-wall normal by more than 0.01°. Rectangularity uses adjacent-edge orthogonality. Measurements are on the geometric prisms before SVG path rounding, not on photographed or independently annotated door positions.

| Floor | Leaves | Non-perpendicular before → after | Non-rectangular before → after | Maximum angle error before → after |
|---|---:|---|---|---|
| 00 | 140 | 114 → 0 | 114 → 0 | 27.987° → <0.000001° |
| 01 | 59 | 51 → 0 | 51 → 0 | 27.992° → <0.000001° |
| 02 | 62 | 52 → 0 | 55 → 0 | 27.997° → <0.000001° |
| 03 | 59 | 54 → 0 | 54 → 0 | 28.000° → <0.000001° |
| 04 | 63 | 54 → 0 | 54 → 0 | 27.995° → <0.000001° |

Thus 325 of 383 leaves exceeded the perpendicularity tolerance before; none do after. Synthetic tests independently check the physical hinge position under wall reversal. The current format still chooses a nominal 90° open leaf; an observed arbitrary opening angle is not represented.

| Floor | Old rendered slab m² | Derived slab m² | Components | Holes |
|---|---:|---:|---:|---:|
| 00 | 7068.25 | 4406.91 | 1 | 2 |
| 01 | 4183.79 | 4183.81 | 1 | 1 |
| 02 | 4166.65 | 4166.68 | 1 | 1 |
| 03 | 6652.07 | 4157.80 | 1 | 1 |
| 04 | 7086.97 | 4304.80 | 1 | 0 |

Floors 00, 03 and 04 no longer acquire large rectangular slab regions when no validation outline survives. Floors 01 and 02 already have an outline, and their small area differences reflect the robust offset and rounding. These area changes diagnose the displayed fallback; they are not a ground-truth slab IoU or a claim that all removed area was independently reviewed. A synthetic L-shaped floor independently verifies that a documented bay is retained and its adjacent recess remains outside, with the expected wall allowance. Separate-part and exact void-retention fixtures accompany it.

## Wall recovery investigation

The initial stage comparison separates front-end extraction from shared graph construction. Most missed wall-ink length on floors 01–03 is on structural layers, and the graph loses a further 4–6 percentage points after extraction. Those bulk losses remain unresolved; the continuation trial addresses one concrete failure rather than claiming to explain the full deficit.

Pairing only part of a wall face previously suppressed the entire remaining face. The experiment retains sufficiently long uncovered spans on eligible wall layers. Each contributing source stroke keeps its identity, so a loose door leaf can be removed after swing recognition even when its face merged with a separate frame stroke. Both failures have small synthetic DXF regressions. `Options.WallContinuations` is off by default and survives automatic file framing. Source: [wall pairing and continuations](../../internal/autotrace/cadfront/walls.go).

| Floor | Reader wall recall % | Default graph recall % | Continuation recall % | Default precision % | Continuation precision % | Predictions default → trial |
|---|---:|---:|---:|---:|---:|---|
| 00 | 93.199 | 93.530 | 93.884 | 96.826 | 96.855 | 73 → 79 |
| 01 | 75.701 | 71.532 | 71.939 | 97.785 | 97.814 | 56 → 56 |
| 02 | 74.921 | 70.190 | 70.698 | 97.527 | 97.552 | 60 → 60 |
| 03 | 76.781 | 70.655 | 72.038 | 97.853 | 97.965 | 59 → 61 |
| 04 | 98.701 | 98.563 | 99.035 | 93.961 | 94.025 | 59 → 60 |

The final continuation trial retains all 271 previous source-space matches, but adds nine predictions. Space precision falls from 84.9% to 78.5% on floor 00, 89.8% to 86.9% on floor 03, and 88.1% to 86.7% on floor 04. Source close-ups reveal facade pockets and door-related geometry among additions; their classification needs further work. Floor 03 gains one matched door (42/45 → 43/45), with precision 87.5% → 89.6%. Other door scores are unchanged. The wall-recall gains therefore do not justify enabling this experiment by default.

## Trials, validation and reproduction

`make quality` now emits 137 rows: 25 CAD configurations (five floors × the four earlier modes plus `package-wall-continuations`) and 112 raster configurations. All 137 validate. Every scored field of the selected CAD mode and all 112 raster rows is unchanged from tracer 8, apart from elapsed time. Current source-space recall remains 98.0–98.4% and precision 84.9–90.0%; these still measure preservation of supplied semantics. Optional plan artifacts are written under `SPACEWEAVE_EVAL_OUT/plans`; private geometry must remain outside the repository.

```sh
SPACEWEAVE_DWG_DIR=/path/to/private-inputs \
SPACEWEAVE_EVAL_OUT=/path/to/private-output make quality

GOWORK=off SPACEWEAVE_DWG_DIR=/path/to/private-inputs \
go test -count=1 -run TestWallStages -v ./internal/autotrace/cadfront

git show 43a1b74:web/floorplan-iso/js/geometry.js > /tmp/spaceweave-geometry-v8.mjs
node web/floorplan-iso/tools/render-quality.mjs "$PLAN_JSON" --geometry /tmp/spaceweave-geometry-v8.mjs
node web/floorplan-iso/tools/render-quality.mjs "$PLAN_JSON"
```

The renderer tool reports counts, angle error and slab geometry without printing private names or coordinates. Its diagnostic tests include an independently supplied skewed polygon. Source: [render-quality tool](../../web/floorplan-iso/tools/render-quality.mjs).

All 148 microsite tests and semantic checks pass. The five original golden plan hashes are unchanged; the two package goldens change for the new footprint field. All five private drafts passed Chrome checks in isometric, reverse-angle and Plan views. Actual compound SVG top paths retain every expected ring with even-odd fill; overlays and reports load, registration is used, and no uncaught browser exceptions were recorded. Screenshots remain private. The complete final local `make gate` passed on Go 1.27.4: vet, lint, race/shuffle, accuracy/goldens, acceptance, coverage (97.7%, 13399/13721 statements; every package ≥90%), no vulnerabilities and all six platform builds. The final automatic-framing option regression is included. Cross-platform execution still needs this follow-up's CI.

Next work: classify facade continuations using stronger physical evidence, isolate structural-layer extraction and graph losses, independently adjudicate enclosing versus net space boundaries, and add persistent review corrections. Renderer consistency improvements do not establish 99% architectural accuracy.
