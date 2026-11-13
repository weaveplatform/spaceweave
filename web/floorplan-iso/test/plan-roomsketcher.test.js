import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as plan from '../js/plans/roomsketcher-office.js';
import { polygonArea, furniturePrisms, wallPrisms } from '../js/geometry.js';

const near = (a, b, tol, msg) => assert.ok(Math.abs(a - b) <= tol, `${msg}: ${a} vs ${b}`);
const within = ([x, y], pad = 0) => x >= -pad && x <= plan.WIDTH + pad && y >= -pad && y <= plan.DEPTH + pad;

test('the envelope matches the dimension chains on the source plan', () => {
  near(plan.WIDTH, 4.08 + 6.56 + 4.18 + 2.30 + 2.22, 0.01, 'bottom chain');
  near(plan.DEPTH, 3.32 + 3.47 + 3.86, 0.01, 'left chain');
  near(plan.DEPTH, 3.17 + 3.61 + 2.75 + 1.11, 0.1, 'right chain');
});

test('interior walls sit where the dimension chains put them', () => {
  const wall = (id) => plan.walls.find((w) => w.id === id);
  near(wall('meet-east').from[0], 4.08 + 2.09, 0.05, 'Meetings east wall');
  near(wall('conf-east').from[0], 4.08, 0.05, 'Conference east wall');
  near(wall('partition').from[0], 4.08 + 6.56 + 4.18, 0.05, 'east partition');
  near(wall('offices-divider').from[0], 4.08 + 6.56 + 4.18 + 2.30, 0.05, 'Mike/Sarah divider');
  near(wall('corridor').from[1], 3.32 + 3.47, 0.05, 'corridor wall');
  near(wall('offices-south').from[1], 3.17, 0.05, 'office south wall');
  near(wall('wc-north').from[1], 3.17 + 3.61 + 2.75, 0.05, 'washroom wall');
  const corridorDoor = wall('corridor').openings[0];
  near(corridorDoor.b - corridorDoor.a, 1.9, 0.05, 'labelled 1.90 m door');
  const retreatDoor = wall('partition').openings[1];
  near(retreatDoor.b - retreatDoor.a, 1.4, 0.05, 'labelled 1.40 m door');
});

test('walls are axis-aligned and every opening fits on its wall', () => {
  for (const w of plan.walls) {
    const horizontal = w.from[1] === w.to[1];
    assert.ok(horizontal || w.from[0] === w.to[0], `${w.id} is axis-aligned`);
    const [lo, hi] = horizontal ? [w.from[0], w.to[0]] : [w.from[1], w.to[1]];
    for (const o of w.openings ?? []) {
      assert.ok(o.a < o.b, `${w.id} opening is ordered`);
      assert.ok(o.a >= lo && o.b <= hi, `${w.id} opening ${o.a}–${o.b} lies within ${lo}–${hi}`);
    }
  }
});

test('rooms lie inside the envelope and tile it without gaps or overlaps', () => {
  for (const r of plan.rooms) assert.ok(r.polygon.every((p) => within(p)), `${r.id} inside`);
  const total = plan.rooms.reduce((s, r) => s + polygonArea(r.polygon), 0);
  near(total, plan.WIDTH * plan.DEPTH, 0.01, 'sum of room areas');
  const meetings = plan.rooms.find((r) => r.id === 'meetings');
  near(polygonArea(meetings.polygon), 6.15 * 3.32, 0.01, 'Meetings area');
});

test('every piece of furniture builds and stays inside the building', () => {
  for (const item of plan.furniture) {
    const prisms = furniturePrisms(item);
    assert.ok(prisms.length > 0);
    for (const p of prisms) {
      assert.ok(p.poly.every((pt) => within(pt, 0.02)), `${item.type} at ${item.x},${item.y} inside`);
      assert.ok(p.z1 > p.z0);
    }
  }
});

test('RoomSketcher: all eighteen named people have a seat', () => {
  const names = plan.furniture.filter((f) => f.label).map((f) => f.label).sort();
  assert.deepEqual(names, ['Cece', 'Dana', 'Eric', 'Frank', 'John', 'Kate', 'Linda', 'Liv', 'Mike', 'Nathan', 'Ned', 'Nicole', 'Pearl', 'Ryan', 'Sarah', 'Sebastian', 'Thomas', 'Tina']);
});

test('the whole plan converts to wall prisms', () => {
  const { solids, leaves } = wallPrisms(plan.walls, 2.7, { thickness: plan.THICKNESS, width: plan.WIDTH, depth: plan.DEPTH });
  assert.ok(solids.length > 40);
  assert.equal(leaves.length, 16); // 8 single doors + 4 double doors × 2 leaves
});
