#!/usr/bin/env node
// Regenerates the golden fixtures for convert_test.go from hand-traced plan
// modules. For each module it writes
//
//   <name>.trace.json     the plan as a tracer would submit it (pixels)
//   <name>.expected.json  the module's own data, moved so the walls' top-left is
//                         the origin, as Convert does
//
// Run from the repository root after changing a plan module or the trace
// format:
//
//   node internal/plan/testdata/gen.mjs
import { writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { resolve } from 'node:path';

const PLANS = {
  office: 'web/floorplan-iso/js/plans/roomsketcher-office.js',
  corporate: 'web/floorplan-iso/js/plans/corporate-floor.js', // squashed: [38.6, 44] px/m
};
const out = new URL('./', import.meta.url);

for (const [name, path] of Object.entries(PLANS)) {
  const plan = await import(pathToFileURL(resolve(path)).href);
  const [sx, sy] = [plan.SOURCE.pxPerMetre].flat().length === 2 ? plan.SOURCE.pxPerMetre : [plan.SOURCE.pxPerMetre, plan.SOURCE.pxPerMetre];
  const [ox, oy] = plan.SOURCE.originPx;
  const px = ([x, y]) => [ox + x * sx, oy + y * sy];
  const dominant = (w) => (Math.abs(w.to[0] - w.from[0]) >= Math.abs(w.to[1] - w.from[1]) ? 0 : 1);
  // A plan-space bearing as it appears in the squashed image.
  const pxAngle = (deg) => (Math.atan2(Math.sin((deg * Math.PI) / 180) * sy, Math.cos((deg * Math.PI) / 180) * sx) * 180) / Math.PI;

  const trace = {
    title: plan.SOURCE.title,
    calibration: { pxPerMetre: [sx, sy], evidence: `golden fixture from ${path}` },
    thickness: plan.THICKNESS,
    ...(plan.OUTLINE ? { outline: plan.OUTLINE.map(px) } : {}),
    entrance: px(plan.ENTRANCE),
    walls: plan.walls.map((w) => {
      const axis = dominant(w);
      const k = axis === 0 ? sx : sy;
      const o = axis === 0 ? ox : oy;
      return {
        id: w.id, kind: w.kind, from: px(w.from), to: px(w.to),
        ...(w.thickness ? { thickness: w.thickness } : {}),
        ...(w.ext !== undefined ? { ext: w.ext } : {}),
        openings: (w.openings ?? []).map((op) => ({ ...op, a: o + op.a * k, b: o + op.b * k })),
      };
    }),
    rooms: plan.rooms.map((r) => ({
      id: r.id, name: r.name, zone: r.zone, floor: r.floor, polygon: r.polygon.map(px),
      ...(r.label ? { label: px(r.label) } : {}),
    })),
    floorPatches: plan.floorPatches.map((f) => ({ ...f, polygon: f.polygon.map(px) })),
    furniture: plan.furniture.map((f) => {
      const [x, y] = px([f.x, f.y]);
      const item = { ...f, x, y };
      if (f.rot !== undefined) item.rot = pxAngle(f.rot);
      if (f.facing !== undefined) item.facing = pxAngle(f.facing);
      return item;
    }),
  };

  // Convert puts the origin at the walls' (and outline's) top-left.
  const pts = [...plan.walls.flatMap((w) => [w.from, w.to]), ...(plan.OUTLINE ?? [])];
  const [mx, my] = [Math.min(...pts.map((p) => p[0])), Math.min(...pts.map((p) => p[1]))];
  const shift = ([x, y]) => [x - mx, y - my];
  const expected = {
    WIDTH: Math.max(...pts.map((p) => p[0])) - mx,
    DEPTH: Math.max(...pts.map((p) => p[1])) - my,
    ...(plan.OUTLINE ? { OUTLINE: plan.OUTLINE.map(shift) } : {}),
    ENTRANCE: shift(plan.ENTRANCE),
    walls: plan.walls.map((w) => {
      const axis = dominant(w);
      const d = axis === 0 ? mx : my;
      return { ...w, from: shift(w.from), to: shift(w.to), openings: (w.openings ?? []).map((op) => ({ ...op, a: op.a - d, b: op.b - d })) };
    }),
    rooms: plan.rooms.map((r) => ({ ...r, polygon: r.polygon.map(shift), ...(r.label ? { label: shift(r.label) } : {}) })),
    floorPatches: plan.floorPatches.map((f) => ({ ...f, polygon: f.polygon.map(shift) })),
    furniture: plan.furniture.map((f) => ({ ...f, x: f.x - mx, y: f.y - my })),
  };

  writeFileSync(new URL(`${name}.trace.json`, out), `${JSON.stringify(trace)}\n`);
  writeFileSync(new URL(`${name}.expected.json`, out), `${JSON.stringify(expected)}\n`);
  console.log(`${name}: ${trace.walls.length} walls, ${trace.rooms.length} rooms, ${trace.furniture.length} items`);
}
