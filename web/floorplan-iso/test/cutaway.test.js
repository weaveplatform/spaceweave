import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createRoomCutaway } from '../js/cutaway.js';
import { makeView } from '../js/projection.js';

const thickness = { exterior: 0.2, interior: 0.1, glass: 0.05 };
const room = [[2, 2], [4, 2], [4, 4], [2, 4]];
const wall = (id, from, to, extra = {}) => ({ id, from, to, kind: 'interior', ...extra });
const enclosure = [
  wall('north', [2, 2], [4, 2]), wall('south', [2, 4], [4, 4]),
  wall('west', [2, 2], [2, 4]), wall('east', [4, 2], [4, 4]),
];
const view = (theta, phi = 45) => makeView({ theta, phi });

test('cutaways follow every camera quadrant and preserve rear walls on room centrelines', () => {
  const cuts = createRoomCutaway(enclosure, thickness);
  for (const [theta, expected] of [[0, ['south']], [90, ['east']], [180, ['north']], [-90, ['west']],
    [45, ['east', 'south']], [135, ['east', 'north']], [-135, ['north', 'west']], [-45, ['south', 'west']]]) {
    assert.deepEqual([...cuts(room, view(theta), 2.7)].sort(), expected, `camera ${theta}°`);
  }
});

test('cutaways find intervening walls outside the room, including glass, and leave unrelated walls', () => {
  const cuts = createRoomCutaway([
    wall('obstruction', [2, 6], [4, 6]),
    wall('glass', [2, 5], [4, 5], { kind: 'glass' }),
    wall('behind', [2, 1], [4, 1]),
    wall('too-far', [2, 9], [4, 9]),
    wall('beside', [8, 4], [9, 4]),
  ], thickness);
  assert.deepEqual([...cuts(room, view(0), 2.7)].sort(), ['glass', 'obstruction']);
});

test('occlusion uses wall height and camera elevation, with no cuts in top-down Plan', () => {
  const cuts = createRoomCutaway([wall('front', [2, 5], [4, 5])], thickness);
  assert.equal(cuts(room, view(0), 2.7).has('front'), true);
  assert.equal(cuts(room, view(0), 0.3).size, 0);
  assert.equal(cuts(room, view(0, 80), 2.7).size, 0);
  assert.equal(cuts(room, view(0, 20), 2.7).has('front'), true);
  assert.equal(cuts(room, view(0, 90), 2.7).size, 0);
  assert.equal(cuts(room, view(0), 0).size, 0);
  assert.equal(cuts(null, view(0), 2.7).size, 0);
});

test('a concave room does not cut walls whose shadows only hit its empty notch', () => {
  const polygon = [[2, 2], [5, 2], [5, 3], [3, 3], [3, 5], [2, 5]];
  const cuts = createRoomCutaway([wall('notch', [4, 4.5], [4.8, 4.5])], thickness);
  assert.equal(cuts(polygon, view(0), 1).size, 0);
  assert.equal(cuts(polygon, view(0), 2).has('notch'), true);
});

test('reversed and rotated walls use the same camera rays', () => {
  const theta = 31, angle = theta * Math.PI / 180;
  const rotate = ([x, y]) => [100 + x * Math.cos(angle) - y * Math.sin(angle), 75 + x * Math.sin(angle) + y * Math.cos(angle)];
  const walls = enclosure.map(w => ({ ...w, from: rotate(w.to), to: rotate(w.from), thickness: 0.24 }));
  const cuts = createRoomCutaway(walls, thickness);
  assert.deepEqual([...cuts(room.map(rotate), view(-theta), 2.7)], ['south']);
});
