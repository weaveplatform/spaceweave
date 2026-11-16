import { test } from 'node:test';
import assert from 'node:assert/strict';
import { compare, summary } from '../tools/compare.mjs';
import * as office from '../js/plans/roomsketcher-office.js';

const hand = { ...office };

// The same drawing calibrated differently: 10% too many px per metre on x and
// the origin elsewhere. Every px position is unchanged.
function recalibrated(plan, [kx, ky], [dx, dy]) {
  const [sx, sy] = Array.isArray(plan.SOURCE.pxPerMetre) ? plan.SOURCE.pxPerMetre : [plan.SOURCE.pxPerMetre, plan.SOURCE.pxPerMetre];
  const nsx = sx * kx;
  const nsy = sy * ky;
  const [ox, oy] = plan.SOURCE.originPx;
  const p = ([x, y]) => [(x * sx + ox - (ox + dx)) / nsx, (y * sy + oy - (oy + dy)) / nsy];
  return {
    ...plan,
    SOURCE: { ...plan.SOURCE, pxPerMetre: [nsx, nsy], originPx: [ox + dx, oy + dy] },
    WIDTH: (plan.WIDTH * sx) / nsx,
    DEPTH: (plan.DEPTH * sy) / nsy,
    walls: plan.walls.map((w) => ({ ...w, from: p(w.from), to: p(w.to) })),
  };
}

test('a plan scores perfectly against itself', () => {
  const r = compare(hand, hand);
  assert.deepEqual(r.scaleError, [0, 0]);
  assert.equal(r.wallRecall, 1);
  assert.equal(r.wallPrecision, 1);
  assert.equal(r.rooms.matched, hand.rooms.length);
  assert.equal(r.furnitureRatio, 1);
});

test('a calibration error shows up as scale error, not as misplaced walls', () => {
  const r = compare(recalibrated(hand, [1.1, 1], [-12, 7]), hand);
  assert.ok(Math.abs(r.scaleError[0] - 0.1) < 1e-9);
  assert.ok(Math.abs(r.scaleError[1]) < 1e-9);
  assert.ok(r.wallRecall > 0.999);
  assert.ok(Math.abs(r.size.auto[0] - hand.WIDTH) < 1e-6);
});

test('misplaced walls, missing rooms and furniture are counted', () => {
  const auto = {
    ...hand,
    walls: hand.walls.map((w) => ({ ...w, from: [w.from[0] + 1, w.from[1] + 1], to: [w.to[0] + 1, w.to[1] + 1], openings: [] })),
    rooms: [...hand.rooms.slice(1), { name: 'Server room' }],
    furniture: hand.furniture.slice(0, Math.floor(hand.furniture.length / 2)),
  };
  const r = compare(auto, hand);
  assert.ok(r.wallRecall < 0.5, `recall ${r.wallRecall}`);
  assert.equal(r.rooms.matched, hand.rooms.length - 1);
  assert.deepEqual(r.rooms.missing, [hand.rooms[0].name]);
  assert.deepEqual(r.rooms.extra, ['server room']);
  assert.ok(r.furnitureRatio < 0.51);
  assert.deepEqual(r.openings.auto, {});
  assert.match(summary(r), /rooms by name +\d+\/\d+[\s\S]*missing[\s\S]*extra/);
});
