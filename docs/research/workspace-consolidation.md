# Workspace consolidation

The viewer's single sidebar mixed floor selection, imports, review, layer roles,
appearance, camera controls and implementation details. The owner agreed to
consolidate those workflows before adding features. This change reorganises the
existing capabilities and makes selection, focus and cutaways explicit.

## Interface and behaviour

| Area | Contents |
|---|---|
| View, the default | Source-based room/desk selection, walls, visibility; collapsed appearance and camera/help |
| Review | Model/trace-overlay/source comparison, checks, approval, processing, source files and staged layer roles |
| Import dialog | File selection/drop/paste, supported formats and processing disclosure |
| Header | Imported floors, review status, separate examples, workspace tabs and Import |

Click or choose a room to select it; this does not move the camera or change
cuts. Focus frames the selected room or desk and identifies its containing room.
Enabled cutaways become Reveal focused room; Off remains off. The banner names
the room context. Clear/Escape clears selection while retaining that context.
Fit changes only the camera; Whole floor clears room focus. Camera turns preserve
the explicitly focused object until Fit or Whole floor. All six appearances,
visibility controls and the full 2.8-second build remain. Rebuild is now labelled
Replay animation, distinct from Retrace.

Wall controls distinguish Off, Near exterior walls and Reveal focused room. They
hide unused settings and explain the missing target or the exterior-only policy.
Cut height still supports fully hidden walls at 0 m. The room-shadow algorithm,
height limits, host-door cuts and source geometry remain unchanged.

Review displays trace caveats and the actual overlay/source images, with a full
size link. Tracer confidence lives in Technical details and is explicitly not a
measured accuracy score. The automatic tracer offers Retrace with a deterministic
input explanation; Request changes and its note form are offered only when the
active Claude tracer can use those notes.

Layer role edits are staged per floor, survive floor navigation in the same
page, and protect against closing/reloading with unsaved changes. Apply and
retrace performs one profile PUT followed by one retrace POST. Failed saves keep
the edits and never retrace. If queuing fails after a successful save, the UI
states that the profile is saved, keeps the pending retry and retries only the
queue request. Clearing that retry explicitly keeps the saved profile. Pending
edits disable approval and separate retrace;
Discard returns to reported roles. The profile's effect on future drawings is
stated beside the editor. A completed retrace refreshes the active model;
processing states label an existing model as the previous result.

## Local verification, 2026-11-18

- All 161 Node tests pass, including source-based selection details, containing
  room lookup, stale selection removal and honest review status/progress text.
- The workspace browser trial checks desktop 1440 × 1000 and mobile 390 × 844,
  selection versus camera/cutaway state, Focus, Fit, room focus, modal Escape,
  source/overlay display, keyboard tabs, import validation and processing.
- All API writes in that trial are intercepted with Chrome Fetch. It verifies
  no writes during staging, per-floor draft preservation, one save/one retrace,
  failed-save retention, failed-queue retry without a second save, discard,
  the unload guard, conditional agent notes and
  refreshing after a simulated trace finishes. It never queues traces, changes
  profiles or approves the real review floors.
- All 1,801 desks pass the existing mouse, keyboard, touch, replay, style and
  camera trial, updated to require explicit Focus for room zoom.
- The cutaway trial passes all five floors and 25 camera poses, including exact
  wall restoration, unaffected furniture/guardrails, all 306 room choices,
  full-height room framing and zero settled SVG writes. Fit retains room context;
  Whole floor clears it.
- All 45 fixed-stage SVG fingerprints match the frozen pre-consolidation commit
  `91eeffb`. The full animation, zero unchanged SVG writes, cancellation handling
  and zero native furniture position queries pass. Accessibility attributes are
  excluded from the fingerprints; geometry and material attributes are compared.
- Desktop, review-image and mobile screenshots were inspected locally. Private
  drawings, screenshots, trial data and CPU timings remain outside the repository.

This is a frontend change. No tracing code, generated plans, dependencies or API
routes change. The Go gate and CAD/raster extraction trials were not repeated.
The full animation still exceeds a consistent 30/60 fps budget on dense floors;
this consolidation does not claim a rendering-speed or tracing-accuracy gain.

## Repeat

With the review server and an isolated Chrome debugging session on port 9267:

```sh
npm --prefix web/floorplan-iso test
node web/floorplan-iso/tools/workspace-trial.mjs http://127.0.0.1:8011/ "$SCRATCH/workspace.json"
node web/floorplan-iso/tools/desk-trial.mjs http://127.0.0.1:8011/ "$SCRATCH/desks.json"
node web/floorplan-iso/tools/cutaway-trial.mjs http://127.0.0.1:8011/ "$SCRATCH/cutaways.json"
node web/floorplan-iso/tools/animation-trial.mjs http://127.0.0.1:8011/ "$SCRATCH/animation.json" "$SCRATCH/baseline.json"
```

The animation baseline must use the same generated catalogue and fixed-state
configuration. Set `SPACEWEAVE_SCREENSHOT_DIR` to a private scratch directory to
capture the workspace trial's desktop, review and mobile views. CDP connections
are shared through `tools/browser.mjs`; each tool closes only its own target.
