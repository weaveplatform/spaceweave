// Invariants every plan must satisfy, hand-traced or generated. Shared by
// test/plans.test.js and tools/validate.mjs (which the Go tracer runs on each
// submission), so the worker and the site's tests can't disagree about what a
// valid plan is. Returns messages instead of throwing so a tracer gets every
// problem from one submission, not just the first.
import { furniturePrisms, wallPrisms, pointInPolygon, polygonArea, projectPrism, BLOCKY } from '../js/geometry.js';
import { makeView, PRESETS } from '../js/projection.js';

export const ZONES = new Set(['meet', 'work', 'social', 'service']);
export const FLOORS = new Set(['carpet', 'wood', 'tile']);
const REQUIRED = ['SOURCE', 'WIDTH', 'DEPTH', 'ENTRANCE', 'THICKNESS', 'walls', 'rooms', 'floorPatches', 'furniture'];
// Enough for a tracer to fix the pattern without flooding its context.
const MAX_ERRORS = 60;

export function checkPlan(plan) {
  const errors = [];
  const fail = (msg) => { if (errors.length < MAX_ERRORS) errors.push(msg); };
  const guard = (label, fn) => {
    try { fn(); } catch (e) { fail(`${label}: ${e.message}`); }
  };

  for (const key of REQUIRED) if (plan[key] === undefined) fail(`missing ${key}`);
  if (errors.length) return errors;

  const outline = plan.OUTLINE ?? [[0, 0], [plan.WIDTH, 0], [plan.WIDTH, plan.DEPTH], [0, plan.DEPTH]];
  // Inside the building, allowing for half an exterior wall and a little tracing slack.
  const inside = (p, slack = 0.15) => pointInPolygon(p, outline)
    || [-1, 0, 1].some((i) => [-1, 0, 1].some((j) => pointInPolygon([p[0] + i * slack, p[1] + j * slack], outline)));
  const at = (p) => `(${p.map((v) => (+v).toFixed(2)).join(', ')})`;

  // ---- shape of the data
  const { SOURCE } = plan;
  if (!(SOURCE.url && SOURCE.credit && SOURCE.title)) fail('SOURCE needs url, credit and title');
  const scale = [SOURCE.pxPerMetre].flat();
  if (!(scale.length >= 1 && scale.length <= 2 && scale.every((v) => v > 0))) fail('SOURCE.pxPerMetre must be a positive number or [x, y]');
  if (SOURCE.originPx?.length !== 2 || SOURCE.sizePx?.length !== 2) fail('SOURCE needs originPx and sizePx');
  // JSON.stringify writes NaN and Infinity as null.
  const json = JSON.stringify([plan.walls, plan.rooms, plan.furniture, plan.floorPatches, plan.OUTLINE ?? []]);
  if (json.includes('null')) fail('NaN, Infinity or null in the data');
  for (const v of [plan.WIDTH, plan.DEPTH, ...plan.ENTRANCE]) if (!Number.isFinite(v)) fail('WIDTH, DEPTH and ENTRANCE must be finite');
  if (plan.OUTLINE && !plan.OUTLINE.every(([x, y]) => x >= -0.01 && y >= -0.01 && x <= plan.WIDTH + 0.01 && y <= plan.DEPTH + 0.01)) {
    fail('OUTLINE leaves the WIDTH × DEPTH bounding box');
  }

  // ---- walls and openings
  const ids = new Set();
  for (const w of plan.walls) {
    if (ids.has(w.id)) fail(`wall id ${w.id} is repeated`);
    ids.add(w.id);
    if (!['exterior', 'interior', 'glass'].includes(w.kind)) fail(`wall ${w.id}: kind ${w.kind}`);
    const [dx, dy] = [w.to[0] - w.from[0], w.to[1] - w.from[1]];
    if (!(Math.hypot(dx, dy) > 0.05)) fail(`wall ${w.id} is shorter than 5 cm`);
    const axis = Math.abs(dx) >= Math.abs(dy) ? 0 : 1;
    const [lo, hi] = [Math.min(w.from[axis], w.to[axis]), Math.max(w.from[axis], w.to[axis])];
    for (const o of w.openings ?? []) {
      if (!(o.a < o.b)) fail(`wall ${w.id}: opening ${o.a}–${o.b} must have a < b`);
      if (!(o.a >= lo - 1e-6 && o.b <= hi + 1e-6)) fail(`wall ${w.id}: opening ${o.a}–${o.b} runs off the wall (${lo}–${hi} along ${'xy'[axis]})`);
      if (!['door', 'double-door', 'window', 'opening'].includes(o.kind)) fail(`wall ${w.id}: opening kind ${o.kind}`);
      if ((o.kind === 'door' || o.kind === 'double-door') && !(typeof o.swing === 'string' && o.swing.length === 1 && 'NSEW'.includes(o.swing))) {
        fail(`wall ${w.id}: door swing must be N, S, E or W`);
      }
    }
    if (!(inside(w.from, 0.2) && inside(w.to, 0.2))) fail(`wall ${w.id} ${at(w.from)}→${at(w.to)} has an end outside the outline`);
  }

  // ---- rooms
  const roomIds = new Set();
  for (const r of plan.rooms) {
    if (roomIds.has(r.id)) fail(`room id ${r.id} is repeated`);
    roomIds.add(r.id);
    if (!(r.name && ZONES.has(r.zone) && FLOORS.has(r.floor))) fail(`room ${r.id}: needs a name, a zone (${[...ZONES]}) and a floor (${[...FLOORS]})`);
    if (!(r.polygon?.length >= 3 && polygonArea(r.polygon) > 0.5)) fail(`room ${r.id} has under 0.5 m² of area`);
    else if (!r.polygon.every((p) => inside(p))) fail(`room ${r.id} has a corner outside the outline`);
  }
  const total = plan.rooms.reduce((s, r) => s + (r.polygon?.length >= 3 ? polygonArea(r.polygon) : 0), 0);
  if (total > polygonArea(outline) * 1.02) fail(`rooms total ${total.toFixed(1)} m² but the building is ${polygonArea(outline).toFixed(1)} m²: rooms overlap`);

  // ---- furniture
  for (const item of plan.furniture) {
    guard(`${item.type} at ${at([item.x, item.y])}`, () => {
      for (const style of [BLOCKY, { corner: 0.06, facets: 1.5 }]) {
        for (const p of furniturePrisms(item, style)) {
          if (!(p.z1 > p.z0)) throw new Error('has no height');
          if (!p.poly.every((pt) => inside(pt))) throw new Error('pokes outside the outline');
        }
      }
    });
  }

  // ---- the renderer can build and project it
  guard('walls', () => {
    const { solids, leaves } = wallPrisms(plan.walls, 2.7, {
      thickness: plan.THICKNESS, width: plan.WIDTH, depth: plan.DEPTH, outline: plan.OUTLINE,
    });
    const view = makeView(PRESETS.iso);
    for (const p of [...solids, ...leaves]) {
      if (!projectPrism(p, view).bbox.every(Number.isFinite)) throw new Error('a wall prism projects to a non-finite point');
    }
    if (!solids.some((p) => p.outward)) throw new Error('no exterior walls');
  });

  return errors;
}
