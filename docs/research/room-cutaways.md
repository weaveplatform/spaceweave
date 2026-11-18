# Camera-aware room cutaways

The original cutaway lowered near-facing exterior walls to a fixed 0.3 m. It
could not reveal a room obscured by interior walls. Room focus now comes from
the picker or a floor click; the viewer cuts intervening walls along the actual
parallel camera ray and preserves rear walls. A separate cut height supports
0 m (hidden), independently of the main wall height. Room zoom includes the full
height of the rear walls. Fit clears room focus.
Whole-floor mode keeps the exterior-wall policy. Plan view cuts no walls.

For each source wall, `createRoomCutaway` sweeps its full footprint away from
the camera by `-height * (vx, vy) / vz` and intersects that shadow with the room
polygon. It subtracts the wall footprint's own overlap so room outlines following
wall centrelines do not falsely cut the back wall. The overlap calculation
supports concave rooms. Bounds reject distant candidates first, and classifications
are cached across build frames. Source wall geometry and saved plan data do not
change. Cuts apply to whole source wall segments, so a long segment may be cut
beyond the width of the focused room.

Doors, glazing, sills and lintels follow the same host-wall cut. This also fixes
door leaves standing above an exterior cutaway in whole-floor mode. Furniture
and void guardrails remain untouched.

## Local trials, 2026-11-18

Chrome trials use the five private review floors. Each selects one room, checks
four opposing isometric angles and top-down Plan, then exercises the controls and
an actual floor click. These counts describe the selected test room at each
camera angle; they are not tracing-accuracy scores.

| Floor | Rooms available | Walls cut at −45° / 45° / 135° / −135° | Walls cut in Plan | Classification p95, ms |
|---|---:|---|---:|---:|
| 00 | 73 | 23 / 24 / 51 / 29 | 0 | 1.38 |
| 01 | 52 | 45 / 48 / 62 / 47 | 0 | 0.86 |
| 02 | 60 | 50 / 50 / 85 / 65 | 0 | 0.97 |
| 03 | 62 | 60 / 77 / 53 / 55 | 0 | 1.46 |
| 04 | 59 | 62 / 58 / 52 / 57 | 0 | 3.73 |

The classification timing is a Node microbenchmark over 180 camera azimuths per
floor at 35.26° elevation and 2.7 m wall height. It measures the obstruction
calculation alone, not total browser frame time or a 60 fps guarantee.

All 25 Chrome poses pass: cuts respect the requested height, 0 m removes the
complete wall assembly, nonblocking walls retain their height, furniture and
guardrail geometry stay identical, and disabling cutaways restores exact wall
paths. A settled room focus also produces zero SVG writes, including its cached
outline. All 306 rooms appear in the picker. Slider, checkbox, floor click, Fit
and floor-change checks pass. All 1,801 desks still pass mouse, keyboard, touch,
selection, style, replay and pan checks.

All 157 Node tests pass. The five new geometry cases cover all camera quadrants,
walls outside the room, height/elevation, Plan, concave rooms and rotated/reversed
walls. The full 2.8-second animation remains, with 45 fixed-stage renders checked;
settled SVG writes and native furniture document-position queries remain zero.
Compared with the accepted desk build, 37 default-view fingerprints match;
the other eight differ only in exterior door geometry following its host cut.
Those differences were inspected separately against the frozen baseline.

Private screenshots are inspected locally. No private drawings, coordinates or
renders belong in this repository. The Go gate and tracing trials are not rerun
for this frontend-only change; tracing code and generated data are unchanged.

## Repeat

With a review server and isolated Chrome debugging port 9267:

```sh
(cd web/floorplan-iso && npm test)
node web/floorplan-iso/tools/cutaway-trial.mjs http://127.0.0.1:8011/ "$SCRATCH/cutaways.json"
node web/floorplan-iso/tools/desk-trial.mjs http://127.0.0.1:8011/ "$SCRATCH/desks.json"
node web/floorplan-iso/tools/animation-trial.mjs http://127.0.0.1:8011/ "$SCRATCH/animation.json"
```
