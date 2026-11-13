// checkPlan must catch each kind of mistake a tracer makes, with a message
// that says where. The Office plan is valid, so each test breaks one thing.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as office from '../js/plans/roomsketcher-office.js';
import { checkPlan } from './invariants.mjs';

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
  expectError(plan, /rooms overlap/);
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
