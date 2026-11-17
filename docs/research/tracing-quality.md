# Deterministic CAD and raster tracing quality

Research, baseline and implementation trials recorded 17 November 2026. The 99% per-floor target is not achieved. The complete local `make gate` passed on Go 1.27.4, including all six cross-compilation targets; total merged coverage is 97.7%.

## Scope and evidence

The accepted scope is local, deterministic Go extraction, with CAD and raster tracks developed together from the outset. Do not introduce local or cloud model inference. Python, Java, C++ and OpenCV projects below are references for portable algorithms, not proposed production dependencies or model trials. Improvements should share geometry and topology logic in `internal/floorgraph` where the evidence is independent of its source; CAD entity interpretation stays in `cadfront`, and pixel interpretation stays in `rasterfront`.

Evaluate automatic extraction and assistance separately. Assistance may supply explicit scale, layer mappings, registration anchors, room seeds or geometric corrections; record exactly which evidence was supplied and keep it reproducible. Do not count a manual correction as an automatic detection. Editor-assisted correction should preserve source coordinates, identifiers and the affected boundary/opening so a subsequent deterministic run can replay it.

The initial source inspection found that both front ends feed `floorgraph.Input`, including wall thickness, openings, labels, scale and optional registered spaces. `autotrace.readSpaces` reads and registers supplementary space outlines and hands them to `Input.Spaces`; the baseline room-building path did not consume them as authoritative room polygons. The label-seeded room path uses nominal 0.05 m cells, a 0.20 m barrier expansion (closing gaps up to approximately 0.40 m), and a second 0.50 m expansion for doorway pockets. Without usable labels it falls back to graph faces. These are existing mechanisms, not new research contributions. Raster input has no OCR labels in the baseline, and registered space assistance is rejected for raster-only input. The measured implementation below adds registered-space consumption and an opt-in raster experiment.

Local evidence: [pipeline](../../internal/autotrace/autotrace.go), [shared input](../../internal/floorgraph/input.go), [room construction](../../internal/floorgraph/build.go), [barrier grid](../../internal/floorgraph/spaces.go), [raster reader](../../internal/autotrace/rasterfront/rasterfront.go), [real-CAD evaluator](../../internal/autotrace/cadfront/realcad_test.go), [registration evaluator](../../internal/autotrace/cadfront/spaces_real_test.go), [raster evaluator](../../internal/autotrace/eval_test.go).

The baseline tables transcribe the supplied CAD and raster logs into anonymized numeric summaries. Private drawing filenames, client names, layer names, source text, coordinates and raw drawings are deliberately omitted. CAD-00 through CAD-04 are five architectural floors; SPACE-00 through SPACE-04 are the corresponding space-plan-only inputs. Public synthetic fixture names are retained. The source logs do not record a commit hash or exact invocation, so their revision provenance remains incomplete. Baseline metrics are preserved separately from the implementation trials, whose different room scorer is explained below.

## Measured CAD baseline

`TestRealCAD` and `TestRealSpaces` passed. A passing evaluation means the run completed and its checks passed; it does not mean extraction reached the quality target.

Percentages below are as printed, rounded to one decimal place. Wall recall measures truth wall-ink length within half the traced wall thickness plus 50 mm; wall precision measures traced centreline length, excluding openings, with wall ink within that distance. Columns are excluded from wall truth. A door match requires an end within 150 mm of the hinge and width within 25% of the leaf; double doors count once. Kind, hinge and swing are conditional on matched doors. See the evaluator legend for the full definitions.

| Floor | Read s | Build s | Wall recall % | Wall precision % | Truth doors | Door recall % | Kind % | Hinge % | Swing % | Door precision % |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| CAD-00 | 3.1 | 3.0 | 93.5 | 96.8 | 117 | 81.2 | 100.0 | 98.5 | 95.8 | 86.0 |
| CAD-01 | 6.7 | 5.2 | 71.5 | 97.8 | 48 | 91.7 | 100.0 | 100.0 | 100.0 | 95.7 |
| CAD-02 | 7.7 | 3.4 | 70.2 | 97.5 | 50 | 96.0 | 100.0 | 100.0 | 100.0 | 98.0 |
| CAD-03 | 9.5 | 4.0 | 70.7 | 97.9 | 45 | 93.3 | 100.0 | 100.0 | 100.0 | 87.5 |
| CAD-04 | 7.4 | 2.5 | 98.6 | 94.0 | 51 | 92.2 | 100.0 | 100.0 | 100.0 | 92.2 |

An enclosed truth area has at least 90% of its boundary within 250 mm of wall, glazing or door ink. Room recall uses IoU ≥ 0.9; the second recall column uses ≥ 0.75. Architectural-floor comparisons exclude points within a traced wall thickness from both polygons to accommodate face-versus-centreline conventions. Boundary recall is the fraction whose boundary and best traced room boundary are on average within 200 mm in both directions. Room precision counts traced rooms matching enclosed truth areas at IoU ≥ 0.9. Open coverage is coverage of non-enclosed truth area; zone recall uses best-zone IoU ≥ 0.5 on a 200 mm grid. These metrics have different denominators and must not be conflated.

| Floor | Truth areas | Enclosed | Room recall .9 % | Room recall .75 % | Boundary recall % | Room precision % | Traced rooms | Open coverage % | Zones | Zone recall % | Zone coverage % |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| CAD-00 | 63 | 56 | 33.9 | 37.5 | 33.9 | 23.2 | 82 | 94.2 | 7 | 0.0 | 94.2 |
| CAD-01 | 51 | 43 | 65.1 | 72.1 | 65.1 | 50.0 | 56 | 98.9 | 1 | 12.5 | 86.8 |
| CAD-02 | 55 | 47 | 66.0 | 74.5 | 66.0 | 51.7 | 60 | 99.0 | 2 | 12.5 | 86.1 |
| CAD-03 | 54 | 43 | 65.1 | 72.1 | 62.8 | 49.1 | 57 | 98.3 | 2 | 9.1 | 87.2 |
| CAD-04 | 53 | 41 | 61.0 | 68.3 | 65.9 | 41.0 | 61 | 98.0 | 9 | 8.3 | 83.9 |
| SPACE-00 | 63 | 63 | 31.7 | 66.7 | 68.3 | 42.6 | 47 | — | 0 | — | — |
| SPACE-01 | 51 | 51 | 52.9 | 80.4 | 80.4 | 57.4 | 47 | — | 2 | — | — |
| SPACE-02 | 55 | 55 | 49.1 | 83.6 | 81.8 | 54.0 | 50 | — | 2 | — | — |
| SPACE-03 | 54 | 54 | 48.1 | 81.5 | 79.6 | 53.1 | 49 | — | 2 | — | — |
| SPACE-04 | 53 | 53 | 37.7 | 79.2 | 75.5 | 42.6 | 47 | — | 2 | — | — |

SPACE-only wall and door truth is unavailable: the log prints zero wall/door values and zero truth doors. Those zeroes are not measurements of architectural detection quality. SPACE read/build times were respectively 0.4/0.2, 0.4/2.1, 0.5/1.2, 0.3/0.9 and 0.3/1.1 seconds.

Name accuracy is conditional on rooms matched at IoU ≥ 0.75 and having a truth name in the source being traced. The denominator is especially small on some architectural floors. Furniture placement checks bounding-box overlap/size criteria; it is not a complete semantic classification score. Scale is traced pixels/metre divided by truth pixels/metre. Every row finalized successfully.

| Floor | Name accuracy % / eligible rooms | Scale ratio | Furniture placement % / truth inserts | Finalization |
|---|---:|---:|---:|---|
| CAD-00 | 100.0 / 2 | 1.0000 | 78.9 / 730 | ok |
| CAD-01 | 100.0 / 4 | 1.0000 | 85.5 / 1073 | ok |
| CAD-02 | 100.0 / 17 | 1.0000 | 83.1 / 1078 | ok |
| CAD-03 | 100.0 / 6 | 1.0000 | 85.2 / 1075 | ok |
| CAD-04 | 100.0 / 8 | 1.0000 | 82.5 / 1100 | ok |
| SPACE-00 | 83.3 / 42 | 1.0000 | unavailable | ok |
| SPACE-01 | 95.1 / 41 | 1.0000 | unavailable | ok |
| SPACE-02 | 87.0 / 46 | 1.0000 | unavailable | ok |
| SPACE-03 | 88.6 / 44 | 1.0000 | unavailable | ok |
| SPACE-04 | 85.7 / 42 | 1.0000 | unavailable | ok |

The evaluator classifies failures below only when IoU is below 0.9 **and** boundary error exceeds 200 mm. These counts therefore need not sum to all missed IoU matches.

| Floor | Truth area inside a larger traced room | Truth area split | Offset | In no room |
|---|---:|---:|---:|---:|
| CAD-00 | 16 | 10 | 3 | 8 |
| CAD-01 | 9 | 2 | 3 | 0 |
| CAD-02 | 8 | 2 | 6 | 0 |
| CAD-03 | 9 | 3 | 3 | 0 |
| CAD-04 | 7 | 4 | 2 | 1 |
| SPACE-00 | 2 | 6 | 2 | 10 |
| SPACE-01 | 3 | 5 | 2 | 0 |
| SPACE-02 | 2 | 5 | 3 | 0 |
| SPACE-03 | 5 | 4 | 2 | 0 |
| SPACE-04 | 3 | 4 | 5 | 1 |

### Existing space registration baseline

The supplementary outlines already register successfully on all five floors. The original inputs were classified as sharing coordinates, with `poor=false`; independently moving the placement by 40 m and 25 m was recovered by the fitted path with `poor=false`. Corner-on-wall share is not room IoU. The source log records a unit inconsistency resolved using written polygon areas, which reinforces the need to retain scale evidence rather than blindly trust nominal CAD units.

| Floor | Spaces | Numbered or named | Zones | Shared corners on walls % | Shared residual m | Moved-fit RMS m | Moved corners on walls % | Moved residual m |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 00 | 63 | 60 | 14 | 93.0 | 0.043 | 0.046 | 93.2 | 0.047 |
| 01 | 51 | 48 | 9 | 84.0 | 0.018 | 0.020 | 84.0 | 0.020 |
| 02 | 55 | 50 | 8 | 86.7 | 0.016 | 0.009 | 86.7 | 0.016 |
| 03 | 54 | 50 | 11 | 85.3 | 0.043 | 0.014 | 85.0 | 0.040 |
| 04 | 53 | 48 | 15 | 90.4 | 0.031 | 0.017 | 90.4 | 0.031 |

## Measured raster baseline: all 28 rows

`TestAccuracy` passed all 28 cases. These are synthetic renders of four hand traces, with clean, scan, thin, bold, outline, hatch and outline-scan variants. They demonstrate controlled rendering sensitivity, not accuracy on arbitrary scanned drawings or the five real CAD floors. No real-raster score is established by this baseline.

Wall R/P measures wall-length recall/precision within 3 px. Opening K+P is truth openings matched with the same kind within 8 px on a parallel wall; Found accepts any kind. Swing and hinge are conditional counts for matched doors. Got is traced room count; Exact is truth rooms matched at IoU ≥ 0.9. Covered additionally accepts grouped matches where an open area was subdivided differently. Area R/P measures total floor coverage and cannot replace room identity/topology accuracy. Scale is percentage error on the worse axis. Confidence is the tracer's heuristic score, not calibrated accuracy.

| Fixture | Variant | Wall R/P | Opening K+P | Found | Swing | Hinge | Got | Exact | Covered | Area R/P | Named | Scale error | Confidence |
|---|---|---|---|---|---|---|---:|---|---|---|---|---:|---:|
| office | clean | 1.00 / .96 | 22/22 | 22/22 | 12/12 | 8/8 | 11 | 9/13 | 13/13 | 1.00 / 1.00 | 0/13 | -1.7% | .73 |
| office | scan | 1.00 / .93 | 22/22 | 22/22 | 12/12 | 8/8 | 11 | 9/13 | 13/13 | 1.00 / 1.00 | 0/13 | -1.7% | .72 |
| office | thin | .76 / .90 | 12/22 | 12/22 | 7/7 | 4/4 | 1 | 0/13 | 6/13 | .16 / 1.00 | 0/13 | -1.7% | .58 |
| office | bold | 1.00 / .96 | 20/22 | 20/22 | 12/12 | 8/8 | 10 | 8/13 | 12/13 | .97 / 1.00 | 0/13 | -2.5% | .72 |
| office | outline | 1.00 / .88 | 12/22 | 12/22 | 12/12 | 8/8 | 12 | 8/13 | 13/13 | 1.00 / 1.00 | 0/13 | -10.2% | .71 |
| office | hatch | 1.00 / .89 | 12/22 | 12/22 | 12/12 | 8/8 | 11 | 9/13 | 13/13 | 1.00 / 1.00 | 0/13 | -10.2% | .71 |
| office | outline-scan | 1.00 / .91 | 12/22 | 12/22 | 12/12 | 8/8 | 12 | 8/13 | 13/13 | 1.00 / 1.00 | 0/13 | -8.5% | .71 |
| clinic | clean | 1.00 / .97 | 53/53 | 53/53 | 34/34 | 32/32 | 37 | 31/36 | 36/36 | 1.00 / 1.00 | 0/36 | -1.8% | .75 |
| clinic | scan | 1.00 / .96 | 53/53 | 53/53 | 34/34 | 32/32 | 38 | 31/36 | 36/36 | 1.00 / 1.00 | 0/36 | -1.8% | .74 |
| clinic | thin | 1.00 / .95 | 52/53 | 52/53 | 34/34 | 32/32 | 37 | 31/36 | 36/36 | 1.00 / 1.00 | 0/36 | -1.8% | .74 |
| clinic | bold | .99 / .99 | 53/53 | 53/53 | 34/34 | 32/32 | 32 | 27/36 | 36/36 | 1.00 / 1.00 | 0/36 | -1.8% | .75 |
| clinic | outline | 1.00 / .82 | 36/53 | 36/53 | 34/34 | 32/32 | 46 | 30/36 | 35/36 | 1.00 / 1.00 | 0/36 | -1.8% | .72 |
| clinic | hatch | 1.00 / .83 | 36/53 | 36/53 | 34/34 | 32/32 | 45 | 30/36 | 35/36 | 1.00 / 1.00 | 0/36 | -1.8% | .72 |
| clinic | outline-scan | 1.00 / .83 | 36/53 | 36/53 | 34/34 | 32/32 | 43 | 29/36 | 34/36 | 1.00 / 1.00 | 0/36 | -1.8% | .72 |
| suites | clean | .97 / .85 | 49/53 | 49/53 | 23/23 | 23/23 | 27 | 23/36 | 30/36 | .98 / 1.00 | 0/36 | +0.8% | .72 |
| suites | scan | .97 / .86 | 48/53 | 48/53 | 23/23 | 23/23 | 27 | 23/36 | 30/36 | .99 / 1.00 | 0/36 | -0.4% | .72 |
| suites | thin | .98 / .89 | 49/53 | 49/53 | 23/23 | 23/23 | 26 | 21/36 | 30/36 | .99 / 1.00 | 0/36 | -0.4% | .73 |
| suites | bold | .77 / .69 | 40/53 | 40/53 | 18/18 | 18/18 | 22 | 18/36 | 32/36 | .98 / 1.00 | 0/36 | -0.4% | .72 |
| suites | outline | .80 / .50 | 23/53 | 23/53 | 20/20 | 20/20 | 31 | 13/36 | 20/36 | .79 / 1.00 | 0/36 | -0.4% | .70 |
| suites | hatch | .96 / .84 | 45/53 | 45/53 | 23/23 | 23/23 | 23 | 20/36 | 27/36 | .87 / 1.00 | 0/36 | -0.4% | .72 |
| suites | outline-scan | .80 / .47 | 23/53 | 23/53 | 20/20 | 20/20 | 36 | 11/36 | 18/36 | .80 / 1.00 | 0/36 | -0.4% | .71 |
| corporate | clean | .84 / .84 | 74/95 | 74/95 | 37/37 | 35/35 | 32 | 13/58 | 25/58 | .38 / .99 | 0/58 | +11.0% | .70 |
| corporate | scan | .78 / .74 | 71/95 | 71/95 | 37/37 | 35/35 | 34 | 19/58 | 26/58 | .39 / .99 | 0/58 | +9.7% | .69 |
| corporate | thin | .73 / .80 | 56/95 | 56/95 | 27/27 | 25/25 | 13 | 13/58 | 13/58 | .14 / 1.00 | 0/58 | -7.5% | .64 |
| corporate | bold | .88 / .69 | 43/95 | 44/95 | 33/33 | 31/31 | 70 | 14/58 | 23/58 | .96 / .97 | 0/58 | +5.7% | .70 |
| corporate | outline | .71 / .51 | 23/95 | 23/95 | 23/23 | 21/21 | 47 | 9/58 | 15/58 | .35 / .98 | 0/58 | +5.7% | .69 |
| corporate | hatch | .85 / .72 | 37/95 | 37/95 | 37/37 | 35/35 | 62 | 27/58 | 37/58 | .99 / .97 | 0/58 | +11.0% | .69 |
| corporate | outline-scan | .72 / .52 | 25/95 | 25/95 | 25/25 | 23/23 | 24 | 4/58 | 6/58 | .17 / .97 | 0/58 | +5.7% | .68 |

All 28 rows have zero named matches: room naming is not solved for raster input by this baseline. Conditional 100% swing/hinge results do not compensate for missed openings. The corporate hatch row illustrates why floor-area recall alone is insufficient: area recall is .99 while exact room recall is only 27/58.

## Primary research and portable trials

The research proposals below were hypotheses when selected. Only the implementations and outcomes in the measured-results section have subsequently been tested; the other proposals remain unmeasured. The two similar floor-plan projects were inspected beyond their READMEs: the linked segmentation, wall-pairing and registration source files contain the described logic.

### 1. Space semantics before reconstruction

IFC defines physical and virtual space boundaries separately from the building elements providing them; connection geometry can be absent. IfcOpenShell documents outer and inner boundary polylines and coordinates relative to a space placement. Sources: [buildingSMART boundary definition](https://standards.buildingsmart.org/IFC/RELEASE/IFC2x/FINAL/HTML/ifcproductextension/lexical/ifcrelspaceboundary.html), [IfcOpenShell API](https://docs.ifcopenshell.org/autoapi/ifcopenshell/api/boundary/index.html).

Trial: consume trusted registered space polygons as explicit room/zone evidence, preserving source identifiers, holes, scale and transform evidence. Reconstruct only uncovered or rejected areas. Keep virtual zoning separate from physical wall evidence. This is applicable to the existing `Input.Spaces` seam; no IFC parser is required for the first trial.

Caveat: a closed CAD polyline or HATCH is not automatically a room. Validate floor association, closure, plausible area, registration and label association. If the polygons also supply evaluation truth, report this as supplementary-source integration, not independent room discovery. IFC is a semantic reference; no external implementation copying is needed.

### 2. CAD extraction must retain blocks, layers and boundary paths

Ezdxf documents recursive block decomposition, HATCH boundaries and OCS coordinates. Its documented pitfalls include unsupported XREF decomposition, ignored XCLIP, separate MINSERT handling and approximate text bounds. Sources: [decomposition](https://ezdxf.readthedocs.io/en/stable/disassemble.html), [INSERT](https://ezdxf.readthedocs.io/en/stable/blocks/insert.html), [HATCH](https://ezdxf.readthedocs.io/en/stable/dxfentities/hatch.html), [layer inheritance](https://ezdxf.readthedocs.io/en/stable/dxfinternals/rendering_of_dxf_content.html).

Trial: audit nested door blocks, ATTRIB room identifiers, closed room polylines and HATCH boundary paths before reducing evidence to lines. Preserve block/layer/handle provenance and effective world transforms. Normalize inherited layer semantics explicitly.

Caveat: exploding everything first discards useful symbol identity. Hatching can describe walls or finishes rather than rooms; non-uniform scaling can turn circular arcs into other curves. Use these documented cases to check the Go reader rather than add a Python dependency.

### 3. Door arcs and intersection repair from a directly relevant CAD paper

Srivastava, Maheshwari and Rajan (2018), [Towards Generating Semantically-Rich IndoorGML Data from Architectural Plans](https://isprs-archives.copernicus.org/articles/XLII-4/591/2018/isprs-archives-XLII-4-591-2018.pdf), extract parallel-line wall pairs, extend centrelines to intersections, identify door arcs using swing radius versus opening length, associate text and derive loops.

Trial: require compatible gap width, nearby hinge/leaf geometry and host-wall alignment for an arc-based doorway candidate. Insert a local threshold segment across a supported doorway. Test endpoint-to-intersection repair separately. For raster, implement deterministic arc/leaf template evidence in a wall-local coordinate frame and emit the same opening representation.

Caveat: the paper explicitly considers rectilinear spaces, parallel-line walls and basic arc door symbols, excluding curved walls and complex elements. Its rules do not establish general commercial-floor accuracy. The paper is CC BY 4.0; no implementation license was verified.

### 4. Several wall thicknesses, weighted by actual overlap

Inspected [`floorplan-parser/rooms.py`](https://raw.githubusercontent.com/dzakybd/floorplan-parser/main/floorplan_parser/rooms.py): gap histograms are weighted by parallel-line overlap length; several thickness peaks form wall bands; bands extend at their ends and only collinear gaps are bridged. [MIT license verified](https://raw.githubusercontent.com/dzakybd/floorplan-parser/main/LICENSE).

Trial: compare the existing pairing against a small set of floor-specific thickness peaks with scale-aware minimum overlap. Retain candidate gap, overlap and thickness assignment for diagnostics. Use collinearity to avoid sealing passages between parallel walls.

Caveat: the author's [evaluation](https://github.com/dzakybd/floorplan-parser#results-held-out-floorplancad-drawings) reports room F1 about .09 with one thickness and .55 with five peaks on clean vectors generated from real-layout annotations. Those are author-reported results, not reproduced here and not directly comparable with this evaluator. Its default 14-thickness opening bridge is too broad to adopt without testing. The modest real-layout result supports investigating this component, not expecting it to solve the entire pipeline.

### 5. Labels as diagnostics, and deterministic registration scoring

Inspected [`pdf2floor/segment.py`](https://raw.githubusercontent.com/kambar231/pdf2floor/main/pdf2floor/segment.py) and [`georef.py`](https://raw.githubusercontent.com/kambar231/pdf2floor/main/pdf2floor/georef.py): wall raster closing, free-space components, label/component assignment and multi-label warnings; finite rotation/scale/translation search scored using footprint coverage and label containment. [MIT license verified](https://raw.githubusercontent.com/kambar231/pdf2floor/main/LICENSE).

Trial: report zero-label components, multi-label components, labels on barriers and labels moved to a nearby component. Localize missing closures from this audit. Compare registration hypotheses against independent anchors and held-out label positions. Preserve deterministic candidate ordering. [OpenCV's four-parameter robust 2D transform](https://docs.opencv.org/4.13.0/d9/d0c/group__calib3d.html) is an additional algorithm reference where matched points exist; a Go implementation can enumerate hypotheses rather than introduce a runtime dependency.

Caveat: pdf2floor fabricates boxes for some hatched interiors and assigns one polygon to multiple labels; those rendering fallbacks are not accepted extraction. Symmetric footprints can fit incorrectly. Existing registration is already good on the five floors, so extending room consumption has a clearer immediate motivation than replacing registration.

### 6. Shared subdivision and selective gap repair

JTS author Martin Davis describes snapping/noding all boundaries, polygonizing them, assigning resultant faces to their parent polygons, and resolving overlaps and selected narrow gaps. Gap width uses maximum-inscribed-circle diameter. Sources: [author's CoverageCleaner explanation](https://lin-ear-th-inking.blogspot.com/2025/04/coverage-cleaning-in-jts.html), [merged implementation PR](https://github.com/locationtech/jts/pull/1126), [OverlayNG precision explanation](https://lin-ear-th-inking.blogspot.com/2020/05/jts-overlay-next-generation.html).

Trial: construct shared boundary geometry rather than independently snapping each room. Identify faces owned by zero, one or several candidates. Repair small inconsistencies while retaining changed fragments for QA; use physical width rather than only area to classify slivers. Reuse the approach across CAD and raster polygon candidates.

Caveat: room interiors legitimately have wall-width gaps, and narrow corridors/risers are real spaces. Do not apply generic coverage gap filling to all room polygons. Larger snap distances can alter topology and remove small inputs. A numerical precision grid is distinct from a physical gap-sealing distance. Inspect JTS licensing separately before any direct code port.

### 7. Polygonization residuals should remain visible

[`polygonize_full`](https://shapely.readthedocs.io/en/stable/reference/shapely.polygonize_full.html) returns polygons plus cut edges, dangling edges and invalid rings. [`make_valid`](https://shapely.readthedocs.io/en/stable/reference/shapely.make_valid.html) distinguishes a linework-preserving strategy from one that trusts shell/hole structure.

Trial: expose equivalent residual diagnostics from the existing Go graph. Measure whether targeted endpoint-to-segment snapping and intersection noding repair known misses with less movement than increasing the global seal. Keep face extraction and repair reasons inspectable in source-image coordinates.

Caveat: geometric validity does not prove that the result is the correct room. Repair may collapse a polygon to lines or empty output, or produce collections. Retain original geometry, changed area, dimensionality and hole count. These docs provide algorithm semantics; this scope does not call for running a Python production pipeline.

### 8. Robust polygon operations in Go

[`engelsjk/polygol`](https://github.com/engelsjk/polygol) implements Go union, intersection, difference and XOR over polygons/multipolygons; its [MIT license was verified](https://raw.githubusercontent.com/engelsjk/polygol/main/LICENSE). Clipper2's [author documentation](https://www.angusj.com/clipper2/Docs/Overview.htm) explains integer-based clipping, coordinate scaling, holes and fill rules; its [source license](https://github.com/AngusJohnson/Clipper2/blob/main/LICENSE) is Boost Software License 1.0.

Trial: use robust vector intersections/differences for trusted-space versus wall-band reconciliation, room-overlap detection and clipping to a reliable floor envelope. Compare against the existing Go geometry before selecting any dependency. Add only the minimum operation that an observed failure needs.

Caveat: polygol documents queue/sweep limits on large inputs. Boolean clipping does not infer missing architectural boundaries. Test near-coincident edges, touching rings, holes and narrow passages under an explicit precision policy. Clipper2 is a robustness reference, not a proposed non-Go runtime. No new geometry dependency is selected by this report.

## Trial protocol

1. Freeze provenance: record the exact commit, tracer version, dirty-tree state, test command, Go version, thresholds and enabled assistance for each run. Keep private input mappings and raw overlays outside the repository. Store only anonymized metrics here.
2. Run both tracks for every shared geometry change. Retain all five real architectural floors, five SPACE-only inputs, registration checks and all 28 raster cases; never report only the best floor or variant. Add real raster scans and rasterized real CAD as separate future datasets, clearly distinguished from the current synthetic renders.
3. Separate three evidence modes: automatic drawing-only extraction; extraction with supplementary semantic spaces or explicit calibration/seeds; and editor-corrected output. Raster tests with supplied labels/scale must identify that assistance. Never give a candidate the truth polygon or scale silently.
4. Establish one failure hypothesis at a time. For rooms, classify misses as missing, merged, split, offset or wrongly named. For openings, distinguish absent source evidence, front-end failure, wall-host rejection and overlap cleanup. Inspect residual graph edges before changing a global tolerance.
5. First compare trusted-space consumption, local doorway sealing and raster thin/outline recovery. Keep semantic boundaries separate from physical walls: consuming a room polygon must not create imaginary walls. Preserve voids, open-plan zones and door host geometry.
6. Use physical units for geometric thresholds. Test measured width families rather than one global width. Bound snapping and record the largest move. Treat numerical precision, raster resolution, endpoint repair distance and doorway width as separate controls.
7. Keep raw numerator/denominator counts with the percentages. Report room recall and precision at IoU ≥ .9, the existing .75 and boundary alternatives, merge/split counts, wall R/P, opening detection and conditional attributes, names, scale, void defects, furniture where applicable, time, memory and finalization. Confidence must remain separate from accuracy.
8. Re-run determinism checks with shuffled evidence and repeated runs. A new geometry operation must preserve holes, prevent self-intersections/overlapping room interiors and satisfy the normal validators. Add focused regression fixtures from a failure's geometry without copying private drawings.
9. Compare per-row deltas, not just averages. A measurable gain is worthwhile, but disclose any regression and the source of assistance. Five related floors are development evidence, not a statistically broad generalization claim. Freeze thresholds before a separately held-out dataset is scored.

Reproduction entry points, with the private CAD fixture directory already configured externally:

```sh
GOWORK=off go test ./internal/autotrace/cadfront -run '^(TestRealCAD|TestRealSpaces)$' -count=1 -v
GOWORK=off go test ./internal/autotrace -run '^TestAccuracy$' -count=1 -v
```

The real-CAD tests skip without `SPACEWEAVE_DWG_DIR`; a skipped run supplies no evidence. Run repository-required validation after implementation as well. The research document itself requires numeric/source verification rather than additional application tests.

### Interpreting the 99% objective

Treat 99% as a per-floor goal with a named metric and denominator, not as the existing confidence score or total floor-area coverage. For fewer than 100 eligible rooms, ≥99% room recall generally requires every room to match. Keep room precision and merges/splits alongside recall so an oversized polygon cannot satisfy the objective by covering everything. The baseline does not reach 99% room recall on any of the five architectural floors, and the sources above provide no evidence that a single deterministic technique guarantees it. Assistance and semantic-source integration may materially improve useful output, but must remain visible in the reported mode.

## Measured implementation results

The final completed `make quality` run contains 132 rows: 20 CAD configurations and 112 raster configurations (28 fixture/variant cases × 2 assistance modes × 2 raster algorithms, or 56 paired algorithm comparisons). All 112 raster configurations reported `valid=ok`, including the Node validator in this run. The report-only CAD test also completed, but deliberately retained one invalid and one rejected experimental configuration. A passing report-only test is therefore not a statement that every candidate was valid. The complete local repository gate subsequently passed on the final source with Go 1.27.4; see the toolchain validation below.

The selected production behavior is `package-none`: preserve registered source boundaries without snapping. Smaller explicit polygons are processed first to protect enclosed rooms. The shared overlay clips explicit geometry against the floor and voids, reserves the explicit floor before inference, then clips inferred space to the remainder. Supplied partial packages containing one or two outlines are accepted; the architectural area-polygon fallback retains its minimum of three outlines. This consumes supplied geometry; it is not independent discovery of those spaces. Source: [explicit-space overlay](../../internal/floorgraph/explicit.go), [snap options](../../internal/floorgraph/space_input.go), [CAD area fallback](../../internal/autotrace/cadfront/areas.go).

Interim browser inspection found generic block metadata concatenated into space names. The final reader uses only designated name/number layers when those layers exist, while retaining stated-area evidence and a fallback for drawings without designated identity layers. The final table below replaces the earlier trial's name counts; geometry and all wall/door scores are unchanged. Selected-mode named/nameWant counts are now 45/57, 46/47, 48/49, 48/49 and 46/47. This corrects source identity extraction rather than improving boundary discovery. Source: [space text extraction](../../internal/autotrace/cadfront/spaces.go).

### CAD: all 20 configurations under the new scorer

**These scores are not comparable to the earlier room-only masked baseline.** The new pure-Go [space identity scorer](../../internal/autotrace/score/spaces.go) matches all reference rooms and zones one-to-one at raw polygon IoU ≥ .9, with no wall-mask exclusion and no group-coverage credit. A maximum-cardinality assignment prevents duplicate predictions from claiming the same reference. Recall is matched/want; precision is matched/got. Names are counted only for geometrically matched spaces with an equal normalized name and, when present in the reference, equal normalized number. `Names` is named/nameWant across all eligible references, not conditional accuracy among successful matches. Room and zone counts additionally require the correct kind. The scorer also emits splits, merges, mean relative area error and mean sampled bidirectional boundary error.

`architecture` supplies no space polygons. All `package-*` modes receive the registered reference space polygons and names: they measure preservation/reconciliation of supplied semantics. `package-walls` uses adaptive wall snapping; `package-300mm` uses a 300 mm snapping trial; `package-none` preserves boundaries. Their common reference is the same supplied space plan, so this is not an independent room-geometry benchmark. Percentages are rounded to one decimal; matched counts are the primary evidence.

| Floor | Mode | Want | Got | Matched | Recall % | Precision % | Names | Rooms matched/want | Zones matched/want | Splits | Merges | Elapsed s | Valid |
|---|---|---:|---:|---:|---:|---:|---|---|---|---:|---:|---:|---|
| 00 | architecture | 63 | 82 | 4 | 6.3 | 4.9 | 0/57 | 4/49 | 0/14 | 7 | 4 | 2.41 | ok |
| 00 | package-walls | 63 | 67 | 30 | 47.6 | 44.8 | 28/57 | 19/49 | 11/14 | 3 | 0 | 5.21 | ok |
| 00 | package-none | 63 | 71 | 48 | 76.2 | 67.6 | 45/57 | 36/49 | 12/14 | 1 | 0 | 3.04 | ok |
| 00 | package-300mm | 63 | 67 | 30 | 47.6 | 44.8 | 28/57 | 19/49 | 11/14 | 3 | 0 | 4.66 | ok |
| 01 | architecture | 51 | 56 | 14 | 27.5 | 25.0 | 0/47 | 14/42 | 0/9 | 6 | 4 | 2.49 | ok |
| 01 | package-walls | 51 | 62 | 31 | 60.8 | 50.0 | 30/47 | 24/42 | 7/9 | 4 | 0 | 2.68 | invalid overlap |
| 01 | package-none | 51 | 61 | 50 | 98.0 | 82.0 | 46/47 | 42/42 | 8/9 | 1 | 0 | 2.62 | ok |
| 01 | package-300mm | 51 | 59 | 30 | 58.8 | 50.8 | 29/47 | 24/42 | 6/9 | 2 | 0 | 4.06 | ok |
| 02 | architecture | 55 | 60 | 11 | 20.0 | 18.3 | 0/49 | 11/47 | 0/8 | 7 | 3 | 2.58 | ok |
| 02 | package-walls | 55 | 62 | 29 | 52.7 | 46.8 | 28/49 | 23/47 | 6/8 | 1 | 0 | 3.93 | ok |
| 02 | package-none | 55 | 66 | 54 | 98.2 | 81.8 | 48/49 | 47/47 | 7/8 | 1 | 0 | 2.71 | ok |
| 02 | package-300mm | 55 | 61 | 28 | 50.9 | 45.9 | 27/49 | 23/47 | 5/8 | 1 | 0 | 3.63 | ok |
| 03 | architecture | 54 | 57 | 9 | 16.7 | 15.8 | 0/49 | 9/43 | 0/11 | 6 | 4 | 2.05 | ok |
| 03 | package-walls | — | — | — | — | — | — | — | — | — | — | — | rejected overlay |
| 03 | package-none | 54 | 65 | 53 | 98.1 | 81.5 | 48/49 | 43/43 | 10/11 | 1 | 0 | 2.25 | ok |
| 03 | package-300mm | 54 | 68 | 30 | 55.6 | 44.1 | 29/49 | 22/43 | 8/11 | 4 | 0 | 3.50 | ok |
| 04 | architecture | 53 | 61 | 7 | 13.2 | 11.5 | 0/47 | 6/38 | 0/15 | 7 | 3 | 2.09 | ok |
| 04 | package-walls | 53 | 61 | 31 | 58.5 | 50.8 | 29/47 | 21/38 | 10/15 | 3 | 0 | 2.23 | ok |
| 04 | package-none | 53 | 80 | 52 | 98.1 | 65.0 | 46/47 | 38/38 | 14/15 | 1 | 0 | 2.61 | ok |
| 04 | package-300mm | 53 | 60 | 28 | 52.8 | 46.7 | 27/47 | 19/38 | 9/15 | 2 | 0 | 2.96 | ok |

The adaptive floor-01 candidate has a 4.26 m² room overlap. Adaptive floor-03 is rejected after a clipping side-location conflict; no scores were emitted, so its blank cells must not be interpreted as zero. Both snapping approaches are rejected for production: where they produce scores, each has fewer matches than unsnapped preservation. Unsnapped preservation validates on all five floors and has zero reported merges, but retains one split per floor and substantial unmatched output. Floor 00 remains particularly incomplete. High IoU among successful matches does not excuse missed or extra spaces.

Selected-mode diagnostics below average only geometrically matched spaces. Area error is absolute relative error, expressed as a percentage. Boundary error samples vertices and edge midpoints in both directions, in source-image pixels; it is not an exact Hausdorff distance. These conditional diagnostics must not replace all-space precision and recall.

| Floor | Correct kind among matches | Mean IoU | Mean area error % | Mean boundary error px |
|---|---:|---:|---:|---:|
| 00 | 48 | .999628 | .0372 | .0554 |
| 01 | 50 | .999995 | .0004 | .0293 |
| 02 | 54 | .999257 | .0742 | .0133 |
| 03 | 53 | .998178 | .1822 | .0280 |
| 04 | 52 | .999980 | .0019 | .0293 |

All successful CAD configurations have the same wall and door scores for their floor. These also equal the original wall/door baseline; the new space integration changes neither wall evidence nor measured door detection. Opening selection sorts evidence by descending weight, preferring measured evidence to guessed evidence at equal weight. This protects an observed door from a weak overlapping gap in a focused synthetic regression test, but it produces **no measured door-score improvement on these five floors**. Source: [opening selection](../../internal/floorgraph/openings.go), [weak-overlap regression](../../internal/floorgraph/openings_test.go).

| Floor | Wall recall % | Wall precision % | Door recall % | Door precision % |
|---|---:|---:|---:|---:|
| 00 | 93.5 | 96.8 | 81.2 | 86.0 |
| 01 | 71.5 | 97.8 | 91.7 | 95.7 |
| 02 | 70.2 | 97.5 | 96.0 | 98.0 |
| 03 | 70.7 | 97.9 | 93.3 | 87.5 |
| 04 | 98.6 | 94.0 | 92.2 | 92.2 |

The final CAD JSON adds `openingPlacement` to distinguish pipeline losses from front-end detection. These counts cover every cleaned opening hypothesis, including doors, windows and plain gaps; they are not door recall denominators. Placed + unplaced + narrow + overlapped equals input. Unplaced hypotheses find no usable output host; narrow hypotheses fall below the retained width; overlapped hypotheses lose to another candidate. Counts are identical across successful space modes for each floor. Source: [opening statistics](../../internal/floorgraph/input.go).

| Floor | Cleaned input hypotheses | Placed | Unplaced | Too narrow | Lost to overlap |
|---|---:|---:|---:|---:|---:|
| 00 | 179 | 124 | 36 | 7 | 12 |
| 01 | 64 | 55 | 9 | 0 | 0 |
| 02 | 67 | 58 | 9 | 0 | 0 |
| 03 | 69 | 59 | 7 | 0 | 3 |
| 04 | 71 | 62 | 8 | 0 | 1 |

### Raster: opt-in face recovery, with assistance counted explicitly

`bands` is the default raster extraction. `faces` enables `rasterfront.Options.WallFaces`; it remains opt-in because gains are mixed and precision regressions are material. No default raster improvement is claimed.

The assisted mode supplies the **reference calibration to the front end before extraction**, then seeds **reference room-label positions and names after extraction** before shared room building. It does not run OCR. The reference scale and labels are assistance, not successful automatic extraction and not a post-scoring alignment. Both automatic and assisted modes retain the synthetic text and furniture in their input render. Source: [raster trial implementation](../../internal/autotrace/quality_test.go).

Across all 28 automatic pairs, face recovery increases exact matched-space counts in 6 cases, ties in 13 and decreases them in 9; total matches are 488 → 486 across 1001 references. Across 28 assisted pairs, there are 9 increases, 14 ties and 5 decreases; total matches are 476 → 491 across 1001 references. Wall precision decreases in 23 of 28 cases in each mode. These are descriptive counts on related synthetic fixtures, not evidence of a general accuracy gain. Assistance itself can also change room topology adversely; it does not guarantee more matches.

Selected deltas below show both gains and regressions. Every arrow is default bands → opt-in faces within the same evidence mode, with the new one-to-one IoU ≥ .9 scorer. `Matches` and `Names` give the shared reference denominator. Wall values are fractions, rounded to three decimals. The complete run emits all 112 machine-readable `QUALITY` rows.

| Fixture / variant | Mode | Wall recall | Wall precision | Matches | Names |
|---|---|---|---|---|---|
| office / thin | automatic | .756 → .783 | .898 → .884 | 0 → 2 / 13 | 0 → 0 / 13 |
| office / thin | assisted | .756 → .783 | .901 → .887 | 0 → 2 / 13 | 0 → 2 / 13 |
| clinic / bold | automatic | .991 → 1.000 | .988 → .930 | 27 → 31 / 36 | 0 → 0 / 36 |
| clinic / bold | assisted | .991 → 1.000 | .988 → .930 | 27 → 31 / 36 | 4 → 4 / 36 |
| suites / clean | automatic | .974 → .974 | .854 → .741 | 23 → 22 / 36 | 0 → 0 / 36 |
| suites / clean | assisted | .974 → .974 | .877 → .758 | 21 → 20 / 36 | 1 → 1 / 36 |
| suites / hatch | automatic | .957 → .970 | .844 → .725 | 20 → 22 / 36 | 0 → 0 / 36 |
| suites / hatch | assisted | .957 → .970 | .867 → .741 | 18 → 20 / 36 | 1 → 1 / 36 |
| corporate / clean | automatic | .842 → .844 | .843 → .813 | 13 → 11 / 58 | 0 → 0 / 58 |
| corporate / clean | assisted | .841 → .843 | .845 → .814 | 19 → 21 / 58 | 18 → 19 / 58 |
| corporate / thin | automatic | .727 → .783 | .796 → .799 | 13 → 8 / 58 | 0 → 0 / 58 |
| corporate / thin | assisted | .726 → .783 | .799 → .804 | 13 → 12 / 58 | 13 → 12 / 58 |
| corporate / bold | automatic | .882 → .928 | .692 → .706 | 14 → 16 / 58 | 0 → 0 / 58 |
| corporate / bold | assisted | .882 → .928 | .701 → .717 | 16 → 18 / 58 | 8 → 10 / 58 |
| corporate / outline | automatic | .711 → .714 | .507 → .500 | 9 → 10 / 58 | 0 → 0 / 58 |
| corporate / outline | assisted | .707 → .711 | .521 → .514 | 8 → 11 / 58 | 6 → 8 / 58 |

The suites clean case loses wall precision from .854 to .741 while losing a room match. The corporate thin case gains wall recall while losing five room matches. These counterexamples rule out promoting face recovery as the default on the current evidence. Automatic raster naming remains absent; assisted names come from supplied seeds.

### Reproduction and validation limits

With the private CAD fixture directory configured externally and Node available:

```sh
make quality
```

The Makefile exports `GOWORK=off`; this target enables `SPACEWEAVE_QUALITY_TRIALS=1` and runs `TestQualityTrials|TestRasterTrials` with `-count=1 -v` in both front-end packages. The CAD half skips when its fixture directory is not configured. The `QUALITY` JSON rows contain the counts needed to reproduce these tables; do not commit raw private CAD diagnostics or overlays. Source: [quality target](../../Makefile), [CAD trials](../../internal/autotrace/cadfront/quality_test.go).

The five pre-existing golden plan hashes are unchanged, and two package-space fixtures have been added to [golden coverage](../../internal/autotrace/golden_test.go). Metadata-exclusion, partial-package and nested-zone regression tests accompany the implementation. The goldens check reproducibility; the new gold output has not been independently reviewed. The completed quality trials include Node validation. Settled Chrome Plan/3D checks passed on all five floors, with no uncaught browser errors. The complete Go 1.27.4 local `make gate` passed: vet, lint, race/shuffle, full accuracy/goldens, acceptance, coverage (97.7% total), vulnerability scanning and all six cross-compilation targets. Independent gold review is not claimed.

### Outstanding work and target status

**The 99% per-floor target is not achieved.** Selected-mode all-space recall is 76.2%, 98.0%, 98.2%, 98.1% and 98.1%; precision is 67.6%, 82.0%, 81.8%, 81.5% and 65.0%. All five floors fall short even with supplied semantic polygons. Room-only matches on floors 01–04 are complete in this assisted mode, but that excludes the remaining zones and extra predictions and must not be presented as achieving the overall target. Floor 00, residual splits, extra inferred spaces and exact-name misses require further investigation.

| Work | Established outcome | Remaining limitation |
|---|---|---|
| Registered-space consumption | Unsnapped preservation selected; all five selected configurations validate | Same geometry supplies reference and input; misses and extra spaces remain |
| Wall snapping trials | Adaptive and 300 mm modes measured and rejected | Fewer matches; one invalid overlap and one rejected adaptive overlay |
| Weak-overlap opening selection | Focused synthetic regression protects the observed door | All five real-floor door scores unchanged |
| Raster face recovery | Opt-in implementation and 112 configurations measured | Mixed topology results and large precision regressions; no default gain |
| Supplied raster calibration and labels | Assistance is explicit and reproducible in the trial | No OCR; seeded names are not automatic recognition |
| Other research proposals / editor correction workflow | Documented trial directions | No measured improvement or completed implementation established here |
| Settled Chrome Plan/3D checks | Passed locally | Five floors; private screenshots kept local |
| Complete local `make gate` | Passed on Go 1.27.4 | 97.7% total coverage; all six cross-compilation targets; no vulnerabilities |
| Independent gold review and human correction-time trial | Not performed | Required before claiming reviewed accuracy or correction effort |

### Browser verification

Checked all five isolated drafts in Chrome against the local port-8010 server,
after stopping the build animation at its completed state. Every floor rendered
in 3D and switched to Plan view; overlays returned HTTP 200, registration was
reported as used, and there were no uncaught browser errors. Screenshots and
source overlays remain private in scratch storage. Visual inspection found and
fixed metadata strings being concatenated into space names; it does not replace
independent annotation or establish the 99% target.

Semantic diagnostics (`gopls check`) and the 140 microsite tests passed.

### Toolchain validation

The first complete local race/acceptance coverage run passed the configured
thresholds at **97.6% total**, with every package at or above 90%. Its gate then
stopped at the existing Go 1.27.3 standard-library advisory
[GO-2026-6859](https://pkg.go.dev/vuln/GO-2026-6859). The module now selects
`toolchain go1.27.4`, the fixed release, while retaining language version 1.27.
The patched run passed race/shuffle, accuracy/goldens, acceptance and both
coverage thresholds (**97.7% total**), and `govulncheck` reports no vulnerabilities.
The complete patched-toolchain `make gate` exited successfully, including all
six local cross-compilation targets. CI cross-compilation also passed; the
remaining cross-platform CI jobs may still be running at publication.
