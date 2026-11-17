// checkPlan must catch each kind of mistake a tracer makes, with a message
// that says where. The Office plan is valid, so each test breaks one thing.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as office from '../js/plans/roomsketcher-office.js';
import { checkPlan } from './invariants.mjs';

const rect = (x0, y0, x1, y1) => [[x0, y0], [x1, y0], [x1, y1], [x0, y1]];
const copy = () => structuredClone(Object.fromEntries(Object.entries(office)));
const expectError = (plan, pattern) => {
  const errors = checkPlan(plan);
  assert.ok(errors.some((e) => pattern.test(e)), `expected ${pattern} in ${JSON.stringify(errors)}`);
};

test('the Office plan is valid', () => {
  assert.deepEqual(checkPlan(copy()), []);
});

test('missing exports are listed and stop the checks', () => {
  assert.deepEqual(checkPlan({ ...copy(), rooms: undefined, walls: undefined }), ['missing walls', 'missing rooms']);
});

test('wall mistakes', () => {
  const plan = copy();
  plan.walls.push({ ...plan.walls[0] });
  plan.walls[1].kind = 'brick';
  plan.walls[2].to = [...plan.walls[2].from];
  plan.walls[3].from = [-5, -5];
  expectError(plan, new RegExp(`wall id ${plan.walls[0].id} is repeated`));
  expectError(plan, /kind brick/);
  expectError(plan, /shorter than 5 cm/);
  expectError(plan, /end outside the outline/);
});

test('opening mistakes', () => {
  const plan = copy();
  const w = plan.walls.find((x) => x.openings?.length);
  const [o] = w.openings;
  w.openings = [
    { ...o, a: o.b, b: o.a },
    { ...o, a: -100, b: -99 },
    { a: o.a, b: o.b, kind: 'portal' },
    { a: o.a, b: o.b, kind: 'door', swing: 'up' },
  ];
  expectError(plan, /must have a < b/);
  expectError(plan, /runs off the wall/);
  expectError(plan, /opening kind portal/);
  expectError(plan, /swing must be N, S, E or W/);
});

test('room mistakes', () => {
  const plan = copy();
  plan.rooms[0].zone = 'lounge';
  plan.rooms[1].polygon = [[0, 0], [0.1, 0], [0.1, 0.1]];
  plan.rooms[2].polygon = plan.rooms[2].polygon.map(([x, y]) => [x + 100, y]);
  plan.rooms.push({ ...plan.rooms[3] });
  for (let i = 0; i < 3; i++) plan.rooms.push({ ...plan.rooms[4], id: `dup-${i}` });
  expectError(plan, /needs a name, a zone/);
  expectError(plan, /under 0.5 m²/);
  expectError(plan, /corner outside the outline/);
  expectError(plan, /is repeated/);
  expectError(plan, /room sarah and room dup-0 overlap by 6\.94 m²/);
});

test('spaces are rooms unless they say they are zones', () => {
  const plan = copy();
  // The open office has no walls between it and the conference room's door:
  // a zone. Its outline is unchanged, so it still tiles with the rooms.
  plan.rooms.find((r) => r.id === 'open-office').kind = 'zone';
  plan.rooms.find((r) => r.id === 'mike').kind = 'room';
  assert.deepEqual(checkPlan(plan), []);
  plan.rooms[0].kind = 'corridor';
  expectError(plan, /room meetings: kind must be one of room,zone, got corridor/);
});

test('spaces may share an edge or a tracing slip, never floor', () => {
  const plan = copy();
  const byId = (id) => plan.rooms.find((r) => r.id === id);
  // Mike's office reaches 4 cm into Sarah's: 0.13 m², a slip.
  byId('mike').polygon = rect(14.82, 0, 17.19, 3.17);
  assert.deepEqual(checkPlan(plan), []);
  // Half a metre is a room drawn over another.
  byId('mike').polygon = rect(14.82, 0, 17.65, 3.17);
  expectError(plan, /room mike and room sarah overlap by 1\.58 m²/);
});

test('zones may not overlap each other or a room', () => {
  const plan = copy();
  const byId = (id) => plan.rooms.find((r) => r.id === id);
  byId('open-office').kind = 'zone';
  byId('coffee').kind = 'zone';
  byId('coffee').polygon = rect(0, 6, 10.64, 9.53);
  expectError(plan, /zone coffee and zone open-office overlap by 5\.19 m²/);
  expectError(plan, /room conference and zone coffee overlap by 3\.22 m²/);
  byId('coffee').polygon = rect(0, 6.79, 10.64, 9.53);
  byId('retreat').polygon = rect(10, 6.79, 14.82, 9.53);
  byId('retreat').label = [12.6, 8.25];
  expectError(plan, /zone coffee and room retreat overlap by 1\.75 m²/);
});

test('a zone stays inside the building and out of its voids', () => {
  const plan = copy();
  const zone = plan.rooms.find((r) => r.id === 'open-office');
  zone.kind = 'zone';
  // Each corner within the outline's 15 cm slack, but the whole north edge
  // 14 cm out: 1.2 m² of floor that isn't there.
  zone.polygon = zone.polygon.map(([x, y]) => [x, y === 0 ? -0.14 : y]);
  expectError(plan, /zone open-office has 1\.21 m² outside the outline/);
  zone.polygon = zone.polygon.map(([x, y]) => [x, Math.max(0, y)]);
  plan.VOIDS = [rect(8, 1, 10, 3)];
  expectError(plan, /zone open-office covers 4\.00 m² of void 0/);
  zone.label = [9, 2];
  expectError(plan, /zone open-office is labelled inside void 0/);
});

test('a floor of hundreds of spaces is checked quickly', () => {
  const plan = copy();
  // A 20 × 10 grid of 0.9 m cells over the open office and coffee room,
  // every one sharing its edges with its neighbours.
  plan.rooms = [];
  for (let i = 0; i < 20; i++) {
    for (let j = 0; j < 10; j++) {
      plan.rooms.push({
        id: `c${i}-${j}`, name: 'Cell', zone: 'work', floor: 'carpet', kind: (i + j) % 2 ? 'zone' : 'room',
        polygon: rect(0.5 + i * 0.9, 0.3 + j * 0.9, 1.4 + i * 0.9, 1.2 + j * 0.9),
      });
    }
  }
  const t0 = performance.now();
  assert.deepEqual(checkPlan(plan), []);
  assert.ok(performance.now() - t0 < 1000);
});

test('a void is a hole in the floor that no room covers', () => {
  const plan = copy();
  const i = plan.rooms.findIndex((r) => r.id === 'server');
  plan.VOIDS = [plan.rooms[i].polygon];
  plan.rooms.splice(i, 1);
  assert.deepEqual(checkPlan(plan), []);
});

test('void mistakes', () => {
  const plan = copy();
  const server = plan.rooms.find((r) => r.id === 'server');
  plan.VOIDS = [server.polygon, [[0, 0], [1, 0]], [[-20, -20], [-18, -20], [-18, -18], [-20, -18]]];
  plan.rooms.find((r) => r.id === 'file').label = server.polygon[0].map((v) => v + 0.1);
  expectError(plan, /void 1 needs at least 3 corners/);
  expectError(plan, /void 2 has a corner outside the outline/);
  expectError(plan, /room file is labelled inside void 0/);
  expectError(plan, /room server covers 2\.49 m² of void 0/);
  plan.VOIDS = [[[0, 0], [NaN, 0], [1, 1]]];
  expectError(plan, /NaN, Infinity or null/);
  plan.VOIDS = 'atrium';
  expectError(plan, /VOIDS must be a list/);
});

test('furniture mistakes', () => {
  const plan = copy();
  plan.furniture.push({ type: 'hovercraft', x: 1, y: 1 });
  plan.furniture.push({ type: 'cabinet', x: 1, y: 1, w: 1, d: 1, h: 0 });
  plan.furniture.push({ type: 'cabinet', x: 500, y: 1, w: 1, d: 1, h: 1 });
  expectError(plan, /unknown furniture type hovercraft/);
  expectError(plan, /has no height/);
  expectError(plan, /cabinet at \(500\.00, 1\.00\): pokes outside/);
});

test('SOURCE and number mistakes', () => {
  const plan = copy();
  plan.ENTRANCE = [NaN, 0];
  plan.SOURCE = { ...plan.SOURCE, pxPerMetre: [1, 2, 3], credit: '', originPx: [0] };
  plan.OUTLINE = [[0, 0], [50, 0], [50, 50], [0, 50]];
  expectError(plan, /must be finite/);
  expectError(plan, /pxPerMetre/);
  expectError(plan, /SOURCE needs url, credit and title/);
  expectError(plan, /SOURCE needs originPx and sizePx/);
  expectError(plan, /OUTLINE leaves/);
  plan.walls[0].from = [NaN, 0];
  expectError(plan, /NaN, Infinity or null/);
});

test('a plan with no exterior walls is rejected', () => {
  const plan = copy();
  plan.walls = plan.walls.filter((w) => w.kind !== 'exterior');
  expectError(plan, /no exterior walls/);
});

test('errors are capped so a tracer is not flooded', () => {
  const plan = copy();
  for (let i = 0; i < 100; i++) plan.furniture.push({ type: 'hovercraft', x: 1, y: 1 });
  assert.equal(checkPlan(plan).length, 60);
});
