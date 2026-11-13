import { test } from 'node:test';
import assert from 'node:assert/strict';
import { makeView, PRESETS } from '../js/projection.js';
import {
  solidRuns, splitRun, wallPrisms, prism, rectPoly, projectPrism, compareFootprints, orderItems, signedArea, DOOR_HEAD,
} from '../js/geometry.js';

const opts = { thickness: { exterior: 0.15, interior: 0.1, glass: 0.05 }, width: 10, depth: 10 };

test('openings are removed from a wall run', () => {
  assert.deepEqual(solidRuns(0, 5, [[3, 4], [1, 2]]), [[0, 1], [2, 3], [4, 5]]);
  assert.deepEqual(solidRuns(0, 5, [[0, 5]]), []);
});

test('splitting a run keeps its length and respects the cap', () => {
  const pieces = splitRun([0.2, 3.7], 0.5);
  const total = pieces.reduce((s, [a, b]) => s + (b - a), 0);
  assert.ok(Math.abs(total - 3.5) < 1e-9);
  assert.ok(pieces.every(([a, b]) => b - a <= 0.5 + 1e-9));
});

test('a wall with one door becomes two runs, a lintel and a leaf', () => {
  const wall = { id: 'w', kind: 'interior', from: [0, 5], to: [4, 5], openings: [{ a: 1, b: 2, kind: 'door', swing: 'N', hinge: 'a' }] };
  const { solids, leaves } = wallPrisms([wall], 2.7, opts);
  const full = solids.filter((p) => p.z0 === 0);
  const lintels = solids.filter((p) => p.z0 === DOOR_HEAD);
  assert.equal(full.length, 2);
  assert.equal(lintels.length, 1);
  assert.equal(leaves.length, 1);
  // The leaf opens to the north, so it lies entirely above (smaller y than) the wall.
  assert.ok(leaves[0].poly.every(([, y]) => y < 5));
});

test('low walls lose their lintels', () => {
  const wall = { id: 'w', kind: 'interior', from: [0, 0], to: [4, 0], openings: [{ a: 1, b: 2, kind: 'door', swing: 'S' }] };
  const { solids } = wallPrisms([wall], 1.2, opts);
  assert.ok(solids.every((p) => p.z1 <= 1.2));
  assert.equal(solids.length, 2);
});

test('a window keeps a sill, a pane and a header', () => {
  const wall = { id: 'w', kind: 'exterior', from: [0, 0], to: [6, 0], openings: [{ a: 2, b: 4, kind: 'window', sill: 0.9, head: 2.1 }] };
  const { solids } = wallPrisms([wall], 2.7, opts);
  const mats = solids.filter((p) => p.poly.every(([x]) => x >= 2 - 1e-9 && x <= 4 + 1e-9)).map((p) => `${p.mat}:${p.z0}-${p.z1}`);
  assert.deepEqual(mats.sort(), ['glass:0.9-2.1', 'wall:0-0.9', 'wall:2.1-2.7']);
  assert.ok(solids.every((p) => p.outward && p.outward[1] === -1), 'north wall faces north');
});

test('footprints are normalised to positive orientation', () => {
  const p = prism([[0, 0], [0, 1], [1, 1], [1, 0]], 0, 1);
  assert.ok(signedArea(p.poly) > 0);
});

test('an axis-aligned box seen in isometric shows two sides and its top', () => {
  const box = prism(rectPoly(0, 0, 1, 1), 0, 1);
  const { faces } = projectPrism(box, makeView(PRESETS.iso));
  assert.equal(faces.length, 3);
  assert.equal(faces.at(-1).side, 'top');
  const { faces: fromAbove } = projectPrism(box, makeView(PRESETS.plan));
  assert.equal(fromAbove.length, 1);
});

test('the footprint nearer the camera is drawn later', () => {
  const view = makeView({ theta: 0, phi: 35 }); // camera due south (+y)
  const far = prism(rectPoly(0, 0, 4, 0.1), 0, 2.7, { kind: 'wall' });
  const near = prism(rectPoly(1.5, 1, 0.5, 0.5), 0, 0.9);
  assert.equal(compareFootprints(far, near, view), -1);
  assert.equal(compareFootprints(near, far, view), 1);
});

test('a long wall is drawn before a chair in front of its far end', () => {
  // Sorting by centre depth alone gets this wrong: the wall's centre is nearer
  // than the chair, but the chair stands in front of the wall's far end.
  const view = makeView(PRESETS.iso);
  const wall = prism(rectPoly(5, 0, 10, 0.1), 0, 2.7, { kind: 'wall' });
  const chair = prism(rectPoly(1, 0.6, 0.5, 0.5), 0, 0.9);
  const items = [wall, chair].map((p) => {
    const { bbox } = projectPrism(p, view);
    const [, yr] = view.rotate(...p.poly[0]);
    return { ...p, z0e: p.z0, z1e: p.z1, bbox, key: p === wall ? 10 : 0 };
  });
  const order = orderItems(items, view);
  assert.equal(order[0].poly, wall.poly);
});

test('stacked parts draw bottom up', () => {
  const view = makeView(PRESETS.iso);
  const post = prism(rectPoly(0, 0, 0.1, 0.1), 0, 0.4);
  const seat = prism(rectPoly(0, 0, 0.5, 0.5), 0.4, 0.47);
  const items = [seat, post].map((p) => ({ ...p, z0e: p.z0, z1e: p.z1, bbox: projectPrism(p, view).bbox, key: 0 }));
  assert.deepEqual(orderItems(items, view).map((i) => i.z0e), [0, 0.4]);
});
