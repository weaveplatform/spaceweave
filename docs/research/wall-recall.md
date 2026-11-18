# Wall recall: tracer 10

Measured 18 November 2026 against the same five private CAD packages and 112
raster trials used for tracer 9. The baseline is merged commit `3da7863`.
The selected change recovers 4.11–6.02 percentage points of wall recall on
floors 01–03. The 99% target remains open.

## Cause and implementation

The CAD reader already supplied walls that disappeared during graph cleaning.
The previous raw-reader versus built-plan measurements were:

| Floor | CAD-reader wall recall | Tracer-9 wall recall |
|---|---:|---:|
| 00 | 93.20% | 93.53% |
| 01 | 75.70% | 71.53% |
| 02 | 74.92% | 70.19% |
| 03 | 76.78% | 70.65% |
| 04 | 98.70% | 98.56% |

A private replay, with drawing parsing cached outside the repository, isolated
most of this loss to `mergeCollinear`. A two-segment reduction reproduced it:
adjoining thin and thick walls became one wall with a length-weighted mean
thickness. The thicker run lost its measured faces, and the thinner run gained
an unsupported width.

Changes in [segments.go](../../internal/floorgraph/segments.go) preserve a
thickness junction when known widths differ by more than the existing 25%
similarity limit. Straight graph chains already preserve such junctions.
[build.go](../../internal/floorgraph/build.go) now writes every measured width
that differs from its kind's default after millimetre rounding. Previously,
differences up to 20% were silently replaced by that default. This uses observed
widths; there is no scoring-only thickness allowance.

Preserving the walls initially created 31 extra inferred façade spaces on
floor 03. The grid-space façade filter compared widths between wall
centrelines. It now measures clear width from the unsnapped wall-face polygon,
using the image's x/y calibration and one actual grid cell of width uncertainty.
That reduces the increase to three spaces. Door and label evidence still
protects rooms from the façade filter. Four previously unmatched inferred
spaces disappear on floor 01. Explicit package spaces retain all prior matches.
Source: [spaces.go](../../internal/floorgraph/spaces.go).

## Five-floor results

These are the default `package-none` rows. Wall recall samples source-layer ink
every 50 mm and accepts points within half the traced wall's thickness plus
50 mm of its finite centreline. Precision samples traced centrelines, excluding
openings, against the same source ink. Neither reference data nor tolerances
changed.

| Floor | Wall recall before → after | Change, points | Wall precision before → after |
|---|---|---:|---|
| 00 | 93.53% → 93.45% | −0.08 | 96.83% → 96.90% |
| 01 | 71.53% → 75.64% | +4.11 | 97.79% → 98.03% |
| 02 | 70.19% → 74.77% | +4.58 | 97.53% → 97.73% |
| 03 | 70.65% → 76.67% | +6.02 | 97.85% → 98.02% |
| 04 | 98.56% → 98.68% | +0.11 | 93.96% → 94.23% |

| Floor | Matched / supplied spaces | Predicted spaces before → after | Door recall before → after | Door precision before → after |
|---|---|---|---|---|
| 00 | 62 / 63 | 73 → 73 | 81.20% → 82.05% | 85.98% → 86.11% |
| 01 | 50 / 51 | 56 → 52 | 91.67% → 91.67% | 95.65% → 95.65% |
| 02 | 54 / 55 | 60 → 60 | 96.00% → 96.00% | 97.96% → 97.96% |
| 03 | 53 / 54 | 59 → 62 | 93.33% → 95.56% | 87.50% → 89.58% |
| 04 | 52 / 53 | 59 → 59 | 92.16% → 90.20% | 92.16% → 90.20% |

Space recall stays 271/276 (98.19%) at IoU 0.9; precision changes from
271/307 (88.27%) to 271/306 (88.56%). Source-name matches remain unchanged.
The three additional façade spaces on floor 03 still need review.

One door on floor 04 no longer meets the fixed 150 mm hinge-match tolerance.
Its nearest matching-width opening endpoint is 155.6 mm from the reference
hinge on a 425 mm wall, compared with 0.4 mm before; its width remains correct.
This observation does not adjudicate its physical hinge
position: the source and host need review. The score remains a regression.

## What the remaining misses mean

In a raw-reader diagnostic on floor 01, about 88% of missed reference length
was on one structural layer. Much of that ink is a dense band of repeated
parallel strokes along the façade. The existing reader explicitly suppresses
closely spaced hatching/louvre lines; the current reference collects lines on
the structural layer as wall ink. Visual inspection suggests façade detail,
but that interpretation has not been independently adjudicated.

The next useful trial is a private, reviewed distinction between physical
walls, glazing, columns and façade detail, followed by targeted extraction of
any confirmed missing wall faces. Keep reporting the original all-ink score
alongside that reviewed score. Removing difficult ink from the denominator
would not demonstrate recovered walls. The current 75–77% results must not be
presented as 99% physical-wall accuracy.

## Trials and validation

All 137 configurations pass the site validator: 25 CAD package/architecture
rows and 112 raster rows. Across the raster rows, wall recall never decreases;
14 rows improve, by up to 8.52 points. Wall precision changes in 62 rows,
ranging from −0.19 to +3.41 points. Strict space matches fall by two in each
of the assisted corporate clean/scan face trials, and rise by one in each of
the assisted corporate bold and automatic corporate hatch face trials.
These shared-graph effects remain part of the result.

An alternative merge that retained a common centreline with piecewise widths
was tried and rejected: it recovered less recall on floors 01–03 and retained
the façade-space regression. The existing partial-face continuation option
remains experimental and disabled by default.

Synthetic regressions cover the two-width junction and its sampled width,
small measured-width overrides, thick façade bays, a labelled bay that must
survive, calibrated grid uncertainty, and DXF parsing through graph output.
Tracer version is 10. The two raster golden hashes change; all five CAD/package
hashes remain unchanged. Private drawings, caches, diagnostic renders and
trial plans stay outside the repository.

Reproduce the complete trials with `SPACEWEAVE_DWG_DIR` pointing at the private
fixture layout described in [HANDOFF](../HANDOFF.md):

```sh
SPACEWEAVE_DWG_DIR="$DWG" SPACEWEAVE_EVAL_OUT="$SCRATCH/wall-quality" make quality
GOWORK=off go test -count=1 -run 'TestMergePreservesThicknessChange|TestTracePreservesMeasuredThickness|TestThickFacadeClearWidth|TestFacadeGridWidthUncertainty' ./internal/floorgraph
GOWORK=off go test -count=1 -run TestMeasuredWallJunction ./internal/autotrace/cadfront
```

The complete `make gate` passed on Go 1.27.4: vet, lint, shuffled race tests,
accuracy evaluation, goldens, acceptance tests, 97.7% merged coverage
(13402/13724 statements; every package at least 90%), vulnerability checks and
six CGO-disabled platform builds. All 147 site tests pass. Semantic diagnostics
are clean. Chrome checks pass for all five floors in Plan and both 3D viewing
directions, including footprint rings, overlays and reports, with no uncaught
JavaScript exceptions. The served candidate plans match the scored trial plans.
Runtime CI on the other platforms remains required.
