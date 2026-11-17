#!/usr/bin/env node
// Writes pixel traces of the hand-traced plans that internal/plan/testdata
// lacks, for the accuracy evaluation in golden_test.go: the same conversion
// as internal/plan/testdata/gen.mjs, trace only. Run from the repository
// root after changing a plan module:
//
//   node internal/autotrace/testdata/gen.mjs
import { writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { resolve } from 'node:path';

const PLANS = {
  clinic: 'web/floorplan-iso/js/plans/pt-clinic.js',
  suites: 'web/floorplan-iso/js/plans/urban-suites.js',
};
const out = new URL('./', import.meta.url);

for (const [name, path] of Object.entries(PLANS)) {
  const plan = await import(pathToFileURL(resolve(path)).href);
  const [sx, sy] = Array.isArray(plan.SOURCE.pxPerMetre) ? plan.SOURCE.pxPerMetre : [plan.SOURCE.pxPerMetre, plan.SOURCE.pxPerMetre];
  const [ox, oy] = plan.SOURCE.originPx;
  const px = ([x, y]) => [ox + x * sx, oy + y * sy];
  const dominant = (w) => (Math.abs(w.to[0] - w.from[0]) >= Math.abs(w.to[1] - w.from[1]) ? 0 : 1);
  const pxAngle = (deg) => (Math.atan2(Math.sin((deg * Math.PI) / 180) * sy, Math.cos((deg * Math.PI) / 180) * sx) * 180) / Math.PI;
  const trace = {
    title: plan.SOURCE.title,
    calibration: { pxPerMetre: [sx, sy], evidence: `fixture from ${path}` },
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
  writeFileSync(new URL(`${name}.trace.json`, out), `${JSON.stringify(trace)}\n`);
  console.log(`${name}: ${trace.walls.length} walls, ${trace.rooms.length} rooms, ${trace.furniture.length} items`);
}
