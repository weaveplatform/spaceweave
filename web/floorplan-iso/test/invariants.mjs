// Invariants every plan must satisfy, hand-traced or generated. Shared by
// test/plans.test.js and tools/validate.mjs (which the Go tracer runs on each
// submission), so the worker and the site's tests can't disagree about what a
// valid plan is. Returns messages instead of throwing so a tracer gets every
// problem from one submission, not just the first.
import { furniturePrisms, wallPrisms, balustradePrisms, pointInPolygon, polygonArea, overlapArea, projectPrism, floorFootprint, projectFootprint, BLOCKY } from '../js/geometry.js';
import { makeView, PRESETS } from '../js/projection.js';

export const ZONES = new Set(['meet', 'work', 'social', 'service']);
export const FLOORS = new Set(['carpet', 'wood', 'tile']);
// A room is enclosed by walls; a zone is a named open-plan area with no walls
// of its own (circulation, a desk neighbourhood). A room without a kind is a
// room, so plans written before zones existed keep their meaning.
export const KINDS = new Set(['room', 'zone']);
// How much two spaces, or a space and a void or the world outside the
// outline, may share before it is an error: a hand trace's corners land a few
// centimetres off, so the larger of a quarter of a square metre (half the
// smallest room allowed, so a whole room can't hide in it) and 0.5% of the
// smaller polygon. The hand-traced suites and corporate floors have slips of
// 0.09 and 0.13 m². internal/floorgraph's siteCheck uses the same numbers.
export const OVERLAP_SLACK = { area: 0.25, share: 0.005 };
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
  const json = JSON.stringify([plan.walls, plan.rooms, plan.furniture, plan.floorPatches, plan.OUTLINE ?? [], plan.VOIDS ?? [], plan.FOOTPRINT ?? []]);
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

  // ---- voids: holes in the slab (an atrium, a light well). No space covers
  // one, or its floor would be drawn over the hole.
  const voids = Array.isArray(plan.VOIDS) ? plan.VOIDS : [];
  if (!Array.isArray(plan.VOIDS ?? [])) fail('VOIDS must be a list of polygons');
  const holes = [];
  for (const [i, v] of voids.entries()) {
    if (!(Array.isArray(v) && v.length >= 3 && polygonArea(v) > 0.5)) {
      fail(`void ${i} needs at least 3 corners and 0.5 m² of area`);
      continue;
    }
    if (!v.every((p) => inside(p))) fail(`void ${i} has a corner outside the outline`);
    holes.push({ i, poly: v, area: polygonArea(v) });
  }
  const inVoid = (p) => holes.find((h) => pointInPolygon(p, h.poly))?.i ?? -1;

  // ---- rooms and zones
  const roomIds = new Set();
  const spaces = [];
  for (const r of plan.rooms) {
    if (roomIds.has(r.id)) fail(`room id ${r.id} is repeated`);
    roomIds.add(r.id);
    const kind = r.kind ?? 'room';
    if (!KINDS.has(kind)) fail(`room ${r.id}: kind must be one of ${[...KINDS]}, got ${kind}`);
    if (!(r.name && ZONES.has(r.zone) && FLOORS.has(r.floor))) fail(`room ${r.id}: needs a name, a zone (${[...ZONES]}) and a floor (${[...FLOORS]})`);
    if (!(r.polygon?.length >= 3 && polygonArea(r.polygon) > 0.5)) {
      fail(`${kind} ${r.id} has under 0.5 m² of area`);
      continue;
    }
    if (!r.polygon.every((p) => inside(p))) fail(`${kind} ${r.id} has a corner outside the outline`);
    // The label is the one point a space promises is inside it.
    if (r.label && inVoid(r.label) >= 0) fail(`${kind} ${r.id} is labelled inside void ${inVoid(r.label)}`);
    spaces.push({ ...spaceBox(r.polygon), id: r.id, kind, poly: r.polygon, area: polygonArea(r.polygon) });
  }
  const slack = (a, b) => Math.max(OVERLAP_SLACK.area, OVERLAP_SLACK.share * Math.min(a, b));
  const outlineArea = polygonArea(outline);
  for (const s of spaces) {
    const out = s.area - overlapArea(s.poly, outline);
    if (out > slack(s.area, outlineArea)) fail(`${s.kind} ${s.id} has ${out.toFixed(2)} m² outside the outline`);
    for (const h of holes) {
      const o = overlapArea(s.poly, h.poly);
      if (o > slack(s.area, h.area)) fail(`${s.kind} ${s.id} covers ${o.toFixed(2)} m² of void ${h.i}`);
    }
  }
  // Rooms tile the floor between their walls and zones fill what is left, so
  // no two spaces of either kind may share floor. Sorting by left edge and
  // stopping at the first space that starts right of this one ends keeps it
  // well under quadratic for a floor of a few hundred spaces.
  spaces.sort((p, q) => p.x0 - q.x0);
  for (let i = 0; i < spaces.length; i++) {
    const a = spaces[i];
    for (let j = i + 1; j < spaces.length && spaces[j].x0 < a.x1; j++) {
      const b = spaces[j];
      if (!(b.y0 < a.y1 && a.y0 < b.y1)) continue;
      const o = overlapArea(a.poly, b.poly);
      if (o > slack(a.area, b.area)) fail(`${a.kind} ${a.id} and ${b.kind} ${b.id} overlap by ${o.toFixed(2)} m²`);
    }
  }

  checkFootprint(plan.FOOTPRINT, spaces, holes, fail);

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
    for (const p of [...solids, ...leaves, ...balustradePrisms(voids, 2.7)]) {
      if (!projectPrism(p, view).bbox.every(Number.isFinite)) throw new Error('a wall prism projects to a non-finite point');
    }
    if (!solids.some((p) => p.outward)) throw new Error('no exterior walls');
  });

  guard('slab', () => {
    const faces = projectFootprint(floorFootprint(plan), makeView(PRESETS.iso), -.22, 0);
    for (const f of faces) {
      const points = f.rings?.flat() ?? f.pts;
      if (!points.every((p) => p.every(Number.isFinite))) throw new Error('slab projects to a non-finite point');
    }
  });

  return errors;
}

function spaceBox(poly) {
  const xs = poly.map((p) => p[0]);
  const ys = poly.map((p) => p[1]);
  return { x0: Math.min(...xs), x1: Math.max(...xs), y0: Math.min(...ys), y1: Math.max(...ys) };
}

function checkFootprint(footprint, spaces, voids, fail) {
  if (footprint === undefined) return;
  if (!Array.isArray(footprint) || !footprint.length) {
    fail('FOOTPRINT needs at least one polygon');
    return;
  }
  const polygons = [];
  footprint.forEach((polygon, i) => {
    if (!Array.isArray(polygon) || !polygon.length) {
      fail(`footprint ${i} needs an outer ring`);
      return;
    }
    if (!polygon.every((ring) => Array.isArray(ring) && ring.length >= 3 &&
      ring.every((p) => Array.isArray(p) && p.length === 2 && p.every(Number.isFinite)) && polygonArea(ring) > 1e-8)) {
      fail(`footprint ${i} needs finite rings with at least three points and positive area`);
      return;
    }
    const [outer, ...holes] = polygon;
    for (const hole of holes) {
      if (polygonArea(hole) - overlapArea(hole, outer) > .001) fail(`footprint ${i} has a hole outside its outer ring`);
    }
    for (let a = 0; a < holes.length; a++) for (let b = a + 1; b < holes.length; b++) {
      if (overlapArea(holes[a], holes[b]) > .001) fail(`footprint ${i} has overlapping holes`);
    }
    polygons.push(polygon);
  });
  const netOverlap = ([outer, ...holes], ring) => overlapArea(outer, ring) - holes.reduce((s, h) => s + overlapArea(h, ring), 0);
  const coverage = (ring) => polygons.reduce((s, polygon) => s + netOverlap(polygon, ring), 0);
  const slack = (area) => Math.max(OVERLAP_SLACK.area, OVERLAP_SLACK.share * area);
  for (let i = 0; i < polygons.length; i++) for (let j = i + 1; j < polygons.length; j++) {
    const [outer, ...holes] = polygons[j];
    const overlap = netOverlap(polygons[i], outer) - holes.reduce((s, h) => s + netOverlap(polygons[i], h), 0);
    if (overlap > .001) fail(`footprints ${i} and ${j} overlap`);
  }
  for (const s of spaces) {
    if (s.area - coverage(s.poly) > slack(s.area)) fail(`${s.kind} ${s.id} leaves the slab footprint`);
  }
  for (const h of voids) {
    if (coverage(h.poly) > slack(h.area)) fail(`slab footprint covers void ${h.i}`);
  }
}
