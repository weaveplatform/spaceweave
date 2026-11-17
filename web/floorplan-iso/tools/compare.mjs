#!/usr/bin/env node
// Scores an automatic trace against the hand trace of the same image, e.g.
//
//   node tools/compare.mjs generated/<id>/plan.json js/plans/pt-clinic.js
//
// Each plan is in its own metres with its own origin, so the auto plan's
// geometry is first carried through source pixels into the hand plan's frame.
// That keeps a calibration error out of the geometry scores; the scale error
// is reported on its own.
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { pxScale } from '../js/plans/kit.js';

export const TOLERANCE = 0.25; // metres
const STEP = 0.05;

const ratio = (a, b) => (b === 0 ? (a === 0 ? 1 : Infinity) : a / b);

function segDist([px, py], [ax, ay], [bx, by]) {
  const dx = bx - ax;
  const dy = by - ay;
  const len2 = dx * dx + dy * dy;
  const t = len2 === 0 ? 0 : Math.max(0, Math.min(1, ((px - ax) * dx + (py - ay) * dy) / len2));
  return Math.hypot(px - (ax + t * dx), py - (ay + t * dy));
}

// Share of the length of `walls` lying within TOLERANCE of some wall in `other`.
function covered(walls, other) {
  let total = 0;
  let near = 0;
  for (const w of walls) {
    const len = Math.hypot(w.to[0] - w.from[0], w.to[1] - w.from[1]);
    const n = Math.max(1, Math.ceil(len / STEP));
    for (let i = 0; i < n; i++) {
      const t = (i + 0.5) / n;
      const p = [w.from[0] + t * (w.to[0] - w.from[0]), w.from[1] + t * (w.to[1] - w.from[1])];
      total += len / n;
      if (other.some((o) => segDist(p, o.from, o.to) <= TOLERANCE)) near += len / n;
    }
  }
  return total === 0 ? 1 : near / total;
}

const normName = (s) => String(s ?? '').toLowerCase().replace(/[^a-z0-9]+/g, ' ').trim();

function countBy(items, key) {
  const out = {};
  for (const it of items) out[key(it)] = (out[key(it)] ?? 0) + 1;
  return out;
}

/** Carries auto's plan-space points into hand's plan space via source pixels. */
export function reframe(auto, hand) {
  const [ax, ay] = pxScale(auto.SOURCE);
  const [hx, hy] = pxScale(hand.SOURCE);
  const [aox, aoy] = auto.SOURCE.originPx;
  const [hox, hoy] = hand.SOURCE.originPx;
  return ([x, y]) => [(x * ax + aox - hox) / hx, (y * ay + aoy - hoy) / hy];
}

export function compare(auto, hand) {
  const f = reframe(auto, hand);
  const walls = auto.walls.map((w) => ({ ...w, from: f(w.from), to: f(w.to) }));
  const [ax, ay] = pxScale(auto.SOURCE);
  const [hx, hy] = pxScale(hand.SOURCE);

  const pool = auto.rooms.map((r) => normName(r.name));
  const missing = [];
  for (const r of hand.rooms) {
    const i = pool.indexOf(normName(r.name));
    if (i === -1) missing.push(r.name);
    else pool.splice(i, 1);
  }

  const openings = (p) => countBy(p.walls.flatMap((w) => w.openings ?? []), (o) => o.kind);
  return {
    scaleError: [ax / hx - 1, ay / hy - 1],
    size: { auto: [auto.WIDTH * (ax / hx), auto.DEPTH * (ay / hy)], hand: [hand.WIDTH, hand.DEPTH] },
    wallRecall: covered(hand.walls, walls),
    wallPrecision: covered(walls, hand.walls),
    rooms: { matched: hand.rooms.length - missing.length, hand: hand.rooms.length, auto: auto.rooms.length, missing, extra: pool },
    openings: { auto: openings(auto), hand: openings(hand) },
    furnitureRatio: ratio(auto.furniture.length, hand.furniture.length),
    furniture: { auto: auto.furniture.length, hand: hand.furniture.length },
  };
}

const KEYS = ['SOURCE', 'WIDTH', 'DEPTH', 'OUTLINE', 'VOIDS', 'ENTRANCE', 'THICKNESS', 'walls', 'rooms', 'floorPatches', 'furniture'];

async function load(path) {
  if (path.endsWith('.json')) return JSON.parse(await readFile(path, 'utf8'));
  const mod = await import(pathToFileURL(resolve(path)).href);
  return Object.fromEntries(KEYS.filter((k) => mod[k] !== undefined).map((k) => [k, mod[k]]));
}

const pct = (x) => `${(x * 100).toFixed(1)}%`;

export function summary(r) {
  const sign = (x) => `${x >= 0 ? '+' : ''}${pct(x)}`;
  const m = ([w, d]) => `${w.toFixed(2)} × ${d.toFixed(2)} m`;
  const kinds = [...new Set([...Object.keys(r.openings.hand), ...Object.keys(r.openings.auto)])].sort();
  return [
    `scale error      x ${sign(r.scaleError[0])}, y ${sign(r.scaleError[1])}`,
    `size             auto ${m(r.size.auto)} vs hand ${m(r.size.hand)} (auto in hand's scale)`,
    `walls            ${pct(r.wallRecall)} of hand length matched within ${TOLERANCE} m; ${pct(r.wallPrecision)} of auto length is near a hand wall`,
    `rooms by name    ${r.rooms.matched}/${r.rooms.hand} (auto has ${r.rooms.auto})`,
    ...(r.rooms.missing.length ? [`  missing        ${r.rooms.missing.join(', ')}`] : []),
    ...(r.rooms.extra.length ? [`  extra          ${r.rooms.extra.join(', ')}`] : []),
    `openings         ${kinds.map((k) => `${k} ${r.openings.auto[k] ?? 0}/${r.openings.hand[k] ?? 0}`).join(', ')}`,
    `furniture        ${r.furniture.auto}/${r.furniture.hand} (ratio ${r.furnitureRatio.toFixed(2)})`,
  ].join('\n');
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) {
  const args = process.argv.slice(2);
  const json = args[0] === '--json';
  const [autoPath, handPath] = json ? args.slice(1) : args;
  if (!autoPath || !handPath) {
    console.error('usage: compare.mjs [--json] <auto plan.json> <hand plan module or json>');
    process.exit(2);
  }
  const r = compare(await load(autoPath), await load(handPath));
  process.stdout.write(json ? `${JSON.stringify(r)}\n` : `${summary(r)}\n`);
}
