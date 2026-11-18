import { test } from 'node:test';
import assert from 'node:assert/strict';
import { makeView, PRESETS } from '../js/projection.js';
import {
  solidRuns, splitRun, wallPrisms, balustradePrisms, BALUSTRADE, prism, rectPoly, projectPrism, compareFootprints, orderItems, signedArea, overlapArea, DOOR_HEAD, LEAF_THICKNESS,
  floorFootprint, pointInFootprint, projectFootprint,
} from '../js/geometry.js';

const near = (a, b, msg) => assert.ok(Math.abs(a - b) < 1e-9, `${msg}: ${a} vs ${b}`);
const sq = (x0, y0, x1, y1) => [[x0, y0], [x1, y0], [x1, y1], [x0, y1]];

test('overlapArea is the floor two polygons share, whatever their winding', () => {
  near(overlapArea(sq(0, 0, 4, 4), sq(2, 1, 6, 3)), 4, 'overlapping squares');
  near(overlapArea(sq(0, 0, 4, 4), [...sq(2, 1, 6, 3)].reverse()), 4, 'opposite winding');
  near(overlapArea(sq(0, 0, 4, 4), sq(1, 1, 2, 2)), 1, 'one inside the other');
  near(overlapArea(sq(0, 0, 4, 4), sq(5, 0, 6, 4)), 0, 'apart');
  // A diamond over a square: the square's corners stick out as four
  // triangles of 0.5 each.
  near(overlapArea([[2, 0], [4, 2], [2, 4], [0, 2]], sq(0, 0, 4, 4)), 8, 'diamond');
  near(overlapArea([[2, 0], [4, 2], [2, 4], [0, 2]], sq(1, 1, 3, 3)), 4, 'square in a diamond');
  // Crossing slanted edges inside one slab: a bow-tie shaped overlap.
  near(overlapArea([[0, 0], [2, 0], [0, 2]], [[0, 0], [2, 0], [2, 2]]), 1, 'two triangles');
  // An L-shape round a square it doesn't cover.
  const ell = [[0, 0], [4, 0], [4, 1], [1, 1], [1, 4], [0, 4]];
  near(overlapArea(ell, sq(1, 1, 4, 4)), 0, 'concave neighbour');
  near(overlapArea(ell, sq(0, 0, 4, 4)), 7, 'concave inside');
});

test('spaces that only share an edge or a corner overlap by nothing', () => {
  near(overlapArea(sq(0, 0, 2, 2), sq(2, 0, 4, 2)), 0, 'shared side');
  near(overlapArea(sq(0, 0, 2, 2), sq(0, 2, 2, 4)), 0, 'shared top');
  near(overlapArea(sq(0, 0, 2, 2), sq(2, 2, 4, 4)), 0, 'shared corner');
  near(overlapArea(sq(0, 0, 2, 2), sq(1, 2, 3, 4)), 0, 'part of a side');
  near(overlapArea([[0, 0], [3, 0], [0, 3]], [[3, 0], [3, 3], [0, 3]]), 0, 'shared diagonal');
  near(overlapArea([[0, 0], [1, 0]], sq(0, 0, 2, 2)), 0, 'a line has no area');
});

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

test('door leaves are rectangular and perpendicular to angled walls in either direction', () => {
  const swingVector = { N: [0, -1], S: [0, 1], E: [1, 0], W: [-1, 0] };
  for (const angle of [0, 23, 45, 67, 90, 113, 157, 203, 247, 293, 337]) {
    const a = angle * Math.PI / 180;
    const u = [Math.cos(a), Math.sin(a)];
    const axis = Math.abs(u[0]) > Math.abs(u[1]) ? 0 : 1;
    const ends = [1, 2].map((t) => 5 + t * u[axis]).sort((x, y) => x - y);
    for (const swing of axis === 0 ? ['N', 'S'] : ['E', 'W']) {
      for (const hinge of ['a', 'b']) {
        const wall = { id: 'angled', kind: 'interior', from: [5, 5], to: [5 + 4 * u[0], 5 + 4 * u[1]],
          openings: [{ a: ends[0], b: ends[1], kind: 'door', swing, hinge }] };
        const { leaves } = wallPrisms([wall], 2.7, opts);
        const p = leaves[0].poly;
        const edges = p.map((v, i) => [p[(i + 1) % 4][0] - v[0], p[(i + 1) % 4][1] - v[1]]);
        const long = edges.find((v) => Math.hypot(...v) > .5);
        near(long[0] * u[0] + long[1] * u[1], 0, `angle ${angle}: leaf perpendicular to wall`);
        near(Math.abs(signedArea(p)), LEAF_THICKNESS, `angle ${angle}: leaf area`);
        const centre = p.reduce((s, v) => [s[0] + v[0] / 4, s[1] + v[1] / 4], [0, 0]);
        const along = (centre[0] - 5) * u[0] + (centre[1] - 5) * u[1];
        const hingeAt = (hinge === 'a' ? ends[0] : ends[1]) - 5;
        near(along, hingeAt / u[axis] + (hinge === 'a' ? 1 : -1) * Math.sign(u[axis]) * .03,
          `angle ${angle}: same physical hinge`);
        const normal = [-u[1], u[0]];
        const side = (centre[0] - 5) * normal[0] + (centre[1] - 5) * normal[1];
        assert.ok(side * (normal[0] * swingVector[swing][0] + normal[1] * swingVector[swing][1]) > 0,
          `angle ${angle}: requested swing side`);
      }
    }
  }
});

test('a supplied slab retains recesses, holes and disconnected floor without a bounding rectangle', () => {
  const shell = [[0, 0], [4, 0], [4, 1], [2, 1], [2, 4], [0, 4]];
  const hole = sq(.5, 1, 1.5, 2);
  const polygons = floorFootprint({ ...opts, FOOTPRINT: [[shell, hole], [sq(6, 0, 7, 1)]] });
  assert.ok(pointInFootprint([3, .5], polygons), 'bay retained');
  assert.ok(!pointInFootprint([3, 3], polygons), 'recess remains outside');
  assert.ok(!pointInFootprint([1, 1.5], polygons), 'hole remains open');
  assert.ok(pointInFootprint([6.5, .5], polygons), 'separate component retained');
  assert.ok(signedArea(polygons[0][1]) < 0, 'hole sides face into the opening');
  const faces = projectFootprint(polygons, makeView(PRESETS.iso), -.22, 0);
  assert.equal(new Set(faces.map((f) => f.id)).size, faces.length);
  assert.equal(faces.filter((f) => f.side === 'top').length, 2);
  assert.equal(faces.find((f) => f.id === '0/top').rings.length, 2);
  assert.ok(faces.some((f) => f.id.startsWith('0/1/')), 'inner vertical faces are present');
  assert.ok(faces.flatMap((f) => f.rings?.flat() ?? f.pts).every((p) => p.every(Number.isFinite)));
});

test('legacy slab fallback keeps its wall allowance and voids', () => {
  const polygons = floorFootprint({ WIDTH: 10, DEPTH: 8, THICKNESS: opts.thickness, VOIDS: [sq(2, 2, 3, 3)] });
  near(signedArea(polygons[0][0]), 10.15 * 8.15, 'fallback area');
  assert.ok(!pointInFootprint([2.5, 2.5], polygons));
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

test('a void gets a balustrade along each edge, but not along the slit to a core inside it', () => {
  // A 6 × 4 void round a 2 × 1 core, joined by a slit from (3, 0) down to (3, 1.5).
  const ring = [[0, 0], [3, 0], [3, 1.5], [2, 1.5], [2, 2.5], [4, 2.5], [4, 1.5], [3, 1.5], [3, 0], [6, 0], [6, 4], [0, 4]];
  const ps = balustradePrisms([ring], 2.7);
  assert.equal(ps.length, ring.length - 2);
  assert.ok(ps.every((p) => p.z1 === BALUSTRADE.height && p.mat === 'glass' && p.outward === null));
  assert.ok(balustradePrisms([ring], 0.8).every((p) => p.z1 === 0.8), 'no taller than the cut walls');
  assert.deepEqual(balustradePrisms(undefined, 2.7), []);
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
