// Invariants every traced plan must satisfy. A plan whose module doesn't exist
// yet is skipped, so plans can land one at a time.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { PLANS } from '../js/plans/index.js';
import { furniturePrisms, wallPrisms, pointInPolygon, polygonArea, BLOCKY } from '../js/geometry.js';
import { makeView, PRESETS } from '../js/projection.js';
import { projectPrism } from '../js/geometry.js';

const ZONES = new Set(['meet', 'work', 'social', 'service']);
const FLOORS = new Set(['carpet', 'wood', 'tile']);

for (const entry of PLANS) {
  let plan;
  try {
    plan = await entry.load();
  } catch (e) {
    if (e.code === 'ERR_MODULE_NOT_FOUND') {
      test(`${entry.id}: not traced yet`, { skip: true }, () => {});
      continue;
    }
    throw e;
  }

  const outline = plan.OUTLINE ?? [[0, 0], [plan.WIDTH, 0], [plan.WIDTH, plan.DEPTH], [0, plan.DEPTH]];
  // Inside the building, allowing for half an exterior wall and a little tracing slack.
  const inside = (p, slack = 0.15) => pointInPolygon(p, outline)
    || [-1, 0, 1].some((i) => [-1, 0, 1].some((j) => pointInPolygon([p[0] + i * slack, p[1] + j * slack], outline)));

  test(`${entry.id}: exports a complete, finite plan`, () => {
    for (const key of ['SOURCE', 'WIDTH', 'DEPTH', 'ENTRANCE', 'THICKNESS', 'walls', 'rooms', 'floorPatches', 'furniture']) {
      assert.ok(plan[key] !== undefined, `missing export ${key}`);
    }
    assert.ok(plan.SOURCE.url && plan.SOURCE.credit && plan.SOURCE.title);
    assert.ok(plan.SOURCE.pxPerMetre > 0 && plan.SOURCE.originPx.length === 2 && plan.SOURCE.sizePx.length === 2);
    // JSON.stringify writes NaN and Infinity as null.
    const json = JSON.stringify([plan.walls, plan.rooms, plan.furniture, plan.floorPatches, plan.OUTLINE ?? []]);
    assert.ok(!json.includes('null'), 'no NaN, Infinity or null in the data');
    for (const v of [plan.WIDTH, plan.DEPTH, ...plan.ENTRANCE]) assert.ok(Number.isFinite(v));
    if (plan.OUTLINE) {
      assert.ok(plan.OUTLINE.every(([x, y]) => x >= -0.01 && y >= -0.01 && x <= plan.WIDTH + 0.01 && y <= plan.DEPTH + 0.01), 'OUTLINE inside the bounding box');
    }
  });

  test(`${entry.id}: wall ids are unique and every opening lies on its wall`, () => {
    assert.equal(new Set(plan.walls.map((w) => w.id)).size, plan.walls.length);
    for (const w of plan.walls) {
      assert.ok(['exterior', 'interior', 'glass'].includes(w.kind), `${w.id} kind`);
      const [dx, dy] = [w.to[0] - w.from[0], w.to[1] - w.from[1]];
      assert.ok(Math.hypot(dx, dy) > 0.05, `${w.id} has length`);
      const axis = Math.abs(dx) >= Math.abs(dy) ? 0 : 1;
      const [lo, hi] = [Math.min(w.from[axis], w.to[axis]), Math.max(w.from[axis], w.to[axis])];
      for (const o of w.openings ?? []) {
        assert.ok(o.a < o.b, `${w.id} opening ${o.a}–${o.b} is ordered`);
        assert.ok(o.a >= lo - 1e-6 && o.b <= hi + 1e-6, `${w.id} opening ${o.a}–${o.b} within ${lo}–${hi}`);
        if (o.kind !== 'window' && o.kind !== 'opening') assert.ok('NSEW'.includes(o.swing) && o.swing.length === 1, `${w.id} door swing`);
      }
      assert.ok(inside(w.from, 0.2) && inside(w.to, 0.2), `${w.id} endpoints inside the outline`);
    }
  });

  test(`${entry.id}: rooms are named, zoned, inside the building and don't overfill it`, () => {
    assert.equal(new Set(plan.rooms.map((r) => r.id)).size, plan.rooms.length);
    for (const r of plan.rooms) {
      assert.ok(r.name && ZONES.has(r.zone) && FLOORS.has(r.floor), `${r.id} name/zone/floor`);
      assert.ok(r.polygon.length >= 3 && polygonArea(r.polygon) > 0.5, `${r.id} has area`);
      assert.ok(r.polygon.every((p) => inside(p)), `${r.id} inside`);
    }
    const total = plan.rooms.reduce((s, r) => s + polygonArea(r.polygon), 0);
    assert.ok(total <= polygonArea(outline) * 1.02, `rooms ${total.toFixed(1)} m² vs building ${polygonArea(outline).toFixed(1)} m²`);
  });

  test(`${entry.id}: furniture builds in both shape styles and stays inside`, () => {
    for (const item of plan.furniture) {
      for (const style of [BLOCKY, { corner: 0.06, facets: 1.5 }]) {
        const prisms = furniturePrisms(item, style);
        assert.ok(prisms.length > 0);
        for (const p of prisms) {
          assert.ok(p.z1 > p.z0, `${item.type} at ${item.x},${item.y} has height`);
          assert.ok(p.poly.every((pt) => inside(pt)), `${item.type} at ${item.x},${item.y} inside`);
        }
      }
    }
  });

  test(`${entry.id}: walls convert and every prism projects`, () => {
    const { solids, leaves } = wallPrisms(plan.walls, 2.7, {
      thickness: plan.THICKNESS, width: plan.WIDTH, depth: plan.DEPTH, outline: plan.OUTLINE,
    });
    const view = makeView(PRESETS.iso);
    for (const p of [...solids, ...leaves]) {
      const { bbox } = projectPrism(p, view);
      assert.ok(bbox.every(Number.isFinite));
    }
    const exterior = solids.filter((p) => p.outward);
    assert.ok(exterior.length > 0, 'has exterior walls');
  });
}
